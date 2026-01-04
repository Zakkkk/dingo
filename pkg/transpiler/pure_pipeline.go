package transpiler

import (
	"bytes"
	"fmt"
	goparser "go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

/*
DINGO TRANSPILATION PIPELINE (v4 Architecture - No //line directives)
====================================================================

This file implements the main transpilation pipeline from .dingo to .go.

DESIGN PRINCIPLE: Generated Go code should look like human-written code.
//line directives are NOT emitted - position mapping is handled via .dmap files.

POSITION TRACKING FLOW:

  .dingo source
      |
      v
  pkg/tokenizer/Scanner
      - Creates token.FileSet for Dingo
      - Scanner.Pos() returns token.Pos
      |
      v
  pkg/parser/Parser (Pratt-based)
      - Produces Dingo AST nodes
      - Each node has Pos()/End() -> token.Pos
      - KEY: These positions are PRESERVED through all transforms
      |
      v
  pkg/codegen/* (clean Go output)
      - Transforms AST to Go code text
      - NO //line directives - clean, human-readable output
      - Records transforms via PositionTracker using token.Pos
      |
      v
  go/parser + go/printer
      - Standard Go formatting
      |
      v
  .dmap file generation
      - Contains bidirectional Dingo↔Go position mappings
      - LSP uses this for hover, go-to-definition, diagnostics translation

LSP POSITION MAPPING (via .dmap files):

  gopls analyzes .go file
      |
      v
  Reports diagnostic at .go:LINE:COL
      |
      v
  pkg/lsp/translator.go translates using .dmap
      |
      v
  LSP client shows error in .dingo editor

WHY NO //line DIRECTIVES:

  1. Generated code looks human-written (main principle)
  2. .dmap files provide complete bidirectional mapping
  3. LSP translator handles all position translation
  4. Cleaner diffs in version control
  5. Easier to read and debug generated code

WHY token.Pos INSTEAD OF BYTE OFFSETS:

  The old TransformTracker used byte arithmetic:
    goBytePos += untransformedLen  // FRAGILE: breaks after go/printer reformats!

  The new PositionTracker stores token.Pos from Dingo AST:
    tracker.RecordTransform(node.Pos(), node.End(), "lambda")

  Then resolves AFTER go/printer:
    pos := fset.Position(transform.DingoPos)  // Always accurate!

  Key insight: Dingo AST positions survive the entire pipeline because
  we store the token.Pos value, not a byte offset that becomes stale.
*/

// PureASTTranspile uses AST-based transformation for all Dingo features.
//
// Currently handles:
// - Enums: enum Name { Variant } → Go interface pattern
// - Let declarations: let x = expr → x := expr
// - Lambdas: |x| expr → func(x) { return expr }
// - Match expressions: match x { Pattern => result } → type switch
// - Error propagation: x? → error handling code
// - Ternary: cond ? a : b → inline if
// - Null coalescing: a ?? b → nil check
// - Safe navigation: x?.field → safe access
// - Tuples: (a, b) → struct literal
//
// Pipeline:
// 1. Transform Dingo syntax to Go using AST-based codegens (pkg/ast/transform.go)
// 2. Parse transformed Go with standard go/parser
// 3. Run go/types to infer types (optional)
// 4. Lambda type inference from call context
// 5. Rewrite interface{} placeholders with actual types
// 6. Print Go AST to source
//
// Source mappings are tracked during transformation for LSP integration.
func PureASTTranspile(source []byte, filename string) ([]byte, error) {
	return PureASTTranspileWithOptions(source, filename, true)
}

// PureASTTranspileWithOptions transpiles with optional type inference.
// Set inferTypes to false to disable type inference (faster but uses interface{}).
func PureASTTranspileWithOptions(source []byte, filename string, inferTypes bool) ([]byte, error) {
	result, err := PureASTTranspileWithMappings(source, filename, inferTypes)
	if err != nil {
		return nil, err
	}
	return result.GoCode, nil
}

// PureASTTranspileWithMappings transpiles and returns source mappings for LSP integration.
// This is the full-featured version that returns all transformation metadata.
func PureASTTranspileWithMappings(source []byte, filename string, inferTypes bool) (TranspileResult, error) {
	// Extract enum registry from ORIGINAL source (before transformation)
	// This is used by match expressions to prefix variant names correctly
	enumRegistry := dingoast.ExtractEnumRegistry(source)

	// Step 1: Transform Dingo syntax to Go using token-based transformations
	transformedSource, err := dingoast.TransformSource(source, filename)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("transform error: %w", err)
	}

	// Step 2a: Transform tuple type aliases (must run before Go parser)
	// Pattern: type Point = (int, int) → type Point = __tupleType2__(int, int)
	transformedSource, err = transformTupleTypeAliases(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple type alias error: %w", err)
	}

	// Step 2a1: Transform tuple destructuring (must run before tuple literals)
	// Pattern: (x, y) := expr → _ = __tupleDest2__("x:0", "y:1", expr)
	// This MUST run before transformTupleLiterals to avoid treating the LHS as a tuple literal
	transformedSource, err = transformTupleDestructuring(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple destructuring error: %w", err)
	}

	// Step 2a2: Transform tuple literals (must run before Go parser)
	// Pattern: (a, b) → __tuple2__(a, b)
	// Run in a loop to handle nested tuples: ((a, b), (c, d)) needs multiple passes
	// First pass: inner tuples (a, b) → __tuple2__(a, b)
	// Second pass: outer tuple (__tuple2__(a, b), __tuple2__(c, d)) → __tuple2__(__tuple2__(a, b), __tuple2__(c, d))
	const maxTupleLiteralPasses = 5 // Prevent infinite loops on malformed input
	for pass := 0; pass < maxTupleLiteralPasses; pass++ {
		prevLen := len(transformedSource)
		transformedSource, err = transformTupleLiterals(transformedSource)
		if err != nil {
			return TranspileResult{}, fmt.Errorf("tuple literal error (pass %d): %w", pass, err)
		}
		// If no changes were made, we're done
		if len(transformedSource) == prevLen {
			break
		}
	}

	// Step 2b: Transform tuples - Pass 1 (syntax to markers)
	transformedSource, err = transformTuplePass1(transformedSource)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("tuple pass 1 error: %w", err)
	}

	// Step 2.1: Transform statement-level error propagation (MUST run before expression transforms)
	// Note: //line directives are disabled - position mapping is handled via line/column mappings
	transformedSource, lineMappings, columnMappings, err := transformErrorPropStatements(transformedSource, source, "")
	if err != nil {
		return TranspileResult{}, fmt.Errorf("statement transform error: %w", err)
	}

	// Step 2.5: Transform guard statements (MUST run after error propagation, before expressions)
	// Note: //line directives are disabled - position mapping is handled via .dmap files
	transformedSource, err = transformGuardStatements(transformedSource, source, "")
	if err != nil {
		return TranspileResult{}, fmt.Errorf("guard transform error: %w", err)
	}

	// Step 3: Transform match/lambda expressions using AST-based codegen
	// Pass enum registry so match expressions can prefix variant names correctly
	// Also pass original source for accurate position mapping (earlier transforms shift positions)
	// Note: //line directives are disabled - position mapping is handled via .dmap files
	// Returns line mappings for multi-line transforms (safe nav, match, null coalesce)
	var astLineMappings []sourcemap.LineMapping
	transformedSource, astLineMappings, err = transformASTExpressions(transformedSource, enumRegistry, source, filename)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("AST transform error: %w", err)
	}

	// Merge AST line mappings with error prop line mappings
	// Both are metadata - no machine comments in the generated code
	lineMappings = append(lineMappings, astLineMappings...)

	// Step 3: Parse the transformed Go source with standard go/parser
	fset := token.NewFileSet()
	goFile, err := goparser.ParseFile(fset, filename, transformedSource, goparser.ParseComments)
	if err != nil {
		return TranspileResult{}, fmt.Errorf("parse error: %w", err)
	}

	// Step 3.1: Type-check the Go AST (needed for tuple Pass 2)
	pkgName := goFile.Name.Name
	typeChecker, typeErr := typechecker.New(fset, goFile, pkgName)

	// Step 3.2: Transform tuples - Pass 2 (resolve types and generate final structs)
	// This must happen AFTER go/types has analyzed the AST
	if typeErr == nil && typeChecker != nil {
		tuplePass2Result, tupleErr := transformTuplePass2(fset, goFile, typeChecker, transformedSource)
		if tupleErr == nil {
			// Update the transformed source with type-resolved tuple code
			transformedSource = tuplePass2Result
			// Re-parse with updated source
			goFile, err = goparser.ParseFile(fset, filename, transformedSource, goparser.ParseComments)
			if err != nil {
				return TranspileResult{}, fmt.Errorf("parse error after tuple pass 2: %w", err)
			}
			// Re-create type checker for subsequent steps
			typeChecker, typeErr = typechecker.New(fset, goFile, pkgName)
		}
		// If tuple Pass 2 fails, continue with markers (they're valid Go)
	}

	// Step 3.4: Transform bare enum variant identifiers to constructor calls
	// Example: `return Active` → `return NewStatusActive()` when Active is a Status variant
	// This must run AFTER tuple pass 2 (which may re-parse) but before type checking.
	if len(enumRegistry) > 0 {
		variantTransformer := NewEnumVariantTransformer(fset, goFile, enumRegistry)
		variantTransformer.Transform()
	}

	// Step 3.5: None inference - rewrite bare None/None() to None[T]()
	// This must run BEFORE QualifyDingoTypes so we can access the original type names
	noneTransformer := NewNoneInferenceTransformer(fset, goFile)
	if err := noneTransformer.Transform(); err != nil {
		return TranspileResult{}, fmt.Errorf("none inference: %w", err)
	}

	// Step 3.6: Inject dgo import if Result/Option types are detected
	InjectDgoImport(fset, goFile)

	// Step 4: Run type inference to replace interface{} with actual types
	var checker *typechecker.Checker
	if inferTypes {
		// Reuse type checker from tuple Pass 2 if available
		if typeErr == nil && typeChecker != nil {
			checker = typeChecker
		} else {
			// Create new type checker
			checker, err = typechecker.New(fset, goFile, pkgName)
		}
		if err != nil {
			// Type checker unavailable - will fall back to AST-based heuristics
			// for Result/Option wrapping (checks error variable names, function calls)
		}

		// Step 4.1: Multi-pass lambda type inference from call context
		// We run multiple passes because lambda inference may resolve types
		// that enable further inference in subsequent passes.
		// Example: Filter(users, |u| ...) → eligible has type []User
		//          Map(eligible, |u| ...) → now we can infer u is User
		// Without multi-pass, Map's arg type would be "invalid type" because
		// eligible's type depends on Filter's lambda being correctly typed first.
		if checker != nil {
			const maxPasses = 5 // Prevent infinite loops
			var lastInferrer *typechecker.LambdaTypeInferrer
			for pass := 0; pass < maxPasses; pass++ {
				lambdaInferrer := typechecker.NewLambdaTypeInferrer(fset, goFile, checker.Info())
				changed := lambdaInferrer.Infer()
				lastInferrer = lambdaInferrer
				if !changed {
					break // No more changes, stop iterating
				}

				// Re-type-check with updated AST to get fresh type info
				// This is necessary because after updating lambda types,
				// variables that previously had "invalid type" may now have
				// correct types.
				checker, err = typechecker.New(fset, goFile, pkgName)
				if err != nil {
					break // Type checker failed, stop
				}
			}

			// Step 4.1.1: Check for unresolved lambda types (fail-fast)
			// If any lambdas still have 'any' types after inference, error with helpful message
			if lastInferrer != nil {
				unresolved := lastInferrer.FindUnresolvedLambdas()
				if len(unresolved) > 0 {
					// Return structured error for the first unresolved lambda
					// (additional errors can be found after fixing the first one)
					u := unresolved[0]
					return TranspileResult{}, &TranspileError{
						File:    filename,
						Line:    u.Line,
						Col:     u.Column,
						Message: typechecker.FormatUnresolvedError(u),
					}
				}
			}
		}

		// Step 4.2: General type inference (IIFE return types, etc.)
		_, err = typechecker.RewriteSource(fset, goFile)
		if err != nil {
			// Type inference failed - continue without it
			// This is acceptable since interface{} is valid Go
		}
	}

	// Step 4.5: Wrap Result/Option return statements with dgo constructors
	wrapper := NewResultWrapperTransformer(fset, goFile, checker)
	wrapper.Transform()

	// Step 5: Print Go AST to source
	var buf bytes.Buffer
	cfg := printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 4,
	}
	if err := cfg.Fprint(&buf, fset, goFile); err != nil {
		return TranspileResult{}, fmt.Errorf("print error: %w", err)
	}

	// Final Go code - no //line directives, position mapping handled via mappings
	finalGoCode := buf.Bytes()

	// Step 5.5: Adjust line mappings for go/printer offset
	// go/printer may reformat comments (e.g., removing empty comment lines),
	// which shifts line numbers. We need to calculate the actual offset.
	lineMappings, columnMappings = adjustLineMappingsForPrinterOffset(finalGoCode, source, lineMappings, columnMappings)

	// Line mappings are generated during error propagation transformation.
	// They provide Dingo→Go line translation for the LSP semantic builder.
	// Column mappings provide column-level precision for hover/go-to-definition.

	// Return complete transpilation result
	return TranspileResult{
		GoCode:         finalGoCode,
		LineMappings:   lineMappings,   // From error prop transform (adjusted)
		ColumnMappings: columnMappings, // From error prop transform (adjusted)
		DingoSource:    source,
		GoAST:          goFile,
		Metadata: &TranspileMetadata{
			OriginalFile: filename,
		},
	}, nil
}

