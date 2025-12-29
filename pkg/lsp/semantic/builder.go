package semantic

import (
	"go/ast"
	"go/scanner"
	"go/token"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
)

// Builder constructs a semantic Map from transpilation results
type Builder struct {
	goAST          *ast.File
	goFset         *token.FileSet
	typesInfo      *types.Info
	lineMappings   []sourcemap.LineMapping
	columnMappings []sourcemap.ColumnMapping // For accurate column translation
	dingoSource    []byte
	dingoFset      *token.FileSet
	dingoFile      string

	// Computed during Build() for position mapping without //line directives
	importLineOffset  int         // Go import lines - Dingo import lines
	goImportEndLine   int         // Line after last import in Go
	dingoImportEnd    int         // Line after last import in Dingo
	dingoSourceFile   *token.File // For efficient line→offset conversion (CLAUDE.md compliant)

	// Per-region line offsets computed from declaration positions
	// This handles go/printer adding/removing lines throughout the file
	// Each entry says: "from goLine onwards, use this offset"
	regionOffsets []regionOffset // Sorted by goLine
}

// regionOffset represents the line offset for a region starting at goLine.
// Offset = goLine - dingoLine, so dingoLine = goLine - offset.
type regionOffset struct {
	goLine int // First Go line where this offset applies
	offset int // goLine - dingoLine at this declaration
}

// NewBuilder creates a Builder
// IMPORTANT: Follows CLAUDE.md rules - uses token.FileSet for all position tracking
func NewBuilder(
	goAST *ast.File,
	goFset *token.FileSet,
	typesInfo *types.Info,
	lineMappings []sourcemap.LineMapping,
	columnMappings []sourcemap.ColumnMapping,
	dingoSource []byte,
	dingoFset *token.FileSet,
	dingoFile string,
) *Builder {
	return &Builder{
		goAST:          goAST,
		goFset:         goFset,
		typesInfo:      typesInfo,
		lineMappings:   lineMappings,
		columnMappings: columnMappings,
		dingoSource:    dingoSource,
		dingoFset:      dingoFset,
		dingoFile:      dingoFile,
	}
}

// Build creates a semantic Map by:
// 1. Computing import line offset (Go imports - Dingo imports)
// 2. Walking Go AST and collecting typed entities
// 3. Mapping Go positions back to Dingo positions using the offset
// 4. Detecting operators and enriching with context
// 5. Building final sorted map
func (b *Builder) Build() (*Map, error) {
	// Initialize Dingo source file for line→offset conversion (CLAUDE.md compliant)
	// This uses token.FileSet instead of manual newline counting
	if len(b.dingoSource) > 0 {
		fset := token.NewFileSet()
		b.dingoSourceFile = fset.AddFile(b.dingoFile, fset.Base(), len(b.dingoSource))
		b.dingoSourceFile.SetLinesForContent(b.dingoSource)
	}

	// Compute import expansion offset for position mapping
	b.computeImportOffset()

	// Compute per-region offsets using structural correspondence
	// This handles go/printer adding/removing lines throughout the file
	b.computeRegionOffsets()

	var entities []SemanticEntity

	// Walk Go AST and collect entities with type information
	ast.Inspect(b.goAST, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch node := n.(type) {
		case *ast.Ident:
			// Named identifiers (variables, functions, constants)
			entity := b.handleIdent(node)
			if entity != nil {
				entities = append(entities, *entity)
			}

		case *ast.CallExpr:
			// Function/method calls
			entity := b.handleCallExpr(node)
			if entity != nil {
				entities = append(entities, *entity)
			}

		case *ast.SelectorExpr:
			// Field/method access (x.Field)
			entity := b.handleSelectorExpr(node)
			if entity != nil {
				entities = append(entities, *entity)
			}

		case *ast.IndexListExpr:
			// Generic type instantiation: Result[User, DBError]
			entity := b.handleIndexListExpr(node)
			if entity != nil {
				entities = append(entities, *entity)
			}

		case *ast.IndexExpr:
			// Single-param generic type: Option[User]
			entity := b.handleIndexExpr(node)
			if entity != nil {
				entities = append(entities, *entity)
			}
		}

		return true
	})

	// Detect operators and add them to entities
	// Operators need special handling because they don't exist in Go AST
	operatorEntities := b.detectOperators()
	entities = append(entities, operatorEntities...)

	// Detect lambda parameters
	// Lambda params like |err| or e => don't exist in Go AST (they become err2 etc.)
	lambdaEntities := b.detectLambdaParams()
	entities = append(entities, lambdaEntities...)

	// Build and return the map
	return NewMap(entities), nil
}

