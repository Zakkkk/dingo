package preprocessor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// ReturnInfo contains information about the return values of a function call
type ReturnInfo struct {
	Count       int      // Number of return values
	Types       []string // Type names of each return value
	LastIsError bool     // True if last return is error type
	ErrorOnly   bool     // True if single return and it's error
}

// ReturnDetector analyzes call expressions and determines return value information
// using go/types for accurate type inference
type ReturnDetector struct {
	analyzer   *TypeAnalyzer
	cache      map[string]*ReturnInfo      // Expression -> cached result
	loadResult *typeloader.LoadResult      // Type information from typeloader
}

// NewReturnDetector creates a new ReturnDetector with the given TypeAnalyzer
func NewReturnDetector(analyzer *TypeAnalyzer) *ReturnDetector {
	return &ReturnDetector{
		analyzer:   analyzer,
		cache:      make(map[string]*ReturnInfo),
		loadResult: nil,
	}
}

// SetLoadResult sets the typeloader LoadResult for function signature lookups
func (rd *ReturnDetector) SetLoadResult(result *typeloader.LoadResult) {
	rd.loadResult = result
}

// DetectReturns analyzes a call expression and returns information about its return values
// expr: the call expression (e.g., "rows.Scan(&a, &b)", "os.Open(path)")
// sourceCode: complete source code context for parsing (optional, can be empty for simple cases)
// Returns ReturnInfo or error if detection fails
func (rd *ReturnDetector) DetectReturns(expr string, sourceCode string) (*ReturnInfo, error) {
	// Check cache first
	if cached, ok := rd.cache[expr]; ok {
		return cached, nil
	}

	// Parse the expression
	callExpr, err := rd.parseCallExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expression: %w", err)
	}

	var info *ReturnInfo

	// Try type-based analysis if analyzer is available
	if rd.analyzer != nil && rd.analyzer.HasTypeInfo() {
		returnTypes, err := rd.analyzeCallExpr(callExpr)
		if err == nil {
			info = rd.buildReturnInfo(returnTypes)
		}
	}

	// Fall back to heuristics if type analysis failed or unavailable
	if info == nil {
		info = rd.detectByHeuristics(callExpr)
	}

	// Cache the result
	rd.cache[expr] = info

	return info, nil
}

// parseCallExpr parses a string expression into an AST CallExpr node
func (rd *ReturnDetector) parseCallExpr(expr string) (*ast.CallExpr, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}

	// Parse as Go expression
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Verify it's a call expression
	callExpr, ok := node.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("expression is not a function call (got %T)", node)
	}

	return callExpr, nil
}

// analyzeCallExpr analyzes a CallExpr and returns the types of its return values
func (rd *ReturnDetector) analyzeCallExpr(callExpr *ast.CallExpr) ([]types.Type, error) {
	if rd.analyzer == nil || !rd.analyzer.HasTypeInfo() {
		return nil, fmt.Errorf("type analyzer not initialized or missing type info")
	}

	// Determine what kind of call this is
	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		// Simple function call: foo()
		return rd.analyzeFunctionCall(fun.Name)

	case *ast.SelectorExpr:
		// Method or package function call: obj.Method() or pkg.Func()
		return rd.analyzeSelectorCall(fun)

	default:
		return nil, fmt.Errorf("unsupported call type: %T", fun)
	}
}

// analyzeFunctionCall analyzes a simple function call (identifier only)
func (rd *ReturnDetector) analyzeFunctionCall(funcName string) ([]types.Type, error) {
	// Get the function type from the analyzer
	funcType, ok := rd.analyzer.TypeOf(funcName)
	if !ok {
		return nil, fmt.Errorf("function '%s' not found in scope", funcName)
	}

	// Extract signature
	sig, ok := funcType.(*types.Signature)
	if !ok {
		return nil, fmt.Errorf("'%s' is not a function (got %T)", funcName, funcType)
	}

	return rd.extractReturnTypes(sig), nil
}