// fixLineDirectiveIndentation strips leading whitespace from //line directive lines.
// Go's parser REQUIRES //line directives to start at column 1 (no indentation).
// The go/printer adds indentation to comments (including //line), so we must fix them.
//
// Uses go/scanner to find COMMENT tokens - this is the token-based approach
// required by CLAUDE.md (no byte heuristics for position tracking).
func fixLineDirectiveIndentation(src []byte) []byte {
	// Use go/scanner to find //line directive comments
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	// Collect positions of //line directives that need fixing
	type lineDirective struct {
		lineStart int // byte offset of line start (after newline)
		dirStart  int // byte offset of directive (after indentation)
	}
	var directives []lineDirective

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// Check if this is a //line directive comment
		if tok == token.COMMENT && strings.HasPrefix(lit, "//line ") {
			// Use token.FileSet to get position info (not byte scanning)
			position := fset.Position(pos)
			offset := file.Offset(pos)

			// Calculate line start using column from FileSet
			// Column is 1-indexed, so line start is offset - (column - 1)
			lineStart := offset - (position.Column - 1)

			// Only fix if there's indentation (directive not at column 1)
			if position.Column > 1 {
				directives = append(directives, lineDirective{
					lineStart: lineStart,
					dirStart:  offset,
				})
			}
		}
	}

	// If no directives need fixing, return original
	if len(directives) == 0 {
		return src
	}

	// Build result by removing indentation before each directive
	// Process in reverse order so earlier offsets remain valid
	result := make([]byte, len(src))
	copy(result, src)

	// Track cumulative offset adjustment for reverse processing
	for i := len(directives) - 1; i >= 0; i-- {
		d := directives[i]
		indentLen := d.dirStart - d.lineStart
		// Remove bytes from lineStart to dirStart (the indentation)
		// Before: result[:lineStart] contains up to line start
		// After:  result[dirStart:] starts with //line directive
		newResult := make([]byte, 0, len(result)-indentLen)
		newResult = append(newResult, result[:d.lineStart]...)
		newResult = append(newResult, result[d.dirStart:]...)
		result = newResult
	}

	return result
}

