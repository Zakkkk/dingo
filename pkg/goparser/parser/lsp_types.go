// Package parser provides LSP-related types for incremental parsing.
// These types enable IDE integration through the Language Server Protocol.
package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// DiagnosticSeverity represents the severity of a diagnostic
type DiagnosticSeverity int

const (
	SeverityError DiagnosticSeverity = iota
	SeverityWarning
	SeverityInformation
	SeverityHint
)

// Position represents a position in a document (0-based line and character)
type Position struct {
	Line      int
	Character int
}

// Range represents a range in a document
type Range struct {
	Start Position
	End   Position
}

// Diagnostic represents a parser diagnostic
type Diagnostic struct {
	Range    Range
	Severity DiagnosticSeverity
	Source   string
	Message  string
}

// TextDocumentContentChangeEvent represents a change to a text document
type TextDocumentContentChangeEvent struct {
	Range *Range // nil means full document sync
	Text  string
}

// CompletionTriggerKind indicates how completion was triggered
type CompletionTriggerKind int

const (
	CompletionTriggerInvoked CompletionTriggerKind = iota
	CompletionTriggerCharacter
	CompletionTriggerIncomplete
)

// ExpressionKind indicates the type of expression context for completion
type ExpressionKind int

const (
	ExprKindUnknown ExpressionKind = iota
	ExprKindIdentifier
	ExprKindFieldAccess
	ExprKindMethodCall
	ExprKindErrorProp
	ExprKindSafeNav
	ExprKindNullCoalesce
	ExprKindImport
)

// CompletionContext contains context for completion requests
type CompletionContext struct {
	// Position in document
	Pos Position
	// TriggerKind indicates why completion was triggered
	TriggerKind CompletionTriggerKind
	// InComment is true if the cursor is in a comment
	InComment bool
	// InString is true if the cursor is in a string literal
	InString bool
	// Prefix is the text before the cursor on the current line
	Prefix string
	// CurrentToken is the token being typed
	CurrentToken string
	// PrecedingToken is the token before CurrentToken
	PrecedingToken string
	// ExpressionKind indicates the type of expression context
	ExpressionKind ExpressionKind
	// Node is the AST node at the cursor position (may be nil)
	Node ast.Node
	// Scope is the Go AST scope for symbol lookup
	Scope *ast.Scope
}

// HoverInfo contains information for hover requests
type HoverInfo struct {
	// Contents is the hover content (markdown formatted)
	Contents string
	// Range is the range that the hover applies to
	Range Range
	// NodeType describes what kind of node this is
	NodeType string
}

// ParseError represents a parsing error with position information
type ParseError struct {
	Pos     token.Position
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Message)
}

// DocumentState maintains the state of an open document for incremental parsing
type DocumentState struct {
	uri      string
	content  []byte
	fset     *token.FileSet
	file     *ast.File
	filename string
	errors   []error
	// lineOffsets caches the byte offset of each line start for fast position conversion
	lineOffsets []int
}

// NewDocumentState creates a new document state
func NewDocumentState(uri string, content []byte, fset *token.FileSet, filename string) (*DocumentState, error) {
	ds := &DocumentState{
		uri:      uri,
		content:  content,
		fset:     fset,
		filename: filename,
	}

	// Build line offset cache
	ds.buildLineOffsets()

	// Parse the initial content
	if err := ds.parse(); err != nil {
		ds.errors = append(ds.errors, err)
	}

	return ds, nil
}

// buildLineOffsets builds a cache of byte offsets for each line start
func (ds *DocumentState) buildLineOffsets() {
	ds.lineOffsets = []int{0} // Line 0 starts at offset 0
	for i, b := range ds.content {
		if b == '\n' {
			ds.lineOffsets = append(ds.lineOffsets, i+1)
		}
	}
}

// parse parses the current content
func (ds *DocumentState) parse() error {
	// Use the Dingo parser to transform and parse
	file, err := ParseFile(ds.fset, ds.filename, ds.content, ParseComments)
	if err != nil {
		return err
	}
	ds.file = file
	return nil
}