// analyzeSelectorCall analyzes a selector call (method or package function)
func (rd *ReturnDetector) analyzeSelectorCall(sel *ast.SelectorExpr) ([]types.Type, error) {
	// Try to get the type of the selector base (X)
	var baseType types.Type
	var ok bool

	switch x := sel.X.(type) {
	case *ast.Ident:
		// Simple identifier: obj.Method() or pkg.Func()
		baseType, ok = rd.analyzer.TypeOf(x.Name)
		if !ok {
			return nil, fmt.Errorf("type of '%s' unknown", x.Name)
		}

		// Check if this is a package reference (for package functions)
		if named, isNamed := baseType.(*types.Named); isNamed {
			if pkg := named.Obj().Pkg(); pkg != nil {
				// This might be a package function call
				// Try to look up the function in the package scope
				if scope := pkg.Scope(); scope != nil {
					if obj := scope.Lookup(sel.Sel.Name); obj != nil {
						if fn, isFn := obj.(*types.Func); isFn {
							sig := fn.Type().(*types.Signature)
							return rd.extractReturnTypes(sig), nil
						}
					}
				}
			}
		}

	default:
		// Complex expression (e.g., chained calls)
		return nil, fmt.Errorf("complex selector expressions not yet supported")
	}

	// This is a method call - get the full signature to get all return types
	sig, err := rd.getMethodSignature(baseType, sel.Sel.Name)
	if err != nil {
		return nil, err
	}

	return rd.extractReturnTypes(sig), nil
}

// getMethodSignature retrieves the full signature of a method
func (rd *ReturnDetector) getMethodSignature(baseType types.Type, methodName string) (*types.Signature, error) {
	// Get the method set for this type
	methodSet := types.NewMethodSet(baseType)
	for i := 0; i < methodSet.Len(); i++ {
		sel := methodSet.At(i)
		if sel.Obj().Name() == methodName {
			// Get method signature
			if sig, ok := sel.Type().(*types.Signature); ok {
				return sig, nil
			}
		}
	}

	return nil, fmt.Errorf("method '%s' not found on type '%s'", methodName, rd.analyzer.TypeName(baseType))
}

// extractReturnTypes extracts the return types from a function signature
func (rd *ReturnDetector) extractReturnTypes(sig *types.Signature) []types.Type {
	results := sig.Results()
	if results == nil || results.Len() == 0 {
		return nil
	}

	returnTypes := make([]types.Type, results.Len())
	for i := 0; i < results.Len(); i++ {
		returnTypes[i] = results.At(i).Type()
	}

	return returnTypes
}

// buildReturnInfo constructs a ReturnInfo from a list of return types
func (rd *ReturnDetector) buildReturnInfo(returnTypes []types.Type) *ReturnInfo {
	if len(returnTypes) == 0 {
		return &ReturnInfo{
			Count:       0,
			Types:       []string{},
			LastIsError: false,
			ErrorOnly:   false,
		}
	}

	info := &ReturnInfo{
		Count: len(returnTypes),
		Types: make([]string, len(returnTypes)),
	}

	// Convert types to strings
	for i, typ := range returnTypes {
		info.Types[i] = rd.analyzer.TypeName(typ)
	}

	// Check if last return is error
	lastType := returnTypes[len(returnTypes)-1]
	info.LastIsError = rd.isErrorType(lastType)

	// Check if it's error-only (single return value that is error)
	info.ErrorOnly = len(returnTypes) == 1 && info.LastIsError

	return info
}

// isErrorType checks if a type is the error interface
func (rd *ReturnDetector) isErrorType(typ types.Type) bool {
	// Get the underlying type name
	typeName := rd.analyzer.TypeName(typ)

	// Direct check for "error"
	if typeName == "error" {
		return true
	}

	// Check if it's a named type with name "error"
	if named, ok := typ.(*types.Named); ok {
		return named.Obj().Name() == "error"
	}

	return false
}

// ClearCache clears the return info cache
func (rd *ReturnDetector) ClearCache() {
	rd.cache = make(map[string]*ReturnInfo)
}