// handleIdent processes an identifier node
func (b *Builder) handleIdent(node *ast.Ident) *SemanticEntity {
	// Get the object this identifier refers to
	if b.typesInfo == nil {
		return nil
	}
	obj := b.typesInfo.ObjectOf(node)
	if obj == nil {
		return nil
	}

	// Skip built-in identifiers and package names
	if obj.Pkg() == nil && obj.Parent() == types.Universe {
		return nil
	}

	// Map Go position to Dingo position
	goPos := node.Pos()
	dingoLine, dingoCol, ok := b.goPosToDingoPos(goPos)
	if !ok {
		return nil
	}

	// Get the name to verify in Dingo source
	// For enum variants, Go uses "EnumVariant" but Dingo uses just "Variant"
	verifyName := node.Name
	isEnumVariant := false
	if typeName, ok := obj.(*types.TypeName); ok {
		if variantName := extractDingoVariantName(typeName.Type()); variantName != "" {
			verifyName = variantName
			isEnumVariant = true
		}
	}

	// Verify the identifier exists in Dingo source at this position
	// Use fuzzy matching because go/printer can cause line drift (reformatting comments)
	// This filters out generated identifiers (like tmp, err from error propagation)
	// Also get the actual adjusted line/column for entity storage
	var actualLine, actualCol int
	var verified bool
	if isEnumVariant {
		// For enum variants, search the entire file because the Go enum expansion
		// creates multiple types at different positions than the Dingo source
		actualLine, actualCol, verified = b.findIdentInSource(verifyName)
	} else {
		actualLine, actualCol, verified = b.verifyIdentFuzzy(verifyName, dingoLine, dingoCol)
	}
	if !verified {
		return nil
	}
	dingoLine = actualLine

	// Determine context (error propagation, etc.)
	context := b.inferContext(node, obj.Type())

	// Get the type - prefer the expression type (includes instantiated generics)
	// over the object type (generic definition)
	entityType := obj.Type()
	if tv, ok := b.typesInfo.Types[node]; ok && tv.Type != nil {
		entityType = tv.Type
	}

	return &SemanticEntity{
		Line:    dingoLine,
		Col:     actualCol,
		EndCol:  actualCol + len(verifyName),
		Kind:    KindIdent,
		Object:  obj,
		Type:    entityType,
		Context: context,
	}
}

// extractDingoVariantName extracts the Dingo variant name from a Go enum variant type.
// Go enum variants are named "<Enum><Variant>" but Dingo uses just "<Variant>".
// Returns empty string if not an enum variant.
func extractDingoVariantName(t types.Type) string {
	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}

	// Must be a struct
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return ""
	}

	// Check for marker method is<Enum>()
	var enumName string
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if strings.HasPrefix(method.Name(), "is") && len(method.Name()) > 2 {
			sig := method.Type().(*types.Signature)
			if sig.Params().Len() == 0 && sig.Results().Len() == 0 {
				enumName = method.Name()[2:] // Strip "is" prefix
				break
			}
		}
	}

	if enumName == "" {
		return ""
	}

	// Strip enum prefix to get variant name
	typeName := named.Obj().Name()
	if !strings.HasPrefix(typeName, enumName) {
		return ""
	}
	variantName := typeName[len(enumName):]
	if variantName == "" {
		return ""
	}

	return variantName
}

// handleCallExpr processes a function call expression
// NOTE: Currently returns nil because the function identifier (handleIdent)
// already provides the function signature for hover, which is more useful
// than the call's return type. This avoids overlapping entities.
func (b *Builder) handleCallExpr(node *ast.CallExpr) *SemanticEntity {
	// Skip for now - the function identifier provides better hover info
	// The CallExpr type is the return type, but hover on func name should
	// show the function signature (from the Ident), not return type
	return nil
}

