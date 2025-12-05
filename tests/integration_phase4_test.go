package tests

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// TestIntegrationPhase4EndToEnd tests the complete Phase 4 pipeline:
// .dingo → preprocessor (with RustMatchProcessor) → parser → parent map → plugins → .go
func TestIntegrationPhase4EndToEnd(t *testing.T) {
	t.Run("pattern_match_rust_syntax", func(t *testing.T) {
		dingoSource := `package main

import "fmt"

func handleResult(r ResultIntError) string {
	match r {
		Ok(value) => {
			return fmt.Sprintf("Success: %d", value)
		},
		Err(err) => {
			return fmt.Sprintf("Error: %v", err)
		}
	}
}
`

		// Step 1: Preprocess with Rust match processor
		// RustMatchProcessor is already included in default preprocessor chain
		prep := preprocessor.New([]byte(dingoSource))

		preprocessed, _, err := prep.ProcessBytes()
		if err != nil {
			t.Fatalf("Preprocessing failed: %v", err)
		}

		// Verify preprocessor generated switch statement with source map markers
		// The new AST-based MatchProcessor directly generates switch statements
		preprocessedStr := string(preprocessed)
		if !strings.Contains(preprocessedStr, "switch tmp.tag {") {
			t.Errorf("Expected switch statement with tmp.tag, got:\n%s", preprocessedStr)
		}
		if !strings.Contains(preprocessedStr, "// dingo:M:") {
			t.Errorf("Expected source map markers (dingo:M:N), got:\n%s", preprocessedStr)
		}

		// Step 2: Parse preprocessed code
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", preprocessed, parser.ParseComments)
		if err != nil {
			t.Fatalf("Parsing failed: %v\nPreprocessed:\n%s", err, preprocessedStr)
		}

		// Step 3: Build parent map
		ctx := &plugin.Context{
			FileSet: fset,
			Logger:  &testLogger{t: t},
			Config: &plugin.Config{
				EmitGeneratedMarkers: true,
			},
		}
		ctx.BuildParentMap(file)
		ctx.CurrentFile = file // PRIORITY 3 FIX: Plugin needs CurrentFile to find markers

		// Step 4: Run type checker
		typesInfo, err := runTypeChecker(t, fset, file)
		if err != nil {
			t.Logf("Type checker warning (expected): %v", err)
		}
		ctx.TypeInfo = typesInfo

		// Step 5: Create plugin pipeline
		registry := plugin.NewRegistry()
		pipeline, err := plugin.NewPipeline(registry, ctx)
		if err != nil {
			t.Fatalf("Failed to create pipeline: %v", err)
		}

		// Register plugins
		resultPlugin := builtin.NewResultTypePlugin()
		pipeline.RegisterPlugin(resultPlugin)

		patternMatchPlugin := builtin.NewPatternMatchPlugin()
		pipeline.RegisterPlugin(patternMatchPlugin)

		// Step 6: Transform AST
		transformed, err := pipeline.Transform(file)
		if err != nil {
			t.Fatalf("Transformation failed: %v", err)
		}

		// Step 7: Verify exhaustiveness checking worked
		if ctx.HasErrors() {
			t.Errorf("Expected no errors, but got: %v", ctx.GetErrors())
		}

		// Step 8: Verify switch statement was generated for pattern match
		// Note: Statement context match no longer generates panic (only expression context does)
		switchFound := false
		ast.Inspect(transformed, func(n ast.Node) bool {
			if sw, ok := n.(*ast.SwitchStmt); ok {
				if sel, ok := sw.Tag.(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "tag" {
						switchFound = true
						return false
					}
				}
			}
			return true
		})
		if !switchFound {
			t.Error("Expected switch statement for pattern match, but not found")
		}

		t.Log("✓ Pattern match integration test passed")
	})

	// TODO(ast-migration): This test expects preprocessing to succeed and later phase to catch
	// non-exhaustiveness, but AST-based match processor validates exhaustiveness during preprocessing
	t.Run("pattern_match_non_exhaustive_error", func(t *testing.T) {
		t.Skip("AST-based match processor validates exhaustiveness during preprocessing, not later")

		dingoSource := `package main

func handleOption(o OptionString) string {
	match o {
		Some(value) => {
			return value
		}
	}
	// Missing None case - should error
}
`

		// Step 1: Preprocess
		// RustMatchProcessor is already included in default preprocessor chain
		prep := preprocessor.New([]byte(dingoSource))

		preprocessed, _, err := prep.ProcessBytes()
		if err != nil {
			t.Fatalf("Preprocessing failed: %v", err)
		}

		// Step 2: Parse
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", preprocessed, parser.ParseComments)
		if err != nil {
			t.Fatalf("Parsing failed: %v", err)
		}

		// Step 3: Build parent map
		ctx := &plugin.Context{
			FileSet: fset,
			Logger:  &testLogger{t: t},
		}
		ctx.BuildParentMap(file)
		ctx.CurrentFile = file // PRIORITY 3 FIX: Plugin needs CurrentFile to find markers

		// Step 4: Create pipeline
		registry := plugin.NewRegistry()
		pipeline, err := plugin.NewPipeline(registry, ctx)
		if err != nil {
			t.Fatalf("Failed to create pipeline: %v", err)
		}

		optionPlugin := builtin.NewOptionTypePlugin()
		pipeline.RegisterPlugin(optionPlugin)

		patternMatchPlugin := builtin.NewPatternMatchPlugin()
		pipeline.RegisterPlugin(patternMatchPlugin)

		// Step 5: Transform (should detect non-exhaustive match)
		_, err = pipeline.Transform(file)
		if err != nil {
			t.Fatalf("Transformation failed: %v", err)
		}

		// Step 6: Verify error was reported
		if !ctx.HasErrors() {
			t.Error("Expected non-exhaustive match error, but no errors reported")
		} else {
			errors := ctx.GetErrors()
			errorMsg := errors[0].Error()
			if !strings.Contains(errorMsg, "non-exhaustive") {
				t.Errorf("Expected 'non-exhaustive' error, got: %v", errorMsg)
			}
			t.Logf("✓ Correctly detected non-exhaustive match: %v", errorMsg)
		}
	})

	t.Run("none_context_inference_return", func(t *testing.T) {
		dingoSource := `package main

func getAge(valid bool) OptionInt {
	if !valid {
		return None
	}
	return Some(25)
}
`

		// Step 1: Preprocess
		cfg := preprocessor.DefaultConfig()
		prep := preprocessor.NewWithConfig([]byte(dingoSource), cfg)

		preprocessed, _, err := prep.ProcessBytes()
		if err != nil {
			t.Fatalf("Preprocessing failed: %v", err)
		}

		// Step 2: Parse
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", preprocessed, parser.ParseComments)
		if err != nil {
			t.Fatalf("Parsing failed: %v", err)
		}

		// Step 3: Build parent map (required for None inference)
		ctx := &plugin.Context{
			FileSet: fset,
			Logger:  &testLogger{t: t},
		}
		ctx.BuildParentMap(file)
		ctx.CurrentFile = file // PRIORITY 3 FIX: Plugin needs CurrentFile to find markers

		// Step 4: Run type checker
		typesInfo, err := runTypeChecker(t, fset, file)
		if err != nil {
			t.Logf("Type checker warning (expected): %v", err)
		}
		ctx.TypeInfo = typesInfo

		// Step 5: Create pipeline
		registry := plugin.NewRegistry()
		pipeline, err := plugin.NewPipeline(registry, ctx)
		if err != nil {
			t.Fatalf("Failed to create pipeline: %v", err)
		}

		optionPlugin := builtin.NewOptionTypePlugin()
		pipeline.RegisterPlugin(optionPlugin)

		// Note: NoneContextPlugin is redundant when OptionTypePlugin is registered,
		// as OptionTypePlugin already handles None type inference from context

		// Step 6: Transform
		transformed, err := pipeline.Transform(file)
		if err != nil {
			t.Fatalf("Transformation failed: %v", err)
		}

		// Step 7: Verify no errors (None should infer from return type)
		if ctx.HasErrors() {
			t.Errorf("Expected None to infer type from return, but got errors: %v", ctx.GetErrors())
		}

		// Step 8: Verify None was transformed to OptionInt{tag: OptionTagNone} (C6 FIX)
		// Note: OptionTypePlugin generates {tag: OptionTagNone} without explicit some: nil
		noneFound := false
		ast.Inspect(transformed, func(n ast.Node) bool {
			// Look for OptionInt{tag: OptionTagNone} composite literal
			if comp, ok := n.(*ast.CompositeLit); ok {
				if ident, ok := comp.Type.(*ast.Ident); ok {
					if ident.Name == "OptionInt" {
						// Check for tag: OptionTagNone
						for _, elt := range comp.Elts {
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "tag" {
									if val, ok := kv.Value.(*ast.Ident); ok && val.Name == "OptionTagNone" {
										noneFound = true
										return false
									}
								}
							}
						}
					}
				}
			}
			return true
		})

		if !noneFound {
			t.Error("Expected None to be transformed to OptionInt{tag: OptionTagNone}, but not found")
		}

		t.Log("✓ None context inference test passed")
	})

	t.Run("combined_pattern_match_and_none", func(t *testing.T) {
		dingoSource := `package main

func process(r ResultStringError) OptionInt {
	match r {
		Ok(s) => {
			if len(s) > 0 {
				return Some(len(s))
			}
			return None
		},
		Err(_) => {
			return None
		}
	}
}
`

		// Step 1: Preprocess
		// RustMatchProcessor is already included in default preprocessor chain
		prep := preprocessor.New([]byte(dingoSource))

		preprocessed, _, err := prep.ProcessBytes()
		if err != nil {
			t.Fatalf("Preprocessing failed: %v", err)
		}

		// Step 2: Parse
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", preprocessed, parser.ParseComments)
		if err != nil {
			t.Fatalf("Parsing failed: %v", err)
		}

		// Step 3: Build parent map
		ctx := &plugin.Context{
			FileSet: fset,
			Logger:  &testLogger{t: t},
		}
		ctx.BuildParentMap(file)
		ctx.CurrentFile = file // PRIORITY 3 FIX: Plugin needs CurrentFile to find markers

		// Step 4: Run type checker
		typesInfo, err := runTypeChecker(t, fset, file)
		if err != nil {
			t.Logf("Type checker warning (expected): %v", err)
		}
		ctx.TypeInfo = typesInfo

		// Step 5: Create pipeline with ALL plugins
		registry := plugin.NewRegistry()
		pipeline, err := plugin.NewPipeline(registry, ctx)
		if err != nil {
			t.Fatalf("Failed to create pipeline: %v", err)
		}

		resultPlugin := builtin.NewResultTypePlugin()
		pipeline.RegisterPlugin(resultPlugin)

		optionPlugin := builtin.NewOptionTypePlugin()
		pipeline.RegisterPlugin(optionPlugin)

		patternMatchPlugin := builtin.NewPatternMatchPlugin()
		pipeline.RegisterPlugin(patternMatchPlugin)

		// Note: NoneContextPlugin is redundant when OptionTypePlugin is registered,
		// as OptionTypePlugin already handles None type inference from context

		// Step 6: Transform
		transformed, err := pipeline.Transform(file)
		if err != nil {
			t.Fatalf("Transformation failed: %v", err)
		}

		// Step 7: Verify no errors
		if ctx.HasErrors() {
			t.Errorf("Expected no errors, but got: %v", ctx.GetErrors())
		}

		// Step 8: Verify both pattern match and None inference worked (C6 FIX)
		// Note: OptionTypePlugin generates {tag: OptionTagNone} without explicit some: nil
		// And uses camelCase naming: OptionInt (not Option_int)
		switchFound := false
		noneFound := false
		ast.Inspect(transformed, func(n ast.Node) bool {
			// Check for switch statement (pattern match generates switch scrutinee.tag)
			// Note: Statement context match no longer generates panic (only expression context does)
			if sw, ok := n.(*ast.SwitchStmt); ok {
				if sel, ok := sw.Tag.(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "tag" {
						switchFound = true
					}
				}
			}
			// Check for None transformation (tag-based struct with camelCase naming)
			if comp, ok := n.(*ast.CompositeLit); ok {
				if ident, ok := comp.Type.(*ast.Ident); ok && strings.HasPrefix(ident.Name, "Option") {
					for _, elt := range comp.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok {
							if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "tag" {
								if val, ok := kv.Value.(*ast.Ident); ok && val.Name == "OptionTagNone" {
									noneFound = true
								}
							}
						}
					}
				}
			}
			return true
		})

		if !switchFound {
			t.Error("Expected switch statement for pattern match, but not found")
		}
		if !noneFound {
			t.Error("Expected None to be transformed, but not found")
		}

		t.Log("✓ Combined pattern match + None inference test passed")
	})
}

// Helper functions

// Note: testLogger is already defined in golden_test.go, reusing it

func runTypeChecker(t *testing.T, fset *token.FileSet, file *ast.File) (interface{}, error) {
	// Run go/types type checker (C8/C9: TypeInfo Integration)
	// This enables accurate type inference for plugins
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	conf := &types.Config{
		Importer: importer.Default(),
		Error: func(err error) {
			t.Logf("Type checker: %v", err)
		},
		DisableUnusedImportCheck: true,
	}

	pkgName := "main"
	if file.Name != nil {
		pkgName = file.Name.Name
	}

	_, _ = conf.Check(pkgName, fset, []*ast.File{file}, info)
	// Return info even if there were errors - partial information is useful
	return info, nil
}
