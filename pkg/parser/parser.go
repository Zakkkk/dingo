// Package parser provides AST-based parsing for Dingo source code.
// It uses a Pratt parser for expressions and extends it for statements
// and declarations.
package parser

import (
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// Parser is the interface that all Dingo parsers must implement
type Parser interface {
	// ParseFile parses a single Dingo source file
	ParseFile(fset *token.FileSet, filename string, src []byte) (*dingoast.File, error)

	// ParseExpr parses a single expression (useful for REPL, testing)
	ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error)
}

// Mode controls parser behavior
type Mode uint

const (
	// ParseComments tells the parser to include comments in the AST
	ParseComments Mode = 1 << iota

	// Trace enables parser debugging output
	Trace

	// AllErrors reports all errors (not just the first 10)
	AllErrors
)

// ParseFile is a convenience function that uses the default parser
func ParseFile(fset *token.FileSet, filename string, src []byte, mode Mode) (*dingoast.File, error) {
	p := NewParser(mode)
	return p.ParseFile(fset, filename, src)
}

// ParseExpr is a convenience function that parses an expression
func ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
	p := NewParser(0)
	return p.ParseExpr(fset, expr)
}

// NewParser creates a new parser instance with the given mode
func NewParser(mode Mode) Parser {
	return &astParser{mode: mode}
}

// astParser implements Parser using the AST-based Pratt parser
type astParser struct {
	mode Mode
}

func (p *astParser) ParseFile(fset *token.FileSet, filename string, src []byte) (*dingoast.File, error) {
	// Create tokenizer for source
	tok := tokenizer.New(src)

	// Create statement parser (which includes expression parsing via Pratt)
	stmtParser := NewStmtParser(tok, fset)

	// Parse the file
	goFile, err := stmtParser.ParseFile()
	if err != nil {
		return nil, err
	}

	// Wrap in Dingo file
	return &dingoast.File{File: goFile}, nil
}

func (p *astParser) ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
	// Create tokenizer for expression
	tok := tokenizer.New([]byte(expr))

	// Create Pratt parser for expression parsing
	pratt := NewPrattParser(tok)

	// Parse expression
	result := pratt.ParseExpression(PrecLowest)
	if len(pratt.errors) > 0 {
		return nil, ErrorList(pratt.errors)
	}

	// Wrap ast.Expr in a DingoNode wrapper if needed
	if dn, ok := result.(dingoast.DingoNode); ok {
		return dn, nil
	}

	// For standard Go expressions, wrap in a generic node
	return &dingoast.ExprWrapper{DingoExpr: result}, nil
}
