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
		}

		return true
	})

	// Detect operators and add them to entities
	// Operators need special handling because they don't exist in Go AST
	operatorEntities := b.detectOperators()
	entities = append(entities, operatorEntities...)

	// Build and return the map
	return NewMap(entities), nil
}

// handleIdent processes an identifier node
func (b *Builder) handleIdent(node *ast.Ident) *SemanticEntity {
	// Get the object this identifier refers to
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
	if !b.verifyIdentInSource(node.Name, dingoLine, dingoCol) {
		return nil
	}

	// Determine context (error propagation, etc.)
	context := b.inferContext(node, obj.Type())

	return &SemanticEntity{
		Line:    dingoLine,
		Col:     dingoCol,
		EndCol:  dingoCol + len(node.Name),
		Kind:    KindIdent,
		Object:  obj,
		Type:    obj.Type(),
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
	// Get selection information
	sel := b.typesInfo.Selections[node]
	if sel == nil {
		// Not a selection (maybe a qualified identifier)
		return nil
	}

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

	// Determine context
	context := b.inferContext(node, tv.Type)

	return &SemanticEntity{
		Line:    dingoLine,
		Col:     dingoCol,
		EndCol:  dingoCol + len(node.Sel.Name),
		Kind:    KindField,
		Type:    tv.Type,
		Context: context,
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
			// Error propagation: try to find the Result type
			if op.ExprType != nil {
				unwrapped, original, ok := b.extractResultType(op.ExprType)
				if ok {
					context = &DingoContext{
						Kind:          ContextErrorProp,
						OriginalType:  original,
						UnwrappedType: unwrapped,
						Description:   "Error propagation operator",
					}
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

// goPosToDingoPos maps a Go AST position back to Dingo source position
// FOLLOWS CLAUDE.MD: Uses token.FileSet and lineMappings, NOT byte arithmetic
func (b *Builder) goPosToDingoPos(goPos token.Pos) (line, col int, ok bool) {
	// Get Go position from FileSet
	goPosition := b.goFset.Position(goPos)
	goLine := goPosition.Line
	goCol := goPosition.Column

	// Find the line mapping that covers this Go line
	for _, mapping := range b.lineMappings {
		if goLine >= mapping.GoLineStart && goLine <= mapping.GoLineEnd {
			// This Go line maps to a Dingo line
			dingoLine := mapping.DingoLine

			// Try to find precise column mapping
			dingoCol, _ := b.translateGoColumn(goLine, goCol)

			return dingoLine, dingoCol, true
		}
	}

	// No mapping found - this Go code might be generated boilerplate
	return 0, 0, false
}

// verifyIdentInSource checks if the identifier name exists at the given
// position in the Dingo source. This filters out generated code that maps
// to Dingo positions where different text exists.
//
// For example, "tmp" generated by error propagation at Dingo line 55, col 1
// would fail verification if the Dingo source has "userID" at that position.
func (b *Builder) verifyIdentInSource(name string, line, col int) bool {
	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		// No source to verify against - accept everything
		return true
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
		return false
	}

	// Find line end
	lineEnd := lineStart
	for lineEnd < len(b.dingoSource) && b.dingoSource[lineEnd] != '\n' {
		lineEnd++
	}

	// Extract the line content
	lineContent := b.dingoSource[lineStart:lineEnd]

	// Check if the identifier exists at the expected column (1-indexed)
	// col is 1-indexed, so we need col-1 for 0-indexed array access
	startIdx := col - 1
	endIdx := startIdx + len(name)

	if startIdx < 0 || endIdx > len(lineContent) {
		return false
	}

	// Compare the text at that position
	return string(lineContent[startIdx:endIdx]) == name
}

// translateGoColumn translates a Go column to Dingo column using column mappings.
// Returns the translated column and whether a mapping was found.
// This enables accurate hover when multiple Go entities map to the same Dingo line.
func (b *Builder) translateGoColumn(goLine, goCol int) (dingoCol int, found bool) {
	// Look for a column mapping that contains this position
	for _, m := range b.columnMappings {
		if m.GoLine == goLine {
			// Check if column falls within the mapped expression range
			// The mapping covers [GoCol, GoCol + Length)
			if goCol >= m.GoCol && goCol < m.GoCol+m.Length {
				// Translate using the column offset
				// offset = GoCol - DingoCol
				offset := m.GoCol - m.DingoCol
				return goCol - offset, true
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
