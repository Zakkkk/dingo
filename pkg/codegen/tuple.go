package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// TupleCodeGen generates marker-based Go code for tuple expressions (Pass 1).
//
// This is the FIRST pass of a two-pass pipeline:
// - Pass 1 (this): Transform tuple syntax to markers (no type info needed)
// - Pass 2 (tuple_types.go): Use go/types to resolve markers to final structs
//
// Marker formats:
// - Literals: __tuple{N}__(elem1, elem2, ...) where N is element count
// - Destructuring: __tupleDest{N}__("name1", "name2", ..., expr)
// - Type aliases: __tupleType{N}__(type1, type2, ...)
type TupleCodeGen struct {
	*BaseGenerator
}

// NewTupleCodeGen creates a tuple codegen for Pass 1 marker generation.
func NewTupleCodeGen() *TupleCodeGen {
	return &TupleCodeGen{
		BaseGenerator: NewBaseGenerator(),
	}
}

// GenerateFromLocation generates marker code from a TupleLocation.
//
// This method dispatches to the appropriate generator based on the tuple kind.
// It uses the tokenizer to extract elements (no string manipulation per CLAUDE.md).
//
// Returns the marker code as bytes.
func (g *TupleCodeGen) GenerateFromLocation(loc ast.TupleLocation, src []byte) ([]byte, error) {
	switch loc.Kind {
	case ast.TupleKindLiteral:
		return g.generateLiteralMarker(loc, src)
	case ast.TupleKindDestructure:
		return g.generateDestructureMarker(loc, src)
	case ast.TupleKindTypeAlias:
		return g.generateTypeAliasMarker(loc, src)
	case ast.TupleKindFuncReturn:
		return g.generateFuncReturnMarker(loc, src)
	default:
		return nil, fmt.Errorf("unknown tuple kind: %v", loc.Kind)
	}
}

// generateLiteralMarker creates marker for tuple literal.
//
// Uses tokenizer to extract elements and handle nested tuples.
//
// Example: (10, 20) → __tuple2__(10, 20)
// Example: ((1, 2), 3) → __tuple2__(__tuple2__(1, 2), 3)
func (g *TupleCodeGen) generateLiteralMarker(loc ast.TupleLocation, src []byte) ([]byte, error) {
	if loc.Start < 0 || loc.End > len(src) {
		return nil, fmt.Errorf("invalid location bounds: %d-%d", loc.Start, loc.End)
	}

	// Extract elements using tokenizer
	elements, err := extractTupleElements(src[loc.Start:loc.End])
	if err != nil {
		return nil, fmt.Errorf("extract elements: %w", err)
	}

	// Generate marker: __tuple{N}__(elem1, elem2, ...)
	markerName := fmt.Sprintf("__tuple%d__", len(elements))

	var result strings.Builder
	result.WriteString(markerName)
	result.WriteByte('(')

	for i, elem := range elements {
		if i > 0 {
			result.WriteString(", ")
		}

		// Check if element is a nested tuple
		if isNestedTuple(elem) {
			// Recursively transform nested tuple
			nestedLoc := ast.TupleLocation{
				Kind:     ast.TupleKindLiteral,
				Start:    0,
				End:      len(elem),
				Elements: countElements(elem),
			}
			nestedMarker, err := g.generateLiteralMarker(nestedLoc, []byte(elem))
			if err != nil {
				return nil, fmt.Errorf("nested tuple: %w", err)
			}
			result.Write(nestedMarker)
		} else {
			result.WriteString(elem)
		}
	}

	result.WriteByte(')')

	return []byte(result.String()), nil
}