// handleSelectorExpr processes a selector expression (x.Field)
func (b *Builder) handleSelectorExpr(node *ast.SelectorExpr) *SemanticEntity {
	// Get the type of the selector
	tv, ok := b.typesInfo.Types[node]
	if !ok {
		return nil
	}

	// Map Go position to Dingo position (position of the selector, not X)
	goPos := node.Sel.Pos()
	dingoLine, dingoCol, ok := b.goPosToDingoPos(goPos)
	if !ok {
		return nil
	}

	// Verify selector exists in source with fuzzy line matching
	// go/printer can cause 1-2 line drift by reformatting comments
	actualLine, actualCol, verified := b.verifyIdentFuzzy(node.Sel.Name, dingoLine, dingoCol)
	if !verified {
		return nil
	}
	dingoLine = actualLine

	// Get the object for methods/fields/functions
	var obj types.Object
	if sel := b.typesInfo.Selections[node]; sel != nil {
		// Method or field selection (e.g., user.Name, result.IsOk())
		obj = sel.Obj()
	} else if use := b.typesInfo.Uses[node.Sel]; use != nil {
		// Qualified identifier (e.g., json.Marshal, fmt.Println)
		obj = use
	}

	// If we couldn't find an object, let handleIdent handle this node instead
	// handleIdent uses ObjectOf() which works for more cases
	if obj == nil {
		return nil
	}

	// Determine context
	context := b.inferContext(node, tv.Type)

	return &SemanticEntity{
		Line:    dingoLine,
		Col:     actualCol,
		EndCol:  actualCol + len(node.Sel.Name),
		Kind:    KindField,
		Object:  obj,
		Type:    tv.Type,
		Context: context,
	}
}

// handleIndexListExpr processes generic type instantiation: Result[User, DBError]
func (b *Builder) handleIndexListExpr(node *ast.IndexListExpr) *SemanticEntity {
	if b.typesInfo == nil {
		return nil
	}

	// Get the type of the full expression (instantiated generic)
	tv, ok := b.typesInfo.Types[node]
	if !ok || tv.Type == nil {
		return nil
	}

	// Get the base identifier (e.g., "Result" from "Result[User, DBError]")
	var baseIdent *ast.Ident
	switch x := node.X.(type) {
	case *ast.Ident:
		baseIdent = x
	case *ast.SelectorExpr:
		baseIdent = x.Sel
	default:
		return nil
	}

	// Map Go position to Dingo position
	goPos := baseIdent.Pos()
	dingoLine, dingoCol, ok := b.goPosToDingoPos(goPos)
	if !ok {
		return nil
	}

	// Verify in source with fuzzy matching (go/printer can cause line drift)
	actualLine, actualCol, verified := b.verifyIdentFuzzy(baseIdent.Name, dingoLine, dingoCol)
	if !verified {
		return nil
	}

	return &SemanticEntity{
		Line:   actualLine,
		Col:    actualCol,
		EndCol: actualCol + len(baseIdent.Name),
		Kind:   KindType,
		Type:   tv.Type, // This is the instantiated type with type args
	}
}

// handleIndexExpr processes single-param generic type: Option[User]
func (b *Builder) handleIndexExpr(node *ast.IndexExpr) *SemanticEntity {
	if b.typesInfo == nil {
		return nil
	}

	// Get the type of the full expression (instantiated generic)
	tv, ok := b.typesInfo.Types[node]
	if !ok || tv.Type == nil {
		return nil
	}

	// Get the base identifier (e.g., "Option" from "Option[User]")
	var baseIdent *ast.Ident
	switch x := node.X.(type) {
	case *ast.Ident:
		baseIdent = x
	case *ast.SelectorExpr:
		baseIdent = x.Sel
	default:
		return nil
	}

	// Map Go position to Dingo position
	goPos := baseIdent.Pos()
	dingoLine, dingoCol, ok := b.goPosToDingoPos(goPos)
	if !ok {
		return nil
	}

	// Verify in source with fuzzy matching (go/printer can cause line drift)
	actualLine, actualCol, verified := b.verifyIdentFuzzy(baseIdent.Name, dingoLine, dingoCol)
	if !verified {
		return nil
	}

	return &SemanticEntity{
		Line:   actualLine,
		Col:    actualCol,
		EndCol: actualCol + len(baseIdent.Name),
		Kind:   KindType,
		Type:   tv.Type, // This is the instantiated type with type args
	}
}

