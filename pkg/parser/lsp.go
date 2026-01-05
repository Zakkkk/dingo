// Package parser provides LSP-specific document state management
package parser

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"sync"
)

// Position represents a line/character position in a document (LSP protocol)
type Position struct {
	Line      int // 0-based line number
	Character int // 0-based character offset in line
}

// Range represents a text range in a document (LSP protocol)
type Range struct {
	Start Position
	End   Position
}

// Diagnostic represents a parse error or warning (LSP protocol)
type Diagnostic struct {
	Range    Range
	Severity DiagnosticSeverity
	Message  string
	Source   string // "dingo-parser"
}

// DiagnosticSeverity levels (LSP protocol)
type DiagnosticSeverity int

const (
	SeverityError DiagnosticSeverity = iota + 1
	SeverityWarning
	SeverityInformation
	SeverityHint
)

// TextDocumentContentChangeEvent represents a text change (LSP protocol)
type TextDocumentContentChangeEvent struct {
	Range *Range // nil for full document sync
	Text  string
}

// CompletionContext provides context for code completion
type CompletionContext struct {
	Pos            Position
	TriggerKind    CompletionTriggerKind
	CurrentToken   string         // Token being typed
	PrecedingToken string         // Token before cursor
	InExpression   bool           // True if cursor is in an expression
	ExpressionKind ExpressionKind // Type of expression (if InExpression)
	Scope          *goast.Scope   // Current scope
}

// CompletionTriggerKind specifies what triggered completion
type CompletionTriggerKind int

const (
	CompletionInvoked CompletionTriggerKind = iota + 1
	CompletionTriggerCharacter
	CompletionTriggerForIncompleteCompletions
)

// ExpressionKind categorizes expression context for completions
type ExpressionKind int

const (
	ExprNone ExpressionKind = iota
	ExprIdentifier
	ExprFieldAccess
	ExprMethodCall
	ExprErrorPropagation
	ExprSafeNavigation
	ExprNullCoalescing
)

// HoverInfo provides information for hover tooltips
type HoverInfo struct {
	Pos      Position
	Range    Range
	Contents string // Markdown-formatted documentation
	NodeType string // Type of AST node ("identifier", "error_prop", etc.)
}

// DocumentState maintains LSP state for a single document
type DocumentState struct {
	mu sync.RWMutex

	// Document identification
	URI     string
	Version int

	// Parse state
	parser   *IncrementalParser
	lastGood *ParseTree // Last successfully parsed tree (for error recovery)
	fset     *token.FileSet
	filename string

	// Raw content fallback (used when parser fails to initialize)
	rawContent []byte

	// Pending changes (for batching)
	pendingChanges []TextDocumentContentChangeEvent

	// Diagnostics cache
	diagnostics []Diagnostic
}

