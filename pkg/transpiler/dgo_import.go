package transpiler

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

const DgoImportPath = "github.com/MadAppGang/dingo/pkg/dgo"

// isResultOrOption checks if an expression is Result or Option (qualified or unqualified).
// Handles both unqualified (Result, Option) and qualified (dgo.Result, pkg.Option) forms.
func isResultOrOption(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		// Unqualified: Result, Option
		return e.Name == "Result" || e.Name == "Option"
	case *ast.SelectorExpr:
		// Qualified: dgo.Result, pkg.Option
		return e.Sel.Name == "Result" || e.Sel.Name == "Option"
	}
	return false
}

// DetectDgoTypes scans Go AST for Result[T,E] or Option[T] usage.
// Returns true if the file contains references to these types and needs the dgo import.
// Supports both qualified (dgo.Result) and unqualified (Result) forms.
func DetectDgoTypes(file *ast.File) bool {
	needsImport := false
	ast.Inspect(file, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.IndexExpr:
			// Option[T] - single type parameter (Go 1.18+ syntax)
			if isResultOrOption(expr.X) {
				needsImport = true
				return false
			}
		case *ast.IndexListExpr:
			// Result[T, E] - two type parameters (Go 1.18+ syntax)
			if isResultOrOption(expr.X) {
				needsImport = true
				return false
			}
		}
		return true
	})
	return needsImport
}

// InjectDgoImport adds the dgo import to the file if Result/Option types are detected,
// and qualifies all unqualified Result/Option references with dgo prefix.
// Returns true if the import was added, false otherwise (already imported or not needed).
func InjectDgoImport(fset *token.FileSet, file *ast.File) bool {
	if !DetectDgoTypes(file) {
		return false
	}

	// Check if dgo is already imported
	alreadyImported := false
	for _, imp := range file.Imports {
		if imp.Path.Value == `"`+DgoImportPath+`"` {
			alreadyImported = true
			break
		}
	}

	// Add import using astutil if not present
	if !alreadyImported {
		added := astutil.AddImport(fset, file, DgoImportPath)
		if !added {
			// Import might already exist with different qualifier or other edge case
			// This is not an error, just log for debugging
		}
	}

	// Qualify all unqualified Result/Option references with dgo prefix
	// This replaces the type alias approach which breaks multi-file packages
	QualifyDingoTypes(file)

	return !alreadyImported
}

// dgoConstructors are the constructor functions that should be qualified with dgo.
var dgoConstructors = map[string]bool{
	"Some": true,
	"None": true,
	"Ok":   true,
	"Err":  true,
}

// dgoTypes are the type names that should be qualified with dgo.
var dgoTypes = map[string]bool{
	"Result": true,
	"Option": true,
}

// QualifyDingoTypes rewrites all unqualified Result/Option references to dgo.Result/dgo.Option,
// and all unqualified Some/None/Ok/Err constructors to dgo.Some/dgo.None/dgo.Ok/dgo.Err.
// This approach avoids type alias redeclaration errors in multi-file packages.
//
// Transforms:
//   - Result[T, E] → dgo.Result[T, E]
//   - Option[T] → dgo.Option[T]
//   - Some[T](v) → dgo.Some[T](v)
//   - Some(v) → dgo.Some(v)
//   - None[T]() → dgo.None[T]()
//   - Ok[T, E](v) → dgo.Ok[T, E](v)
//   - Err[T](e) → dgo.Err[T](e)
//   - Skips already qualified references (dgo.Result stays as-is)
func QualifyDingoTypes(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IndexExpr:
			// Generic with single type parameter: Option[T], Some[T], None[T], Err[T]
			if ident, ok := node.X.(*ast.Ident); ok {
				if dgoTypes[ident.Name] || dgoConstructors[ident.Name] {
					// Replace with dgo.Name
					node.X = &ast.SelectorExpr{
						X:   ast.NewIdent("dgo"),
						Sel: ast.NewIdent(ident.Name),
					}
				}
			}
		case *ast.IndexListExpr:
			// Generic with multiple type parameters: Result[T, E], Ok[T, E]
			if ident, ok := node.X.(*ast.Ident); ok {
				if dgoTypes[ident.Name] || dgoConstructors[ident.Name] {
					// Replace with dgo.Name
					node.X = &ast.SelectorExpr{
						X:   ast.NewIdent("dgo"),
						Sel: ast.NewIdent(ident.Name),
					}
				}
			}
		case *ast.CallExpr:
			// Non-generic constructor calls: Some(v), Ok(v), Err(e)
			if ident, ok := node.Fun.(*ast.Ident); ok {
				if dgoConstructors[ident.Name] {
					// Replace with dgo.Name
					node.Fun = &ast.SelectorExpr{
						X:   ast.NewIdent("dgo"),
						Sel: ast.NewIdent(ident.Name),
					}
				}
			}
		}
		return true
	})
}