// detectByHeuristics uses pattern matching for common standard library functions
// This is a fallback when type analysis is unavailable
func (rd *ReturnDetector) detectByHeuristics(callExpr *ast.CallExpr) *ReturnInfo {
	// Extract the pattern first to check loadResult
	pattern := rd.extractPattern(callExpr)

	// Try loadResult first if available
	if rd.loadResult != nil && pattern != "" {
		if info := rd.lookupInLoadResult(pattern); info != nil {
			return info
		}
	}

	// Fall back to hardcoded patterns for backward compatibility
	// Map of known patterns: "receiver.method" or "package.function" -> ReturnInfo
	knownPatterns := map[string]*ReturnInfo{
		// database/sql - error-only returns
		"rows.Scan":   {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"rows.Close":  {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"row.Scan":    {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"db.Ping":     {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"tx.Commit":   {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"tx.Rollback": {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},

		// database/sql - (value, error) returns
		"db.Query":     {Count: 2, Types: []string{"*sql.Rows", "error"}, LastIsError: true, ErrorOnly: false},
		"db.QueryRow":  {Count: 1, Types: []string{"*sql.Row"}, LastIsError: false, ErrorOnly: false},
		"db.Exec":      {Count: 2, Types: []string{"sql.Result", "error"}, LastIsError: true, ErrorOnly: false},
		"db.Prepare":   {Count: 2, Types: []string{"*sql.Stmt", "error"}, LastIsError: true, ErrorOnly: false},
		"db.Begin":     {Count: 2, Types: []string{"*sql.Tx", "error"}, LastIsError: true, ErrorOnly: false},
		"stmt.Query":   {Count: 2, Types: []string{"*sql.Rows", "error"}, LastIsError: true, ErrorOnly: false},
		"stmt.QueryRow": {Count: 1, Types: []string{"*sql.Row"}, LastIsError: false, ErrorOnly: false},
		"stmt.Exec":    {Count: 2, Types: []string{"sql.Result", "error"}, LastIsError: true, ErrorOnly: false},

		// os package - (value, error) returns
		"os.Open":       {Count: 2, Types: []string{"*os.File", "error"}, LastIsError: true, ErrorOnly: false},
		"os.Create":     {Count: 2, Types: []string{"*os.File", "error"}, LastIsError: true, ErrorOnly: false},
		"os.ReadFile":   {Count: 2, Types: []string{"[]byte", "error"}, LastIsError: true, ErrorOnly: false},
		"os.WriteFile":  {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"os.Stat":       {Count: 2, Types: []string{"os.FileInfo", "error"}, LastIsError: true, ErrorOnly: false},
		"os.Remove":     {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"os.RemoveAll":  {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"os.Mkdir":      {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"os.MkdirAll":   {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"os.Chdir":      {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"os.Getwd":      {Count: 2, Types: []string{"string", "error"}, LastIsError: true, ErrorOnly: false},
		"file.Close":    {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
		"file.Write":    {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"file.Read":     {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"file.Seek":     {Count: 2, Types: []string{"int64", "error"}, LastIsError: true, ErrorOnly: false},
		"file.Stat":     {Count: 2, Types: []string{"os.FileInfo", "error"}, LastIsError: true, ErrorOnly: false},

		// io package
		"io.Copy":      {Count: 2, Types: []string{"int64", "error"}, LastIsError: true, ErrorOnly: false},
		"io.ReadAll":   {Count: 2, Types: []string{"[]byte", "error"}, LastIsError: true, ErrorOnly: false},
		"io.WriteString": {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"reader.Read":  {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"writer.Write": {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},

		// fmt package
		"fmt.Println":  {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"fmt.Printf":   {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"fmt.Fprintf":  {Count: 2, Types: []string{"int", "error"}, LastIsError: true, ErrorOnly: false},
		"fmt.Sprintf":  {Count: 1, Types: []string{"string"}, LastIsError: false, ErrorOnly: false},

		// json package
		"json.Marshal":   {Count: 2, Types: []string{"[]byte", "error"}, LastIsError: true, ErrorOnly: false},
		"json.Unmarshal": {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},

		// http package
		"http.Get":           {Count: 2, Types: []string{"*http.Response", "error"}, LastIsError: true, ErrorOnly: false},
		"http.Post":          {Count: 2, Types: []string{"*http.Response", "error"}, LastIsError: true, ErrorOnly: false},
		"http.NewRequest":    {Count: 2, Types: []string{"*http.Request", "error"}, LastIsError: true, ErrorOnly: false},
		"client.Do":          {Count: 2, Types: []string{"*http.Response", "error"}, LastIsError: true, ErrorOnly: false},
		"response.Body.Close": {Count: 1, Types: []string{"error"}, LastIsError: true, ErrorOnly: true},
	}

	// Look up in known patterns (pattern already extracted earlier)
	if info, ok := knownPatterns[pattern]; ok {
		return info
	}

	// Default: assume (T, error) for unknown patterns
	return &ReturnInfo{
		Count:       2,
		Types:       []string{"T", "error"},
		LastIsError: true,
		ErrorOnly:   false,
	}
}

// extractPattern extracts the call pattern from a CallExpr
// Returns patterns like "Func", "receiver.Method", or "pkg.Function"
func (rd *ReturnDetector) extractPattern(callExpr *ast.CallExpr) string {
	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		// Simple function call: Func()
		return fun.Name

	case *ast.SelectorExpr:
		// Method or package function call: receiver.Method() or pkg.Func()
		if ident, ok := fun.X.(*ast.Ident); ok {
			return ident.Name + "." + fun.Sel.Name
		} else if sel, ok := fun.X.(*ast.SelectorExpr); ok {
			// Chained selector (e.g., response.Body.Close)
			if baseIdent, ok := sel.X.(*ast.Ident); ok {
				return baseIdent.Name + "." + sel.Sel.Name + "." + fun.Sel.Name
			}
		}
	}

	return ""
}

// lookupInLoadResult looks up a function signature in the LoadResult
// Checks Functions, Methods, and LocalFunctions maps
func (rd *ReturnDetector) lookupInLoadResult(pattern string) *ReturnInfo {
	// Try each map in priority order
	var sig *typeloader.FunctionSignature

	// 1. Try Functions map (for "pkg.Func" patterns)
	if s, ok := rd.loadResult.Functions[pattern]; ok {
		sig = s
	}

	// 2. Try Methods map (for "Type.Method" patterns)
	if sig == nil {
		if s, ok := rd.loadResult.Methods[pattern]; ok {
			sig = s
		}
	}

	// 3. Try LocalFunctions map (for simple "Func" patterns)
	if sig == nil {
		if s, ok := rd.loadResult.LocalFunctions[pattern]; ok {
			sig = s
		}
	}

	// If not found in any map, return nil
	if sig == nil {
		return nil
	}

	// Convert FunctionSignature to ReturnInfo
	return rd.signatureToReturnInfo(sig)
}

// signatureToReturnInfo converts a typeloader.FunctionSignature to ReturnInfo
func (rd *ReturnDetector) signatureToReturnInfo(sig *typeloader.FunctionSignature) *ReturnInfo {
	if sig == nil || len(sig.Results) == 0 {
		return &ReturnInfo{
			Count:       0,
			Types:       []string{},
			LastIsError: false,
			ErrorOnly:   false,
		}
	}

	info := &ReturnInfo{
		Count: len(sig.Results),
		Types: make([]string, len(sig.Results)),
	}

	// Convert TypeRefs to type strings
	for i, result := range sig.Results {
		typeName := result.Name
		if result.IsPointer {
			typeName = "*" + typeName
		}
		info.Types[i] = typeName
	}

	// Check if last return is error
	lastResult := sig.Results[len(sig.Results)-1]
	info.LastIsError = lastResult.IsError

	// Check if it's error-only (single return value that is error)
	info.ErrorOnly = len(sig.Results) == 1 && info.LastIsError

	return info
}
