// Package validator provides semantic validation for Dingo source code.
// Validators run before code transformation to catch common errors early
// and provide helpful diagnostic messages.
package validator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/errors"
	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// ValidateSourceWithCache validates Dingo source using a pre-loaded type cache.
// This is the fast path for multi-file builds - uses cached types instead of
// calling packages.Load() for each file (~1.4s savings per file).
//
// Parameters:
//   - src: Dingo source code
//   - filename: Path to the source file (used for error messages)
//   - workingDir: Working directory for type resolution (usually project root)
//   - typeCache: Pre-loaded type cache (if nil, falls back to per-file loading)
//
// Returns nil if the source is valid, or an *errors.EnhancedError if
// a Result tuple unpacking pattern is detected.
func ValidateSourceWithCache(src []byte, filename string, workingDir string, typeCache *typeloader.BuildCache) error {
	fset := token.NewFileSet()

	// Sanitize Dingo syntax (?, match, =>) to make it parseable by go/parser
	sanitized := sanitizeDingoSource(src)

	// Parse the sanitized source as Go
	f, err := parser.ParseFile(fset, filename, sanitized, parser.SkipObjectResolution)
	if err != nil {
		// Can't parse - let the transpiler handle it
		return nil
	}

	// Create a TypeResolver - use cache if available for fast path
	var resolver *codegen.TypeResolver
	if typeCache != nil {
		resolver, _ = codegen.NewTypeResolverWithCache(src, workingDir, typeCache)
	} else {
		resolver, _ = codegen.NewTypeResolver(src, workingDir)
	}
	// Ignore resolver errors - we can still do local validation

	validator := NewResultTupleValidator(fset, src, resolver)
	return validator.Validate(f)
}

// ResultTupleValidator checks for incorrect tuple unpacking of Result types.
//
// This catches the common mistake of using Go's tuple syntax with Result types:
//
//	// WRONG: Result[T,E] returns a single value, not a tuple
//	company, err := repo.GetBySlug(ctx, slug)
//
//	// CORRECT: Use error propagation operator
//	company := repo.GetBySlug(ctx, slug)?
//
//	// CORRECT: Or handle explicitly
//	result := repo.GetBySlug(ctx, slug)
//	if result.IsErr() { ... }
type ResultTupleValidator struct {
	fset     *token.FileSet
	src      []byte
	resolver *codegen.TypeResolver
}

// NewResultTupleValidator creates a new validator.
//
// Parameters:
//   - fset: FileSet for the Dingo source (used for position information)
//   - src: Original Dingo source bytes
//   - resolver: Optional TypeResolver for cross-file type resolution
func NewResultTupleValidator(fset *token.FileSet, src []byte, resolver *codegen.TypeResolver) *ResultTupleValidator {
	return &ResultTupleValidator{
		fset:     fset,
		src:      src,
		resolver: resolver,
	}
}

// Validate checks an AST for Result tuple unpacking errors.
// Returns the first error found, or nil if the code is valid.
func (v *ResultTupleValidator) Validate(node ast.Node) error {
	var validationErr error

	ast.Inspect(node, func(n ast.Node) bool {
		if validationErr != nil {
			return false // Stop walking if we found an error
		}

		switch stmt := n.(type) {
		case *ast.AssignStmt:
			validationErr = v.checkAssignment(stmt)
		case *ast.ValueSpec:
			validationErr = v.checkVarDecl(stmt)
		}

		return true
	})

	return validationErr
}

// checkAssignment validates assignment statements.
// Detects: x, err := FuncReturningResult()
func (v *ResultTupleValidator) checkAssignment(stmt *ast.AssignStmt) error {
	// Only check assignments with 2+ LHS variables
	if len(stmt.Lhs) < 2 {
		return nil
	}

	// Only check assignments with exactly 1 RHS expression
	if len(stmt.Rhs) != 1 {
		return nil
	}

	rhsExpr := stmt.Rhs[0]

	// Check if RHS is a call expression
	callExpr, ok := rhsExpr.(*ast.CallExpr)
	if !ok {
		return nil
	}

	// Check if the call returns a Result type
	isResult, okType, errType := v.inferCallReturnsResult(callExpr)
	if !isResult {
		return nil
	}

	// Found the anti-pattern: tuple unpacking of Result type
	return v.createDiagnostic(stmt, callExpr, okType, errType)
}

