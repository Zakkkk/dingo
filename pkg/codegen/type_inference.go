package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
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

// findEnclosingFunctionFallback uses simple line-based scanning when go/parser fails
func findEnclosingFunctionFallback(src []byte, exprPos int) *ast.FuncDecl {
	// Find "func" keyword before exprPos
	// Scan backward from exprPos to find the func signature
	srcStr := string(src[:exprPos])

	// Find last occurrence of "func " before exprPos
	lastFunc := strings.LastIndex(srcStr, "func ")
	if lastFunc == -1 {
		return nil
	}

	// Extract from "func" to the opening brace or exprPos
	funcDecl := srcStr[lastFunc:]

	// Find the signature - everything up to the opening brace
	// Look for ) { or just { pattern
	braceIdx := strings.Index(funcDecl, "{")
	if braceIdx != -1 {
		// Extract up to (but not including) the {
		funcDecl = strings.TrimSpace(funcDecl[:braceIdx])
	}

	// Now parse just the signature
	fset := token.NewFileSet()
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

// zeroValueFor returns the zero value for a given type name
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
	default:
		// Pointer, slice, map, interface, chan, func
		if strings.HasPrefix(typeName, "*") ||
			strings.HasPrefix(typeName, "[]") ||
			strings.HasPrefix(typeName, "map[") ||
			strings.HasPrefix(typeName, "chan") ||
			typeName == "interface{}" || typeName == "any" ||
			strings.HasPrefix(typeName, "func") {
			return "nil"
		}
		// Named types from standard library that are nil-able
		if strings.Contains(typeName, ".") {
			// Likely a package.Type - assume nil
			return "nil"
		}
		// Struct or named type - use zero value literal
		return typeName + "{}"
	}
}
