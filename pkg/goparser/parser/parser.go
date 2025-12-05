// Package parser provides Dingo source file parsing.
// This is the AST-based parser that replaces the old string-based transforms.
//
// Architecture:
// - Uses pkg/parser/ for Pratt-based expression parsing
// - Uses pkg/ast/ for Dingo AST nodes and code generation
// - Outputs valid Go code that can be parsed by go/parser
package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"

	dingoparser "github.com/MadAppGang/dingo/pkg/parser"
)

// Mode controls parser behavior
type Mode uint

const (
	ParseComments Mode = 1 << iota
	Trace
	AllErrors
)

// ParseFile parses a Dingo source file and returns a Go AST.
// Uses the AST-based parser from pkg/parser/.
//
// If the Dingo parser fails (e.g., for plain Go files), falls back to go/parser.
func ParseFile(fset *gotoken.FileSet, filename string, src []byte, mode Mode) (*ast.File, error) {
	// Convert mode flags for go/parser
	var goMode goparser.Mode
	if mode&ParseComments != 0 {
		goMode |= goparser.ParseComments
	}
	if mode&AllErrors != 0 {
		goMode |= goparser.AllErrors
	}

	// First, try the standard Go parser for valid Go syntax
	// This handles cases where the input is already valid Go
	goFile, goErr := goparser.ParseFile(fset, filename, src, goMode)
	if goErr == nil {
		return goFile, nil
	}

	// If Go parser failed, try the Dingo AST parser
	// This handles Dingo-specific syntax (?, let, match, enum, etc.)
	var dingoMode dingoparser.Mode
	if mode&ParseComments != 0 {
		dingoMode |= dingoparser.ParseComments
	}
	if mode&Trace != 0 {
		dingoMode |= dingoparser.Trace
	}
	if mode&AllErrors != 0 {
		dingoMode |= dingoparser.AllErrors
	}

	dingoFile, dingoErr := dingoparser.ParseFile(fset, filename, src, dingoMode)
	if dingoErr != nil {
		// Both parsers failed - return the Go parser error as it's more common
		return nil, fmt.Errorf("parse error: %w", goErr)
	}

	// The Dingo parser returns a Go AST file wrapped in a Dingo file
	return dingoFile.File, nil
}

// TokenMapping tracks the relationship between Dingo and Go source positions
type TokenMapping struct {
	DingoStart, DingoEnd int    // Position in original Dingo source
	GoStart, GoEnd       int    // Position in transformed Go source
	Kind                 string // Type of transformation
}

// TransformToGo transforms Dingo source to valid Go source.
//
// DEPRECATED: Use pkg/ast.TransformSource() instead.
// This function is kept for backward compatibility but is no longer
// the primary transformation path. The new transpiler uses:
//
//	dingoast.TransformSource(src) → transformed Go source
//
// See pkg/transpiler/pure_pipeline.go for the new pipeline.
func TransformToGo(src []byte) ([]byte, []TokenMapping, error) {
	// This function is deprecated - use pkg/ast.TransformSource() instead
	return src, nil, nil
}

// ParseExpr parses a Dingo expression and returns a Go AST expression.
func ParseExpr(src string) (ast.Expr, error) {
	// Use standard Go parser for now
	// TODO: Use Dingo parser for Dingo-specific expression syntax
	return goparser.ParseExpr(src)
}
