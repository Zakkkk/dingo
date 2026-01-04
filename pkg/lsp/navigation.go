package lsp

import (
	"context"
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// handleReferences handles textDocument/references requests.
// It translates Dingo positions to Go, calls gopls, then translates results back.
// All references are returned including those in stdlib/vendor .go files.
func (s *Server) handleReferences(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.ReferenceParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[References] Request for URI=%s, Line=%d, Char=%d",
		params.TextDocument.URI.Filename(), params.Position.Line, params.Position.Character)

	// If not a .dingo file, forward directly to gopls
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.References(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position -> Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("[References] Position translation failed: %v", err)
		// Graceful degradation: try with original position
		result, err := s.gopls.References(ctx, params)
		return reply(ctx, result, err)
	}

	s.config.Logger.Debugf("[References] Translated to Go: URI=%s, Line=%d, Char=%d",
		goURI.Filename(), goPos.Line, goPos.Character)

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	locations, err := s.gopls.References(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[References] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[References] gopls returned %d locations", len(locations))

	// Translate all locations Go -> Dingo
	// Decision: Include ALL references (stdlib, vendor) unchanged for completeness
	translated := make([]protocol.Location, 0, len(locations))
	for i, loc := range locations {
		translatedLoc, err := s.translator.TranslateLocation(loc, GoToDingo)
		if err != nil {
			// Keep unchanged for locations that can't be translated (e.g., stdlib, vendor .go files)
			s.config.Logger.Debugf("[References]   [%d] Translation failed, keeping original: URI=%s, Line=%d",
				i, loc.URI.Filename(), loc.Range.Start.Line)
			translated = append(translated, loc)
		} else {
			s.config.Logger.Debugf("[References]   [%d] Translated: URI=%s, Line=%d -> URI=%s, Line=%d",
				i, loc.URI.Filename(), loc.Range.Start.Line,
				translatedLoc.URI.Filename(), translatedLoc.Range.Start.Line)
			translated = append(translated, translatedLoc)
		}
	}

	s.config.Logger.Debugf("[References] Returning %d locations", len(translated))
	return reply(ctx, translated, nil)
}

// handleImplementation handles textDocument/implementation requests.
// It finds implementations of interfaces, translating positions between Dingo and Go.
func (s *Server) handleImplementation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.ImplementationParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[Implementation] Request for URI=%s, Line=%d, Char=%d",
		params.TextDocument.URI.Filename(), params.Position.Line, params.Position.Character)

	// If not a .dingo file, forward directly to gopls
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Implementation(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position -> Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("[Implementation] Position translation failed: %v", err)
		// Graceful degradation: try with original position
		result, err := s.gopls.Implementation(ctx, params)
		return reply(ctx, result, err)
	}

	s.config.Logger.Debugf("[Implementation] Translated to Go: URI=%s, Line=%d, Char=%d",
		goURI.Filename(), goPos.Line, goPos.Character)

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	locations, err := s.gopls.Implementation(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[Implementation] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[Implementation] gopls returned %d locations", len(locations))

	// Translate all locations Go -> Dingo
	// Keep external implementations unchanged (they may be in .go files without .dingo source)
	translated := make([]protocol.Location, 0, len(locations))
	for i, loc := range locations {
		translatedLoc, err := s.translator.TranslateLocation(loc, GoToDingo)
		if err != nil {
			// Keep unchanged for locations that can't be translated (e.g., stdlib, external packages)
			s.config.Logger.Debugf("[Implementation]   [%d] Translation failed, keeping original: URI=%s, Line=%d",
				i, loc.URI.Filename(), loc.Range.Start.Line)
			translated = append(translated, loc)
		} else {
			s.config.Logger.Debugf("[Implementation]   [%d] Translated: URI=%s, Line=%d -> URI=%s, Line=%d",
				i, loc.URI.Filename(), loc.Range.Start.Line,
				translatedLoc.URI.Filename(), translatedLoc.Range.Start.Line)
			translated = append(translated, translatedLoc)
		}
	}

	s.config.Logger.Debugf("[Implementation] Returning %d locations", len(translated))
	return reply(ctx, translated, nil)
}

