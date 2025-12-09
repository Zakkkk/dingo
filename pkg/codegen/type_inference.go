package codegen

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
)

// InferReturnTypes analyzes the source to find the enclosing function's return types
// and returns the corresponding zero values for each return position.
// The last return value is always assumed to be the error being propagated.
func InferReturnTypes(src []byte, exprPos int) []string {
	funcDecl := findEnclosingFunction(src, exprPos)
	if funcDecl == nil {
		return []string{"nil"} // Fallback
	}

	returnTypes := parseReturnTypes(funcDecl)
	if len(returnTypes) == 0 {
		return []string{"nil"}
	}

	// Convert types to zero values (excluding the last one which is err)
	zeroValues := make([]string, len(returnTypes)-1)
	for i := 0; i < len(returnTypes)-1; i++ {
		zeroValues[i] = zeroValueFor(returnTypes[i])
	}

	return zeroValues
}

// InferReturnTypeNames analyzes the source to find the enclosing function's return types
// and returns the actual type names (not zero values).
// Returns nil if unable to determine types.
func InferReturnTypeNames(src []byte, exprPos int) []string {
	funcDecl := findEnclosingFunction(src, exprPos)
	if funcDecl == nil {
		return nil
	}
	return parseReturnTypes(funcDecl)
}

// InferEnclosingFunctionReturnsResult checks if the enclosing function returns a Result type.
// Returns the Result's T type if it does, empty string otherwise.
// For example, for `func foo() Result[User, error]`, returns "User".
func InferEnclosingFunctionReturnsResult(src []byte, exprPos int) string {
	typeNames := InferReturnTypeNames(src, exprPos)
	if len(typeNames) == 0 {
		return ""
	}

	// Check each return type for Result pattern
	for _, typeName := range typeNames {
		if IsResultType(typeName) {
			okType := ExtractResultOkType(typeName)
			if okType != "" {
				return okType
			}
		}
	}
	return ""
}

// findEnclosingFunction finds the function declaration that contains exprPos
func findEnclosingFunction(src []byte, exprPos int) *ast.FuncDecl {
	// First try: parse as complete file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err == nil {
		var enclosing *ast.FuncDecl
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				start := fset.Position(fn.Pos()).Offset
				end := fset.Position(fn.End()).Offset
				if start <= exprPos && exprPos <= end {
					enclosing = fn
				}
			}
			return true
		})
		if enclosing != nil {
			return enclosing
		}
	}

	// Second try: wrap as function body and parse
	wrapped := "package p\n" + string(src)
	fset = token.NewFileSet()
	f, err = parser.ParseFile(fset, "", wrapped, parser.ParseComments)
	if err == nil {
		var enclosing *ast.FuncDecl
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				enclosing = fn
			}
			return true
		})
		if enclosing != nil {
			return enclosing
		}
	}

	// Third try: line-based fallback
	return findEnclosingFunctionFallback(src, exprPos)
}

// findEnclosingFunctionFallback uses go/scanner when go/parser fails.
// This properly tokenizes the source to find the enclosing function declaration.
func findEnclosingFunctionFallback(src []byte, exprPos int) *ast.FuncDecl {
	// Tokenize source up to exprPos using go/scanner
	// Collect FUNC positions along with whether they're named (declarations)
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	type funcInfo struct {
		pos     int
		isNamed bool // true if it's a named function (declaration)
	}
	var funcs []funcInfo

	s.Init(file, src, nil, 0)

	var lastTok token.Token
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		offset := fset.Position(pos).Offset
		if offset > exprPos {
			break // Stop when we pass exprPos
		}
		if lastTok == token.FUNC && tok == token.IDENT {
			// Previous token was FUNC, this is a name → function declaration
			if len(funcs) > 0 {
				funcs[len(funcs)-1].isNamed = true
			}
		}
		if tok == token.FUNC {
			funcs = append(funcs, funcInfo{pos: offset, isNamed: false})
		}
		lastTok = tok
	}

	if len(funcs) == 0 {
		return nil
	}

	// Find the last NAMED function before exprPos (skip anonymous functions)
	var lastFunc int = -1
	for i := len(funcs) - 1; i >= 0; i-- {
		if funcs[i].isNamed {
			lastFunc = funcs[i].pos
			break
		}
	}

	if lastFunc == -1 {
		// No named functions found, try the last function anyway
		lastFunc = funcs[len(funcs)-1].pos
	}

	// Find the opening brace using a new scanner with new file
	remainder := src[lastFunc:]
	fset2 := token.NewFileSet()
	file2 := fset2.AddFile("", fset2.Base(), len(remainder))
	s.Init(file2, remainder, nil, 0)
	braceOffset := -1
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.LBRACE {
			braceOffset = fset2.Position(pos).Offset
			break
		}
	}

	// Extract the function signature
	var funcDecl string
	if braceOffset != -1 {
		funcDecl = string(src[lastFunc : lastFunc+braceOffset])
	} else {
		funcDecl = string(src[lastFunc:exprPos])
	}

	// Parse just the signature
	fset = token.NewFileSet()
	wrapped := funcDecl + " {}"
	f, err := parser.ParseFile(fset, "", "package p\n"+wrapped, 0)
	if err == nil && len(f.Decls) > 0 {
		if fn, ok := f.Decls[0].(*ast.FuncDecl); ok {
			return fn
		}
	}

	return nil
}

