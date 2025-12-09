package registry

import (
	"fmt"
	"go/ast"
	"strings"
	"sync"

	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// TypeRegistry is the central interface for managing type information during AST traversal.
// It provides scope-aware tracking of variables, functions, and type queries for Dingo's
// monadic types (Result[T,E], Option[T]).
type TypeRegistry interface {
	// Scope Management

	// EnterScope creates and enters a new lexical scope
	EnterScope(name string)

	// ExitScope returns to the parent scope
	ExitScope() error

	// CurrentScope returns the currently active scope
	CurrentScope() *Scope

	// GetScopeLevel returns the current scope nesting depth
	GetScopeLevel() int

	// Variable Registration & Lookup

	// RegisterVariable adds a variable to the current scope
	RegisterVariable(info VariableInfo)

	// GetVariable looks up a variable in the current scope and all parent scopes
	GetVariable(name string) (VariableInfo, bool)

	// UpdateVariable updates an existing variable's type information
	UpdateVariable(name string, typeInfo TypeInfo) error

	// Function Registration & Lookup

	// RegisterFunction adds a function signature to the registry
	RegisterFunction(info FunctionInfo)

	// GetFunction looks up a function signature
	GetFunction(name string) (FunctionInfo, bool)

	// LookupMethod looks up a method call pattern (e.g., "rows.Scan" → FunctionInfo)
	// Handles variable name to type normalization using VariableTypeHints
	LookupMethod(receiver, method string) (FunctionInfo, bool)

	// Type Queries

	// IsResult returns true if the given expression is a Result[T,E] type
	IsResult(expr string) bool

	// IsOption returns true if the given expression is an Option[T] type
	IsOption(expr string) bool

	// IsMonadic returns true if the given expression is Result or Option
	IsMonadic(expr string) bool

	// GetResultTypes extracts T and E from Result[T,E]
	GetResultTypes(expr string) (valueType, errorType string, ok bool)

	// GetOptionType extracts T from Option[T]
	GetOptionType(expr string) (valueType string, ok bool)

	// GetExprType attempts to determine the type of an expression
	GetExprType(expr ast.Expr) (TypeInfo, bool)

	// Utilities

	// Reset clears all state and reinitializes with a package scope
	Reset(packageName string)

	// String returns a human-readable representation of the registry state
	String() string
}

// DefaultRegistry is a concrete implementation of TypeRegistry
type DefaultRegistry struct {
	scopeManager *ScopeManager

	// expressionCache caches type information for complex expressions
	// Key is the string representation of the expression
	expressionCache map[string]TypeInfo
	cacheMu         sync.RWMutex // Protects expressionCache

	// packageName is the current package being processed
	packageName string
}

// NewRegistry creates a new DefaultRegistry initialized with a package scope
func NewRegistry(packageName string) TypeRegistry {
	return &DefaultRegistry{
		scopeManager:    NewScopeManager(packageName),
		expressionCache: make(map[string]TypeInfo),
		packageName:     packageName,
	}
}

// EnterScope creates and enters a new lexical scope
func (r *DefaultRegistry) EnterScope(name string) {
	r.scopeManager.EnterScope(name)
}

// ExitScope returns to the parent scope
func (r *DefaultRegistry) ExitScope() error {
	return r.scopeManager.ExitScope()
}

// CurrentScope returns the currently active scope
func (r *DefaultRegistry) CurrentScope() *Scope {
	return r.scopeManager.CurrentScope()
}

// GetScopeLevel returns the current scope nesting depth
func (r *DefaultRegistry) GetScopeLevel() int {
	return r.scopeManager.GetScopeLevel()
}

// RegisterVariable adds a variable to the current scope
func (r *DefaultRegistry) RegisterVariable(info VariableInfo) {
	r.scopeManager.Register(info)
}

// GetVariable looks up a variable in the current scope and all parent scopes
func (r *DefaultRegistry) GetVariable(name string) (VariableInfo, bool) {
	return r.scopeManager.Lookup(name)
}

// UpdateVariable updates an existing variable's type information
func (r *DefaultRegistry) UpdateVariable(name string, typeInfo TypeInfo) error {
	varInfo, ok := r.GetVariable(name)
	if !ok {
		return fmt.Errorf("variable %s not found in any scope", name)
	}

	varInfo.Type = typeInfo
	r.RegisterVariable(varInfo)
	return nil
}

// RegisterFunction adds a function signature to the registry
func (r *DefaultRegistry) RegisterFunction(info FunctionInfo) {
	r.scopeManager.RegisterFunction(info)
}

// GetFunction looks up a function signature
func (r *DefaultRegistry) GetFunction(name string) (FunctionInfo, bool) {
	return r.scopeManager.LookupFunction(name)
}

// LookupMethod looks up a method call pattern (e.g., "rows.Scan" → FunctionInfo)
// It checks registered functions (dynamically loaded via typeloader).
func (r *DefaultRegistry) LookupMethod(receiver, method string) (FunctionInfo, bool) {
	// Try exact match in registered functions (e.g., "Rows.Scan")
	pattern := receiver + "." + method
	if info, ok := r.GetFunction(pattern); ok {
		return info, true
	}

	// Try with capitalized receiver (e.g., "rows" → "Rows")
	if len(receiver) > 0 {
		capitalizedReceiver := strings.ToUpper(receiver[:1]) + receiver[1:]
		pattern = capitalizedReceiver + "." + method
		if info, ok := r.GetFunction(pattern); ok {
			return info, true
		}
	}

	return FunctionInfo{}, false
}

// IsResult returns true if the given expression is a Result[T,E] type
func (r *DefaultRegistry) IsResult(expr string) bool {
	// Direct variable lookup
	if varInfo, ok := r.GetVariable(expr); ok {
		return varInfo.Type.IsResult()
	}

	// Check expression cache
	r.cacheMu.RLock()
	typeInfo, ok := r.expressionCache[expr]
	r.cacheMu.RUnlock()
	if ok {
		return typeInfo.IsResult()
	}

	// Check if the expression string looks like a Result type
	return strings.HasPrefix(expr, "Result<") || strings.Contains(expr, "result.Result")
}

// IsOption returns true if the given expression is an Option[T] type
func (r *DefaultRegistry) IsOption(expr string) bool {
	// Direct variable lookup
	if varInfo, ok := r.GetVariable(expr); ok {
		return varInfo.Type.IsOption()
	}

	// Check expression cache
	r.cacheMu.RLock()
	typeInfo, ok := r.expressionCache[expr]
	r.cacheMu.RUnlock()
	if ok {
		return typeInfo.IsOption()
	}

	// Check if the expression string looks like an Option type
	return strings.HasPrefix(expr, "Option<") || strings.Contains(expr, "option.Option")
}

// IsMonadic returns true if the given expression is Result or Option
func (r *DefaultRegistry) IsMonadic(expr string) bool {
	return r.IsResult(expr) || r.IsOption(expr)
}

// GetResultTypes extracts T and E from Result[T,E]
func (r *DefaultRegistry) GetResultTypes(expr string) (valueType, errorType string, ok bool) {
	// Try variable lookup first
	if varInfo, found := r.GetVariable(expr); found && varInfo.Type.IsResult() {
		return varInfo.Type.ValueType, varInfo.Type.ErrorType, true
	}

	// Try expression cache
	r.cacheMu.RLock()
	typeInfo, found := r.expressionCache[expr]
	r.cacheMu.RUnlock()
	if found && typeInfo.IsResult() {
		return typeInfo.ValueType, typeInfo.ErrorType, true
	}

	// Parse from string representation "Result[T, E]"
	if strings.HasPrefix(expr, "Result[") && strings.HasSuffix(expr, "]") {
		inner := expr[len("Result<") : len(expr)-1]
		parts := strings.Split(inner, ",")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
		} else if len(parts) == 1 {
			return strings.TrimSpace(parts[0]), "error", true
		}
	}

	return "", "", false
}

