package dgo

import (
	"fmt"
	"reflect"
	"testing"
)

// Test helper struct
type person struct {
	name string
	age  int
}

// ============================================================================
// Core Functions Tests
// ============================================================================

func TestMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		fn       func(int) string
		expected []string
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    []int{1, 2, 3},
			fn:       func(x int) string { return fmt.Sprintf("%d", x) },
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "empty slice",
			input:    []int{},
			fn:       func(x int) string { return fmt.Sprintf("%d", x) },
			expected: []string{},
		},
		{
			name:     "nil slice",
			input:    nil,
			fn:       func(x int) string { return fmt.Sprintf("%d", x) },
			expected: nil,
			isNil:    true,
		},
		{
			name:     "single element",
			input:    []int{42},
			fn:       func(x int) string { return fmt.Sprintf("num:%d", x) },
			expected: []string{"num:42"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Map(tt.input, tt.fn)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  []int
		isNil     bool
	}{
		{
			name:      "filter evens",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  []int{2, 4},
		},
		{
			name:      "empty slice",
			input:     []int{},
			predicate: func(x int) bool { return x > 0 },
			expected:  []int{},
		},
		{
			name:      "nil slice",
			input:     nil,
			predicate: func(x int) bool { return x > 0 },
			expected:  nil,
			isNil:     true,
		},
		{
			name:      "all match",
			input:     []int{2, 4, 6},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  []int{2, 4, 6},
		},
		{
			name:      "none match",
			input:     []int{1, 3, 5},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filter(tt.input, tt.predicate)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestReduce(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		initial  int
		fn       func(int, int) int
		expected int
	}{
		{
			name:     "sum",
			input:    []int{1, 2, 3, 4},
			initial:  0,
			fn:       func(acc, x int) int { return acc + x },
			expected: 10,
		},
		{
			name:     "empty slice",
			input:    []int{},
			initial:  100,
			fn:       func(acc, x int) int { return acc + x },
			expected: 100,
		},
		{
			name:     "nil slice",
			input:    nil,
			initial:  42,
			fn:       func(acc, x int) int { return acc + x },
			expected: 42,
		},
		{
			name:     "product",
			input:    []int{2, 3, 4},
			initial:  1,
			fn:       func(acc, x int) int { return acc * x },
			expected: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Reduce(tt.input, tt.initial, tt.fn)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestReduceTypeChange(t *testing.T) {
	input := []int{1, 2, 3}
	result := Reduce(input, "", func(acc string, x int) string {
		return acc + fmt.Sprintf("%d", x)
	})
	expected := "123"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestForEach(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		var sum int
		ForEach([]int{1, 2, 3}, func(x int) { sum += x })
		if sum != 6 {
			t.Errorf("expected sum=6, got %d", sum)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		count := 0
		ForEach([]int{}, func(x int) { count++ })
		if count != 0 {
			t.Errorf("expected count=0, got %d", count)
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		count := 0
		ForEach([]int(nil), func(x int) { count++ })
		if count != 0 {
			t.Errorf("expected count=0, got %d", count)
		}
	})
}

// ============================================================================
// Index-Aware Variants Tests
// ============================================================================

func TestMapWithIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		fn       func(int, string) string
		expected []string
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    []string{"a", "b", "c"},
			fn:       func(i int, s string) string { return fmt.Sprintf("%d:%s", i, s) },
			expected: []string{"0:a", "1:b", "2:c"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			fn:       func(i int, s string) string { return fmt.Sprintf("%d:%s", i, s) },
			expected: []string{},
		},
		{
			name:     "nil slice",
			input:    nil,
			fn:       func(i int, s string) string { return fmt.Sprintf("%d:%s", i, s) },
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapWithIndex(tt.input, tt.fn)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterWithIndex(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int, int) bool
		expected  []int
		isNil     bool
	}{
		{
			name:      "even indices",
			input:     []int{10, 20, 30, 40, 50},
			predicate: func(i, x int) bool { return i%2 == 0 },
			expected:  []int{10, 30, 50},
		},
		{
			name:      "nil slice",
			input:     nil,
			predicate: func(i, x int) bool { return i%2 == 0 },
			expected:  nil,
			isNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterWithIndex(tt.input, tt.predicate)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestForEachWithIndex(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		indices := []int{}
		values := []string{}
		ForEachWithIndex([]string{"a", "b", "c"}, func(i int, s string) {
			indices = append(indices, i)
			values = append(values, s)
		})
		expectedIndices := []int{0, 1, 2}
		expectedValues := []string{"a", "b", "c"}
		if !reflect.DeepEqual(indices, expectedIndices) {
			t.Errorf("expected indices %v, got %v", expectedIndices, indices)
		}
		if !reflect.DeepEqual(values, expectedValues) {
			t.Errorf("expected values %v, got %v", expectedValues, values)
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		count := 0
		ForEachWithIndex([]int(nil), func(i, x int) { count++ })
		if count != 0 {
			t.Errorf("expected count=0, got %d", count)
		}
	})
}

// ============================================================================
// Search/Predicate Functions Tests
// ============================================================================

func TestFind(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		result := Find([]int{1, 2, 3, 4}, func(x int) bool { return x > 2 })
		if !result.IsSome() {
			t.Error("expected Some, got None")
		}
		if result.Unwrap() != 3 {
			t.Errorf("expected 3, got %d", result.Unwrap())
		}
	})

	t.Run("not found", func(t *testing.T) {
		result := Find([]int{1, 2, 3}, func(x int) bool { return x > 10 })
		if result.IsSome() {
			t.Error("expected None, got Some")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		result := Find([]int{}, func(x int) bool { return x > 0 })
		if result.IsSome() {
			t.Error("expected None, got Some")
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		result := Find([]int(nil), func(x int) bool { return x > 0 })
		if result.IsSome() {
			t.Error("expected None, got Some")
		}
	})
}

func TestFindIndex(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		result := FindIndex([]int{1, 2, 3, 4, 5}, func(x int) bool { return x > 3 })
		if !result.IsSome() {
			t.Error("expected Some, got None")
		}
		if result.Unwrap() != 3 {
			t.Errorf("expected index 3, got %d", result.Unwrap())
		}
	})

	t.Run("not found", func(t *testing.T) {
		result := FindIndex([]int{1, 2, 3}, func(x int) bool { return x > 10 })
		if result.IsSome() {
			t.Error("expected None, got Some")
		}
	})
}

func TestAny(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  bool
	}{
		{
			name:      "some match",
			input:     []int{1, 2, 3, 4},
			predicate: func(x int) bool { return x > 3 },
			expected:  true,
		},
		{
			name:      "none match",
			input:     []int{1, 2, 3},
			predicate: func(x int) bool { return x > 10 },
			expected:  false,
		},
		{
			name:      "empty slice",
			input:     []int{},
			predicate: func(x int) bool { return x > 0 },
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Any(tt.input, tt.predicate)
			if result != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestAll(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  bool
	}{
		{
			name:      "all match",
			input:     []int{2, 4, 6},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  true,
		},
		{
			name:      "some don't match",
			input:     []int{2, 3, 4},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  false,
		},
		{
			name:      "empty slice (vacuous truth)",
			input:     []int{},
			predicate: func(x int) bool { return x > 1000 },
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := All(tt.input, tt.predicate)
			if result != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestNoneMatch(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  bool
	}{
		{
			name:      "none match",
			input:     []int{1, 3, 5},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  true,
		},
		{
			name:      "some match",
			input:     []int{1, 2, 3},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  false,
		},
		{
			name:      "empty slice",
			input:     []int{},
			predicate: func(x int) bool { return x > 0 },
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NoneMatch(tt.input, tt.predicate)
			if result != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		value    int
		expected bool
	}{
		{
			name:     "contains",
			input:    []int{1, 2, 3, 4},
			value:    3,
			expected: true,
		},
		{
			name:     "not contains",
			input:    []int{1, 2, 3},
			value:    10,
			expected: false,
		},
		{
			name:     "empty slice",
			input:    []int{},
			value:    1,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.input, tt.value)
			if result != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  int
	}{
		{
			name:      "some match",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  2,
		},
		{
			name:      "none match",
			input:     []int{1, 3, 5},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  0,
		},
		{
			name:      "all match",
			input:     []int{2, 4, 6},
			predicate: func(x int) bool { return x%2 == 0 },
			expected:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Count(tt.input, tt.predicate)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Advanced Functions Tests
// ============================================================================

func TestFlatMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		fn       func(string) []rune
		expected []rune
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    []string{"hi", "go"},
			fn:       func(s string) []rune { return []rune(s) },
			expected: []rune{'h', 'i', 'g', 'o'},
		},
		{
			name:     "empty slice",
			input:    []string{},
			fn:       func(s string) []rune { return []rune(s) },
			expected: []rune{},
		},
		{
			name:     "nil slice",
			input:    nil,
			fn:       func(s string) []rune { return []rune(s) },
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlatMap(tt.input, tt.fn)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]int
		expected []int
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    [][]int{{1, 2}, {3, 4}, {5}},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "empty nested",
			input:    [][]int{},
			expected: []int{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Flatten(tt.input)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPartition(t *testing.T) {
	tests := []struct {
		name             string
		input            []int
		predicate        func(int) bool
		expectedMatch    []int
		expectedNotMatch []int
		isNil            bool
	}{
		{
			name:             "normal case",
			input:            []int{1, 2, 3, 4, 5},
			predicate:        func(x int) bool { return x%2 == 0 },
			expectedMatch:    []int{2, 4},
			expectedNotMatch: []int{1, 3, 5},
		},
		{
			name:             "all match",
			input:            []int{2, 4, 6},
			predicate:        func(x int) bool { return x%2 == 0 },
			expectedMatch:    []int{2, 4, 6},
			expectedNotMatch: []int{},
		},
		{
			name:             "none match",
			input:            []int{1, 3, 5},
			predicate:        func(x int) bool { return x%2 == 0 },
			expectedMatch:    []int{},
			expectedNotMatch: []int{1, 3, 5},
		},
		{
			name:             "nil slice",
			input:            nil,
			predicate:        func(x int) bool { return x%2 == 0 },
			expectedMatch:    nil,
			expectedNotMatch: nil,
			isNil:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, notMatch := Partition(tt.input, tt.predicate)
			if tt.isNil {
				if match != nil || notMatch != nil {
					t.Errorf("expected (nil, nil), got (%v, %v)", match, notMatch)
				}
			} else {
				if !reflect.DeepEqual(match, tt.expectedMatch) {
					t.Errorf("match: expected %v, got %v", tt.expectedMatch, match)
				}
				if !reflect.DeepEqual(notMatch, tt.expectedNotMatch) {
					t.Errorf("notMatch: expected %v, got %v", tt.expectedNotMatch, notMatch)
				}
			}
		})
	}
}

func TestGroupBy(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		people := []person{
			{"Alice", 30},
			{"Bob", 30},
			{"Charlie", 25},
		}
		result := GroupBy(people, func(p person) int { return p.age })
		if len(result) != 2 {
			t.Errorf("expected 2 groups, got %d", len(result))
		}
		if len(result[30]) != 2 {
			t.Errorf("expected 2 people aged 30, got %d", len(result[30]))
		}
		if len(result[25]) != 1 {
			t.Errorf("expected 1 person aged 25, got %d", len(result[25]))
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		result := GroupBy([]int{}, func(x int) int { return x })
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		result := GroupBy([]int(nil), func(x int) int { return x })
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
		isNil    bool
	}{
		{
			name:     "with duplicates",
			input:    []int{1, 2, 2, 3, 1, 4},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "all unique",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Unique(tt.input)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    []int{1, 2, 3, 4},
			expected: []int{4, 3, 2, 1},
		},
		{
			name:     "single element",
			input:    []int{42},
			expected: []int{42},
		},
		{
			name:     "empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Reverse(tt.input)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Slice Manipulation Tests
// ============================================================================

func TestTake(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		n        int
		expected []int
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    []int{1, 2, 3, 4, 5},
			n:        3,
			expected: []int{1, 2, 3},
		},
		{
			name:     "n > len",
			input:    []int{1, 2},
			n:        5,
			expected: []int{1, 2},
		},
		{
			name:     "n <= 0",
			input:    []int{1, 2, 3},
			n:        0,
			expected: []int{},
		},
		{
			name:     "nil slice",
			input:    nil,
			n:        3,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Take(tt.input, tt.n)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDrop(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		n        int
		expected []int
		isNil    bool
	}{
		{
			name:     "normal case",
			input:    []int{1, 2, 3, 4, 5},
			n:        2,
			expected: []int{3, 4, 5},
		},
		{
			name:     "n >= len",
			input:    []int{1, 2},
			n:        5,
			expected: []int{},
		},
		{
			name:     "n <= 0",
			input:    []int{1, 2, 3},
			n:        0,
			expected: []int{1, 2, 3},
		},
		{
			name:     "nil slice",
			input:    nil,
			n:        2,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Drop(tt.input, tt.n)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTakeWhile(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  []int
		isNil     bool
	}{
		{
			name:      "normal case",
			input:     []int{1, 2, 3, 4, 1, 2},
			predicate: func(x int) bool { return x < 4 },
			expected:  []int{1, 2, 3},
		},
		{
			name:      "all match",
			input:     []int{1, 2, 3},
			predicate: func(x int) bool { return x < 10 },
			expected:  []int{1, 2, 3},
		},
		{
			name:      "none match",
			input:     []int{5, 6, 7},
			predicate: func(x int) bool { return x < 4 },
			expected:  []int{},
		},
		{
			name:      "nil slice",
			input:     nil,
			predicate: func(x int) bool { return x > 0 },
			expected:  nil,
			isNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TakeWhile(tt.input, tt.predicate)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDropWhile(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  []int
		isNil     bool
	}{
		{
			name:      "normal case",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(x int) bool { return x < 4 },
			expected:  []int{4, 5},
		},
		{
			name:      "all match",
			input:     []int{1, 2, 3},
			predicate: func(x int) bool { return x < 10 },
			expected:  []int{},
		},
		{
			name:      "none match",
			input:     []int{5, 6, 7},
			predicate: func(x int) bool { return x < 4 },
			expected:  []int{5, 6, 7},
		},
		{
			name:      "nil slice",
			input:     nil,
			predicate: func(x int) bool { return x > 0 },
			expected:  nil,
			isNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DropWhile(tt.input, tt.predicate)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestChunk(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		size     int
		expected [][]int
		isNil    bool
		panics   bool
	}{
		{
			name:     "normal case",
			input:    []int{1, 2, 3, 4, 5, 6, 7},
			size:     3,
			expected: [][]int{{1, 2, 3}, {4, 5, 6}, {7}},
		},
		{
			name:     "exact multiple",
			input:    []int{1, 2, 3, 4, 5, 6},
			size:     2,
			expected: [][]int{{1, 2}, {3, 4}, {5, 6}},
		},
		{
			name:     "size > len",
			input:    []int{1, 2},
			size:     5,
			expected: [][]int{{1, 2}},
		},
		{
			name:     "nil slice",
			input:    nil,
			size:     2,
			expected: nil,
			isNil:    true,
		},
		{
			name:   "size <= 0 panics",
			input:  []int{1, 2, 3},
			size:   0,
			panics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panics {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic, but didn't panic")
					}
				}()
			}
			result := Chunk(tt.input, tt.size)
			if tt.panics {
				return
			}
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestZipSlices(t *testing.T) {
	tests := []struct {
		name     string
		a        []int
		b        []string
		expected []Pair[int, string]
		isNil    bool
	}{
		{
			name: "same length",
			a:    []int{1, 2, 3},
			b:    []string{"a", "b", "c"},
			expected: []Pair[int, string]{
				{1, "a"},
				{2, "b"},
				{3, "c"},
			},
		},
		{
			name: "different lengths",
			a:    []int{1, 2, 3},
			b:    []string{"a", "b"},
			expected: []Pair[int, string]{
				{1, "a"},
				{2, "b"},
			},
		},
		{
			name:     "first nil",
			a:        nil,
			b:        []string{"a", "b"},
			expected: nil,
			isNil:    true,
		},
		{
			name:     "second nil",
			a:        []int{1, 2},
			b:        nil,
			expected: nil,
			isNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ZipSlices(tt.a, tt.b)
			if tt.isNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Integration Tests (Chaining)
// ============================================================================

func TestChaining(t *testing.T) {
	t.Run("filter then map", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		evens := Filter(numbers, func(x int) bool { return x%2 == 0 })
		doubled := Map(evens, func(x int) int { return x * 2 })
		expected := []int{4, 8}
		if !reflect.DeepEqual(doubled, expected) {
			t.Errorf("expected %v, got %v", expected, doubled)
		}
	})

	t.Run("map then reduce", func(t *testing.T) {
		words := []string{"hello", "world"}
		lengths := Map(words, func(s string) int { return len(s) })
		total := Reduce(lengths, 0, func(acc, x int) int { return acc + x })
		expected := 10
		if total != expected {
			t.Errorf("expected %d, got %d", expected, total)
		}
	})

	t.Run("find with option map", func(t *testing.T) {
		people := []person{
			{"Alice", 30},
			{"Bob", 25},
		}
		found := Find(people, func(p person) bool { return p.age > 28 })
		if !found.IsSome() {
			t.Error("expected to find Alice")
		}
		if found.Unwrap().name != "Alice" {
			t.Errorf("expected Alice, got %s", found.Unwrap().name)
		}
	})
}