// generateDestructureMarker creates marker for tuple destructuring.
//
// Example: let (x, y) = point → __tupleDest2__("x:0", "y:1", point)
// Example: let (x, _) = pair → __tupleDest2__("x:0", "_:1", pair)
// Example: let ((a, b), c) = nested → __tupleDest3__("a:0.0", "b:0.1", "c:1", nested)
//
// Each name is encoded as "name:path" where path is dot-separated indices.
func (g *TupleCodeGen) generateDestructureMarker(loc ast.TupleLocation, src []byte) ([]byte, error) {
	if loc.Start < 0 || loc.End > len(src) {
		return nil, fmt.Errorf("invalid location bounds: %d-%d", loc.Start, loc.End)
	}

	// Extract pattern elements with paths from tuple
	patternSrc := src[loc.Start:loc.End]
	elements, err := extractPatternElements(patternSrc)
	if err != nil {
		return nil, fmt.Errorf("extract pattern: %w", err)
	}

	// Find the RHS expression after = or :=
	rhsExpr, err := extractDestructureRHS(src, loc.End)
	if err != nil {
		return nil, fmt.Errorf("extract RHS: %w", err)
	}

	// Generate marker: __tupleDest{N}__("name1:path1", "name2:path2", ..., expr)
	markerName := fmt.Sprintf("__tupleDest%d__", len(elements))

	var result strings.Builder
	result.WriteString(markerName)
	result.WriteByte('(')

	for i, elem := range elements {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteByte('"')
		result.WriteString(elem.Name)
		result.WriteByte(':')
		// Encode path as dot-separated indices
		for j, idx := range elem.Path {
			if j > 0 {
				result.WriteByte('.')
			}
			result.WriteString(strconv.Itoa(idx))
		}
		result.WriteByte('"')
	}

	result.WriteString(", ")
	result.WriteString(rhsExpr)
	result.WriteByte(')')

	return []byte(result.String()), nil
}

// generateTypeAliasMarker creates marker for tuple type alias.
//
// Example: type Point = (int, int) → __tupleType2__(int, int)
func (g *TupleCodeGen) generateTypeAliasMarker(loc ast.TupleLocation, src []byte) ([]byte, error) {
	if loc.Start < 0 || loc.End > len(src) {
		return nil, fmt.Errorf("invalid location bounds: %d-%d", loc.Start, loc.End)
	}

	// Extract type names from tuple
	typeSrc := src[loc.Start:loc.End]
	types, err := extractTypeNames(typeSrc)
	if err != nil {
		return nil, fmt.Errorf("extract types: %w", err)
	}

	// Generate marker: __tupleType{N}__(type1, type2, ...)
	markerName := fmt.Sprintf("__tupleType%d__", len(types))

	var result strings.Builder
	result.WriteString(markerName)
	result.WriteByte('(')

	for i, typ := range types {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(typ)
	}

	result.WriteByte(')')

	return []byte(result.String()), nil
}

// generateFuncReturnMarker creates marker for function return tuple types.
//
// Example: func foo() (int, string) → struct { _0 int; _1 string }
func (g *TupleCodeGen) generateFuncReturnMarker(loc ast.TupleLocation, src []byte) ([]byte, error) {
	// For function return types, ElementsInfo should be populated by finder
	if len(loc.ElementsInfo) == 0 {
		return nil, fmt.Errorf("ElementsInfo not populated for func return tuple")
	}

	// Generate anonymous struct directly (can't use markers in type position)
	var result strings.Builder
	result.WriteString("struct { ")

	for i, elem := range loc.ElementsInfo {
		if i > 0 {
			result.WriteString("; ")
		}
		result.WriteString(fmt.Sprintf("_%d %s", i, elem.Name))
	}

	result.WriteString(" }")

	return []byte(result.String()), nil
}