// detectOperators finds Dingo operators and creates entities for them
func (b *Builder) detectOperators() []SemanticEntity {
	// Create a FileSet for operator detection if not provided
	// This is safe because we only need it for position tracking during tokenization
	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	// Use the operators.go DetectOperators function
	operators := DetectOperators(b.dingoSource, fset, b.dingoFile)

	var entities []SemanticEntity
	for _, op := range operators {
		// Try to infer the type for this operator by looking nearby in the semantic map
		// For now, we create the entity without a type - this will be enriched later
		// when we have more context about the expression it applies to

		var context *DingoContext
		switch op.Kind {
		case ContextErrorProp:
			// Error propagation: always provide context (hover should work even without type info)
			context = &DingoContext{
				Kind:        ContextErrorProp,
				Description: "Error propagation operator",
			}
			// Try to extract Result type for richer hover info
			if op.ExprType != nil {
				unwrapped, original, ok := b.extractResultType(op.ExprType)
				if ok {
					context.OriginalType = original
					context.UnwrappedType = unwrapped
				}
			}

		case ContextSafeNav:
			context = &DingoContext{
				Kind:        ContextSafeNav,
				Description: "Safe navigation operator",
			}

		case ContextNullCoal:
			context = &DingoContext{
				Kind:        ContextNullCoal,
				Description: "Null coalescing operator",
			}
		}

		entities = append(entities, SemanticEntity{
			Line:    op.Line,
			Col:     op.Col,
			EndCol:  op.EndCol,
			Kind:    KindOperator,
			Type:    op.ExprType,
			Context: context,
		})
	}

	return entities
}

// detectLambdaParams finds lambda parameters in Dingo source and creates entities
// Lambda parameters (|err|, e =>) don't exist in Go AST - they become err2 etc.
func (b *Builder) detectLambdaParams() []SemanticEntity {
	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	params := DetectLambdaParams(b.dingoSource, fset, b.dingoFile)

	var entities []SemanticEntity
	for _, p := range params {
		// For error propagation lambdas, the parameter type is always error
		// We create an entity with a context describing this
		entities = append(entities, SemanticEntity{
			Line:   p.Line,
			Col:    p.Col,
			EndCol: p.EndCol,
			Kind:   KindLambda,
			Context: &DingoContext{
				Kind:        ContextLambda,
				Description: "Lambda parameter (type: `error`)",
			},
		})
	}

	return entities
}

