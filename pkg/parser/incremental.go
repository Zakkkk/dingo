// Package parser provides incremental parsing support for LSP
package parser

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// ParseTree represents a syntax tree with change tracking
type ParseTree struct {
	Root    *goast.File
	Fset    *token.FileSet
	Nodes   []ParseNode  // All nodes with byte ranges for invalidation
	Version int          // Incremented on each edit
	Errors  []ParseError // Parse errors from last parse
}

// ParseNode wraps an AST node with byte range information
type ParseNode struct {
	Node  goast.Node
	Start int  // Byte offset in source
	End   int  // Byte offset in source
	Valid bool // False if invalidated by edit
}

// IncrementalParser maintains parse state for incremental reparsing
type IncrementalParser struct {
	tokenizer *tokenizer.IncrementalTokenizer
	tree      *ParseTree
	filename  string
}

// NewIncrementalParser creates an incremental parser for the given source
func NewIncrementalParser(src []byte, fset *token.FileSet, filename string) (*IncrementalParser, error) {
	// Create incremental tokenizer
	incTokenizer, err := tokenizer.NewIncremental(src, fset, filename)
	if err != nil {
		return nil, fmt.Errorf("tokenizer initialization failed: %w", err)
	}

	ip := &IncrementalParser{
		tokenizer: incTokenizer,
		filename:  filename,
		tree: &ParseTree{
			Fset:    fset,
			Version: 1,
		},
	}

	// Initial full parse
	if err := ip.parseFullTree(); err != nil {
		return nil, fmt.Errorf("initial parse failed: %w", err)
	}

	return ip, nil
}

// parseFullTree performs a full parse (used for initial load)
func (ip *IncrementalParser) parseFullTree() error {
	// Get all tokens
	tokens := ip.tokenizer.Tokens()
	if len(tokens) == 0 {
		ip.tree.Root = &goast.File{
			Name: &goast.Ident{Name: "main"},
		}
		ip.tree.Nodes = nil
		ip.tree.Errors = nil
		return nil
	}

	// TODO: Implement full file parsing
	// For now, create placeholder AST
	ip.tree.Root = &goast.File{
		Name: &goast.Ident{Name: "main"},
	}
	ip.tree.Nodes = []ParseNode{}
	ip.tree.Errors = []ParseError{}

	return nil
}

// ApplyEdit updates the parse tree for a text change
// start, end are byte offsets in the old source
// newText is the replacement text
// Returns true if reparsing succeeded
func (ip *IncrementalParser) ApplyEdit(start, end int, newText []byte) error {
	// Update tokenizer
	if err := ip.tokenizer.Retokenize(start, end, newText); err != nil {
		return fmt.Errorf("tokenization failed: %w", err)
	}

	// Calculate affected byte range (with expansion for context)
	const contextMargin = 50 // bytes of context to reparse
	affectedStart := max(0, start-contextMargin)
	affectedEnd := min(len(ip.tokenizer.Source()), start+len(newText)+contextMargin)

	// Invalidate nodes overlapping the affected range
	invalidatedCount := ip.invalidateNodesInRange(affectedStart, affectedEnd)

	// If invalidation affected many nodes or hit critical syntax,
	// fall back to full reparse
	if invalidatedCount > 100 || ip.needsFullReparse(affectedStart, affectedEnd) {
		return ip.parseFullTree()
	}

	// Incremental reparse of affected region
	if err := ip.reparseRegion(affectedStart, affectedEnd); err != nil {
		// On incremental parse failure, fall back to full reparse
		return ip.parseFullTree()
	}

	// Increment version
	ip.tree.Version++

	return nil
}

// invalidateNodesInRange marks nodes overlapping the byte range as invalid
// Returns the number of invalidated nodes
func (ip *IncrementalParser) invalidateNodesInRange(start, end int) int {
	count := 0
	for i := range ip.tree.Nodes {
		node := &ip.tree.Nodes[i]
		// Node overlaps if: node.Start < end && node.End > start
		if node.Start < end && node.End > start {
			node.Valid = false
			count++
		}
	}
	return count
}

// needsFullReparse checks if the edit affects critical syntax
// (e.g., package declaration, imports, top-level declarations)
func (ip *IncrementalParser) needsFullReparse(start, end int) bool {
	// For now, use simple heuristic: if edit is in first 500 bytes,
	// reparse fully (likely package/import declarations)
	return start < 500
}

// reparseRegion reparses nodes in the affected byte range
func (ip *IncrementalParser) reparseRegion(start, end int) error {
	// Get tokens in affected range
	tokens := ip.tokenizer.TokensInRange(start, end)
	if len(tokens) == 0 {
		// No tokens to reparse, just remove invalidated nodes
		ip.removeInvalidNodes()
		return nil
	}

	// TODO: Implement region-specific parsing
	// For now, remove invalid nodes and maintain consistency
	ip.removeInvalidNodes()

	return nil
}

// removeInvalidNodes removes invalidated nodes from the tree
func (ip *IncrementalParser) removeInvalidNodes() {
	validNodes := make([]ParseNode, 0, len(ip.tree.Nodes))
	for _, node := range ip.tree.Nodes {
		if node.Valid {
			validNodes = append(validNodes, node)
		}
	}
	ip.tree.Nodes = validNodes
}

// Tree returns the current parse tree
func (ip *IncrementalParser) Tree() *ParseTree {
	return ip.tree
}

// Errors returns parse errors from the last parse
func (ip *IncrementalParser) Errors() []ParseError {
	return ip.tree.Errors
}

// NodeAt returns the ParseNode containing the given byte offset
func (ip *IncrementalParser) NodeAt(offset int) *ParseNode {
	// Binary search for node containing offset
	left, right := 0, len(ip.tree.Nodes)
	for left < right {
		mid := (left + right) / 2
		node := &ip.tree.Nodes[mid]

		if offset < node.Start {
			right = mid
		} else if offset >= node.End {
			left = mid + 1
		} else {
			// Found containing node
			if node.Valid {
				return node
			}
			return nil
		}
	}
	return nil
}

// NodesInRange returns all valid nodes overlapping the byte range
func (ip *IncrementalParser) NodesInRange(start, end int) []ParseNode {
	result := make([]ParseNode, 0)
	for i := range ip.tree.Nodes {
		node := &ip.tree.Nodes[i]
		if node.Valid && node.Start < end && node.End > start {
			result = append(result, *node)
		}
	}
	return result
}

// FullAST returns the complete AST (forces full parse if needed)
func (ip *IncrementalParser) FullAST() (*goast.File, error) {
	// Check if we have invalid nodes that need reparsing
	hasInvalid := false
	for i := range ip.tree.Nodes {
		if !ip.tree.Nodes[i].Valid {
			hasInvalid = true
			break
		}
	}

	if hasInvalid {
		if err := ip.parseFullTree(); err != nil {
			return nil, err
		}
	}

	return ip.tree.Root, nil
}

// Version returns the current parse tree version
func (ip *IncrementalParser) Version() int {
	return ip.tree.Version
}

// Source returns the current source text
func (ip *IncrementalParser) Source() []byte {
	return ip.tokenizer.Source()
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