// extractTupleElements extracts elements from a tuple literal using tokenizer.
// Input: (elem1, elem2, ...) - includes parens
// Output: ["elem1", "elem2", ...]
func extractTupleElements(tupleSrc []byte) ([]string, error) {
	tok := tokenizer.New(tupleSrc)
	if _, err := tok.Tokenize(); err != nil {
		return nil, fmt.Errorf("tokenize tuple: %w", err)
	}
	tok.Reset()
	tok.Advance() // skip (

	var elements []string
	var currentElem strings.Builder
	depth := 0

	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			break
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			depth++
			currentElem.WriteString(string(tupleSrc[current.BytePos():current.ByteEnd()]))

		case tokenizer.RPAREN:
			if depth == 0 {
				// End of tuple
				elem := strings.TrimSpace(currentElem.String())
				if elem != "" {
					elements = append(elements, elem)
				}
				return elements, nil
			}
			depth--
			currentElem.WriteString(string(tupleSrc[current.BytePos():current.ByteEnd()]))

		case tokenizer.COMMA:
			if depth == 0 {
				// Top-level comma - element separator
				elem := strings.TrimSpace(currentElem.String())
				if elem != "" {
					elements = append(elements, elem)
				}
				currentElem.Reset()
			} else {
				// Nested comma - part of element
				currentElem.WriteString(string(tupleSrc[current.BytePos():current.ByteEnd()]))
			}

		default:
			// Add token to current element
			currentElem.WriteString(string(tupleSrc[current.BytePos():current.ByteEnd()]))
		}

		tok.Advance()
	}

	return elements, nil
}

// PatternElement represents a destructuring pattern element with its access path.
// For nested patterns like ((a, b), c), the paths are:
// - a: [0, 0] (first element of first inner tuple)
// - b: [0, 1] (second element of first inner tuple)
// - c: [1]    (second element of outer tuple)
type PatternElement struct {
	Name string // Variable name or "_" for wildcard
	Path []int  // Access path, e.g., [0, 1] means ._0._1
}

// extractPatternNames extracts variable names from a destructuring pattern.
// Input: (x, y) or (x, _) or ((a, b), c) - includes parens
// Output: ["x", "y"] or ["x", "_"] or ["a", "b", "c"]
// For flat patterns only - use extractPatternElements for nested patterns.
func extractPatternNames(patternSrc []byte) ([]string, error) {
	elements, err := extractPatternElements(patternSrc)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(elements))
	for i, elem := range elements {
		names[i] = elem.Name
	}
	return names, nil
}

// extractPatternElements extracts pattern elements with their access paths.
// Handles nested patterns like ((minX, minY), (maxX, maxY)).
func extractPatternElements(patternSrc []byte) ([]PatternElement, error) {
	tok := tokenizer.New(patternSrc)
	if _, err := tok.Tokenize(); err != nil {
		return nil, fmt.Errorf("tokenize pattern: %w", err)
	}
	tok.Reset()
	tok.Advance() // skip (

	var elements []PatternElement
	var pathStack []int // Current position at each depth
	pathStack = append(pathStack, 0) // Start at element 0

	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			break
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			// Enter nested tuple - push new position
			pathStack = append(pathStack, 0)

		case tokenizer.RPAREN:
			if len(pathStack) == 1 {
				// Closing outer paren - done
				return elements, nil
			}
			// Exit nested tuple - pop and increment parent position
			pathStack = pathStack[:len(pathStack)-1]

		case tokenizer.COMMA:
			// Move to next element at current depth
			if len(pathStack) > 0 {
				pathStack[len(pathStack)-1]++
			}

		case tokenizer.IDENT:
			// Found a variable name - record with current path
			name := string(patternSrc[current.BytePos():current.ByteEnd()])
			path := make([]int, len(pathStack))
			copy(path, pathStack)
			elements = append(elements, PatternElement{Name: name, Path: path})

		case tokenizer.UNDERSCORE:
			// Found a wildcard - record with current path
			path := make([]int, len(pathStack))
			copy(path, pathStack)
			elements = append(elements, PatternElement{Name: "_", Path: path})
		}

		tok.Advance()
	}

	return elements, nil
}

// extractDestructureRHS extracts the RHS expression after = or :=
func extractDestructureRHS(src []byte, patternEnd int) (string, error) {
	// Tokenize from pattern end to find assignment and RHS
	remaining := src[patternEnd:]
	tok := tokenizer.New(remaining)
	if _, err := tok.Tokenize(); err != nil {
		return "", fmt.Errorf("tokenize RHS: %w", err)
	}
	tok.Reset()

	// Skip whitespace and find =
	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			return "", fmt.Errorf("no assignment found")
		}

		if current.Kind == tokenizer.ASSIGN || current.Kind == tokenizer.DEFINE {
			tok.Advance() // skip = or :=
			break
		}

		tok.Advance()
	}

	// Collect RHS until end of statement
	var rhs strings.Builder
	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF || current.Kind == tokenizer.NEWLINE || current.Kind == tokenizer.SEMICOLON {
			break
		}
		rhs.WriteString(string(remaining[current.BytePos():current.ByteEnd()]))
		tok.Advance()
	}

	return strings.TrimSpace(rhs.String()), nil
}

