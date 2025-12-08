package ast

import (
	"testing"
)

func TestFindTuples_TypeAlias(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLocs int
		checks   []func(t *testing.T, loc TupleLocation)
	}{
		{
			name: "simple type alias",
			src:  `type Point2D = (float64, float64)`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Kind != TupleKindTypeAlias {
						t.Errorf("expected TupleKindTypeAlias, got %v", loc.Kind)
					}
					if loc.Context != TupleContextTypeDecl {
						t.Errorf("expected TupleContextTypeDecl, got %v", loc.Context)
					}
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name: "three element type alias",
			src:  `type RGB = (int, int, int)`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Elements != 3 {
						t.Errorf("expected 3 elements, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name: "nested tuple type",
			src:  `type BBox = ((float64, float64), (float64, float64))`,
			wantLocs: 3, // Outer tuple + 2 inner tuples
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					// First should be outer tuple with 2 elements
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements in outer tuple, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name:     "not a tuple - single element",
			src:      `type Wrapper = (int)`,
			wantLocs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locs, err := FindTuples([]byte(tt.src))
			if err != nil {
				t.Fatalf("FindTuples() error = %v", err)
			}

			if len(locs) != tt.wantLocs {
				t.Fatalf("expected %d locations, got %d", tt.wantLocs, len(locs))
			}

			for i, check := range tt.checks {
				if i >= len(locs) {
					t.Fatalf("check %d: no location at index %d", i, i)
				}
				check(t, locs[i])
			}
		})
	}
}

func TestFindTuples_Destructuring(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLocs int
		checks   []func(t *testing.T, loc TupleLocation)
	}{
		{
			name: "simple destructuring",
			src:  `let (x, y) = point`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Kind != TupleKindDestructure {
						t.Errorf("expected TupleKindDestructure, got %v", loc.Kind)
					}
					if loc.Context != TupleContextAssignment {
						t.Errorf("expected TupleContextAssignment, got %v", loc.Context)
					}
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements, got %d", loc.Elements)
					}
					if loc.HasWildcard {
						t.Errorf("expected no wildcards, but HasWildcard=true")
					}
				},
			},
		},
		{
			name: "destructuring with wildcard",
			src:  `let (x, _, z) = triple`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Elements != 3 {
						t.Errorf("expected 3 elements, got %d", loc.Elements)
					}
					if !loc.HasWildcard {
						t.Errorf("expected HasWildcard=true, got false")
					}
				},
			},
		},
		{
			name: "nested destructuring",
			src:  `let ((a, b), c) = nested`,
			wantLocs: 2, // Outer pattern + inner pattern
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					// First should be outer pattern with 2 elements
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements in outer pattern, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name: "wildcard first position",
			src:  `let (_, y) = point`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if !loc.HasWildcard {
						t.Errorf("expected HasWildcard=true")
					}
				},
			},
		},
		{
			name: "wildcard last position",
			src:  `let (x, _) = point`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if !loc.HasWildcard {
						t.Errorf("expected HasWildcard=true")
					}
				},
			},
		},
		{
			name:     "not destructuring - single element",
			src:      `let (x) = value`,
			wantLocs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locs, err := FindTuples([]byte(tt.src))
			if err != nil {
				t.Fatalf("FindTuples() error = %v", err)
			}

			if len(locs) != tt.wantLocs {
				t.Fatalf("expected %d locations, got %d", tt.wantLocs, len(locs))
			}

			for i, check := range tt.checks {
				if i >= len(locs) {
					t.Fatalf("check %d: no location at index %d", i, i)
				}
				check(t, locs[i])
			}
		})
	}
}

func TestFindTuples_Literals(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLocs int
		checks   []func(t *testing.T, loc TupleLocation)
	}{
		{
			name: "simple literal",
			src:  `x := (10, 20)`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Kind != TupleKindLiteral {
						t.Errorf("expected TupleKindLiteral, got %v", loc.Kind)
					}
					if loc.Context != TupleContextAssignment {
						t.Errorf("expected TupleContextAssignment, got %v", loc.Context)
					}
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name: "return context",
			src:  `return (10, 20)`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Context != TupleContextReturn {
						t.Errorf("expected TupleContextReturn, got %v", loc.Context)
					}
				},
			},
		},
		{
			name: "argument context",
			src:  `foo((1, 2), (3, 4))`,
			wantLocs: 1, // Only second tuple found (first filtered by func call check)
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Context != TupleContextArgument {
						t.Errorf("expected TupleContextArgument, got %v", loc.Context)
					}
				},
			},
		},
		{
			name: "nested tuple literal",
			src:  `x := ((1, 2), (3, 4))`,
			wantLocs: 3, // Outer + 2 inner tuples
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					// First (outer) tuple has 2 elements
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements in outer tuple, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name:     "not a tuple - function call",
			src:      `foo(a, b)`,
			wantLocs: 0,
		},
		{
			name:     "not a tuple - grouping",
			src:      `x := (a + b)`,
			wantLocs: 0,
		},
		{
			name:     "not a tuple - single element",
			src:      `x := (42)`,
			wantLocs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locs, err := FindTuples([]byte(tt.src))
			if err != nil {
				t.Fatalf("FindTuples() error = %v", err)
			}

			if len(locs) != tt.wantLocs {
				t.Fatalf("expected %d locations, got %d", tt.wantLocs, len(locs))
			}

			for i, check := range tt.checks {
				if i >= len(locs) {
					t.Fatalf("check %d: no location at index %d", i, i)
				}
				check(t, locs[i])
			}
		})
	}
}

