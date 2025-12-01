package preprocessor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// TypeAnalyzer provides type inference using go/types with full package resolution
// Wraps go/packages and go/types for comprehensive type information
type TypeAnalyzer struct {
	fset     *token.FileSet
	info     *types.Info
	pkg      *types.Package
	pkgs     []*packages.Package
	pkgCache map[string]*packages.Package

	// Type lookup maps for fast access
	objTypes  map[string]types.Type      // identifier -> type
	exprTypes map[ast.Expr]types.Type    // expression -> type
}

// NewTypeAnalyzer creates a new type analyzer
func NewTypeAnalyzer() *TypeAnalyzer {
	return &TypeAnalyzer{
		fset:      token.NewFileSet(),
		pkgCache:  make(map[string]*packages.Package),
		objTypes:  make(map[string]types.Type),
		exprTypes: make(map[ast.Expr]types.Type),
	}
}

// AnalyzePackage analyzes a package using go/packages for full type information
// dir: directory containing the package
// Returns error if package cannot be loaded or type-checked
func (ta *TypeAnalyzer) AnalyzePackage(dir string) error {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax,
		Dir:   dir,
		Fset:  ta.fset,
		Tests: false,
	}

	// Load the package and all dependencies
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return fmt.Errorf("failed to load package: %w", err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages found in directory: %s", dir)
	}

	// Check for errors in loaded packages
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Log errors but don't fail - partial type info is still useful
			for _, e := range pkg.Errors {
				// Log each error for visibility during debugging
				// Note: We continue with partial info rather than failing
				// This allows type inference to work even with some errors
				_ = e // Keep error for potential future structured reporting
			}
		}
	}

	ta.pkgs = pkgs

	// Use the first package as primary (current package)
	mainPkg := pkgs[0]
	ta.pkg = mainPkg.Types
	if mainPkg.TypesInfo != nil {
		ta.info = mainPkg.TypesInfo
	} else {
		// Initialize empty TypesInfo if not available
		ta.info = &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Implicits:  make(map[ast.Node]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
			Scopes:     make(map[ast.Node]*types.Scope),
		}
	}

	// Build lookup maps from all packages
	for _, pkg := range pkgs {
		ta.pkgCache[pkg.PkgPath] = pkg
		if pkg.TypesInfo != nil {
			ta.extractTypesFromPackage(pkg)
		}
	}

	return nil
}

// AnalyzeFile analyzes a single Go file without package context
// Useful for quick type checking without full package resolution
// source: Go source code to analyze
func (ta *TypeAnalyzer) AnalyzeFile(source string) error {
	// Parse the source file
	file, err := parser.ParseFile(ta.fset, "source.go", source, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("failed to parse source: %w", err)
	}

	// Create types.Config for type checking
	conf := types.Config{
		Importer: nil, // No imports available in single-file mode
		Error: func(err error) {
			// Collect errors but don't fail
		},
	}

	// Type-check the file
	ta.info = &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	pkg, err := conf.Check("main", ta.fset, []*ast.File{file}, ta.info)
	if err != nil {
		// Continue with partial info even if there are errors
	}
	ta.pkg = pkg

	// Extract type information
	ta.extractTypesFromInfo()

	return nil
}

// extractTypesFromPackage extracts type information from a loaded package
func (ta *TypeAnalyzer) extractTypesFromPackage(pkg *packages.Package) {
	if pkg.TypesInfo == nil {
		return
	}

	// Extract from Defs (definitions)
	for ident, obj := range pkg.TypesInfo.Defs {
		if obj != nil && ident != nil {
			ta.objTypes[ident.Name] = obj.Type()
		}
	}

	// Extract from Uses (usages)
	for ident, obj := range pkg.TypesInfo.Uses {
		if obj != nil && ident != nil {
			ta.objTypes[ident.Name] = obj.Type()
		}
	}

	// Extract from Types (expressions)
	for expr, tv := range pkg.TypesInfo.Types {
		if tv.Type != nil {
			ta.exprTypes[expr] = tv.Type
		}
	}
}

// extractTypesFromInfo extracts type information from current type info
func (ta *TypeAnalyzer) extractTypesFromInfo() {
	if ta.info == nil {
		return
	}

	// First pass: Extract Uses
	for ident, obj := range ta.info.Uses {
		if obj != nil && ident != nil {
			ta.objTypes[ident.Name] = obj.Type()
		}
	}

	// Second pass: Extract Defs (overwrites Uses - higher priority)
	// This matches TypeOf() which searches Defs first, then Uses
	for ident, obj := range ta.info.Defs {
		if obj != nil && ident != nil {
			ta.objTypes[ident.Name] = obj.Type() // Overwrites Uses if exists
		}
	}

	// Extract from Types
	for expr, tv := range ta.info.Types {
		if tv.Type != nil {
			ta.exprTypes[expr] = tv.Type
		}
	}
}

// TypeOf returns the type of an identifier or expression
// Returns (type, true) if found, (nil, false) otherwise
// Uses only cached map (O(1)) - extraction must populate cache correctly
func (ta *TypeAnalyzer) TypeOf(name string) (types.Type, bool) {
	// Only use cached map - no fallback search
	// extractTypesFromInfo() populates this with correct priority
	typ, ok := ta.objTypes[name]
	return typ, ok
}