// computeImportOffset calculates the line offset between Go and Dingo code.
// This is needed because import expansion and comment processing shift line numbers.
// Without //line directives, we must compute this offset.
//
// CLAUDE.md COMPLIANT: Uses go/scanner for tokenization instead of string manipulation.
// Position information flows through the token system.
//
// Algorithm: Find the first non-import declaration in both files and compute
// the difference. This is more robust than import-end comparison because it
// accounts for comment processing differences too.
func (b *Builder) computeImportOffset() {
	// Find line of first non-import declaration in Go AST
	b.goImportEndLine = 1
	goFirstDeclLine := 0
	if b.goAST != nil {
		for _, decl := range b.goAST.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					// Track import end
					endPos := b.goFset.Position(d.End())
					if endPos.Line > b.goImportEndLine {
						b.goImportEndLine = endPos.Line
					}
				} else {
					// First non-import GenDecl (type, const, var)
					if goFirstDeclLine == 0 {
						goFirstDeclLine = b.goFset.Position(d.Pos()).Line
					}
				}
			case *ast.FuncDecl:
				// First function
				if goFirstDeclLine == 0 {
					goFirstDeclLine = b.goFset.Position(d.Pos()).Line
				}
			}
		}
	}

	// Find line of first non-import declaration in Dingo source using go/scanner
	// This is CLAUDE.md compliant - uses token system instead of string manipulation
	b.dingoImportEnd = 1
	dingoFirstDeclLine := 0
	if b.dingoSourceFile != nil && len(b.dingoSource) > 0 {
		var s scanner.Scanner
		fset := token.NewFileSet()
		file := fset.AddFile("", fset.Base(), len(b.dingoSource))
		// Ignore errors - Dingo syntax extensions may cause scanner errors
		s.Init(file, b.dingoSource, func(pos token.Position, msg string) {}, scanner.ScanComments)

		inImportBlock := false
		for {
			pos, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}

			position := fset.Position(pos)

			// Track import end
			if tok == token.IMPORT {
				b.dingoImportEnd = position.Line
			}
			if tok == token.LPAREN && b.dingoImportEnd == position.Line {
				inImportBlock = true
			}
			if tok == token.RPAREN && inImportBlock {
				inImportBlock = false
				b.dingoImportEnd = position.Line
			}

			// Find first declaration keyword (type, func, const, var, or enum)
			// Note: "enum" is a Dingo keyword that Go's scanner sees as IDENT
			if !inImportBlock && dingoFirstDeclLine == 0 {
				isGoDecl := tok == token.TYPE || tok == token.FUNC || tok == token.CONST || tok == token.VAR
				isDingoEnum := tok == token.IDENT && lit == "enum"
				if isGoDecl || isDingoEnum {
					dingoFirstDeclLine = position.Line
					break // Found the first declaration
				}
			}
		}
	}

	// Compute offset using first declaration lines (most accurate)
	// This accounts for both import expansion AND comment processing differences
	if goFirstDeclLine > 0 && dingoFirstDeclLine > 0 {
		b.importLineOffset = goFirstDeclLine - dingoFirstDeclLine
	} else {
		// Fallback to import-end calculation
		b.importLineOffset = b.goImportEndLine - b.dingoImportEnd
	}
}

// computeRegionOffsets builds per-region line offsets by matching declarations
// between Go AST and Dingo source. This handles go/printer adding/removing
// lines at various points in the file.
//
// CLAUDE.md COMPLIANT: Uses go/ast for Go positions and go/scanner for Dingo.
// Position information flows through the token system.
func (b *Builder) computeRegionOffsets() {
	// Collect Go declaration names and lines from AST
	goDecls := b.collectGoDeclarations()

	// Collect Dingo declaration names and lines using go/scanner
	dingoDecls := b.collectDingoDeclarations()

	// Match declarations by name and compute offsets
	// Start with the import offset as the first region
	b.regionOffsets = []regionOffset{{
		goLine: b.goImportEndLine + 1,
		offset: b.importLineOffset,
	}}

	for _, goDecl := range goDecls {
		if dingoLine, ok := dingoDecls[goDecl.name]; ok {
			offset := goDecl.line - dingoLine
			// Only add if offset differs from previous region
			if len(b.regionOffsets) == 0 || b.regionOffsets[len(b.regionOffsets)-1].offset != offset {
				b.regionOffsets = append(b.regionOffsets, regionOffset{
					goLine: goDecl.line,
					offset: offset,
				})
			}
		}
	}
}

// declInfo holds declaration name and line number
type declInfo struct {
	name string
	line int
}

// collectGoDeclarations walks the Go AST to collect declaration names and lines.
// Uses token.Pos from AST nodes (CLAUDE.md compliant).
func (b *Builder) collectGoDeclarations() []declInfo {
	var decls []declInfo

	if b.goAST == nil {
		return decls
	}

	for _, decl := range b.goAST.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			line := b.goFset.Position(d.Pos()).Line
			decls = append(decls, declInfo{name: d.Name.Name, line: line})

		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue // Skip imports
			}
			// Handle type, const, var declarations
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					line := b.goFset.Position(s.Pos()).Line
					decls = append(decls, declInfo{name: s.Name.Name, line: line})
				case *ast.ValueSpec:
					for _, name := range s.Names {
						line := b.goFset.Position(name.Pos()).Line
						decls = append(decls, declInfo{name: name.Name, line: line})
					}
				}
			}
		}
	}

	return decls
}

