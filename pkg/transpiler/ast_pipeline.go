// Package transpiler provides AST-based transpilation pipeline
package transpiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
	prattparser "github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/transformer"
)

// TranspileResult holds the result of AST-based transpilation
type TranspileResult struct {
	// GoCode is the generated Go source code
	GoCode []byte

	// Errors contains all errors from parsing and transformation
	Errors []error

	// GoAST is the final Go AST (for further processing)
	GoAST *ast.File

	// Metadata contains transformation metadata for source maps
	Metadata *TranspileMetadata
}

// TranspileMetadata contains metadata about the transpilation
type TranspileMetadata struct {
	// OriginalFile is the original Dingo filename
	OriginalFile string

	// TokenCount is the number of tokens parsed
	TokenCount int

	// TransformCount is the number of Dingo nodes transformed
	TransformCount int
}

// ASTTranspile transpiles Dingo source using the new AST-based pipeline
//
// Pipeline stages:
// 1. Tokenize - Convert source to tokens (pkg/tokenizer)
// 2. Parse - Build Dingo AST using Pratt parser (pkg/parser/pratt.go)
// 3. Transform - Convert Dingo AST to Go AST (pkg/transformer)
// 4. Print - Generate Go source from AST (go/printer)
//
// This is the future replacement for the current preprocessor-based pipeline.
// Currently under development (Phase: Parser Implementation).
func ASTTranspile(source []byte, filename string, fset *token.FileSet) (*TranspileResult, error) {
	result := &TranspileResult{
		Metadata: &TranspileMetadata{
			OriginalFile: filename,
		},
		Errors: make([]error, 0),
	}

	// Step 1: Tokenize
	tok := tokenizer.NewWithFileSet(source, fset, filename)
	tokens, err := tok.Tokenize()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("tokenization error: %w", err))
		return result, err
	}
	result.Metadata.TokenCount = len(tokens)

	// Step 2: Parse (Dingo AST)
	// The Pratt parser builds a Dingo AST from tokens
	parser := prattparser.NewPrattParser(tok)

	// Parse expressions using the Pratt parser
	// Note: Currently the Pratt parser is designed for expression parsing
	// Full file parsing will be implemented in parser/decl.go
	// For now, we'll parse a single expression to test the pipeline
	expr := parser.ParseExpression(prattparser.PrecLowest)

	// Collect parse errors
	parseErrors := parser.Errors()
	if len(parseErrors) > 0 {
		for _, pe := range parseErrors {
			result.Errors = append(result.Errors, pe)
		}
		// Continue with partial AST for LSP support
	}

	// TODO: Build full file AST when parser/decl.go is implemented
	// For now, create a minimal file structure with the parsed expression
	// This is a placeholder until we have full file parsing
	if expr == nil {
		err := fmt.Errorf("parsing failed: no expression parsed")
		result.Errors = append(result.Errors, err)
		return result, err
	}

	// Create a minimal Go file structure wrapped in Dingo File
	// This will be replaced with proper file parsing
	// Note: We can't use Dingo expressions directly in Go AST
	// The transformer will handle conversion of Dingo nodes to Go nodes
	goFileAST := &ast.File{
		Name: &ast.Ident{Name: "main"}, // Placeholder package name
		Decls: []ast.Decl{
			// Placeholder: empty file for now
			// When we have full parsing, this will contain declarations
		},
	}

	// Wrap in Dingo File structure
	dingoFile := &dingoast.File{
		File:       goFileAST,
		DingoNodes: []dingoast.DingoNode{},
	}

	// Step 3: Transform (Dingo AST → Go AST)
	// The transformer walks the Dingo AST and converts Dingo-specific nodes
	// (ErrorPropExpr, LambdaExpr, MatchExpr, etc.) to standard Go AST
	t := transformer.New(fset, &types.Info{}) // TODO: Provide actual type info from type checker
	goAST, err := t.Transform(dingoFile)

	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("transformation error: %w", err))
		return result, err
	}

	// Collect transformation errors
	transformErrors := t.GetContext().GetErrors()
	if len(transformErrors) > 0 {
		result.Errors = append(result.Errors, transformErrors...)
		// Continue for partial results
	}

	result.GoAST = goAST
	result.Metadata.TransformCount = len(transformErrors) // Approximation

	// Step 4: Print (Go AST → source code)
	var buf bytes.Buffer
	printConfig := &printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8,
	}

	if err := printConfig.Fprint(&buf, fset, goAST); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("code generation error: %w", err))
		return result, err
	}

	result.GoCode = buf.Bytes()

	// Return result even if there were recoverable errors (for LSP)
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("transpilation completed with %d error(s)", len(result.Errors))
	}

	return result, nil
}

// ASTTranspileIncremental is a variant for incremental transpilation (LSP mode)
// It returns partial results even when errors occur
func ASTTranspileIncremental(source []byte, filename string, fset *token.FileSet) *TranspileResult {
	result, _ := ASTTranspile(source, filename, fset)
	// Always return result, errors are stored in result.Errors
	return result
}

// HasErrors returns true if the transpilation result contains errors
func (r *TranspileResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetErrorMessages returns formatted error messages
func (r *TranspileResult) GetErrorMessages() []string {
	messages := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		messages[i] = err.Error()
	}
	return messages
}