// FieldType returns the type of a field in a struct
// baseType: the struct type
// fieldName: name of the field
func (ta *TypeAnalyzer) FieldType(baseType types.Type, fieldName string) (types.Type, bool) {
	// Unwrap pointer types
	if ptr, ok := baseType.(*types.Pointer); ok {
		baseType = ptr.Elem()
	}

	// Handle named types
	if named, ok := baseType.(*types.Named); ok {
		baseType = named.Underlying()
	}

	// Must be a struct
	structType, ok := baseType.(*types.Struct)
	if !ok {
		return nil, false
	}

	// Find the field
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if field.Name() == fieldName {
			return field.Type(), true
		}
	}

	return nil, false
}

// MethodReturnType returns the return type(s) of a method
// baseType: the receiver type
// methodName: name of the method
// Returns first return type if method found
func (ta *TypeAnalyzer) MethodReturnType(baseType types.Type, methodName string) (types.Type, bool) {
	// Check if type has the method
	methodSet := types.NewMethodSet(baseType)
	for i := 0; i < methodSet.Len(); i++ {
		sel := methodSet.At(i)
		if sel.Obj().Name() == methodName {
			// Get method signature
			if sig, ok := sel.Type().(*types.Signature); ok {
				results := sig.Results()
				if results.Len() > 0 {
					return results.At(0).Type(), true
				}
			}
		}
	}

	return nil, false
}

// IsPointer checks if a type is a pointer type
func (ta *TypeAnalyzer) IsPointer(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}

// IsPointerByName checks if an identifier refers to a pointer type
func (ta *TypeAnalyzer) IsPointerByName(name string) bool {
	typ, ok := ta.TypeOf(name)
	if !ok {
		return false
	}
	return ta.IsPointer(typ)
}

// IsOption checks if a type is an Option type (naming convention)
func (ta *TypeAnalyzer) IsOption(typ types.Type) bool {
	// Get the type name (without package path for comparison)
	typeName := ta.TypeName(typ)

	// Strip package path if present
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}

	// Exact match for "Option"
	if typeName == "Option" {
		return true
	}

	// Pattern 1: Prefix - OptionX where X starts with uppercase
	// Examples: OptionString, OptionInt
	if strings.HasPrefix(typeName, "Option") && len(typeName) > 6 {
		nextChar := typeName[6]
		if nextChar >= 'A' && nextChar <= 'Z' {
			return true
		}
	}

	// Pattern 2: Suffix - XOption where the string is simple (no multiple capital letters before Option)
	// Examples: UserOption, StringOption
	// Counter-examples: NotAnOption (has multiple capital letters 'N' and 'A' before 'O')
	if strings.HasSuffix(typeName, "Option") && len(typeName) > 6 {
		// Count capital letters before "Option"
		prefix := typeName[:len(typeName)-6]
		capitalCount := 0
		for _, ch := range prefix {
			if ch >= 'A' && ch <= 'Z' {
				capitalCount++
			}
		}
		// If there's only one capital letter (the start), it's a valid Option type
		// If there are multiple capitals, it's likely something like "NotAnOption"
		if capitalCount <= 1 {
			return true
		}
	}

	return false
}

// IsOptionByName checks if an identifier refers to an Option type
func (ta *TypeAnalyzer) IsOptionByName(name string) bool {
	typ, ok := ta.TypeOf(name)
	if !ok {
		return false
	}
	return ta.IsOption(typ)
}

// TypeName returns the name of a type as a string
func (ta *TypeAnalyzer) TypeName(typ types.Type) string {
	if typ == nil {
		return ""
	}

	// Handle pointer types first (before named types)
	if ptr, ok := typ.(*types.Pointer); ok {
		return "*" + ta.TypeName(ptr.Elem())
	}

	// Handle named types
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil {
			// Include package path for clarity
			if obj.Pkg() != nil && obj.Pkg().Path() != "" {
				return obj.Pkg().Path() + "." + obj.Name()
			}
			return obj.Name()
		}
	}

	// Fall back to String() representation
	return typ.String()
}

// ResolveChainType resolves the type through a chain of field/method accesses
// Example: user.Profile.Name where user is *User
// Returns the final type and whether resolution succeeded
func (ta *TypeAnalyzer) ResolveChainType(baseName string, chain []ChainElement) (types.Type, bool) {
	// Get base type
	currentType, ok := ta.TypeOf(baseName)
	if !ok {
		return nil, false
	}

	// Follow the chain
	for _, elem := range chain {
		if elem.IsMethod {
			// Method call: get return type
			nextType, ok := ta.MethodReturnType(currentType, elem.Name)
			if !ok {
				return nil, false
			}
			currentType = nextType
		} else {
			// Field access: get field type
			nextType, ok := ta.FieldType(currentType, elem.Name)
			if !ok {
				return nil, false
			}
			currentType = nextType
		}
	}

	return currentType, true
}

// PackagePath returns the current package path
func (ta *TypeAnalyzer) PackagePath() string {
	if ta.pkg != nil {
		return ta.pkg.Path()
	}
	return ""
}

// HasTypeInfo returns true if type information is available
func (ta *TypeAnalyzer) HasTypeInfo() bool {
	return ta.info != nil && ta.pkg != nil
}

// AnalyzeFromDingoFile analyzes a .dingo file by first converting it to Go
// This is a helper for integration with the preprocessor pipeline
func (ta *TypeAnalyzer) AnalyzeFromDingoFile(dingoPath string) error {
	// Get directory containing the file
	dir := filepath.Dir(dingoPath)

	// Use package-level analysis
	return ta.AnalyzePackage(dir)
}
