// Package builtin provides Result<T, E> type generation plugin
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// ResultTypePlugin generates Result<T, E> type declarations and transformations
//
// This plugin implements the Result type as a tagged union (sum type) with two variants:
// - Ok(T): Success case containing a value of type T
// - Err(E): Error case containing an error of type E
//
// Generated structure:
//
//	type Result_T_E struct {
//	    tag    ResultTag
//	    ok     *T        // Pointer for zero-value safety
//	    err    *E        // Pointer for nil-ability
//	}
//
// The plugin also generates:
// - ResultTag enum (Ok, Err)
// - Constructor functions (Result_T_E_Ok, Result_T_E_Err)
// - Helper methods (IsOk, IsErr, Unwrap, UnwrapOr, etc.)
type ResultTypePlugin struct {
	ctx *plugin.Context

	// Reference to the current file being transformed (for comment stripping)
	file *ast.File

	// Track which Result types we've already emitted to avoid duplicates
	emittedTypes map[string]bool

	// Declarations to inject at package level
	pendingDecls []ast.Decl

	// Type inference service for accurate type resolution (Fix A5)
	typeInference *TypeInferenceService

	// Track generic type references (Result[T, E]) that need to be rewritten to concrete types (ResultTE)
	genericTypeRewrites map[*ast.IndexExpr]string
	genericListRewrites map[*ast.IndexListExpr]string

	// Track function return types for implicit Result wrapping
	// Maps FuncDecl/FuncLit to their parsed Result return type info
	funcResultTypes map[ast.Node]*resultReturnInfo

	// Track if we need the dgo import (for Result/Option types)
	needsDgoImport bool

	// Track Result[T,E] nodes that need dgo. prefix (AST-based rewriting)
	resultTypeRewrites map[*ast.IndexListExpr]bool
}

// resultReturnInfo holds parsed Result<T, E> return type information
type resultReturnInfo struct {
	okType         string // The T in Result<T, E>
	errType        string // The E in Result<T, E>
	resultTypeName string // The sanitized type name (e.g., "ResultUserDBError")
}

// dgoImportPath is the import path for the Dingo runtime package
const dgoImportPath = "github.com/MadAppGang/dingo/pkg/dgo"

// NewResultTypePlugin creates a new Result type plugin
func NewResultTypePlugin() *ResultTypePlugin {
	return &ResultTypePlugin{
		emittedTypes:       make(map[string]bool),
		pendingDecls:       make([]ast.Decl, 0),
		funcResultTypes:    make(map[ast.Node]*resultReturnInfo),
		resultTypeRewrites: make(map[*ast.IndexListExpr]bool),
	}
}

// Name returns the plugin name
func (p *ResultTypePlugin) Name() string {
	return "result_type"
}

// GetRequiredImports implements plugin.ImportProvider
// Returns the dgo import path if Result types were detected
func (p *ResultTypePlugin) GetRequiredImports() []string {
	if p.needsDgoImport {
		return []string{dgoImportPath}
	}
	return nil
}

// SetContext sets the plugin context (ContextAware interface)
func (p *ResultTypePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	// Initialize type inference service with go/types integration (Fix A5)
	if ctx != nil && ctx.FileSet != nil {
		// Create type inference service
		service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
		if err != nil {
			ctx.Logger.Warnf("Failed to create type inference service: %v", err)
		} else {
			p.typeInference = service

			// Inject go/types.Info if available in context
			if ctx.TypeInfo != nil {
				if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
					service.SetTypesInfo(typesInfo)
					ctx.Logger.Debugf("Result plugin: go/types integration enabled (Fix A5)")
				}
			}
		}
	}
}

// Process processes AST nodes to find and transform Result types
func (p *ResultTypePlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Walk the AST to find Result type usage and track function return types
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			// Track function return type if it's a Result type
			p.trackFunctionResultType(n, n.Type)
		case *ast.FuncLit:
			// Track function literal return type if it's a Result type
			p.trackFunctionResultType(n, n.Type)
		case *ast.IndexExpr:
			// Result<T> or Result<T, E>
			p.handleGenericResult(n)
		case *ast.IndexListExpr:
			// Go 1.18+ generic syntax: Result[T, E]
			p.handleGenericResultList(n)
		case *ast.CallExpr:
			// Ok(value) or Err(error) constructor calls
			p.handleConstructorCall(n)
		}
		return true
	})

	return nil
}

// stripCommentsNearExpr is a helper that safely strips comments near an expression
// before wrapping it in a Result constructor. This prevents comment misplacement.
func (p *ResultTypePlugin) stripCommentsNearExpr(expr ast.Expr, stmt ast.Node) {
	if p.ctx == nil || p.ctx.CurrentFile == nil {
		return
	}

	// Comment stripping removed - handled by go/printer
	// The go/printer package automatically handles comment positioning
}

// findParentStmt is no longer needed - comment stripping handled by go/printer

// wrapReturnForResult checks if a return expression needs implicit wrapping
// and returns the wrapped expression, or nil if no wrapping needed.
//
// The logic:
// 1. If return value is already a Result constructor call (Ok, Err, dgo.Ok, dgo.Err) → no wrapping
// 2. If return value is already a Result type value → no wrapping
// 3. If return value type matches the error type E → wrap in dgo.Err[T, E]
// 4. If return value type matches the ok type T → wrap in dgo.Ok[T, E]
// 5. If we can't determine the type, assume Ok wrapping (compiler will catch mismatches)
//
// IMPORTANT: This method strips comments near the expression before wrapping to prevent
// comment misplacement inside generic type parameters (AST-level fix for comment positioning).
func (p *ResultTypePlugin) wrapReturnForResult(expr ast.Expr, info *resultReturnInfo, stmt *ast.ReturnStmt) ast.Expr {
	// Skip if already a Result constructor call
	if p.isResultConstructorCall(expr) {
		return nil
	}

	// Skip if it's already a Result type (e.g., variable of Result type)
	if p.isResultTypeValue(expr, info.resultTypeName) {
		return nil
	}

	// Comment stripping removed - handled by go/printer

	// Try to determine the type of the expression
	exprType := p.determineExpressionType(expr)

	// Check if the expression type matches the error type
	if p.typeMatchesError(expr, exprType, info.errType) {
		// Wrap in dgo.Err[T](value) - T explicit, E inferred from argument
		p.ctx.Logger.Debugf("Implicit wrapping: return %s → dgo.Err[%s](...)", FormatExprForDebug(expr), info.okType)
		return &ast.CallExpr{
			Fun: &ast.IndexExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("dgo"),
					Sel: ast.NewIdent("Err"),
				},
				Lbrack: expr.Pos(),
				Index:  ast.NewIdent(info.okType),
				Rbrack: expr.Pos(),
			},
			Lparen: expr.Pos(),
			Args:   []ast.Expr{expr},
			Rparen: expr.End(),
		}
	}

	// Check if the expression type matches the ok type (or is unknown)
	// If we can determine Ok match OR we can't determine at all, wrap as Ok
	// The Go compiler will catch any type mismatches
	if p.typeMatchesOk(expr, exprType, info.okType) || exprType == "" {
		// Wrap in dgo.Ok[T, E](value) - both type args required (E is second, can't skip T)
		p.ctx.Logger.Debugf("Implicit wrapping: return %s → dgo.Ok[%s, %s](...)", FormatExprForDebug(expr), info.okType, info.errType)
		return &ast.CallExpr{
			Fun: &ast.IndexListExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("dgo"),
					Sel: ast.NewIdent("Ok"),
				},
				Lbrack: expr.Pos(),
				Indices: []ast.Expr{
					ast.NewIdent(info.okType),
					ast.NewIdent(info.errType),
				},
				Rbrack: expr.Pos(),
			},
			Lparen: expr.Pos(),
			Args:   []ast.Expr{expr},
			Rparen: expr.End(),
		}
	}

	// Type is known but doesn't match either - don't wrap, let compiler catch
	return nil
}

