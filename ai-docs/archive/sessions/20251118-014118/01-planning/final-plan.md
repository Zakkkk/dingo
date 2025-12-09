# Final Implementation Plan: Comprehensive Parser Fix & Result[T,E] Integration

## Executive Summary

**Approach**: 4-phase incremental delivery
**Total Time**: 13-18 hours
**Strategy**: Quick win first (Phase 1), then build up complexity

**Key Decisions**:
- ✅ Rust-like enum syntax: `enum Result { Ok(T), Err(E) }`
- ✅ Bare constructors: `Ok(value)`, `Err(error)`
- ✅ Type inference: Use heuristics for now, defer full go/types to Phase 3
- ✅ Lenient error handling in preprocessor
- ✅ Both unit and integration tests

---

## Phase 1: Fix Golden Tests (QUICK WIN - 1 hour)

### Goal
Get golden tests using the preprocessor so they can parse Dingo syntax.

### Problem
Tests call `parser.ParseFile()` directly, bypassing preprocessor. This is why tests fail even though `dingo build` CLI works perfectly.

### Solution

**File**: `tests/golden_test.go`

**Change** (lines 87-91):
```go
// OLD: Direct parsing (fails on Dingo syntax)
dingoAST, err := parser.ParseFile(fset, dingoFile, dingoSrc, 0)
require.NoError(t, err, "Failed to parse Dingo file: %s", dingoFile)

// NEW: Preprocess THEN parse
preprocessor := preprocessor.New(nil) // Use default config
preprocessed, _, err := preprocessor.Process(dingoSrc)
require.NoError(t, err, "Failed to preprocess Dingo file: %s", dingoFile)

dingoAST, err := parser.ParseFile(fset, dingoFile, preprocessed, 0)
require.NoError(t, err, "Failed to parse preprocessed Dingo file: %s", dingoFile)
```

### Success Criteria
- ✅ error_prop_01_simple parses without error
- ✅ error_prop_03_expression parses without error
- ✅ error_prop_06_mixed_context parses without error
- ✅ At least 3-5 tests now parse (may still fail on generation)

### Testing
```bash
go test ./tests -run TestGoldenFiles/error_prop_01
go test ./tests -run TestGoldenFiles/error_prop_03
go test ./tests -run TestGoldenFiles/error_prop_06
```

### Time Estimate
- Code changes: 15 minutes
- Testing: 15 minutes
- Fixing any issues: 30 minutes
- **Total: 1 hour**

---

## Phase 2: Enum Preprocessor (4-6 hours)

### Goal
Add preprocessor support for `enum` keyword to enable sum types.

### Task 2.1: Create Enum Preprocessor (2-3 hours)

**New File**: `pkg/preprocessor/enum.go`

**Implementation**:

