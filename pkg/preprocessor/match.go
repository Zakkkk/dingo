package preprocessor

import (
	"fmt"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/matchparser"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// MatchProcessor handles AST-based pattern matching transformation
// This is the new AST-based implementation that replaces RustMatchASTProcessor
//
// Architecture:
//   1. FindMatchRegions: Locate all match expressions in source
//   2. Tokenize: Convert each match region to tokens (tokenizer package)
//   3. Parse: Build AST from tokens (parser package)
//   4. Generate: Transform AST to Go code (generator package)
//   5. Replace: Substitute match expressions with generated code
//
// P0 Bugs Fixed:
//   - Comments in match arms (tokenizer handles comments correctly)
//   - Nested patterns like Ok(Some(x)) (parser supports recursive patterns)
type MatchProcessor struct {
	matchIDCounter int
	fset           *token.FileSet
}

// NewMatchProcessor creates a new match processor
func NewMatchProcessor() *MatchProcessor {
	return &MatchProcessor{
		matchIDCounter: 0,
		fset:           token.NewFileSet(),
	}
}

// Name returns the processor name
func (p *MatchProcessor) Name() string {
	return "match_ast"
}

// ProcessBody implements BodyProcessor interface for lambda body injection
// This allows lambdas to use match expressions in their bodies
func (p *MatchProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := p.Process(body)
	return result, err
}

// Process transforms all match expressions in source
func (p *MatchProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Find all match expression regions
	regions, err := p.findMatchRegions(source)
	if err != nil {
		return nil, nil, err
	}

	if len(regions) == 0 {
		return source, nil, nil
	}

	// Process in reverse order to preserve positions
	result := source
	var allMappings []Mapping

	for i := len(regions) - 1; i >= 0; i-- {
		region := regions[i]

		// Extract match region source
		matchSource := result[region.start:region.end]

		// Tokenize the match region
		tok := tokenizer.New(matchSource)
		_, err := tok.Tokenize()
		if err != nil {
			return nil, nil, fmt.Errorf("tokenize error at offset %d (line %d): %w", region.start, region.line, err)
		}

		// Parse to AST
		matchParser := matchparser.NewMatchParser(tok)
		matchExpr, err := matchParser.ParseMatchExpr()
		if err != nil {
			return nil, nil, fmt.Errorf("parse error at offset %d (line %d): %w", region.start, region.line, err)
		}

		// Detect if match is used as expression (return, assignment, function arg)
		matchExpr.IsExpr = p.isExpressionContext(result, region)

		// Detect if match is in "return match" context (no IIFE needed)
		isReturnContext := p.isReturnContext(result, region)

		// Calculate replacement start position and initial indentation
		// The approach differs based on context:
		// - Return context: Replace "return match" and leading whitespace, generator handles indent
		// - IIFE context: Replace only "match {...}", IIFE starts inline after "="
		replaceStart := region.start
		initialIndent := p.calculateIndentLevel(result, region.start)

		if isReturnContext {
			// For "return match" context:
			// 1. Find and replace the "return " keyword
			// 2. Consume leading whitespace (generator will re-add proper indent)
			pos := region.start - 1
			for pos >= 0 && (result[pos] == ' ' || result[pos] == '\t') {
				pos--
			}
			// Check for "return" keyword (6 chars)
			if pos >= 5 {
				keyword := string(result[pos-5 : pos+1])
				if keyword == "return" {
					replaceStart = pos - 5
				}
			}

			// Include leading whitespace in replacement to prevent double-indentation
			// (generated code already includes proper indentation)
			for replaceStart > 0 && (result[replaceStart-1] == ' ' || result[replaceStart-1] == '\t') {
				replaceStart--
			}
		}
		// For IIFE context (assignment), replaceStart stays at region.start
		// Generator will handle inline first line (no indent on func() but proper indent inside)

		// Generate Go code
		gen := generator.NewMatchGenerator(p.matchIDCounter)
		gen.SetReturnContext(isReturnContext)
		gen.SetInitialIndent(initialIndent)
		goCode, genMappings := gen.Generate(matchExpr)
		p.matchIDCounter++

		// Convert generator.Mapping to preprocessor.Mapping
		for _, gm := range genMappings {
			mapping := Mapping{
				OriginalLine:       gm.OriginalLine + region.line - 1,
				OriginalColumn:     gm.OriginalColumn + region.column,
				GeneratedLine:      gm.GeneratedLine,
				GeneratedColumn:    gm.GeneratedColumn,
				ProcessorInputLine: region.line,
			}
			allMappings = append(allMappings, mapping)
		}

		// Replace in result (replaceStart was calculated earlier)
		result = append(result[:replaceStart], append([]byte(goCode), result[region.end:]...)...)
	}

	return result, allMappings, nil
}

type matchRegion struct {
	start  int
	end    int
	line   int
	column int
}

// findMatchRegions locates all match expressions in source
// Uses a simple tokenizer pass to find 'match' keywords and their closing braces
func (p *MatchProcessor) findMatchRegions(source []byte) ([]matchRegion, error) {
	// Quick tokenize to find 'match' keywords
	tok := tokenizer.New(source)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil, err
	}

	var regions []matchRegion

	for i, t := range tokens {
		if t.Kind != tokenizer.MATCH {
			continue
		}

		// Find matching closing brace
		endPos, err := p.findMatchEnd(tokens, i, source)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", t.Line, err)
		}

		// Safe position conversion with validation
		start := int(t.Pos)
		if start > 0 {
			start-- // Convert 1-based to 0-based
		} else {
			return nil, fmt.Errorf("invalid token position at line %d", t.Line)
		}

		regions = append(regions, matchRegion{
			start:  start,
			end:    endPos,
			line:   t.Line,
			column: t.Column,
		})
	}

	return regions, nil
}