// parseReturnTypes extracts return type names from a function declaration
func parseReturnTypes(fn *ast.FuncDecl) []string {
	if fn == nil || fn.Type == nil || fn.Type.Results == nil {
		return nil
	}

	var types []string
	for _, field := range fn.Type.Results.List {
		typeName := exprToTypeName(field.Type)
		// Handle multiple names: (a, b int) counts as 2 returns
		if len(field.Names) > 0 {
			for range field.Names {
				types = append(types, typeName)
			}
		} else {
			types = append(types, typeName)
		}
	}
	return types
}

// exprToTypeName converts an ast.Expr to a type name string
func exprToTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToTypeName(t.X)
	case *ast.ArrayType:
		return "[]" + exprToTypeName(t.Elt)
	case *ast.MapType:
		return "map[" + exprToTypeName(t.Key) + "]" + exprToTypeName(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.SelectorExpr:
		return exprToTypeName(t.X) + "." + t.Sel.Name
	case *ast.ChanType:
		return "chan " + exprToTypeName(t.Value)
	case *ast.FuncType:
		return "func"
	case *ast.IndexExpr:
		// Generic type with single parameter: Result[T]
		return exprToTypeName(t.X) + "[" + exprToTypeName(t.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple parameters: Result[T, E]
		result := exprToTypeName(t.X) + "["
		for i, idx := range t.Indices {
			if i > 0 {
				result += ", "
			}
			result += exprToTypeName(idx)
		}
		result += "]"
		return result
	default:
		return "interface{}"
	}
}

// InferExprReturnCount determines how many values an expression returns.
// Returns 1 for single-return expressions (like row.Scan() returning just error),
// Returns 2 for multi-return expressions (like db.Query() returning (*Rows, error)),
// Returns -1 if detection fails (fallback to multi-return assumption).
//
// This function attempts to type-check the expression by:
// 1. Extracting the function being called from the expression
// 2. Finding the function declaration in the source
// 3. Counting return values from the signature
//
// For external package functions (like sql.Row.Scan), detection may fail
// and the caller should use the fallback value.
func InferExprReturnCount(src []byte, exprBytes []byte, exprPos int) int {
	// Parse the expression using go/parser to extract method name
	exprStr := string(exprBytes)
	methodName := extractMethodName(exprStr)
	if methodName != "" {
		// Check if this method is defined in the current source
		count := findMethodReturnCount(src, methodName)
		if count > 0 {
			return count
		}
	}

	// For external packages, we can't easily determine return count without full type info
	// Return -1 to signal caller should use fallback
	return -1
}

// extractMethodName extracts the method/function name from a call expression
// using go/parser to properly parse the expression AST.
// e.g., "row.Scan(&user.ID)" -> "Scan", "foo()" -> "foo"
func extractMethodName(expr string) string {
	// Parse as Go expression using go/parser
	parsedExpr, err := parser.ParseExpr(expr)
	if err != nil {
		return ""
	}

	// Check if it's a call expression
	callExpr, ok := parsedExpr.(*ast.CallExpr)
	if !ok {
		return ""
	}

	// Extract the function/method name from the call
	return extractFuncName(callExpr.Fun)
}