// insertImportResetDirective adds a //line directive after the import block
// to reset line numbering to the correct Dingo position.
//
// When the transpiler injects imports (e.g., "github.com/MadAppGang/dingo/dgo"),
// or when imports expand from single-line to multi-line format, all subsequent
// line numbers shift. This causes hover and diagnostics to point to wrong lines
// in the .dingo file.
//
// Solution: Insert a //line directive after the import block that points to the
// first line of actual code in the original Dingo source.
//
// Uses go/scanner to find the end of imports (token-based, not byte heuristics).
func insertImportResetDirective(goCode []byte, filename string, dingoSource []byte) []byte {
	// Use go/scanner to find import block boundaries
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(goCode))

	var s scanner.Scanner
	s.Init(file, goCode, nil, scanner.ScanComments)

	// Find the last import statement
	// Pattern: import ( ... ) or import "..."
	// We need the position AFTER the closing paren or string literal
	var lastImportEnd token.Pos
	var inImportBlock bool
	var importBlockDepth int

	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}

		// Track import keyword
		if tok == token.IMPORT {
			// Check next token - if it's LPAREN, we're in a multi-line import block
			nextPos, nextTok, _ := s.Scan()
			if nextTok == token.LPAREN {
				inImportBlock = true
				importBlockDepth = 1
				lastImportEnd = nextPos
			} else if nextTok == token.STRING {
				// Single-line import: import "pkg"
				lastImportEnd = nextPos
			}
			continue
		}

		// Track paren depth in import blocks
		if inImportBlock {
			if tok == token.LPAREN {
				importBlockDepth++
			} else if tok == token.RPAREN {
				importBlockDepth--
				if importBlockDepth == 0 {
					inImportBlock = false
					lastImportEnd = pos
				}
			}
		}

		// If we find a non-import declaration after imports ended, stop scanning
		if !inImportBlock && lastImportEnd != token.NoPos {
			// Check if this is a declaration keyword (type, func, var, const)
			if tok == token.TYPE || tok == token.FUNC || tok == token.VAR || tok == token.CONST {
				break
			}
		}
	}

	// If no imports found, nothing to do
	if lastImportEnd == token.NoPos {
		return goCode
	}

	// Calculate position after import block in Go code
	importEndPosition := fset.Position(lastImportEnd)
	importEndLine := importEndPosition.Line

	// Calculate corresponding Dingo line
	// Strategy: Count import statements in Dingo source to determine offset
	dingoFset := token.NewFileSet()
	dingoFile := dingoFset.AddFile("", dingoFset.Base(), len(dingoSource))
	var dingoScanner scanner.Scanner
	dingoScanner.Init(dingoFile, dingoSource, nil, scanner.ScanComments)

	var dingoLastImportEnd token.Pos
	var dingoInImportBlock bool
	var dingoImportBlockDepth int

	for {
		pos, tok, _ := dingoScanner.Scan()
		if tok == token.EOF {
			break
		}

		if tok == token.IMPORT {
			nextPos, nextTok, _ := dingoScanner.Scan()
			if nextTok == token.LPAREN {
				dingoInImportBlock = true
				dingoImportBlockDepth = 1
				dingoLastImportEnd = nextPos
			} else if nextTok == token.STRING {
				dingoLastImportEnd = nextPos
			}
			continue
		}

		if dingoInImportBlock {
			if tok == token.LPAREN {
				dingoImportBlockDepth++
			} else if tok == token.RPAREN {
				dingoImportBlockDepth--
				if dingoImportBlockDepth == 0 {
					dingoInImportBlock = false
					dingoLastImportEnd = pos
				}
			}
		}

		if !dingoInImportBlock && dingoLastImportEnd != token.NoPos {
			if tok == token.TYPE || tok == token.FUNC || tok == token.VAR || tok == token.CONST {
				break
			}
		}
	}

	// If no imports in Dingo source, the first line of code is line 1 after package
	dingoLineAfterImports := 1
	if dingoLastImportEnd != token.NoPos {
		dingoEndPosition := dingoFset.Position(dingoLastImportEnd)
		// Find first non-blank line after import block in Dingo source
		// IMPORTANT: //line directives set position for the NEXT line.
		// If imports end at line 35, we need to find the first actual code line
		// (e.g., line 37 if line 36 is blank) to emit the correct directive.
		importEndLine := dingoEndPosition.Line
		dingoLineAfterImports = findFirstNonBlankLine(dingoSource, importEndLine)
	} else {
		// No imports in Dingo - find package declaration line
		dingoFset2 := token.NewFileSet()
		dingoFile2 := dingoFset2.AddFile("", dingoFset2.Base(), len(dingoSource))
		var pkgScanner scanner.Scanner
		pkgScanner.Init(dingoFile2, dingoSource, nil, scanner.ScanComments)

		for {
			pos, tok, _ := pkgScanner.Scan()
			if tok == token.EOF {
				break
			}
			if tok == token.PACKAGE {
				// Skip package name
				pkgScanner.Scan()
				pkgPos := dingoFset2.Position(pos)
				dingoLineAfterImports = pkgPos.Line + 1
				break
			}
		}
	}

	// Format //line directive
	directive := fmt.Sprintf("//line %s:%d:1\n", filename, dingoLineAfterImports)

	// Find newline after last import in Go code to insert directive
	// Scan forward from importEndLine to find good insertion point
	lines := bytes.Split(goCode, []byte("\n"))
	if importEndLine > len(lines) {
		importEndLine = len(lines)
	}

	// Find the blank line or first code line after imports
	insertLine := importEndLine
	for i := importEndLine; i < len(lines) && i < importEndLine+5; i++ {
		trimmed := bytes.TrimSpace(lines[i])
		// Insert before first non-empty, non-comment line
		if len(trimmed) > 0 && !bytes.HasPrefix(trimmed, []byte("//")) { // OK: content type check, not position
			insertLine = i
			break
		}
	}

	// Insert directive at the beginning of insertLine
	var result bytes.Buffer
	for i, line := range lines {
		if i == insertLine {
			result.WriteString(directive)
		}
		result.Write(line)
		if i < len(lines)-1 {
			result.WriteByte('\n')
		}
	}

	return result.Bytes()
}

