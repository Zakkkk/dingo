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
}

// resultReturnInfo holds parsed Result<T, E> return type information
type resultReturnInfo struct {
	okType         string // The T in Result<T, E>
	errType        string // The E in Result<T, E>
	resultTypeName string // The sanitized type name (e.g., "ResultUserDBError")
}

// NewResultTypePlugin creates a new Result type plugin
func NewResultTypePlugin() *ResultTypePlugin {
	return &ResultTypePlugin{
		emittedTypes:    make(map[string]bool),
		pendingDecls:    make([]ast.Decl, 0),
		funcResultTypes: make(map[ast.Node]*resultReturnInfo),
	}
}

// Name returns the plugin name
func (p *ResultTypePlugin) Name() string {
	return "result_type"
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

// wrapReturnForResult checks if a return expression needs implicit wrapping
// and returns the wrapped expression, or nil if no wrapping needed.
//
// The logic:
// 1. If return value is already a Result constructor call (Ok, Err, ResultXOk, ResultXErr) → no wrapping
// 2. If return value is already a Result type value → no wrapping
// 3. If return value type matches the error type E → wrap in Err
// 4. If return value type matches the ok type T → wrap in Ok
// 5. If we can't determine the type, assume Ok wrapping (compiler will catch mismatches)
func (p *ResultTypePlugin) wrapReturnForResult(expr ast.Expr, info *resultReturnInfo) ast.Expr {
	// Skip if already a Result constructor call
	if p.isResultConstructorCall(expr) {
		return nil
	}

	// Skip if it's already a Result type (e.g., variable of Result type)
	if p.isResultTypeValue(expr, info.resultTypeName) {
		return nil
	}

	// Try to determine the type of the expression
	exprType := p.determineExpressionType(expr)

	// Check if the expression type matches the error type
	if p.typeMatchesError(expr, exprType, info.errType) {
		// Wrap in Err constructor
		p.ctx.Logger.Debugf("Implicit wrapping: return %s → %sErr(...)", FormatExprForDebug(expr), info.resultTypeName)
		return &ast.CallExpr{
			Fun:  ast.NewIdent(info.resultTypeName + "Err"),
			Args: []ast.Expr{expr},
		}
	}

	// Check if the expression type matches the ok type (or is unknown)
	// If we can determine Ok match OR we can't determine at all, wrap as Ok
	// The Go compiler will catch any type mismatches
	if p.typeMatchesOk(expr, exprType, info.okType) || exprType == "" {
		// Wrap in Ok constructor
		p.ctx.Logger.Debugf("Implicit wrapping: return %s → %sOk(...)", FormatExprForDebug(expr), info.resultTypeName)
		return &ast.CallExpr{
			Fun:  ast.NewIdent(info.resultTypeName + "Ok"),
			Args: []ast.Expr{expr},
		}
	}

	// Type is known but doesn't match either - don't wrap, let compiler catch
	return nil
}

// isResultConstructorCall checks if expr is a call to Ok(), Err(), or ResultXOk/ResultXErr
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
			// UnwrapErr() returns the error type - mark as "error-returning"
			if methodName == "UnwrapErr" {
				return "__error_type__" // Special marker
			}
			// Unwrap() returns the ok type
			if methodName == "Unwrap" {
				return "__ok_type__" // Special marker
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
	var okType, errType string

	switch rt := firstResult.Type.(type) {
	case *ast.IndexListExpr:
		// Result[T, E] with two type parameters
		if ident, ok := rt.X.(*ast.Ident); ok && ident.Name == "Result" {
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
		if ident, ok := rt.X.(*ast.Ident); ok && ident.Name == "Result" {
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
	// Check if the base type is "Result"
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Result" {
		var typeName string
		var resultType string
		// This is a Result<T> (single type parameter)
		// Default error type to "error"

		// Check if type inference is available
		if p.typeInference != nil {
			telemType, ok := p.typeInference.InferType(expr.Index)
			if ok && telemType != nil {
				typeName = p.typeInference.TypeToString(telemType)
				resultType = fmt.Sprintf("Result%s", SanitizeTypeName(typeName, "error"))
			} else {
				p.ctx.Logger.Warnf("ResultTypePlugin: Could not infer type for Result<T> element. Falling back to heuristic.")
				// Fallback to old heuristic if type inference fails
				typeName = p.getTypeName(expr.Index)
				resultType = fmt.Sprintf("Result%s", SanitizeTypeName(typeName, "error"))
			}
		} else {
			// No type inference service - use heuristic
			typeName = p.getTypeName(expr.Index)
			resultType = fmt.Sprintf("Result%s", SanitizeTypeName(typeName, "error"))
		}

		if !p.emittedTypes[resultType] {
			p.emitResultDeclaration(typeName, "error", resultType)
			p.emittedTypes[resultType] = true
		}

		// Track this IndexExpr for replacement during Transform phase
		if p.genericTypeRewrites == nil {
			p.genericTypeRewrites = make(map[*ast.IndexExpr]string)
		}
		p.genericTypeRewrites[expr] = resultType
	}
}

// handleGenericResultList processes Result[T, E] syntax (IndexListExpr for Go 1.18+)
func (p *ResultTypePlugin) handleGenericResultList(expr *ast.IndexListExpr) {
	// Check if the base type is "Result"
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Result" {
		var resultType string

		if len(expr.Indices) == 2 {
			var okType, errType string
			// Result<T, E> with explicit error type

			// Check if type inference is available
			if p.typeInference != nil {
				okElemType, ok := p.typeInference.InferType(expr.Indices[0])
				if !ok || okElemType == nil {
					p.ctx.Logger.Warnf("ResultTypePlugin: Could not infer 'Ok' type for Result<T,E>. Falling back to heuristic.")
					okType = p.getTypeName(expr.Indices[0])
				} else {
					okType = p.typeInference.TypeToString(okElemType)
				}

				errElemType, ok := p.typeInference.InferType(expr.Indices[1])
				if !ok || errElemType == nil {
					p.ctx.Logger.Warnf("ResultTypePlugin: Could not infer 'Err' type for Result<T,E>. Falling back to heuristic.")
					errType = p.getTypeName(expr.Indices[1])
				} else {
					errType = p.typeInference.TypeToString(errElemType)
				}
			} else {
				// No type inference service - use heuristic
				okType = p.getTypeName(expr.Indices[0])
				errType = p.getTypeName(expr.Indices[1])
			}

			resultType = fmt.Sprintf("Result%s",
				SanitizeTypeName(okType, errType))

			if !p.emittedTypes[resultType] {
				p.emitResultDeclaration(okType, errType, resultType)
				p.emittedTypes[resultType] = true
			}
		} else if len(expr.Indices) == 1 {
			var okType string
			// Result<T> with default error type

			// Check if type inference is available
			if p.typeInference != nil {
				okElemType, ok := p.typeInference.InferType(expr.Indices[0])
				if !ok || okElemType == nil {
					p.ctx.Logger.Warnf("ResultTypePlugin: Could not infer 'Ok' type for Result<T>. Falling back to heuristic.")
					okType = p.getTypeName(expr.Indices[0])
				} else {
					okType = p.typeInference.TypeToString(okElemType)
				}
			} else {
				// No type inference service - use heuristic
				okType = p.getTypeName(expr.Indices[0])
			}
			resultType = fmt.Sprintf("Result%s", SanitizeTypeName(okType, "error"))

			if !p.emittedTypes[resultType] {
				p.emitResultDeclaration(okType, "error", resultType)
				p.emittedTypes[resultType] = true
			}
		}

		// Track this IndexListExpr for replacement during Transform phase
		if resultType != "" {
			if p.genericListRewrites == nil {
				p.genericListRewrites = make(map[*ast.IndexListExpr]string)
			}
			p.genericListRewrites[expr] = resultType
		}
	}
}

// handleConstructorCall processes Ok(value) and Err(error) calls
//
// Task 1.2: Transform constructor calls to struct literals
//
// This method detects Ok() and Err() calls and transforms them into
// Result struct literals with the appropriate tag and field values.
//
// Handles both:
// - Ok(value), Err(error) - plain calls
// - Ok[ErrType](value), Err[OkType](error) - calls with explicit type parameter
//
// Type inference strategy:
// 1. Check for explicit type annotation (e.g., let x: Result<int, error> = Ok(42))
// 2. Infer from argument type for T, default error for E
// 3. Use context from surrounding expression (assignment, return, etc.)
func (p *ResultTypePlugin) handleConstructorCall(call *ast.CallExpr) {
	// Case 1: Ok(value) or Err(error) - plain identifier
	if ident, ok := call.Fun.(*ast.Ident); ok {
		switch ident.Name {
		case "Ok":
			p.transformOkConstructor(call)
		case "Err":
			p.transformErrConstructor(call)
		}
		return
	}

	// Case 2: Ok[ErrType](value) or Err[OkType](error) - IndexExpr with type parameter
	if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
		if ident, ok := indexExpr.X.(*ast.Ident); ok {
			switch ident.Name {
			case "Ok":
				p.transformOkConstructorWithType(call, indexExpr.Index)
			case "Err":
				p.transformErrConstructorWithType(call, indexExpr.Index)
			}
		}
	}
}

// transformOkConstructor transforms Ok(value) → Result_T_E{tag: ResultTagOk, ok: &value}
//
// Fix A5: Uses TypeInferenceService for accurate type resolution
// Fix A4: Wraps non-addressable expressions (literals) in IIFE
//
// Returns the replacement node, or the original call if transformation fails
func (p *ResultTypePlugin) transformOkConstructor(call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Ok() expects exactly one argument, found %d", len(call.Args))
		return call // Return unchanged
	}

	valueArg := call.Args[0]

	// CRITICAL FIX #3: Check error from inferTypeFromExpr
	// CRITICAL FIX #5 (Code Review): Use interface{} fallback instead of returning unchanged
	okType, err := p.inferTypeFromExpr(valueArg)
	if err != nil {
		// Type inference failed - use interface{} as fallback
		p.ctx.Logger.Warnf("Type inference failed for Ok(%s): %v, using interface{} fallback", FormatExprForDebug(valueArg), err)
		okType = "interface{}"
	}

	// CRITICAL FIX #3: Validate okType is not empty
	// CRITICAL FIX #5 (Code Review): Use interface{} fallback instead of returning unchanged
	if okType == "" {
		p.ctx.Logger.Warnf("Type inference returned empty string for Ok(%s), using interface{} fallback", FormatExprForDebug(valueArg))
		okType = "interface{}"
	}

	errType := "error" // Default error type

	// Generate unique Result type name
	resultTypeName := fmt.Sprintf("Result%s",
		SanitizeTypeName(okType, errType))

	// Ensure the Result type is declared
	if !p.emittedTypes[resultTypeName] {
		p.emitResultDeclaration(okType, errType, resultTypeName)
		p.emittedTypes[resultTypeName] = true
	}

	// Log transformation with type inference details
	p.ctx.Logger.Debugf("Fix A5: Inferred type for Ok(%s) → %s", FormatExprForDebug(valueArg), okType)

	// Fix A4: Handle addressability - wrap literals in IIFE if needed
	var okValue ast.Expr
	if isAddressable(valueArg) {
		// Direct address-of for addressable expressions
		okValue = &ast.UnaryExpr{
			Op: token.AND,
			X:  valueArg,
		}
		p.ctx.Logger.Debugf("Fix A4: Expression is addressable, using &expr")
	} else {
		// Non-addressable (literal, function call, etc.) - wrap in IIFE
		okValue = wrapInIIFE(valueArg, okType, p.ctx)
		p.ctx.Logger.Debugf("Fix A4: Expression is non-addressable, wrapping in IIFE (temp var: __tmp%d)", p.ctx.TempVarCounter-1)
	}

	// Create the replacement CompositeLit
	// Ok(value) → Result_T_E{tag: ResultTagOk, ok: &value or IIFE}
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(resultTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("tag"),
				Value: ast.NewIdent("ResultTagOk"),
			},
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("ok"),
				Value: okValue,
			},
		},
	}

	return replacement
}

