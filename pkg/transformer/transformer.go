// Package transformer provides the framework for converting Dingo AST to Go AST
package transformer

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"go/types"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"golang.org/x/tools/go/ast/astutil"
)

// Transformer converts Dingo AST to Go AST
//
// The transformer uses the visitor pattern to traverse Dingo AST nodes
// and transform them into equivalent Go AST nodes. It supports:
// - Standard Go nodes (pass-through unchanged)
// - Dingo-specific extensions (ErrorPropExpr, NullCoalesceExpr, SafeNavExpr, etc.)
// - Context-aware transformations (error propagation with return type inference)
// - Position preservation for accurate source maps
type Transformer struct {
	ctx *TransformContext

	// Registry of node-specific transformers
	// Maps node type to transformer function
	nodeTransformers map[string]NodeTransformer
}

// NodeTransformer is a function that transforms a specific Dingo node to Go AST
// It receives the node and context, returns transformed Go node(s)
// May return multiple statements (e.g., error propagation expands to if block)
type NodeTransformer func(node goast.Node, ctx *TransformContext) (goast.Node, error)

// New creates a new transformer with the given file set and type info
func New(fset *token.FileSet, typeInfo *types.Info) *Transformer {
	ctx := NewTransformContext(fset, typeInfo)

	t := &Transformer{
		ctx:              ctx,
		nodeTransformers: make(map[string]NodeTransformer),
	}

	// Register built-in node transformers
	t.registerBuiltinTransformers()

	return t
}

// registerBuiltinTransformers registers the built-in Dingo node transformers
func (t *Transformer) registerBuiltinTransformers() {
	// Error propagation: expr? → if err != nil { return ..., err }
	t.RegisterNodeTransformer("ErrorPropExpr", t.transformErrorProp)

	// Null coalescing: a ?? b → (fallback pattern)
	t.RegisterNodeTransformer("NullCoalesceExpr", t.transformNullCoalesce)

	// Safe navigation: expr?.field → (safe access pattern)
	t.RegisterNodeTransformer("SafeNavExpr", t.transformSafeNav)

	// Safe navigation call: expr?.method() → (safe call pattern)
	t.RegisterNodeTransformer("SafeNavCallExpr", t.transformSafeNavCall)

	// Lambda expressions: (x) => x + 1 → func(x int) int { return x + 1 }
	t.RegisterNodeTransformer("LambdaExpr", t.transformLambda)

	// Enum declarations: enum Result { Ok(T), Err(E) } → tagged union
	t.RegisterEnumTransformer()

	// Match expressions: match x { Ok(v) => v, Err(e) => 0 } → switch x.tag { ... }
	t.RegisterMatchTransformer()
}

// RegisterNodeTransformer registers a transformer for a specific node type
func (t *Transformer) RegisterNodeTransformer(nodeType string, transformer NodeTransformer) {
	t.nodeTransformers[nodeType] = transformer
}

// Transform transforms a Dingo AST file to a Go AST file
//
// This is the main entry point for the transformation process.
// It:
// 1. Builds the parent map for context-aware transformations
// 2. Traverses the AST using the visitor pattern
// 3. Transforms Dingo-specific nodes to Go nodes
// 4. Preserves positions for source map generation
// 5. Collects any transformation errors
func (t *Transformer) Transform(file *dingoast.File) (*goast.File, error) {
	if file == nil || file.File == nil {
		return nil, fmt.Errorf("nil file provided to transformer")
	}

	// Build parent map for context-aware transformations
	t.ctx.BuildParentMap(file.File)

	// Transform the AST using astutil.Apply
	// This walks the tree and allows us to replace nodes
	transformed := astutil.Apply(file.File,
		// Pre-visit: track context (scopes, current function, etc.)
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()
			if n == nil {
				return true
			}

			// Track current function for error propagation
			if funcDecl, ok := n.(*goast.FuncDecl); ok {
				t.ctx.SetCurrentFunc(funcDecl)
			}

			// Track scopes (blocks create new scopes)
			if _, ok := n.(*goast.BlockStmt); ok {
				t.ctx.PushScope()
			}

			return true
		},

		// Post-visit: transform nodes and clean up context
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()
			if n == nil {
				return true
			}

			// Transform the node if it's a Dingo extension
			if transformed, err := t.transformNode(n); err != nil {
				t.ctx.ReportError(err)
			} else if transformed != nil && transformed != n {
				// Replace the node with the transformed version
				cursor.Replace(transformed)
			}

			// Pop scope when exiting block
			if _, ok := n.(*goast.BlockStmt); ok {
				t.ctx.PopScope()
			}

			// Clear current function when exiting
			if _, ok := n.(*goast.FuncDecl); ok {
				t.ctx.SetCurrentFunc(nil)
			}

			return true
		},
	)

	// Check for transformation errors
	if t.ctx.HasErrors() {
		return nil, fmt.Errorf("transformation failed: %v", t.ctx.GetErrors())
	}

	// Return the transformed file
	if file, ok := transformed.(*goast.File); ok {
		return file, nil
	}

	return nil, fmt.Errorf("transformation did not produce a valid file")
}