// ApplyChanges applies incremental changes to the document
func (ds *DocumentState) ApplyChanges(changes []TextDocumentContentChangeEvent) error {
	for _, change := range changes {
		if change.Range == nil {
			// Full document sync
			ds.content = []byte(change.Text)
		} else {
			// Incremental change - apply to content
			ds.content = ds.applyChange(change)
		}
	}

	// Rebuild line offset cache
	ds.buildLineOffsets()

	// Re-parse after changes
	ds.errors = nil
	if err := ds.parse(); err != nil {
		ds.errors = append(ds.errors, err)
	}

	return nil
}

// applyChange applies a single incremental change to content
func (ds *DocumentState) applyChange(change TextDocumentContentChangeEvent) []byte {
	if change.Range == nil {
		return []byte(change.Text)
	}

	// Convert line/character positions to byte offsets
	startOffset := ds.positionToOffset(change.Range.Start)
	endOffset := ds.positionToOffset(change.Range.End)

	// Clamp to valid range
	if startOffset > len(ds.content) {
		startOffset = len(ds.content)
	}
	if endOffset > len(ds.content) {
		endOffset = len(ds.content)
	}
	if startOffset > endOffset {
		startOffset = endOffset
	}

	// Build new content
	result := make([]byte, 0, len(ds.content)-endOffset+startOffset+len(change.Text))
	result = append(result, ds.content[:startOffset]...)
	result = append(result, []byte(change.Text)...)
	result = append(result, ds.content[endOffset:]...)

	return result
}

// positionToOffset converts an LSP position (line, character) to a byte offset
func (ds *DocumentState) positionToOffset(pos Position) int {
	if pos.Line < 0 || pos.Line >= len(ds.lineOffsets) {
		return len(ds.content)
	}

	lineStart := ds.lineOffsets[pos.Line]
	offset := lineStart + pos.Character

	// Clamp to content length
	if offset > len(ds.content) {
		return len(ds.content)
	}

	return offset
}

// offsetToPosition converts a byte offset to an LSP position (line, character)
func (ds *DocumentState) offsetToPosition(offset int) Position {
	if offset < 0 {
		return Position{Line: 0, Character: 0}
	}
	if offset >= len(ds.content) {
		if len(ds.lineOffsets) == 0 {
			return Position{Line: 0, Character: offset}
		}
		lastLine := len(ds.lineOffsets) - 1
		return Position{
			Line:      lastLine,
			Character: offset - ds.lineOffsets[lastLine],
		}
	}

	// Binary search for the line containing this offset
	line := 0
	for i, lineOffset := range ds.lineOffsets {
		if lineOffset > offset {
			break
		}
		line = i
	}

	return Position{
		Line:      line,
		Character: offset - ds.lineOffsets[line],
	}
}

// GetDiagnostics returns diagnostics for the document
func (ds *DocumentState) GetDiagnostics() []Diagnostic {
	var result []Diagnostic

	for _, err := range ds.errors {
		diag := Diagnostic{
			Severity: SeverityError,
			Source:   "dingo",
			Message:  err.Error(),
		}

		// Try to extract position from error
		if pe, ok := err.(*ParseError); ok {
			diag.Range = Range{
				Start: Position{Line: pe.Pos.Line - 1, Character: pe.Pos.Column - 1}, // token.Position is 1-based
				End:   Position{Line: pe.Pos.Line - 1, Character: pe.Pos.Column - 1},
			}
		} else {
			// Try to parse position from error message (format: "filename:line:col: message")
			errStr := err.Error()
			if pos := parseErrorPosition(errStr); pos != nil {
				diag.Range = Range{
					Start: *pos,
					End:   *pos,
				}
				// Extract just the message part
				if idx := strings.LastIndex(errStr, ": "); idx != -1 {
					diag.Message = errStr[idx+2:]
				}
			} else {
				// Default to beginning of file
				diag.Range = Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 0},
				}
			}
		}

		result = append(result, diag)
	}

	return result
}

