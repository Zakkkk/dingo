package lsp

import (
	"context"
	"os"

	"go.lsp.dev/protocol"

	"github.com/MadAppGang/dingo/pkg/lint"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// runLintOnSave runs the Dingo linter when a .dingo file is saved
// and publishes diagnostics to the IDE
func (s *Server) runLintOnSave(ctx context.Context, uri protocol.DocumentURI) {
	dingoPath := uri.Filename()
	s.config.Logger.Debugf("[Lint] Running linter on saved file: %s", dingoPath)

	// Read file contents
	src, err := os.ReadFile(dingoPath)
	if err != nil {
		s.config.Logger.Warnf("[Lint] Failed to read file for linting: %v", err)
		return
	}

	// Create lint runner with default config
	runner := lint.NewRunner(lint.DefaultConfig())

	// Run analyzers on the file
	diagnostics, err := runner.Run(dingoPath, src)
	if err != nil {
		s.config.Logger.Warnf("[Lint] Linter failed: %v", err)
		// Clear diagnostics on error
		s.publishLintDiagnostics(uri, nil)
		return
	}

	s.config.Logger.Debugf("[Lint] Found %d diagnostic(s)", len(diagnostics))

	// Convert to LSP diagnostics
	lspDiags := make([]protocol.Diagnostic, len(diagnostics))
	for i, d := range diagnostics {
		lspDiags[i] = convertToLSPDiagnostic(d)
	}

	// Publish diagnostics
	s.publishLintDiagnostics(uri, lspDiags)
}

// convertToLSPDiagnostic converts a Dingo analyzer.Diagnostic to protocol.Diagnostic
func convertToLSPDiagnostic(d analyzer.Diagnostic) protocol.Diagnostic {
	// Convert position to LSP Range
	// token.Position is 1-based, LSP is 0-based
	startLine := uint32(d.Pos.Line - 1)
	startChar := uint32(d.Pos.Column - 1)
	endLine := uint32(d.End.Line - 1)
	endChar := uint32(d.End.Column - 1)

	// If End is not set (zero value), use start position + 1
	if d.End.Line == 0 {
		endLine = startLine
		endChar = startChar + 1
	}

	lspRange := protocol.Range{
		Start: protocol.Position{
			Line:      startLine,
			Character: startChar,
		},
		End: protocol.Position{
			Line:      endLine,
			Character: endChar,
		},
	}

	// All diagnostics are warnings (advisory mode)
	severity := protocol.DiagnosticSeverityWarning

	// Convert analyzer.Severity to LSP severity if needed
	switch d.Severity {
	case analyzer.SeverityHint:
		severity = protocol.DiagnosticSeverityHint
	case analyzer.SeverityInfo:
		severity = protocol.DiagnosticSeverityInformation
	case analyzer.SeverityWarning:
		severity = protocol.DiagnosticSeverityWarning
	}

	lspDiag := protocol.Diagnostic{
		Range:    lspRange,
		Severity: severity,
		Code:     d.Code, // e.g., "D001", "R001"
		Source:   "dingo-lint",
		Message:  d.Message,
	}

	// Add related information if present
	if len(d.Related) > 0 {
		relatedInfo := make([]protocol.DiagnosticRelatedInformation, len(d.Related))
		for j, rel := range d.Related {
			relatedInfo[j] = protocol.DiagnosticRelatedInformation{
				Location: protocol.Location{
					URI: protocol.DocumentURI("file://" + rel.Pos.Filename),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(rel.Pos.Line - 1),
							Character: uint32(rel.Pos.Column - 1),
						},
						End: protocol.Position{
							Line:      uint32(rel.Pos.Line - 1),
							Character: uint32(rel.Pos.Column),
						},
					},
				},
				Message: rel.Message,
			}
		}
		lspDiag.RelatedInformation = relatedInfo
	}

	return lspDiag
}

// publishLintDiagnostics publishes lint diagnostics to the IDE
// This is separate from publishDingoDiagnostics (transpiler errors) and
// handlePublishDiagnostics (gopls diagnostics)
func (s *Server) publishLintDiagnostics(uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) {
	// Get IDE connection (thread-safe)
	ideConn, serverCtx := s.GetConn()
	if ideConn == nil {
		s.config.Logger.Warnf("[Lint] No IDE connection available, cannot publish diagnostics")
		return
	}

	// Prepare params
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	// Use server context if available, otherwise background
	publishCtx := serverCtx
	if publishCtx == nil {
		publishCtx = context.Background()
	}

	// Double-check connection before notify (prevent TOCTOU race)
	if ideConn == nil {
		s.config.Logger.Warnf("[Lint] Connection became nil before publish")
		return
	}

	// Publish to IDE
	err := ideConn.Notify(publishCtx, "textDocument/publishDiagnostics", params)
	if err != nil {
		s.config.Logger.Errorf("[Lint] Failed to publish diagnostics: %v", err)
		return
	}

	if len(diagnostics) > 0 {
		s.config.Logger.Debugf("[Lint] Published %d lint diagnostic(s) for %s", len(diagnostics), uri)
	} else {
		s.config.Logger.Debugf("[Lint] Cleared lint diagnostics for %s", uri)
	}
}
