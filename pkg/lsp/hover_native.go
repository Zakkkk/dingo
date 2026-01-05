package lsp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MadAppGang/dingo/pkg/lsp/semantic"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"go.lsp.dev/protocol"
)

// nativeHover implements Dingo-native hover using semantic map
// This is the Phase 1 implementation: run go/types directly on generated Go code
// and build a semantic map from Dingo positions to type information.
func (s *Server) nativeHover(ctx context.Context, params protocol.HoverParams) (*protocol.Hover, error) {
	uri := string(params.TextDocument.URI)

	s.config.Logger.Debugf("[Native Hover] Request for URI=%s, Line=%d, Char=%d",
		uri, params.Position.Line, params.Position.Character)

	// 1. Check for stored transpile errors FIRST (before semantic manager)
	// This ensures error hovers work even when transpilation fails
	line := int(params.Position.Line) + 1
	col := int(params.Position.Character) + 1
	if hover := s.getErrorHover(uri, line, col); hover != nil {
		s.config.Logger.Debugf("[Native Hover] Returning error hover for line %d", line)
		return hover, nil
	}

	// 2. Get document source
	source, err := s.getDocumentSource(uri)
	if err != nil {
		s.config.Logger.Warnf("[Native Hover] Failed to get document source: %v", err)
		return nil, err
	}

	// 3. Get typed document from semantic manager
	// This may return a slightly stale document during typing (debounced rebuild)
	doc, err := s.semanticManager.Get(uri, source)
	if err != nil {
		s.config.Logger.Warnf("[Native Hover] Failed to get semantic document: %v", err)
		return nil, err
	}

	// 4. Check if build succeeded
	if doc.BuildError != nil {
		s.config.Logger.Debugf("[Native Hover] Document has build errors: %v", doc.BuildError)
		// Return nil hover instead of error - allow user to keep typing
		return nil, nil
	}

	if doc.SemanticMap == nil {
		s.config.Logger.Debugf("[Native Hover] No semantic map available")
		return nil, nil
	}

	// line, col already computed above (1-indexed Dingo position)
	s.config.Logger.Debugf("[Native Hover] Looking up semantic entity at Dingo position: line=%d, col=%d", line, col)

	// 5. Look up semantic entity at position

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

// getErrorHover returns rich markdown hover for error positions
// This provides detailed help when hovering over code with errors
func (s *Server) getErrorHover(uri string, line, col int) *protocol.Hover {
	s.lastErrorMu.RLock()
	te := s.lastErrors[uri]
	s.lastErrorMu.RUnlock()

	s.config.Logger.Debugf("[Error Hover] Checking for error at URI=%s, line=%d (stored errors: %d)",
		uri, line, len(s.lastErrors))

	if te == nil {
		s.config.Logger.Debugf("[Error Hover] No stored error for URI")
		return nil
	}

	s.config.Logger.Debugf("[Error Hover] Found stored error at line %d, hover line %d", te.Line, line)

	// Check if hover position is on the same line as the error
	// Allow some tolerance for column position
	if te.Line != line {
		s.config.Logger.Debugf("[Error Hover] Line mismatch: error=%d, hover=%d", te.Line, line)
		return nil
	}

	// Format rich markdown based on error kind
	content := formatErrorHoverMarkdown(te)
	if content == "" {
		return nil
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: content,
		},
	}
}

// formatErrorHoverMarkdown creates rich markdown content for an error
func formatErrorHoverMarkdown(te *transpiler.TranspileError) string {
	if te == nil {
		return ""
	}

	switch te.Kind {
	case transpiler.ErrorKindUnresolvedLambda:
		if data, ok := te.Data.(transpiler.UnresolvedLambdaErrorData); ok {
			return formatUnresolvedLambdaHoverMD(data)
		}
	case transpiler.ErrorKindNullCoalesce:
		if data, ok := te.Data.(transpiler.NullCoalesceErrorData); ok {
			return formatNullCoalesceHoverMD(data)
		}
	case transpiler.ErrorKindParsing:
		if data, ok := te.Data.(transpiler.ParsingErrorData); ok {
			return formatParsingHoverMD(data)
		}
	}

	// Fallback: enhance generic message
	return formatGenericHoverMD(te.Message)
}