func TestFindTuples_ComplexExpressions(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLocs int
		checks   []func(t *testing.T, loc TupleLocation)
	}{
		{
			name: "method call elements",
			src:  `pair := (foo.Bar(), baz.Qux())`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name: "array index elements",
			src:  `pair := (arr[0], arr[1])`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements, got %d", loc.Elements)
					}
				},
			},
		},
		{
			name: "map access elements",
			src:  `pair := (m["key1"], m["key2"])`,
			wantLocs: 1,
			checks: []func(t *testing.T, loc TupleLocation){
				func(t *testing.T, loc TupleLocation) {
					if loc.Elements != 2 {
						t.Errorf("expected 2 elements, got %d", loc.Elements)
					}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locs, err := FindTuples([]byte(tt.src))
			if err != nil {
				t.Fatalf("FindTuples() error = %v", err)
			}

			if len(locs) != tt.wantLocs {
				t.Fatalf("expected %d locations, got %d", tt.wantLocs, len(locs))
			}

			for i, check := range tt.checks {
				if i >= len(locs) {
					t.Fatalf("check %d: no location at index %d", i, i)
				}
				check(t, locs[i])
			}
		})
	}
}

func TestFindTuples_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLocs int
	}{
		{
			name:     "empty source",
			src:      ``,
			wantLocs: 0,
		},
		{
			name:     "only comments",
			src:      `// comment\n/* block */`,
			wantLocs: 0,
		},
		{
			name:     "tuple in string literal - ignored",
			src:      `s := "(1, 2)"`,
			wantLocs: 0,
		},
		{
			name:     "tuple in comment - ignored",
			src:      `// let (x, y) = point`,
			wantLocs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locs, err := FindTuples([]byte(tt.src))
			if err != nil {
				t.Fatalf("FindTuples() error = %v", err)
			}

			if len(locs) != tt.wantLocs {
				t.Fatalf("expected %d locations, got %d", tt.wantLocs, len(locs))
			}
		})
	}
}

func TestFindTuples_MultipleTypes(t *testing.T) {
	src := `
package main

type Point2D = (float64, float64)

func main() {
	let (x, y) = (10.0, 20.0)
	result := Distance((0, 0), (x, y))
}
`
	locs, err := FindTuples([]byte(src))
	if err != nil {
		t.Fatalf("FindTuples() error = %v", err)
	}

	// Expected:
	// 1. type Point2D = (float64, float64)
	// 2. let (x, y) = ...
	// 3. (10.0, 20.0)
	// 4. (x, y) - only second arg found (first filtered by Distance( check)
	expectedCount := 4

	if len(locs) != expectedCount {
		t.Fatalf("expected %d locations, got %d", expectedCount, len(locs))
	}

	// Verify kinds
	kinds := make(map[TupleKind]int)
	for _, loc := range locs {
		kinds[loc.Kind]++
	}

	if kinds[TupleKindTypeAlias] != 1 {
		t.Errorf("expected 1 type alias, got %d", kinds[TupleKindTypeAlias])
	}
	if kinds[TupleKindDestructure] != 1 {
		t.Errorf("expected 1 destructure, got %d", kinds[TupleKindDestructure])
	}
	if kinds[TupleKindLiteral] != 2 {
		t.Errorf("expected 2 literals, got %d", kinds[TupleKindLiteral])
	}
}

func TestTupleLocation_BytePositions(t *testing.T) {
	src := `type Point = (int, int)`
	locs, err := FindTuples([]byte(src))
	if err != nil {
		t.Fatalf("FindTuples() error = %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}

	loc := locs[0]

	// Extract the actual tuple text using byte positions
	actual := string(src[loc.Start:loc.End])
	expected := "(int, int)"

	if actual != expected {
		t.Errorf("byte positions incorrect: got %q, want %q", actual, expected)
	}
}
