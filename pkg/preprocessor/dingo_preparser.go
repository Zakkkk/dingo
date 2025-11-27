package preprocessor

import (
	"bytes"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lexer"
)

// DingoPreParser transforms Dingo-specific syntax to Go
// Runs BEFORE all other preprocessors
type DingoPreParser struct {
	nodes []ast.DingoNode
}

// NewDingoPreParser creates a new DingoPreParser
func NewDingoPreParser() *DingoPreParser {
	return &DingoPreParser{
		nodes: make([]ast.DingoNode, 0),
	}
}

// Name implements FeatureProcessor
func (p *DingoPreParser) Name() string {
	return "DingoPreParser"
}

// Process implements FeatureProcessor
// Scans source for let declarations and transforms them
func (p *DingoPreParser) Process(source []byte) ([]byte, []Mapping, error) {
	// Reset nodes from previous run
	p.nodes = make([]ast.DingoNode, 0)

	lines := strings.Split(string(source), "\n")
	var result bytes.Buffer

	currentLine := 1

	for lineIdx, line := range lines {
		// Check if line contains "let " at the start (after trimming spaces)
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "let ") {
			// Not a let declaration, preserve as-is
			result.WriteString(line)
			if lineIdx < len(lines)-1 {
				result.WriteString("\n")
			}
			currentLine++
			continue
		}

		// Calculate indentation
		indent := getIndentation(line)

		// Parse let declaration using lexer
		lex := lexer.New(trimmed)
		letDecl, err := parseLetDeclaration(lex)
		if err != nil {
			// If parsing fails, preserve original line
			result.WriteString(line)
			if lineIdx < len(lines)-1 {
				result.WriteString("\n")
			}
			currentLine++
			continue
		}

		// Store parsed node
		p.nodes = append(p.nodes, letDecl)

		// Generate Go code
		goCode := letDecl.ToGo()

		// Apply original indentation
		indentedCode := indent + goCode

		// Write transformed code
		result.WriteString(indentedCode)
		if lineIdx < len(lines)-1 {
			result.WriteString("\n")
		}

		// NOTE: We don't create legacy mappings here
		// DingoPreParser is a simple text transformation (let → var/short decl)
		// No metadata needed since it's 1:1 line mapping

		currentLine++
	}

	return result.Bytes(), nil, nil
}

// GetNodes returns parsed Dingo AST nodes for inspection
func (p *DingoPreParser) GetNodes() []ast.DingoNode {
	return p.nodes
}

// getIndentation extracts the leading whitespace from a line
func getIndentation(line string) string {
	var indent strings.Builder
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			indent.WriteRune(ch)
		} else {
			break
		}
	}
	return indent.String()
}
