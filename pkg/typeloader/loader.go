package typeloader

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Loader loads type information from Go packages
type Loader struct {
	config LoaderConfig
}

// NewLoader creates a new type loader
func NewLoader(config LoaderConfig) *Loader {
	// Default to fail-fast behavior
	if !config.FailFast {
		config.FailFast = true
	}
	return &Loader{
		config: config,
	}
}

// LoadFromImports loads type information for the given import paths
// Returns error immediately if any package fails to load (fail-fast)
func (l *Loader) LoadFromImports(imports []string) (*LoadResult, error) {
	if len(imports) == 0 {
		return &LoadResult{
			Functions:      make(map[string]*FunctionSignature),
			Methods:        make(map[string]*FunctionSignature),
			LocalFunctions: make(map[string]*FunctionSignature),
		}, nil
	}

	// Deduplicate imports
	importSet := make(map[string]bool)
	for _, imp := range imports {
		if imp != "" {
			importSet[imp] = true
		}
	}

	uniqueImports := make([]string, 0, len(importSet))
	for imp := range importSet {
		uniqueImports = append(uniqueImports, imp)
	}

	if len(uniqueImports) == 0 {
		return &LoadResult{
			Functions:      make(map[string]*FunctionSignature),
			Methods:        make(map[string]*FunctionSignature),
			LocalFunctions: make(map[string]*FunctionSignature),
		}, nil
	}

	// Configure go/packages
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName | packages.NeedImports,
		Dir:  l.config.WorkingDir,
	}

	if len(l.config.BuildTags) > 0 {
		cfg.BuildFlags = []string{"-tags", joinBuildTags(l.config.BuildTags)}
	}

	// Load all packages in one call (efficient batch loading)
	pkgs, err := packages.Load(cfg, uniqueImports...)
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w\n\n"+
			"Troubleshooting:\n"+
			"  1. Ensure you're running from a directory with go.mod\n"+
			"  2. Run 'go mod download' to fetch dependencies\n"+
			"  3. Verify import paths are correct", err)
	}

	result := &LoadResult{
		Functions:      make(map[string]*FunctionSignature),
		Methods:        make(map[string]*FunctionSignature),
		LocalFunctions: make(map[string]*FunctionSignature),
	}

	// Process each loaded package - FAIL FAST on any error
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Fail fast: return error immediately with clear message
			return nil, fmt.Errorf("failed to load package %q: %v\n\n"+
				"Troubleshooting:\n"+
				"  1. Run 'go mod download' to fetch dependencies\n"+
				"  2. Ensure the package compiles: 'go build ./...'\n"+
				"  3. Check that the import path is correct\n"+
				"  4. If using CGO, ensure CGO_ENABLED=1",
				pkg.PkgPath, pkg.Errors[0])
		}

		if pkg.Types == nil {
			return nil, fmt.Errorf("package %q has no type information\n\n"+
				"This usually indicates a build configuration issue.\n"+
				"Try: go clean -cache && go build ./...",
				pkg.PkgPath)
		}

		l.extractFromPackage(pkg, result)
	}

	return result, nil
}

// LoadWithLocalFuncs loads both imported packages and local function definitions
func (l *Loader) LoadWithLocalFuncs(source []byte, imports []string) (*LoadResult, error) {
	// Load imported packages
	result, err := l.LoadFromImports(imports)
	if err != nil {
		return nil, err
	}

	// Parse local function declarations from Dingo source
	parser := &LocalFuncParser{}
	localFuncs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse local functions: %w", err)
	}

	// Merge local functions into result
	for name, sig := range localFuncs {
		result.LocalFunctions[name] = sig
	}

	return result, nil
}

// extractFromPackage extracts all exported functions and methods from a package
func (l *Loader) extractFromPackage(pkg *packages.Package, result *LoadResult) {
	scope := pkg.Types.Scope()
	pkgPath := pkg.PkgPath

	// Extract all exported names from package scope
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil || !obj.Exported() {
			continue
		}

		switch obj := obj.(type) {
		case *types.Func:
			// Package-level function
			sig := l.funcToSignature(obj, pkgPath)
			if sig != nil {
				key := pkgPath + "." + name
				result.Functions[key] = sig
			}

		case *types.TypeName:
			// Named type - extract methods
			l.extractMethods(obj.Type(), pkgPath, result)
		}
	}
}