// NewDocumentState creates a new document state for the given source
func NewDocumentState(uri string, src []byte, fset *token.FileSet, filename string) (*DocumentState, error) {
	parser, err := NewIncrementalParser(src, fset, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	ds := &DocumentState{
		URI:            uri,
		Version:        1,
		parser:         parser,
		lastGood:       nil,
		fset:           fset,
		filename:       filename,
		pendingChanges: []TextDocumentContentChangeEvent{},
		diagnostics:    []Diagnostic{},
	}

	// Store initial good tree
	ds.lastGood = copyParseTree(parser.Tree())

	// Generate initial diagnostics
	ds.updateDiagnostics()

	return ds, nil
}

// ApplyChange applies a text change to the document
func (ds *DocumentState) ApplyChange(change TextDocumentContentChangeEvent) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Handle full document sync
	if change.Range == nil {
		// Full document update - recreate parser
		newParser, err := NewIncrementalParser([]byte(change.Text), ds.fset, ds.filename)
		if err != nil {
			// Parse failed - still update the content for transpilation
			// Create a minimal parser that just stores the content
			ds.rawContent = []byte(change.Text)
			ds.diagnostics = []Diagnostic{{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 0},
				},
				Severity: SeverityError,
				Message:  fmt.Sprintf("Parse error: %v", err),
				Source:   "dingo-parser",
			}}
			ds.Version++
			return nil // Don't fail the LSP operation - transpiler will catch detailed errors
		}
		ds.parser = newParser
		ds.rawContent = nil // Clear raw content when parser succeeds
		ds.Version++
		ds.updateDiagnostics()
		return nil
	}

	// Incremental update
	start, end := ds.rangeToByteOffsets(change.Range)
	err := ds.parser.ApplyEdit(start, end, []byte(change.Text))
	if err != nil {
		// Parse failed, but keep old tree for error recovery
		ds.diagnostics = []Diagnostic{{
			Range: Range{
				Start: change.Range.Start,
				End:   change.Range.End,
			},
			Severity: SeverityError,
			Message:  fmt.Sprintf("Parse error: %v", err),
			Source:   "dingo-parser",
		}}
		return nil // Don't fail the LSP operation
	}

	// Successful parse - update last good tree
	ds.lastGood = copyParseTree(ds.parser.Tree())
	ds.rawContent = nil // Clear raw content when parser succeeds
	ds.Version++
	ds.updateDiagnostics()

	return nil
}

// ApplyChanges applies multiple changes (for batching)
func (ds *DocumentState) ApplyChanges(changes []TextDocumentContentChangeEvent) error {
	for _, change := range changes {
		if err := ds.ApplyChange(change); err != nil {
			return err
		}
	}
	return nil
}

// GetDiagnostics returns current diagnostics
func (ds *DocumentState) GetDiagnostics() []Diagnostic {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// Return copy to avoid concurrent modification
	result := make([]Diagnostic, len(ds.diagnostics))
	copy(result, ds.diagnostics)
	return result
}

// GetCompletionContext returns completion context for the given position
func (ds *DocumentState) GetCompletionContext(pos Position) *CompletionContext {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// Convert position to byte offset
	offset := ds.positionToByteOffset(pos)

	// Use last good tree if current parse has errors
	tree := ds.parser.Tree()
	if len(tree.Errors) > 0 && ds.lastGood != nil {
		tree = ds.lastGood
	}

	// Find node at position
	node := ds.parser.NodeAt(offset)
	if node == nil {
		return &CompletionContext{
			Pos:            pos,
			TriggerKind:    CompletionInvoked,
			InExpression:   false,
			ExpressionKind: ExprNone,
		}
	}

	// Analyze context
	ctx := &CompletionContext{
		Pos:         pos,
		TriggerKind: CompletionInvoked,
	}

	// Determine expression kind based on node type
	switch n := node.Node.(type) {
	case *goast.Ident:
		ctx.InExpression = true
		ctx.ExpressionKind = ExprIdentifier
		ctx.CurrentToken = n.Name
	case *goast.SelectorExpr:
		ctx.InExpression = true
		ctx.ExpressionKind = ExprFieldAccess
	case *goast.CallExpr:
		ctx.InExpression = true
		ctx.ExpressionKind = ExprMethodCall
	}

	return ctx
}

// GetHoverInfo returns hover information for the given position
func (ds *DocumentState) GetHoverInfo(pos Position) *HoverInfo {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// Convert position to byte offset
	offset := ds.positionToByteOffset(pos)

	// Use last good tree if current parse has errors
	tree := ds.parser.Tree()
	if len(tree.Errors) > 0 && ds.lastGood != nil {
		tree = ds.lastGood
	}

	// Find node at position
	node := ds.parser.NodeAt(offset)
	if node == nil {
		return nil
	}

	// Generate hover info based on node type
	info := &HoverInfo{
		Pos:   pos,
		Range: ds.nodeToRange(node),
	}

	switch n := node.Node.(type) {
	case *goast.Ident:
		info.Contents = fmt.Sprintf("**Identifier**: `%s`", n.Name)
		info.NodeType = "identifier"
	case *goast.SelectorExpr:
		info.Contents = fmt.Sprintf("**Field/Method**: `%s`", n.Sel.Name)
		info.NodeType = "selector"
	case *goast.CallExpr:
		info.Contents = "**Function Call**"
		info.NodeType = "call"
	default:
		info.Contents = fmt.Sprintf("**AST Node**: `%T`", n)
		info.NodeType = "unknown"
	}

	return info
}

