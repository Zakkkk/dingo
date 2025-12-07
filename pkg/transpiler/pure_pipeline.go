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

	// Step 2: Transform statement-level error propagation (MUST run before expression transforms)
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
	// Combine token mappings, statement mappings, guard let mappings, and AST mappings
	_ = tokenMappings
	_ = stmtMappings
	_ = guardLetMappings
	_ = astMappings

	// Step 3: Parse the transformed Go source with standard go/parser
	fset := token.NewFileSet()
	goFile, err := goparser.ParseFile(fset, filename, transformedSource, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
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
		// Extract actual package name from AST instead of hardcoded "main"
		pkgName := goFile.Name.Name
		checker, err = typechecker.New(fset, goFile, pkgName)
		if err != nil {
			// Type checker unavailable - will fall back to AST-based heuristics
			// for Result/Option wrapping (checks error variable names, function calls)
		}

		// Step 4.1: Lambda type inference from call context
		// This MUST run before general type rewriting to ensure lambda types
		// are available for the rest of type inference
		if checker != nil {
			lambdaInferrer := typechecker.NewLambdaTypeInferrer(fset, goFile, checker.Info())
			lambdaInferrer.Infer()
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
