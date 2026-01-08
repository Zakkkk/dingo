package codegen

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"

	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// TypeResolver provides type information for expressions using go/types.
// It handles cross-file and cross-package type resolution.
//
// This resolver is optional - if it fails to load types, callers should
// fall back to existing local-only search behavior.
type TypeResolver struct {
	fset       *token.FileSet
	file       *ast.File
	typeInfo   *types.Info
	pkg        *types.Package
	loadResult *typeloader.LoadResult
}

// NewTypeResolver creates a resolver from Dingo source.
//
// Parameters:
//   - src: Dingo source code (will be sanitized internally)
//   - workingDir: Directory for resolving imports (usually project root)
//
// Returns error if source cannot be parsed. Type checking errors are
// silently ignored - we want partial type info even if some expressions fail.
//
// NOTE: The sanitization here is NOT byte-based code transformation (forbidden by CLAUDE.md).
// It's a preprocessing step to make Dingo source parseable by go/parser.
// All actual type analysis is done via go/ast and go/types (token-based).
func NewTypeResolver(src []byte, workingDir string) (*TypeResolver, error) {
	// Step 1: Sanitize Dingo syntax for go/parser
	// Replace '?' with space to make source parseable
	// This preserves byte positions while making valid Go
	sanitized := sanitizeDingoSource(src)

	// Step 2: Parse with go/parser to get AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", sanitized, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse sanitized source: %w", err)
	}

	// Step 3: Extract imports from AST
	imports := extractImportsFromAST(file)

	// Step 4: Load external package types using typeloader
	// Make working directory absolute for go/packages
	if workingDir == "" || workingDir == "." {
		workingDir, _ = filepath.Abs(".")
	} else {
		workingDir, _ = filepath.Abs(workingDir)
	}

	loader := typeloader.NewLoader(typeloader.LoaderConfig{
		WorkingDir: workingDir,
		FailFast:   false, // Don't fail on first error
	})

	loadResult, loadErr := loader.LoadFromImports(imports)
	// Non-fatal: we can still do local type checking even if imports fail
	if loadErr != nil {
		loadResult = nil
	}

	// Step 5: Run go/types type checker
	typeInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	config := types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
		Error: func(err error) {
			// Ignore errors - we want partial type info
			// Dingo syntax may cause type mismatches
		},
	}

	pkg, _ := config.Check("main", fset, []*ast.File{file}, typeInfo)
	// Ignore check errors - we still get partial type info

	return &TypeResolver{
		fset:       fset,
		file:       file,
		typeInfo:   typeInfo,
		pkg:        pkg,
		loadResult: loadResult,
	}, nil
}

// GetReturnCount returns the number of return values for a call expression.
// Returns -1 if unable to determine (caller should use fallback).
//
// This method attempts multiple strategies:
// 1. Look up the call expression in go/types info
// 2. Fall back to typeloader results for imported packages
// 3. Return -1 if all attempts fail
func (r *TypeResolver) GetReturnCount(exprBytes []byte) int {
	if r == nil {
		return -1
	}

	// Strategy 1: Parse the expression and look it up in type info
	exprAST, err := parser.ParseExpr(string(exprBytes))
	if err != nil {
		return -1
	}

	callExpr, ok := exprAST.(*ast.CallExpr)
	if !ok {
		return -1
	}

	// Try to find a matching call expression in our type-checked file
	var matchedType types.Type
	ast.Inspect(r.file, func(n ast.Node) bool {
		if matchedType != nil {
			return false // Already found
		}
		if e, ok := n.(*ast.CallExpr); ok {
			// Compare expressions structurally
			if exprsEqual(e.Fun, callExpr.Fun) {
				if tv, found := r.typeInfo.Types[e]; found {
					matchedType = tv.Type
					return false
				}
			}
		}
		return true
	})

	if matchedType != nil {
		return countReturnsFromType(matchedType)
	}

	// Strategy 2: Try typeloader for imported package methods
	if r.loadResult != nil {
		methodName := extractMethodFromCallExpr(callExpr)
		if methodName != "" {
			// Check methods (Type.Method)
			if sig, found := r.loadResult.Methods[methodName]; found {
				return len(sig.Results)
			}
			// Check package functions (package.Function)
			if sig, found := r.loadResult.Functions[methodName]; found {
				return len(sig.Results)
			}
			// Check local functions
			if sig, found := r.loadResult.LocalFunctions[methodName]; found {
				return len(sig.Results)
			}
		}
	}

	return -1
}