// GetOptionType extracts T from Option[T]
func (r *DefaultRegistry) GetOptionType(expr string) (valueType string, ok bool) {
	// Try variable lookup first
	if varInfo, found := r.GetVariable(expr); found && varInfo.Type.IsOption() {
		return varInfo.Type.ValueType, true
	}

	// Try expression cache
	r.cacheMu.RLock()
	typeInfo, found := r.expressionCache[expr]
	r.cacheMu.RUnlock()
	if found && typeInfo.IsOption() {
		return typeInfo.ValueType, true
	}

	// Parse from string representation "Option[T]"
	if strings.HasPrefix(expr, "Option[") && strings.HasSuffix(expr, "]") {
		inner := expr[len("Option<") : len(expr)-1]
		return strings.TrimSpace(inner), true
	}

	return "", false
}

// GetExprType attempts to determine the type of an AST expression
func (r *DefaultRegistry) GetExprType(expr ast.Expr) (TypeInfo, bool) {
	switch e := expr.(type) {
	case *ast.Ident:
		// Variable reference
		if varInfo, ok := r.GetVariable(e.Name); ok {
			return varInfo.Type, true
		}
		// Could be a type name
		return TypeInfo{Kind: TypeKindNamed, Name: e.Name}, false

	case *ast.SelectorExpr:
		// Package.Type or value.Method
		exprStr := formatExpr(expr)
		r.cacheMu.RLock()
		typeInfo, ok := r.expressionCache[exprStr]
		r.cacheMu.RUnlock()
		if ok {
			return typeInfo, true
		}
		return TypeInfo{Kind: TypeKindUnknown, Name: exprStr}, false

	case *ast.CallExpr:
		// Function call - try to get return type
		if funIdent, ok := e.Fun.(*ast.Ident); ok {
			if funInfo, ok := r.GetFunction(funIdent.Name); ok && len(funInfo.Results) > 0 {
				return funInfo.Results[0], true
			}
		}
		return TypeInfo{Kind: TypeKindUnknown}, false

	case *ast.IndexExpr:
		// Generic type instantiation (Go 1.18+) or array/slice access
		exprStr := formatExpr(expr)
		r.cacheMu.RLock()
		typeInfo, ok := r.expressionCache[exprStr]
		r.cacheMu.RUnlock()
		if ok {
			return typeInfo, true
		}
		return TypeInfo{Kind: TypeKindUnknown, Name: exprStr}, false

	default:
		return TypeInfo{Kind: TypeKindUnknown}, false
	}
}