// findMatchEnd finds the closing brace of a match expression
// Returns the byte offset of the position after the closing brace
func (p *MatchProcessor) findMatchEnd(tokens []tokenizer.Token, matchIdx int, source []byte) (int, error) {
	depth := 0
	foundOpen := false

	for i := matchIdx; i < len(tokens); i++ {
		tok := tokens[i]

		if tok.Kind == tokenizer.LBRACE {
			if !foundOpen {
				foundOpen = true
			}
			depth++
		} else if tok.Kind == tokenizer.RBRACE {
			depth--
			if depth == 0 && foundOpen {
				// Return position after closing brace
				end := int(tok.End)
				if end > 0 {
					return end - 1, nil
				}
				return 0, fmt.Errorf("invalid token end position")
			}
		}
	}

	return 0, fmt.Errorf("unterminated match expression")
}

// isExpressionContext determines if match is used as expression
// Returns true if match appears after: return, :=, =, (, ,
func (p *MatchProcessor) isExpressionContext(source []byte, region matchRegion) bool {
	// Look backwards from match start to find context
	// Skip whitespace and newlines
	pos := region.start - 1
	for pos >= 0 && (source[pos] == ' ' || source[pos] == '\t' || source[pos] == '\n' || source[pos] == '\r') {
		pos--
	}

	if pos < 0 {
		return false
	}

	// Check for expression context markers
	// 1. return match { ... }
	if pos >= 5 {
		keyword := string(source[pos-5 : pos+1])
		if keyword == "return" {
			return true
		}
	}

	// 2. x := match { ... }  or  x = match { ... }
	if source[pos] == '=' || source[pos] == ':' {
		return true
	}

	// 3. func(match { ... })  or  foo(match { ... })
	if source[pos] == '(' || source[pos] == ',' {
		return true
	}

	return false
}

// isReturnContext determines if match is directly after 'return' keyword
// In this context, we don't need IIFE wrapping - just switch with returns
func (p *MatchProcessor) isReturnContext(source []byte, region matchRegion) bool {
	// Look backwards from match start to find context
	// Skip whitespace and newlines
	pos := region.start - 1
	for pos >= 0 && (source[pos] == ' ' || source[pos] == '\t' || source[pos] == '\n' || source[pos] == '\r') {
		pos--
	}

	if pos < 5 {
		return false
	}

	// Check for "return" keyword
	keyword := string(source[pos-5 : pos+1])
	return keyword == "return"
}

// calculateIndentLevel calculates the indentation level at a given position
// by counting tabs from the start of the line
func (p *MatchProcessor) calculateIndentLevel(source []byte, pos int) int {
	// Find start of line
	lineStart := pos
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}

	// Count leading tabs
	tabCount := 0
	for i := lineStart; i < pos && i < len(source); i++ {
		if source[i] == '\t' {
			tabCount++
		} else if source[i] != ' ' {
			break
		}
	}

	return tabCount
}

// Compile-time interface check
var _ FeatureProcessor = (*MatchProcessor)(nil)
var _ BodyProcessor = (*MatchProcessor)(nil)