// checkVarDecl validates variable declarations with initialization.
// Detects: var x, err = FuncReturningResult()
func (v *ResultTupleValidator) checkVarDecl(spec *ast.ValueSpec) error {
	// Only check declarations with 2+ names
	if len(spec.Names) < 2 {
		return nil
	}

	// Only check declarations with exactly 1 value
	if len(spec.Values) != 1 {
		return nil
	}

	valueExpr := spec.Values[0]

	// Check if value is a call expression
	callExpr, ok := valueExpr.(*ast.CallExpr)
	if !ok {
		return nil
	}

	// Check if the call returns a Result type
	isResult, okType, errType := v.inferCallReturnsResult(callExpr)
	if !isResult {
		return nil
	}

	// Found the anti-pattern: tuple unpacking of Result type
	return v.createDiagnosticForVarDecl(spec, callExpr, okType, errType)
}

// inferCallReturnsResult determines if a call expression returns Result[T, E].
// Uses TypeResolver if available, falls back to local inference.
func (v *ResultTupleValidator) inferCallReturnsResult(callExpr *ast.CallExpr) (bool, string, string) {
	// Get the source bytes for the call expression
	start := v.fset.Position(callExpr.Pos()).Offset
	end := v.fset.Position(callExpr.End()).Offset

	if start < 0 || end > len(v.src) || start > end {
		return false, "", ""
	}

	exprBytes := v.src[start:end]

	// Use existing type inference with resolver
	return codegen.InferExprReturnsResultWithResolver(v.src, exprBytes, start, v.resolver)
}

// createDiagnostic creates an enhanced error for assignment statements.
func (v *ResultTupleValidator) createDiagnostic(stmt *ast.AssignStmt, callExpr *ast.CallExpr, okType, errType string) error {
	// Create base error at the assignment operator position
	err := errors.NewEnhancedError(
		v.fset,
		stmt.TokPos, // Position of := or =
		fmt.Sprintf("cannot unpack Result[%s, %s] as tuple", okType, errType),
	)

	// Add annotation explaining the issue
	err.WithAnnotation("Result types return a single value, not a tuple")

	// Build suggestion based on context
	suggestion := v.buildSuggestion(stmt, callExpr)
	err.WithSuggestion(suggestion)

	return err
}

// createDiagnosticForVarDecl creates an enhanced error for var declarations.
func (v *ResultTupleValidator) createDiagnosticForVarDecl(spec *ast.ValueSpec, callExpr *ast.CallExpr, okType, errType string) error {
	// Create base error at first variable name
	err := errors.NewEnhancedError(
		v.fset,
		spec.Names[0].Pos(),
		fmt.Sprintf("cannot unpack Result[%s, %s] as tuple", okType, errType),
	)

	err.WithAnnotation("Result types return a single value, not a tuple")

	callStr := v.formatCall(callExpr)
	suggestion := fmt.Sprintf(
		"Use the '?' operator for error propagation:\n"+
			"    value := %s?\n"+
			"\n"+
			"Or handle the Result explicitly:\n"+
			"    result := %s\n"+
			"    if result.IsErr() {\n"+
			"        return dgo.Err[ReturnT, ReturnE](result.MustErr())\n"+
			"    }\n"+
			"    value := result.MustOk()",
		callStr, callStr,
	)

	err.WithSuggestion(suggestion)

	return err
}