// isResultConstructorCall checks if expr is a call to Ok(), Err(), dgo.Ok[], dgo.Err[], or ResultXOk/ResultXErr
func (p *ResultTypePlugin) isResultConstructorCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name := fun.Name
		// Check for Ok, Err, or ResultXOk/ResultXErr patterns
		if name == "Ok" || name == "Err" {
			return true
		}
		if strings.HasPrefix(name, "Result") && (strings.HasSuffix(name, "Ok") || strings.HasSuffix(name, "Err")) {
			return true
		}
	case *ast.IndexExpr:
		// Ok[Type](...) or Err[Type](...)
		if ident, ok := fun.X.(*ast.Ident); ok {
			if ident.Name == "Ok" || ident.Name == "Err" {
				return true
			}
		}
		// dgo.Ok[T](...) or dgo.Err[T](...)
		if sel, ok := fun.X.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "dgo" {
				if sel.Sel.Name == "Ok" || sel.Sel.Name == "Err" {
					return true
				}
			}
		}
	case *ast.IndexListExpr:
		// dgo.Ok[T, E](...) or dgo.Err[T, E](...)
		if sel, ok := fun.X.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "dgo" {
				if sel.Sel.Name == "Ok" || sel.Sel.Name == "Err" {
					return true
				}
			}
		}
		// Ok[T, E](...) or Err[T, E](...)
		if ident, ok := fun.X.(*ast.Ident); ok {
			if ident.Name == "Ok" || ident.Name == "Err" {
				return true
			}
		}
	case *ast.SelectorExpr:
		// dgo.Ok(...) or dgo.Err(...)
		if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "dgo" {
			if fun.Sel.Name == "Ok" || fun.Sel.Name == "Err" {
				return true
			}
		}
	}
	return false
}

// isResultType checks if an expression represents the Result type
// Handles both "Result" (Ident) and "dgo.Result" (SelectorExpr)
func (p *ResultTypePlugin) isResultType(expr ast.Expr) bool {
	// Case 1: Plain "Result" identifier
	if ident, ok := expr.(*ast.Ident); ok && ident.Name == "Result" {
		return true
	}
	// Case 2: "dgo.Result" selector expression
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "dgo" && sel.Sel.Name == "Result" {
			return true
		}
	}
	return false
}

// isResultTypeValue checks if expr is already a Result type value (variable, field, etc.)
func (p *ResultTypePlugin) isResultTypeValue(expr ast.Expr, resultTypeName string) bool {
	// Use type inference if available
	if p.typeInference != nil {
		if typ, ok := p.typeInference.InferType(expr); ok {
			typeName := p.typeInference.TypeToString(typ)
			if strings.HasPrefix(typeName, "Result") {
				return true
			}
		}
	}
	return false
}

// determineExpressionType tries to determine the type of an expression
func (p *ResultTypePlugin) determineExpressionType(expr ast.Expr) string {
	// Use type inference service if available
	if p.typeInference != nil {
		if typ, ok := p.typeInference.InferType(expr); ok {
			return p.typeInference.TypeToString(typ)
		}
	}

	// Fallback heuristics
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// Struct literal: DBError{...} → DBError
		if e.Type != nil {
			return p.getTypeName(e.Type)
		}
	case *ast.Ident:
		// Check for built-in constants
		switch e.Name {
		case "true", "false":
			return "bool"
		case "nil":
			return "nil"
		}
		// Check for common error variable naming conventions
		// Go convention: err, err1, err2, etc.
		name := e.Name
		if name == "err" || (strings.HasPrefix(name, "err") && len(name) > 3 && name[3] >= '0' && name[3] <= '9') {
			return "error"
		}
		// Variable - would need symbol table lookup
		// For now, we can't determine
		return ""
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "int"
		case token.FLOAT:
			return "float64"
		case token.STRING:
			return "string"
		}
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			// &value → pointer type
			innerType := p.determineExpressionType(e.X)
			if innerType != "" {
				return "*" + innerType
			}
		}
	case *ast.CallExpr:
		// Check for Result method calls that return known types
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name
			pkgName := ""
			if pkg, ok := sel.X.(*ast.Ident); ok {
				pkgName = pkg.Name
			}

			// UnwrapErr() returns the error type - mark as "error-returning"
			if methodName == "UnwrapErr" {
				return "__error_type__" // Special marker
			}
			// Unwrap() returns the ok type
			if methodName == "Unwrap" {
				return "__ok_type__" // Special marker
			}

			// Known error-returning functions
			if pkgName == "errors" && (methodName == "New" || methodName == "Wrap" || methodName == "Wrapf") {
				return "error"
			}
			if pkgName == "fmt" && methodName == "Errorf" {
				return "error"
			}
		}
	}
	return ""
}

// typeMatchesError checks if the expression type matches the error type E
func (p *ResultTypePlugin) typeMatchesError(expr ast.Expr, exprType, errType string) bool {
	// Check for special marker from UnwrapErr() calls
	if exprType == "__error_type__" {
		return true
	}

	// Direct type match
	if exprType != "" && exprType == errType {
		return true
	}

	// If expression is an error type, it should be wrapped with Err
	// regardless of the specific error type in the Result signature.
	// The Go compiler will catch any type mismatches.
	if exprType == "error" {
		return true
	}

	// Check for composite literal of error type
	if lit, ok := expr.(*ast.CompositeLit); ok {
		if lit.Type != nil {
			litType := p.getTypeName(lit.Type)
			if litType == errType {
				return true
			}
		}
	}

	// Check for error interface - any type implementing error
	if errType == "error" {
		// If the type has an Error() method, it implements error
		// For now, check common patterns
		if lit, ok := expr.(*ast.CompositeLit); ok {
			// Any struct literal could implement error
			// We'd need go/types to be sure, but for custom error types, assume yes
			if lit.Type != nil {
				return false // Can't assume struct implements error without checking
			}
		}
	}

	return false
}

// typeMatchesOk checks if the expression type matches the ok type T
func (p *ResultTypePlugin) typeMatchesOk(expr ast.Expr, exprType, okType string) bool {
	// Check for special marker from Unwrap() calls
	if exprType == "__ok_type__" {
		return true
	}

	// Direct type match
	if exprType != "" && exprType == okType {
		return true
	}

	// Check pointer types
	if strings.HasPrefix(okType, "*") && exprType == okType {
		return true
	}

	// Check for composite literal of ok type
	if lit, ok := expr.(*ast.CompositeLit); ok {
		if lit.Type != nil {
			litType := p.getTypeName(lit.Type)
			if litType == okType {
				return true
			}
		}
	}

	// Check for identifier (variable) - need type inference
	if ident, ok := expr.(*ast.Ident); ok {
		// Use type inference if available
		if p.typeInference != nil {
			if typ, ok := p.typeInference.InferType(ident); ok {
				typeName := p.typeInference.TypeToString(typ)
				if typeName == okType {
					return true
				}
			}
		}
	}

	return false
}

