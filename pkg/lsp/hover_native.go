package lsp

import (
	"context"
	"fmt"
	"os"

	"github.com/MadAppGang/dingo/pkg/lsp/semantic"
	"go.lsp.dev/protocol"
)

// nativeHover implements Dingo-native hover using semantic map
// This is the Phase 1 implementation: run go/types directly on generated Go code
// and build a semantic map from Dingo positions to type information.
func (s *Server) nativeHover(ctx context.Context, params protocol.HoverParams) (*protocol.Hover, error) {
	uri := string(params.TextDocument.URI)

	s.config.Logger.Debugf("[Native Hover] Request for URI=%s, Line=%d, Char=%d",
		uri, params.Position.Line, params.Position.Character)

	// 1. Get document source
	source, err := s.getDocumentSource(uri)
	if err != nil {
		s.config.Logger.Warnf("[Native Hover] Failed to get document source: %v", err)
		return nil, err
	}

	// 2. Get typed document from semantic manager
	// This may return a slightly stale document during typing (debounced rebuild)
	doc, err := s.semanticManager.Get(uri, source)
	if err != nil {
		s.config.Logger.Warnf("[Native Hover] Failed to get semantic document: %v", err)
		return nil, err
	}

	// 3. Check if build succeeded
	if doc.BuildError != nil {
		s.config.Logger.Debugf("[Native Hover] Document has build errors: %v", doc.BuildError)
		// Return nil hover instead of error - allow user to keep typing
		return nil, nil
	}

	if doc.SemanticMap == nil {
		s.config.Logger.Debugf("[Native Hover] No semantic map available")
		return nil, nil
	}

	// 4. Convert LSP position (0-indexed) to Dingo position (1-indexed)
	line := int(params.Position.Line) + 1
	col := int(params.Position.Character) + 1

	s.config.Logger.Debugf("[Native Hover] Looking up semantic entity at Dingo position: line=%d, col=%d", line, col)

	// 5. Look up semantic entity at position
	// Debug: log all entities on this line
	s.config.Logger.Debugf("[Native Hover] Entities on line %d:", line)
	for i := 0; i < doc.SemanticMap.Count(); i++ {
		e := doc.SemanticMap.EntityAt(i)
		if e != nil && e.Line == line {
			name := ""
			if e.Object != nil {
				name = e.Object.Name()
			}
			s.config.Logger.Debugf("[Native Hover]   - Col=%d-%d, Kind=%d, Name=%q",
				e.Col, e.EndCol, e.Kind, name)
		}
	}

	entity := doc.SemanticMap.FindAt(line, col)
	if entity == nil {
		// Try nearby search (useful for single-character operators like ?)
		entity = doc.SemanticMap.FindNearest(line, col, 2)
		if entity == nil {
			s.config.Logger.Debugf("[Native Hover] No semantic entity found at position")
			return nil, nil // No hover info available
		}
		s.config.Logger.Debugf("[Native Hover] Found nearest entity at line=%d, col=%d-%d (kind=%d)",
			entity.Line, entity.Col, entity.EndCol, entity.Kind)
	} else {
		s.config.Logger.Debugf("[Native Hover] Found exact entity at line=%d, col=%d-%d (kind=%d)",
			entity.Line, entity.Col, entity.EndCol, entity.Kind)
	}

	// 6. Format hover response with documentation for external symbols
	hover := semantic.FormatHoverWithDocs(entity, doc.TypesPkg, s.semanticManager.DocProvider())
	if hover != nil {
		s.config.Logger.Debugf("[Native Hover] Returning hover: Kind=%s, ValueLen=%d",
			hover.Contents.Kind, len(hover.Contents.Value))
	} else {
		s.config.Logger.Debugf("[Native Hover] FormatHover returned nil")
	}

	return hover, nil
}

// getDocumentSource retrieves the current source for a document
// This reads from the incremental document manager which tracks live edits
func (s *Server) getDocumentSource(uri string) ([]byte, error) {
	// Try to get from incremental document manager first (live content)
	doc := s.docManager.GetDocument(uri)
	if doc != nil {
		return doc.Content(), nil
	}

	// Fallback: read from disk
	// Convert URI to file path
	dingoPath := protocol.DocumentURI(uri).Filename()
	source, err := os.ReadFile(dingoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	return source, nil
}
