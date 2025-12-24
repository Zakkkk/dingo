package semantic

import (
	"go/ast"
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
// 1. Walking Go AST and collecting typed entities
// 2. Mapping Go positions back to Dingo positions using lineMappings
// 3. Detecting operators and enriching with context
// 4. Building final sorted map
func (b *Builder) Build() (*Map, error) {
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

	// Verify the identifier exists in Dingo source at this position
	// This filters out generated identifiers (like tmp, err from error propagation)
	// that map to Dingo positions where different code exists
	// Also get the actual adjusted column for entity storage
	verified, actualCol := b.verifyIdentInSource(node.Name, dingoLine, dingoCol)
	if !verified {
		return nil
	}

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
		EndCol:  actualCol + len(node.Name),
		Kind:    KindIdent,
		Object:  obj,
		Type:    entityType,
		Context: context,
	}
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

	// Verify in source
	verified, actualCol := b.verifyIdentInSource(baseIdent.Name, dingoLine, dingoCol)
	if !verified {
		return nil
	}

	return &SemanticEntity{
		Line:   dingoLine,
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

	// Verify in source
	verified, actualCol := b.verifyIdentInSource(baseIdent.Name, dingoLine, dingoCol)
	if !verified {
		return nil
	}

	return &SemanticEntity{
		Line:   dingoLine,
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

// goPosToDingoPos maps a Go AST position back to Dingo source position
// FOLLOWS CLAUDE.MD: Uses token.FileSet and lineMappings, NOT byte arithmetic
func (b *Builder) goPosToDingoPos(goPos token.Pos) (line, col int, ok bool) {
	// Get Go position from FileSet
	// NOTE: Due to //line directives, FileSet reports positions in terms of Dingo lines,
	// not actual Go line numbers. So goLine here is actually the Dingo line.
	goPosition := b.goFset.Position(goPos)
	goLine := goPosition.Line // This is actually the Dingo line due to //line directives
	goCol := goPosition.Column

	// Check if //line directive has mapped this to a Dingo file
	// If the filename ends with .dingo, we're already in Dingo coordinates
	isDingoPos := strings.HasSuffix(goPosition.Filename, ".dingo")

	// Check if this is a pass-through file (no transformations that shift lines)
	// When the transpiler doesn't add //line directives (e.g., files with only type rewrites
	// like Result[T,E] → dgo.Result[T,E]), the filename will be *.dingo.go
	// IMPORTANT: We only use pass-through if line counts are similar.
	// Import expansion (single-line → multi-line) can shift all subsequent lines.
	isPassThroughFile := false
	if strings.HasSuffix(goPosition.Filename, ".dingo.go") && len(b.lineMappings) == 0 {
		// Count lines in Dingo source
		dingoLineCount := 1
		for _, c := range b.dingoSource {
			if c == '\n' {
				dingoLineCount++
			}
		}
		// Get Go file line count from FileSet
		// The Go AST file ends at b.goAST.End(), which gives us the last position
		if b.goAST != nil {
			goEndPos := b.goFset.Position(b.goAST.End())
			goLineCount := goEndPos.Line
			// Allow small difference (2 lines) for trailing whitespace
			if abs(goLineCount-dingoLineCount) <= 2 {
				isPassThroughFile = true
			}
		}
	}

	// Find the line mapping that covers this position (for transformed lines)
	for _, mapping := range b.lineMappings {
		if goLine == mapping.DingoLine {
			// This position maps to this Dingo line
			// Try to find precise column mapping
			dingoCol, _ := b.translateGoColumn(goLine, goCol)
			return mapping.DingoLine, dingoCol, true
		}
	}

	// No explicit mapping found
	// If we're already in Dingo coordinates (//line directive applied), use identity mapping
	if isDingoPos {
		return goLine, goCol, true
	}

	// For pass-through files (no transformations), use identity mapping
	// The Go code is essentially identical to Dingo code except for type rewrites
	if isPassThroughFile {
		return goLine, goCol, true
	}

	// Not in Dingo coordinates - this Go code is before any //line directive
	// These are typically imports/package which map 1:1
	return 0, 0, false
}

// abs returns the absolute value of x
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// verifyIdentFuzzy tries to verify an identifier with fuzzy line matching.
// go/printer can cause 1-2 line drift by reformatting comments, so we try
// the exact line first, then ±1 and ±2 lines.
//
// Returns (actualLine, actualCol, found):
//   - actualLine: the line where the identifier was found
//   - actualCol: the column where the identifier was found
//   - found: true if identifier was found
func (b *Builder) verifyIdentFuzzy(name string, line, col int) (int, int, bool) {
	// Try exact line first, then adjacent lines
	for _, delta := range []int{0, -1, 1, -2, 2} {
		tryLine := line + delta
		if tryLine < 1 {
			continue
		}
		found, actualCol := b.verifyIdentInSource(name, tryLine, col)
		if found {
			return tryLine, actualCol, true
		}
	}
	return 0, 0, false
}

// verifyIdentInSource checks if the identifier name exists at the given
// position in the Dingo source. This filters out generated code that maps
// to Dingo positions where different text exists.
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

	// Find the line in Dingo source (1-indexed)
	lineStart := 0
	currentLine := 1
	for i, c := range b.dingoSource {
		if currentLine == line {
			lineStart = i
			break
		}
		if c == '\n' {
			currentLine++
		}
	}
	if currentLine != line {
		// Line not found - might be beyond source
		return false, 0
	}

	// Find line end
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

	// Not found within ±2 tolerance - try searching the entire line
	// This handles error propagation lambdas where column positions don't match
	// due to transformation (e.g., `? |err| NewAppError(...)` → `return NewAppError(...)`)
	//
	// Safety: We only accept identifiers that go/types found in the generated Go,
	// and //line directives ensure they map to the correct Dingo line.
	// Searching the line is safe because we're looking for the SAME identifier.
	foundIdx := findIdentifierInLine(lineContent, name)
	if foundIdx >= 0 {
		actualCol := foundIdx + 1 // Convert 0-indexed to 1-indexed
		return true, actualCol
	}

	// Truly not found on this line
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
	// NOTE: goLine is the line as reported by Go FileSet, which equals DingoLine
	// due to //line directives. So we match against DingoLine, not GoLine.
	for _, m := range b.columnMappings {
		if m.DingoLine == goLine {
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
func (b *Builder) inferContext(node ast.Node, typ types.Type) *DingoContext {
	// Check if this is an unwrapped Result/Option type
	unwrapped, original, ok := b.extractResultType(typ)
	if ok {
		return &DingoContext{
			Kind:          ContextErrorProp,
			OriginalType:  original,
			UnwrappedType: unwrapped,
			Description:   "From error propagation",
		}
	}

	// No special context
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
