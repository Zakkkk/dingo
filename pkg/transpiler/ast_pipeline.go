// Package transpiler provides AST-based transpilation pipeline
package transpiler

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
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

	// Mappings contains source mappings from Dingo to Go (for .dmap generation)
	Mappings []dingoast.SourceMapping

	// DingoSource is the original Dingo source (for line index in .dmap)
	DingoSource []byte
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

// ASTTranspile transpiles Dingo source using the token-level transformation pipeline.
//
// Pipeline stages:
// 1. Transform Dingo syntax to Go syntax (pkg/goparser/parser)
// 2. Parse with go/parser to get Go AST
// 3. Return Go AST and source code
//
// This function uses the same pipeline as PureASTTranspile but returns
// additional metadata and the parsed AST for further processing (e.g., LSP).
func ASTTranspile(source []byte, filename string, fset *token.FileSet) (*TranspileResult, error) {
	result := &TranspileResult{
		Metadata: &TranspileMetadata{
			OriginalFile: filename,
		},
		Errors: make([]error, 0),
	}

	// Use the pure AST pipeline to transform and parse
	goSource, err := PureASTTranspile(source, filename)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("transpilation error: %w", err))
		return result, err
	}

	result.GoCode = goSource

	// Parse the generated Go source to get AST
	goAST, err := goparser.ParseFile(fset, filename, goSource, goparser.ParseComments)
	if err != nil {
		// Still return the code, just without AST
		result.Errors = append(result.Errors, fmt.Errorf("parse error: %w", err))
		return result, nil // Return with errors but don't fail completely
	}

	result.GoAST = goAST

	// Estimate metadata from source
	result.Metadata.TokenCount = len(source) / 5 // Rough approximation

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