```go
package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// EnumProcessor transforms enum declarations to Go sum types
type EnumProcessor struct {
	// Pattern: enum Name { Variant1, Variant2, Variant3(Type) }
	enumPattern *regexp.Regexp
}

func NewEnumProcessor() *EnumProcessor {
	return &EnumProcessor{
		enumPattern: regexp.MustCompile(`enum\s+(\w+)\s*\{([^}]+)\}`),
	}
}

func (e *EnumProcessor) Process(source []byte, mappings []Mapping) ([]byte, []Mapping, error) {
	// Find all enum declarations
	matches := e.enumPattern.FindAllSubmatchIndex(source, -1)
	if len(matches) == 0 {
		return source, mappings, nil  // No enums, pass through
	}

	var result bytes.Buffer
	lastEnd := 0

	for _, match := range matches {
		// Copy source before enum
		result.Write(source[lastEnd:match[0]])

		// Extract enum name and body
		enumName := string(source[match[2]:match[3]])
		enumBody := string(source[match[4]:match[5]])

		// Generate Go sum type
		generated, err := e.generateSumType(enumName, enumBody)
		if err != nil {
			// Lenient mode: log warning, keep original
			result.Write(source[match[0]:match[1]])
			lastEnd = match[1]
			continue
		}

		result.WriteString(generated)
		lastEnd = match[1]

		// TODO: Update source mappings
	}

	// Copy remaining source
	result.Write(source[lastEnd:])

	return result.Bytes(), mappings, nil
}

func (e *EnumProcessor) generateSumType(name string, body string) (string, error) {
	// Parse variants: "Ok(T), Err(E)" or "Red, Green, Blue"
	variants := e.parseVariants(body)
	if len(variants) == 0 {
		return "", fmt.Errorf("enum %s has no variants", name)
	}

	var buf bytes.Buffer

	// 1. Generate tag type
	buf.WriteString(fmt.Sprintf("type %sTag uint8\n", name))
	buf.WriteString(fmt.Sprintf("const (\n"))
	for i, v := range variants {
		if i == 0 {
			buf.WriteString(fmt.Sprintf("\t%sTag_%s %sTag = iota\n", name, v.Name, name))
		} else {
			buf.WriteString(fmt.Sprintf("\t%sTag_%s\n", name, v.Name))
		}
	}
	buf.WriteString(")\n\n")

	// 2. Generate struct with tag + variant fields
	buf.WriteString(fmt.Sprintf("type %s struct {\n", name))
	buf.WriteString(fmt.Sprintf("\ttag %sTag\n", name))
	for _, v := range variants {
		if v.Type != "" {
			buf.WriteString(fmt.Sprintf("\t%s_0 *%s\n", strings.ToLower(v.Name), v.Type))
		}
	}
	buf.WriteString("}\n\n")

	// 3. Generate constructor functions
	for _, v := range variants {
		if v.Type != "" {
			// Variant with data: func Name_Variant(value Type) Name
			buf.WriteString(fmt.Sprintf("func %s_%s(value %s) %s {\n", name, v.Name, v.Type, name))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %sTag_%s, %s_0: &value}\n",
				name, name, v.Name, strings.ToLower(v.Name)))
			buf.WriteString("}\n\n")
		} else {
			// Unit variant: var Name_Variant = Name{tag: NameTag_Variant}
			buf.WriteString(fmt.Sprintf("var %s_%s = %s{tag: %sTag_%s}\n\n",
				name, v.Name, name, name, v.Name))
		}
	}

	return buf.String(), nil
}

type Variant struct {
	Name string
	Type string  // Empty for unit variants
}

func (e *EnumProcessor) parseVariants(body string) []Variant {
	parts := strings.Split(body, ",")
	var variants []Variant

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for Variant(Type) syntax
		if idx := strings.Index(part, "("); idx != -1 {
			name := strings.TrimSpace(part[:idx])
			typeStr := strings.TrimSpace(part[idx+1 : len(part)-1])  // Remove ( and )
			variants = append(variants, Variant{Name: name, Type: typeStr})
		} else {
			// Unit variant
			variants = append(variants, Variant{Name: part, Type: ""})
		}
	}

	return variants
}
```

### Task 2.2: Integration (30 minutes)

**File**: `pkg/preprocessor/preprocessor.go`

**Change** (add to processor list):
```go
func New(config *Config) *Preprocessor {
	if config == nil {
		config = DefaultConfig()
	}

	processors := []FeatureProcessor{
		NewTypeAnnotProcessor(),
		NewErrorPropProcessorWithConfig(config),
		NewEnumProcessor(),  // ← ADD THIS
		NewKeywordProcessor(),
	}

	return &Preprocessor{
		processors: processors,
		config:     config,
	}
}
```

### Task 2.3: Testing (1.5-2 hours)

**New File**: `pkg/preprocessor/enum_test.go`

**Test Cases**:
1. Simple enum (unit variants only)
2. Enum with single type (Option-like)
3. Enum with multiple types (Result-like)
4. Nested types
5. Edge cases (empty, malformed)

**Example Test**:
```go
func TestEnumProcessor_SimpleEnum(t *testing.T) {
	processor := NewEnumProcessor()
	source := []byte(`
package main

enum Status { Pending, Active, Done }
`)

	result, _, err := processor.Process(source, nil)
	require.NoError(t, err)

	// Verify generated code
	output := string(result)
	assert.Contains(t, output, "type StatusTag uint8")
	assert.Contains(t, output, "StatusTag_Pending StatusTag = iota")
	assert.Contains(t, output, "var Status_Pending = Status{tag: StatusTag_Pending}")

	// Verify it's valid Go
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "test.go", result, 0)
	assert.NoError(t, err, "Generated code must be valid Go")
}
```

### Success Criteria
- ✅ All enum tests passing
- ✅ Generated code is valid Go
- ✅ Source mappings tracked (basic)

### Time Estimate
- Implementation: 2-3 hours
- Integration: 30 minutes
- Testing: 1.5-2 hours
- **Total: 4-6 hours**

---

## Phase 3: Activate Plugin Pipeline (6-8 hours)

### Goal
Wire Result type plugin into the generator so Ok()/Err() transformations actually run.

### Task 3.1: Implement Plugin Interfaces (2 hours)

**File**: `pkg/plugin/plugin.go`