// extractTypeNames extracts type names from a tuple type.
// Input: (int, string) - includes parens
// Output: ["int", "string"]
func extractTypeNames(typeSrc []byte) ([]string, error) {
	tok := tokenizer.New(typeSrc)
	if _, err := tok.Tokenize(); err != nil {
		return nil, fmt.Errorf("tokenize types: %w", err)
	}
	tok.Reset()
	tok.Advance() // skip (

	var types []string
	var currentType strings.Builder
	depth := 0

	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			break
		}

		switch current.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACKET:
			depth++
			currentType.WriteString(string(typeSrc[current.BytePos():current.ByteEnd()]))

		case tokenizer.RPAREN:
			if depth == 0 {
				// End of tuple type
				typ := strings.TrimSpace(currentType.String())
				if typ != "" {
					types = append(types, typ)
				}
				return types, nil
			}
			depth--
			currentType.WriteString(string(typeSrc[current.BytePos():current.ByteEnd()]))

		case tokenizer.RBRACKET:
			depth--
			currentType.WriteString(string(typeSrc[current.BytePos():current.ByteEnd()]))

		case tokenizer.COMMA:
			if depth == 0 {
				typ := strings.TrimSpace(currentType.String())
				if typ != "" {
					types = append(types, typ)
				}
				currentType.Reset()
			} else {
				currentType.WriteString(string(typeSrc[current.BytePos():current.ByteEnd()]))
			}

		default:
			currentType.WriteString(string(typeSrc[current.BytePos():current.ByteEnd()]))
		}

		tok.Advance()
	}

	return types, nil
}

// isNestedTuple checks if an element string represents a nested tuple.
func isNestedTuple(elem string) bool {
	elem = strings.TrimSpace(elem)
	if len(elem) < 2 {
		return false
	}

	// Check if it's wrapped in parens and contains a comma at depth 0
	if elem[0] != '(' || elem[len(elem)-1] != ')' {
		return false
	}

	// Tokenize to verify it has commas at depth 0
	tok := tokenizer.New([]byte(elem))
	if _, err := tok.Tokenize(); err != nil {
		return false
	}
	tok.Reset()
	tok.Advance() // skip (
	depth := 0

	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			break
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			depth++
		case tokenizer.RPAREN:
			if depth == 0 {
				return false // Closed before finding comma
			}
			depth--
		case tokenizer.COMMA:
			if depth == 0 {
				return true // Found comma at top level
			}
		}

		tok.Advance()
	}

	return false
}

// countElements counts elements in a tuple string.
func countElements(tuple string) int {
	tok := tokenizer.New([]byte(tuple))
	if _, err := tok.Tokenize(); err != nil {
		return 0
	}
	tok.Reset()
	tok.Advance() // skip (

	count := 0
	depth := 0
	hasContent := false

	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			break
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			depth++
			hasContent = true
		case tokenizer.RPAREN:
			if depth == 0 {
				if hasContent {
					count++
				}
				return count
			}
			depth--
		case tokenizer.COMMA:
			if depth == 0 {
				count++
			}
		default:
			if current.Kind != tokenizer.NEWLINE && current.Kind != tokenizer.COMMENT {
				hasContent = true
			}
		}

		tok.Advance()
	}

	return count
}

// formatTmpVar formats temporary variable name following CLAUDE.md naming convention.
// First tmp is unnumbered, subsequent are tmp1, tmp2, etc.
func formatTmpVar(counter int) string {
	if counter == 1 {
		return "tmp"
	}
	return "tmp" + strconv.Itoa(counter-1)
}