// findFirstNonBlankLine finds the first non-blank, non-comment line after startLine.
// Returns startLine+1 if no non-blank line is found within a reasonable range.
// Lines are 1-indexed to match Go's token.Position conventions.
func findFirstNonBlankLine(source []byte, startLine int) int {
	lines := bytes.Split(source, []byte("\n"))

	// Search from startLine+1 (first line after the reference line)
	for i := startLine; i < len(lines) && i < startLine+10; i++ {
		trimmed := bytes.TrimSpace(lines[i])
		// Skip blank lines
		if len(trimmed) == 0 {
			continue
		}
		// Skip comment-only lines
		if bytes.HasPrefix(trimmed, []byte("//")) { // OK: content type check, not position
			continue
		}
		// Found a non-blank, non-comment line
		return i + 1 // Convert 0-indexed to 1-indexed
	}

	// Fallback: return next line after start
	return startLine + 1
}

// adjustLineMappingsForPrinterOffset fixes line mappings after go/printer runs.
//
// go/printer may reformat the code, changing line numbers. This function adjusts
// the line mappings to match the final Go output. It handles two cases:
//
// 1. If //line directives are present, use them for accurate mapping
// 2. If no //line directives, calculate the header offset from line counts
//
// The header offset accounts for lines removed/added by go/printer before the
// first transform (e.g., empty comment lines removed from the header).
//
// CLAUDE.md COMPLIANT: Uses go/scanner for position tracking, not byte manipulation.
func adjustLineMappingsForPrinterOffset(goCode, dingoSource []byte, lineMappings []sourcemap.LineMapping, columnMappings []sourcemap.ColumnMapping) ([]sourcemap.LineMapping, []sourcemap.ColumnMapping) {
	if len(lineMappings) == 0 {
		return lineMappings, columnMappings
	}

	// Build a map of Dingo line → Go line from //line directives in final output
	directiveMap := findLineDirectivePositions(goCode)

	// If we have directives, use them for accurate mapping
	if len(directiveMap) > 0 {
		return adjustWithDirectives(directiveMap, lineMappings, columnMappings)
	}

	// No directives - calculate header offset from the first mapping
	// The header offset is the difference between expected and actual Go line
	// for the first transform. This accounts for lines removed by go/printer.
	headerOffset := calculateHeaderOffset(goCode, dingoSource, lineMappings)

	// Adjust all mappings by the header offset
	adjustedLineMappings := make([]sourcemap.LineMapping, 0, len(lineMappings))
	for _, lm := range lineMappings {
		adjustedLineMappings = append(adjustedLineMappings, sourcemap.LineMapping{
			DingoLine:   lm.DingoLine,
			GoLineStart: lm.GoLineStart + headerOffset,
			GoLineEnd:   lm.GoLineEnd + headerOffset,
			Kind:        lm.Kind,
		})
	}

	adjustedColumnMappings := make([]sourcemap.ColumnMapping, 0, len(columnMappings))
	for _, cm := range columnMappings {
		adjustedColumnMappings = append(adjustedColumnMappings, sourcemap.ColumnMapping{
			DingoLine: cm.DingoLine,
			DingoCol:  cm.DingoCol,
			GoLine:    cm.GoLine + headerOffset,
			GoCol:     cm.GoCol,
			Length:    cm.Length,
			Kind:      cm.Kind,
		})
	}

	return adjustedLineMappings, adjustedColumnMappings
}