// transformErrConstructor transforms Err(error) → Result_T_E{tag: ResultTagErr, err: &error}
//
// Fix A5: Uses TypeInferenceService for accurate type resolution
// Fix A4: Wraps non-addressable expressions (literals) in IIFE
//
// Returns the replacement node, or the original call if transformation fails
func (p *ResultTypePlugin) transformErrConstructor(call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Err() expects exactly one argument, found %d", len(call.Args))
		return call // Return unchanged
	}

	errorArg := call.Args[0]

	// CRITICAL FIX #3: Check error from inferTypeFromExpr
	errType, err := p.inferTypeFromExpr(errorArg)
	if err != nil {
		// Type inference failed - default to "error"
		p.ctx.Logger.Warnf("Type inference failed for Err(%s): %v, defaulting to 'error'", FormatExprForDebug(errorArg), err)
		errType = "error"
	}

	// CRITICAL FIX #3: Validate errType is not empty
	if errType == "" {
		p.ctx.Logger.Warnf("Type inference returned empty string for Err(%s), defaulting to 'error'", FormatExprForDebug(errorArg))
		errType = "error"
	}

	// For Err(), the Ok type must be inferred from context
	// This is a limitation without full type inference
	// For now, we'll use "interface{}" as a placeholder
	// TODO(Phase 4): Context-based type inference for Err()
	okType := "interface{}" // Will be refined with type inference

	// Generate unique Result type name
	resultTypeName := fmt.Sprintf("Result%s",
		SanitizeTypeName(okType, errType))

	// Ensure the Result type is declared
	if !p.emittedTypes[resultTypeName] {
		p.emitResultDeclaration(okType, errType, resultTypeName)
		p.emittedTypes[resultTypeName] = true
	}

	// Log transformation with type inference details
	p.ctx.Logger.Debugf("Fix A5: Inferred error type for Err(%s) → %s", FormatExprForDebug(errorArg), errType)

	// Fix A4: Handle addressability - wrap literals in IIFE if needed
	var errValue ast.Expr
	if isAddressable(errorArg) {
		// Direct address-of for addressable expressions
		errValue = &ast.UnaryExpr{
			Op: token.AND,
			X:  errorArg,
		}
		p.ctx.Logger.Debugf("Fix A4: Error expression is addressable, using &expr")
	} else {
		// Non-addressable (literal, function call, etc.) - wrap in IIFE
		errValue = wrapInIIFE(errorArg, errType, p.ctx)
		p.ctx.Logger.Debugf("Fix A4: Error expression is non-addressable, wrapping in IIFE (temp var: __tmp%d)", p.ctx.TempVarCounter-1)
	}

	// Create the replacement CompositeLit
	// Err(error) → Result_T_E{tag: ResultTagErr, err: &error or IIFE}
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(resultTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("tag"),
				Value: ast.NewIdent("ResultTagErr"),
			},
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("err"),
				Value: errValue,
			},
		},
	}

	return replacement
}

