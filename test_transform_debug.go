package main

import (
	"fmt"
	"go/scanner"
	"go/token"
)

func main() {
	src := []byte(`add := (x: int, y: int): int => x + y`)

	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	parenDepth := 0
	inParamList := false
	
	fmt.Println("Tokens:")
	prevTok := token.ILLEGAL
	prevLit := ""
	
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		
		// Track parentheses for parameter context
		if tok == token.LPAREN {
			if prevTok == token.IDENT || prevTok == token.RBRACK || prevTok == token.FUNC {
				inParamList = true
				fmt.Printf("  Setting inParamList=true after %s\n", prevLit)
			}
			parenDepth++
		}
		if tok == token.RPAREN {
			parenDepth--
			if parenDepth == 0 {
				inParamList = false
				fmt.Printf("  Setting inParamList=false\n")
			}
		}
		
		offset := file.Offset(pos)
		fmt.Printf("%3d: %-10s %-10s inParamList=%v parenDepth=%d\n", 
			offset, tok, lit, inParamList, parenDepth)
		
		prevTok = tok
		prevLit = lit
	}
}