// collectDingoDeclarations scans Dingo source to find declaration names and lines.
// Uses go/scanner for tokenization (CLAUDE.md compliant).
func (b *Builder) collectDingoDeclarations() map[string]int {
	decls := make(map[string]int)

	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		return decls
	}

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(b.dingoSource))
	// Ignore errors - Dingo syntax extensions may cause scanner errors
	s.Init(file, b.dingoSource, func(pos token.Position, msg string) {}, scanner.ScanComments)

	// State machine for finding declarations
	// Track both token type and literal for Dingo-specific keywords like "enum"
	var prevTok token.Token
	var prevLit string
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// After "type", "func", "const", "var", or "enum", the next IDENT is a declaration name
		// Note: "enum" is a Dingo keyword that Go's scanner sees as IDENT
		isGoDecl := prevTok == token.TYPE || prevTok == token.FUNC || prevTok == token.CONST || prevTok == token.VAR
		isDingoEnum := prevTok == token.IDENT && prevLit == "enum"

		if isGoDecl || isDingoEnum {
			if tok == token.IDENT {
				line := fset.Position(pos).Line
				decls[lit] = line
			}
		}

		prevTok = tok
		prevLit = lit
	}

	return decls
}

// goPosToDingoPos maps a Go AST position back to Dingo source position
// Uses region-specific offsets computed from declaration matching.
func (b *Builder) goPosToDingoPos(goPos token.Pos) (line, col int, ok bool) {
	// Get Go position from FileSet
	goPosition := b.goFset.Position(goPos)
	goLine := goPosition.Line
	goCol := goPosition.Column

	// Check if //line directive has mapped this to a Dingo file
	// If the filename ends with .dingo, we're already in Dingo coordinates
	isDingoPos := strings.HasSuffix(goPosition.Filename, ".dingo")
	if isDingoPos {
		return goLine, goCol, true
	}

	// Find the line mapping that covers this position (for transformed lines)
	// Without //line directives, we check if goLine falls within [GoLineStart, GoLineEnd]
	for _, mapping := range b.lineMappings {
		if goLine >= mapping.GoLineStart && goLine <= mapping.GoLineEnd {
			dingoCol, _ := b.translateGoColumn(goLine, goCol)
			return mapping.DingoLine, dingoCol, true
		}
	}

	// No explicit line mappings - use region-specific offsets
	// For positions before/in imports, use identity mapping
	if goLine <= b.goImportEndLine {
		// Within package/import block - identity mapping
		return goLine, goCol, true
	}

	// Find the applicable region offset using binary search
	// Region offsets are sorted by goLine, find the last one where goLine >= offset.goLine
	offset := b.importLineOffset // Default fallback
	for i := len(b.regionOffsets) - 1; i >= 0; i-- {
		if goLine >= b.regionOffsets[i].goLine {
			offset = b.regionOffsets[i].offset
			break
		}
	}

	// Calculate cumulative expansion from error prop line mappings
	// Each line mapping represents 1 Dingo line expanding to multiple Go lines
	// We need to subtract this expansion to get the correct Dingo line
	cumulativeExpansion := 0
	for _, mapping := range b.lineMappings {
		// Only count mappings that are BEFORE this Go line
		if goLine > mapping.GoLineEnd {
			// This error prop is before our position, count its expansion
			// Expansion = (GoLineEnd - GoLineStart + 1) - 1 = GoLineEnd - GoLineStart
			expansion := mapping.GoLineEnd - mapping.GoLineStart
			cumulativeExpansion += expansion
		}
	}

	// For positions after imports, subtract both region offset and cumulative expansion
	dingoLine := goLine - offset - cumulativeExpansion
	if dingoLine < 1 {
		dingoLine = 1
	}
	return dingoLine, goCol, true
}

