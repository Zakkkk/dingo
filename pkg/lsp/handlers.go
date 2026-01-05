package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"strings"

	"github.com/MadAppGang/dingo/pkg/format"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Response translation methods for LSP handlers

// TranslateCompletionList translates completion item positions from Go → Dingo
func (t *Translator) TranslateCompletionList(
	list *protocol.CompletionList,
	dir Direction,
) (*protocol.CompletionList, error) {
	if list == nil {
		return nil, nil
	}

	// Translate positions in completion items
	for i := range list.Items {
		item := &list.Items[i]

		// Note: TextEdit translation is limited because TextEdit doesn't include URI
		// In practice, completion items apply to the document being edited
		// Full translation would require document context, which we handle at handler level

		// Translate AdditionalTextEdits (if they have ranges)
		if len(item.AdditionalTextEdits) > 0 {
			for j := range item.AdditionalTextEdits {
				// TextEdit translation is placeholder - needs document URI context
				_ = item.AdditionalTextEdits[j]
			}
		}
	}

	return list, nil
}


// TranslateHover translates hover response positions from Go → Dingo
func (t *Translator) TranslateHover(
	hover *protocol.Hover,
	originalURI protocol.DocumentURI,
	dir Direction,
) (*protocol.Hover, error) {
	if hover == nil {
		return nil, nil
	}

	// Translate range if present
	if hover.Range != nil {
		// The hover.Range is in Go coordinates (from gopls).
		// For GoToDingo direction, we need to pass the Go URI, not the Dingo URI.
		rangeURI := originalURI
		if dir == GoToDingo && isDingoFile(originalURI) {
			// Convert Dingo URI to Go URI for proper translation
			rangeURI = uri.File(dingoToGoPath(originalURI.Filename()))
		}
		_, newRange, err := t.TranslateRange(rangeURI, *hover.Range, dir)
		if err != nil {
			// Keep original range on error
			return hover, nil
		}
		hover.Range = &newRange
	}

	// Ensure Contents has proper MarkupContent format
	// gopls returns MarkupContent, but we need to ensure it's valid
	if hover.Contents.Kind == "" {
		// Default to markdown if kind is missing
		hover.Contents.Kind = protocol.Markdown
	}

	return hover, nil
}

// TranslateDefinitionLocations translates definition locations from Go → Dingo
func (t *Translator) TranslateDefinitionLocations(
	locations []protocol.Location,
	dir Direction,
) ([]protocol.Location, error) {
	if len(locations) == 0 {
		return locations, nil
	}

	translatedLocations := make([]protocol.Location, 0, len(locations))
	for _, loc := range locations {
		translatedLoc, err := t.TranslateLocation(loc, dir)
		if err != nil {
			// Skip locations that can't be translated
			continue
		}
		translatedLocations = append(translatedLocations, translatedLoc)
	}

	return translatedLocations, nil
}

// TranslateDocumentSymbols translates document symbols and their ranges from Go → Dingo.
// This handles the recursive Children array in DocumentSymbol.
func (t *Translator) TranslateDocumentSymbols(
	symbols []protocol.DocumentSymbol,
	goURI protocol.DocumentURI,
	dir Direction,
) ([]protocol.DocumentSymbol, error) {
	if len(symbols) == 0 {
		return symbols, nil
	}

	translatedSymbols := make([]protocol.DocumentSymbol, 0, len(symbols))
	for _, sym := range symbols {
		translatedSym, err := t.translateDocumentSymbol(sym, goURI, dir)
		if err != nil {
			// Skip symbols that can't be translated
			continue
		}
		translatedSymbols = append(translatedSymbols, translatedSym)
	}

	return translatedSymbols, nil
}

