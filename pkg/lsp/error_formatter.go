package lsp

import (
	"regexp"
	"strings"

	"github.com/MadAppGang/dingo/pkg/transpiler"
)

// ErrorFormatter formats transpile errors for specific editor types.
// Each editor has different capabilities for rendering error messages.
type ErrorFormatter interface {
	Format(err *transpiler.TranspileError) string
}

// =============================================================================
// SimpleFormatter - Single-line messages for unknown editors
// =============================================================================

type SimpleFormatter struct{}

func (f *SimpleFormatter) Format(err *transpiler.TranspileError) string {
	if err == nil {
		return ""
	}

	switch err.Kind {
	case transpiler.ErrorKindUnresolvedLambda:
		if data, ok := err.Data.(transpiler.UnresolvedLambdaErrorData); ok {
			return formatUnresolvedLambdaSimple(data)
		}
	case transpiler.ErrorKindNullCoalesce:
		if data, ok := err.Data.(transpiler.NullCoalesceErrorData); ok {
			return formatNullCoalesceSimple(data)
		}
	case transpiler.ErrorKindParsing:
		if data, ok := err.Data.(transpiler.ParsingErrorData); ok {
			return formatParsingSimple(data)
		}
	}

	// Fallback: check message for known patterns and enhance
	return enhanceGenericMessage(err.Message)
}

func formatUnresolvedLambdaSimple(data transpiler.UnresolvedLambdaErrorData) string {
	issue := buildIssueParts(data)
	paramExample := getParamExample(data)
	return "cannot infer type for '" + issue + "' - add annotation: |" + paramExample + " Type| or (" + paramExample + " Type) =>"
}

func formatNullCoalesceSimple(data transpiler.NullCoalesceErrorData) string {
	return "null coalescing '??' requires default value: " + data.Expression + " ?? defaultValue"
}

func formatParsingSimple(data transpiler.ParsingErrorData) string {
	if data.Expected != "" && data.Found != "" {
		return "syntax error: expected '" + data.Expected + "', found '" + data.Found + "'"
	}
	return "syntax error"
}

// =============================================================================
// MultilineFormatter - VS Code, Neovim, Emacs (newlines work)
// =============================================================================

type MultilineFormatter struct{}

func (f *MultilineFormatter) Format(err *transpiler.TranspileError) string {
	if err == nil {
		return ""
	}

	switch err.Kind {
	case transpiler.ErrorKindUnresolvedLambda:
		if data, ok := err.Data.(transpiler.UnresolvedLambdaErrorData); ok {
			return formatUnresolvedLambdaMultiline(data)
		}
	case transpiler.ErrorKindNullCoalesce:
		if data, ok := err.Data.(transpiler.NullCoalesceErrorData); ok {
			return formatNullCoalesceMultiline(data)
		}
	case transpiler.ErrorKindParsing:
		if data, ok := err.Data.(transpiler.ParsingErrorData); ok {
			return formatParsingMultiline(data)
		}
	}

	// Fallback: enhance generic message with multiline formatting
	return enhanceGenericMessageMultiline(err.Message)
}

func formatUnresolvedLambdaMultiline(data transpiler.UnresolvedLambdaErrorData) string {
	issue := buildIssueParts(data)
	paramExample := getParamExample(data)

	return "❌ Cannot infer type for '" + issue + "'\n\n" +
		"💡 Add explicit type annotation:\n\n" +
		"    |" + paramExample + " int| expr     — Rust style\n" +
		"    (" + paramExample + " int) => expr  — TypeScript style"
}

func formatNullCoalesceMultiline(data transpiler.NullCoalesceErrorData) string {
	return "❌ Null coalescing ?? requires a default value\n\n" +
		"💡 Usage: value ?? defaultValue\n\n" +
		"    name := user.Name ?? \"Anonymous\"\n" +
		"    count := getCount() ?? 0"
}

