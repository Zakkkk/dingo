package transpiler

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/token"
	gotoken "go/token"
	"sort"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// tupleNodeWithPos holds a parsed tuple AST node with position information
type tupleNodeWithPos struct {
	kind        ast.TupleKind
	literal     *ast.TupleLiteral
	destructure *ast.TupleDestructure
}

// transformTupleTypeAliases transforms tuple type aliases before Go parsing.
// This is a PRE-PASS that must run before the Go parser because the syntax
// `type Point = (int, int)` is not valid Go and would cause parse errors.
//
// Pattern: type Name = (Type1, Type2, ...) → type Name = __tupleType{N}__(Type1, Type2, ...)
//
// NOTE: This uses tokenizer-based scanning which is normally discouraged per CLAUDE.md,
// but is necessary here because type declarations cannot be parsed by our statement parser.
func transformTupleTypeAliases(src []byte) ([]byte, []ast.SourceMapping, error) {
	tok := tokenizer.New(src)
	result := src
	var mappings []ast.SourceMapping

	// Find all type alias locations (scan backwards to avoid offset drift)
	type typeAliasLoc struct {
		tupleStart int      // Position of '('
		tupleEnd   int      // Position after ')'
		types      []string // Element type strings
	}
	var locs []typeAliasLoc

	for {
		current := tok.NextToken()
		if current.Kind == tokenizer.EOF {
			break
		}

		// Look for: type IDENT = (
		if current.Kind == tokenizer.TYPE {
			// Save position to restore if not a tuple type
			savedPos := tok.SavePos()

			ident := tok.NextToken()
			if ident.Kind != tokenizer.IDENT {
				tok.RestorePos(savedPos)
				continue
			}

			assign := tok.NextToken()
			if assign.Kind != tokenizer.ASSIGN {
				tok.RestorePos(savedPos)
				continue
			}

			lparen := tok.NextToken()
			if lparen.Kind != tokenizer.LPAREN {
				tok.RestorePos(savedPos)
				continue
			}

			// Found "type X = (" - now parse the tuple type elements
			// NOTE: token.Pos is 1-based, so subtract 1 for 0-based array indexing
			tupleStart := int(lparen.Pos) - 1
			var types []string
			depth := 1
			typeStart := int(lparen.End) - 1
			hasComma := false
			var tupleEnd int

			for depth > 0 {
				t := tok.NextToken()
				if t.Kind == tokenizer.EOF {
					break
				}

				switch t.Kind {
				case tokenizer.LPAREN:
					depth++
				case tokenizer.RPAREN:
					depth--
					if depth == 0 {
						tupleEnd = int(t.End) - 1
						if hasComma {
							// Collect final type (1-based to 0-based conversion)
							typeStr := string(src[typeStart : int(t.Pos)-1])
							types = append(types, trimTypeWhitespace(typeStr))
						}
					}
				case tokenizer.COMMA:
					if depth == 1 {
						// Collect type before comma (1-based to 0-based conversion)
						typeStr := string(src[typeStart : int(t.Pos)-1])
						types = append(types, trimTypeWhitespace(typeStr))
						typeStart = int(t.End) - 1
						hasComma = true
					}
				}
			}

			// Only add if it's actually a tuple (has comma)
			if hasComma && len(types) >= 2 {
				locs = append(locs, typeAliasLoc{
					tupleStart: tupleStart,
					tupleEnd:   tupleEnd,
					types:      types,
				})
			}
		}
	}

	// Transform from end to beginning (reverse order)
	for i := len(locs) - 1; i >= 0; i-- {
		loc := locs[i]
		// Generate Go generic type: tuples.Tuple{N}[Type1, Type2, ...]
		// This directly produces valid Go code without needing Pass 2 resolution
		marker := fmt.Sprintf("tuples.Tuple%d[%s]", len(loc.types), joinTypes(loc.types))

		// Replace the tuple syntax with the marker
		newResult := make([]byte, 0, len(result)-(loc.tupleEnd-loc.tupleStart)+len(marker))
		newResult = append(newResult, result[:loc.tupleStart]...)
		newResult = append(newResult, []byte(marker)...)
		newResult = append(newResult, result[loc.tupleEnd:]...)
		result = newResult

		mappings = append(mappings, ast.SourceMapping{
			DingoStart: loc.tupleStart,
			DingoEnd:   loc.tupleEnd,
			GoStart:    loc.tupleStart,
			GoEnd:      loc.tupleStart + len(marker),
			Kind:       "tuple_type_alias",
		})
	}

	return result, mappings, nil
}