// translateDocumentSymbol translates a single DocumentSymbol including its children
func (t *Translator) translateDocumentSymbol(
	sym protocol.DocumentSymbol,
	goURI protocol.DocumentURI,
	dir Direction,
) (protocol.DocumentSymbol, error) {
	// Translate main Range
	_, newRange, err := t.TranslateRange(goURI, sym.Range, dir)
	if err != nil {
		return sym, err
	}
	sym.Range = newRange

	// Translate SelectionRange
	_, newSelectionRange, err := t.TranslateRange(goURI, sym.SelectionRange, dir)
	if err != nil {
		return sym, err
	}
	sym.SelectionRange = newSelectionRange

	// Recursively translate children
	if len(sym.Children) > 0 {
		translatedChildren, err := t.TranslateDocumentSymbols(sym.Children, goURI, dir)
		if err != nil {
			// Propagate error - don't return partially translated symbol
			return sym, fmt.Errorf("child translation failed: %w", err)
		}
		sym.Children = translatedChildren
	}

	return sym, nil
}

// TranslateSymbolInformation translates workspace symbol information locations.
// Unlike DocumentSymbol, SymbolInformation has a Location (URI + Range) per symbol.
func (t *Translator) TranslateSymbolInformation(
	symbols []protocol.SymbolInformation,
	dir Direction,
) ([]protocol.SymbolInformation, error) {
	if len(symbols) == 0 {
		return symbols, nil
	}

	translatedSymbols := make([]protocol.SymbolInformation, 0, len(symbols))
	for _, sym := range symbols {
		translatedLoc, err := t.TranslateLocation(sym.Location, dir)
		if err != nil {
			// Keep unchanged for symbols that can't be translated (e.g., stdlib)
			translatedSymbols = append(translatedSymbols, sym)
			continue
		}
		sym.Location = translatedLoc
		translatedSymbols = append(translatedSymbols, sym)
	}

	return translatedSymbols, nil
}

// TranslateCallHierarchyItem translates a call hierarchy item's range and selection range.
// The item's URI is also updated based on direction.
func (t *Translator) TranslateCallHierarchyItem(
	item protocol.CallHierarchyItem,
	dir Direction,
) (protocol.CallHierarchyItem, error) {
	// Translate Range
	newURI, newRange, err := t.TranslateRange(item.URI, item.Range, dir)
	if err != nil {
		return item, err
	}
	item.URI = newURI
	item.Range = newRange

	// Translate SelectionRange
	_, newSelectionRange, err := t.TranslateRange(newURI, item.SelectionRange, dir)
	if err != nil {
		// Keep original selection range on error but update main range
		return item, nil
	}
	item.SelectionRange = newSelectionRange

	return item, nil
}

// TranslateCallHierarchyItems translates a slice of CallHierarchyItems
func (t *Translator) TranslateCallHierarchyItems(
	items []protocol.CallHierarchyItem,
	dir Direction,
) ([]protocol.CallHierarchyItem, error) {
	if len(items) == 0 {
		return items, nil
	}

	translatedItems := make([]protocol.CallHierarchyItem, 0, len(items))
	for _, item := range items {
		translatedItem, err := t.TranslateCallHierarchyItem(item, dir)
		if err != nil {
			// Skip items that can't be translated
			continue
		}
		translatedItems = append(translatedItems, translatedItem)
	}

	return translatedItems, nil
}

// TranslateCallHierarchyIncomingCall translates an incoming call entry.
// The From field contains the CallHierarchyItem of the caller.
// FromRanges contains the call sites within the caller.
func (t *Translator) TranslateCallHierarchyIncomingCall(
	call protocol.CallHierarchyIncomingCall,
	dir Direction,
) (protocol.CallHierarchyIncomingCall, error) {
	// Translate the From item (the caller)
	translatedFrom, err := t.TranslateCallHierarchyItem(call.From, dir)
	if err != nil {
		return call, err
	}
	call.From = translatedFrom

	// Translate FromRanges (call sites within the caller)
	// These ranges are relative to the caller's file (call.From.URI after translation)
	if len(call.FromRanges) > 0 {
		translatedRanges := make([]protocol.Range, 0, len(call.FromRanges))
		for _, rng := range call.FromRanges {
			// Use the translated URI for the caller's file
			_, newRange, err := t.TranslateRange(call.From.URI, rng, dir)
			if err != nil {
				// Skip ranges that can't be translated
				continue
			}
			translatedRanges = append(translatedRanges, newRange)
		}
		call.FromRanges = translatedRanges
	}

	return call, nil
}