// buildSuggestion generates helpful suggestions based on the error pattern.
func (v *ResultTupleValidator) buildSuggestion(stmt *ast.AssignStmt, callExpr *ast.CallExpr) string {
	// Check if second variable is blank identifier (_)
	hasBlankIdentifier := false
	if len(stmt.Lhs) == 2 {
		if ident, ok := stmt.Lhs[1].(*ast.Ident); ok && ident.Name == "_" {
			hasBlankIdentifier = true
		}
	}

	callStr := v.formatCall(callExpr)

	if hasBlankIdentifier {
		// User wrote: x, _ := Func()
		// They're ignoring errors - suggest ? operator
		return fmt.Sprintf(
			"Use the '?' operator to propagate errors:\n"+
				"    value := %s?\n"+
				"\n"+
				"Or handle the Result explicitly:\n"+
				"    result := %s\n"+
				"    if result.IsErr() {\n"+
				"        // Handle error\n"+
				"    }\n"+
				"    value := result.MustOk()",
			callStr, callStr,
		)
	}

	// User wrote: x, err := Func()
	// They want error handling - show full pattern
	return fmt.Sprintf(
		"Use the '?' operator for automatic error propagation:\n"+
			"    value := %s?\n"+
			"\n"+
			"Or handle the Result explicitly:\n"+
			"    result := %s\n"+
			"    if result.IsErr() {\n"+
			"        err := result.MustErr()\n"+
			"        // Handle err\n"+
			"        return dgo.Err[ReturnT, ReturnE](err)\n"+
			"    }\n"+
			"    value := result.MustOk()",
		callStr, callStr,
	)
}

// formatCall formats a call expression as a string (best effort).
func (v *ResultTupleValidator) formatCall(callExpr *ast.CallExpr) string {
	start := v.fset.Position(callExpr.Pos()).Offset
	end := v.fset.Position(callExpr.End()).Offset

	if start < 0 || end > len(v.src) || start > end {
		return "FunctionCall()"
	}

	return string(v.src[start:end])
}

// ValidateSource is a convenience function to validate Dingo source code.
// It parses the source and runs the ResultTupleValidator.
//
// Parameters:
//   - src: Dingo source code
//   - filename: Path to the source file (used for error messages)
//   - workingDir: Working directory for type resolution (usually project root)
//
// Returns nil if the source is valid, or an *errors.EnhancedError if
// a Result tuple unpacking pattern is detected.
func ValidateSource(src []byte, filename string, workingDir string) error {
	fset := token.NewFileSet()

	// Sanitize Dingo syntax (?, match, =>) to make it parseable by go/parser
	// We need to handle Dingo-specific syntax without failing to parse
	sanitized := sanitizeDingoSource(src)

	// Parse the sanitized source as Go
	// This works because we're looking for assignment patterns
	// which use valid Go syntax even in Dingo files.
	f, err := parser.ParseFile(fset, filename, sanitized, parser.SkipObjectResolution)
	if err != nil {
		// Can't parse - let the transpiler handle it
		return nil
	}

	// Create a TypeResolver for cross-file type resolution
	resolver, _ := codegen.NewTypeResolver(src, workingDir)
	// Ignore resolver errors - we can still do local validation

	validator := NewResultTupleValidator(fset, src, resolver)
	return validator.Validate(f)
}

// sanitizeDingoSource replaces Dingo-specific syntax with valid Go.
// This is a local copy to avoid circular dependencies.
func sanitizeDingoSource(src []byte) []byte {
	sanitized := make([]byte, len(src))
	copy(sanitized, src)

	// Replace '?' and any following error message string with spaces
	// Pattern: expr? "error message" -> expr  (spaces)
	for i := 0; i < len(sanitized); i++ {
		if sanitized[i] == '?' {
			sanitized[i] = ' '
			// Check if followed by whitespace and a string literal
			j := i + 1
			// Skip whitespace
			for j < len(sanitized) && (sanitized[j] == ' ' || sanitized[j] == '\t') {
				j++
			}
			// Check for string literal
			if j < len(sanitized) && sanitized[j] == '"' {
				// Replace the string literal with spaces
				sanitized[j] = ' '
				j++
				for j < len(sanitized) && sanitized[j] != '"' {
					if sanitized[j] == '\\' && j+1 < len(sanitized) {
						// Skip escaped character
						sanitized[j] = ' '
						j++
						if j < len(sanitized) {
							sanitized[j] = ' '
						}
					} else if sanitized[j] != '\n' {
						sanitized[j] = ' '
					}
					j++
				}
				if j < len(sanitized) {
					sanitized[j] = ' ' // closing quote
				}
			}
		}
	}

	// Replace "=>" with ": " (arrow to colon-space for case-like syntax)
	for i := 0; i < len(sanitized)-1; i++ {
		if sanitized[i] == '=' && sanitized[i+1] == '>' {
			sanitized[i] = ':'
			sanitized[i+1] = ' '
		}
	}

	return sanitized
}