func formatParsingMultiline(data transpiler.ParsingErrorData) string {
	var b strings.Builder
	b.WriteString("❌ Syntax Error\n\n")

	if data.Expected != "" && data.Found != "" {
		b.WriteString("    Expected: " + data.Expected + "\n")
		b.WriteString("    Found:    " + data.Found + "\n")
	}

	b.WriteString("\n💡 Check for missing or mismatched brackets")

	if data.Context != "" {
		b.WriteString("\n\n📝 Context: " + data.Context)
	}

	return b.String()
}

func enhanceGenericMessageMultiline(msg string) string {
	// Enhance known error patterns with helpful suggestions
	if strings.Contains(msg, "expected ')'") {
		return "❌ " + msg + "\n\n💡 Check for missing closing parenthesis"
	}
	if strings.Contains(msg, "expected '}'") {
		return "❌ " + msg + "\n\n💡 Check for missing closing brace"
	}
	if strings.Contains(msg, "expected ';'") {
		return "❌ " + msg + "\n\n💡 Semicolons are optional in Dingo - check previous line"
	}
	if strings.Contains(msg, "undefined:") {
		return "❌ " + msg + "\n\n💡 Check spelling or ensure it's imported/defined"
	}
	return "❌ " + msg
}

// =============================================================================
// JetBrainsFormatter - HTML for WebStorm/GoLand hover tooltips
// =============================================================================

type JetBrainsFormatter struct{}

func (f *JetBrainsFormatter) Format(err *transpiler.TranspileError) string {
	if err == nil {
		return ""
	}

	switch err.Kind {
	case transpiler.ErrorKindUnresolvedLambda:
		if data, ok := err.Data.(transpiler.UnresolvedLambdaErrorData); ok {
			return formatUnresolvedLambdaHTML(data)
		}
	case transpiler.ErrorKindNullCoalesce:
		if data, ok := err.Data.(transpiler.NullCoalesceErrorData); ok {
			return formatNullCoalesceHTML(data)
		}
	case transpiler.ErrorKindParsing:
		if data, ok := err.Data.(transpiler.ParsingErrorData); ok {
			return formatParsingHTML(data)
		}
	}

	// Fallback: enhance generic message with HTML
	return enhanceGenericMessageHTML(err.Message)
}

func formatUnresolvedLambdaHTML(data transpiler.UnresolvedLambdaErrorData) string {
	issue := buildIssueParts(data)
	paramExample := getParamExample(data)

	return "<html><body>" +
		"<b style='color:#ff6b6b'>Cannot infer type for '" + issue + "'</b><br/><br/>" +
		"<b>Add explicit type annotation:</b><br/>" +
		"<table style='margin-left:10px'>" +
		"<tr><td><b>Rust:</b></td><td><code>|" + paramExample + " int| expr</code></td></tr>" +
		"<tr><td><b>TypeScript:</b></td><td><code>(" + paramExample + " int) =&gt; expr</code></td></tr>" +
		"<tr><td><b>Go:</b></td><td><code>func(" + paramExample + " int) int { return expr }</code></td></tr>" +
		"</table>" +
		"</body></html>"
}

func formatNullCoalesceHTML(data transpiler.NullCoalesceErrorData) string {
	return "<html><body>" +
		"<b style='color:#ff6b6b'>Null coalescing '??' requires a default value</b><br/><br/>" +
		"<b>Usage:</b><br/>" +
		"<code style='margin-left:10px'>value ?? defaultValue</code><br/><br/>" +
		"<b>Examples:</b><br/>" +
		"<code style='margin-left:10px'>name := user.Name ?? \"Anonymous\"</code><br/>" +
		"<code style='margin-left:10px'>count := getCount() ?? 0</code>" +
		"</body></html>"
}