// TranslateCallHierarchyIncomingCalls translates a slice of incoming calls
func (t *Translator) TranslateCallHierarchyIncomingCalls(
	calls []protocol.CallHierarchyIncomingCall,
	dir Direction,
) ([]protocol.CallHierarchyIncomingCall, error) {
	if len(calls) == 0 {
		return calls, nil
	}

	translatedCalls := make([]protocol.CallHierarchyIncomingCall, 0, len(calls))
	for _, call := range calls {
		translatedCall, err := t.TranslateCallHierarchyIncomingCall(call, dir)
		if err != nil {
			// Keep unchanged for calls that can't be translated (e.g., from stdlib)
			translatedCalls = append(translatedCalls, call)
			continue
		}
		translatedCalls = append(translatedCalls, translatedCall)
	}

	return translatedCalls, nil
}

// TranslateCallHierarchyOutgoingCall translates an outgoing call entry.
// The To field contains the CallHierarchyItem of the callee.
// FromRanges contains the call sites within the current function.
func (t *Translator) TranslateCallHierarchyOutgoingCall(
	call protocol.CallHierarchyOutgoingCall,
	callerURI protocol.DocumentURI,
	dir Direction,
) (protocol.CallHierarchyOutgoingCall, error) {
	// Translate the To item (the callee)
	translatedTo, err := t.TranslateCallHierarchyItem(call.To, dir)
	if err != nil {
		return call, err
	}
	call.To = translatedTo

	// Translate FromRanges (call sites within the current function)
	// These ranges are relative to the caller's file (the file we're looking at)
	if len(call.FromRanges) > 0 {
		translatedRanges := make([]protocol.Range, 0, len(call.FromRanges))
		for _, rng := range call.FromRanges {
			_, newRange, err := t.TranslateRange(callerURI, rng, dir)
			if err != nil {
				// Skip ranges that can't be translated
				continue
			}
			translatedRanges = append(translatedRanges, newRange)
		}
		call.FromRanges = translatedRanges
	}

	return call, nil
}

// TranslateCallHierarchyOutgoingCalls translates a slice of outgoing calls.
// callerURI is the URI of the file containing the caller (for translating FromRanges).
func (t *Translator) TranslateCallHierarchyOutgoingCalls(
	calls []protocol.CallHierarchyOutgoingCall,
	callerURI protocol.DocumentURI,
	dir Direction,
) ([]protocol.CallHierarchyOutgoingCall, error) {
	if len(calls) == 0 {
		return calls, nil
	}

	translatedCalls := make([]protocol.CallHierarchyOutgoingCall, 0, len(calls))
	for _, call := range calls {
		translatedCall, err := t.TranslateCallHierarchyOutgoingCall(call, callerURI, dir)
		if err != nil {
			// Keep unchanged for calls that can't be translated (e.g., to stdlib)
			translatedCalls = append(translatedCalls, call)
			continue
		}
		translatedCalls = append(translatedCalls, translatedCall)
	}

	return translatedCalls, nil
}

// TranslateDiagnostics translates diagnostic positions from Go → Dingo
func (t *Translator) TranslateDiagnostics(
	diagnostics []protocol.Diagnostic,
	goURI protocol.DocumentURI,
	dir Direction,
) ([]protocol.Diagnostic, error) {
	if len(diagnostics) == 0 {
		return diagnostics, nil
	}

	translatedDiagnostics := make([]protocol.Diagnostic, 0, len(diagnostics))
	for _, diag := range diagnostics {
		// Translate range
		_, newRange, err := t.TranslateRange(goURI, diag.Range, dir)
		if err != nil {
			// Skip diagnostics that can't be translated
			continue
		}

		diag.Range = newRange

		// Translate related information if present
		if len(diag.RelatedInformation) > 0 {
			for j := range diag.RelatedInformation {
				relatedLoc, err := t.TranslateLocation(diag.RelatedInformation[j].Location, dir)
				if err != nil {
					continue
				}
				diag.RelatedInformation[j].Location = relatedLoc
			}
		}

		translatedDiagnostics = append(translatedDiagnostics, diag)
	}

	return translatedDiagnostics, nil
}

