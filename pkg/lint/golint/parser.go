package golint

import (
	"encoding/json"
	"fmt"
	"go/token"
	"path/filepath"

	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// GolangciOutput represents the top-level JSON output from golangci-lint
type GolangciOutput struct {
	Issues []Issue `json:"Issues"`
	Report Report  `json:"Report"`
}

// Issue represents a single linting issue from golangci-lint
type Issue struct {
	FromLinter  string   `json:"FromLinter"`
	Text        string   `json:"Text"`
	Severity    string   `json:"Severity"`
	SourceLines []string `json:"SourceLines"`
	Pos         Position `json:"Pos"`
	// Replacement field is optional and used for auto-fixes
	Replacement *Replacement `json:"Replacement,omitempty"`
}

// Position represents the position of an issue
type Position struct {
	Filename string `json:"Filename"`
	Offset   int    `json:"Offset"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// Replacement represents a suggested fix
type Replacement struct {
	NewLines   []string `json:"NewLines"`
	Inline     *InlineFix `json:"Inline,omitempty"`
}

// InlineFix represents an inline replacement
type InlineFix struct {
	StartCol int    `json:"StartCol"`
	Length   int    `json:"Length"`
	NewString string `json:"NewString"`
}

// Report contains summary statistics
type Report struct {
	Linters []LinterStats `json:"Linters"`
	Warnings []string      `json:"Warnings"`
}

// LinterStats contains statistics for a single linter
type LinterStats struct {
	Name             string `json:"Name"`
	Enabled          bool   `json:"Enabled"`
	EnabledByDefault bool   `json:"EnabledByDefault"`
}

// Parser converts golangci-lint JSON output to Dingo diagnostics
type Parser struct {
	// basePath is used to make file paths relative
	basePath string
}

// NewParser creates a new golangci-lint output parser
func NewParser(basePath string) *Parser {
	return &Parser{
		basePath: basePath,
	}
}

// Parse parses golangci-lint JSON output into diagnostics
func (p *Parser) Parse(jsonData []byte) ([]analyzer.Diagnostic, error) {
	if len(jsonData) == 0 || string(jsonData) == "{}" {
		return nil, nil
	}

	var output GolangciOutput
	if err := json.Unmarshal(jsonData, &output); err != nil {
		return nil, fmt.Errorf("unmarshal golangci-lint output: %w", err)
	}

	diagnostics := make([]analyzer.Diagnostic, 0, len(output.Issues))
	for _, issue := range output.Issues {
		diag, err := p.issueToDiagnostic(issue)
		if err != nil {
			// Log but don't fail on individual issue parsing errors
			continue
		}
		diagnostics = append(diagnostics, diag)
	}

	return diagnostics, nil
}

// issueToDiagnostic converts a golangci-lint Issue to a Dingo Diagnostic
func (p *Parser) issueToDiagnostic(issue Issue) (analyzer.Diagnostic, error) {
	// Make filename relative to base path if possible
	filename := issue.Pos.Filename
	if p.basePath != "" {
		if rel, err := filepath.Rel(p.basePath, filename); err == nil {
			filename = rel
		}
	}

	// Create position
	pos := token.Position{
		Filename: filename,
		Line:     issue.Pos.Line,
		Column:   issue.Pos.Column,
	}

	// End position is same as start (golangci-lint doesn't provide ranges)
	// The position mapper will need to infer the range
	end := pos

	// Map severity
	severity := mapSeverity(issue.Severity)

	// Create diagnostic
	diag := analyzer.Diagnostic{
		Pos:      pos,
		End:      end,
		Message:  issue.Text,
		Severity: severity,
		Code:     fmt.Sprintf("go:%s", issue.FromLinter),
		Category: "go-lint",
	}

	// Add fixes if available
	if issue.Replacement != nil {
		fix := p.replacementToFix(issue.Replacement, pos)
		if fix != nil {
			diag.Fixes = []analyzer.Fix{*fix}
		}
	}

	return diag, nil
}

// replacementToFix converts a golangci-lint Replacement to a Dingo Fix
func (p *Parser) replacementToFix(repl *Replacement, pos token.Position) *analyzer.Fix {
	if repl.Inline != nil {
		// Inline replacement
		editPos := pos
		editPos.Column = repl.Inline.StartCol

		editEnd := editPos
		editEnd.Column += repl.Inline.Length

		return &analyzer.Fix{
			Title: "Apply suggested fix",
			Edits: []analyzer.TextEdit{
				{
					Pos:     editPos,
					End:     editEnd,
					NewText: repl.Inline.NewString,
				},
			},
			IsPreferred: true,
		}
	}

	if len(repl.NewLines) > 0 {
		// Multi-line replacement
		// This is more complex - we'd need to know the full range
		// For now, we'll skip these and only support inline fixes
		return nil
	}

	return nil
}

// mapSeverity maps golangci-lint severity to Dingo severity
func mapSeverity(golangciSev string) analyzer.Severity {
	switch golangciSev {
	case "error":
		return analyzer.SeverityWarning // All Go lint issues are warnings in advisory mode
	case "warning":
		return analyzer.SeverityWarning
	default:
		return analyzer.SeverityInfo
	}
}

// ParseWithWarnings parses the output and also returns any warnings
func (p *Parser) ParseWithWarnings(jsonData []byte) ([]analyzer.Diagnostic, []string, error) {
	if len(jsonData) == 0 || string(jsonData) == "{}" {
		return nil, nil, nil
	}

	var output GolangciOutput
	if err := json.Unmarshal(jsonData, &output); err != nil {
		return nil, nil, fmt.Errorf("unmarshal golangci-lint output: %w", err)
	}

	diagnostics := make([]analyzer.Diagnostic, 0, len(output.Issues))
	for _, issue := range output.Issues {
		diag, err := p.issueToDiagnostic(issue)
		if err != nil {
			continue
		}
		diagnostics = append(diagnostics, diag)
	}

	return diagnostics, output.Report.Warnings, nil
}
