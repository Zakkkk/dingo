// Package transformer provides the framework for converting Dingo AST to Go AST
package transformer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// TransformContext holds the state and utilities for AST transformation
//
// The context tracks:
// - Current function signature (for error propagation return types)
// - Variable name generation (for temporary variables)
// - Scope tracking (for variable declarations)
// - Parent node tracking (for context-aware transformations)
// - Error collection (for transformation errors)
type TransformContext struct {
	// FileSet for position tracking and source maps
	FileSet *token.FileSet

	// Type information from go/types (optional, may be nil)
	TypeInfo *types.Info

	// Current function declaration (nil if outside function)
	// Updated as we traverse the AST, used for error propagation
	currentFunc *ast.FuncDecl

	// Parent map for upward traversal
	// Maps each node to its parent in the AST
	parentMap map[ast.Node]ast.Node

	// Variable name counters for generating unique names
	// Format: tmp, tmp1, tmp2, etc. (no-number-first pattern)
	tmpCounter int // For tmp variables
	errCounter int // For err variables

	// Scope stack for tracking variable declarations
	// Each scope contains the variables declared in that scope
	scopes []map[string]bool

	// Errors accumulated during transformation
	errors []error
}

// NewTransformContext creates a new transformation context
func NewTransformContext(fset *token.FileSet, typeInfo *types.Info) *TransformContext {
	return &TransformContext{
		FileSet:     fset,
		TypeInfo:    typeInfo,
		parentMap:   make(map[ast.Node]ast.Node),
		scopes:      []map[string]bool{make(map[string]bool)}, // Global scope
		tmpCounter:  0,
		errCounter:  0,
		currentFunc: nil,
		errors:      make([]error, 0),
	}
}

// FreshTempVar generates a fresh temporary variable name
// Uses the no-number-first pattern: tmp, tmp1, tmp2, tmp3, ...
func (ctx *TransformContext) FreshTempVar() string {
	if ctx.tmpCounter == 0 {
		ctx.tmpCounter = 1
		return "tmp"
	}
	name := fmt.Sprintf("tmp%d", ctx.tmpCounter)
	ctx.tmpCounter++
	return name
}

// FreshErrVar generates a fresh error variable name
// Uses the no-number-first pattern: err, err1, err2, err3, ...
func (ctx *TransformContext) FreshErrVar() string {
	if ctx.errCounter == 0 {
		ctx.errCounter = 1
		return "err"
	}
	name := fmt.Sprintf("err%d", ctx.errCounter)
	ctx.errCounter++
	return name
}

// FreshVar generates a fresh variable name with a custom prefix
// Uses the no-number-first pattern: prefix, prefix1, prefix2, ...
// This is a generic version for any prefix
func (ctx *TransformContext) FreshVar(prefix string, counter *int) string {
	if *counter == 0 {
		*counter = 1
		return prefix
	}
	name := fmt.Sprintf("%s%d", prefix, *counter)
	*counter++
	return name
}

// PushScope enters a new scope (function, block, etc.)
func (ctx *TransformContext) PushScope() {
	ctx.scopes = append(ctx.scopes, make(map[string]bool))
}

// PopScope exits the current scope
func (ctx *TransformContext) PopScope() {
	if len(ctx.scopes) > 1 {
		ctx.scopes = ctx.scopes[:len(ctx.scopes)-1]
	}
}

// DeclareVar declares a variable in the current scope
func (ctx *TransformContext) DeclareVar(name string) {
	if len(ctx.scopes) > 0 {
		ctx.scopes[len(ctx.scopes)-1][name] = true
	}
}

// IsDeclared checks if a variable is declared in any visible scope
func (ctx *TransformContext) IsDeclared(name string) bool {
	for i := len(ctx.scopes) - 1; i >= 0; i-- {
		if ctx.scopes[i][name] {
			return true
		}
	}
	return false
}

// SetCurrentFunc sets the current function declaration
// This is used for context-aware error propagation (return type inference)
func (ctx *TransformContext) SetCurrentFunc(fn *ast.FuncDecl) {
	ctx.currentFunc = fn
}

// GetCurrentFunc returns the current function declaration
func (ctx *TransformContext) GetCurrentFunc() *ast.FuncDecl {
	return ctx.currentFunc
}

// GetFuncReturnTypes returns the return types of the current function
// Returns nil if not inside a function or if function has no return types
func (ctx *TransformContext) GetFuncReturnTypes() *ast.FieldList {
	if ctx.currentFunc == nil || ctx.currentFunc.Type == nil {
		return nil
	}
	return ctx.currentFunc.Type.Results
}

// BuildParentMap constructs the parent map for the given file
// This enables GetParent() and upward traversal
func (ctx *TransformContext) BuildParentMap(file *ast.File) {
	ctx.parentMap = make(map[ast.Node]ast.Node)

	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			// Pop from stack when exiting a node
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}

		// Set parent relationship (all nodes except root)
		if len(stack) > 0 {
			ctx.parentMap[n] = stack[len(stack)-1]
		}

		// Push current node to stack
		stack = append(stack, n)
		return true
	})
}

// GetParent returns the parent node of the given node
// Returns nil if the node is the root or not in the parent map
func (ctx *TransformContext) GetParent(node ast.Node) ast.Node {
	return ctx.parentMap[node]
}

// WalkParents walks up the parent chain from the given node
// Calls visitor for each parent until visitor returns false or reaches root
// Returns true if reached root, false if visitor stopped early
func (ctx *TransformContext) WalkParents(node ast.Node, visitor func(ast.Node) bool) bool {
	current := node
	for {
		parent := ctx.parentMap[current]
		if parent == nil {
			return true // Reached root
		}
		if !visitor(parent) {
			return false // Visitor stopped
		}
		current = parent
	}
}

// ReportError adds a transformation error to the context
func (ctx *TransformContext) ReportError(err error) {
	if err != nil {
		ctx.errors = append(ctx.errors, err)
	}
}

// GetErrors returns all transformation errors
func (ctx *TransformContext) GetErrors() []error {
	return ctx.errors
}

// HasErrors returns true if any errors were reported
func (ctx *TransformContext) HasErrors() bool {
	return len(ctx.errors) > 0
}

// ClearErrors clears all accumulated errors
func (ctx *TransformContext) ClearErrors() {
	ctx.errors = make([]error, 0)
}
