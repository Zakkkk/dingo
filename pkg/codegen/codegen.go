// Package codegen generates Go source code from Dingo AST nodes.
//
// CRITICAL: This package must NEVER parse source bytes.
// All input must be pre-parsed AST nodes from pkg/parser/.
//
// Forbidden patterns:
//   - bytes.Index, strings.Index, strings.Contains
//   - regexp.MustCompile, regexp.Match
//   - Character scanning loops
//
// Correct pattern:
//   func Generate(expr *ast.MatchExpr) ast.CodeGenResult
//   - Input: AST node
//   - Output: Generated Go + source mappings
package codegen

import (
	"bytes"
	"strconv"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// Generator interface for all codegens
type Generator interface {
	Generate() ast.CodeGenResult
}

// BaseGenerator provides common functionality for all code generators.
// All specific generators (match, lambda, etc.) should embed this.
type BaseGenerator struct {
	Buf     bytes.Buffer      // Output buffer for generated Go code
	MB      *ast.MappingBuilder // Source mapping builder
	Counter int               // For unique variable naming (start at 1, not 0)
}

// NewBaseGenerator creates a base generator with initialized state.
func NewBaseGenerator() *BaseGenerator {
	return &BaseGenerator{
		MB:      ast.NewMappingBuilder(),
		Counter: 1, // First variable has no number, second is "1", etc.
	}
}

// TempVar generates unique temporary variable names following Go naming convention.
// Returns "base" for first, "base1" for second, "base2" for third, etc.
//
// Examples:
//   - TempVar("tmp")  → "tmp"  (first)
//   - TempVar("tmp")  → "tmp1" (second)
//   - TempVar("err")  → "err"  (first)
//   - TempVar("err")  → "err1" (second)
func (g *BaseGenerator) TempVar(base string) string {
	// First variable has no number, subsequent have numbers
	if g.Counter == 1 {
		g.Counter++
		return base
	}
	name := base + strconv.Itoa(g.Counter-1)
	g.Counter++
	return name
}

// Write writes a string to the output buffer.
func (g *BaseGenerator) Write(s string) {
	g.Buf.WriteString(s)
}

// WriteByte writes a single byte to the output buffer.
func (g *BaseGenerator) WriteByte(b byte) {
	g.Buf.WriteByte(b)
}

// Result returns the final CodeGenResult with generated code and mappings.
func (g *BaseGenerator) Result() ast.CodeGenResult {
	return ast.CodeGenResult{
		Output:   g.Buf.Bytes(),
		Mappings: g.MB.Build(),
	}
}
