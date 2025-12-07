package typechecker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
)

// ExprTypeCache stores detected types for expressions.
// Key: expression string (e.g., "config.Database.Host")
// Value: type string (e.g., "*string")
type ExprTypeCache map[string]string

// SourceChecker provides type inference from Go source code.
// Unlike Checker which works with pre-parsed AST, SourceChecker
// handles parsing internally.
type SourceChecker struct {
	checker *Checker
	files   []*ast.File
	pkgPath string
}

// NewSourceChecker creates a new checker that parses source code.
func NewSourceChecker() *SourceChecker {
	return &SourceChecker{}
}

// ParseAndCheck parses Go source and runs type checking.
// The source must be valid Go code.
// Returns an error if parsing fails.
func (sc *SourceChecker) ParseAndCheck(filename string, src []byte) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}
	sc.files = []*ast.File{f}

	// Extract package path from AST or use default
	sc.pkgPath = "main"
	if f.Name != nil {
		sc.pkgPath = f.Name.Name
	}

	// Use existing Checker
	sc.checker, _ = New(fset, f, sc.pkgPath)
	return nil
}

// GetExprType returns the type of an expression given as a string.
// For example: GetExprType("config.Database.Host") might return "*string".
// Returns empty string if expression not found or type unknown.
func (sc *SourceChecker) GetExprType(exprStr string) string {
	if sc.checker == nil || sc.checker.Info() == nil {
		return ""
	}

	// Search for matching expression in the AST
	var resultType string
	for _, f := range sc.files {
		ast.Inspect(f, func(n ast.Node) bool {
			if resultType != "" {
				return false // Already found
			}
			if expr, ok := n.(ast.Expr); ok {
				if formatExprToString(expr) == exprStr {
					if t := sc.checker.TypeOf(expr); t != nil {
						resultType = sc.formatType(t)
					}
				}
			}
			return true
		})
	}
	return resultType
}

// GetAllExprTypes returns types for all selector expressions in the source.
// This is useful for building a type cache for safe-nav transformations.
func (sc *SourceChecker) GetAllExprTypes() ExprTypeCache {
	cache := make(ExprTypeCache)
	if sc.checker == nil || sc.checker.Info() == nil {
		return cache
	}

	for _, f := range sc.files {
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				exprStr := formatExprToString(sel)
				if t := sc.checker.TypeOf(sel); t != nil {
					cache[exprStr] = sc.formatType(t)
				}
			}
			return true
		})
	}
	return cache
}

// formatType converts a types.Type to a clean string representation.
// Removes package prefixes for the current package (e.g., "main.Database" -> "Database")
func (sc *SourceChecker) formatType(t types.Type) string {
	if t == nil {
		return ""
	}

	typeStr := types.TypeString(t, nil)

	// Remove current package prefix
	prefix := sc.pkgPath + "."
	typeStr = strings.ReplaceAll(typeStr, "*"+prefix, "*")
	typeStr = strings.ReplaceAll(typeStr, prefix, "")

	return typeStr
}

// formatExprToString converts an ast.Expr to its string representation.
// Handles identifiers, selectors, and index expressions.
func formatExprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return formatExprToString(x.X) + "." + x.Sel.Name
	case *ast.IndexExpr:
		return formatExprToString(x.X) + "[...]"
	case *ast.CallExpr:
		return formatExprToString(x.Fun) + "(...)"
	case *ast.StarExpr:
		return "*" + formatExprToString(x.X)
	case *ast.ParenExpr:
		return formatExprToString(x.X)
	default:
		return ""
	}
}

// InferSafeNavTypes takes a source file and a list of safe-nav chain expressions,
// and returns the inferred types for each chain.
//
// Example:
//
//	chains := []string{"config.Database.Host", "user.Profile.Name"}
//	types := InferSafeNavTypes(src, chains)
//	// types["config.Database.Host"] = "*string"
//	// types["user.Profile.Name"] = "string"
func InferSafeNavTypes(src []byte, chains []string) ExprTypeCache {
	sc := NewSourceChecker()
	if err := sc.ParseAndCheck("input.go", src); err != nil {
		return make(ExprTypeCache)
	}

	cache := make(ExprTypeCache)
	for _, chain := range chains {
		if t := sc.GetExprType(chain); t != "" {
			cache[chain] = t
		}
	}
	return cache
}

// ChainToExprString converts a safe-nav chain to its equivalent Go expression.
// Example: "config?.Database?.Host" -> "config.Database.Host"
func ChainToExprString(chain string) string {
	return strings.ReplaceAll(chain, "?.", ".")
}