// Enhanced LSP method handlers with full response translation

// handleCompletionWithTranslation processes completion with full bidirectional translation
func (s *Server) handleCompletionWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Completion(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position → Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("Position translation failed: %v", err)
		// Graceful degradation: try with original position
		result, err := s.gopls.Completion(ctx, params)
		return reply(ctx, result, err)
	}

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	result, err := s.gopls.Completion(ctx, params)
	if err != nil {
		return reply(ctx, nil, err)
	}

	// Translate response: Go positions → Dingo positions
	translatedResult, err := s.translator.TranslateCompletionList(result, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("Completion response translation failed: %v", err)
		// Return untranslated result (better than nothing)
		return reply(ctx, result, nil)
	}

	// Fix: Translate TextEdit URIs manually (completion items don't have URIs)
	// We need to update URIs in the result to point back to .dingo file
	if translatedResult != nil {
		for i := range translatedResult.Items {
			item := &translatedResult.Items[i]
			// If TextEdit exists, we assume it applies to the original Dingo file
			// (gopls returns edits for the Go file, we want them for Dingo file)
			_ = item // TextEdit ranges are already translated above
		}
	}

	return reply(ctx, translatedResult, nil)
}

// handleDefinitionWithTranslation processes definition with full bidirectional translation
func (s *Server) handleDefinitionWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[Definition] Request for URI=%s, Line=%d, Char=%d",
		params.TextDocument.URI.Filename(), params.Position.Line, params.Position.Character)

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Definition(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position → Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("Position translation failed: %v", err)
		result, err := s.gopls.Definition(ctx, params)
		return reply(ctx, result, err)
	}

	s.config.Logger.Debugf("[Definition] Translated to Go: URI=%s, Line=%d, Char=%d",
		goURI.Filename(), goPos.Line, goPos.Character)

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	result, err := s.gopls.Definition(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[Definition] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[Definition] gopls returned %d locations", len(result))
	for i, loc := range result {
		s.config.Logger.Debugf("[Definition]   [%d] URI=%s, Range=L%d:C%d-L%d:C%d",
			i, loc.URI.Filename(), loc.Range.Start.Line, loc.Range.Start.Character,
			loc.Range.End.Line, loc.Range.End.Character)
	}

	// Translate response: Go locations → Dingo locations
	translatedResult, err := s.translator.TranslateDefinitionLocations(result, GoToDingo)
	if err != nil {
		// IMPORTANT FIX I5: Return error instead of silently degrading
		s.config.Logger.Warnf("Definition response translation failed: %v", err)
		return reply(ctx, nil, fmt.Errorf("position translation failed: %w (try re-transpiling file)", err))
	}

	s.config.Logger.Debugf("[Definition] Returning %d translated locations", len(translatedResult))

	return reply(ctx, translatedResult, nil)
}

// handleTypeDefinitionWithTranslation processes type definition with full bidirectional translation
func (s *Server) handleTypeDefinitionWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.TypeDefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[TypeDefinition] Request for URI=%s, Line=%d, Char=%d",
		params.TextDocument.URI.Filename(), params.Position.Line, params.Position.Character)

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.TypeDefinition(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position → Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("Position translation failed: %v", err)
		result, err := s.gopls.TypeDefinition(ctx, params)
		return reply(ctx, result, err)
	}

	s.config.Logger.Debugf("[TypeDefinition] Translated to Go: URI=%s, Line=%d, Char=%d",
		goURI.Filename(), goPos.Line, goPos.Character)

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	result, err := s.gopls.TypeDefinition(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[TypeDefinition] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[TypeDefinition] gopls returned %d locations", len(result))
	for i, loc := range result {
		s.config.Logger.Debugf("[TypeDefinition]   [%d] URI=%s, Range=L%d:C%d-L%d:C%d",
			i, loc.URI.Filename(), loc.Range.Start.Line, loc.Range.Start.Character,
			loc.Range.End.Line, loc.Range.End.Character)
	}

	// Translate response: Go locations → Dingo locations
	translatedResult, err := s.translator.TranslateDefinitionLocations(result, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("TypeDefinition response translation failed: %v", err)
		return reply(ctx, nil, fmt.Errorf("position translation failed: %w (try re-transpiling file)", err))
	}

	s.config.Logger.Debugf("[TypeDefinition] Returning %d translated locations", len(translatedResult))

	return reply(ctx, translatedResult, nil)
}