// parseErrorPosition tries to extract line:column from an error message
func parseErrorPosition(errStr string) *Position {
	// Format: "filename:line:col: message" or "line:col: message"
	parts := strings.SplitN(errStr, ":", 4)
	if len(parts) >= 3 {
		var lineStr, colStr string
		if len(parts) == 4 {
			// filename:line:col:message
			lineStr = parts[1]
			colStr = parts[2]
		} else {
			// line:col:message
			lineStr = parts[0]
			colStr = parts[1]
		}

		var line, col int
		if _, err := fmt.Sscanf(lineStr, "%d", &line); err == nil {
			if _, err := fmt.Sscanf(colStr, "%d", &col); err == nil {
				return &Position{Line: line - 1, Character: col - 1} // Convert to 0-based
			}
		}
	}
	return nil
}

// GetCompletionContext returns completion context for a position using AST analysis
func (ds *DocumentState) GetCompletionContext(pos Position) *CompletionContext {
	ctx := &CompletionContext{
		Pos:            pos,
		TriggerKind:    CompletionTriggerInvoked,
		ExpressionKind: ExprKindUnknown,
	}

	offset := ds.positionToOffset(pos)

	// Find AST node at position first - this is our primary source of truth
	if ds.file != nil {
		ctx.Node = ds.findNodeAtPosition(offset)
		ctx.Scope = ds.findScopeAtPosition(offset)

		// Determine context from AST node type
		ctx.ExpressionKind, ctx.InComment, ctx.InString = ds.analyzeNodeContext(ctx.Node, offset)

		// Extract tokens from AST
		ctx.CurrentToken, ctx.PrecedingToken = ds.extractTokensFromAST(ctx.Node)
	}

	// Get line prefix for display purposes only (not for parsing)
	if pos.Line < len(ds.lineOffsets) {
		lineStart := ds.lineOffsets[pos.Line]
		if offset > lineStart {
			ctx.Prefix = string(ds.content[lineStart:offset])
		}
	}

	return ctx
}

// analyzeNodeContext determines the expression kind and context from an AST node
func (ds *DocumentState) analyzeNodeContext(node ast.Node, offset int) (kind ExpressionKind, inComment, inString bool) {
	if node == nil {
		return ExprKindUnknown, false, false
	}

	// Check if we're in a comment by looking at the file's comments
	if ds.file != nil && ds.file.Comments != nil {
		for _, cg := range ds.file.Comments {
			for _, c := range cg.List {
				file := ds.fset.File(c.Pos())
				if file != nil {
					start := file.Offset(c.Pos())
					end := file.Offset(c.End())
					if offset >= start && offset <= end {
						return ExprKindUnknown, true, false
					}
				}
			}
		}
	}

	// Determine expression kind from AST node type
	switch n := node.(type) {
	case *ast.SelectorExpr:
		// Field access: x.field
		return ExprKindFieldAccess, false, false

	case *ast.CallExpr:
		// Function/method call
		if _, ok := n.Fun.(*ast.SelectorExpr); ok {
			return ExprKindMethodCall, false, false
		}
		return ExprKindMethodCall, false, false

	case *ast.Ident:
		return ExprKindIdentifier, false, false

	case *ast.BasicLit:
		// Check if it's a string literal
		if n.Kind.String() == "STRING" {
			return ExprKindUnknown, false, true
		}
		return ExprKindUnknown, false, false

	case *ast.ImportSpec:
		return ExprKindImport, false, false

	default:
		return ExprKindUnknown, false, false
	}
}

// extractTokensFromAST extracts current and preceding tokens from AST nodes
func (ds *DocumentState) extractTokensFromAST(node ast.Node) (current, preceding string) {
	if node == nil {
		return "", ""
	}

	switch n := node.(type) {
	case *ast.Ident:
		current = n.Name

	case *ast.SelectorExpr:
		current = n.Sel.Name
		// Get preceding from the base expression
		if id, ok := n.X.(*ast.Ident); ok {
			preceding = id.Name
		} else if sel, ok := n.X.(*ast.SelectorExpr); ok {
			preceding = sel.Sel.Name
		}

	case *ast.CallExpr:
		// For calls, get the function name
		if id, ok := n.Fun.(*ast.Ident); ok {
			current = id.Name
		} else if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
			current = sel.Sel.Name
			if id, ok := sel.X.(*ast.Ident); ok {
				preceding = id.Name
			}
		}

	case *ast.BasicLit:
		current = n.Value
	}

	return current, preceding
}