// trimTypeWhitespace removes leading/trailing whitespace from type string
func trimTypeWhitespace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// joinTypes joins type strings with ", "
func joinTypes(types []string) string {
	result := ""
	for i, t := range types {
		if i > 0 {
			result += ", "
		}
		result += t
	}
	return result
}

// tupleLiteralLoc holds location and elements of a tuple literal
type tupleLiteralLoc struct {
	start    int      // start position of '('
	end      int      // end position after ')'
	elements []string // element expressions as strings
}

// transformTupleLiterals transforms tuple literal expressions before Go parsing.
// This is a PRE-PASS that must run before the Go parser because the syntax
// `(expr1, expr2)` in expression contexts is ambiguous.
//
// Pattern: (a, b) → tuples.Tuple2[<types>]{First: a, Second: b}
// Since we don't have types yet, we use: (a, b) → __tuple2__(a, b)
// which will be resolved in Pass 2 with type information.
//
// Key distinction from type aliases:
// - Type aliases: `type X = (A, B)` - types after `=`
// - Literals: `return (a, b)` - expressions in various contexts
func transformTupleLiterals(src []byte) ([]byte, []ast.SourceMapping, error) {
	tok := tokenizer.New(src)
	result := src
	var mappings []ast.SourceMapping
	var locs []tupleLiteralLoc

	// Track context to distinguish tuple literals from other parens
	// We need to find `(expr, expr)` but NOT:
	// - Function calls: `foo(a, b)` (preceded by IDENT)
	// - Type assertions: `x.(Type)` (preceded by `.`)
	// - Grouped expressions: `(a + b)` (no comma at depth 1)
	// - Slice index: `a[i:j]` (inside brackets)
	// - Let destructure patterns: `let ((a, b), c) = ...` (all parens after let until =)

	var prevToken tokenizer.Token
	var prevPrevToken tokenizer.Token
	inLetDestructure := false // Track if we're inside a let destructure pattern
	letDestructureDepth := 0  // Track paren depth within let destructure

	for {
		t := tok.NextToken()
		if t.Kind == tokenizer.EOF {
			break
		}

		// Track let destructure context
		if t.Kind == tokenizer.LET {
			inLetDestructure = true
			letDestructureDepth = 0
			prevPrevToken = prevToken
			prevToken = t
			continue
		}

		// Exit let destructure context when we see = (the assignment)
		if inLetDestructure && t.Kind == tokenizer.ASSIGN {
			inLetDestructure = false
			letDestructureDepth = 0
			prevPrevToken = prevToken
			prevToken = t
			continue
		}

		// Track paren depth within let destructure
		if inLetDestructure {
			if t.Kind == tokenizer.LPAREN {
				letDestructureDepth++
				prevPrevToken = prevToken
				prevToken = t
				continue // Skip all parens within let destructure
			}
			if t.Kind == tokenizer.RPAREN {
				letDestructureDepth--
				prevPrevToken = prevToken
				prevToken = t
				continue
			}
			// Continue to next token if we're in destructure pattern
			prevPrevToken = prevToken
			prevToken = t
			continue
		}

		// Look for LPAREN that could start a tuple literal
		if t.Kind == tokenizer.LPAREN {
			// Skip if this is a function call (preceded by IDENT or RPAREN)
			// Also skip if preceded by FUNC keyword (function literal parameters)
			// Also skip if preceded by RBRACKET (generic function parameters: func F[T any](...))
			if prevToken.Kind == tokenizer.IDENT || prevToken.Kind == tokenizer.RPAREN ||
				prevToken.Kind == tokenizer.FUNC || prevToken.Kind == tokenizer.RBRACKET {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Skip if this is a type assertion (preceded by .)
			if prevToken.Kind == tokenizer.DOT {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Skip if this is a type alias context (handled by transformTupleTypeAliases)
			// Pattern: type X = ( → already handled
			if prevToken.Kind == tokenizer.ASSIGN && prevPrevToken.Kind == tokenizer.IDENT {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Potential tuple literal - scan to find if it has a comma at depth 1
			startPos := int(t.Pos) - 1 // 1-based to 0-based
			depth := 1
			hasCommaAtDepth1 := false
			var elements []string
			elemStart := int(t.End) - 1
			var rparenEnd int // Track the end position of the closing paren

			for depth > 0 {
				inner := tok.NextToken()
				if inner.Kind == tokenizer.EOF {
					break
				}

				switch inner.Kind {
				case tokenizer.LPAREN:
					depth++
				case tokenizer.RPAREN:
					depth--
					if depth == 0 {
						rparenEnd = int(inner.End) - 1
						// Collect final element
						if hasCommaAtDepth1 {
							elemStr := string(src[elemStart : int(inner.Pos)-1])
							elements = append(elements, trimTypeWhitespace(elemStr))
						}
					}
				case tokenizer.COMMA:
					if depth == 1 {
						// Collect element before comma
						elemStr := string(src[elemStart : int(inner.Pos)-1])
						elements = append(elements, trimTypeWhitespace(elemStr))
						elemStart = int(inner.End) - 1
						hasCommaAtDepth1 = true
					}
				}
			}

			// After collecting elements, check if this is actually a lambda parameter list
			// by looking ahead for => (TypeScript-style lambda)
			if hasCommaAtDepth1 && len(elements) >= 2 {
				// Peek at the next token to see if it's => (lambda indicator)
				nextTok := tok.NextToken()
				if nextTok.Kind == tokenizer.ARROW {
					// This is a lambda parameter list (acc, u) => ..., not a tuple
					// Don't add to locs, continue processing
					prevPrevToken = prevToken
					prevToken = nextTok
					continue
				}
				// Not a lambda, it's a tuple literal - add to locs
				locs = append(locs, tupleLiteralLoc{
					start:    startPos,
					end:      rparenEnd,
					elements: elements,
				})
				// Restore position since we consumed nextTok
				// Note: We can't easily restore, so we update prevToken
				prevPrevToken = prevToken
				prevToken = nextTok
				continue
			}
		}

		prevPrevToken = prevToken
		prevToken = t
	}

	// Transform from end to beginning (reverse order to preserve positions)
	for i := len(locs) - 1; i >= 0; i-- {
		loc := locs[i]
		if len(loc.elements) < 2 {
			continue // Not a valid tuple
		}

		// Generate marker: __tuple{N}__(elem1, elem2, ...)
		// This will be resolved in Pass 2 with type information
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("__tuple%d__(", len(loc.elements)))
		for j, elem := range loc.elements {
			if j > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(elem)
		}
		buf.WriteString(")")
		marker := buf.Bytes()

		// Replace the tuple literal with the marker
		newResult := make([]byte, 0, len(result)-(loc.end-loc.start)+len(marker))
		newResult = append(newResult, result[:loc.start]...)
		newResult = append(newResult, marker...)
		newResult = append(newResult, result[loc.end:]...)
		result = newResult

		mappings = append(mappings, ast.SourceMapping{
			DingoStart: loc.start,
			DingoEnd:   loc.end,
			GoStart:    loc.start,
			GoEnd:      loc.start + len(marker),
			Kind:       "tuple_literal",
		})
	}

	return result, mappings, nil
}

// transformTuplePass1 transforms all tuple syntax using parser-produced AST nodes.
// This is Pass 1 of the two-pass tuple pipeline:
//   - Literals: (a, b) → __tuple2__(a, b)
//   - Destructuring: let (x, y) = point → let __tupleDest2__("x", "y", point)
//
// Note: Type aliases are handled by transformTupleTypeAliases() which runs BEFORE this.
//
// The markers are valid Go code that will be type-checked by go/types.
// Pass 2 will then resolve the markers to actual struct types.
//
// ARCHITECTURE: This function now uses the parser to produce AST nodes instead of
// scanning raw bytes. This follows the mandated pipeline: tokenizer → parser → AST → codegen.
func transformTuplePass1(src []byte) ([]byte, []ast.SourceMapping, error) {
	// Parse the source to extract tuple AST nodes
	fset := token.NewFileSet()
	tok := tokenizer.New(src)
	stmtParser := parser.NewStmtParser(tok, fset)

	// Parse all statements to collect tuples
	// Note: This is a quick parse just to collect tuple nodes for transformation
	// The actual Go parsing happens later in the pipeline
	for {
		// Check for EOF before parsing
		if stmtParser.IsAtEnd() {
			break
		}
		stmt, err := stmtParser.ParseStatement()
		if err != nil {
			// Currently, ParseStatement wraps errors in recovery, so we treat any error
			// as a signal to stop parsing. In a future enhancement, we could distinguish
			// between fatal errors and "no statement" via error sentinels.
			// For now, this is conservative and allows the pipeline to continue.
			break
		}
		if stmt == nil {
			// Not an error - parser returns nil for most Go statements (package, import, func, etc.)
			// Continue to next statement instead of breaking
			continue
		}
	}

	// Collect all parsed tuple nodes
	var tupleNodes []tupleNodeWithPos

	// Add tuple destructures from parser
	for _, td := range stmtParser.TupleDestructures {
		tupleNodes = append(tupleNodes, tupleNodeWithPos{
			kind:       ast.TupleKindDestructure,
			destructure: td,
		})
	}

	// Add tuple literals from parser
	for _, tl := range stmtParser.TupleLiterals {
		tupleNodes = append(tupleNodes, tupleNodeWithPos{
			kind:    ast.TupleKindLiteral,
			literal: tl,
		})
	}

	// If no tuples found, return source unchanged
	if len(tupleNodes) == 0 {
		return src, nil, nil
	}

	// Sort tuple nodes by position descending (transform from end to avoid offset shifts)
	sort.Slice(tupleNodes, func(i, j int) bool {
		return tupleNodes[i].Pos() > tupleNodes[j].Pos()
	})

	result := src
	var mappings []ast.SourceMapping

	// Transform each tuple from end to beginning
	for _, node := range tupleNodes {
		var genResult ast.CodeGenResult
		var replaceStart, replaceEnd int

		// IMPORTANT: Create fresh generator for each node to avoid buffer accumulation.
		// Each codegen has its own buffer, so reusing would concatenate all outputs.
		gen := codegen.NewTupleCodeGen()

		switch node.kind {
		case ast.TupleKindLiteral:
			// Generate code for tuple literal using AST node
			genResult = gen.GenerateLiteral(node.literal)
			// NOTE: token.Pos is 1-based, subtract 1 for 0-based array indexing
			replaceStart = int(node.literal.Lparen) - 1
			replaceEnd = int(node.literal.Rparen) // +1-1=0 (includes closing paren)

		case ast.TupleKindDestructure:
			// Generate code for tuple destructuring using AST node
			genResult = gen.GenerateDestructure(node.destructure)
			// NOTE: token.Pos is 1-based, subtract 1 for 0-based array indexing
			replaceStart = int(node.destructure.LetPos) - 1
			// Guard against nil Value (malformed input like "let (x, y) =")
			if node.destructure.Value == nil {
				continue // Skip malformed destructure
			}
			replaceEnd = int(node.destructure.Value.End()) - 1

			// Make it a valid Go statement: _ = marker
			// The destructure codegen produces the marker, we just need to prefix it
			genResult.Output = append([]byte("_ = "), genResult.Output...)
		}

		if len(genResult.Output) == 0 {
			continue
		}

		// TECHNICAL DEBT: This byte splicing approach works but violates the pure AST
		// pipeline philosophy documented in CLAUDE.md. In a future refactor, we should
		// build the complete AST first, then generate all code in one pass.
		// For now, this incremental approach is pragmatic and maintains backward compatibility.
		// See: https://github.com/MadAppGang/dingo/issues/XXX (TODO: create issue)
		marker := genResult.Output
		newResult := make([]byte, 0, len(result)-(replaceEnd-replaceStart)+len(marker))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, marker...)
		newResult = append(newResult, result[replaceEnd:]...)
		result = newResult

		// Add source mapping
		mappings = append(mappings, ast.SourceMapping{
			DingoStart: replaceStart,
			DingoEnd:   replaceEnd,
			GoStart:    replaceStart,
			GoEnd:      replaceStart + len(marker),
			Kind:       "tuple_" + node.kind.String(),
		})
	}

	return result, mappings, nil
}

// Pos returns the starting position of a tuple node
func (n tupleNodeWithPos) Pos() gotoken.Pos {
	switch n.kind {
	case ast.TupleKindLiteral:
		return n.literal.Lparen
	case ast.TupleKindDestructure:
		return n.destructure.LetPos
	default:
		return gotoken.NoPos
	}
}


// transformTuplePass2 resolves tuple markers to final struct types using go/types.
// This is Pass 2 of the two-pass tuple pipeline.
//
// Markers from Pass 1:
//   - __tuple2__(a, b) → Tuple2IntString{_0: a, _1: b}
//   - __tupleDest2__("x", "y", point) → tmp := point; x := tmp._0; y := tmp._1
//   - __tupleType2__(int, string) → Tuple2IntString
//
// Returns the transformed source with markers replaced by actual Go code.
func transformTuplePass2(fset *gotoken.FileSet, file *goast.File, checker *typechecker.Checker, src []byte) ([]byte, error) {
	// Quick check: do we have any tuple markers?
	// If not, skip the transformation entirely to avoid overhead
	if !bytes.Contains(src, []byte("__tuple")) {
		return src, nil
	}

	// Create a type resolver from the marker-infused source
	resolver, err := codegen.NewTupleTypeResolver(src)
	if err != nil {
		return nil, fmt.Errorf("create tuple resolver: %w", err)
	}

	// Resolve markers to final Go code
	result, err := resolver.Resolve(src)
	if err != nil {
		return nil, fmt.Errorf("resolve tuple markers: %w", err)
	}

	return result.Output, nil
}