// transformOkConstructorWithType transforms Ok[ErrType](value) → Result_T_E{tag: ResultTagOk, ok: &value}
//
// The error type is explicitly provided via the type parameter.
// The ok type is inferred from the value argument.
func (p *ResultTypePlugin) transformOkConstructorWithType(call *ast.CallExpr, errTypeExpr ast.Expr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Ok[E]() expects exactly one argument, found %d", len(call.Args))
		return call
	}

	valueArg := call.Args[0]

	// Infer okType from the value argument
	okType, err := p.inferTypeFromExpr(valueArg)
	if err != nil {
		p.ctx.Logger.Warnf("Type inference failed for Ok[E](%s): %v, using interface{} fallback", FormatExprForDebug(valueArg), err)
		okType = "interface{}"
	}
	if okType == "" {
		okType = "interface{}"
	}

	// Get errType from the explicit type parameter
	errType := p.getTypeName(errTypeExpr)
	if errType == "" {
		errType = "error"
	}

	// Generate unique Result type name
	resultTypeName := fmt.Sprintf("Result%s", SanitizeTypeName(okType, errType))

	// Ensure the Result type is declared
	if !p.emittedTypes[resultTypeName] {
		p.emitResultDeclaration(okType, errType, resultTypeName)
		p.emittedTypes[resultTypeName] = true
	}

	p.ctx.Logger.Debugf("transformOkConstructorWithType: Ok[%s](%s) → %s", errType, FormatExprForDebug(valueArg), resultTypeName)

	// Handle addressability
	var okValue ast.Expr
	if isAddressable(valueArg) {
		okValue = &ast.UnaryExpr{Op: token.AND, X: valueArg}
	} else {
		okValue = wrapInIIFE(valueArg, okType, p.ctx)
	}

	// Create replacement
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(resultTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: ast.NewIdent("ResultTagOk")},
			&ast.KeyValueExpr{Key: ast.NewIdent("ok"), Value: okValue},
		},
	}

	return replacement
}

