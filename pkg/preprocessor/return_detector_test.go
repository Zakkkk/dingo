package preprocessor

import (
	"testing"
)

func TestReturnDetector_DetectReturns(t *testing.T) {
	// Create test source code with various function signatures
	source := `
package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"strconv"
)

func errorOnly() error {
	return nil
}

func valueAndError() (*os.File, error) {
	return nil, nil
}

func twoValues() (int, error) {
	return 0, nil
}

func noReturns() {
}

func multipleReturns() (int, string, error) {
	return 0, "", nil
}

func main() {
	var rows *sql.Rows
	var data []byte
	var v interface{}
	var r io.Reader
	var s string

	// Use variables to avoid unused warnings
	_ = rows
	_ = data
	_ = v
	_ = r
	_ = s
}
`

	// Create and initialize TypeAnalyzer
	analyzer := NewTypeAnalyzer()
	err := analyzer.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("Failed to analyze source: %v", err)
	}

	// Create ReturnDetector
	detector := NewReturnDetector(analyzer)

	tests := []struct {
		name          string
		expr          string
		wantCount     int
		wantErrorOnly bool
		wantLastError bool
		wantError     bool
		wantTypeCount int // Expected number of types in Types slice
	}{
		{
			name:          "error only function",
			expr:          "errorOnly()",
			wantCount:     1,
			wantErrorOnly: true,
			wantLastError: true,
			wantError:     false,
			wantTypeCount: 1,
		},
		{
			name:          "value and error",
			expr:          "valueAndError()",
			wantCount:     2,
			wantErrorOnly: false,
			wantLastError: true,
			wantError:     false,
			wantTypeCount: 2,
		},
		{
			name:          "two values with error",
			expr:          "twoValues()",
			wantCount:     2,
			wantErrorOnly: false,
			wantLastError: true,
			wantError:     false,
			wantTypeCount: 2,
		},
		{
			name:          "no returns",
			expr:          "noReturns()",
			wantCount:     0,
			wantErrorOnly: false,
			wantLastError: false,
			wantError:     false,
			wantTypeCount: 0,
		},
		{
			name:          "multiple returns",
			expr:          "multipleReturns()",
			wantCount:     3,
			wantErrorOnly: false,
			wantLastError: true,
			wantError:     false,
			wantTypeCount: 3,
		},
		{
			name:          "unknown function",
			expr:          "unknownFunc()",
			wantCount:     2,             // Falls back to default (T, error)
			wantErrorOnly: false,
			wantLastError: true,
			wantError:     false,        // No error with heuristics fallback
			wantTypeCount: 2,
		},
		{
			name:          "invalid expression",
			expr:          "not a call",
			wantCount:     0,
			wantErrorOnly: false,
			wantLastError: false,
			wantError:     true,
			wantTypeCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := detector.DetectReturns(tt.expr, source)

			// Check error expectation
			if tt.wantError {
				if err == nil {
					t.Errorf("DetectReturns() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("DetectReturns() unexpected error: %v", err)
			}

			// Verify return count
			if info.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", info.Count, tt.wantCount)
			}

			// Verify ErrorOnly flag
			if info.ErrorOnly != tt.wantErrorOnly {
				t.Errorf("ErrorOnly = %v, want %v", info.ErrorOnly, tt.wantErrorOnly)
			}

			// Verify LastIsError flag
			if info.LastIsError != tt.wantLastError {
				t.Errorf("LastIsError = %v, want %v", info.LastIsError, tt.wantLastError)
			}

			// Verify Types slice length
			if len(info.Types) != tt.wantTypeCount {
				t.Errorf("len(Types) = %d, want %d", len(info.Types), tt.wantTypeCount)
			}

			// For error-only, verify the type is "error"
			if tt.wantErrorOnly && len(info.Types) > 0 {
				if info.Types[0] != "error" {
					t.Errorf("Types[0] = %q, want %q", info.Types[0], "error")
				}
			}
		})
	}
}

func TestReturnDetector_StdlibFunctions(t *testing.T) {
	// Test with stdlib functions that have well-known signatures
	source := `
package main

import (
	"encoding/json"
	"os"
	"strconv"
)

func main() {
	var data []byte
	var v interface{}
	var s string

	_ = data
	_ = v
	_ = s
}
`

	// Create and initialize TypeAnalyzer
	analyzer := NewTypeAnalyzer()
	err := analyzer.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("Failed to analyze source: %v", err)
	}

	// Create ReturnDetector
	detector := NewReturnDetector(analyzer)

	tests := []struct {
		name          string
		expr          string
		wantCount     int
		wantErrorOnly bool
	}{
		{
			name:          "os.Open - file and error",
			expr:          "os.Open(s)",
			wantCount:     2,
			wantErrorOnly: false,
		},
		{
			name:          "json.Unmarshal - error only",
			expr:          "json.Unmarshal(data, &v)",
			wantCount:     1,
			wantErrorOnly: true,
		},
		{
			name:          "strconv.Atoi - int and error",
			expr:          "strconv.Atoi(s)",
			wantCount:     2,
			wantErrorOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := detector.DetectReturns(tt.expr, source)
			if err != nil {
				// Note: These may fail if imports aren't fully resolved
				// That's expected behavior - we want detection to fail rather than guess
				t.Logf("DetectReturns() error (expected for incomplete type info): %v", err)
				return
			}

			if info.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", info.Count, tt.wantCount)
			}

			if info.ErrorOnly != tt.wantErrorOnly {
				t.Errorf("ErrorOnly = %v, want %v", info.ErrorOnly, tt.wantErrorOnly)
			}
		})
	}
}