// findNodeAtPosition finds the AST node at a given byte offset
func (ds *DocumentState) findNodeAtPosition(offset int) ast.Node {
	if ds.file == nil {
		return nil
	}

	// Convert byte offset to token.Pos
	file := ds.fset.File(ds.file.Pos())
	if file == nil {
		return nil
	}

	// Clamp offset to file size
	if offset >= file.Size() {
		offset = file.Size() - 1
	}
	if offset < 0 {
		offset = 0
	}

	targetPos := file.Pos(offset)

	var found ast.Node
	ast.Inspect(ds.file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n.Pos() <= targetPos && targetPos <= n.End() {
			found = n
			return true // Continue to find more specific node
		}
		return true
	})

	return found
}

// findScopeAtPosition finds the scope at a given byte offset
func (ds *DocumentState) findScopeAtPosition(offset int) *ast.Scope {
	if ds.file == nil || ds.file.Scope == nil {
		return nil
	}

	// For now, return the file scope
	// A more sophisticated implementation would traverse the AST
	// to find the innermost scope containing the position
	return ds.file.Scope
}

// GetHoverInfo returns hover information for a position
func (ds *DocumentState) GetHoverInfo(pos Position) *HoverInfo {
	if ds.file == nil {
		return nil
	}

	offset := ds.positionToOffset(pos)
	node := ds.findNodeAtPosition(offset)
	if node == nil {
		return nil
	}

	info := &HoverInfo{}

	// Build hover content based on node type
	switch n := node.(type) {
	case *ast.Ident:
		info.Contents = fmt.Sprintf("**Identifier**: `%s`", n.Name)
		info.NodeType = "identifier"

	case *ast.SelectorExpr:
		info.Contents = fmt.Sprintf("**Field/Method**: `%s`", n.Sel.Name)
		info.NodeType = "selector"

	case *ast.CallExpr:
		if fn, ok := n.Fun.(*ast.Ident); ok {
			info.Contents = fmt.Sprintf("**Function call**: `%s()`", fn.Name)
		} else if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
			info.Contents = fmt.Sprintf("**Method call**: `%s()`", sel.Sel.Name)
		} else {
			info.Contents = "**Function call**"
		}
		info.NodeType = "call"

	case *ast.FuncDecl:
		info.Contents = fmt.Sprintf("**Function**: `%s`", n.Name.Name)
		info.NodeType = "function"

	case *ast.TypeSpec:
		info.Contents = fmt.Sprintf("**Type**: `%s`", n.Name.Name)
		info.NodeType = "type"

	case *ast.BasicLit:
		info.Contents = fmt.Sprintf("**Literal**: `%s` (kind: %s)", n.Value, n.Kind)
		info.NodeType = "literal"

	case *ast.Field:
		if len(n.Names) > 0 {
			info.Contents = fmt.Sprintf("**Field**: `%s`", n.Names[0].Name)
		} else {
			info.Contents = "**Embedded field**"
		}
		info.NodeType = "field"

	default:
		info.Contents = fmt.Sprintf("**%T**", n)
		info.NodeType = fmt.Sprintf("%T", n)
	}

	// Set range based on node position
	info.Range = ds.nodeToRange(node)

	return info
}

// nodeToRange converts an AST node's position to an LSP Range
func (ds *DocumentState) nodeToRange(node ast.Node) Range {
	file := ds.fset.File(node.Pos())
	if file == nil {
		return Range{}
	}

	startOffset := file.Offset(node.Pos())
	endOffset := file.Offset(node.End())

	return Range{
		Start: ds.offsetToPosition(startOffset),
		End:   ds.offsetToPosition(endOffset),
	}
}

// Content returns the current document content
func (ds *DocumentState) Content() []byte {
	return ds.content
}

// File returns the parsed AST file
func (ds *DocumentState) File() *ast.File {
	return ds.file
}

// URI returns the document URI
func (ds *DocumentState) URI() string {
	return ds.uri
}