// transformErrConstructorWithType transforms Err[OkType](error) → Result_T_E{tag: ResultTagErr, err: &error}
//
// The ok type is explicitly provided via the type parameter.
// The error type is inferred from the error argument.
func (p *ResultTypePlugin) transformErrConstructorWithType(call *ast.CallExpr, okTypeExpr ast.Expr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warnf("Err[T]() expects exactly one argument, found %d", len(call.Args))
		return call
	}

	errorArg := call.Args[0]

	// Get okType from the explicit type parameter
	okType := p.getTypeName(okTypeExpr)
	if okType == "" {
		okType = "interface{}"
	}

	// Infer errType from the error argument
	errType, err := p.inferTypeFromExpr(errorArg)
	if err != nil {
		p.ctx.Logger.Warnf("Type inference failed for Err[T](%s): %v, defaulting to 'error'", FormatExprForDebug(errorArg), err)
		errType = "error"
	}
	if errType == "" {
		errType = "error"
	}

	// Generate unique Result type name
	resultTypeName := fmt.Sprintf("Result%s", SanitizeTypeName(okType, errType))

	// Ensure the Result type is declared
	if !p.emittedTypes[resultTypeName] {
		p.emitResultDeclaration(okType, errType, resultTypeName)
		p.emittedTypes[resultTypeName] = true
	}

	p.ctx.Logger.Debugf("transformErrConstructorWithType: Err[%s](%s) → %s", okType, FormatExprForDebug(errorArg), resultTypeName)

	// Handle addressability
	var errValue ast.Expr
	if isAddressable(errorArg) {
		errValue = &ast.UnaryExpr{Op: token.AND, X: errorArg}
	} else {
		errValue = wrapInIIFE(errorArg, errType, p.ctx)
	}

	// Create replacement
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(resultTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: ast.NewIdent("ResultTagErr")},
			&ast.KeyValueExpr{Key: ast.NewIdent("err"), Value: errValue},
		},
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