// handlePrepareCallHierarchy processes callHierarchy/prepare requests.
// Identifies the call hierarchy item at the cursor position.
//
// The call hierarchy protocol is stateful:
// 1. prepare returns CallHierarchyItem (we translate to Dingo positions for the client)
// 2. Client sends back item for incomingCalls/outgoingCalls
// 3. gopls expects original Go item, not translated Dingo item
//
// Solution: We embed the original Go item in the Data field (opaque JSON).
func (s *Server) handlePrepareCallHierarchy(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CallHierarchyPrepareParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/Prepare] Request for URI=%s, Line=%d, Char=%d",
		params.TextDocument.URI.Filename(), params.Position.Line, params.Position.Character)

	originalDingoURI := params.TextDocument.URI

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.PrepareCallHierarchy(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position -> Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("[CallHierarchy/Prepare] Position translation failed: %v", err)
		// Graceful degradation
		result, err := s.gopls.PrepareCallHierarchy(ctx, params)
		return reply(ctx, result, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/Prepare] Translated to Go: URI=%s, Line=%d, Char=%d",
		goURI.Filename(), goPos.Line, goPos.Character)

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Call gopls
	goItems, err := s.gopls.PrepareCallHierarchy(ctx, params)
	if err != nil {
		s.config.Logger.Warnf("[CallHierarchy/Prepare] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/Prepare] gopls returned %d items", len(goItems))

	if len(goItems) == 0 {
		return reply(ctx, goItems, nil)
	}

	// For each item, translate to Dingo and embed original Go item in Data
	dingoItems := make([]protocol.CallHierarchyItem, 0, len(goItems))
	for _, goItem := range goItems {
		// Check if this item is from a Dingo file
		goItemPath := goItem.URI.Filename()
		dingoPath := goToDingoPath(goItemPath)

		// Only translate if there's a corresponding .dingo file
		if dingoPath != goItemPath {
			dingoItem, err := s.callHierarchyCtx.TranslateItemToDingo(s.translator, goItem)
			if err != nil {
				s.config.Logger.Warnf("[CallHierarchy/Prepare] Translation failed: %v", err)
				// Keep original Go item for non-translatable items
				dingoItems = append(dingoItems, goItem)
				continue
			}
			dingoItems = append(dingoItems, dingoItem)
		} else {
			// Not a transpiled file, keep as-is but still embed for consistency
			item := goItem
			if err := s.callHierarchyCtx.EmbedGoItem(&item, goItem); err != nil {
				s.config.Logger.Warnf("[CallHierarchy/Prepare] Embed failed: %v", err)
			}
			dingoItems = append(dingoItems, item)
		}
	}

	s.config.Logger.Debugf("[CallHierarchy/Prepare] Returning %d Dingo items for %s",
		len(dingoItems), originalDingoURI.Filename())

	return reply(ctx, dingoItems, nil)
}

// handleIncomingCalls processes callHierarchy/incomingCalls requests.
// Finds all callers of a function.
func (s *Server) handleIncomingCalls(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CallHierarchyIncomingCallsParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/IncomingCalls] Request for item: %s at %s",
		params.Item.Name, params.Item.URI.Filename())

	// Recover the original Go item from the Data field
	goItem, err := s.callHierarchyCtx.RecoverGoItem(params.Item)
	if err != nil {
		s.config.Logger.Warnf("[CallHierarchy/IncomingCalls] Failed to recover Go item: %v", err)
		// Try with the item as-is (may work for pure Go files)
		goItem = params.Item
		goItem.Data = nil // Clear potentially invalid Data
	}

	s.config.Logger.Debugf("[CallHierarchy/IncomingCalls] Using Go item: %s at %s",
		goItem.Name, goItem.URI.Filename())

	// Call gopls with the original Go item
	goParams := protocol.CallHierarchyIncomingCallsParams{
		Item: goItem,
	}

	goCalls, err := s.gopls.IncomingCalls(ctx, goParams)
	if err != nil {
		s.config.Logger.Warnf("[CallHierarchy/IncomingCalls] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/IncomingCalls] gopls returned %d calls", len(goCalls))

	if len(goCalls) == 0 {
		return reply(ctx, goCalls, nil)
	}

	// Translate each incoming call result
	dingoCalls := make([]protocol.CallHierarchyIncomingCall, 0, len(goCalls))
	for _, goCall := range goCalls {
		// Translate the From item (the caller)
		goFromPath := goCall.From.URI.Filename()
		dingoFromPath := goToDingoPath(goFromPath)

		if dingoFromPath != goFromPath {
			// This is from a Dingo file - translate the From item
			dingoFrom, err := s.callHierarchyCtx.TranslateItemToDingo(s.translator, goCall.From)
			if err != nil {
				s.config.Logger.Warnf("[CallHierarchy/IncomingCalls] From translation failed: %v", err)
				dingoCalls = append(dingoCalls, goCall)
				continue
			}

			// Translate FromRanges (call sites within the caller)
			goFromURI := goCall.From.URI
			translatedRanges := make([]protocol.Range, 0, len(goCall.FromRanges))
			for _, rng := range goCall.FromRanges {
				_, newRange, err := s.translator.TranslateRange(goFromURI, rng, GoToDingo)
				if err != nil {
					// Skip untranslatable ranges
					continue
				}
				translatedRanges = append(translatedRanges, newRange)
			}

			dingoCalls = append(dingoCalls, protocol.CallHierarchyIncomingCall{
				From:       dingoFrom,
				FromRanges: translatedRanges,
			})
		} else {
			// Not from a Dingo file - keep as-is but embed Go item for follow-up
			item := goCall.From
			if err := s.callHierarchyCtx.EmbedGoItem(&item, goCall.From); err != nil {
				s.config.Logger.Warnf("[CallHierarchy/IncomingCalls] Embed failed: %v", err)
			}
			dingoCalls = append(dingoCalls, protocol.CallHierarchyIncomingCall{
				From:       item,
				FromRanges: goCall.FromRanges,
			})
		}
	}

	s.config.Logger.Debugf("[CallHierarchy/IncomingCalls] Returning %d Dingo calls", len(dingoCalls))
	return reply(ctx, dingoCalls, nil)
}

