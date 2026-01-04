package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// handleDocumentSymbol processes textDocument/documentSymbol requests.
// Returns symbols (functions, types, variables) in the current document.
// For .dingo files, forwards to gopls after URI translation, then translates
// the result ranges back to Dingo positions.
func (s *Server) handleDocumentSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[DocumentSymbol] Request for URI=%s", params.TextDocument.URI.Filename())

	// If not a .dingo file, forward directly to gopls
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.DocumentSymbol(ctx, params)
		return reply(ctx, result, err)
	}

	// For .dingo files:
	// 1. Translate Dingo URI to Go URI
	dingoURI := params.TextDocument.URI
	goPath := dingoToGoPath(dingoURI.Filename())
	goURI := protocol.DocumentURI(uri.File(goPath))

	// Update params with Go URI
	params.TextDocument.URI = goURI

	// 2. Call gopls.DocumentSymbol()
	result, err := s.gopls.DocumentSymbol(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[DocumentSymbol] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[DocumentSymbol] gopls returned %d symbols", len(result))

	// 3. Convert result to []protocol.DocumentSymbol
	// gopls typically returns []DocumentSymbol (hierarchical), but the return type
	// is []any to handle both DocumentSymbol and SymbolInformation
	docSymbols, err := convertToDocumentSymbols(result)
	if err != nil {
		s.config.Logger.Warnf("[DocumentSymbol] Failed to convert symbols: %v", err)
		return reply(ctx, result, nil) // Return original result on conversion error
	}

	// 4. Translate ranges back to Dingo positions
	translatedSymbols, err := s.translator.TranslateDocumentSymbols(docSymbols, goURI, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("[DocumentSymbol] Translation failed: %v", err)
		// Return untranslated symbols rather than failing completely
		return reply(ctx, docSymbols, nil)
	}

	// 5. Dingo-native symbol enhancement (enum variants, match arms, lambdas)
	// Extract Dingo-specific symbols using go/scanner (CLAUDE.md compliant)
	dingoContent := s.docManager.GetContent(string(dingoURI))
	if dingoContent != "" {
		extractor := NewDingoSymbolExtractor(s.config.Logger)
		dingoSymbols, err := extractor.ExtractDingoSymbols([]byte(dingoContent), dingoURI)
		if err != nil {
			s.config.Logger.Warnf("[DocumentSymbol] Dingo symbol extraction failed: %v", err)
		} else if len(dingoSymbols) > 0 {
			s.config.Logger.Debugf("[DocumentSymbol] Extracted %d Dingo-native symbols", len(dingoSymbols))
			// Merge Dingo symbols with gopls symbols
			translatedSymbols = MergeDingoSymbols(translatedSymbols, dingoSymbols)
		}
	}

	s.config.Logger.Debugf("[DocumentSymbol] Returning %d translated symbols", len(translatedSymbols))
	return reply(ctx, translatedSymbols, nil)
}

// handleWorkspaceSymbol processes workspace/symbol requests.
// Searches for symbols across the entire workspace matching a query.
// For results in .go files that have corresponding .dingo files, the
// locations are translated back to Dingo positions.
func (s *Server) handleWorkspaceSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[WorkspaceSymbol] Query: %q", params.Query)

	// Forward query to gopls (no position translation needed for the query itself)
	result, err := s.gopls.WorkspaceSymbol(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[WorkspaceSymbol] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[WorkspaceSymbol] gopls returned %d symbols", len(result))

	// Translate SymbolInformation locations
	// For each result, if the URI ends with .go and has a corresponding .dingo file,
	// translate the location to point to the .dingo file with correct positions.
	translatedSymbols, err := s.translator.TranslateSymbolInformation(result, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("[WorkspaceSymbol] Translation failed: %v", err)
		// Return untranslated symbols rather than failing
		return reply(ctx, result, nil)
	}

	s.config.Logger.Debugf("[WorkspaceSymbol] Returning %d translated symbols", len(translatedSymbols))
	return reply(ctx, translatedSymbols, nil)
}

// convertToDocumentSymbols converts the []any result from gopls to []protocol.DocumentSymbol.
// gopls can return either []DocumentSymbol (hierarchical) or []SymbolInformation (flat).
// We prefer DocumentSymbol as it supports hierarchy (children).
func convertToDocumentSymbols(result []any) ([]protocol.DocumentSymbol, error) {
	if len(result) == 0 {
		return []protocol.DocumentSymbol{}, nil
	}

	// Re-marshal and unmarshal to properly convert the interface{} values
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	// Try to unmarshal as []DocumentSymbol first (gopls default)
	var docSymbols []protocol.DocumentSymbol
	if err := json.Unmarshal(data, &docSymbols); err != nil {
		// If that fails, try []SymbolInformation and convert
		var symInfos []protocol.SymbolInformation
		if err := json.Unmarshal(data, &symInfos); err != nil {
			return nil, err
		}

		// Convert SymbolInformation to DocumentSymbol (loses hierarchy but works)
		docSymbols = make([]protocol.DocumentSymbol, len(symInfos))
		for i, si := range symInfos {
			docSymbols[i] = protocol.DocumentSymbol{
				Name:           si.Name,
				Kind:           si.Kind,
				Tags:           si.Tags,
				Range:          si.Location.Range,
				SelectionRange: si.Location.Range,
				// Children: nil - SymbolInformation is flat
			}
		}
	}

	return docSymbols, nil
}

// isGoFileWithDingoCounterpart checks if a .go file has a corresponding .dingo source file.
// Used to determine if a symbol location should be translated to Dingo.
func isGoFileWithDingoCounterpart(uri protocol.DocumentURI) bool {
	uriStr := string(uri)
	if !strings.HasSuffix(uriStr, ".go") {
		return false
	}

	// Check if corresponding .dingo file exists
	// This is implicit in the translator - if no source map exists, translation
	// returns the original location unchanged. We could optimize by checking
	// file existence first, but the translator already handles this gracefully.
	return true // Let translator handle existence check
}