// trackFunctionResultType checks if a function returns a Result type and records the info
func (p *ResultTypePlugin) trackFunctionResultType(funcNode ast.Node, funcType *ast.FuncType) {
	if funcType == nil || funcType.Results == nil || len(funcType.Results.List) == 0 {
		return
	}

	// Check first return type for Result
	firstResult := funcType.Results.List[0]
	if firstResult.Type == nil {
		return
	}

	// Check for Result[T, E] (IndexListExpr) or Result[T] (IndexExpr)
	// Handle both "Result" (Ident) and "dgo.Result" (SelectorExpr)
	var okType, errType string

	switch rt := firstResult.Type.(type) {
	case *ast.IndexListExpr:
		// Result[T, E] with two type parameters
		if p.isResultType(rt.X) {
			if len(rt.Indices) >= 1 {
				okType = p.getTypeName(rt.Indices[0])
			}
			if len(rt.Indices) >= 2 {
				errType = p.getTypeName(rt.Indices[1])
			} else {
				errType = "error"
			}
		}
	case *ast.IndexExpr:
		// Result[T] with single type parameter (default error type)
		if p.isResultType(rt.X) {
			okType = p.getTypeName(rt.Index)
			errType = "error"
		}
	}

	// If we found a Result return type, record it
	if okType != "" {
		resultTypeName := fmt.Sprintf("Result%s", SanitizeTypeName(okType, errType))
		p.funcResultTypes[funcNode] = &resultReturnInfo{
			okType:         okType,
			errType:        errType,
			resultTypeName: resultTypeName,
		}
		p.ctx.Logger.Debugf("Implicit wrapping: tracked function returning %s (Ok=%s, Err=%s)", resultTypeName, okType, errType)
	}
}

// handleGenericResult processes Result<T> or Result<T, E> syntax (IndexExpr)
func (p *ResultTypePlugin) handleGenericResult(expr *ast.IndexExpr) {
	// With Go 1.18+ generics, we keep Result[T] as-is
	// The preprocessor already adds runtime. prefix (Result[T] → dgo.Result[T])
	// No type rewriting needed - the runtime package provides the generic type

	// Check if the base type is "Result" or "dgo.Result"
	isResult := false
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Result" {
		isResult = true
	}
	if sel, ok := expr.X.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "dgo" && sel.Sel.Name == "Result" {
			isResult = true
		}
	}

	if !isResult {
		return
	}

	// Log but don't rewrite - keep the generic syntax
	p.ctx.Logger.Debugf("Go 1.18+ generics: Found Result[T], keeping as-is (no rewrite)")

	// NOTE: With Go 1.18+ generics, we do NOT:
	// - Generate type declarations (runtime package provides Result[T, E])
	// - Rewrite Result[T, E] to ResultTE (keep generic syntax)
}

// handleGenericResultList processes Result[T, E] syntax (IndexListExpr for Go 1.18+)
// Tracks nodes that need dgo. prefix during Transform phase
func (p *ResultTypePlugin) handleGenericResultList(expr *ast.IndexListExpr) {
	// Check if the base type is "Result" (needs dgo. prefix) or already "dgo.Result"
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Result" {
		// Track this node for rewriting: Result[T, E] → dgo.Result[T, E]
		p.resultTypeRewrites[expr] = true
		p.needsDgoImport = true
		p.ctx.Logger.Debugf("Discovery: Found Result[T, E], will add dgo. prefix in Transform phase")
		return
	}

	// Already has dgo. prefix - no rewrite needed
	if sel, ok := expr.X.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "dgo" && sel.Sel.Name == "Result" {
			p.needsDgoImport = true
			p.ctx.Logger.Debugf("Discovery: Found dgo.Result[T, E], no rewrite needed")
			return
		}
	}

	// Not a Result type, ignore
}

// handleConstructorCall processes Ok(value) and Err(error) calls during discovery phase
//
// This is the discovery phase - just logs that we found constructor calls.
// Actual transformation happens in Transform() phase where we have function context.
func (p *ResultTypePlugin) handleConstructorCall(call *ast.CallExpr) {
	// Case 1: Ok(value) or Err(error) - plain identifier
	if ident, ok := call.Fun.(*ast.Ident); ok {
		switch ident.Name {
		case "Ok", "Err":
			p.ctx.Logger.Debugf("Discovery: Found %s() constructor call", ident.Name)
		}
		return
	}

	// Case 2: Ok[ErrType](value) or Err[OkType](error) - IndexExpr with type parameter
	if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
		if ident, ok := indexExpr.X.(*ast.Ident); ok {
			switch ident.Name {
			case "Ok", "Err":
				p.ctx.Logger.Debugf("Discovery: Found %s[T]() constructor call", ident.Name)
			}
		}
	}
}

// transformOkConstructor transforms Ok(value) → dgo.Ok(value) or dgo.Ok[T, E](value)
//
// When enclosing function's return type is known (resultInfo != nil), omits type
// arguments since Go can infer them from context. Otherwise uses explicit types.
//
// Returns the replacement node, or the original call if transformation fails
func (p *ResultTypePlugin) transformOkConstructor(call *ast.CallExpr, resultInfo *resultReturnInfo) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Ok() expects exactly one argument, found %d", len(call.Args))
		return call // Return unchanged
	}

	valueArg := call.Args[0]

	// Comment stripping removed - handled by go/printer

	// Determine type arguments - use resultInfo if available, otherwise infer
	var okType, errType string
	if resultInfo != nil {
		// Use types from enclosing function's return type
		okType = resultInfo.okType
		errType = resultInfo.errType
	} else {
		// Infer types from expression
		var err error
		okType, err = p.inferTypeFromExpr(valueArg)
		if err != nil {
			p.ctx.Logger.Warnf("Type inference failed for Ok(%s): %v, using any fallback", FormatExprForDebug(valueArg), err)
			okType = "any"
		}
		if okType == "" {
			p.ctx.Logger.Warnf("Type inference returned empty string for Ok(%s), using any fallback", FormatExprForDebug(valueArg))
			okType = "any"
		}
		errType = "error" // Default error type
	}

	p.ctx.Logger.Debugf("Go 1.18+ generics: Ok(%s) → dgo.Ok[%s, %s](...)", FormatExprForDebug(valueArg), okType, errType)

	// Create: dgo.Ok[T, E] - both type args required for Ok (E is second, can't be specified alone)
	runtimeOk := &ast.SelectorExpr{
		X:   ast.NewIdent("dgo"),
		Sel: ast.NewIdent("Ok"),
	}

	// Create generic instantiation: dgo.Ok[T, E]
	//
	// POSITION STRATEGY: We copy positions from the original call expression to all
	// synthetic AST nodes. This is intentional - go/printer uses token positions to
	// determine line breaks. By giving all nodes the same position (or positions on
	// the same line), we force go/printer to keep the entire expression on one line.
	genericOk := &ast.IndexListExpr{
		X:      runtimeOk,
		Lbrack: call.Lparen, // Position hint for go/printer (forces same line)
		Indices: []ast.Expr{
			ast.NewIdent(okType),
			ast.NewIdent(errType),
		},
		Rbrack: call.Lparen, // Same position = same line
	}

	// Create call: dgo.Ok[T, E](value)
	replacement := &ast.CallExpr{
		Fun:    genericOk,
		Lparen: call.Lparen,
		Args:   []ast.Expr{valueArg},
		Rparen: call.Rparen,
	}

	return replacement
}