// adjustWithDirectives uses //line directives for accurate line mapping.
func adjustWithDirectives(directiveMap map[int]int, lineMappings []sourcemap.LineMapping, columnMappings []sourcemap.ColumnMapping) ([]sourcemap.LineMapping, []sourcemap.ColumnMapping) {
	adjustedLineMappings := make([]sourcemap.LineMapping, 0, len(lineMappings))
	for _, lm := range lineMappings {
		if goLine, found := directiveMap[lm.DingoLine]; found {
			codeLength := lm.GoLineEnd - lm.GoLineStart + 1
			adjustedLineMappings = append(adjustedLineMappings, sourcemap.LineMapping{
				DingoLine:   lm.DingoLine,
				GoLineStart: goLine + 1,
				GoLineEnd:   goLine + codeLength,
				Kind:        lm.Kind,
			})
		} else {
			adjustedLineMappings = append(adjustedLineMappings, lm)
		}
	}

	adjustedColumnMappings := make([]sourcemap.ColumnMapping, 0, len(columnMappings))
	for _, cm := range columnMappings {
		if goLine, found := directiveMap[cm.DingoLine]; found {
			adjustedColumnMappings = append(adjustedColumnMappings, sourcemap.ColumnMapping{
				DingoLine: cm.DingoLine,
				DingoCol:  cm.DingoCol,
				GoLine:    goLine + 1,
				GoCol:     cm.GoCol,
				Length:    cm.Length,
				Kind:      cm.Kind,
			})
		} else {
			adjustedColumnMappings = append(adjustedColumnMappings, cm)
		}
	}

	return adjustedLineMappings, adjustedColumnMappings
}

