package refactor

import (
	"bytes"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// NilCheckDetector implements R002: prefer-match-nil
//
// Detects patterns like:
//
//	if x != nil {
//	    use(x)
//	} else {
//	    fallback()
//	}
//
// Suggests using match with Option instead:
//
//	match x {
//	    Some(v) => use(v)
//	    None    => fallback()
//	}
type NilCheckDetector struct{}

func (d *NilCheckDetector) Code() string { return "R002" }
func (d *NilCheckDetector) Name() string { return "prefer-match-nil" }
func (d *NilCheckDetector) Doc() string {
	return "Suggests using match with Option instead of nil checks with both branches"
}

func (d *NilCheckDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	// Use Go's standard parser to get full AST with function bodies
	goFile, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk the Go AST looking for if statements with nil checks
	ast.Inspect(goFile, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}

		// Check if this is a nil comparison (x != nil or x == nil)
		varName, isNotNil := extractNilCheck(ifStmt.Cond)
		if varName == "" {
			return true
		}

		// Must have both branches (if and else) for full refactoring
		if ifStmt.Else == nil {
			return true
		}

		// Generate the refactoring suggestion
		pos := fset.Position(ifStmt.Pos())
		end := fset.Position(ifStmt.End())

		// Extract the code blocks
		thenBlock := extractBlock(ifStmt.Body, src, fset)
		elseBlock := extractElseBlock(ifStmt.Else, src, fset)

		// Build the match expression
		var matchCode strings.Builder
		matchCode.WriteString("match ")
		matchCode.WriteString(varName)
		matchCode.WriteString(" {\n")

		if isNotNil {
			// if x != nil { ... } else { ... }
			// becomes: Some(x) => { ... }, None => { ... }
			matchCode.WriteString("\tSome(")
			matchCode.WriteString(varName)
			matchCode.WriteString(") => ")
			matchCode.WriteString(thenBlock)
			matchCode.WriteString("\n\tNone => ")
			matchCode.WriteString(elseBlock)
		} else {
			// if x == nil { ... } else { ... }
			// becomes: None => { ... }, Some(x) => { ... }
			matchCode.WriteString("\tNone => ")
			matchCode.WriteString(thenBlock)
			matchCode.WriteString("\n\tSome(")
			matchCode.WriteString(varName)
			matchCode.WriteString(") => ")
			matchCode.WriteString(elseBlock)
		}
		matchCode.WriteString("\n}")

		diagnostics = append(diagnostics, analyzer.Diagnostic{
			Pos:      pos,
			End:      end,
			Message:  "Consider using match with Option instead of nil check",
			Severity: analyzer.SeverityHint,
			Code:     "R002",
			Category: "refactor",
			Fixes: []analyzer.Fix{{
				Title:       "Replace with match expression",
				IsPreferred: true,
				Edits: []analyzer.TextEdit{{
					Pos:     pos,
					End:     end,
					NewText: matchCode.String(),
				}},
			}},
		})

		return true
	})

	return diagnostics
}

// GuardDetector implements R004: prefer-guard
//
// Detects early-return nil check patterns like:
//
//	if config == nil {
//	    return errors.New("config required")
//	}
//	// config used here
//
// Suggests using guard:
//
//	guard cfg = config else {
//	    return errors.New("config required")
//	}
type GuardDetector struct{}

func (d *GuardDetector) Code() string { return "R004" }
func (d *GuardDetector) Name() string { return "prefer-guard" }
func (d *GuardDetector) Doc() string {
	return "Suggests using guard for early-return nil checks"
}

func (d *GuardDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	// Use Go's standard parser to get full AST with function bodies
	goFile, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk the Go AST looking for if statements with early returns
	ast.Inspect(goFile, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}

		// Must have no else clause (early return pattern)
		if ifStmt.Else != nil {
			return true
		}

		// Check if this is a nil comparison (x == nil)
		varName, isNotNil := extractNilCheck(ifStmt.Cond)
		if varName == "" {
			return true
		}

		// For guard-let, we want "if x == nil { return }" pattern
		// not "if x != nil { ... }" pattern
		if isNotNil {
			return true
		}

		// Check if the body contains an early return
		if !containsReturn(ifStmt.Body) {
			return true
		}

		// Generate the refactoring suggestion
		pos := fset.Position(ifStmt.Pos())
		end := fset.Position(ifStmt.End())

		// Extract the return block
		returnBlock := extractBlock(ifStmt.Body, src, fset)

		// Build the guard-let expression
		var guardCode strings.Builder
		guardCode.WriteString("guard ")
		guardCode.WriteString(varName)
		guardCode.WriteString(" = ")
		guardCode.WriteString(varName)
		guardCode.WriteString(" else ")
		guardCode.WriteString(returnBlock)

		diagnostics = append(diagnostics, analyzer.Diagnostic{
			Pos:      pos,
			End:      end,
			Message:  "Consider using guard for early-return nil check",
			Severity: analyzer.SeverityHint,
			Code:     "R004",
			Category: "refactor",
			Fixes: []analyzer.Fix{{
				Title:       "Replace with guard",
				IsPreferred: true,
				Edits: []analyzer.TextEdit{{
					Pos:     pos,
					End:     end,
					NewText: guardCode.String(),
				}},
			}},
		})

		return true
	})

	return diagnostics
}

