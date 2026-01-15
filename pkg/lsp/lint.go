package lsp

import (
	"context"
	"os"

	"go.lsp.dev/protocol"

	"github.com/MadAppGang/dingo/pkg/lint"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// runLintOnSave runs the Dingo linter when a .dingo file is saved
func (s *Server) runLintOnSave(ctx context.Context, uri protocol.DocumentURI) {
	s.runLint(uri, "save")
}

// runLintOnOpen runs the Dingo linter when a .dingo file is opened
func (s *Server) runLintOnOpen(ctx context.Context, uri protocol.DocumentURI) {
	s.runLint(uri, "open")
}

// runLint is the shared implementation for linting on open/save
func (s *Server) runLint(uri protocol.DocumentURI, trigger string) {
	dingoPath := uri.Filename()
	s.config.Logger.Debugf("[Lint] Running linter on %s (%s)", dingoPath, trigger)

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

	// Default to warning
	severity := protocol.DiagnosticSeverityWarning

	// Convert analyzer.Severity to LSP severity
	switch d.Severity {
	case analyzer.SeverityError:
		severity = protocol.DiagnosticSeverityError
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
// Uses the unified diagnostic cache to merge with gopls/transpiler diagnostics
func (s *Server) publishLintDiagnostics(uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) {
	s.config.Logger.Debugf("[Lint] Updating %d lint diagnostic(s) for %s", len(diagnostics), uri)
	s.updateAndPublishDiagnostics(uri, "lint", diagnostics)
}
