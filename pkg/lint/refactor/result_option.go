package refactor

import (
	"go/ast"
	goparser "go/parser"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// ResultTypeDetector detects functions returning (T, error) and suggests Result[T, E].
// This is an annotation-only refactoring (no auto-fix) since changing return types
// is a breaking API change that requires updating all callers.
//
// Example:
//
//	func loadUser(id int) (*User, error) { ... }
//
// Suggests:
//
//	func loadUser(id int) Result[*User, error] { ... }
//
// Code: R005
type ResultTypeDetector struct{}

func (d *ResultTypeDetector) Code() string { return "R005" }
func (d *ResultTypeDetector) Name() string { return "prefer-result-type" }
func (d *ResultTypeDetector) Doc() string {
	return "Suggests using Result[T, E] instead of (T, error) return type"
}

func (d *ResultTypeDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	// Use Go's standard parser to get full AST with function bodies
	goFile, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk the AST looking for function declarations
	ast.Inspect(goFile, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if function has a return type
		if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
			return true
		}

		// Check for (T, error) pattern
		results := funcDecl.Type.Results.List
		if len(results) != 2 {
			return true
		}

		// Second result must be 'error'
		secondType := results[1].Type
		if !isErrorType(secondType) {
			return true
		}

		// Verify the function body actually returns errors
		if !hasErrorReturns(funcDecl.Body) {
			return true
		}

		// Get the first return type as a string
		firstTypeStr := typeToString(results[0].Type)

		// Create diagnostic (annotation-only, no auto-fix)
		diagnostics = append(diagnostics, analyzer.Diagnostic{
			Pos:      fset.Position(funcDecl.Type.Pos()),
			End:      fset.Position(funcDecl.Type.End()),
			Message:  "Consider using Result[" + firstTypeStr + ", error] instead of (" + firstTypeStr + ", error)",
			Severity: analyzer.SeverityHint,
			Code:     "R005",
			Category: "refactor",
			// No Fixes - this is a breaking API change
		})

		return true
	})

	return diagnostics
}

// OptionTypeDetector detects functions returning *T with nil semantics and suggests Option[T].
// This is an annotation-only refactoring (no auto-fix) since changing return types
// is a breaking API change.
//
// Example:
//
//	func findUser(name string) *User {
//	    if found { return &user }
//	    return nil  // nil means "not found"
//	}
//
// Suggests:
//
//	func findUser(name string) Option[*User] { ... }
//
// Code: R006
type OptionTypeDetector struct{}

func (d *OptionTypeDetector) Code() string { return "R006" }
func (d *OptionTypeDetector) Name() string { return "prefer-option-type" }
func (d *OptionTypeDetector) Doc() string {
	return "Suggests using Option[T] instead of *T with nil semantics"
}

func (d *OptionTypeDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	// Use Go's standard parser to get full AST with function bodies
	goFile, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk the AST looking for function declarations
	ast.Inspect(goFile, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if function has a return type
		if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
			return true
		}

		// Check for single pointer return type
		results := funcDecl.Type.Results.List
		if len(results) != 1 {
			return true
		}

		// Must be a pointer type
		starExpr, ok := results[0].Type.(*ast.StarExpr)
		if !ok {
			return true
		}

		// Verify the function body has explicit nil returns
		if !hasNilReturns(funcDecl.Body) {
			return true
		}

		// Get the pointer type as a string
		ptrTypeStr := typeToString(starExpr)

		// Create diagnostic (annotation-only, no auto-fix)
		diagnostics = append(diagnostics, analyzer.Diagnostic{
			Pos:      fset.Position(funcDecl.Type.Pos()),
			End:      fset.Position(funcDecl.Type.End()),
			Message:  "Consider using Option[" + ptrTypeStr + "] instead of " + ptrTypeStr + " with nil semantics",
			Severity: analyzer.SeverityHint,
			Code:     "R006",
			Category: "refactor",
			// No Fixes - this is a breaking API change
		})

		return true
	})

	return diagnostics
}

// Helper: isErrorType checks if a type expression is 'error'
func isErrorType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

// Helper: hasErrorReturns checks if a function body contains return statements with error
func hasErrorReturns(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}

	hasError := false
	ast.Inspect(body, func(n ast.Node) bool {
		retStmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		// Look for return statements with 2 values where second might be error
		if len(retStmt.Results) == 2 {
			// Check if second result is an error identifier or error-related
			second := retStmt.Results[1]

			// Check for 'err' identifier
			if ident, ok := second.(*ast.Ident); ok {
				if ident.Name == "err" {
					hasError = true
					return false
				}
			}

			// Check for errors.New(), fmt.Errorf(), etc.
			if call, ok := second.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok {
						if (x.Name == "errors" && sel.Sel.Name == "New") ||
							(x.Name == "fmt" && sel.Sel.Name == "Errorf") {
							hasError = true
							return false
						}
					}
				}
			}
		}

		return true
	})

	return hasError
}

// Helper: hasNilReturns checks if a function body contains explicit nil returns
func hasNilReturns(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}

	hasNil := false
	ast.Inspect(body, func(n ast.Node) bool {
		retStmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		// Look for return nil
		for _, result := range retStmt.Results {
			if ident, ok := result.(*ast.Ident); ok && ident.Name == "nil" {
				hasNil = true
				return false
			}
		}

		return true
	})

	return hasNil
}

// Helper: typeToString converts an ast.Expr representing a type to a string
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeToString(t.Elt)
		}
		return "[...]" + typeToString(t.Elt)
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + typeToString(t.Value)
		case ast.RECV:
			return "<-chan " + typeToString(t.Value)
		default:
			return "chan " + typeToString(t.Value)
		}
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.FuncType:
		return "func(...)"
	default:
		return "T"
	}
}