// transformErrConstructor transforms Err(error) → dgo.Err(error) or dgo.Err[T, E](error)
//
// When enclosing function's return type is known (resultInfo != nil), omits type
// arguments since Go can infer them from context. Otherwise uses explicit types.
//
// Returns the replacement node, or the original call if transformation fails
func (p *ResultTypePlugin) transformErrConstructor(call *ast.CallExpr, resultInfo *resultReturnInfo) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Err() expects exactly one argument, found %d", len(call.Args))
		return call // Return unchanged
	}

	errorArg := call.Args[0]

	// Comment stripping removed - handled by go/printer

	// Determine type arguments - use resultInfo if available, otherwise infer
	var okType, errType string
	if resultInfo != nil {
		// Use types from enclosing function's return type
		okType = resultInfo.okType
		errType = resultInfo.errType
	} else {
		// Infer types from expression
		var err error
		errType, err = p.inferTypeFromExpr(errorArg)
		if err != nil {
			p.ctx.Logger.Warnf("Type inference failed for Err(%s): %v, defaulting to 'error'", FormatExprForDebug(errorArg), err)
			errType = "error"
		}
		if errType == "" {
			p.ctx.Logger.Warnf("Type inference returned empty string for Err(%s), defaulting to 'error'", FormatExprForDebug(errorArg))
			errType = "error"
		}
		okType = "any" // For Err() without context, use "any" for Ok type
	}

	p.ctx.Logger.Debugf("Go 1.18+ generics: Err(%s) → dgo.Err[%s](...) (E=%s inferred from arg)", FormatExprForDebug(errorArg), okType, errType)

	// Create: dgo.Err[T] - only T is explicit, E is inferred from argument
	runtimeErr := &ast.SelectorExpr{
		X:   ast.NewIdent("dgo"),
		Sel: ast.NewIdent("Err"),
	}

	// Create generic instantiation: dgo.Err[T]
	// See POSITION STRATEGY comment in transformOkConstructor for rationale
	genericErr := &ast.IndexExpr{
		X:      runtimeErr,
		Lbrack: call.Lparen, // Position hint for go/printer (forces same line)
		Index:  ast.NewIdent(okType),
		Rbrack: call.Lparen, // Same position = same line
	}

	// Create call: dgo.Err[T](error)
	replacement := &ast.CallExpr{
		Fun:    genericErr,
		Lparen: call.Lparen,
		Args:   []ast.Expr{errorArg},
		Rparen: call.Rparen,
	}

	return replacement
}

// transformOkConstructorWithType transforms Ok[ErrType](value) → dgo.Ok[T, E](value)
//
// The error type is explicitly provided via the type parameter.
// The ok type is inferred from the value argument.
func (p *ResultTypePlugin) transformOkConstructorWithType(call *ast.CallExpr, errTypeExpr ast.Expr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Ok[E]() expects exactly one argument, found %d", len(call.Args))
		return call
	}

	valueArg := call.Args[0]

	// Comment stripping removed - handled by go/printer

	// Infer okType from the value argument
	okType, err := p.inferTypeFromExpr(valueArg)
	if err != nil {
		p.ctx.Logger.Warnf("Type inference failed for Ok[E](%s): %v, using any fallback", FormatExprForDebug(valueArg), err)
		okType = "any"
	}
	if okType == "" {
		okType = "any"
	}

	// Get errType from the explicit type parameter
	errType := p.getTypeName(errTypeExpr)
	if errType == "" {
		errType = "error"
	}

	p.ctx.Logger.Debugf("Go 1.18+ generics: Ok[%s](%s) → dgo.Ok[%s, %s](...)", errType, FormatExprForDebug(valueArg), okType, errType)

	// Create: dgo.Ok[T, E](value) - both type args required (E is second, can't skip T)
	// See POSITION STRATEGY comment in transformOkConstructor for rationale
	runtimeOk := &ast.SelectorExpr{
		X:   ast.NewIdent("dgo"),
		Sel: ast.NewIdent("Ok"),
	}
	genericOk := &ast.IndexListExpr{
		X:      runtimeOk,
		Lbrack: call.Lparen, // Position hint for go/printer (forces same line)
		Indices: []ast.Expr{
			ast.NewIdent(okType),
			ast.NewIdent(errType),
		},
		Rbrack: call.Lparen, // Same position = same line
	}
	replacement := &ast.CallExpr{
		Fun:    genericOk,
		Lparen: call.Lparen,
		Args:   []ast.Expr{valueArg},
		Rparen: call.Rparen,
	}

	return replacement
}

// transformErrConstructorWithType transforms Err[OkType](error) → dgo.Err[T](error)
//
// The ok type (T) is explicitly provided via the type parameter.
// The error type (E) is inferred from the error argument by Go.
func (p *ResultTypePlugin) transformErrConstructorWithType(call *ast.CallExpr, okTypeExpr ast.Expr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Err[T]() expects exactly one argument, found %d", len(call.Args))
		return call
	}

	errorArg := call.Args[0]

	// Comment stripping removed - handled by go/printer

	// Get okType from the explicit type parameter
	okType := p.getTypeName(okTypeExpr)
	if okType == "" {
		okType = "any"
	}

	p.ctx.Logger.Debugf("Go 1.18+ generics: Err[%s](%s) → dgo.Err[%s](...) (E inferred from arg)", okType, FormatExprForDebug(errorArg), okType)

	// Create: dgo.Err[T](error) - E is inferred from argument
	// See POSITION STRATEGY comment in transformOkConstructor for rationale
	runtimeErr := &ast.SelectorExpr{
		X:   ast.NewIdent("dgo"),
		Sel: ast.NewIdent("Err"),
	}
	genericErr := &ast.IndexExpr{
		X:      runtimeErr,
		Lbrack: call.Lparen, // Position hint for go/printer (forces same line)
		Index:  ast.NewIdent(okType),
		Rbrack: call.Lparen, // Same position = same line
	}
	replacement := &ast.CallExpr{
		Fun:    genericErr,
		Lparen: call.Lparen,
		Args:   []ast.Expr{errorArg},
		Rparen: call.Rparen,
	}

	return replacement
}

