package preprocessor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
)

// UnqualifiedImportProcessorAST transforms unqualified stdlib calls to qualified calls
// using AST-based parsing for 100% accuracy.
//
// Example transformations:
//   ReadFile(path) → os.ReadFile(path) (adds "os" import)
//   Printf("hello") → fmt.Printf("hello") (adds "fmt" import)
//
// The processor uses:
// - FunctionExclusionCache to skip local user-defined functions
// - StdlibRegistry to determine which package a function belongs to
// - go/ast for accurate parsing and transformation
type UnqualifiedImportProcessorAST struct {
	cache         *FunctionExclusionCache
	neededImports map[string]bool // Package paths to import
	fset          *token.FileSet
	mappings      []Mapping
	errors        []error
}

// NewUnqualifiedImportProcessorAST creates a new AST-based processor for the given package
func NewUnqualifiedImportProcessorAST(cache *FunctionExclusionCache) *UnqualifiedImportProcessorAST {
	return &UnqualifiedImportProcessorAST{
		cache:         cache,
		neededImports: make(map[string]bool),
		fset:          token.NewFileSet(),
		mappings:      []Mapping{},
		errors:        []error{},
	}
}

// Name returns the processor name for logging
func (p *UnqualifiedImportProcessorAST) Name() string {
	return "UnqualifiedImportProcessorAST"
}

// Process transforms unqualified stdlib calls to qualified calls
// Returns:
//   - Transformed source code
//   - Source mappings for LSP
//   - Error if transformation fails (e.g., ambiguous function)
func (p *UnqualifiedImportProcessorAST) Process(source []byte) ([]byte, []Mapping, error) {
	// Reset state for this run
	p.neededImports = make(map[string]bool)
	p.fset = token.NewFileSet()
	p.mappings = []Mapping{}
	p.errors = []error{}

	// Parse source as Go AST
	file, err := parser.ParseFile(p.fset, "input.dingo", source, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing source: %w", err)
	}

	// Walk AST and transform unqualified calls
	ast.Inspect(file, p.visitNode)

	// Check for errors during traversal
	if len(p.errors) > 0 {
		return nil, nil, p.errors[0]
	}

	// Generate output with go/printer (RawFormat to minimize changes)
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.RawFormat, Tabwidth: 8}
	if err := cfg.Fprint(&buf, p.fset, file); err != nil {
		return nil, nil, fmt.Errorf("printing AST: %w", err)
	}

	return buf.Bytes(), p.mappings, nil
}

// visitNode is called for each AST node during inspection
func (p *UnqualifiedImportProcessorAST) visitNode(n ast.Node) bool {
	// We're looking for function calls
	callExpr, ok := n.(*ast.CallExpr)
	if !ok {
		return true // Continue traversal
	}

	// Extract function identifier
	ident, ok := callExpr.Fun.(*ast.Ident)
	if !ok {
		// Not a simple identifier (could be selector, function literal, etc.)
		return true
	}

	funcName := ident.Name

	// Skip if not capitalized (stdlib functions are capitalized)
	if !isCapitalized(funcName) {
		return true
	}

	// Check if this is a local function (skip transformation)
	if p.cache.IsLocalSymbol(funcName) {
		return true
	}

	// Look up in stdlib registry (returns full path like "encoding/json")
	// We need to get the FULL import path for tracking imports
	pkgs, exists := StdlibRegistry[funcName]
	if !exists {
		// Not a stdlib function, skip
		return true
	}

	if len(pkgs) == 0 {
		return true
	}

	if len(pkgs) > 1 {
		// Ambiguous function - record error with position
		pos := p.fset.Position(ident.Pos())
		normalizedPkgs := make([]string, len(pkgs))
		for i, pkg := range pkgs {
			normalizedPkgs[i] = normalizePackageName(pkg)
		}
		ambigErr := &AmbiguousFunctionError{
			Function: funcName,
			Packages: normalizedPkgs,
		}
		p.errors = append(p.errors, fmt.Errorf("%s at %s:%d:%d", ambigErr.Error(), pos.Filename, pos.Line, pos.Column))
		return false // Stop traversal on error
	}

	// Unique mapping - get full import path and normalized alias
	fullPkgPath := pkgs[0]           // e.g., "encoding/json"
	pkgAlias := normalizePackageName(fullPkgPath) // e.g., "json"

	// Create selector: pkg.funcName
	selector := &ast.SelectorExpr{
		X:   &ast.Ident{Name: pkgAlias},
		Sel: ident,
	}

	// Replace the function expression
	callExpr.Fun = selector

	// Track import (full path)
	p.neededImports[fullPkgPath] = true

	// Create source mapping
	pos := p.fset.Position(ident.Pos())
	qualifiedName := pkgAlias + "." + funcName

	mapping := Mapping{
		GeneratedLine:   pos.Line,
		GeneratedColumn: pos.Column,
		OriginalLine:    pos.Line,
		OriginalColumn:  pos.Column,
		Length:          len(qualifiedName),
		Name:            fmt.Sprintf("unqualified:%s", funcName),
	}
	p.mappings = append(p.mappings, mapping)

	return true
}

// GetNeededImports returns the list of import paths that should be added
// Implements the ImportProvider interface
func (p *UnqualifiedImportProcessorAST) GetNeededImports() []string {
	imports := make([]string, 0, len(p.neededImports))
	for pkg := range p.neededImports {
		imports = append(imports, pkg)
	}
	return imports
}

// isCapitalized checks if a name starts with a capital letter
func isCapitalized(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}
