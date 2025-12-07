package transpiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// NoneInferenceTransformer rewrites bare None/None() to None[T]()
// by inferring T from context (struct fields, return types, function params, etc.).
type NoneInferenceTransformer struct {
	fset   *token.FileSet
	file   *ast.File
	errors []InferenceError
}

// InferenceError represents a None inference error (e.g., x := None)
type InferenceError struct {
	Pos     token.Pos
	Message string
}

// NewNoneInferenceTransformer creates a new transformer
func NewNoneInferenceTransformer(fset *token.FileSet, file *ast.File) *NoneInferenceTransformer {
	return &NoneInferenceTransformer{
		fset:   fset,
		file:   file,
		errors: nil,
	}
}

// Transform walks the AST and rewrites bare None expressions.
// Returns error if any None usage cannot be inferred.
func (t *NoneInferenceTransformer) Transform() error {
	// Order matters: var decls establish types, then others
	t.transformVarDecls()
	t.transformStructLiterals()
	t.transformCompositeLitElements() // slice/map literals
	t.transformReturns()
	t.transformCallArgs()
	t.transformAssignments()

	// Check for errors last
	t.detectInferenceErrors()

	if t.HasErrors() {
		return fmt.Errorf("None inference errors:\n%s", t.FormatErrors(t.fset))
	}
	return nil
}

// HasErrors returns true if inference errors were found
func (t *NoneInferenceTransformer) HasErrors() bool {
	return len(t.errors) > 0
}

// Errors returns all inference errors
func (t *NoneInferenceTransformer) Errors() []InferenceError {
	return t.errors
}

// FormatErrors returns errors as a formatted string
func (t *NoneInferenceTransformer) FormatErrors(fset *token.FileSet) string {
	var msgs []string
	for _, err := range t.errors {
		pos := fset.Position(err.Pos)
		msgs = append(msgs, fmt.Sprintf("%s:%d:%d: %s", pos.Filename, pos.Line, pos.Column, err.Message))
	}
	return strings.Join(msgs, "\n")
}

// isNoneExpr checks if expr is bare "None" identifier
func isNoneExpr(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "None"
	}
	return false
}

// isNoneCall checks if expr is "None()" call without type param
func isNoneCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	// Check for None() - identifier form
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return ident.Name == "None" && len(call.Args) == 0
	}
	return false
}

// isNoneAny checks for either None or None()
func isNoneAny(expr ast.Expr) bool {
	return isNoneExpr(expr) || isNoneCall(expr)
}

// extractOptionTypeParam extracts T from Option[T] or dgo.Option[T]
func extractOptionTypeParam(expr ast.Expr) ast.Expr {
	idx, ok := expr.(*ast.IndexExpr)
	if !ok {
		return nil
	}
	// Check if it's Option[T]
	if ident, ok := idx.X.(*ast.Ident); ok {
		if ident.Name == "Option" {
			return idx.Index
		}
	}
	// Check if it's dgo.Option[T]
	if sel, ok := idx.X.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "dgo" && sel.Sel.Name == "Option" {
				return idx.Index
			}
		}
	}
	return nil
}

// createTypedNoneCall creates None[T]() call expression
func createTypedNoneCall(typeParam ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun: &ast.IndexExpr{
			X:     ast.NewIdent("None"),
			Index: typeParam,
		},
		Args: []ast.Expr{},
	}
}

// transformStructLiterals handles: Config{Language: None}
func (t *NoneInferenceTransformer) transformStructLiterals() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Get struct type definition
		structType := t.resolveStructType(compLit.Type)
		if structType == nil {
			return true
		}

		for _, elt := range compLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			if !isNoneAny(kv.Value) {
				continue
			}

			// Get field type from struct definition
			fieldName := kv.Key.(*ast.Ident).Name
			fieldType := t.getFieldType(structType, fieldName)
			if fieldType == nil {
				continue
			}

			typeParam := extractOptionTypeParam(fieldType)
			if typeParam != nil {
				kv.Value = createTypedNoneCall(typeParam)
			}
		}
		return true
	})
}

// transformCompositeLitElements handles: []Option[T]{None} and map[K]Option[V]{"key": None}
func (t *NoneInferenceTransformer) transformCompositeLitElements() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Extract element type from slice or map
		var elementType ast.Expr

		switch typeExpr := compLit.Type.(type) {
		case *ast.ArrayType:
			// []Option[T] or [N]Option[T]
			elementType = typeExpr.Elt
		case *ast.MapType:
			// map[K]Option[V]
			elementType = typeExpr.Value
		default:
			return true
		}

		// Extract T from Option[T]
		typeParam := extractOptionTypeParam(elementType)
		if typeParam == nil {
			return true
		}

		// Transform each element
		for i, elt := range compLit.Elts {
			switch elem := elt.(type) {
			case *ast.KeyValueExpr:
				// Map element: "key": None
				if isNoneAny(elem.Value) {
					elem.Value = createTypedNoneCall(typeParam)
				}
			default:
				// Slice/array element: None
				if isNoneAny(elt) {
					compLit.Elts[i] = createTypedNoneCall(typeParam)
				}
			}
		}
		return true
	})
}

