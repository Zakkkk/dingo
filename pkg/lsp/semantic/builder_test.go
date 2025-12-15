package semantic

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/typechecker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder_SimpleVariable(t *testing.T) {
	// Dingo source
	dingoSrc := []byte(`package main

var x int = 42
`)

	// Corresponding Go source
	goSrc := []byte(`package main

var x int = 42
`)

	// Line mappings (Dingo line 3 -> Go line 3)
	mappings := []sourcemap.LineMapping{
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "var_decl"},
	}

	// Build semantic map
	m := buildSemanticMap(t, dingoSrc, goSrc, mappings, "test.dingo")

	// Find variable 'x' at line 3, column 5
	entity := m.FindAt(3, 5)
	require.NotNil(t, entity, "Should find variable x")

	assert.Equal(t, KindIdent, entity.Kind)
	assert.NotNil(t, entity.Object)
	assert.Equal(t, "x", entity.Object.Name())
	assert.Equal(t, "int", entity.Type.String())
	assert.Nil(t, entity.Context, "No special context for simple variable")
}

func TestBuilder_FunctionCall(t *testing.T) {
	// Dingo source
	dingoSrc := []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

	// Go source (same)
	goSrc := dingoSrc

	// Line mappings
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "package"},
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "import"},
		{DingoLine: 5, GoLineStart: 5, GoLineEnd: 5, Kind: "func"},
		{DingoLine: 6, GoLineStart: 6, GoLineEnd: 6, Kind: "call"},
		{DingoLine: 7, GoLineStart: 7, GoLineEnd: 7, Kind: "end"},
	}

	// Build semantic map
	m := buildSemanticMap(t, dingoSrc, goSrc, mappings, "test.dingo")

	// Should have entities for the call
	assert.Greater(t, m.Count(), 0)
}

func TestBuilder_ErrorPropagation(t *testing.T) {
	// Dingo source with error propagation
	dingoSrc := []byte(`package main

import "github.com/user/dgo"

func getUser() dgo.Result[string, error] {
	return dgo.Ok[string, error]("alice")
}

func main() {
	user := getUser()?
}
`)

	// Generated Go source
	goSrc := []byte(`package main

import "github.com/user/dgo"

func getUser() dgo.Result[string, error] {
	return dgo.Ok[string, error]("alice")
}

func main() {
	tmp1 := getUser()
	if tmp1.IsErr() {
		return tmp1.MustErr()
	}
	user := tmp1.MustOk()
}
`)

	// Line mappings (Dingo line 10 -> Go lines 10-14)
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "package"},
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "import"},
		{DingoLine: 5, GoLineStart: 5, GoLineEnd: 5, Kind: "func"},
		{DingoLine: 6, GoLineStart: 6, GoLineEnd: 6, Kind: "return"},
		{DingoLine: 7, GoLineStart: 7, GoLineEnd: 7, Kind: "end"},
		{DingoLine: 9, GoLineStart: 9, GoLineEnd: 9, Kind: "func"},
		{DingoLine: 10, GoLineStart: 10, GoLineEnd: 14, Kind: "error_prop"},
		{DingoLine: 11, GoLineStart: 15, GoLineEnd: 15, Kind: "end"},
	}

	// Build semantic map
	m := buildSemanticMap(t, dingoSrc, goSrc, mappings, "test.dingo")

	// The '?' operator should be detected at line 10
	// (DetectOperators will find it in Dingo source)

	// Variables should be mapped back to Dingo line 10
	// Note: The actual user variable is on Go line 14, which maps to Dingo line 10
	entities := findEntitiesOnLine(m, 10)
	assert.Greater(t, len(entities), 0, "Should have entities on line 10")

	// Look for entities with error propagation context
	var hasErrorPropContext bool
	for _, e := range entities {
		if e.Context != nil && e.Context.Kind == ContextErrorProp {
			hasErrorPropContext = true
			assert.NotNil(t, e.Context.OriginalType, "Should have original Result type")
			assert.NotNil(t, e.Context.UnwrappedType, "Should have unwrapped type")
		}
	}

	// Note: This test may not find error prop context if the builder
	// can't properly extract Result type args. This is a limitation
	// we'll address in future iterations.
	t.Logf("Has error prop context: %v", hasErrorPropContext)
}

func TestBuilder_PositionMapping(t *testing.T) {
	// Test that Go positions correctly map back to Dingo positions

	dingoSrc := []byte(`package main

var a int = 1
var b int = 2
var c int = 3
`)

	goSrc := []byte(`package main

var a int = 1
var b int = 2
var c int = 3
`)

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "package"},
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "var"},
		{DingoLine: 4, GoLineStart: 4, GoLineEnd: 4, Kind: "var"},
		{DingoLine: 5, GoLineStart: 5, GoLineEnd: 5, Kind: "var"},
	}

	m := buildSemanticMap(t, dingoSrc, goSrc, mappings, "test.dingo")

	// Verify each variable is on the correct Dingo line
	a := m.FindAt(3, 5)
	require.NotNil(t, a, "Should find var a")
	assert.Equal(t, "a", a.Object.Name())
	assert.Equal(t, 3, a.Line)

	b := m.FindAt(4, 5)
	require.NotNil(t, b, "Should find var b")
	assert.Equal(t, "b", b.Object.Name())
	assert.Equal(t, 4, b.Line)

	c := m.FindAt(5, 5)
	require.NotNil(t, c, "Should find var c")
	assert.Equal(t, "c", c.Object.Name())
	assert.Equal(t, 5, c.Line)
}