**Add Interfaces**:
```go
// ContextAware plugins need initialization
type ContextAware interface {
	SetContext(*Context)
}

// Transformer plugins can modify AST
type Transformer interface {
	Plugin
	Transform(node ast.Node) (ast.Node, error)
}

// DeclarationProvider plugins inject declarations
type DeclarationProvider interface {
	GetPendingDeclarations() []ast.Decl
	ClearPendingDeclarations()
}
```

**Update Pipeline**:
```go
type Pipeline struct {
	Ctx     *Context
	plugins []Plugin
}

func NewPipeline(registry *Registry, ctx *Context) (*Pipeline, error) {
	// Register builtin plugins
	plugins := []Plugin{
		builtin.NewResultTypePlugin(),
		// Future: NewOptionTypePlugin(), etc.
	}

	// Initialize plugins with context
	for _, p := range plugins {
		if ca, ok := p.(ContextAware); ok {
			ca.SetContext(ctx)
		}
	}

	return &Pipeline{
		Ctx:     ctx,
		plugins: plugins,
	}, nil
}

func (p *Pipeline) Transform(file *ast.File) (*ast.File, error) {
	// Phase 1: Discovery (find Ok/Err calls)
	for _, plugin := range p.plugins {
		if err := plugin.Process(file); err != nil {
			return nil, fmt.Errorf("plugin %s process failed: %w", plugin.Name(), err)
		}
	}

	// Phase 2: Transformation (replace with CompositeLit)
	transformed := file
	for _, plugin := range p.plugins {
		if transformer, ok := plugin.(Transformer); ok {
			node, err := transformer.Transform(transformed)
			if err != nil {
				return nil, fmt.Errorf("plugin transform failed: %w", err)
			}
			transformed = node.(*ast.File)
		}
	}

	// Phase 3: Declaration injection (add Result type defs)
	for _, plugin := range p.plugins {
		if dp, ok := plugin.(DeclarationProvider); ok {
			decls := dp.GetPendingDeclarations()
			transformed.Decls = append(decls, transformed.Decls...)
			dp.ClearPendingDeclarations()
		}
	}

	return transformed, nil
}
```

### Task 3.2: Update Result Type Plugin (1 hour)

**File**: `pkg/plugin/builtin/result_type.go`

**Add Methods**:
```go
// SetContext implements ContextAware
func (p *ResultTypePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx
}

// Transform implements Transformer (already exists from Fix A2)
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
	// Implementation already complete
	return p.transformAST(node)
}

// GetPendingDeclarations implements DeclarationProvider
func (p *ResultTypePlugin) GetPendingDeclarations() []ast.Decl {
	return p.pendingDecls
}

// ClearPendingDeclarations implements DeclarationProvider
func (p *ResultTypePlugin) ClearPendingDeclarations() {
	p.pendingDecls = nil
	p.emittedTypes = make(map[string]bool)
}
```

### Task 3.3: Update Generator (1 hour)

**File**: `pkg/generator/generator.go`

**Update Generate Method** (around line 130):
```go
func (g *Generator) Generate(file *ast.File) ([]byte, error) {
	// Transform AST through plugin pipeline
	transformed, err := g.pipeline.Transform(file)
	if err != nil {
		return nil, fmt.Errorf("plugin pipeline failed: %w", err)
	}

	// Generate code from transformed AST
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, g.fset, transformed); err != nil {
		return nil, fmt.Errorf("code generation failed: %w", err)
	}

	// ... rest of formatting
}
```

### Task 3.4: Testing (2-4 hours)

**Unit Tests**:
- Plugin initialization
- Transform called correctly
- Declarations injected

**Integration Tests**:
```go
func TestResultTypeEndToEnd(t *testing.T) {
	source := `
package main

func main() {
	result := Ok(42)
}
`
	// Preprocess
	preprocessor := preprocessor.New(nil)
	preprocessed, _, _ := preprocessor.Process([]byte(source))

	// Parse
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "test.dingo", preprocessed, 0)

	// Generate with plugins
	gen, _ := generator.NewWithPlugins(fset, plugin.NewRegistry(), plugin.NewNoOpLogger())
	output, err := gen.Generate(file)

	require.NoError(t, err)

	// Verify Result type declaration exists
	assert.Contains(t, string(output), "type Result_int_error struct")
	assert.Contains(t, string(output), "ResultTag_Ok")

	// Verify Ok() was transformed
	assert.Contains(t, string(output), "Result_int_error{tag: ResultTag_Ok")
}
```

### Success Criteria
- ✅ Pipeline calls Process, Transform, GetDeclarations in order
- ✅ Result type declarations appear in output
- ✅ Ok()/Err() transformed to CompositeLit
- ✅ End-to-end test passes

### Time Estimate
- Interfaces: 2 hours
- Plugin updates: 1 hour
- Generator update: 1 hour
- Testing: 2-4 hours
- **Total: 6-8 hours**