// handleOutgoingCalls processes callHierarchy/outgoingCalls requests.
// Finds all functions called by a function.
func (s *Server) handleOutgoingCalls(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CallHierarchyOutgoingCallsParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/OutgoingCalls] Request for item: %s at %s",
		params.Item.Name, params.Item.URI.Filename())

	// Recover the original Go item from the Data field
	goItem, err := s.callHierarchyCtx.RecoverGoItem(params.Item)
	if err != nil {
		s.config.Logger.Warnf("[CallHierarchy/OutgoingCalls] Failed to recover Go item: %v", err)
		// Try with the item as-is (may work for pure Go files)
		goItem = params.Item
		goItem.Data = nil // Clear potentially invalid Data
	}

	s.config.Logger.Debugf("[CallHierarchy/OutgoingCalls] Using Go item: %s at %s",
		goItem.Name, goItem.URI.Filename())

	// Call gopls with the original Go item
	goParams := protocol.CallHierarchyOutgoingCallsParams{
		Item: goItem,
	}

	goCalls, err := s.gopls.OutgoingCalls(ctx, goParams)
	if err != nil {
		s.config.Logger.Warnf("[CallHierarchy/OutgoingCalls] gopls error: %v", err)
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CallHierarchy/OutgoingCalls] gopls returned %d calls", len(goCalls))

	if len(goCalls) == 0 {
		return reply(ctx, goCalls, nil)
	}

	// Get the caller's Dingo path for translating FromRanges
	callerGoURI := goItem.URI
	callerDingoPath := goToDingoPath(callerGoURI.Filename())

	// Translate each outgoing call result
	dingoCalls := make([]protocol.CallHierarchyOutgoingCall, 0, len(goCalls))
	for _, goCall := range goCalls {
		// Translate the To item (the callee)
		goToPath := goCall.To.URI.Filename()
		dingoToPath := goToDingoPath(goToPath)

		var dingoTo protocol.CallHierarchyItem
		if dingoToPath != goToPath {
			// This points to a Dingo file - translate the To item
			translated, err := s.callHierarchyCtx.TranslateItemToDingo(s.translator, goCall.To)
			if err != nil {
				s.config.Logger.Warnf("[CallHierarchy/OutgoingCalls] To translation failed: %v", err)
				dingoTo = goCall.To
			} else {
				dingoTo = translated
			}
		} else {
			// Not a Dingo file - keep as-is but embed for potential follow-up
			dingoTo = goCall.To
			if err := s.callHierarchyCtx.EmbedGoItem(&dingoTo, goCall.To); err != nil {
				s.config.Logger.Warnf("[CallHierarchy/OutgoingCalls] Embed failed: %v", err)
			}
		}

		// Translate FromRanges (call sites within the caller)
		// These are in the caller's file, which may be a Dingo file
		translatedRanges := make([]protocol.Range, 0, len(goCall.FromRanges))
		if callerDingoPath != callerGoURI.Filename() {
			// Caller is in a Dingo file - translate ranges
			for _, rng := range goCall.FromRanges {
				_, newRange, err := s.translator.TranslateRange(callerGoURI, rng, GoToDingo)
				if err != nil {
					// Skip untranslatable ranges
					continue
				}
				translatedRanges = append(translatedRanges, newRange)
			}
		} else {
			// Caller is pure Go - keep ranges as-is
			translatedRanges = goCall.FromRanges
		}

		dingoCalls = append(dingoCalls, protocol.CallHierarchyOutgoingCall{
			To:         dingoTo,
			FromRanges: translatedRanges,
		})
	}

	s.config.Logger.Debugf("[CallHierarchy/OutgoingCalls] Returning %d Dingo calls", len(dingoCalls))
	return reply(ctx, dingoCalls, nil)
}
