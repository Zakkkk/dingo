package transpiler

import (
	"bytes"
	"fmt"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

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
	// Create TransformTracker to record all transformations
	tracker := sourcemap.NewTransformTracker(source)

	// Extract enum registry from ORIGINAL source (before transformation)
	// This is used by match expressions to prefix variant names correctly
	enumRegistry := dingoast.ExtractEnumRegistry(source)

	// Step 1: Transform Dingo syntax to Go using token-based transformations
	transformedSource, tokenMappings, err := dingoast.TransformSource(source)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("transform error: %w", err)
	}

	// Step 2a: Transform tuple type aliases (must run before Go parser)
	// Pattern: type Point = (int, int) → type Point = __tupleType2__(int, int)
	transformedSource, typeAliasMappings, err := transformTupleTypeAliases(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple type alias error: %w", err)
	}

	// Step 2a1: Transform tuple destructuring (must run before tuple literals)
	// Pattern: (x, y) := expr → _ = __tupleDest2__("x:0", "y:1", expr)
	// This MUST run before transformTupleLiterals to avoid treating the LHS as a tuple literal
	transformedSource, destMappings, err := transformTupleDestructuring(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple destructuring error: %w", err)
	}
	typeAliasMappings = append(typeAliasMappings, destMappings...)

	// Step 2a2: Transform tuple literals (must run before Go parser)
	// Pattern: (a, b) → __tuple2__(a, b)
	// Run in a loop to handle nested tuples: ((a, b), (c, d)) needs multiple passes
	// First pass: inner tuples (a, b) → __tuple2__(a, b)
	// Second pass: outer tuple (__tuple2__(a, b), __tuple2__(c, d)) → __tuple2__(__tuple2__(a, b), __tuple2__(c, d))
	const maxTupleLiteralPasses = 5 // Prevent infinite loops on malformed input
	for pass := 0; pass < maxTupleLiteralPasses; pass++ {
		prevLen := len(transformedSource)
		var tupleLitMappings []dingoast.SourceMapping
		transformedSource, tupleLitMappings, err = transformTupleLiterals(transformedSource)
		if err != nil {
			return TranspileResult{}, fmt.Errorf("tuple literal error (pass %d): %w", pass, err)
		}
		typeAliasMappings = append(typeAliasMappings, tupleLitMappings...)
		// If no changes were made, we're done
		if len(transformedSource) == prevLen {
			break
		}
	}

	// Step 2b: Transform tuples - Pass 1 (syntax to markers)
	transformedSource, tupleMappings, err := transformTuplePass1(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple pass 1 error: %w", err)
	}
	tupleMappings = append(tupleMappings, typeAliasMappings...)

	// Step 2.1: Transform statement-level error propagation (MUST run before expression transforms)
	transformedSource, stmtMappings, err := transformErrorPropStatementsWithTracker(transformedSource, source, tracker)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("statement transform error: %w", err)
	}

	// Step 2.5: Transform guard statements (MUST run after error propagation, before expressions)
	transformedSource, guardMappings, err := transformGuardStatements(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("guard transform error: %w", err)
	}

	// Step 3: Transform match/lambda expressions using AST-based codegen
	// Pass enum registry so match expressions can prefix variant names correctly
	// Also pass original source for accurate position mapping (earlier transforms shift positions)
	transformedSource, astMappings, err := transformASTExpressionsWithRegistry(transformedSource, enumRegistry, source, tracker)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("AST transform error: %w", err)
	}

	// Combine all mappings from the transformation pipeline
	allMappings := make([]dingoast.SourceMapping, 0,
		len(tokenMappings)+len(tupleMappings)+len(stmtMappings)+len(guardMappings)+len(astMappings))
	allMappings = append(allMappings, tokenMappings...)
	allMappings = append(allMappings, tupleMappings...)
	allMappings = append(allMappings, stmtMappings...)
	allMappings = append(allMappings, guardMappings...)
	allMappings = append(allMappings, astMappings...)

	// Deduplicate and sort mappings by GoStart for efficient lookup
	allMappings = deduplicateAndSortMappings(allMappings)

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

	// Finalize tracker with final Go output to compute line mappings
	finalGoCode := buf.Bytes()
	if err := tracker.Finalize(finalGoCode); err != nil {
		return TranspileResult{}, fmt.Errorf("tracker finalize error: %w", err)
	}

	// Return complete transpilation result with mappings
	return TranspileResult{
		GoCode:       finalGoCode,
		Mappings:     allMappings,
		LineMappings: tracker.LineMappings(),
		DingoSource:  source,
		GoAST:        goFile,
		Metadata: &TranspileMetadata{
			OriginalFile: filename,
		},
	}, nil
}

// deduplicateAndSortMappings removes duplicate mappings and sorts by GoStart.
// Duplicates are identified by having identical DingoStart, DingoEnd, GoStart, and GoEnd.
// When duplicates exist, the first one encountered (by Kind) is kept.
func deduplicateAndSortMappings(mappings []dingoast.SourceMapping) []dingoast.SourceMapping {
	if len(mappings) == 0 {
		return mappings
	}

	// Sort by GoStart first (primary key for lookups)
	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].GoStart < mappings[j].GoStart
	})

	// Deduplicate by checking if adjacent mappings have identical positions
	deduped := make([]dingoast.SourceMapping, 0, len(mappings))
	deduped = append(deduped, mappings[0])

	for i := 1; i < len(mappings); i++ {
		curr := mappings[i]
		prev := deduped[len(deduped)-1]

		// Skip if positions are identical (duplicate mapping)
		if curr.DingoStart == prev.DingoStart &&
			curr.DingoEnd == prev.DingoEnd &&
			curr.GoStart == prev.GoStart &&
			curr.GoEnd == prev.GoEnd {
			continue
		}

		deduped = append(deduped, curr)
	}

	return deduped
}