// verifyIdentFuzzy verifies an identifier exists at the computed position.
// With per-region offsets (computed from structural declaration matching),
// line positions are accurate. We only apply column tolerance (±2) for
// tab handling and minor transformation differences.
//
// CLAUDE.md COMPLIANT: No string-based line searching. Position accuracy
// comes from token-based region offset computation.
//
// Returns (actualLine, actualCol, found):
//   - actualLine: the line where the identifier was found (same as input line)
//   - actualCol: the actual column where identifier was found
//   - found: true if identifier was verified
func (b *Builder) verifyIdentFuzzy(name string, line, col int) (int, int, bool) {
	// With accurate region offsets, we check the exact line
	found, actualCol := b.verifyIdentInSource(name, line, col)
	if found {
		return line, actualCol, true
	}
	return 0, 0, false
}

// findIdentInSource searches the entire Dingo source for an identifier using go/scanner.
// Used for enum variants where position mapping breaks down due to Go expansion.
// Returns (line, col, found) where line/col are 1-indexed.
//
// CLAUDE.md COMPLIANT: Uses go/scanner for tokenization instead of string scanning.
// Position information flows through the token system.
func (b *Builder) findIdentInSource(name string) (int, int, bool) {
	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		return 0, 0, false
	}

	// Use go/scanner to find the identifier token
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(b.dingoSource))
	// Ignore errors - Dingo syntax extensions may cause scanner errors
	s.Init(file, b.dingoSource, func(pos token.Position, msg string) {}, scanner.ScanComments)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT && lit == name {
			position := fset.Position(pos)
			return position.Line, position.Column, true
		}
	}

	return 0, 0, false
}

// verifyIdentInSource checks if the identifier name exists at the given
// position in the Dingo source. This filters out generated code that maps
// to Dingo positions where different text exists.
//
// CLAUDE.md COMPLIANT: Uses token.File.LineStart() for line→offset conversion
// instead of manual newline counting. Position information flows through token system.
//
// For example, "tmp" generated by error propagation at Dingo line 55, col 1
// would fail verification if the Dingo source has "userID" at that position.
//
// Due to column mapping precision (tab handling, transformation offsets),
// we allow a small tolerance of ±2 columns when searching for the identifier.
//
// Returns (found bool, adjustedCol int):
//   - found: true if identifier exists at or near the position
//   - adjustedCol: the actual column where identifier was found (for entity storage)
func (b *Builder) verifyIdentInSource(name string, line, col int) (bool, int) {
	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		// No source to verify against - accept everything
		return true, col
	}

	// Use token.File for line→offset conversion (CLAUDE.md compliant)
	// This avoids manual newline counting
	if b.dingoSourceFile == nil {
		// Fallback if not initialized
		return true, col
	}

	// Check line bounds
	lineCount := b.dingoSourceFile.LineCount()
	if line < 1 || line > lineCount {
		return false, 0
	}

	// Get line start offset using token system
	lineStartPos := b.dingoSourceFile.LineStart(line)
	lineStart := int(lineStartPos) - b.dingoSourceFile.Base()

	// Find line end by scanning from lineStart
	lineEnd := lineStart
	for lineEnd < len(b.dingoSource) && b.dingoSource[lineEnd] != '\n' {
		lineEnd++
	}

	// Extract the line content
	lineContent := b.dingoSource[lineStart:lineEnd]

	// Try to find the identifier at the expected column or nearby (±2 tolerance)
	// This handles small column drift from transformation offsets
	// Search pattern: 0, -1, +1, -2, +2
	for _, delta := range []int{0, -1, 1, -2, 2} {
		tryCol := col + delta
		startIdx := tryCol - 1 // Convert 1-indexed to 0-indexed

		if startIdx < 0 {
			continue
		}
		endIdx := startIdx + len(name)
		if endIdx > len(lineContent) {
			continue
		}

		actual := string(lineContent[startIdx:endIdx])
		if actual == name {
			return true, tryCol
		}
	}

	// Not found within ±2 column tolerance
	// Fall back to searching the entire line for the identifier
	// This is necessary for lambda bodies in error propagation:
	// - Go line 72: return NewAppError(403, "permission denied", err2)
	// - This line is within the error prop range [70, 74] mapping to Dingo line 63
	// - But the column mapping only covers line 70, not 72
	// - So we need to search the whole line to find NewAppError
	//
	// We only do this for identifiers that are relatively unique (>=4 chars)
	// to avoid false matches with short common names
	if len(name) >= 4 {
		foundIdx := findIdentifierInLine(lineContent, name)
		if foundIdx >= 0 {
			return true, foundIdx + 1 // Convert 0-indexed to 1-indexed
		}
	}

	return false, 0
}

