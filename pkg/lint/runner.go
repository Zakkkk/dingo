// Package lint provides the main linting infrastructure for Dingo files.
package lint

import (
	"fmt"
	"go/token"
	"os"

	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
	"github.com/MadAppGang/dingo/pkg/lint/refactor"
	"github.com/MadAppGang/dingo/pkg/parser"
)

// Runner orchestrates multiple analyzers and produces diagnostics
type Runner struct {
	analyzers []analyzer.Analyzer
	config    *Config
}

// NewRunner creates a new Runner with the given configuration
func NewRunner(cfg *Config) *Runner {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	analyzers := []analyzer.Analyzer{
		// Correctness analyzers
		&analyzer.ExhaustivenessAnalyzer{},
		// Style analyzers
		&analyzer.NamingAnalyzer{},
		// Refactoring suggestions
		refactor.NewRefactoringAnalyzer(),
	}

	// TODO: Add other analyzers as they are implemented:
	// &analyzer.ErrorPropAnalyzer{},
	// &analyzer.OptionResultAnalyzer{},
	// &analyzer.PatternAnalyzer{},
	// &analyzer.StyleAnalyzer{},

	if len(analyzers) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: No analyzers registered - linter will produce no diagnostics\n")
	}

	return &Runner{
		analyzers: analyzers,
		config:    cfg,
	}
}

// RegisterAnalyzer adds an analyzer to the runner
func (r *Runner) RegisterAnalyzer(a analyzer.Analyzer) {
	r.analyzers = append(r.analyzers, a)
}

// Run executes all enabled analyzers on the given Dingo source file
func (r *Runner) Run(filename string, src []byte) ([]analyzer.Diagnostic, error) {
	// Parse the Dingo file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// Run all enabled analyzers
	var diagnostics []analyzer.Diagnostic
	for _, a := range r.analyzers {
		// Wrap analyzer execution in panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Analyzer panicked - add diagnostic
					diagnostics = append(diagnostics, analyzer.Diagnostic{
						Message:  fmt.Sprintf("Analyzer %s panicked: %v", a.Name(), r),
						Severity: analyzer.SeverityWarning,
						Code:     "INTERNAL",
						Category: "internal",
						Pos:      token.Position{Filename: filename},
					})
				}
			}()

			if !r.config.IsEnabled(a.Name()) {
				return
			}

			diags := a.Run(fset, file, src)

			// Apply severity overrides from config
			for i := range diags {
				if overrideSev, ok := r.config.SeverityOverrides[diags[i].Code]; ok {
					diags[i].Severity = overrideSev
				}
			}

			diagnostics = append(diagnostics, diags...)
		}()
	}

	return diagnostics, nil
}

// RunMultiple runs the linter on multiple files and aggregates results
func (r *Runner) RunMultiple(files map[string][]byte) (map[string][]analyzer.Diagnostic, error) {
	results := make(map[string][]analyzer.Diagnostic)

	for filename, src := range files {
		diags, err := r.Run(filename, src)
		if err != nil {
			return nil, fmt.Errorf("failed to lint %s: %w", filename, err)
		}
		results[filename] = diags
	}

	return results, nil
}