// inferTypeFromExpr infers the type of an expression
//
// Fix A5: Updated to use TypeInferenceService with go/types integration
// CRITICAL FIX #3: Now returns error on failure instead of empty string
//
// Strategy:
// 1. Use TypeInferenceService.InferType() for go/types-based inference (most accurate)
// 2. Fall back to heuristics if go/types unavailable
// 3. Return explicit error on complete failure
//
// Returns: (Type name string, error) - error is non-nil if inference fails
func (p *ResultTypePlugin) inferTypeFromExpr(expr ast.Expr) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("cannot infer type from nil expression")
	}

	// Fix A5: Use TypeInferenceService if available
	if p.typeInference != nil {
		typ, ok := p.typeInference.InferType(expr)
		if ok && typ != nil {
			typeName := p.typeInference.TypeToString(typ)
			p.ctx.Logger.Debugf("Fix A5: TypeInferenceService resolved %T to %s", expr, typeName)
			return typeName, nil
		}
		p.ctx.Logger.Debugf("Fix A5: TypeInferenceService could not infer type for %T", expr)
	}

	// Fallback to structural heuristics for basic cases
	switch e := expr.(type) {
	case *ast.BasicLit:
		// Infer from literal kind
		switch e.Kind {
		case token.INT:
			return "int", nil
		case token.FLOAT:
			return "float64", nil
		case token.STRING:
			return "string", nil
		case token.CHAR:
			return "rune", nil
		}

	case *ast.Ident:
		// Special built-in types
		switch e.Name {
		case "nil":
			return "interface{}", nil
		case "true", "false":
			return "bool", nil
		}

		// CRITICAL FIX #3: Return explicit error for identifiers
		return "", fmt.Errorf("cannot determine type of identifier '%s' without go/types", e.Name)

	case *ast.CompositeLit:
		// Struct/array/map literals with explicit type
		if e.Type != nil {
			return p.exprToTypeString(e.Type), nil
		}
		// CRITICAL FIX #3: Return explicit error
		return "", fmt.Errorf("cannot infer composite literal type without explicit type")

	case *ast.UnaryExpr:
		// &x → pointer to x's type
		if e.Op == token.AND {
			innerType, err := p.inferTypeFromExpr(e.X)
			if err == nil && innerType != "" && innerType != "interface{}" {
				return "*" + innerType, nil
			}
			return "", fmt.Errorf("cannot infer pointer type: %w", err)
		}
		// CRITICAL FIX #3: Return explicit error
		return "", fmt.Errorf("cannot infer unary expression type for op %v", e.Op)

	case *ast.CallExpr:
		// CRITICAL FIX #3: Return explicit error for function calls
		return "", fmt.Errorf("function call requires go/types for return type inference")

	case *ast.StarExpr:
		// CRITICAL FIX #3: Return explicit error
		return "", fmt.Errorf("dereference requires type info")

	case *ast.SelectorExpr:
		// CRITICAL FIX #3: Return explicit error
		return "", fmt.Errorf("field/method access requires type info")

	case *ast.IndexExpr:
		// CRITICAL FIX #3: Return explicit error
		return "", fmt.Errorf("array/slice/map indexing requires type info")

	case *ast.ArrayType:
		return p.exprToTypeString(e), nil

	case *ast.StructType:
		return p.exprToTypeString(e), nil

	case *ast.FuncType:
		return p.exprToTypeString(e), nil

	case *ast.InterfaceType:
		return p.exprToTypeString(e), nil

	case *ast.MapType:
		return p.exprToTypeString(e), nil

	case *ast.ChanType:
		return p.exprToTypeString(e), nil
	}

	// CRITICAL FIX #3: Return explicit error for unknown expression types
	return "", fmt.Errorf("type inference failed for expression type %T", expr)
}

// exprToTypeString converts an AST type expression to a string representation
func (p *ResultTypePlugin) exprToTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name

	case *ast.StarExpr:
		return "*" + p.exprToTypeString(t.X)

	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.exprToTypeString(t.Elt)
		}
		// For sized arrays, would need to evaluate length expression
		return "[]" + p.exprToTypeString(t.Elt)

	case *ast.SelectorExpr:
		pkg := p.exprToTypeString(t.X)
		return pkg + "." + t.Sel.Name

	case *ast.MapType:
		key := p.exprToTypeString(t.Key)
		value := p.exprToTypeString(t.Value)
		return fmt.Sprintf("map[%s]%s", key, value)

	case *ast.ChanType:
		elem := p.exprToTypeString(t.Value)
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + elem
		case ast.RECV:
			return "<-chan " + elem
		default:
			return "chan " + elem
		}

	case *ast.InterfaceType:
		return "interface{}"

	case *ast.StructType:
		return "struct{}"

	case *ast.FuncType:
		return "func()"
	}

	return "interface{}"
}

// emitResultDeclaration is a no-op with Go 1.18+ generics
// Type declarations are provided by the runtime package (github.com/MadAppGang/dingo/pkg/dgo)
// We only need to register types for type inference, not generate AST declarations
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
	if p.ctx == nil {
		return
	}

	// Normalize type names for type inference registration
	okType = NormalizeTypeName(okType)
	errType = NormalizeTypeName(errType)

	// Register type with type inference service (for compile-time checking)
	if p.typeInference != nil {
		okTypeObj := p.typeInference.makeBasicType(okType)
		errTypeObj := p.typeInference.makeBasicType(errType)
		p.typeInference.RegisterResultType(resultTypeName, okTypeObj, errTypeObj, okType, errType)
	}

	// Mark as emitted (to avoid duplicate registrations)
	p.emittedTypes[resultTypeName] = true

	// NOTE: No AST declarations are generated
	// The generic Result[T, E] type is provided by runtime package
	return

	// ---- LEGACY CODE BELOW (disabled) ----
	// This code generated per-type declarations, no longer needed with Go 1.18+ generics
	_ = p.emittedTypes // Keep compiler happy

	// Generate ResultTag enum (only once)
	if false && !p.emittedTypes["ResultTag"] {
		p.emitResultTagEnum()
		p.emittedTypes["ResultTag"] = true
	}

	// Generate Result struct
	resultStruct := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    resultTypeName,
				},
				Type: &ast.StructType{
					Struct: token.NoPos, // Prevent comment grabbing
					Fields: &ast.FieldList{
						Opening: token.NoPos, // Prevent comment grabbing
						Closing: token.NoPos, // Prevent comment grabbing
						List: []*ast.Field{
							{
								Names: []*ast.Ident{
									{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "tag",
									},
								},
								Type: &ast.Ident{
									NamePos: token.NoPos, // Prevent comment grabbing
									Name:    "ResultTag",
								},
							},
							{
								Names: []*ast.Ident{
									{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "ok",
									},
								},
								Type: p.typeToAST(okType, true), // Pointer for zero-value safety
							},
							{
								Names: []*ast.Ident{
									{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "err",
									},
								},
								Type: p.typeToAST(errType, true), // Pointer
							},
						},
					},
				},
			},
		},
	}

	p.pendingDecls = append(p.pendingDecls, resultStruct)

	// CRITICAL FIX #1: Register the Result type with type inference service
	if p.typeInference != nil {
		okTypeObj := p.typeInference.makeBasicType(okType)
		errTypeObj := p.typeInference.makeBasicType(errType)
		p.typeInference.RegisterResultType(resultTypeName, okTypeObj, errTypeObj, okType, errType)
	}

	// Generate constructor functions
	p.emitConstructorFunction(resultTypeName, okType, true, "Ok")
	p.emitConstructorFunction(resultTypeName, errType, false, "Err")

	// Generate helper methods
	p.emitHelperMethods(resultTypeName, okType, errType)
}

// emitResultTagEnum generates the ResultTag enum
func (p *ResultTypePlugin) emitResultTagEnum() {
	// type ResultTag uint8
	tagTypeDecl := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    "ResultTag",
				},
				Type: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    "uint8",
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, tagTypeDecl)

	// const ( ResultTagOk ResultTag = iota; ResultTagErr )
	tagConstDecl := &ast.GenDecl{
		Tok:    token.CONST,
		Lparen: 1, // Required for const block
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					{
						NamePos: token.NoPos, // Prevent comment grabbing
						Name:    "ResultTagOk",
					},
				},
				Type: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    "ResultTag",
				},
				Values: []ast.Expr{
					&ast.Ident{
						NamePos: token.NoPos, // Prevent comment grabbing
						Name:    "iota",
					},
				},
			},
			&ast.ValueSpec{
				Names: []*ast.Ident{
					{
						NamePos: token.NoPos, // Prevent comment grabbing
						Name:    "ResultTagErr",
					},
				},
			},
		},
		Rparen: 2, // Required for const block
	}
	p.pendingDecls = append(p.pendingDecls, tagConstDecl)
}

