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
	goAST        *ast.File
	goFset       *token.FileSet
	typesInfo    *types.Info
	lineMappings []sourcemap.LineMapping
	dingoSource  []byte
	dingoFset    *token.FileSet
	dingoFile    string
}

// NewBuilder creates a Builder
// IMPORTANT: Follows CLAUDE.md rules - uses token.FileSet for all position tracking
func NewBuilder(
	goAST *ast.File,
	goFset *token.FileSet,
	typesInfo *types.Info,
	lineMappings []sourcemap.LineMapping,
	dingoSource []byte,
	dingoFset *token.FileSet,
	dingoFile string,
) *Builder {
	return &Builder{
		goAST:        goAST,
		goFset:       goFset,
		typesInfo:    typesInfo,
		lineMappings: lineMappings,
		dingoSource:  dingoSource,
		dingoFset:    dingoFset,
		dingoFile:    dingoFile,
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
func (b *Builder) handleCallExpr(node *ast.CallExpr) *SemanticEntity {
	// Get the type of the call expression (return type)
	tv, ok := b.typesInfo.Types[node]
	if !ok {
		return nil
	}

	// Map Go position to Dingo position
	goPos := node.Pos()
	dingoLine, dingoCol, ok := b.goPosToDingoPos(goPos)
	if !ok {
		return nil
	}

	// Calculate end column (roughly - covers function name)
	endCol := dingoCol + 5 // Conservative estimate

	// Determine context
	context := b.inferContext(node, tv.Type)

	return &SemanticEntity{
		Line:    dingoLine,
		Col:     dingoCol,
		EndCol:  endCol,
		Kind:    KindCall,
		Type:    tv.Type,
		Context: context,
	}
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
			// For now, we map all Go lines in the range to the same Dingo line
			// Column mapping is approximate - we can't recover exact Dingo column
			// from transformed Go code
			return mapping.DingoLine, goCol, true
		}
	}

	// No mapping found - this Go code might be generated boilerplate
	return 0, 0, false
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
