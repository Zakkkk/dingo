package typeloader

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractImports(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected []string
	}{
		{
			name: "single import",
			source: `package main
import "fmt"
func main() {}`,
			expected: []string{"fmt"},
		},
		{
			name: "multiple single imports",
			source: `package main
import "fmt"
import "os"
import "strings"
func main() {}`,
			expected: []string{"fmt", "os", "strings"},
		},
		{
			name: "grouped imports",
			source: `package main
import (
	"fmt"
	"os"
	"strings"
)
func main() {}`,
			expected: []string{"fmt", "os", "strings"},
		},
		{
			name: "mixed single and grouped",
			source: `package main
import "fmt"
import (
	"os"
	"strings"
)
func main() {}`,
			expected: []string{"fmt", "os", "strings"},
		},
		{
			name: "with aliases",
			source: `package main
import (
	"fmt"
	io "io/ioutil"
	"os"
)
func main() {}`,
			expected: []string{"fmt", "io/ioutil", "os"},
		},
		{
			name: "third-party packages",
			source: `package main
import (
	"database/sql"
	"github.com/lib/pq"
	"myproject/repository"
)
func main() {}`,
			expected: []string{"database/sql", "github.com/lib/pq", "myproject/repository"},
		},
		{
			name: "no imports",
			source: `package main
func main() {
	println("hello")
}`,
			expected: []string{},
		},
		{
			name: "with blank imports",
			source: `package main
import (
	_ "database/sql"
	"fmt"
)
func main() {}`,
			expected: []string{"database/sql", "fmt"},
		},
		{
			name: "with dot imports",
			source: `package main
import (
	. "fmt"
	"os"
)
func main() {}`,
			expected: []string{"fmt", "os"},
		},
		{
			name: "with comments",
			source: `package main
import (
	"fmt"  // for printing
	"os"   // file operations
)
func main() {}`,
			expected: []string{"fmt", "os"},
		},
		{
			name: "dingo syntax with type annotations",
			source: `package main
import (
	"fmt"
	"os"
)
func readFile(path: string) ([]byte, error) {
	return os.ReadFile(path)?
}`,
			expected: []string{"fmt", "os"},
		},
		{
			name: "duplicate imports",
			source: `package main
import "fmt"
import "fmt"
import (
	"fmt"
	"os"
)`,
			expected: []string{"fmt", "os"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractImports([]byte(tt.source))
			if err != nil {
				t.Fatalf("ExtractImports() error = %v", err)
			}

			// Sort both for comparison (order doesn't matter)
			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractImports() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractImportsWithAliases(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected map[string]string
	}{
		{
			name: "regular imports",
			source: `package main
import (
	"fmt"
	"os"
)`,
			expected: map[string]string{
				"fmt": "fmt",
				"os":  "os",
			},
		},
		{
			name: "with explicit aliases",
			source: `package main
import (
	"fmt"
	io "io/ioutil"
	sql "database/sql"
)`,
			expected: map[string]string{
				"fmt": "fmt",
				"io":  "io/ioutil",
				"sql": "database/sql",
			},
		},
		{
			name: "with blank import",
			source: `package main
import (
	_ "database/sql"
	"fmt"
)`,
			expected: map[string]string{
				"_":   "database/sql",
				"fmt": "fmt",
			},
		},
		{
			name: "with dot import",
			source: `package main
import (
	. "fmt"
	"os"
)`,
			expected: map[string]string{
				".":  "fmt",
				"os": "os",
			},
		},
		{
			name: "package with multiple path components",
			source: `package main
import (
	"database/sql"
	"net/http"
)`,
			expected: map[string]string{
				"sql":  "database/sql",
				"http": "net/http",
			},
		},
		{
			name: "third-party with alias",
			source: `package main
import (
	pq "github.com/lib/pq"
	"github.com/gorilla/mux"
)`,
			expected: map[string]string{
				"pq":  "github.com/lib/pq",
				"mux": "github.com/gorilla/mux",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractImportsWithAliases([]byte(tt.source))
			if err != nil {
				t.Fatalf("ExtractImportsWithAliases() error = %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractImportsWithAliases() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractImportsRegexFallback(t *testing.T) {
	// Test cases that might fail go/parser but work with regex
	tests := []struct {
		name     string
		source   string
		expected []string
	}{
		{
			name: "invalid go syntax but valid imports",
			source: `package main
import (
	"fmt"
	"os"
)
// Dingo-specific syntax that breaks parser
func test(x: int)? {}`,
			expected: []string{"fmt", "os"},
		},
		{
			name: "imports with tabs and spaces",
			source: `package main
import	(
		"fmt"
	"os"
)`,
			expected: []string{"fmt", "os"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractImportsRegex([]byte(tt.source))
			if err != nil {
				t.Fatalf("extractImportsRegex() error = %v", err)
			}

			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("extractImportsRegex() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDeduplicateImports(t *testing.T) {
	tests := []struct {
		name     string
		imports  []string
		expected []string
	}{
		{
			name:     "no duplicates",
			imports:  []string{"fmt", "os", "strings"},
			expected: []string{"fmt", "os", "strings"},
		},
		{
			name:     "with duplicates",
			imports:  []string{"fmt", "os", "fmt", "strings", "os"},
			expected: []string{"fmt", "os", "strings"},
		},
		{
			name:     "empty",
			imports:  []string{},
			expected: []string{},
		},
		{
			name:     "single",
			imports:  []string{"fmt"},
			expected: []string{"fmt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeduplicateImports(tt.imports)

			// Sort for comparison (order doesn't matter for deduplication)
			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DeduplicateImports() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkExtractImports(b *testing.B) {
	source := []byte(`package main
import (
	"fmt"
	"os"
	"strings"
	"database/sql"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)
func main() {}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractImports(source)
	}
}

func BenchmarkExtractImportsRegex(b *testing.B) {
	source := []byte(`package main
import (
	"fmt"
	"os"
	"strings"
	"database/sql"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)
func main() {}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = extractImportsRegex(source)
	}
}

func TestFormatImportError(t *testing.T) {
	err := FormatImportError("github.com/unknown/package", nil)
	if err == nil {
		t.Error("FormatImportError() should return non-nil error")
	}

	errMsg := err.Error()
	if !containsAll(errMsg, "github.com/unknown/package", "go get", "go mod download") {
		t.Errorf("FormatImportError() message missing expected hints: %s", errMsg)
	}
}

// Helper function
func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !contains(s, substr) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
