// Package transpiler provides return type analysis for implicit Result/Option wrapping.
// It uses go/types to accurately determine what type a function returns and what
// type a return expression has, enabling automatic wrapping with Ok/Err/Some/None.
package transpiler

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// ReturnAnalyzer analyzes function return types and return expressions
// to determine if implicit wrapping is needed for Result/Option types.
type ReturnAnalyzer struct {
	checker *typechecker.Checker
}

// NewReturnAnalyzer creates a new return analyzer with type checking support.
func NewReturnAnalyzer(checker *typechecker.Checker) *ReturnAnalyzer {
	return &ReturnAnalyzer{checker: checker}
}

// ReturnTypeInfo contains information about a function's return type.
type ReturnTypeInfo struct {
	// Kind is either "result", "option", or "other"
	Kind string

	// For Result[T, E]:
	TType types.Type // The success type T
	EType types.Type // The error type E

	// For Option[T]:
	// TType is the wrapped type T
	// EType is nil

	// AST expressions for cloning
	TAstExpr ast.Expr // AST expression for T
	EAstExpr ast.Expr // AST expression for E (nil for Option)
}

// isResultOrOptionType checks if an AST expression represents Result or Option type.
// Handles both unqualified (Result, Option) and qualified (dgo.Result, pkg.Option) forms.
func isResultOrOptionType(expr ast.Expr) (bool, string) {
	switch e := expr.(type) {
	case *ast.Ident:
		// Unqualified: Result, Option
		if e.Name == "Result" || e.Name == "Option" {
			return true, e.Name
		}
	case *ast.SelectorExpr:
		// Qualified: dgo.Result, pkg.Option
		if e.Sel.Name == "Result" || e.Sel.Name == "Option" {
			return true, e.Sel.Name
		}
	}
	return false, ""
}

// AnalyzeReturnType analyzes a function declaration to determine if it returns
// Result[T,E] or Option[T], and extracts the type parameters.
//
// Supports both qualified (dgo.Result[T,E]) and unqualified (Result[T,E]) forms.
// Returns nil if the function doesn't return Result or Option.
func (ra *ReturnAnalyzer) AnalyzeReturnType(fn *ast.FuncDecl) *ReturnTypeInfo {
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return nil // Must have exactly one return value
	}

	resultType := fn.Type.Results.List[0].Type

	// Check for Result[T, E] (IndexListExpr - two type parameters)
	if indexList, ok := resultType.(*ast.IndexListExpr); ok {
		isROrO, typeName := isResultOrOptionType(indexList.X)
		if isROrO && typeName == "Result" && len(indexList.Indices) == 2 {
			info := &ReturnTypeInfo{
				Kind:     "result",
				TAstExpr: indexList.Indices[0],
				EAstExpr: indexList.Indices[1],
			}

			// Get go/types information if checker available
			if ra.checker != nil {
				info.TType = ra.checker.TypeOf(indexList.Indices[0])
				info.EType = ra.checker.TypeOf(indexList.Indices[1])
			}

			return info
		}
	}

	// Check for Option[T] (IndexExpr - one type parameter)
	if indexExpr, ok := resultType.(*ast.IndexExpr); ok {
		isROrO, typeName := isResultOrOptionType(indexExpr.X)
		if isROrO && typeName == "Option" {
			info := &ReturnTypeInfo{
				Kind:     "option",
				TAstExpr: indexExpr.Index,
				EAstExpr: nil,
			}

			if ra.checker != nil {
				info.TType = ra.checker.TypeOf(indexExpr.Index)
			}

			return info
		}
	}

	return nil // Not Result or Option
}

// WrapperType determines what wrapper to use for a return expression.
// For Result[T,E]: returns "Ok" if expr is type T, "Err" if expr is type E
// For Option[T]: returns "Some" if expr is non-nil, "None" if expr is nil
// Returns empty string if no wrapping needed (already wrapped).
type WrapperType string

const (
	WrapperOk   WrapperType = "Ok"
	WrapperErr  WrapperType = "Err"
	WrapperSome WrapperType = "Some"
	WrapperNone WrapperType = "None"
	WrapperSkip WrapperType = "" // Already wrapped, skip
)

