package analyzer

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
)

// ResultTupleAnalyzer detects tuple destructuring of Result-returning functions.
//
// In Dingo, functions returning Result[T, E] should NOT be destructured with tuple syntax:
//
//	// WRONG - Result[T, E] is a single value, not a tuple
//	value, err := functionReturningResult()
//
//	// CORRECT - Use single assignment and Result methods
//	result := functionReturningResult()
//	if result.IsErr() { return dgo.Err[T, E](result.MustErr()) }
//	value := result.MustOk()
//
//	// ALSO CORRECT - Use ? operator for propagation
//	value := functionReturningResult()?
//
// Rule: D005 - result-tuple-destructure
// Category: correctness
type ResultTupleAnalyzer struct {
	// workingDir is the directory for resolving imports (usually set from file path)
	workingDir string
}

func (a *ResultTupleAnalyzer) Name() string {
	return "result-tuple-destructure"
}

func (a *ResultTupleAnalyzer) Doc() string {
	return "Detects tuple destructuring of Result[T,E]-returning functions (which is invalid)"
}

func (a *ResultTupleAnalyzer) Category() string {
	return "correctness"
}

func (a *ResultTupleAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// Determine working directory from file position for import resolution
	if file.File != nil && fset != nil {
		if pos := fset.Position(file.Pos()); pos.Filename != "" {
			a.workingDir = filepath.Dir(pos.Filename)
		}
	}
	// Fallback to current working directory if filename not available
	if a.workingDir == "" {
		a.workingDir, _ = os.Getwd()
	}

	// Sanitize Dingo source to make it parseable by Go's parser
	// Uses the shared sanitization from codegen package (not position tracking, just preprocessing)
	sanitized := codegen.SanitizeDingoSource(src)

	// Use Go's standard parser to get full AST with function bodies
	// The Dingo parser skips function bodies, so we need Go's parser here
	goFile, err := goparser.ParseFile(fset, "", sanitized, goparser.ParseComments)
	if err != nil {
		// Even after sanitization, parsing may fail for complex Dingo patterns
		// Return empty diagnostics, don't treat as error
		return nil
	}

	// Create a TypeResolver for cross-file type resolution
	var resolver *codegen.TypeResolver
	if a.workingDir != "" {
		resolver, _ = codegen.NewTypeResolver(src, a.workingDir)
	}

	// Walk the Go AST looking for assignment statements
	ast.Inspect(goFile, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			// Check if this is a tuple destructuring (2+ LHS variables)
			if len(stmt.Lhs) >= 2 {
				// Check if RHS is a single call expression
				if len(stmt.Rhs) == 1 {
					if callExpr, ok := stmt.Rhs[0].(*ast.CallExpr); ok {
						// Extract the expression bytes for type checking
						startPos := fset.Position(callExpr.Pos()).Offset
						endPos := fset.Position(callExpr.End()).Offset

						// Validate positions are within source bounds
						if startPos >= 0 && endPos <= len(src) && startPos < endPos {
							exprBytes := src[startPos:endPos]

							// Check if this call returns a Result type
							isResult, _, _ := codegen.InferExprReturnsResultWithResolver(src, exprBytes, startPos, resolver)
							if isResult {
								diagnostics = append(diagnostics, a.createDiagnostic(fset, stmt))
							}
						}
					}
				}
			}
		}
		return true
	})

	return diagnostics
}

// createDiagnostic creates a D005 diagnostic for a tuple destructuring of Result
func (a *ResultTupleAnalyzer) createDiagnostic(fset *token.FileSet, stmt *ast.AssignStmt) Diagnostic {
	return Diagnostic{
		Pos:      fset.Position(stmt.Pos()),
		End:      fset.Position(stmt.End()),
		Message:  "Cannot destructure Result[T, E] as tuple. Use `result := ...` then `result.IsOk()`/`result.MustOk()`, or use `?` for error propagation.",
		Severity: SeverityError, // This WILL cause compilation failure
		Code:     "D005",
		Category: "correctness",
		Related: []RelatedInfo{
			{
				Pos:     fset.Position(stmt.Rhs[0].Pos()),
				Message: "This function returns Result[T, E], not (T, error)",
			},
		},
	}
}