// emitConstructorFunction generates Ok or Err constructor
func (p *ResultTypePlugin) emitConstructorFunction(resultTypeName, argType string, isOk bool, funcSuffix string) {
	variantTag := "ResultTagOk"
	fieldName := "ok"
	if !isOk {
		variantTag = "ResultTagErr"
		fieldName = "err"
	}

	funcName := fmt.Sprintf("%s%s", resultTypeName, funcSuffix)
	argTypeAST := p.typeToAST(argType, false) // Non-pointer parameter

	// func Result_T_E_Ok(arg0 T) Result_T_E {
	//     return Result_T_E{tag: ResultTagOk, ok: &arg0}
	// }
	constructorFunc := &ast.FuncDecl{
		Name: &ast.Ident{
			NamePos: token.NoPos, // Prevent comment grabbing
			Name:    funcName,
		},
		Type: &ast.FuncType{
			Func: token.NoPos, // Prevent comment grabbing
			Params: &ast.FieldList{
				Opening: token.NoPos, // Prevent comment grabbing
				Closing: token.NoPos, // Prevent comment grabbing
				List: []*ast.Field{
					{
						Names: []*ast.Ident{
							{
								NamePos: token.NoPos, // Prevent comment grabbing
								Name:    "arg0",
							},
						},
						Type: argTypeAST,
					},
				},
			},
			Results: &ast.FieldList{
				Opening: token.NoPos, // Prevent comment grabbing
				Closing: token.NoPos, // Prevent comment grabbing
				List: []*ast.Field{
					{
						Type: &ast.Ident{
							NamePos: token.NoPos, // Prevent comment grabbing
							Name:    resultTypeName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			Lbrace: token.NoPos, // Prevent comment grabbing
			Rbrace: token.NoPos, // Prevent comment grabbing
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Return: token.NoPos, // Prevent comment grabbing
					Results: []ast.Expr{
						&ast.CompositeLit{
							Lbrace: token.NoPos, // Prevent comment grabbing
							Rbrace: token.NoPos, // Prevent comment grabbing
							Type: &ast.Ident{
								NamePos: token.NoPos, // Prevent comment grabbing
								Name:    resultTypeName,
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Colon: token.NoPos, // Prevent comment grabbing
									Key: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "tag",
									},
									Value: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    variantTag,
									},
								},
								&ast.KeyValueExpr{
									Colon: token.NoPos, // Prevent comment grabbing
									Key: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    fieldName,
									},
									Value: &ast.UnaryExpr{
										OpPos: token.NoPos, // Prevent comment grabbing
										Op:    token.AND,
										X: &ast.Ident{
											NamePos: token.NoPos, // Prevent comment grabbing
											Name:    "arg0",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	p.pendingDecls = append(p.pendingDecls, constructorFunc)
}

// emitHelperMethods generates IsOk, IsErr, Unwrap, UnwrapOr, etc.
func (p *ResultTypePlugin) emitHelperMethods(resultTypeName, okType, errType string) {
	// IsOk() bool
	isOkMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("IsOk"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("bool")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagOk"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, isOkMethod)

	// IsErr() bool
	isErrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("IsErr"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("bool")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagErr"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, isErrMethod)

	// Unwrap() T - panics if Err
	// Note: Returns *T (dereferenced), so we need to handle pointer unwrapping
	unwrapMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("Unwrap"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(okType, false)}, // Non-pointer return
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag != ResultTagOk { panic("called Unwrap on Err") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.NEQ,
						Y:  ast.NewIdent("ResultTagOk"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"called Unwrap on Err"`,
										},
									},
								},
							},
						},
					},
				},
				// if r.ok == nil { panic("Result contains nil Ok value") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
						Op: token.EQL,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"Result contains nil Ok value"`,
										},
									},
								},
							},
						},
					},
				},
				// return *r.ok
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.StarExpr{
							X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapMethod)

	// UnwrapOr(defaultValue T) T
	unwrapOrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapOr"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("defaultValue")},
						Type:  p.typeToAST(okType, false),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(okType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk { return *r.ok }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("ResultTagOk"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
									},
								},
							},
						},
					},
				},
				// return defaultValue
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("defaultValue")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapOrMethod)

	// UnwrapErr() E - panics if Ok
	unwrapErrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapErr"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(errType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag != ResultTagErr { panic("called UnwrapErr on Ok") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.NEQ,
						Y:  ast.NewIdent("ResultTagErr"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"called UnwrapErr on Ok"`,
										},
									},
								},
							},
						},
					},
				},
				// if r.err == nil { panic("Result contains nil Err value") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
						Op: token.EQL,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"Result contains nil Err value"`,
										},
									},
								},
							},
						},
					},
				},
				// return *r.err
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.StarExpr{
							X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapErrMethod)

	// Task 3a: Enable complete helper method set
	p.emitAdvancedHelperMethods(resultTypeName, okType, errType)
}