// DetermineWrapper analyzes a return expression and determines what wrapper
// constructor should be used (Ok, Err, Some, None, or none if already wrapped).
//
// For Result[T,E]:
//   - Returns WrapperOk if expr type matches T
//   - Returns WrapperErr if expr type matches E
//
// For Option[T]:
//   - Returns WrapperNone if expr is nil literal
//   - Returns WrapperSome otherwise
//
// Returns WrapperSkip if expression is already wrapped with dgo.Ok/Err/Some/None.
func (ra *ReturnAnalyzer) DetermineWrapper(expr ast.Expr, returnInfo *ReturnTypeInfo) WrapperType {
	// Check if already wrapped
	if ra.isAlreadyWrapped(expr) {
		return WrapperSkip
	}

	switch returnInfo.Kind {
	case "result":
		return ra.determineResultWrapper(expr, returnInfo)
	case "option":
		return ra.determineOptionWrapper(expr, returnInfo)
	default:
		return WrapperSkip
	}
}

// determineResultWrapper determines Ok vs Err for Result[T,E] returns.
// Uses multiple strategies with fallbacks when type checker is unavailable.
func (ra *ReturnAnalyzer) determineResultWrapper(expr ast.Expr, returnInfo *ReturnTypeInfo) WrapperType {
	// Strategy 1: AST-based error detection (works without type checker)
	// This runs first to catch common error patterns even when type checker fails
	if ra.isErrorExpression(expr) {
		return WrapperErr
	}

	// Strategy 2: Use go/types for accurate type comparison
	if ra.checker != nil && returnInfo.EType != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil {
			// Check if expression type is assignable to E (error type)
			if types.AssignableTo(exprType, returnInfo.EType) {
				return WrapperErr
			}
			// Check if expression type is assignable to T (success type)
			if returnInfo.TType != nil && types.AssignableTo(exprType, returnInfo.TType) {
				return WrapperOk
			}
		}
	}

	// Strategy 3: Heuristic - check for composite literal matching E type
	if compLit, ok := expr.(*ast.CompositeLit); ok {
		if eIdent, ok := returnInfo.EAstExpr.(*ast.Ident); ok {
			if litIdent, ok := compLit.Type.(*ast.Ident); ok {
				if litIdent.Name == eIdent.Name {
					return WrapperErr
				}
			}
		}
	}

	// Strategy 4: Check for error interface (requires type checker)
	if ra.checker != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil && ra.implementsError(exprType) {
			return WrapperErr
		}
	}

	// Default: assume success type (Ok)
	return WrapperOk
}

// determineOptionWrapper determines Some vs None for Option[T] returns.
func (ra *ReturnAnalyzer) determineOptionWrapper(expr ast.Expr, returnInfo *ReturnTypeInfo) WrapperType {
	// Check for nil literal
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Name == "nil" {
			return WrapperNone
		}
	}

	// Check if expression is already an Option[T] type (e.g., returning a variable of type Option[T])
	// Strategy 1: Use type checker if available
	if ra.checker != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil {
			// Check if the expression type is Option (named type check)
			if named, ok := exprType.(*types.Named); ok {
				typeName := named.Obj().Name()
				if typeName == "Option" {
					return WrapperSkip // Already Option[T], no wrapping needed
				}
			}
		}
	}

	// Strategy 2: AST-based check for known Option[T] variable names
	// Check if expr is an identifier that was declared with Option[T] type
	if ident, ok := expr.(*ast.Ident); ok {
		// This is a simple heuristic - if the variable was declared as Option[T],
		// the type checker would catch it above. Here we do a simpler check.
		if ident.Obj != nil {
			if decl, ok := ident.Obj.Decl.(*ast.ValueSpec); ok {
				if decl.Type != nil {
					// Check if the declared type is Option[T]
					if isOptionType(decl.Type) {
						return WrapperSkip
					}
				}
			}
		}
	}

	// All non-nil, non-Option values wrapped with Some
	return WrapperSome
}