// emitResultDeclaration generates the Result type declaration and helper methods
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
	if p.ctx == nil {
		return
	}
	// FileSet is only needed for position information (token.NoPos), not for type generation

	// Normalize type names to camelCase (e.g., Option_int → OptionInt)
	// This ensures type references are consistent with generated type names
	okType = NormalizeTypeName(okType)
	errType = NormalizeTypeName(errType)

	// Generate ResultTag enum (only once)
	if !p.emittedTypes["ResultTag"] {
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
			}

			// Check for return statements that need implicit wrapping
			if ret, ok := n.(*ast.ReturnStmt); ok {
				if len(funcStack) > 0 && len(ret.Results) == 1 {
					currentFunc := funcStack[len(funcStack)-1]
					if resultInfo, found := p.funcResultTypes[currentFunc]; found {
						// Check if the return value needs wrapping
						wrapped := p.wrapReturnForResult(ret.Results[0], resultInfo)
						if wrapped != nil {
							ret.Results[0] = wrapped
						}
					}
				}
			}

			// Check if this is a CallExpr we need to transform
			if call, ok := n.(*ast.CallExpr); ok {
				var replacement ast.Expr

				// Case 1: Ok(value) or Err(error) - plain identifier
				if ident, ok := call.Fun.(*ast.Ident); ok {
					switch ident.Name {
					case "Ok":
						replacement = p.transformOkConstructor(call)
					case "Err":
						replacement = p.transformErrConstructor(call)
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
