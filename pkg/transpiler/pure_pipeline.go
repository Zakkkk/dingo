package transpiler

import (
	"bytes"
	"fmt"
	goparser "go/parser"
	"go/printer"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
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
	// Extract enum registry from ORIGINAL source (before transformation)
	// This is used by match expressions to prefix variant names correctly
	enumRegistry := dingoast.ExtractEnumRegistry(source)

	// Step 1: Transform Dingo syntax to Go using token-based transformations
	transformedSource, tokenMappings, err := dingoast.TransformSource(source)
	if err != nil {
		return nil, fmt.Errorf("transform error: %w", err)
	}

	// Step 2a: Transform tuple type aliases (must run before Go parser)
	// Pattern: type Point = (int, int) → type Point = __tupleType2__(int, int)
	transformedSource, typeAliasMappings, err := transformTupleTypeAliases(transformedSource)
	if err != nil {
		return nil, fmt.Errorf("tuple type alias error: %w", err)
	}

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
			return nil, fmt.Errorf("tuple literal error (pass %d): %w", pass, err)
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
		return nil, fmt.Errorf("tuple pass 1 error: %w", err)
	}
	tupleMappings = append(tupleMappings, typeAliasMappings...)

	// Step 2.1: Transform statement-level error propagation (MUST run before expression transforms)
	transformedSource, stmtMappings, err := transformErrorPropStatements(transformedSource)
	if err != nil {
		return nil, fmt.Errorf("statement transform error: %w", err)
	}

	// Step 2.5: Transform guard let statements (MUST run after error propagation, before expressions)
	transformedSource, guardLetMappings, err := transformGuardLetStatements(transformedSource)
	if err != nil {
		return nil, fmt.Errorf("guard let transform error: %w", err)
	}

	// Step 3: Transform match/lambda expressions using AST-based codegen
	// Pass enum registry so match expressions can prefix variant names correctly
	transformedSource, astMappings, err := transformASTExpressionsWithRegistry(transformedSource, enumRegistry)
	if err != nil {
		return nil, fmt.Errorf("AST transform error: %w", err)
	}


	// TODO: Store mappings for LSP integration
	// Combine token mappings, tuple mappings, statement mappings, guard let mappings, and AST mappings
	_ = tokenMappings
	_ = tupleMappings
	_ = stmtMappings
	_ = guardLetMappings
	_ = astMappings

	// Step 3: Parse the transformed Go source with standard go/parser
	fset := token.NewFileSet()
	goFile, err := goparser.ParseFile(fset, filename, transformedSource, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
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
				return nil, fmt.Errorf("parse error after tuple pass 2: %w", err)
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
		return nil, fmt.Errorf("none inference: %w", err)
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
			for pass := 0; pass < maxPasses; pass++ {
				lambdaInferrer := typechecker.NewLambdaTypeInferrer(fset, goFile, checker.Info())
				changed := lambdaInferrer.Infer()
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
		return nil, fmt.Errorf("print error: %w", err)
	}

	return buf.Bytes(), nil
}