// isOptionType checks if a type expression is Option[T] or dgo.Option[T]
func isOptionType(expr ast.Expr) bool {
	indexExpr, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	switch x := indexExpr.X.(type) {
	case *ast.Ident:
		return x.Name == "Option"
	case *ast.SelectorExpr:
		if ident, ok := x.X.(*ast.Ident); ok {
			return ident.Name == "dgo" && x.Sel.Name == "Option"
		}
	}
	return false
}

// isAlreadyWrapped checks if an expression is already a dgo.Ok/Err/Some/None call.
func (ra *ReturnAnalyzer) isAlreadyWrapped(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check for dgo.Ok, dgo.Err, dgo.Some, dgo.None (non-generic form)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "dgo" {
			name := sel.Sel.Name
			return name == "Ok" || name == "Err" || name == "Some" || name == "None"
		}
	}

	// Check for generic forms: dgo.Ok[T,E], dgo.Err[T,E], dgo.Some[T], dgo.None[T]
	var selectorExpr *ast.SelectorExpr

	// Handle IndexListExpr (two type params: Result)
	if indexList, ok := call.Fun.(*ast.IndexListExpr); ok {
		selectorExpr, _ = indexList.X.(*ast.SelectorExpr)
	}

	// Handle IndexExpr (one type param: Option)
	if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
		selectorExpr, _ = indexExpr.X.(*ast.SelectorExpr)
	}

	if selectorExpr != nil {
		if ident, ok := selectorExpr.X.(*ast.Ident); ok && ident.Name == "dgo" {
			name := selectorExpr.Sel.Name
			return name == "Ok" || name == "Err" || name == "Some" || name == "None"
		}
	}

	return false
}

// isErrorExpression detects error expressions using AST patterns (no type checker needed).
// This provides fallback error detection when type inference is disabled.
//
// Detects:
//   - Error variable names: err, error, userErr, etc.
//   - Error constructor calls: fmt.Errorf, errors.New, errors.Wrap, etc.
//   - Address-of error expressions: &myError
func (ra *ReturnAnalyzer) isErrorExpression(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		// Check for error-like variable names
		name := e.Name
		if name == "err" || name == "error" {
			return true
		}
		// Check for variables with "err" or "error" in the name
		// (e.g., userErr, validationError, dbError)
		lower := name
		if len(lower) > 0 {
			firstLower := lower[0] >= 'a' && lower[0] <= 'z'
			if firstLower && (containsSubstring(lower, "err") || containsSubstring(lower, "error")) {
				return true
			}
		}
	case *ast.CallExpr:
		// Check for error constructor functions
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			funcName := sel.Sel.Name
			// Common error constructors: fmt.Errorf, errors.New, errors.Wrap, etc.
			switch funcName {
			case "Errorf", "New", "Wrap", "Wrapf", "WithMessage", "WithStack":
				return true
			}
		}
	case *ast.UnaryExpr:
		// Address-of error expression: &myError
		if e.Op == token.AND {
			return ra.isErrorExpression(e.X)
		}
	}
	return false
}

// containsSubstring checks if s contains substr (case-insensitive for ASCII).
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1, c2 := s[i+j], substr[j]
			// Simple ASCII lowercase
			if c1 >= 'A' && c1 <= 'Z' {
				c1 = c1 + ('a' - 'A')
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 = c2 + ('a' - 'A')
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// implementsError checks if a type implements the error interface.
func (ra *ReturnAnalyzer) implementsError(t types.Type) bool {
	// Get the error interface type from the universe scope
	// The error interface has a single method: Error() string
	errorType := types.Universe.Lookup("error")
	if errorType == nil {
		return false
	}

	errorInterface, ok := errorType.Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}

	// Check if t implements error interface
	return types.Implements(t, errorInterface)
}

// GetTypeString returns a string representation of a type for debugging.
func (ra *ReturnAnalyzer) GetTypeString(t types.Type) string {
	if t == nil {
		return "<nil>"
	}
	return types.TypeString(t, nil)
}