// Reset clears all state and reinitializes with a package scope
func (r *DefaultRegistry) Reset(packageName string) {
	r.scopeManager.Reset(packageName)
	r.cacheMu.Lock()
	r.expressionCache = make(map[string]TypeInfo)
	r.cacheMu.Unlock()
	r.packageName = packageName
}

// PopulateFromLoadResult registers functions, methods, and local functions from a LoadResult
func (r *DefaultRegistry) PopulateFromLoadResult(result *typeloader.LoadResult) {
	// Register all Functions (e.g., "os.Open", "json.Marshal")
	for name, sig := range result.Functions {
		funcInfo := convertFunctionSignature(sig)
		r.RegisterFunction(funcInfo)

		// Also register with qualified name if package is present
		if sig.Package != "" {
			qualifiedName := sig.Package + "." + sig.Name
			if qualifiedName != name {
				r.RegisterFunction(funcInfo)
			}
		}
	}

	// Register all Methods (e.g., "File.Close", "Rows.Scan")
	for _, sig := range result.Methods {
		funcInfo := convertFunctionSignature(sig)
		// Methods are registered with Type.Method pattern
		r.RegisterFunction(funcInfo)
	}

	// Register all LocalFunctions (e.g., "getUserData", "processFile")
	for _, sig := range result.LocalFunctions {
		funcInfo := convertFunctionSignature(sig)
		r.RegisterFunction(funcInfo)
	}
}

// convertFunctionSignature converts a typeloader.FunctionSignature to registry.FunctionInfo
func convertFunctionSignature(sig *typeloader.FunctionSignature) FunctionInfo {
	funcInfo := FunctionInfo{
		Name:    sig.Name,
		Package: sig.Package,
	}

	// Convert receiver if present
	if sig.Receiver != nil {
		receiverType := convertTypeRef(*sig.Receiver)
		funcInfo.Receiver = &receiverType
	}

	// Convert parameters
	funcInfo.Parameters = make([]TypeInfo, len(sig.Parameters))
	for i, param := range sig.Parameters {
		funcInfo.Parameters[i] = convertTypeRef(param)
	}

	// Convert results
	funcInfo.Results = make([]TypeInfo, len(sig.Results))
	for i, result := range sig.Results {
		funcInfo.Results[i] = convertTypeRef(result)
	}

	return funcInfo
}

// convertTypeRef converts a typeloader.TypeRef to registry.TypeInfo
func convertTypeRef(ref typeloader.TypeRef) TypeInfo {
	typeInfo := TypeInfo{
		Name:      ref.Name,
		Package:   ref.Package,
		IsPointer: ref.IsPointer,
	}

	// Determine type kind
	if ref.IsError {
		typeInfo.Kind = TypeKindNamed
		typeInfo.Name = "error"
	} else if ref.Package != "" {
		typeInfo.Kind = TypeKindNamed
	} else {
		// Check if it's a basic Go type
		switch ref.Name {
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64", "complex64", "complex128",
			"string", "bool", "byte", "rune":
			typeInfo.Kind = TypeKindBasic
		default:
			typeInfo.Kind = TypeKindNamed
		}
	}

	return typeInfo
}

// String returns a human-readable representation of the registry state
func (r *DefaultRegistry) String() string {
	r.cacheMu.RLock()
	cacheSize := len(r.expressionCache)
	r.cacheMu.RUnlock()
	return fmt.Sprintf("DefaultRegistry{package=%s, %s, cached=%d}",
		r.packageName, r.scopeManager.String(), cacheSize)
}

// CacheExprType stores type information for a complex expression
// This is useful for expressions that don't correspond to simple variables
func (r *DefaultRegistry) CacheExprType(expr string, typeInfo TypeInfo) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	// Simple eviction policy: if cache is too large, clear oldest half
	const maxCacheSize = 1000
	if len(r.expressionCache) >= maxCacheSize {
		// Clear half the cache (simple FIFO-like eviction)
		count := 0
		for k := range r.expressionCache {
			delete(r.expressionCache, k)
			count++
			if count >= maxCacheSize/2 {
				break
			}
		}
	}

	r.expressionCache[expr] = typeInfo
}

// formatExpr converts an AST expression to a string representation
func formatExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return formatExpr(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		return formatExpr(e.Fun) + "()"
	case *ast.IndexExpr:
		return formatExpr(e.X) + "[" + formatExpr(e.Index) + "]"
	default:
		return "expr"
	}
}