// emitAdvancedHelperMethods generates Map, MapErr, Filter, AndThen, OrElse, And, Or methods
// Task 3a: Complete helper method implementation
func (p *ResultTypePlugin) emitAdvancedHelperMethods(resultTypeName, okType, errType string) {
	// UnwrapOrElse(fn func(error) T) T
	// Returns Ok value or calls fn with Err value
	unwrapOrElseMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapOrElse"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(errType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(okType, false)},
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(okType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk && r.ok != nil { return *r.ok }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagOk"),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
							Op: token.NEQ,
							Y:  ast.NewIdent("nil"),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
									},
								},
							},
						},
					},
				},
				// if r.err != nil { return fn(*r.err) }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
						Op: token.NEQ,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.CallExpr{
										Fun: ast.NewIdent("fn"),
										Args: []ast.Expr{
											&ast.StarExpr{
												X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
											},
										},
									},
								},
							},
						},
					},
				},
				// panic("Result in invalid state")
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: ast.NewIdent("panic"),
						Args: []ast.Expr{
							&ast.BasicLit{
								Kind:  token.STRING,
								Value: `"Result in invalid state"`,
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapOrElseMethod)

	// Map(fn func(T) U) Result<U, E>
	// Transforms the Ok value if present
	// Note: Since we don't have generics, we use interface{} for U and return a generic Result
	mapMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("Map"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(okType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Generic U type
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")}, // Returns Result<U, E>
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk && r.ok != nil {
				//     u := fn(*r.ok)
				//     return Result_interface{}_error{tag: ResultTagOk, ok: &u}
				// }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagOk"),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
							Op: token.NEQ,
							Y:  ast.NewIdent("nil"),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							// u := fn(*r.ok)
							&ast.AssignStmt{
								Lhs: []ast.Expr{ast.NewIdent("u")},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.CallExpr{
										Fun: ast.NewIdent("fn"),
										Args: []ast.Expr{
											&ast.StarExpr{
												X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
											},
										},
									},
								},
							},
							// return struct with u
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.CompositeLit{
										Type: &ast.StructType{
											Fields: &ast.FieldList{
												List: []*ast.Field{
													{Names: []*ast.Ident{ast.NewIdent("tag")}, Type: ast.NewIdent("ResultTag")},
													{Names: []*ast.Ident{ast.NewIdent("ok")}, Type: &ast.StarExpr{X: ast.NewIdent("interface{}")}},
													{Names: []*ast.Ident{ast.NewIdent("err")}, Type: p.typeToAST(errType, true)},
												},
											},
										},
										Elts: []ast.Expr{
											&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: ast.NewIdent("ResultTagOk")},
											&ast.KeyValueExpr{
												Key: ast.NewIdent("ok"),
												Value: &ast.UnaryExpr{
													Op: token.AND,
													X:  ast.NewIdent("u"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				// return Err variant unchanged (cast to interface{})
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{
										{Names: []*ast.Ident{ast.NewIdent("tag")}, Type: ast.NewIdent("ResultTag")},
										{Names: []*ast.Ident{ast.NewIdent("ok")}, Type: &ast.StarExpr{X: ast.NewIdent("interface{}")}},
										{Names: []*ast.Ident{ast.NewIdent("err")}, Type: p.typeToAST(errType, true)},
									},
								},
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")}},
								&ast.KeyValueExpr{Key: ast.NewIdent("ok"), Value: ast.NewIdent("nil")},
								&ast.KeyValueExpr{Key: ast.NewIdent("err"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")}},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, mapMethod)

	// MapErr(fn func(E) F) Result<T, F>
	// Transforms the Err value if present (returns interface{} for simplicity)
	mapErrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("MapErr"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(errType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Generic F type
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")}, // Returns Result<T, F>
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagErr && r.err != nil {
				//     f := fn(*r.err)
				//     return Result with mapped error
				// }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagErr"),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
							Op: token.NEQ,
							Y:  ast.NewIdent("nil"),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							// f := fn(*r.err)
							&ast.AssignStmt{
								Lhs: []ast.Expr{ast.NewIdent("f")},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.CallExpr{
										Fun: ast.NewIdent("fn"),
										Args: []ast.Expr{
											&ast.StarExpr{
												X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
											},
										},
									},
								},
							},
							// return Result with mapped error
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.CompositeLit{
										Type: &ast.StructType{
											Fields: &ast.FieldList{
												List: []*ast.Field{
													{Names: []*ast.Ident{ast.NewIdent("tag")}, Type: ast.NewIdent("ResultTag")},
													{Names: []*ast.Ident{ast.NewIdent("ok")}, Type: p.typeToAST(okType, true)},
													{Names: []*ast.Ident{ast.NewIdent("err")}, Type: &ast.StarExpr{X: ast.NewIdent("interface{}")}},
												},
											},
										},
										Elts: []ast.Expr{
											&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: ast.NewIdent("ResultTagErr")},
											&ast.KeyValueExpr{Key: ast.NewIdent("ok"), Value: ast.NewIdent("nil")},
											&ast.KeyValueExpr{
												Key: ast.NewIdent("err"),
												Value: &ast.UnaryExpr{
													Op: token.AND,
													X:  ast.NewIdent("f"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				// return Ok variant unchanged
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{
										{Names: []*ast.Ident{ast.NewIdent("tag")}, Type: ast.NewIdent("ResultTag")},
										{Names: []*ast.Ident{ast.NewIdent("ok")}, Type: p.typeToAST(okType, true)},
										{Names: []*ast.Ident{ast.NewIdent("err")}, Type: &ast.StarExpr{X: ast.NewIdent("interface{}")}},
									},
								},
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")}},
								&ast.KeyValueExpr{Key: ast.NewIdent("ok"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")}},
								&ast.KeyValueExpr{Key: ast.NewIdent("err"), Value: ast.NewIdent("nil")},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, mapErrMethod)

	// Filter(predicate func(T) bool) Result<T, E>
	// Converts Ok to Err if predicate fails
	filterMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("Filter"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("predicate")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(okType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("bool")},
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent(resultTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk && predicate(*r.ok) { return r }
				// else { return Err variant }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagOk"),
						},
						Op: token.LAND,
						Y: &ast.CallExpr{
							Fun: ast.NewIdent("predicate"),
							Args: []ast.Expr{
								&ast.StarExpr{
									X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
								},
							},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("r")},
							},
						},
					},
				},
				// Return error variant (would need proper error creation)
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("r")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, filterMethod)

	// AndThen(fn func(T) Result<U, E>) Result<U, E>
	// Monadic bind operation
	andThenMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("AndThen"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(okType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Result<U, E>
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk && r.ok != nil { return fn(*r.ok) }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagOk"),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
							Op: token.NEQ,
							Y:  ast.NewIdent("nil"),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.CallExpr{
										Fun: ast.NewIdent("fn"),
										Args: []ast.Expr{
											&ast.StarExpr{
												X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")},
											},
										},
									},
								},
							},
						},
					},
				},
				// Return Err variant as interface{} with same structure
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{
										{Names: []*ast.Ident{ast.NewIdent("tag")}, Type: ast.NewIdent("ResultTag")},
										{Names: []*ast.Ident{ast.NewIdent("ok")}, Type: &ast.StarExpr{X: ast.NewIdent("interface{}")}},
										{Names: []*ast.Ident{ast.NewIdent("err")}, Type: p.typeToAST(errType, true)},
									},
								},
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")}},
								&ast.KeyValueExpr{Key: ast.NewIdent("ok"), Value: ast.NewIdent("nil")},
								&ast.KeyValueExpr{Key: ast.NewIdent("err"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")}},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, andThenMethod)

	// OrElse(fn func(E) Result<T, F>) Result<T, F>
	// Handle Err case with fallback
	orElseMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("OrElse"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(errType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Result<T, F>
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagErr && r.err != nil { return fn(*r.err) }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTagErr"),
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
							Op: token.NEQ,
							Y:  ast.NewIdent("nil"),
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.CallExpr{
										Fun: ast.NewIdent("fn"),
										Args: []ast.Expr{
											&ast.StarExpr{
												X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err")},
											},
										},
									},
								},
							},
						},
					},
				},
				// Return Ok variant as interface{} with same structure
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{
										{Names: []*ast.Ident{ast.NewIdent("tag")}, Type: ast.NewIdent("ResultTag")},
										{Names: []*ast.Ident{ast.NewIdent("ok")}, Type: p.typeToAST(okType, true)},
										{Names: []*ast.Ident{ast.NewIdent("err")}, Type: &ast.StarExpr{X: ast.NewIdent("interface{}")}},
									},
								},
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")}},
								&ast.KeyValueExpr{Key: ast.NewIdent("ok"), Value: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok")}},
								&ast.KeyValueExpr{Key: ast.NewIdent("err"), Value: ast.NewIdent("nil")},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, orElseMethod)

	// And(other Result<U, E>) Result<U, E>
	// Returns other if Ok, returns Err if Err
	andMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("And"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("other")},
						Type:  ast.NewIdent("interface{}"), // Generic Result<U, E>
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk { return other }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("ResultTagOk"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("other")},
							},
						},
					},
				},
				// return r (as Err variant)
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("r")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, andMethod)

	// Or(other Result<T, E>) Result<T, E>
	// Returns r if Ok, returns other if Err
	orMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("Or"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("other")},
						Type:  ast.NewIdent(resultTypeName),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent(resultTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTagOk { return r }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("ResultTagOk"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("r")},
							},
						},
					},
				},
				// return other
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("other")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, orMethod)
}

// getTypeName extracts type name from AST expression
func (p *ResultTypePlugin) getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.getTypeName(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.getTypeName(t.Elt)
		}
		return "[N]" + p.getTypeName(t.Elt)
	case *ast.SelectorExpr:
		return p.getTypeName(t.X) + "." + t.Sel.Name
	default:
		return "unknown"
	}
}

