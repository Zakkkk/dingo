// Package transpiler provides mode selection for transpilation strategies
package transpiler

import (
	"bytes"
	"fmt"
	"go/printer"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
	"github.com/MadAppGang/dingo/pkg/transformer"
)

// TranspileMode represents the transpilation strategy to use
type TranspileMode int

const (
	// ModeLegacy uses the preprocessor-based pipeline (current working implementation)
	// This is the battle-tested production mode with full feature support
	ModeLegacy TranspileMode = iota

	// ModeAST uses the new AST parser + transformer pipeline
	// This is the future architecture, currently under development
	ModeAST

	// ModeHybrid tries AST mode first, falls back to legacy on error
	// This allows gradual migration while maintaining stability
	ModeHybrid
)

// String returns the string representation of the mode
func (m TranspileMode) String() string {
	switch m {
	case ModeLegacy:
		return "legacy"
	case ModeAST:
		return "ast"
	case ModeHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// ParseMode parses a string into a TranspileMode
func ParseMode(s string) (TranspileMode, error) {
	switch s {
	case "legacy":
		return ModeLegacy, nil
	case "ast":
		return ModeAST, nil
	case "hybrid":
		return ModeHybrid, nil
	default:
		return ModeLegacy, fmt.Errorf("invalid transpile mode: %q (must be 'legacy', 'ast', or 'hybrid')", s)
	}
}

// transpileAST uses the new AST-based pipeline:
// 1. Tokenize with pkg/tokenizer
// 2. Parse with pkg/parser (Pratt parser)
// 3. Transform with pkg/transformer
// 4. Print with go/printer
func (t *Transpiler) transpileAST(source []byte, filename string) ([]byte, error) {
	// Step 1: Tokenize
	tok := tokenizer.New(source)

	// Step 2: Parse with Pratt parser
	pratt := parser.NewPrattParser(tok)
	// TODO: Add full file parsing (currently only expressions)
	// For now, just parse a single expression as proof of concept
	expr := pratt.ParseExpression(0)
	if expr == nil || len(pratt.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %v", pratt.Errors())
	}

	// Step 3: Transform Dingo AST to Go AST
	fset := token.NewFileSet()
	trans := transformer.New(fset, nil) // nil type info for now

	// TODO: Create a full dingoast.File from the expression
	// For now, this is a placeholder - full implementation pending
	_ = trans

	// Step 4: Print Go AST to source
	var buf bytes.Buffer
	cfg := printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8,
	}

	// TODO: Print the transformed file
	// For now, return placeholder
	_ = cfg
	return buf.Bytes(), fmt.Errorf("AST mode not fully implemented yet")
}

// transpileWithMode executes transpilation using the specified mode
func (t *Transpiler) transpileWithMode(source []byte, filename string, mode TranspileMode) ([]byte, error) {
	switch mode {
	case ModeLegacy:
		// Use the existing preprocessor-based pipeline
		// This is handled by TranspileFile - no source transformation needed here
		return nil, fmt.Errorf("legacy mode should use TranspileFile directly")

	case ModeAST:
		// Use the new AST-based pipeline
		return t.transpileAST(source, filename)

	case ModeHybrid:
		// Try AST first, fall back to legacy on error
		result, err := t.transpileAST(source, filename)
		if err != nil {
			// AST mode failed, signal to use legacy
			return nil, fmt.Errorf("hybrid mode: AST failed (%v), falling back to legacy", err)
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown transpile mode: %d", mode)
	}
}