// formatUnresolvedLambdaHoverMD creates detailed help for unresolved lambda errors
func formatUnresolvedLambdaHoverMD(data transpiler.UnresolvedLambdaErrorData) string {
	var parts []string
	if len(data.ParamNames) > 0 {
		parts = append(parts, strings.Join(data.ParamNames, ", "))
	}
	if data.HasAnyReturn {
		parts = append(parts, "return")
	}
	issue := strings.Join(parts, ", ")

	paramExample := "x"
	if len(data.ParamNames) > 0 {
		paramExample = data.ParamNames[0]
	}

	// Use proper markdown with inline code for hover
	return "## 🔴 Cannot Infer Type\n\n" +
		"Type inference failed for: **" + issue + "**\n\n" +
		"### 💡 Add explicit type annotation\n\n" +
		"- " + "`|" + paramExample + " int| expr`" + " — Rust style\n" +
		"- " + "`(" + paramExample + " int) => expr`" + " — TypeScript style\n" +
		"- " + "`func(" + paramExample + " int) int { return expr }`" + " — Go style"
}

// formatNullCoalesceHoverMD creates detailed help for null coalesce errors
func formatNullCoalesceHoverMD(data transpiler.NullCoalesceErrorData) string {
	return "## 🔴 Null Coalescing Error\n\n" +
		"The " + "`??`" + " operator requires a default value on the right side.\n\n" +
		"### 💡 Usage\n\n" +
		"`value ?? defaultValue`\n\n" +
		"### 📝 Examples\n\n" +
		"- " + "`name := user.Name ?? \"Anonymous\"`" + "\n" +
		"- " + "`count := getCount() ?? 0`"
}

// formatParsingHoverMD creates detailed help for parsing errors
func formatParsingHoverMD(data transpiler.ParsingErrorData) string {
	var b strings.Builder
	b.WriteString("## 🔴 Syntax Error\n\n")

	if data.Expected != "" && data.Found != "" {
		b.WriteString("| | |\n")
		b.WriteString("|---|---|\n")
		b.WriteString("| **Expected** | `" + data.Expected + "` |\n")
		b.WriteString("| **Found** | `" + data.Found + "` |\n\n")
	}

	// Add helpful hints based on what was expected
	b.WriteString("### 💡 Common fixes\n\n")
	if strings.Contains(data.Expected, ")") {
		b.WriteString("- Check for missing closing parenthesis\n")
		b.WriteString("- Ensure all function calls have matching `(` and `)`\n")
	} else if strings.Contains(data.Expected, "}") {
		b.WriteString("- Check for missing closing brace\n")
		b.WriteString("- Ensure all blocks have matching `{` and `}`\n")
	} else if strings.Contains(data.Expected, ";") {
		b.WriteString("- Check for syntax errors on the previous line\n")
		b.WriteString("- In Dingo, semicolons are optional\n")
	} else {
		b.WriteString("- Review the syntax near this location\n")
		b.WriteString("- Check for typos or missing tokens\n")
	}

	return b.String()
}

// formatGenericHoverMD creates markdown for generic error messages
func formatGenericHoverMD(msg string) string {
	var b strings.Builder
	b.WriteString("## 🔴 Error\n\n")
	b.WriteString("```\n")
	b.WriteString(msg)
	b.WriteString("\n```\n")

	// Add hints for common patterns
	if strings.Contains(msg, "undefined:") {
		b.WriteString("\n### 💡 Hint\n\n")
		b.WriteString("Check that the identifier is:\n")
		b.WriteString("- Spelled correctly\n")
		b.WriteString("- Properly imported\n")
		b.WriteString("- Defined in scope\n")
	}

	return b.String()
}
