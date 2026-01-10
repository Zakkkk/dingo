package lsp

import (
	"context"
	"encoding/json"
	"os"

	"github.com/MadAppGang/dingo/pkg/lint"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
	"github.com/MadAppGang/dingo/pkg/lint/refactor"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// handleCodeAction processes textDocument/codeAction requests.
// This handler provides refactoring suggestions as Code Actions for .dingo files.
//
// It runs the refactoring analyzer on the file and converts matching diagnostics
// to LSP CodeAction responses with WorkspaceEdit for IDE quick fixes.
func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.config.Logger.Debugf("[CodeAction] Request for URI=%s, Range=L%d:C%d-L%d:C%d",
		params.TextDocument.URI.Filename(),
		params.Range.Start.Line, params.Range.Start.Character,
		params.Range.End.Line, params.Range.End.Character)

	// Only process .dingo files
	if !isDingoFile(params.TextDocument.URI) {
		s.config.Logger.Debugf("[CodeAction] Skipping non-dingo file: %s", params.TextDocument.URI)
		return reply(ctx, nil, nil)
	}

	// Read file contents
	dingoPath := params.TextDocument.URI.Filename()
	src, err := os.ReadFile(dingoPath)
	if err != nil {
		// File might not exist yet or be inaccessible - return empty actions
		s.config.Logger.Debugf("[CodeAction] File not readable, returning empty actions: %v", err)
		return reply(ctx, []protocol.CodeAction{}, nil)
	}

	// Create lint runner with refactoring analyzer
	runner := createRefactoringRunner()

	// Run refactoring analyzer
	diagnostics, err := runner.Run(dingoPath, src)
	if err != nil {
		// Parse errors are expected when user is actively editing with syntax errors.
		// Return empty code actions instead of failing - this is graceful degradation.
		s.config.Logger.Debugf("[CodeAction] Refactoring analysis skipped (parse error): %v", err)
		return reply(ctx, []protocol.CodeAction{}, nil)
	}

	s.config.Logger.Debugf("[CodeAction] Found %d diagnostics from refactoring analyzer", len(diagnostics))

	// Filter diagnostics to those overlapping the requested range
	relevantDiags := filterDiagnosticsInRange(diagnostics, params.Range)
	s.config.Logger.Debugf("[CodeAction] Filtered to %d diagnostics in requested range", len(relevantDiags))

	// Convert diagnostics with fixes to CodeActions
	var actions []protocol.CodeAction
	for _, diag := range relevantDiags {
		if len(diag.Fixes) == 0 {
			continue
		}

		s.config.Logger.Debugf("[CodeAction] Diagnostic [%s] has %d fix(es): %s",
			diag.Code, len(diag.Fixes), diag.Message)

		for _, fix := range diag.Fixes {
			action := convertFixToCodeAction(fix, diag, params.TextDocument.URI)
			actions = append(actions, action)

			s.config.Logger.Debugf("[CodeAction]   - Fix: %s (preferred=%v, edits=%d)",
				fix.Title, fix.IsPreferred, len(fix.Edits))
		}
	}

	s.config.Logger.Infof("[CodeAction] Returning %d code action(s)", len(actions))
	return reply(ctx, actions, nil)
}

// createRefactoringRunner creates a lint runner configured for refactoring analysis only
func createRefactoringRunner() *lint.Runner {
	cfg := lint.DefaultConfig()
	runner := lint.NewRunner(cfg)

	// Register refactoring analyzer
	runner.RegisterAnalyzer(refactor.NewRefactoringAnalyzer())

	return runner
}

// filterDiagnosticsInRange filters diagnostics to those overlapping the requested range
func filterDiagnosticsInRange(diagnostics []analyzer.Diagnostic, requestedRange protocol.Range) []analyzer.Diagnostic {
	var filtered []analyzer.Diagnostic

	for _, diag := range diagnostics {
		// Convert token.Position to protocol.Position (0-based)
		diagRange := protocol.Range{
			Start: protocol.Position{
				Line:      uint32(diag.Pos.Line - 1),
				Character: uint32(diag.Pos.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(diag.End.Line - 1),
				Character: uint32(diag.End.Column - 1),
			},
		}

		// Check if ranges overlap
		if rangesOverlap(diagRange, requestedRange) {
			filtered = append(filtered, diag)
		}
	}

	return filtered
}

// rangesOverlap checks if two LSP ranges overlap
func rangesOverlap(a, b protocol.Range) bool {
	// No overlap if one range ends before the other starts
	if comparePositions(a.End, b.Start) < 0 {
		return false
	}
	if comparePositions(b.End, a.Start) < 0 {
		return false
	}
	return true
}

// comparePositions compares two LSP positions
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func comparePositions(a, b protocol.Position) int {
	if a.Line < b.Line {
		return -1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Character < b.Character {
		return -1
	}
	if a.Character > b.Character {
		return 1
	}
	return 0
}

// convertFixToCodeAction converts an analyzer.Fix to an LSP CodeAction
func convertFixToCodeAction(fix analyzer.Fix, diag analyzer.Diagnostic, uri protocol.DocumentURI) protocol.CodeAction {
	// Convert analyzer.TextEdit to protocol.TextEdit
	var textEdits []protocol.TextEdit
	for _, edit := range fix.Edits {
		textEdits = append(textEdits, protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(edit.Pos.Line - 1),
					Character: uint32(edit.Pos.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(edit.End.Line - 1),
					Character: uint32(edit.End.Column - 1),
				},
			},
			NewText: edit.NewText,
		})
	}

	// Determine code action kind based on category
	kind := protocol.QuickFix
	if diag.Category == "refactor" {
		kind = protocol.RefactorRewrite
	}

	// Convert diagnostic to LSP format
	lspDiag := protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(diag.Pos.Line - 1),
				Character: uint32(diag.Pos.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(diag.End.Line - 1),
				Character: uint32(diag.End.Column - 1),
			},
		},
		Severity: convertSeverityToLSP(diag.Severity),
		Code:     diag.Code,
		Source:   "dingo",
		Message:  diag.Message,
	}

	return protocol.CodeAction{
		Title:       fix.Title,
		Kind:        kind,
		Diagnostics: []protocol.Diagnostic{lspDiag},
		IsPreferred: fix.IsPreferred,
		Edit: &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				uri: textEdits,
			},
		},
	}
}

// convertSeverityToLSP converts analyzer.Severity to protocol.DiagnosticSeverity
func convertSeverityToLSP(sev analyzer.Severity) protocol.DiagnosticSeverity {
	switch sev {
	case analyzer.SeverityHint:
		return protocol.DiagnosticSeverityHint
	case analyzer.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	case analyzer.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	default:
		return protocol.DiagnosticSeverityHint
	}
}