// extractMethods extracts all methods from a named type
func (l *Loader) extractMethods(typ types.Type, pkgPath string, result *LoadResult) {
	// Get method set for both value and pointer receivers
	mset := types.NewMethodSet(typ)
	for i := 0; i < mset.Len(); i++ {
		method := mset.At(i)
		obj := method.Obj()

		if !obj.Exported() {
			continue
		}

		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}

		sig := l.funcToSignature(fn, pkgPath)
		if sig != nil {
			// Extract receiver type name
			if named, ok := typ.(*types.Named); ok {
				receiverName := named.Obj().Name()
				key := receiverName + "." + obj.Name()
				result.Methods[key] = sig
			}
		}
	}

	// Also check pointer type methods
	ptrType := types.NewPointer(typ)
	ptrMset := types.NewMethodSet(ptrType)
	for i := 0; i < ptrMset.Len(); i++ {
		method := ptrMset.At(i)
		obj := method.Obj()

		if !obj.Exported() {
			continue
		}

		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}

		sig := l.funcToSignature(fn, pkgPath)
		if sig != nil {
			// Extract receiver type name
			if named, ok := typ.(*types.Named); ok {
				receiverName := named.Obj().Name()
				key := receiverName + "." + obj.Name()
				// Only add if not already present (avoid duplicates)
				if _, exists := result.Methods[key]; !exists {
					result.Methods[key] = sig
				}
			}
		}
	}
}

// funcToSignature converts a go/types function to FunctionSignature
func (l *Loader) funcToSignature(fn *types.Func, pkgPath string) *FunctionSignature {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil
	}

	funcSig := &FunctionSignature{
		Name:       fn.Name(),
		Package:    pkgPath,
		Parameters: make([]TypeRef, 0),
		Results:    make([]TypeRef, 0),
	}

	// Extract receiver (for methods)
	if recv := sig.Recv(); recv != nil {
		funcSig.Receiver = l.typeToRef(recv.Type())
	}

	// Extract parameters
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		funcSig.Parameters = append(funcSig.Parameters, *l.typeToRef(param.Type()))
	}

	// Extract results
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		result := results.At(i)
		funcSig.Results = append(funcSig.Results, *l.typeToRef(result.Type()))
	}

	return funcSig
}

// typeToRef converts a go/types Type to TypeRef
func (l *Loader) typeToRef(typ types.Type) *TypeRef {
	ref := &TypeRef{}

	// Handle pointer types
	if ptr, ok := typ.(*types.Pointer); ok {
		ref.IsPointer = true
		typ = ptr.Elem()
	}

	// Extract type information
	switch t := typ.(type) {
	case *types.Named:
		obj := t.Obj()
		ref.Name = obj.Name()
		if pkg := obj.Pkg(); pkg != nil {
			ref.Package = pkg.Path()
		}

		// Check if this is the error interface
		if ref.Package == "" && ref.Name == "error" {
			ref.IsError = true
		}

	case *types.Interface:
		// Check if implements error interface
		errorType := types.Universe.Lookup("error")
		if errorType != nil {
			errorIface, ok := errorType.Type().Underlying().(*types.Interface)
			if ok && types.Implements(t, errorIface) {
				ref.IsError = true
			}
		}
		// Also check by name for the error interface itself
		if t.String() == "error" {
			ref.Name = "error"
			ref.IsError = true
		} else {
			ref.Name = t.String()
		}

	case *types.Basic:
		ref.Name = t.Name()

	case *types.Slice:
		ref.Name = "[]" + l.typeToRef(t.Elem()).Name

	case *types.Array:
		ref.Name = fmt.Sprintf("[%d]%s", t.Len(), l.typeToRef(t.Elem()).Name)

	case *types.Map:
		keyRef := l.typeToRef(t.Key())
		valRef := l.typeToRef(t.Elem())
		ref.Name = fmt.Sprintf("map[%s]%s", keyRef.Name, valRef.Name)

	case *types.Chan:
		elemRef := l.typeToRef(t.Elem())
		switch t.Dir() {
		case types.SendRecv:
			ref.Name = "chan " + elemRef.Name
		case types.SendOnly:
			ref.Name = "chan<- " + elemRef.Name
		case types.RecvOnly:
			ref.Name = "<-chan " + elemRef.Name
		}

	default:
		// Preserve string representation for debugging
		ref.Name = fmt.Sprintf("complex(%s)", typ.String())
	}

	return ref
}

// joinBuildTags joins build tags into a comma-separated string
func joinBuildTags(tags []string) string {
	return strings.Join(tags, ",")
}