// resolveStructType resolves a type expression to its struct definition
func (t *NoneInferenceTransformer) resolveStructType(typeExpr ast.Expr) *ast.StructType {
	// Handle direct struct type
	if st, ok := typeExpr.(*ast.StructType); ok {
		return st
	}

	// Handle named type - look up in file
	ident, ok := typeExpr.(*ast.Ident)
	if !ok {
		return nil
	}

	// Search for type definition in file
	for _, decl := range t.file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != ident.Name {
				continue
			}
			if st, ok := typeSpec.Type.(*ast.StructType); ok {
				return st
			}
		}
	}
	return nil
}

// getFieldType gets the type of a field from a struct definition
func (t *NoneInferenceTransformer) getFieldType(st *ast.StructType, fieldName string) ast.Expr {
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			if name.Name == fieldName {
				return field.Type
			}
		}
	}
	return nil
}

// transformVarDecls handles: var x Option[string] = None
func (t *NoneInferenceTransformer) transformVarDecls() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			return true
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || valueSpec.Type == nil {
				continue
			}

			typeParam := extractOptionTypeParam(valueSpec.Type)
			if typeParam == nil {
				continue
			}

			for i, val := range valueSpec.Values {
				if isNoneAny(val) {
					valueSpec.Values[i] = createTypedNoneCall(typeParam)
				}
			}
		}
		return true
	})
}

// transformReturns handles: func f() Option[T] { return None }
// Includes support for generic functions: func GetValue[T any]() Option[T]
func (t *NoneInferenceTransformer) transformReturns() {
	// Track enclosing function for return type
	var currentFunc *ast.FuncDecl
	var currentFuncLit *ast.FuncLit

	ast.Inspect(t.file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			currentFunc = node
			currentFuncLit = nil
		case *ast.FuncLit:
			currentFuncLit = node
		case *ast.ReturnStmt:
			var funcType *ast.FuncType
			if currentFuncLit != nil {
				funcType = currentFuncLit.Type
			} else if currentFunc != nil {
				funcType = currentFunc.Type
			}

			if funcType == nil || funcType.Results == nil {
				return true
			}

			// Match return values to result types
			resultIdx := 0
			for _, resultField := range funcType.Results.List {
				numNames := len(resultField.Names)
				if numNames == 0 {
					numNames = 1 // Unnamed result
				}

				for i := 0; i < numNames; i++ {
					if resultIdx < len(node.Results) {
						if isNoneAny(node.Results[resultIdx]) {
							typeParam := extractOptionTypeParam(resultField.Type)
							if typeParam != nil {
								node.Results[resultIdx] = createTypedNoneCall(typeParam)
							}
						}
					}
					resultIdx++
				}
			}
		}
		return true
	})
}

// transformCallArgs handles: foo(None) where func foo(x Option[T])
func (t *NoneInferenceTransformer) transformCallArgs() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcType := t.resolveFuncType(call.Fun)
		if funcType == nil || funcType.Params == nil {
			return true
		}

		paramIdx := 0
		for _, param := range funcType.Params.List {
			numNames := len(param.Names)
			if numNames == 0 {
				numNames = 1 // Unnamed parameter
			}

			for i := 0; i < numNames; i++ {
				if paramIdx < len(call.Args) {
					if isNoneAny(call.Args[paramIdx]) {
						typeParam := extractOptionTypeParam(param.Type)
						if typeParam != nil {
							call.Args[paramIdx] = createTypedNoneCall(typeParam)
						}
					}
				}
				paramIdx++
			}
		}
		return true
	})
}

// resolveFuncType resolves a function call to its type signature
func (t *NoneInferenceTransformer) resolveFuncType(fun ast.Expr) *ast.FuncType {
	// Handle direct identifier (local function)
	if ident, ok := fun.(*ast.Ident); ok {
		for _, decl := range t.file.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				if funcDecl.Name.Name == ident.Name {
					return funcDecl.Type
				}
			}
		}
	}
	return nil
}

// transformAssignments handles: x = None where x is Option[T]
// Note: This requires type information from prior declarations
func (t *NoneInferenceTransformer) transformAssignments() {
	// Build map of variable types from declarations
	varTypes := make(map[string]ast.Expr)

	// Collect from var declarations
	ast.Inspect(t.file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok && valueSpec.Type != nil {
					for _, name := range valueSpec.Names {
						varTypes[name.Name] = valueSpec.Type
					}
				}
			}
		}
		return true
	})

	// Transform assignments
	ast.Inspect(t.file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Tok != token.ASSIGN {
			return true
		}

		for i, rhs := range assign.Rhs {
			if !isNoneAny(rhs) {
				continue
			}

			if i >= len(assign.Lhs) {
				continue
			}

			// Get LHS variable name
			if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
				if varType, ok := varTypes[ident.Name]; ok {
					typeParam := extractOptionTypeParam(varType)
					if typeParam != nil {
						assign.Rhs[i] = createTypedNoneCall(typeParam)
					}
				}
			}
		}
		return true
	})
}

// detectInferenceErrors finds None usages that cannot be inferred
func (t *NoneInferenceTransformer) detectInferenceErrors() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		// Check short declarations: x := None
		if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
			for _, rhs := range assign.Rhs {
				if isNoneAny(rhs) {
					t.errors = append(t.errors, InferenceError{
						Pos:     rhs.Pos(),
						Message: "cannot infer Option type for None in short declaration, use None[T]() or declare variable type",
					})
				}
			}
		}
		return true
	})
}
