package parser

import (
	"go/parser"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

type simpleParser struct {
	mode Mode
}

func newParticipleParser(mode Mode) Parser {
	return &simpleParser{mode: mode}
}

func (p *simpleParser) ParseFile(fset *token.FileSet, filename string, src []byte) (*dingoast.File, error) {
	var goCode []byte

	// Check if caller has disabled preprocessing (SkipPreprocess mode)
	// This is used when the caller (cmd/dingo) has already preprocessed the source
	if p.mode&SkipPreprocess != 0 {
		// Source is already valid Go - use directly
		goCode = src
	} else {
		// Preprocess Dingo syntax to valid Go
		// This is the default for backward compatibility with tests
		prep := preprocessor.New(src)
		goSource, _, err := prep.Process()
		if err != nil {
			return nil, err
		}
		goCode = []byte(goSource)
	}

	// Use go/parser to parse the Go code
	var parserMode parser.Mode
	if p.mode&ParseComments != 0 {
		parserMode |= parser.ParseComments
	}
	if p.mode&AllErrors != 0 {
		parserMode |= parser.AllErrors
	}

	file, err := parser.ParseFile(fset, filename, goCode, parserMode)
	if err != nil {
		return nil, err
	}

	return &dingoast.File{File: file}, nil
}

func (p *simpleParser) ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
	// Not implemented for now
	return nil, nil
}