func formatParsingHTML(data transpiler.ParsingErrorData) string {
	var b strings.Builder
	b.WriteString("<html><body>")
	b.WriteString("<b style='color:#ff6b6b'>Syntax Error</b><br/><br/>")

	if data.Expected != "" && data.Found != "" {
		b.WriteString("<table>")
		b.WriteString("<tr><td><b>Expected:</b></td><td><code>" + escapeHTML(data.Expected) + "</code></td></tr>")
		b.WriteString("<tr><td><b>Found:</b></td><td><code>" + escapeHTML(data.Found) + "</code></td></tr>")
		b.WriteString("</table>")
	}

	if data.Context != "" {
		b.WriteString("<br/><b>Context:</b><br/>")
		b.WriteString("<code style='margin-left:10px'>" + escapeHTML(data.Context) + "</code>")
	}

	b.WriteString("</body></html>")
	return b.String()
}

func enhanceGenericMessageHTML(msg string) string {
	escapedMsg := escapeHTML(msg)
	hint := ""

	if strings.Contains(msg, "expected ')'") {
		hint = "Check for missing closing parenthesis or extra opening parenthesis"
	} else if strings.Contains(msg, "expected '}'") {
		hint = "Check for missing closing brace or mismatched brackets"
	} else if strings.Contains(msg, "expected ';'") {
		hint = "In Dingo, semicolons are optional - check for syntax errors on previous line"
	} else if strings.Contains(msg, "undefined:") {
		hint = "Check spelling or ensure the identifier is imported/defined"
	}

	if hint != "" {
		return "<html><body>" +
			"<b style='color:#ff6b6b'>" + escapedMsg + "</b><br/><br/>" +
			"<i style='color:#888'>Hint: " + hint + "</i>" +
			"</body></html>"
	}

	return "<html><body><b style='color:#ff6b6b'>" + escapedMsg + "</b></body></html>"
}

// =============================================================================
// Helper functions
// =============================================================================

func buildIssueParts(data transpiler.UnresolvedLambdaErrorData) string {
	var parts []string
	if len(data.ParamNames) > 0 {
		parts = append(parts, strings.Join(data.ParamNames, ", "))
	}
	if data.HasAnyReturn {
		parts = append(parts, "return")
	}
	return strings.Join(parts, ", ")
}

func getParamExample(data transpiler.UnresolvedLambdaErrorData) string {
	if len(data.ParamNames) > 0 {
		return data.ParamNames[0]
	}
	return "x"
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func enhanceGenericMessage(msg string) string {
	// For simple formatter, just return the message as-is
	return msg
}

// ParseErrorMessage extracts structured data from common error message patterns
func ParseErrorMessage(msg string) (transpiler.ErrorKind, any) {
	// Match "expected 'X', found 'Y'" pattern (Y may or may not have quotes)
	expectedFoundRe := regexp.MustCompile(`expected '([^']+)', found '?([^']+)'?`)
	if matches := expectedFoundRe.FindStringSubmatch(msg); len(matches) == 3 {
		return transpiler.ErrorKindParsing, transpiler.ParsingErrorData{
			Expected: matches[1],
			Found:    matches[2],
		}
	}

	// Match null coalescing error
	if strings.Contains(msg, "??") && strings.Contains(msg, "requires") {
		return transpiler.ErrorKindNullCoalesce, transpiler.NullCoalesceErrorData{
			Expression: extractExpression(msg),
		}
	}

	return transpiler.ErrorKindGeneric, nil
}

func extractExpression(msg string) string {
	// Simple extraction - could be enhanced
	if idx := strings.Index(msg, "??"); idx > 0 {
		return strings.TrimSpace(msg[:idx])
	}
	return ""
}

// GetFormatterForEditor returns the appropriate formatter for the detected editor type.
func GetFormatterForEditor(editorType EditorType) ErrorFormatter {
	switch editorType {
	case EditorVSCode, EditorNeovim, EditorEmacs:
		return &MultilineFormatter{}
	case EditorJetBrains:
		return &JetBrainsFormatter{}
	default:
		return &SimpleFormatter{}
	}
}