// calculateHeaderOffset determines the line offset between Dingo and Go sources
// caused by go/printer reformatting (e.g., removing empty comment lines).
//
// The algorithm counts lines before "package" in both sources. If Dingo has
// more lines before "package" than Go, those extra lines are the header offset.
//
// Returns negative offset if Go has fewer lines (common case).
func calculateHeaderOffset(goCode, dingoSource []byte, lineMappings []sourcemap.LineMapping) int {
	// Use go/scanner to find "package" keyword in both sources
	goPackageLine := findPackageLine(goCode)
	dingoPackageLine := findPackageLine(dingoSource)

	if goPackageLine == 0 || dingoPackageLine == 0 {
		return 0 // Can't determine offset
	}

	// Header offset = Go package line - Dingo package line
	// If Dingo has 17 lines before package and Go has 16, offset = -1
	// This means Go line N corresponds to Dingo line N+1
	return goPackageLine - dingoPackageLine
}

// findPackageLine returns the 1-indexed line number of the "package" keyword.
// Uses go/scanner for CLAUDE.md compliant position tracking.
func findPackageLine(source []byte) int {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(source))

	var s scanner.Scanner
	s.Init(file, source, nil, scanner.ScanComments)

	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.PACKAGE {
			return fset.Position(pos).Line
		}
	}
	return 0
}