// transformNode transforms a single AST node
// Returns the transformed node (may be the same node if no transformation needed)
// Returns error if transformation fails
func (t *Transformer) transformNode(node goast.Node) (goast.Node, error) {
	if node == nil {
		return nil, nil
	}

	// Check if this is a Dingo-specific node
	// We use type assertions to identify Dingo nodes
	switch n := node.(type) {
	case *dingoast.ErrorPropExpr:
		if transformer, ok := t.nodeTransformers["ErrorPropExpr"]; ok {
			return transformer(n, t.ctx)
		}
	case *dingoast.NullCoalesceExpr:
		if transformer, ok := t.nodeTransformers["NullCoalesceExpr"]; ok {
			return transformer(n, t.ctx)
		}
	case *dingoast.SafeNavExpr:
		if transformer, ok := t.nodeTransformers["SafeNavExpr"]; ok {
			return transformer(n, t.ctx)
		}
	case *dingoast.SafeNavCallExpr:
		if transformer, ok := t.nodeTransformers["SafeNavCallExpr"]; ok {
			return transformer(n, t.ctx)
		}
	case *dingoast.LambdaExpr:
		if transformer, ok := t.nodeTransformers["LambdaExpr"]; ok {
			return transformer(n, t.ctx)
		}
	case *dingoast.EnumDecl:
		if transformer, ok := t.nodeTransformers["EnumDecl"]; ok {
			return transformer(n, t.ctx)
		}
	case *dingoast.MatchExpr:
		if transformer, ok := t.nodeTransformers["MatchExpr"]; ok {
			return transformer(n, t.ctx)
		}
	}

	// Not a Dingo node or no transformer registered - pass through unchanged
	return node, nil
}

// transformExpr transforms an expression node
// This is a helper for expression-level transformations
func (t *Transformer) transformExpr(expr goast.Expr) (goast.Expr, error) {
	transformed, err := t.transformNode(expr)
	if err != nil {
		return nil, err
	}
	if transformed == nil {
		return expr, nil
	}
	if e, ok := transformed.(goast.Expr); ok {
		return e, nil
	}
	return nil, fmt.Errorf("transformed node is not an expression: %T", transformed)
}

// transformStmt transforms a statement node
// May return multiple statements (e.g., error propagation expands to multiple statements)
func (t *Transformer) transformStmt(stmt goast.Stmt) ([]goast.Stmt, error) {
	transformed, err := t.transformNode(stmt)
	if err != nil {
		return nil, err
	}
	if transformed == nil {
		return []goast.Stmt{stmt}, nil
	}

	// Check if transformed into a statement
	if s, ok := transformed.(goast.Stmt); ok {
		return []goast.Stmt{s}, nil
	}

	return nil, fmt.Errorf("transformed node is not a statement: %T", transformed)
}

// transformDecl transforms a declaration node
func (t *Transformer) transformDecl(decl goast.Decl) (goast.Decl, error) {
	transformed, err := t.transformNode(decl)
	if err != nil {
		return nil, err
	}
	if transformed == nil {
		return decl, nil
	}
	if d, ok := transformed.(goast.Decl); ok {
		return d, nil
	}
	return nil, fmt.Errorf("transformed node is not a declaration: %T", transformed)
}

// Built-in transformers for Dingo nodes
// These are placeholder implementations - actual logic will be added in separate files

func (t *Transformer) transformErrorProp(node goast.Node, ctx *TransformContext) (goast.Node, error) {
	// TODO: Implement error propagation transformation
	// This will generate: tmp, err := <operand>; if err != nil { return ..., err }
	// For now, just pass through
	return node, nil
}

func (t *Transformer) transformNullCoalesce(node goast.Node, ctx *TransformContext) (goast.Node, error) {
	// TODO: Implement null coalescing transformation
	// This will generate: (func() T { if left.IsNone() { return right }; return left.Unwrap() })()
	// For now, just pass through
	return node, nil
}

func (t *Transformer) transformSafeNav(node goast.Node, ctx *TransformContext) (goast.Node, error) {
	// TODO: Implement safe navigation transformation
	// This will generate: (func() Option[T] { if x.IsNone() { return None() }; return Some(x.Unwrap().field) })()
	// For now, just pass through
	return node, nil
}

func (t *Transformer) transformSafeNavCall(node goast.Node, ctx *TransformContext) (goast.Node, error) {
	// TODO: Implement safe navigation call transformation
	// Similar to safe nav but for method calls
	// For now, just pass through
	return node, nil
}

// GetContext returns the transformation context
// This is useful for testing and debugging
func (t *Transformer) GetContext() *TransformContext {
	return t.ctx
}
