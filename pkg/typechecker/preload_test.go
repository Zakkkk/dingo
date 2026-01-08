package typechecker

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestPreloadPackage(t *testing.T) {
	src := `package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func main() {
	numbers := []int{1, 2, 3}
	doubled := dgo.Map(numbers, func(x any) any { return x })
	_ = doubled
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Create the importer
	imp := importer.ForCompiler(fset, "gc", nil)

	// PRE-LOAD the dgo package before type-checking
	t.Log("=== Pre-loading dgo package ===")
	dgoPkg, err := imp.Import("github.com/MadAppGang/dingo/pkg/dgo")
	if err != nil {
		t.Logf("Import error (expected in test context): %v", err)
		// Try source importer instead
		imp = importer.ForCompiler(fset, "source", nil)
		dgoPkg, err = imp.Import("github.com/MadAppGang/dingo/pkg/dgo")
		if err != nil {
			t.Logf("Source import also failed: %v", err)
		}
	}

	if dgoPkg != nil {
		t.Logf("dgo package loaded: %s", dgoPkg.Path())
		t.Log("Scope names:")
		for _, name := range dgoPkg.Scope().Names() {
			t.Logf("  - %s", name)
		}

		// Now look up Map directly
		mapObj := dgoPkg.Scope().Lookup("Map")
		if mapObj == nil {
			t.Log("Map not found in preloaded package")
		} else {
			t.Logf("SUCCESS: Map = %v", mapObj.Type())
		}
	}

	// Now type-check the code
	t.Log("\n=== Type-checking code ===")
	conf := types.Config{
		Importer: imp,
		Error: func(err error) {
			// Ignore type errors
		},
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Uses:       make(map[*ast.Ident]types.Object),
		Instances:  make(map[*ast.Ident]types.Instance),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	_, _ = conf.Check("main", fset, []*ast.File{f}, info)

	// Find dgo.Map call
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != "dgo" {
			return true
		}

		found = true
		t.Logf("\n=== Found call: dgo.%s ===", sel.Sel.Name)

		// Test the critical path: package lookup
		pkgObj := info.Uses[pkgIdent]
		if pkgObj == nil {
			t.Log("info.Uses[pkgIdent] returned nil")
			return true
		}

		pkgName, ok := pkgObj.(*types.PkgName)
		if !ok {
			t.Log("pkgObj is not *types.PkgName")
			return true
		}

		pkg := pkgName.Imported()
		t.Logf("Package from info: %s", pkg.Path())

		// Now Scope().Lookup should work!
		funcObj := pkg.Scope().Lookup(sel.Sel.Name)
		if funcObj == nil {
			t.Logf("pkg.Scope().Lookup('%s') returned nil", sel.Sel.Name)
			t.Log("Scope names:")
			for _, name := range pkg.Scope().Names() {
				t.Logf("  - %s", name)
			}
		} else {
			t.Logf("SUCCESS: pkg.Scope().Lookup('%s') = %v", sel.Sel.Name, funcObj.Type())

			// Check if it's generic
			if sig, ok := funcObj.Type().(*types.Signature); ok {
				if sig.TypeParams() != nil && sig.TypeParams().Len() > 0 {
					t.Logf("Generic signature with %d type params", sig.TypeParams().Len())
					for i := 0; i < sig.TypeParams().Len(); i++ {
						t.Logf("  TypeParam[%d]: %s", i, sig.TypeParams().At(i).Obj().Name())
					}
				}
			}
		}

		return true
	})

	if !found {
		t.Log("Did not find dgo.Map call")
	}
}
