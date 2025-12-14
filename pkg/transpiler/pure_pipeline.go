package transpiler

import (
	"bytes"
	"fmt"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

/*
DINGO TRANSPILATION PIPELINE (v3 Architecture)
==============================================

This file implements the main transpilation pipeline from .dingo to .go.

POSITION TRACKING FLOW:

  .dingo source
      |
      v
  pkg/tokenizer/Scanner
      - Creates token.FileSet for Dingo
      - Scanner.Pos() returns token.Pos
      |
      v
  pkg/parser/Parser (Pratt-based)
      - Produces Dingo AST nodes
      - Each node has Pos()/End() -> token.Pos
      - KEY: These positions are PRESERVED through all transforms
      |
      v
  pkg/codegen/* (with //line directives)
      - Transforms AST to Go code text
      - Emits //line file.dingo:LINE:COL directives (Go 1.17+)
      - Records transforms via PositionTracker using token.Pos
      |
      v
  go/parser + go/printer
      - PRESERVES //line directives in output
      - go/printer may reformat but directives survive
      |
      v
  PositionTracker.Finalize()
      - Resolves token.Pos to line:col using fset.Position()
      - Generates v3 .dmap file with column-level mappings

DIAGNOSTIC FLOW (why //line directives matter):

  gopls analyzes .go file
      |
      v
  Sees //line file.dingo:42:5
      |
      v
  Reports diagnostic at file.dingo:42:5
      |
      v
  LSP client shows error in .dingo editor

  → No remapping needed for diagnostics! //line directives handle it.

WHY token.Pos INSTEAD OF BYTE OFFSETS:

  The old TransformTracker used byte arithmetic:
    goBytePos += untransformedLen  // FRAGILE: breaks after go/printer reformats!

  The new PositionTracker stores token.Pos from Dingo AST:
    tracker.RecordTransform(node.Pos(), node.End(), "lambda")

  Then resolves AFTER go/printer:
    pos := fset.Position(transform.DingoPos)  // Always accurate!

  Key insight: Dingo AST positions survive the entire pipeline because
  we store the token.Pos value, not a byte offset that becomes stale.
*/

// PureASTTranspile uses AST-based transformation for all Dingo features.
//
// Currently handles:
// - Enums: enum Name { Variant } → Go interface pattern
// - Let declarations: let x = expr → x := expr
// - Lambdas: |x| expr → func(x) { return expr }
// - Match expressions: match x { Pattern => result } → type switch
// - Error propagation: x? → error handling code
// - Ternary: cond ? a : b → inline if
// - Null coalescing: a ?? b → nil check
// - Safe navigation: x?.field → safe access
// - Tuples: (a, b) → struct literal
//
// Pipeline:
// 1. Transform Dingo syntax to Go using AST-based codegens (pkg/ast/transform.go)
// 2. Parse transformed Go with standard go/parser
// 3. Run go/types to infer types (optional)
// 4. Lambda type inference from call context
// 5. Rewrite interface{} placeholders with actual types
// 6. Print Go AST to source
//
// Source mappings are tracked during transformation for LSP integration.
func PureASTTranspile(source []byte, filename string) ([]byte, error) {
	return PureASTTranspileWithOptions(source, filename, true)
}

// PureASTTranspileWithOptions transpiles with optional type inference.
// Set inferTypes to false to disable type inference (faster but uses interface{}).
func PureASTTranspileWithOptions(source []byte, filename string, inferTypes bool) ([]byte, error) {
	result, err := PureASTTranspileWithMappings(source, filename, inferTypes)
	if err != nil {
		return nil, err
	}
	return result.GoCode, nil
}

// PureASTTranspileWithMappings transpiles and returns source mappings for LSP integration.
// This is the full-featured version that returns all transformation metadata.
func PureASTTranspileWithMappings(source []byte, filename string, inferTypes bool) (TranspileResult, error) {
	// Extract enum registry from ORIGINAL source (before transformation)
	// This is used by match expressions to prefix variant names correctly
	enumRegistry := dingoast.ExtractEnumRegistry(source)

	// Step 1: Transform Dingo syntax to Go using token-based transformations
	transformedSource, err := dingoast.TransformSource(source)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("transform error: %w", err)
	}

	// Step 2a: Transform tuple type aliases (must run before Go parser)
	// Pattern: type Point = (int, int) → type Point = __tupleType2__(int, int)
	transformedSource, err = transformTupleTypeAliases(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple type alias error: %w", err)
	}

	// Step 2a1: Transform tuple destructuring (must run before tuple literals)
	// Pattern: (x, y) := expr → _ = __tupleDest2__("x:0", "y:1", expr)
	// This MUST run before transformTupleLiterals to avoid treating the LHS as a tuple literal
	transformedSource, err = transformTupleDestructuring(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple destructuring error: %w", err)
	}

	// Step 2a2: Transform tuple literals (must run before Go parser)
	// Pattern: (a, b) → __tuple2__(a, b)
	// Run in a loop to handle nested tuples: ((a, b), (c, d)) needs multiple passes
	// First pass: inner tuples (a, b) → __tuple2__(a, b)
	// Second pass: outer tuple (__tuple2__(a, b), __tuple2__(c, d)) → __tuple2__(__tuple2__(a, b), __tuple2__(c, d))
	const maxTupleLiteralPasses = 5 // Prevent infinite loops on malformed input
	for pass := 0; pass < maxTupleLiteralPasses; pass++ {
		prevLen := len(transformedSource)
		transformedSource, err = transformTupleLiterals(transformedSource)
		if err != nil {
			return TranspileResult{}, fmt.Errorf("tuple literal error (pass %d): %w", pass, err)
		}
		// If no changes were made, we're done
		if len(transformedSource) == prevLen {
			break
		}
	}

	// Step 2b: Transform tuples - Pass 1 (syntax to markers)
	transformedSource, err = transformTuplePass1(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple pass 1 error: %w", err)
	}

	// Step 2.1: Transform statement-level error propagation (MUST run before expression transforms)
	// Emits //line directives for gopls position mapping
	transformedSource, columnMappings, err := transformErrorPropStatements(transformedSource, source, filename)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("statement transform error: %w", err)
	}

	// Step 2.5: Transform guard statements (MUST run after error propagation, before expressions)
	// Emits //line directives for gopls position mapping
	transformedSource, err = transformGuardStatements(transformedSource, source, filename)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("guard transform error: %w", err)
	}

	// Step 3: Transform match/lambda expressions using AST-based codegen
	// Pass enum registry so match expressions can prefix variant names correctly
	// Also pass original source for accurate position mapping (earlier transforms shift positions)
	// Emits //line directives for gopls position mapping
	transformedSource, err = transformASTExpressions(transformedSource, enumRegistry, source, filename)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("AST transform error: %w", err)
	}

	// Step 3: Parse the transformed Go source with standard go/parser
	fset := token.NewFileSet()
	goFile, err := goparser.ParseFile(fset, filename, transformedSource, goparser.ParseComments)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("parse error: %w", err)
	}

	// Step 3.1: Type-check the Go AST (needed for tuple Pass 2)
	pkgName := goFile.Name.Name
	typeChecker, typeErr := typechecker.New(fset, goFile, pkgName)

	// Step 3.2: Transform tuples - Pass 2 (resolve types and generate final structs)
	// This must happen AFTER go/types has analyzed the AST
	if typeErr == nil && typeChecker != nil {
		tuplePass2Result, tupleErr := transformTuplePass2(fset, goFile, typeChecker, transformedSource)
		if tupleErr == nil {
			// Update the transformed source with type-resolved tuple code
			transformedSource = tuplePass2Result
			// Re-parse with updated source
			goFile, err = goparser.ParseFile(fset, filename, transformedSource, goparser.ParseComments)
			if err != nil {
				return TranspileResult{}, fmt.Errorf("parse error after tuple pass 2: %w", err)
			}
			// Re-create type checker for subsequent steps
			typeChecker, typeErr = typechecker.New(fset, goFile, pkgName)
		}
		// If tuple Pass 2 fails, continue with markers (they're valid Go)
	}

	// Step 3.5: None inference - rewrite bare None/None() to None[T]()
	// This must run BEFORE QualifyDingoTypes so we can access the original type names
	noneTransformer := NewNoneInferenceTransformer(fset, goFile)
	if err := noneTransformer.Transform(); err != nil {
		return TranspileResult{}, fmt.Errorf("none inference: %w", err)
	}

	// Step 3.6: Inject dgo import if Result/Option types are detected
	InjectDgoImport(fset, goFile)

	// Step 4: Run type inference to replace interface{} with actual types
	var checker *typechecker.Checker
	if inferTypes {
		// Reuse type checker from tuple Pass 2 if available
		if typeErr == nil && typeChecker != nil {
			checker = typeChecker
		} else {
			// Create new type checker
			checker, err = typechecker.New(fset, goFile, pkgName)
		}
		if err != nil {
			// Type checker unavailable - will fall back to AST-based heuristics
			// for Result/Option wrapping (checks error variable names, function calls)
		}

		// Step 4.1: Multi-pass lambda type inference from call context
		// We run multiple passes because lambda inference may resolve types
		// that enable further inference in subsequent passes.
		// Example: Filter(users, |u| ...) → eligible has type []User
		//          Map(eligible, |u| ...) → now we can infer u is User
		// Without multi-pass, Map's arg type would be "invalid type" because
		// eligible's type depends on Filter's lambda being correctly typed first.
		if checker != nil {
			const maxPasses = 5 // Prevent infinite loops
			var lastInferrer *typechecker.LambdaTypeInferrer
			for pass := 0; pass < maxPasses; pass++ {
				lambdaInferrer := typechecker.NewLambdaTypeInferrer(fset, goFile, checker.Info())
				changed := lambdaInferrer.Infer()
				lastInferrer = lambdaInferrer
				if !changed {
					break // No more changes, stop iterating
				}

				// Re-type-check with updated AST to get fresh type info
				// This is necessary because after updating lambda types,
				// variables that previously had "invalid type" may now have
				// correct types.
				checker, err = typechecker.New(fset, goFile, pkgName)
				if err != nil {
					break // Type checker failed, stop
				}
			}

			// Step 4.1.1: Check for unresolved lambda types (fail-fast)
			// If any lambdas still have 'any' types after inference, error with helpful message
			if lastInferrer != nil {
				unresolved := lastInferrer.FindUnresolvedLambdas()
				if len(unresolved) > 0 {
					var errMsgs []string
					for _, u := range unresolved {
						errMsgs = append(errMsgs, typechecker.FormatUnresolvedError(u))
					}
					return TranspileResult{}, fmt.Errorf("lambda type inference failed:\n%s", strings.Join(errMsgs, "\n\n"))
				}
			}
		}

		// Step 4.2: General type inference (IIFE return types, etc.)
		_, err = typechecker.RewriteSource(fset, goFile)
		if err != nil {
			// Type inference failed - continue without it
			// This is acceptable since interface{} is valid Go
		}
	}

	// Step 4.5: Wrap Result/Option return statements with dgo constructors
	wrapper := NewResultWrapperTransformer(fset, goFile, checker)
	wrapper.Transform()

	// Step 5: Print Go AST to source
	var buf bytes.Buffer
	cfg := printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 4,
	}
	if err := cfg.Fprint(&buf, fset, goFile); err != nil {
		return TranspileResult{}, fmt.Errorf("print error: %w", err)
	}

	finalGoCode := buf.Bytes()

	// Extract line mappings using Go's scanner (token-based, not byte manipulation).
	// The scanner finds //line directive COMMENT tokens and we get their positions
	// from token.Pos. These mappings are used for .dmap files.
	lineMappings := extractLineMappingsFromGoAST(finalGoCode, "error_prop")

	// Fix column mapping GoLine values using line mappings.
	// Column mappings are created during transformation with GoLine = DingoLine (wrong).
	// Now that we have line mappings, we can set the correct GoLine for each.
	// For error propagation: GoLine = GoLineStart + 1 (the assignment line after the directive)
	columnMappings = fixColumnMappingGoLines(columnMappings, lineMappings)

	// Return complete transpilation result
	return TranspileResult{
		GoCode:         finalGoCode,
		LineMappings:   lineMappings,
		ColumnMappings: columnMappings,
		DingoSource:    source,
		GoAST:          goFile,
		Metadata: &TranspileMetadata{
			OriginalFile: filename,
		},
	}, nil
}