// GetAST returns the current AST
func (ds *DocumentState) GetAST() (*goast.File, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.parser.FullAST()
}

// GetGoodAST returns the last successfully parsed AST
func (ds *DocumentState) GetGoodAST() *goast.File {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	if ds.lastGood != nil {
		return ds.lastGood.Root
	}
	return ds.parser.Tree().Root
}

// Content returns the current document content as bytes
func (ds *DocumentState) Content() []byte {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	// Return raw content if parser failed to initialize
	if ds.rawContent != nil {
		return ds.rawContent
	}
	return ds.parser.Source()
}

// updateDiagnostics updates the diagnostics cache from parser errors
func (ds *DocumentState) updateDiagnostics() {
	errors := ds.parser.Errors()
	ds.diagnostics = make([]Diagnostic, 0, len(errors))

	for _, err := range errors {
		// ParseError already has Line and Column fields
		pos := Position{
			Line:      err.Line,
			Character: err.Column,
		}
		ds.diagnostics = append(ds.diagnostics, Diagnostic{
			Range: Range{
				Start: pos,
				End:   Position{Line: pos.Line, Character: pos.Character + 1},
			},
			Severity: SeverityError,
			Message:  err.Message,
			Source:   "dingo-parser",
		})
	}
}

// Helper functions for position conversion

func (ds *DocumentState) rangeToByteOffsets(r *Range) (start, end int) {
	start = ds.positionToByteOffset(r.Start)
	end = ds.positionToByteOffset(r.End)
	return
}

func (ds *DocumentState) positionToByteOffset(pos Position) int {
	src := ds.parser.Source()
	line := 0

	for i := 0; i < len(src); i++ {
		if line == pos.Line {
			// Count characters on this line
			charCount := 0
			for j := i; j < len(src) && src[j] != '\n'; j++ {
				if charCount == pos.Character {
					return j
				}
				charCount++
			}
			return i + pos.Character
		}
		if src[i] == '\n' {
			line++
		}
	}

	// Position beyond end of file
	return len(src)
}

func (ds *DocumentState) byteOffsetToPosition(offset int) Position {
	src := ds.parser.Source()
	line := 0
	lineStart := 0

	for i := 0; i < len(src) && i < offset; i++ {
		if src[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}

	character := offset - lineStart
	return Position{Line: line, Character: character}
}

func (ds *DocumentState) nodeToRange(node *ParseNode) Range {
	return Range{
		Start: ds.byteOffsetToPosition(node.Start),
		End:   ds.byteOffsetToPosition(node.End),
	}
}

// copyParseTree creates a deep copy of a parse tree for error recovery
func copyParseTree(tree *ParseTree) *ParseTree {
	if tree == nil {
		return nil
	}

	// Create copy with same AST root (ASTs are immutable in this context)
	copy := &ParseTree{
		Root:    tree.Root,
		Fset:    tree.Fset,
		Version: tree.Version,
		Nodes:   make([]ParseNode, len(tree.Nodes)),
		Errors:  make([]ParseError, len(tree.Errors)),
	}

	// Copy nodes
	for i := range tree.Nodes {
		copy.Nodes[i] = tree.Nodes[i]
	}

	// Copy errors
	for i := range tree.Errors {
		copy.Errors[i] = tree.Errors[i]
	}

	return copy
}