// findIdentifierInLine searches for an identifier in the line content.
// Returns the 0-indexed position of the identifier, or -1 if not found.
// Uses word boundary checking to avoid matching substrings.
func findIdentifierInLine(line []byte, name string) int {
	nameBytes := []byte(name)
	for i := 0; i <= len(line)-len(nameBytes); i++ {
		// Check if this position has the identifier
		if string(line[i:i+len(nameBytes)]) != name {
			continue
		}

		// Check word boundaries to avoid matching substrings
		// Before: must be start of line or non-identifier char
		if i > 0 && isIdentChar(line[i-1]) {
			continue
		}
		// After: must be end of line or non-identifier char
		afterIdx := i + len(nameBytes)
		if afterIdx < len(line) && isIdentChar(line[afterIdx]) {
			continue
		}

		return i
	}
	return -1
}

// isIdentChar returns true if c is a valid identifier character (letter, digit, underscore)
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// translateGoColumn translates a Go column to Dingo column using column mappings.
// Returns the translated column and whether a mapping was found.
// This enables accurate hover when multiple Go entities map to the same Dingo line.
func (b *Builder) translateGoColumn(goLine, goCol int) (dingoCol int, found bool) {
	// Look for a column mapping that contains this position
	// NOTE: Without //line directives, goLine is the actual Go line number.
	// We match against m.GoLine (the Go line in the mapping).
	for _, m := range b.columnMappings {
		if m.GoLine == goLine {
			// Check if column falls within the mapped expression range
			// The mapping covers [GoCol, GoCol + Length)
			if goCol >= m.GoCol && goCol < m.GoCol+m.Length {
				// Translate using the column offset
				// offset = GoCol - DingoCol
				offset := m.GoCol - m.DingoCol
				result := goCol - offset
				return result, true
			}
		}
	}
	// No column mapping found - return original column
	return goCol, false
}

// inferContext determines Dingo-specific context for an entity
// NOTE: This no longer adds error propagation context just because a type is Option/Result.
// Error propagation context is only added for:
// 1. The ? operator (detected separately in detectOperators)
// 2. Variables that are the RESULT of unwrapping via ? (TODO: track during transformation)
//
// A field of type Option[string] should show Option[string], not "string (from Option)"
func (b *Builder) inferContext(node ast.Node, typ types.Type) *DingoContext {
	// Currently no automatic context inference
	// Error propagation context is handled by detectOperators for ? operators
	// Regular Option/Result fields should just show their actual type
	return nil
}

// extractResultType extracts T from Result[T, E] or Option[T]
// Returns (innerType, originalType, true) if successful
func (b *Builder) extractResultType(t types.Type) (inner, original types.Type, ok bool) {
	// Handle named types
	named, ok := t.(*types.Named)
	if !ok {
		return nil, nil, false
	}

	// Get the type name
	typeName := named.Obj().Name()

	// Check for Result[T, E] or Option[T]
	if typeName != "Result" && typeName != "Option" {
		return nil, nil, false
	}

	// Check package path (should be dgo package)
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return nil, nil, false
	}

	// Package name should contain "dgo" (could be "github.com/user/dgo")
	if !strings.Contains(pkg.Path(), "dgo") && pkg.Name() != "dgo" {
		return nil, nil, false
	}

	// Extract type arguments
	// Result[T, E] has 2 type args, Option[T] has 1
	if named.TypeArgs() == nil || named.TypeArgs().Len() == 0 {
		return nil, nil, false
	}

	// Get the first type argument (T)
	innerType := named.TypeArgs().At(0)

	return innerType, t, true
}