---

## Phase 4: Integration & Polish (2-3 hours)

### Task 4.1: Update Golden Files (1 hour)

**Action**: Regenerate .go.golden files for Result tests
```bash
cd tests/golden
for f in result_*.dingo; do
  ../../cmd/dingo/dingo build $f
  mv ${f%.dingo}.go ${f%.dingo}.go.golden
done
```

**Verify**: Each .go.golden should contain:
- Result type declarations
- Transformed Ok()/Err() calls
- Valid, compilable Go code

### Task 4.2: Add CLI Integration Test (30 minutes)

**New File**: `tests/integration_result_test.go`

```go
func TestResultTypeE2E(t *testing.T) {
	// Create temp .dingo file
	source := `
package main
import "fmt"

func divide(a, b int) Result_int_error {
	if b == 0 {
		return Err(fmt.Errorf("division by zero"))
	}
	return Ok(a / b)
}

func main() {
	result := divide(10, 2)
	if result.IsOk() {
		fmt.Println("Result:", result.Unwrap())
	}
}
`
	tmpFile := filepath.Join(t.TempDir(), "test.dingo")
	os.WriteFile(tmpFile, []byte(source), 0644)

	// Build
	cmd := exec.Command("go", "run", "./cmd/dingo", "build", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Build failed: %s", output)

	// Compile generated Go
	goFile := strings.TrimSuffix(tmpFile, ".dingo") + ".go"
	cmd = exec.Command("go", "build", "-o", "/dev/null", goFile)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Generated Go doesn't compile: %s", output)

	// Run
	cmd = exec.Command("go", "run", goFile)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Execution failed: %s", output)
	assert.Contains(t, string(output), "Result: 5")
}
```

### Task 4.3: Documentation (30 minutes)

**Update**:
- `CHANGELOG.md` - Add Phase 2.16 entry
- `CLAUDE.md` - Update "Current Phase" to 2.16
- Session summary

### Task 4.4: Code Review (1 hour)

Run internal code-reviewer agent:
- Check for bugs, safety issues
- Verify test coverage
- Review generated code quality

### Success Criteria
- ✅ 10+ golden tests passing
- ✅ End-to-end integration test passes
- ✅ Generated code compiles and runs
- ✅ Documentation updated
- ✅ Code review approved

### Time Estimate
- Golden files: 1 hour
- Integration test: 30 minutes
- Documentation: 30 minutes
- Code review: 1 hour
- **Total: 3 hours**

---

## Total Timeline

| Phase | Tasks | Time |
|-------|-------|------|
| Phase 1 | Fix golden tests | 1 hour |
| Phase 2 | Enum preprocessor | 4-6 hours |
| Phase 3 | Activate plugins | 6-8 hours |
| Phase 4 | Integration & polish | 2-3 hours |
| **Total** | **All phases** | **13-18 hours** |

---

## Implementation Order

### Day 1 (6-8 hours)
- ✅ Phase 1: Fix golden tests (1 hour)
- ✅ Phase 2: Enum preprocessor (4-6 hours)
- **Checkpoint**: Commit progress, validate approach

### Day 2 (6-8 hours)
- ✅ Phase 3: Activate plugin pipeline (6-8 hours)
- **Checkpoint**: End-to-end test working

### Day 3 (2-3 hours)
- ✅ Phase 4: Integration & polish (2-3 hours)
- ✅ Code review, commit, push

---

## Risk Management

### High Risk: Plugin pipeline complexity
**Mitigation**: Start with simplest possible pipeline, add complexity incrementally

### Medium Risk: Enum preprocessor regex brittleness
**Mitigation**: Extensive test suite, lenient error handling

### Low Risk: Timeline overrun
**Mitigation**: Each phase delivers value, can stop early if needed

---

## Success Metrics

**Phase 1 Success**:
- 3+ golden tests parse

**Phase 2 Success**:
- Enum preprocessor unit tests: 10/10 passing
- Generated enum code compiles

**Phase 3 Success**:
- Result type declarations generated
- Ok()/Err() transformed
- End-to-end test passes

**Phase 4 Success**:
- 10+ golden tests passing
- CLI integration test passes
- Code review approved

**Final Success**:
- Full workflow: Write .dingo → transpile → compile → run
- Result[T,E] type usable in real code
- Comprehensive test coverage
- Clean, maintainable codebase

---

## Next Steps

1. Get user approval for plan
2. Start Phase 1 (1 hour quick win)
3. Proceed through phases sequentially
4. Code review after each major phase
5. Commit incrementally

**Ready to proceed?**