// findLineDirectivePositions scans Go source for //line directives and returns
// a map of Dingo line → Go line where the directive appears.
//
// IMPORTANT: go/scanner respects //line directives and modifies position tracking.
// We use TWO FileSets: one for scanning (may be modified) and one for position
// lookup (stays pristine). This matches the pattern in pkg/lsp/translator.go.
//
// CLAUDE.md COMPLIANT: Uses go/scanner for position tracking.
func findLineDirectivePositions(goCode []byte) map[int]int {
	result := make(map[int]int)

	// Create two file sets:
	// 1. scannerFset - used by the scanner (will be modified by //line directives)
	// 2. lookupFset - for looking up physical line numbers (stays pristine)
	scannerFset := token.NewFileSet()
	scannerFile := scannerFset.AddFile("", scannerFset.Base(), len(goCode))

	lookupFset := token.NewFileSet()
	lookupFile := lookupFset.AddFile("", lookupFset.Base(), len(goCode))
	lookupFile.SetLinesForContent(goCode)

	var s scanner.Scanner
	s.Init(scannerFile, goCode, func(pos token.Position, msg string) {}, scanner.ScanComments)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT && len(lit) > 7 && lit[:7] == "//line " {
			// Get offset from scanner's position, then lookup physical line in pristine file
			offset := scannerFile.Offset(pos)
			goLine := lookupFset.Position(lookupFile.Pos(offset)).Line

			// Extract the Dingo line number from the directive
			// Format: //line path/to/file.dingo:LINE:COL
			directive := lit[7:]
			lastColon := -1
			for i := len(directive) - 1; i >= 0; i-- {
				if directive[i] == ':' {
					lastColon = i
					break
				}
			}
			if lastColon > 0 {
				secondLastColon := -1
				for i := lastColon - 1; i >= 0; i-- {
					if directive[i] == ':' {
						secondLastColon = i
						break
					}
				}
				if secondLastColon > 0 {
					var dingoLine int
					_, err := fmt.Sscanf(directive[secondLastColon+1:lastColon], "%d", &dingoLine)
					if err == nil && dingoLine > 0 {
						result[dingoLine] = goLine
					}
				}
			}
		}
	}

	return result
}

// findPackageLineWithScanner uses go/scanner to find the line number of the package declaration.
// Returns 0 if no package declaration is found.
//
// CLAUDE.md COMPLIANT: Uses token system for position tracking.
func findPackageLineWithScanner(source []byte) int {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(source))

	var s scanner.Scanner
	// Ignore errors - Dingo syntax extensions may cause scanner errors
	s.Init(file, source, func(pos token.Position, msg string) {}, scanner.ScanComments)

	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.PACKAGE {
			return fset.Position(pos).Line
		}
	}
	return 0
}
