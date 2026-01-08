package semantic

import (
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectOperators(t *testing.T) {
	// expectedOp is a simplified expected operator (without exact column positions)
	type expectedOp struct {
		Line int
		Kind ContextKind
	}

	tests := []struct {
		name     string
		source   string
		expected []expectedOp
	}{
		{
			name:   "error propagation - simple",
			source: "x := foo()?",
			expected: []expectedOp{
				{Line: 1, Kind: ContextErrorProp},
			},
		},
		{
			name:   "null coalescing - simple",
			source: "y := a ?? b",
			expected: []expectedOp{
				{Line: 1, Kind: ContextNullCoal},
			},
		},
		{
			name:   "safe navigation - simple",
			source: "z := x?.y",
			expected: []expectedOp{
				{Line: 1, Kind: ContextSafeNav},
			},
		},
		{
			name:   "multiple operators on same line",
			source: "result := foo()? ?? bar()?.baz",
			expected: []expectedOp{
				{Line: 1, Kind: ContextErrorProp},
				{Line: 1, Kind: ContextNullCoal},
				{Line: 1, Kind: ContextSafeNav},
			},
		},
		{
			name:   "operators on multiple lines",
			source: "x := foo()?\ny := a ?? b\nz := x?.y",
			expected: []expectedOp{
				{Line: 1, Kind: ContextErrorProp},
				{Line: 2, Kind: ContextNullCoal},
				{Line: 3, Kind: ContextSafeNav},
			},
		},
		{
			name:     "operators inside strings should NOT be detected",
			source:   `s := "foo()? and a ?? b"`,
			expected: []expectedOp{
				// String contents should be treated as STRING token, not operators
			},
		},
		{
			name:     "operators in comments should NOT be detected",
			source:   "// x := foo()?\nx := 42",
			expected: []expectedOp{
				// Comments should be COMMENT token, not operators
			},
		},
		{
			name: "complex expression",
			source: `func process() Result[User, error] {
    user := getUser()?
    if user.Name?.Length > 0 {
        return user.Email ?? "unknown"
    }
}`,
			expected: []expectedOp{
				{Line: 2, Kind: ContextErrorProp},
				{Line: 3, Kind: ContextSafeNav},
				{Line: 4, Kind: ContextNullCoal},
			},
		},
		{
			name:   "chained safe navigation",
			source: "x := a?.b?.c?.d",
			expected: []expectedOp{
				{Line: 1, Kind: ContextSafeNav},
				{Line: 1, Kind: ContextSafeNav},
				{Line: 1, Kind: ContextSafeNav},
			},
		},
		{
			name:   "error prop in function call",
			source: "foo(bar()?, baz()?)",
			expected: []expectedOp{
				{Line: 1, Kind: ContextErrorProp},
				{Line: 1, Kind: ContextErrorProp},
			},
		},
		{
			name:     "empty source",
			source:   "",
			expected: []expectedOp{
				// No operators
			},
		},
		{
			name:     "no operators",
			source:   "x := foo()\ny := bar()",
			expected: []expectedOp{
				// No operators
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			operators := DetectOperators([]byte(tt.source), fset, "test.dingo")

			require.Len(t, operators, len(tt.expected), "operator count mismatch")

			for i, expected := range tt.expected {
				actual := operators[i]
				assert.Equal(t, expected.Line, actual.Line, "operator %d line mismatch", i)
				assert.Equal(t, expected.Kind, actual.Kind, "operator %d kind mismatch", i)
				// Validate column positions are reasonable (1-indexed, positive, EndCol > Col)
				assert.True(t, actual.Col > 0, "operator %d: Col should be positive", i)
				assert.True(t, actual.EndCol > actual.Col, "operator %d: EndCol should be > Col", i)
			}
		})
	}
}

func TestDetectOperators_TokenizationError(t *testing.T) {
	// Test that tokenization errors are handled gracefully
	fset := token.NewFileSet()

	// Unterminated string should cause tokenization error
	source := `x := "unterminated string`
	operators := DetectOperators([]byte(source), fset, "test.dingo")

	// Should return nil (graceful degradation)
	assert.Nil(t, operators)
}

func TestOperatorInfo_Positions(t *testing.T) {
	// Test that operator positions are accurate for hover
	source := "x := foo()?"
	fset := token.NewFileSet()
	operators := DetectOperators([]byte(source), fset, "test.dingo")

	require.Len(t, operators, 1)
	op := operators[0]

	// Verify the operator is on line 1 and spans 1 character
	assert.Equal(t, 1, op.Line)
	assert.Equal(t, op.EndCol, op.Col+1) // ? is 1 character wide

	// The exact column depends on tokenizer implementation
	// Important: position should allow hover lookup to work
	// For hover at any column from Col to EndCol-1, the operator should be found
	assert.True(t, op.Col > 0, "Column should be positive (1-indexed)")
	assert.True(t, op.EndCol > op.Col, "EndCol should be after Col")
}

func TestOperatorInfo_MultibyteCharacters(t *testing.T) {
	// Test that operators work correctly with multibyte UTF-8 characters
	source := "名前 := foo()?"
	fset := token.NewFileSet()
	operators := DetectOperators([]byte(source), fset, "test.dingo")

	require.Len(t, operators, 1)
	op := operators[0]

	// Tokenizer should handle UTF-8 correctly
	assert.Equal(t, 1, op.Line)
	assert.Equal(t, ContextErrorProp, op.Kind)
	// Column position should be character-based, not byte-based
	assert.True(t, op.Col > 0)
	assert.True(t, op.EndCol > op.Col)
}
