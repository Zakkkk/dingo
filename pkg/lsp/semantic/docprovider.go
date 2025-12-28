package semantic

import (
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// DocProvider fetches documentation for external symbols.
// It caches loaded packages to avoid repeated loading.
type DocProvider struct {
	mu    sync.RWMutex
	cache map[string]*doc.Package // pkgPath -> parsed docs
}

// NewDocProvider creates a new documentation provider.
func NewDocProvider() *DocProvider {
	return &DocProvider{
		cache: make(map[string]*doc.Package),
	}
}

// GetDoc returns documentation for a types.Object.
// Returns empty string if no documentation is available.
func (p *DocProvider) GetDoc(obj types.Object) string {
	if obj == nil {
		return ""
	}

	// Get the package path
	pkg := obj.Pkg()
	if pkg == nil {
		return "" // Built-in, no docs
	}

	pkgPath := pkg.Path()
	if pkgPath == "" {
		return ""
	}

	// Load package docs (cached)
	docPkg := p.loadPackageDocs(pkgPath)
	if docPkg == nil {
		return ""
	}

	// Find documentation based on object type
	name := obj.Name()
	switch obj.(type) {
	case *types.Func:
		// Check top-level functions
		for _, fn := range docPkg.Funcs {
			if fn.Name == name {
				return formatDocComment(fn.Doc)
			}
		}
		// Check methods - need to find the receiver type
		// Methods are attached to types
		for _, t := range docPkg.Types {
			for _, m := range t.Methods {
				if m.Name == name {
					return formatDocComment(m.Doc)
				}
			}
			// Also check funcs that return this type (constructors)
			for _, fn := range t.Funcs {
				if fn.Name == name {
					return formatDocComment(fn.Doc)
				}
			}
		}

	case *types.TypeName:
		for _, t := range docPkg.Types {
			if t.Name == name {
				return formatDocComment(t.Doc)
			}
		}

	case *types.Var:
		// Check if it's a package-level variable
		for _, v := range docPkg.Vars {
			for _, n := range v.Names {
				if n == name {
					return formatDocComment(v.Doc)
				}
			}
		}

	case *types.Const:
		for _, c := range docPkg.Consts {
			for _, n := range c.Names {
				if n == name {
					return formatDocComment(c.Doc)
				}
			}
		}
	}

	return ""
}

// loadPackageDocs loads and caches documentation for a package.
func (p *DocProvider) loadPackageDocs(pkgPath string) *doc.Package {
	// Check cache first
	p.mu.RLock()
	if cached, ok := p.cache[pkgPath]; ok {
		p.mu.RUnlock()
		return cached
	}
	p.mu.RUnlock()

	// Load package with syntax (needed for doc extraction)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedImports,
		Fset: token.NewFileSet(),
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil || len(pkgs) == 0 {
		return nil
	}

	pkg := pkgs[0]
	if len(pkg.Syntax) == 0 {
		return nil
	}

	// Convert []*ast.File to map for go/doc
	files := make(map[string]*ast.File)
	for i, f := range pkg.Syntax {
		if i < len(pkg.GoFiles) {
			files[pkg.GoFiles[i]] = f
		}
	}

	// Extract documentation
	docPkg, err := doc.NewFromFiles(cfg.Fset, pkg.Syntax, pkgPath, doc.PreserveAST)
	if err != nil {
		return nil
	}

	// Cache result
	p.mu.Lock()
	p.cache[pkgPath] = docPkg
	p.mu.Unlock()

	return docPkg
}

// formatDocComment formats a doc comment for hover display.
func formatDocComment(docStr string) string {
	if docStr == "" {
		return ""
	}

	// Trim whitespace and limit length for hover
	docStr = strings.TrimSpace(docStr)

	// Remove excessive newlines but preserve paragraph structure
	lines := strings.Split(docStr, "\n")
	var result []string
	prevEmpty := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !prevEmpty {
				result = append(result, "")
				prevEmpty = true
			}
		} else {
			result = append(result, line)
			prevEmpty = false
		}
	}

	return strings.Join(result, "\n")
}

// IsExternalPackage returns true if the object is from an external package
// (not the current package being edited).
func IsExternalPackage(obj types.Object, currentPkg *types.Package) bool {
	if obj == nil {
		return false
	}

	objPkg := obj.Pkg()
	if objPkg == nil {
		return false // Built-in
	}

	if currentPkg == nil {
		return true // No current package, assume external
	}

	return objPkg.Path() != currentPkg.Path()
}