// Helper functions

// extractNilCheck extracts the variable name from a nil comparison expression.
// Returns (varName, isNotNil) where isNotNil is true for "x != nil" and false for "x == nil".
// Returns ("", false) if not a nil check.
func extractNilCheck(expr ast.Expr) (varName string, isNotNil bool) {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return "", false
	}

	// Check for x != nil or x == nil
	if bin.Op != token.NEQ && bin.Op != token.EQL {
		return "", false
	}

	// Extract variable name and nil
	var ident *ast.Ident
	var nilIdent *ast.Ident

	// Check both orders: x != nil or nil != x
	if id, ok := bin.X.(*ast.Ident); ok {
		if nid, ok := bin.Y.(*ast.Ident); ok && nid.Name == "nil" {
			ident = id
			nilIdent = nid
		}
	}
	if ident == nil {
		if nid, ok := bin.X.(*ast.Ident); ok && nid.Name == "nil" {
			if id, ok := bin.Y.(*ast.Ident); ok {
				ident = id
				nilIdent = nid
			}
		}
	}

	if ident == nil || nilIdent == nil {
		return "", false
	}

	return ident.Name, bin.Op == token.NEQ
}

// extractBlock extracts the source code for a block statement
func extractBlock(block *ast.BlockStmt, src []byte, fset *token.FileSet) string {
	if block == nil {
		return "{}"
	}

	start := fset.Position(block.Pos())
	end := fset.Position(block.End())

	// Convert positions to byte offsets
	startOffset := findByteOffset(src, start)
	endOffset := findByteOffset(src, end)

	if startOffset < 0 || endOffset < 0 || startOffset >= endOffset {
		return "{}"
	}

	blockCode := string(src[startOffset:endOffset])
	return strings.TrimSpace(blockCode)
}

// extractElseBlock extracts the source code for an else clause
func extractElseBlock(elseStmt ast.Stmt, src []byte, fset *token.FileSet) string {
	if elseStmt == nil {
		return "{}"
	}

	// Handle both else { ... } and else if { ... }
	switch e := elseStmt.(type) {
	case *ast.BlockStmt:
		return extractBlock(e, src, fset)
	case *ast.IfStmt:
		start := fset.Position(e.Pos())
		end := fset.Position(e.End())
		startOffset := findByteOffset(src, start)
		endOffset := findByteOffset(src, end)
		if startOffset < 0 || endOffset < 0 || startOffset >= endOffset {
			return "{}"
		}
		return strings.TrimSpace(string(src[startOffset:endOffset]))
	default:
		return "{}"
	}
}

// containsReturn checks if a block contains a return statement
func containsReturn(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}

	for _, stmt := range block.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// findByteOffset converts a token.Position to a byte offset in the source
func findByteOffset(src []byte, pos token.Position) int {
	if pos.Line <= 0 || pos.Column <= 0 {
		return -1
	}

	line := 1
	col := 1
	for i, b := range src {
		if line == pos.Line && col == pos.Column {
			return i
		}
		if b == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	// If we reached the end of the file
	if line == pos.Line && col == pos.Column {
		return len(src)
	}

	return -1
}

// findByteOffsetOld is a simpler version for testing
func findByteOffsetOld(src []byte, pos token.Position) int {
	lines := bytes.Split(src, []byte("\n"))
	if pos.Line < 1 || pos.Line > len(lines) {
		return -1
	}

	offset := 0
	for i := 0; i < pos.Line-1; i++ {
		offset += len(lines[i]) + 1 // +1 for newline
	}

	offset += pos.Column - 1
	return offset
}