// extractFuncName extracts the function name from a call's Fun expression
func extractFuncName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		// Simple function call: foo()
		return e.Name
	case *ast.SelectorExpr:
		// Method call: obj.Method() or pkg.Func()
		return e.Sel.Name
	case *ast.IndexExpr:
		// Generic function: foo[T]()
		return extractFuncName(e.X)
	case *ast.IndexListExpr:
		// Generic function with multiple type params: foo[T, U]()
		return extractFuncName(e.X)
	default:
		return ""
	}
}

// findMethodReturnCount searches the source for a function/method with the given name
// and returns its return count, or 0 if not found.
//
// NOTE: This function sanitizes Dingo-specific '?' syntax to allow go/parser to work.
// This is NOT byte-based code transformation - all actual analysis is done via go/ast.
// The sanitization is necessary because we need to parse function signatures that
// exist in the same file as error propagation expressions, and go/parser cannot
// handle the '?' character. The actual return count detection is performed entirely
// through ast.Inspect on the parsed AST.
func findMethodReturnCount(src []byte, methodName string) int {
	// Sanitize source: replace Dingo's '?' with space to allow go/parser to work.
	// This preserves byte positions while making the source valid Go syntax.
	sanitized := make([]byte, len(src))
	copy(sanitized, src)
	for i := 0; i < len(sanitized); i++ {
		if sanitized[i] == '?' {
			sanitized[i] = ' '
		}
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", sanitized, parser.ParseComments)
	if err != nil {
		return 0
	}

	var returnCount int
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if fn.Name.Name == methodName {
			if fn.Type.Results != nil {
				returnCount = fn.Type.Results.NumFields()
			}
			return false
		}
		return true
	})

	return returnCount
}

// IsResultType checks if a type string represents a Result type (dgo.Result or Result)
func IsResultType(typeStr string) bool {
	// Check for dgo.Result[T, E] or Result[T, E]
	for i := 0; i < len(typeStr); i++ {
		if typeStr[i] == '.' {
			// Has package prefix, check for dgo.Result
			remaining := typeStr[i+1:]
			if len(remaining) >= 6 && remaining[:6] == "Result" {
				return true
			}
		}
	}
	// Check for unqualified Result
	if len(typeStr) >= 6 && typeStr[:6] == "Result" {
		return true
	}
	return false
}

// ExtractResultOkType extracts the T from Result[T, E] or dgo.Result[T, E]
// Returns empty string if not a Result type or extraction fails
func ExtractResultOkType(typeStr string) string {
	// Find "Result[" in the string
	resultIdx := -1
	for i := 0; i <= len(typeStr)-7; i++ {
		if typeStr[i:i+7] == "Result[" {
			resultIdx = i + 7
			break
		}
	}
	if resultIdx == -1 {
		return ""
	}

	// Extract until first comma or ]
	depth := 1
	start := resultIdx
	for i := resultIdx; i < len(typeStr); i++ {
		switch typeStr[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return typeStr[start:i]
			}
		case ',':
			if depth == 1 {
				return typeStr[start:i]
			}
		}
	}
	return ""
}

// zeroValueFor returns the zero value for a given type name.
// NOTE: This operates on type name strings from go/ast (parsed data),
// not source code bytes. Simple prefix checks are used instead of strings package.
func zeroValueFor(typeName string) string {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "byte", "rune":
		return "0"
	case "bool":
		return "false"
	case "string":
		return `""`
	case "error":
		return "nil"
	case "interface{}", "any":
		return "nil"
	default:
		// Pointer, slice, map, interface, chan, func
		// Use character checks instead of strings.HasPrefix
		if len(typeName) > 0 && typeName[0] == '*' {
			return "nil" // Pointer
		}
		if len(typeName) >= 2 && typeName[:2] == "[]" {
			return "nil" // Slice
		}
		if len(typeName) >= 4 && typeName[:4] == "map[" {
			return "nil" // Map
		}
		if len(typeName) >= 4 && typeName[:4] == "chan" {
			return "nil" // Channel
		}
		if len(typeName) >= 4 && typeName[:4] == "func" {
			return "nil" // Function
		}
		// Named types from standard library that are nil-able
		// Check for "." in type name (e.g., "http.Client")
		for i := 0; i < len(typeName); i++ {
			if typeName[i] == '.' {
				return "nil" // package.Type - assume nil
			}
		}
		// Struct or named type - use zero value literal
		return typeName + "{}"
	}
}
