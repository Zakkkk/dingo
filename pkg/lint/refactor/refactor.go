package refactor

import (
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// PatternDetector detects a specific Go pattern that can be improved with Dingo syntax.
// Each detector corresponds to one refactoring rule (R001-R007).
//
// Detectors return Diagnostics with Category="refactor" and Fixes attached,
// which the LSP server translates into Code Actions for the IDE.
type PatternDetector interface {
	Code() string // e.g., "R001"
	Name() string // e.g., "prefer-error-prop"
	Doc() string  // Human-readable description

	// Detect analyzes the AST and returns diagnostics for detected patterns.
	// Each diagnostic should include Fix suggestions that can be applied by the IDE.
	Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic
}

// RefactoringAnalyzer implements the Analyzer interface and orchestrates
// multiple PatternDetectors to find Go patterns that can be improved with
// idiomatic Dingo syntax.
//
// This analyzer is registered alongside correctness and style analyzers in
// the linter pipeline. It returns diagnostics with Category="refactor" and
// attached Fixes for LSP Code Actions.
type RefactoringAnalyzer struct {
	detectors []PatternDetector
}

// NewRefactoringAnalyzer creates a RefactoringAnalyzer with all built-in
// pattern detectors. Individual detectors can be disabled via configuration.
func NewRefactoringAnalyzer() *RefactoringAnalyzer {
	return &RefactoringAnalyzer{
		detectors: []PatternDetector{
			&ErrorPropDetector{},  // R001: prefer-error-prop
			&NilCheckDetector{},   // R002: prefer-match-nil
			&TypeSwitchDetector{}, // R003: prefer-match-type
			&GuardLetDetector{},   // R004: prefer-guard-let
			&ResultTypeDetector{}, // R005: prefer-result-type
			&OptionTypeDetector{}, // R006: prefer-option-type
			&OkPatternDetector{},  // R007: prefer-match-ok
		},
	}
}

// Name implements analyzer.Analyzer
func (r *RefactoringAnalyzer) Name() string {
	return "refactoring"
}

// Doc implements analyzer.Analyzer
func (r *RefactoringAnalyzer) Doc() string {
	return "Suggests Dingo idioms for Go patterns (R001-R007)"
}

// Category implements analyzer.Analyzer
func (r *RefactoringAnalyzer) Category() string {
	return "refactor"
}

// Run implements analyzer.Analyzer
//
// Runs all registered PatternDetectors and collects their diagnostics.
// Each diagnostic will have:
// - Category: "refactor"
// - Severity: SeverityHint (default for refactoring suggestions)
// - Fixes: One or more Fix suggestions for LSP Code Actions
func (r *RefactoringAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	var diagnostics []analyzer.Diagnostic

	for _, detector := range r.detectors {
		diags := detector.Detect(fset, file, src)
		diagnostics = append(diagnostics, diags...)
	}

	return diagnostics
}

// AddDetector adds a custom PatternDetector to the analyzer.
// This allows external plugins to register additional refactoring rules.
func (r *RefactoringAnalyzer) AddDetector(d PatternDetector) {
	r.detectors = append(r.detectors, d)
}

// Detectors returns all registered pattern detectors.
// Useful for configuration and filtering.
func (r *RefactoringAnalyzer) Detectors() []PatternDetector {
	return r.detectors
}