// sanitizeTypeName is deprecated - use shared SanitizeTypeName instead
// This function is kept for backward compatibility during migration
func (p *ResultTypePlugin) sanitizeTypeName(typeName string) string {
	// Delegate to shared utility
	// Note: SanitizeTypeName takes variadic args, so pass single type
	return SanitizeTypeName(typeName)
}

// typeToAST converts a type string to an AST type expression
func (p *ResultTypePlugin) typeToAST(typeName string, asPointer bool) ast.Expr {
	var baseType ast.Expr

	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		baseType = &ast.StarExpr{
			X: ast.NewIdent(strings.TrimPrefix(typeName, "*")),
		}
	} else if strings.HasPrefix(typeName, "[]") {
		// Slice type
		baseType = &ast.ArrayType{
			Elt: ast.NewIdent(strings.TrimPrefix(typeName, "[]")),
		}
	} else {
		// Simple identifier
		baseType = ast.NewIdent(typeName)
	}

	// Wrap in pointer if requested
	if asPointer {
		return &ast.StarExpr{X: baseType}
	}

	return baseType
}

// GetPendingDeclarations returns declarations to be injected at package level
func (p *ResultTypePlugin) GetPendingDeclarations() []ast.Decl {
	return p.pendingDecls
}

// ClearPendingDeclarations clears the pending declarations list
func (p *ResultTypePlugin) ClearPendingDeclarations() {
	p.pendingDecls = make([]ast.Decl, 0)
}

// Transform performs AST transformations on the node
// This method replaces:
// 1. Ok() and Err() constructor calls with struct literals
// 2. Result[T, E] generic syntax with concrete ResultTE types
// 3. Implicit Result wrapping: `return value` → `return ResultTEOk(value)` or `return ResultTEErr(error)`
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
	if p.ctx == nil {
		return nil, fmt.Errorf("plugin context not initialized")
	}

	// Store reference to file for comment stripping
	if file, ok := node.(*ast.File); ok {
		p.file = file
	}

	// Stack to track current function context for implicit wrapping
	var funcStack []ast.Node

	// Use astutil.Apply to walk and transform the AST
	transformed := astutil.Apply(node,
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()

			// Track function entry for implicit wrapping
			if fd, ok := n.(*ast.FuncDecl); ok {
				funcStack = append(funcStack, fd)
			} else if fl, ok := n.(*ast.FuncLit); ok {
				funcStack = append(funcStack, fl)
			}

			// Check for generic type references (Result[T, E]) that need rewriting
			if indexExpr, ok := n.(*ast.IndexExpr); ok {
				if replacement, found := p.genericTypeRewrites[indexExpr]; found {
					cursor.Replace(&ast.Ident{
						NamePos: indexExpr.Pos(),
						Name:    replacement,
					})
					return true
				}
			}

			// Check for generic type references with multiple type params (Result[T, E])
			if indexListExpr, ok := n.(*ast.IndexListExpr); ok {
				if replacement, found := p.genericListRewrites[indexListExpr]; found {
					cursor.Replace(&ast.Ident{
						NamePos: indexListExpr.Pos(),
						Name:    replacement,
					})
					return true
				}

				// Check for Result[T, E] that needs dgo. prefix
				if p.resultTypeRewrites[indexListExpr] {
					// Rewrite Result[T, E] → dgo.Result[T, E]
					// Keep the same IndexListExpr structure, just change the X (base) to dgo.Result
					newExpr := &ast.IndexListExpr{
						X: &ast.SelectorExpr{
							X:   ast.NewIdent("dgo"),
							Sel: ast.NewIdent("Result"),
						},
						Lbrack:  indexListExpr.Lbrack,
						Indices: indexListExpr.Indices,
						Rbrack:  indexListExpr.Rbrack,
					}
					cursor.Replace(newExpr)
					p.ctx.Logger.Debugf("Transform: Result[T, E] → dgo.Result[T, E]")
					return true
				}
			}

			// Check for return statements that need implicit wrapping
			if ret, ok := n.(*ast.ReturnStmt); ok {
				if len(funcStack) > 0 && len(ret.Results) == 1 {
					currentFunc := funcStack[len(funcStack)-1]
					if resultInfo, found := p.funcResultTypes[currentFunc]; found {
						// Check if the return value needs wrapping
						wrapped := p.wrapReturnForResult(ret.Results[0], resultInfo, ret)
						if wrapped != nil {
							ret.Results[0] = wrapped
						}
					}
				}
			}

			// Check if this is a CallExpr we need to transform
			if call, ok := n.(*ast.CallExpr); ok {
				var replacement ast.Expr

				// Get result info from enclosing function if available
				var resultInfo *resultReturnInfo
				if len(funcStack) > 0 {
					currentFunc := funcStack[len(funcStack)-1]
					resultInfo = p.funcResultTypes[currentFunc]
				}

				// Case 1: Ok(value) or Err(error) - plain identifier
				if ident, ok := call.Fun.(*ast.Ident); ok {
					switch ident.Name {
					case "Ok":
						replacement = p.transformOkConstructor(call, resultInfo)
					case "Err":
						replacement = p.transformErrConstructor(call, resultInfo)
					}
				}

				// Case 2: Ok[ErrType](value) or Err[OkType](error) - IndexExpr with type param
				if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
					if ident, ok := indexExpr.X.(*ast.Ident); ok {
						switch ident.Name {
						case "Ok":
							replacement = p.transformOkConstructorWithType(call, indexExpr.Index)
						case "Err":
							replacement = p.transformErrConstructorWithType(call, indexExpr.Index)
						}
					}
				}

				// Case 3: dgo.Err[T, E](error) from preprocessor - simplify to dgo.Err[T](error)
				// E can be inferred from argument, so we only need to specify T
				if indexListExpr, ok := call.Fun.(*ast.IndexListExpr); ok {
					if selExpr, ok := indexListExpr.X.(*ast.SelectorExpr); ok {
						if pkgIdent, ok := selExpr.X.(*ast.Ident); ok && pkgIdent.Name == "dgo" {
							if selExpr.Sel.Name == "Err" && len(indexListExpr.Indices) == 2 {
								// Simplify dgo.Err[T, E](...) → dgo.Err[T](...)
								p.ctx.Logger.Debugf("Simplifying dgo.Err[T, E](...) → dgo.Err[T](...)")
								replacement = &ast.CallExpr{
									Fun: &ast.IndexExpr{
										X: &ast.SelectorExpr{
											X:   ast.NewIdent("dgo"),
											Sel: ast.NewIdent("Err"),
										},
										Lbrack: call.Lparen,
										Index:  indexListExpr.Indices[0], // Keep only T
										Rbrack: call.Lparen,
									},
									Lparen: call.Lparen,
									Args:   call.Args,
									Rparen: call.Rparen,
								}
							}
							// Note: dgo.Ok[T, E] stays as-is because E is second param and can't be skipped
						}
					}
				}

				// Replace the node if transformation occurred
				if replacement != nil && replacement != call {
					cursor.Replace(replacement)
				}
			}
			return true
		},
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()
			// Track function exit (pop from stack)
			if _, ok := n.(*ast.FuncDecl); ok {
				if len(funcStack) > 0 {
					funcStack = funcStack[:len(funcStack)-1]
				}
			} else if _, ok := n.(*ast.FuncLit); ok {
				if len(funcStack) > 0 {
					funcStack = funcStack[:len(funcStack)-1]
				}
			}
			return true
		},
	)

	return transformed, nil
}