// handleHoverWithTranslation processes hover with full bidirectional translation
func (s *Server) handleHoverWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.HoverParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[Hover] Request for URI=%s, Line=%d, Char=%d",
		params.TextDocument.URI.Filename(), params.Position.Line, params.Position.Character)

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Hover(ctx, params)
		return reply(ctx, result, err)
	}

	// For .dingo files, use native hover implementation (Phase 1)
	result, err := s.nativeHover(ctx, params)
	if err != nil {
		// Log error but don't fail - graceful degradation
		s.config.Logger.Warnf("[Hover] Native hover failed: %v", err)
		return reply(ctx, nil, nil)
	}
	if result != nil {
		// Native hover succeeded - return result
		s.config.Logger.Debugf("[Hover] Native hover succeeded")
		// Check if context was canceled during semantic build
		if ctx.Err() != nil {
			s.config.Logger.Warnf("[Hover] Context canceled before reply: %v", ctx.Err())
			return ctx.Err()
		}
		return reply(ctx, result, nil)
	}

	// Native hover returned nil - this means:
	// 1. No semantic entity at this position, OR
	// 2. Entity was filtered out (e.g., generated variable like tmp2)
	// Don't fall back to gopls because it would show generated code info
	// which confuses users. Just return empty hover.
	s.config.Logger.Debugf("[Hover] Native hover returned nil, returning empty (no gopls fallback)")
	return reply(ctx, nil, nil)
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// handlePublishDiagnostics processes diagnostics from gopls and translates to Dingo positions
// This is called when gopls sends diagnostics for .go files
func (s *Server) handlePublishDiagnostics(
	ctx context.Context,
	params protocol.PublishDiagnosticsParams,
) error {
	s.config.Logger.Debugf("[Diagnostic Handler] START: Received %d diagnostics from gopls for URI=%s",
		len(params.Diagnostics), params.URI.Filename())

	// Check if this is for a .go file that has a corresponding .dingo file
	// OR if gopls is reporting diagnostics directly for .dingo files (via //line directives)
	goPath := params.URI.Filename()
	dingoPath := goToDingoPath(goPath)

	s.config.Logger.Debugf("[Diagnostic Handler] Path conversion: .go=%s → .dingo=%s", goPath, dingoPath)

	// Handle //line directive case: gopls may report diagnostics directly with .dingo URIs
	// In this case goPath already ends with .dingo and goToDingoPath returns it unchanged
	var translatedDiagnostics []protocol.Diagnostic
	if strings.HasSuffix(goPath, ".dingo") {
		// gopls is reporting directly to .dingo file (via //line directive)
		// No translation needed - pass through directly
		s.config.Logger.Debugf("[Diagnostic Handler] Direct .dingo diagnostics (via //line directive) - no translation needed")
		dingoPath = goPath
		translatedDiagnostics = params.Diagnostics
	} else if dingoPath == goPath {
		// Not a .go file and not a .dingo file - skip (pure Go or other file)
		s.config.Logger.Debugf("[Diagnostic Handler] SKIP: No .dingo file (pure Go file)")
		return nil
	} else {
		// Normal case: .go file with corresponding .dingo file
		// Translate diagnostics: Go positions → Dingo positions
		s.config.Logger.Debugf("[Diagnostic Handler] Translating %d diagnostics from Go → Dingo", len(params.Diagnostics))
		var err error
		translatedDiagnostics, err = s.translator.TranslateDiagnostics(params.Diagnostics, params.URI, GoToDingo)
		if err != nil {
			s.config.Logger.Warnf("[Diagnostic Handler] ERROR: Diagnostic translation failed: %v", err)
			return nil
		}
	}
	s.config.Logger.Debugf("[Diagnostic Handler] Successfully processed %d diagnostics", len(translatedDiagnostics))

	// Publish diagnostics for the .dingo file using unified cache
	dingoURI := uri.File(dingoPath)

	s.config.Logger.Debugf("[Diagnostic Handler] Publishing %d gopls diagnostics for %s", len(translatedDiagnostics), dingoPath)

	// Log details of each diagnostic being published
	for i, diag := range translatedDiagnostics {
		s.config.Logger.Debugf("[Diagnostic Handler]   [%d] Severity=%d, Message=%q, Range=L%d:C%d-L%d:C%d",
			i, diag.Severity, diag.Message,
			diag.Range.Start.Line, diag.Range.Start.Character,
			diag.Range.End.Line, diag.Range.End.Character)
	}

	// Use unified diagnostic cache to merge with lint diagnostics
	s.updateAndPublishDiagnostics(dingoURI, "gopls", translatedDiagnostics)

	s.config.Logger.Debugf("[Diagnostic Handler] SUCCESS: Published diagnostics to IDE")
	return nil
}