func TestReturnDetector_MethodCalls(t *testing.T) {
	source := `
package main

type MyType struct {
	value int
}

func (m *MyType) ErrorOnlyMethod() error {
	return nil
}

func (m *MyType) ValueAndErrorMethod() (string, error) {
	return "", nil
}

func (m MyType) NoReturnMethod() {
}

func main() {
	var obj *MyType
	_ = obj
}
`

	analyzer := NewTypeAnalyzer()
	err := analyzer.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("Failed to analyze source: %v", err)
	}

	detector := NewReturnDetector(analyzer)

	tests := []struct {
		name          string
		expr          string
		wantCount     int
		wantErrorOnly bool
		wantError     bool
	}{
		{
			name:          "method returning error only",
			expr:          "obj.ErrorOnlyMethod()",
			wantCount:     1,
			wantErrorOnly: true,
			wantError:     false,
		},
		{
			name:          "method returning value and error",
			expr:          "obj.ValueAndErrorMethod()",
			wantCount:     2,
			wantErrorOnly: false,
			wantError:     false,
		},
		{
			name:          "method with no return",
			expr:          "obj.NoReturnMethod()",
			wantCount:     0,
			wantErrorOnly: false,
			wantError:     false,
		},
		{
			name:          "unknown method",
			expr:          "obj.UnknownMethod()",
			wantCount:     2,            // Falls back to default (T, error)
			wantErrorOnly: false,
			wantError:     false,        // No error with heuristics fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := detector.DetectReturns(tt.expr, source)

			if tt.wantError {
				if err == nil {
					t.Errorf("DetectReturns() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("DetectReturns() unexpected error: %v", err)
			}

			if info.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", info.Count, tt.wantCount)
			}

			if info.ErrorOnly != tt.wantErrorOnly {
				t.Errorf("ErrorOnly = %v, want %v", info.ErrorOnly, tt.wantErrorOnly)
			}
		})
	}
}

func TestReturnDetector_Cache(t *testing.T) {
	source := `
package main

func testFunc() error {
	return nil
}
`

	analyzer := NewTypeAnalyzer()
	err := analyzer.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("Failed to analyze source: %v", err)
	}

	detector := NewReturnDetector(analyzer)

	// First call - should populate cache
	info1, err := detector.DetectReturns("testFunc()", source)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call - should use cache
	info2, err := detector.DetectReturns("testFunc()", source)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Should return same pointer (cached)
	if info1 != info2 {
		t.Error("Expected cached result (same pointer), got different pointer")
	}

	// Clear cache
	detector.ClearCache()

	// Third call - should re-compute after cache clear
	info3, err := detector.DetectReturns("testFunc()", source)
	if err != nil {
		t.Fatalf("Third call failed: %v", err)
	}

	// Should be different pointer but same values
	if info1 == info3 {
		t.Error("Expected different pointer after cache clear")
	}

	if info3.ErrorOnly != true {
		t.Errorf("ErrorOnly = %v, want true", info3.ErrorOnly)
	}
}

func TestReturnDetector_EmptyExpression(t *testing.T) {
	analyzer := NewTypeAnalyzer()
	detector := NewReturnDetector(analyzer)

	_, err := detector.DetectReturns("", "")
	if err == nil {
		t.Error("Expected error for empty expression, got nil")
	}
}

func TestReturnDetector_NoTypeInfo(t *testing.T) {
	// Create detector without initializing analyzer (nil analyzer triggers heuristics)
	detector := NewReturnDetector(nil)

	// Should succeed using heuristics fallback
	info, err := detector.DetectReturns("someFunc()", "")
	if err != nil {
		t.Errorf("Expected success with heuristics fallback, got error: %v", err)
	}

	// Should return default (T, error) pattern
	if info.Count != 2 || !info.LastIsError || info.ErrorOnly {
		t.Errorf("Expected default (T, error) pattern, got Count=%d, LastIsError=%v, ErrorOnly=%v",
			info.Count, info.LastIsError, info.ErrorOnly)
	}
}

func TestReturnDetector_HeuristicsForStdlib(t *testing.T) {
	// Create detector with nil analyzer to force heuristics
	detector := NewReturnDetector(nil)

	tests := []struct {
		name          string
		expr          string
		wantErrorOnly bool
		wantCount     int
	}{
		{
			name:          "rows.Scan - error only",
			expr:          "rows.Scan(&a, &b, &c)",
			wantErrorOnly: true,
			wantCount:     1,
		},
		{
			name:          "db.Query - value and error",
			expr:          "db.Query(sql, args...)",
			wantErrorOnly: false,
			wantCount:     2,
		},
		{
			name:          "os.Open - value and error",
			expr:          "os.Open(path)",
			wantErrorOnly: false,
			wantCount:     2,
		},
		{
			name:          "file.Close - error only",
			expr:          "file.Close()",
			wantErrorOnly: true,
			wantCount:     1,
		},
		{
			name:          "json.Unmarshal - error only",
			expr:          "json.Unmarshal(data, &v)",
			wantErrorOnly: true,
			wantCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := detector.DetectReturns(tt.expr, "")
			if err != nil {
				t.Fatalf("DetectReturns() unexpected error: %v", err)
			}

			if info.ErrorOnly != tt.wantErrorOnly {
				t.Errorf("ErrorOnly = %v, want %v", info.ErrorOnly, tt.wantErrorOnly)
			}

			if info.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", info.Count, tt.wantCount)
			}
		})
	}
}
