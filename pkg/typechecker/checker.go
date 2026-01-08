// Package typechecker provides go/types integration for Dingo transpilation.
// It runs the Go type checker on transformed Go code to infer types,
// enabling replacement of interface{} placeholders with actual types.
package typechecker

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
)

// Checker runs the Go type checker and provides type queries.
type Checker struct {
	fset *token.FileSet
	info *types.Info
	pkg  *types.Package
}

// cachedImporter is a shared importer that caches import results across type-checks.
// This dramatically speeds up repeated type-checking of files with the same imports.
var cachedImporter types.Importer

func init() {
	// Use "gc" mode (reads pre-compiled .a files) which is much faster.
	// Falls back gracefully for packages not yet compiled.
	cachedImporter = importer.Default()
}

// New creates a new Checker from a parsed Go AST.
// The pkgName should match the package declaration in the file.
// The pkgPath is used for the package's import path (e.g., "main" for standalone files).
func New(fset *token.FileSet, file *ast.File, pkgPath string) (*Checker, error) {
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
		Instances:  make(map[*ast.Ident]types.Instance),
	}

	// Configure the type checker with cached importer
	// Using importer.Default() (gc mode) which reads pre-compiled .a files.
	// This is MUCH faster than "source" mode which parses all imports from source.
	conf := types.Config{
		Importer: cachedImporter,
		Error: func(err error) {
			// Ignore type errors - we want partial type info even with errors
			// This is important because the transformed code may have interface{}
			// placeholders that cause type mismatches
		},
	}

	// Run the type checker
	pkg, _ := conf.Check(pkgPath, fset, []*ast.File{file}, info)
	// Ignore check errors - we still get partial type info

	return &Checker{
		fset: fset,
		info: info,
		pkg:  pkg,
	}, nil
}

// TypeOf returns the type of an expression, or nil if unknown.
func (c *Checker) TypeOf(expr ast.Expr) types.Type {
	if tv, ok := c.info.Types[expr]; ok {
		return tv.Type
	}
	return nil
}

// ObjectOf returns the object that an identifier refers to, or nil if unknown.
func (c *Checker) ObjectOf(id *ast.Ident) types.Object {
	if obj := c.info.Defs[id]; obj != nil {
		return obj
	}
	return c.info.Uses[id]
}

// SelectionOf returns selection info for a selector expression (x.f).
func (c *Checker) SelectionOf(sel *ast.SelectorExpr) *types.Selection {
	return c.info.Selections[sel]
}

// Info returns the raw types.Info for advanced queries.
func (c *Checker) Info() *types.Info {
	return c.info
}

// Package returns the type-checked package.
func (c *Checker) Package() *types.Package {
	return c.pkg
}

// FileSet returns the file set used for type checking.
func (c *Checker) FileSet() *token.FileSet {
	return c.fset
}

// TypeString returns a string representation of a type.
func TypeString(t types.Type) string {
	return types.TypeString(t, nil)
}

// UnderlyingType returns the underlying type, unwrapping named types.
func UnderlyingType(t types.Type) types.Type {
	return t.Underlying()
}

// IsPointer returns true if the type is a pointer type.
func IsPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// PointerElem returns the element type of a pointer, or nil if not a pointer.
func PointerElem(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return nil
}

// IsNilable returns true if the type can be nil (pointer, slice, map, chan, interface, func).
func IsNilable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
		return true
	}
	return false
}

// FieldType returns the type of a struct field by name, or nil if not found.
func FieldType(t types.Type, fieldName string) types.Type {
	// Unwrap pointers
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	// Get the struct type
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	// Find the field
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f.Name() == fieldName {
			return f.Type()
		}
	}

	return nil
}

// MethodType returns the signature of a method by name, or nil if not found.
func MethodType(t types.Type, methodName string) *types.Signature {
	// Try to find in method set
	methods := types.NewMethodSet(t)
	for i := 0; i < methods.Len(); i++ {
		m := methods.At(i)
		if m.Obj().Name() == methodName {
			if sig, ok := m.Type().(*types.Signature); ok {
				return sig
			}
		}
	}

	// Also try pointer to type
	ptrMethods := types.NewMethodSet(types.NewPointer(t))
	for i := 0; i < ptrMethods.Len(); i++ {
		m := ptrMethods.At(i)
		if m.Obj().Name() == methodName {
			if sig, ok := m.Type().(*types.Signature); ok {
				return sig
			}
		}
	}

	return nil
}

// ResultType returns the return type of a function/method signature.
// For multiple return values, returns a tuple type.
// For no return value, returns nil.
func ResultType(sig *types.Signature) types.Type {
	results := sig.Results()
	if results.Len() == 0 {
		return nil
	}
	if results.Len() == 1 {
		return results.At(0).Type()
	}
	// Multiple return values - return tuple
	return results
}
