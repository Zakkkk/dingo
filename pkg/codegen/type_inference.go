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
	default:
		return "interface{}"
	}
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