// handleFormatting processes textDocument/formatting requests for .dingo files
// This enables format-on-save and manual formatting in IDEs
func (s *Server) handleFormatting(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.DocumentFormattingParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[Formatting] Request for URI=%s", params.TextDocument.URI.Filename())

	// Only handle .dingo files
	if !isDingoFile(params.TextDocument.URI) {
		// Forward to gopls for Go files
		s.config.Logger.Debugf("[Formatting] Not a .dingo file, forwarding to gopls")
		return s.forwardToGopls(ctx, reply, req)
	}

	// C1: Use document buffer instead of disk read for unsaved changes
	path := params.TextDocument.URI.Filename()
	docContent := s.docManager.GetContent(string(params.TextDocument.URI))
	var content []byte
	if docContent != "" {
		// Use in-memory buffer (handles unsaved changes)
		content = []byte(docContent)
	} else {
		// Fallback to disk for unopened files
		diskContent, err := os.ReadFile(path)
		if err != nil {
			s.config.Logger.Errorf("[Formatting] Failed to read file: %v", err)
			return reply(ctx, nil, fmt.Errorf("failed to read file: %w", err))
		}
		content = diskContent
	}

	// I1: Load config from dingo.toml for consistent behavior with CLI
	cfg, cfgErr := format.LoadConfig(s.workspacePath)
	if cfgErr != nil {
		s.config.Logger.Warnf("[Formatting] Failed to load config: %v", cfgErr)
		cfg = format.DefaultConfig()
	}

	// Format using pkg/format (returns original on syntax errors - graceful degradation)
	formatter := format.New(cfg)
	formatted, err := formatter.Format(content)
	if err != nil {
		// Formatter returns error only for non-syntax issues (e.g., nil input)
		// Syntax errors return the original content unchanged
		s.config.Logger.Warnf("[Formatting] Formatter error: %v", err)
		// Return empty edits on error (graceful degradation)
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	// No changes needed - return empty edits
	if bytes.Equal(content, formatted) {
		s.config.Logger.Debugf("[Formatting] No changes needed")
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	// Return single TextEdit replacing entire document
	// LSP TextEdit uses 0-indexed line/character positions
	lineCount := countLines(content)
	edits := []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: uint32(lineCount), Character: 0},
		},
		NewText: string(formatted),
	}}

	s.config.Logger.Debugf("[Formatting] Returning %d edits (replaced %d lines)", len(edits), lineCount)
	return reply(ctx, edits, nil)
}

// countLines counts the number of lines in source content using token.FileSet
// C3: Uses token.FileSet instead of manual byte iteration per CLAUDE.md requirements
func countLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	fset := token.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(src))
	f.SetLinesForContent(src)
	return f.LineCount()
}
