package preprocessor

// TODO(ast-migration): This file uses regex-based transformations which are fragile.
// MIGRATE TO: pkg/ast/let.go with LetDecl AST node
// See: ai-docs/AST_MIGRATION.md for migration plan
// DO NOT fix regex bugs - implement AST-based solution instead

import (
	"regexp"
)

// Package-level compiled regex - LEGACY, TO BE REPLACED WITH AST
var (
	// Match: let identifier(s) [: type] = expression
	// Handles:
	//   - Single: let x = 5
	//   - Multiple: let x, y, z = func()
	//   - With type: let name: string = "hello"
	//   - With complex type: let opt: Option<int> = Some(42)
	// Captures identifiers and optional type annotation
	letPattern = regexp.MustCompile(`\blet\s+([\w\s,]+?)(?:\s*:\s*[^=]+?)?\s*=`)

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
// Converts: let x = value → x := value
// Converts: let identifier Type → var identifier Type
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

	// Step 1: Transform assignments: let x = value → x := value
	result := letPattern.ReplaceAll(source, []byte("$1 :="))

	// Step 2: Transform declarations without initialization: let identifier Type → var identifier Type
	// Preserve trailing whitespace with $3
	result = letDeclPattern.ReplaceAll(result, []byte("var $1 $2$3"))

	return result, nil, nil
}
