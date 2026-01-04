package lsp

import (
	"fmt"
	"go/token"
	"sync"

	"go.lsp.dev/protocol"

	"github.com/MadAppGang/dingo/pkg/goparser/parser"
)

// IncrementalDocumentManager manages incremental parsing state for open documents
type IncrementalDocumentManager struct {
	documents map[string]*parser.DocumentState
	mu        sync.RWMutex
	logger    Logger
	// Note: Each DocumentState has its own token.FileSet to avoid position conflicts
	// when multiple documents are open simultaneously.
}

// NewIncrementalDocumentManager creates a new document manager
func NewIncrementalDocumentManager(logger Logger) *IncrementalDocumentManager {
	return &IncrementalDocumentManager{
		documents: make(map[string]*parser.DocumentState),
		logger:    logger,
	}
}

// OpenDocument initializes incremental parsing for a newly opened document
func (m *IncrementalDocumentManager) OpenDocument(uri string, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debugf("[IncrementalDocMgr] Opening document: %s", uri)

	// Create a fresh FileSet for this document to avoid position conflicts
	// when multiple documents are open. Each document gets its own FileSet
	// so line numbers are always relative to the document itself.
	fset := token.NewFileSet()

	// Create document state with incremental parser
	docState, err := parser.NewDocumentState(uri, []byte(content), fset, uri)
	if err != nil {
		return fmt.Errorf("failed to create document state: %w", err)
	}

	m.documents[uri] = docState
	m.logger.Debugf("[IncrementalDocMgr] Document opened successfully: %s", uri)

	return nil
}

// UpdateDocument applies incremental changes to a document
func (m *IncrementalDocumentManager) UpdateDocument(uri string, changes []protocol.TextDocumentContentChangeEvent) error {
	m.mu.RLock()
	doc, exists := m.documents[uri]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("document not open: %s", uri)
	}

	m.logger.Debugf("[IncrementalDocMgr] Updating document %s with %d changes", uri, len(changes))

	// Convert protocol changes to parser changes
	parserChanges := make([]parser.TextDocumentContentChangeEvent, len(changes))
	for i, change := range changes {
		// Check if this is a full document sync
		// Full sync is indicated by zero Range values (Start and End are both 0,0)
		isFullSync := change.Range.Start.Line == 0 &&
			change.Range.Start.Character == 0 &&
			change.Range.End.Line == 0 &&
			change.Range.End.Character == 0 &&
			change.RangeLength == 0

		var pRange *parser.Range
		if !isFullSync {
			// Incremental change
			pRange = &parser.Range{
				Start: parser.Position{
					Line:      int(change.Range.Start.Line),
					Character: int(change.Range.Start.Character),
				},
				End: parser.Position{
					Line:      int(change.Range.End.Line),
					Character: int(change.Range.End.Character),
				},
			}
		}

		parserChanges[i] = parser.TextDocumentContentChangeEvent{
			Range: pRange,
			Text:  change.Text,
		}
	}

	// Apply changes incrementally
	if err := doc.ApplyChanges(parserChanges); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	m.logger.Debugf("[IncrementalDocMgr] Document updated successfully: %s", uri)

	return nil
}

// CloseDocument removes a document from incremental tracking
func (m *IncrementalDocumentManager) CloseDocument(uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debugf("[IncrementalDocMgr] Closing document: %s", uri)
	delete(m.documents, uri)
}

// GetDocument retrieves the document state for a given URI
func (m *IncrementalDocumentManager) GetDocument(uri string) *parser.DocumentState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.documents[uri]
}

// GetDiagnostics returns diagnostics for a document (converted to LSP format)
func (m *IncrementalDocumentManager) GetDiagnostics(uri string) []protocol.Diagnostic {
	m.mu.RLock()
	doc := m.documents[uri]
	m.mu.RUnlock()

	if doc == nil {
		return nil
	}

	// Get parser diagnostics
	parserDiags := doc.GetDiagnostics()

	// Convert to LSP protocol format
	result := make([]protocol.Diagnostic, len(parserDiags))
	for i, diag := range parserDiags {
		result[i] = protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(diag.Range.Start.Line),
					Character: uint32(diag.Range.Start.Character),
				},
				End: protocol.Position{
					Line:      uint32(diag.Range.End.Line),
					Character: uint32(diag.Range.End.Character),
				},
			},
			Severity: convertSeverity(diag.Severity),
			Source:   diag.Source,
			Message:  diag.Message,
		}
	}

	return result
}

// GetCompletionContext returns completion context for a position in a document
func (m *IncrementalDocumentManager) GetCompletionContext(uri string, pos protocol.Position) *parser.CompletionContext {
	m.mu.RLock()
	doc := m.documents[uri]
	m.mu.RUnlock()

	if doc == nil {
		return nil
	}

	parserPos := parser.Position{
		Line:      int(pos.Line),
		Character: int(pos.Character),
	}

	return doc.GetCompletionContext(parserPos)
}

// GetHoverInfo returns hover information for a position in a document
func (m *IncrementalDocumentManager) GetHoverInfo(uri string, pos protocol.Position) *parser.HoverInfo {
	m.mu.RLock()
	doc := m.documents[uri]
	m.mu.RUnlock()

	if doc == nil {
		return nil
	}

	parserPos := parser.Position{
		Line:      int(pos.Line),
		Character: int(pos.Character),
	}

	return doc.GetHoverInfo(parserPos)
}

// convertSeverity converts parser diagnostic severity to LSP severity
func convertSeverity(severity parser.DiagnosticSeverity) protocol.DiagnosticSeverity {
	switch severity {
	case parser.SeverityError:
		return protocol.DiagnosticSeverityError
	case parser.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case parser.SeverityInformation:
		return protocol.DiagnosticSeverityInformation
	case parser.SeverityHint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityError
	}
}
