package transpiler

import (
	"go/scanner"
	"go/token"
)

// ImportCollector extracts imports from Dingo source files using go/scanner.
// This is a fast operation (~1ms per file) compared to full parsing.
//
// CLAUDE.md COMPLIANT: Uses token-based scanning, not byte manipulation.
type ImportCollector struct{}

// CollectImports scans a Dingo source file and extracts all import paths.
// Returns a slice of import paths (e.g., ["dgo", "github.com/example/util"]).
//
// This function handles:
// - Single-line imports: import "pkg"
// - Multi-line imports: import ( "pkg1" "pkg2" )
// - Aliased imports: import alias "pkg" (extracts just the path)
func (c *ImportCollector) CollectImports(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var s scanner.Scanner
	// Use error handler that ignores errors - Dingo syntax may cause scanner errors
	s.Init(file, src, func(pos token.Position, msg string) {}, scanner.ScanComments)

	var imports []string
	inImportBlock := false
	inParenImport := false

	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// Detect 'import' keyword
		if tok == token.IMPORT {
			inImportBlock = true
			continue
		}

		// Detect start of multi-line import block
		if inImportBlock && tok == token.LPAREN {
			inParenImport = true
			continue
		}

		// Collect import paths (STRING tokens)
		if inImportBlock && tok == token.STRING {
			// Remove quotes: "dgo" -> dgo
			if len(lit) >= 2 && lit[0] == '"' && lit[len(lit)-1] == '"' {
				path := lit[1 : len(lit)-1]
				imports = append(imports, path)
			}
		}

		// Detect end of import block
		if inImportBlock && tok == token.RPAREN {
			inImportBlock = false
			inParenImport = false
		}

		// Single-line import ends at semicolon (go/scanner inserts virtual semicolons)
		if inImportBlock && !inParenImport && tok == token.SEMICOLON {
			inImportBlock = false
		}
	}

	return imports, nil
}
