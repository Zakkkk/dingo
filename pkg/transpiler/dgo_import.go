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

// QualifyDingoTypes rewrites all unqualified Result/Option references to dgo.Result/dgo.Option.
// This approach avoids type alias redeclaration errors in multi-file packages.
//
// Transforms:
//   - Result[T, E] → dgo.Result[T, E]
//   - Option[T] → dgo.Option[T]
//   - Skips already qualified references (dgo.Result stays as-is)
func QualifyDingoTypes(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IndexExpr:
			// Option[T] - single type parameter
			if ident, ok := node.X.(*ast.Ident); ok {
				if ident.Name == "Result" || ident.Name == "Option" {
					// Replace Result with dgo.Result, Option with dgo.Option
					node.X = &ast.SelectorExpr{
						X:   ast.NewIdent("dgo"),
						Sel: ast.NewIdent(ident.Name),
					}
				}
			}
		case *ast.IndexListExpr:
			// Result[T, E] - two type parameters
			if ident, ok := node.X.(*ast.Ident); ok {
				if ident.Name == "Result" || ident.Name == "Option" {
					// Replace Result with dgo.Result, Option with dgo.Option
					node.X = &ast.SelectorExpr{
						X:   ast.NewIdent("dgo"),
						Sel: ast.NewIdent(ident.Name),
					}
				}
			}
		}
		return true
	})
}