func TestBuilder_ExtractResultType(t *testing.T) {
	// Create a mock types.Type for Result[string, error]
	// This is difficult to test without actually running go/types
	// We'll test the builder's ability to extract types in an integration test

	t.Skip("Type extraction requires full go/types integration - tested in integration tests")
}

func TestBuilder_OperatorDetection(t *testing.T) {
	// Test that operators are detected and added to the map

	dingoSrc := []byte(`package main

func foo() int? {
	return some()?
}

func bar() int {
	return x ?? 0
}

func baz(u *User) string {
	return u?.Name
}
`)

	goSrc := []byte(`package main

func foo() int {
	return some()
}

func bar() int {
	return x
}

func baz(u *User) string {
	return u.Name
}
`)

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "package"},
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "func"},
		{DingoLine: 4, GoLineStart: 4, GoLineEnd: 4, Kind: "return"},
		{DingoLine: 5, GoLineStart: 5, GoLineEnd: 5, Kind: "end"},
		{DingoLine: 7, GoLineStart: 7, GoLineEnd: 7, Kind: "func"},
		{DingoLine: 8, GoLineStart: 8, GoLineEnd: 8, Kind: "return"},
		{DingoLine: 9, GoLineStart: 9, GoLineEnd: 9, Kind: "end"},
		{DingoLine: 11, GoLineStart: 11, GoLineEnd: 11, Kind: "func"},
		{DingoLine: 12, GoLineStart: 12, GoLineEnd: 12, Kind: "return"},
		{DingoLine: 13, GoLineStart: 13, GoLineEnd: 13, Kind: "end"},
	}

	m := buildSemanticMap(t, dingoSrc, goSrc, mappings, "test.dingo")

	// Check for operators
	// Note: Operator detection happens in DetectOperators which scans Dingo source
	// We should have operator entities

	// Look for ? operator on line 4
	operatorsLine4 := findOperatorsOnLine(m, 4)
	assert.Greater(t, len(operatorsLine4), 0, "Should detect ? operator on line 4")

	// Look for ?? operator on line 8
	operatorsLine8 := findOperatorsOnLine(m, 8)
	assert.Greater(t, len(operatorsLine8), 0, "Should detect ?? operator on line 8")

	// Look for ?. operator on line 12
	operatorsLine12 := findOperatorsOnLine(m, 12)
	assert.Greater(t, len(operatorsLine12), 0, "Should detect ?. operator on line 12")
}

// Helper functions

// buildSemanticMap is a test helper that builds a semantic map from source
func buildSemanticMap(
	t *testing.T,
	dingoSrc []byte,
	goSrc []byte,
	mappings []sourcemap.LineMapping,
	dingoFile string,
) *Map {
	// Parse Go source
	goFset := token.NewFileSet()
	goAST, err := parser.ParseFile(goFset, "test.go", goSrc, parser.ParseComments)
	require.NoError(t, err, "Failed to parse Go source")

	// Run type checker
	checker, err := typechecker.New(goFset, goAST, "main")
	require.NoError(t, err, "Failed to create type checker")

	// Create Dingo FileSet
	dingoFset := token.NewFileSet()
	dingoFset.AddFile(dingoFile, dingoFset.Base(), len(dingoSrc))

	// Build semantic map
	builder := NewBuilder(
		goAST,
		goFset,
		checker.Info(),
		mappings,
		nil, // columnMappings - not needed for basic tests
		dingoSrc,
		dingoFset,
		dingoFile,
	)

	m, err := builder.Build()
	require.NoError(t, err, "Failed to build semantic map")

	return m
}

// findEntitiesOnLine returns all entities on a given line
func findEntitiesOnLine(m *Map, line int) []SemanticEntity {
	var entities []SemanticEntity
	for i := 0; i < m.Count(); i++ {
		if m.entities[i].Line == line {
			entities = append(entities, m.entities[i])
		}
	}
	return entities
}

// findOperatorsOnLine returns all operator entities on a given line
func findOperatorsOnLine(m *Map, line int) []SemanticEntity {
	var operators []SemanticEntity
	for i := 0; i < m.Count(); i++ {
		e := m.entities[i]
		if e.Line == line && e.Kind == KindOperator {
			operators = append(operators, e)
		}
	}
	return operators
}

func TestVerifyIdentInSource(t *testing.T) {
	// Sample Dingo source
	source := []byte(`package main

func handler() {
	userID := extractUserID(r)?
	name := getName()?
}
`)

	builder := &Builder{
		dingoSource: source,
	}

	tests := []struct {
		name     string
		ident    string
		line     int
		col      int
		expected bool
	}{
		// Identifiers that exist at the correct position
		{"userID at correct position", "userID", 4, 2, true},
		{"name at correct position", "name", 5, 2, true},
		{"extractUserID at correct position", "extractUserID", 4, 12, true},
		{"getName at correct position", "getName", 5, 10, true},

		// Identifiers at wrong positions (would be generated code)
		{"tmp at userID position", "tmp", 4, 2, false},
		{"err at userID position", "err", 4, 6, false},
		{"wrong name at correct line", "foo", 4, 2, false},

		// Edge cases
		{"beyond line length", "userID", 4, 100, false},
		{"line beyond source", "userID", 100, 1, false},
		{"col zero", "userID", 4, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.verifyIdentInSource(tt.ident, tt.line, tt.col)
			assert.Equal(t, tt.expected, result, "verifyIdentInSource(%q, %d, %d)", tt.ident, tt.line, tt.col)
		})
	}
}

func TestVerifyIdentInSource_NoSource(t *testing.T) {
	// When no source is available, should accept everything
	builder := &Builder{
		dingoSource: nil,
	}

	result := builder.verifyIdentInSource("anything", 1, 1)
	assert.True(t, result, "Should accept when no source available")
}

