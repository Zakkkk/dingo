package preprocessor

// TODO(ast-migration): This file uses regex-based transformations which are fragile.
// MIGRATE TO: pkg/ast/let.go with LetDecl AST node
// See: ai-docs/AST_MIGRATION.md for migration plan
// DO NOT fix regex bugs - implement AST-based solution instead

import (
	"fmt"
	"regexp"
	"strings"
)

// Package-level compiled regex - LEGACY, TO BE REPLACED WITH AST
var (
	// Match: let identifier(s) [: type] = expression
	// Handles:
	//   - Single: let x = 5
	//   - Multiple: let x, y, z = func()
	//   - With type: let name: string = "hello"
	//   - With complex type: let opt: Option<int> = Some(42)
	// Captures identifiers with type annotation preserved (capture group 1)
	letPattern = regexp.MustCompile(`\blet\s+([\w\s,]+(?:\s*:\s*[^=]+)?)\s*=`)

	// Match: let identifier Type or let identifier: Type (declaration without initialization)
	// Handles: let action Action, let x: int
	// Transform to: var action Action, var x int
	// Captures trailing whitespace to preserve formatting
	// Pattern matches: let + identifier + optional colon + type + (space or end)
	letDeclPattern = regexp.MustCompile(`\blet\s+([\w]+)\s*:?\s*([\w\[\]*<>]+)(\s|$)`)
)

// KeywordProcessor converts Dingo keywords to Go keywords
type KeywordProcessor struct{}

// NewKeywordProcessor creates a new keyword processor
func NewKeywordProcessor() *KeywordProcessor {
	return &KeywordProcessor{}
}

// Name returns the processor name
func (k *KeywordProcessor) Name() string {
	return "keywords"
}

// Process transforms Dingo keywords to Go keywords
// Converts: let x = value → x := value // dingo:let:x
// Converts: let identifier Type → var identifier Type // dingo:let:identifier
func (k *KeywordProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// CRITICAL: Order matters to prevent regex backtracking bugs!
	//
	// The letDeclPattern regex can backtrack and incorrectly split identifiers
	// when followed by `=`. For example, `let status = func() string` would
	// incorrectly become `var statu s = func() string` because the regex
	// backtracks to make `([\w]+)` capture "statu" and `([\w\[\]*<>]+)` capture "s".
	//
	// Fix: Process assignments FIRST to consume the `let x =` pattern,
	// then process declarations (which have no `=`).

	// Step 1: Transform assignments with marker: let x = value → x := value // dingo:let:x
	result := letPattern.ReplaceAllFunc(source, func(match []byte) []byte {
		submatches := letPattern.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		vars := strings.TrimSpace(string(submatches[1])) // "x" or "x, y, z" or "name: string"

		// Extract variable names (remove type annotations if present)
		// "name: string" → "name"
		// "x, y, z" → "x,y,z"
		varNames := extractVarNames(vars)

		// Generate transformed output with marker
		// IMPORTANT: Preserve type annotations in output (e.g., "name: string := ...")
		// Note: Space before value comes from original source (not part of regex match)
		return []byte(fmt.Sprintf("%s := // dingo:let:%s", vars, varNames))
	})

	// Step 2: Transform declarations with marker: let identifier Type → var identifier Type // dingo:let:identifier
	result = letDeclPattern.ReplaceAllFunc(result, func(match []byte) []byte {
		submatches := letDeclPattern.FindSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		varName := string(submatches[1])
		typeName := string(submatches[2])
		trailing := string(submatches[3])

		// Add space before marker comment, then preserve trailing character (space/newline)
		return []byte(fmt.Sprintf("var %s %s // dingo:let:%s%s", varName, typeName, varName, trailing))
	})

	return result, nil, nil
}

// extractVarNames extracts clean variable names for marker comments
// Input: "x" → Output: "x"
// Input: "x, y, z" → Output: "x,y,z"
// Input: "name: string" → Output: "name"
func extractVarNames(vars string) string {
	// Remove type annotation if present (everything after ':')
	if idx := strings.Index(vars, ":"); idx != -1 {
		vars = vars[:idx]
	}

	// Split by comma and trim spaces
	parts := strings.Split(vars, ",")
	cleaned := make([]string, len(parts))
	for i, p := range parts {
		cleaned[i] = strings.TrimSpace(p)
	}

	return strings.Join(cleaned, ",")
}