// sanitizeDingoSource replaces Dingo-specific syntax with valid Go.
// Currently handles: ? -> space (for error propagation)
//
// This is NOT byte-based code transformation - it's preprocessing to make
// Dingo source parseable by go/parser. All actual analysis is done via
// go/ast and go/types.
func sanitizeDingoSource(src []byte) []byte {
	sanitized := make([]byte, len(src))
	copy(sanitized, src)

	for i := 0; i < len(sanitized); i++ {
		if sanitized[i] == '?' {
			sanitized[i] = ' '
		}
	}

	return sanitized
}

// extractImportsFromAST extracts import paths from an AST.
func extractImportsFromAST(file *ast.File) []string {
	var imports []string
	for _, imp := range file.Imports {
		// Remove quotes from import path
		path := imp.Path.Value
		if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
			path = path[1 : len(path)-1]
		}
		imports = append(imports, path)
	}
	return imports
}

// exprsEqual compares two expressions structurally (simplified).
func exprsEqual(a, b ast.Expr) bool {
	// Compare string representations for matching
	return exprToString(a) == exprToString(b)
}

// exprToString converts an expression to a comparable string.
func exprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprToString(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return exprToString(x.Fun) + "()"
	case *ast.IndexExpr:
		// Generic type: foo[T]
		return exprToString(x.X) + "[" + exprToString(x.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple params: foo[T, E]
		result := exprToString(x.X) + "["
		for i, idx := range x.Indices {
			if i > 0 {
				result += ", "
			}
			result += exprToString(idx)
		}
		result += "]"
		return result
	default:
		return fmt.Sprintf("%T", e)
	}
}

// countReturnsFromType extracts return count from a type.
func countReturnsFromType(t types.Type) int {
	switch typ := t.(type) {
	case *types.Tuple:
		return typ.Len()
	case *types.Named:
		// Single named return type
		return 1
	case *types.Basic:
		// Single basic return type
		return 1
	case *types.Pointer:
		// Single pointer return type
		return 1
	case *types.Slice:
		// Single slice return type
		return 1
	case *types.Interface:
		// Single interface return type (including error)
		return 1
	default:
		// For other types, assume single return
		if t != nil {
			return 1
		}
		return -1
	}
}

// extractMethodFromCallExpr extracts the method/function name from a call expression.
// Returns qualified name like "Type.Method" or "pkg.Function" for lookups.
func extractMethodFromCallExpr(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Simple function call: foo()
		return fun.Name
	case *ast.SelectorExpr:
		// Method or package function: obj.Method() or pkg.Func()
		switch x := fun.X.(type) {
		case *ast.Ident:
			// pkg.Func or receiver.Method
			return x.Name + "." + fun.Sel.Name
		case *ast.SelectorExpr:
			// Chained access: a.b.Method - return last receiver.method
			return extractLastIdent(x) + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	case *ast.IndexExpr:
		// Generic function: foo[T]()
		return extractMethodFromCallExpr(&ast.CallExpr{Fun: fun.X})
	case *ast.IndexListExpr:
		// Generic function with multiple type params: foo[T, E]()
		return extractMethodFromCallExpr(&ast.CallExpr{Fun: fun.X})
	}
	return ""
}

// extractLastIdent extracts the last identifier from a selector chain.
func extractLastIdent(sel *ast.SelectorExpr) string {
	switch x := sel.X.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return sel.Sel.Name
	}
	return sel.Sel.Name
}
