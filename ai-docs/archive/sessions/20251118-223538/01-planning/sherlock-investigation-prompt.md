# LSP Source Mapping Bug Investigation

## Problem Description

The Dingo language server is underlining the WRONG part of code when reporting errors from gopls.

**Expected behavior:**
- Should underline `ReadFile` (the function call that doesn't exist)

**Actual behavior:**
- Underlines `e(path)?` instead (the error propagation operator)

## Example Code

**Dingo source** (`error_prop_01_simple.dingo`):
```dingo
package main

func readConfig(path string) ([]byte, error) {
	data := ReadFile(path)?
	return data, nil
}
```

**Generated Go** (`error_prop_01_simple.go`):
```go
package main

func readConfig(path string) ([]byte, error) {
	__tmp0, __err0 := ReadFile(path)
	// dingo:s:1
	if __err0 != nil {
		return nil, __err0
	}
	// dingo:e:1
	var data = __tmp0
	return data, nil
}
```

**Source Map** (`error_prop_01_simple.go.map`):
```json
{
  "version": 1,
  "mappings": [
    {
      "generated_line": 4,
      "generated_column": 20,
      "original_line": 4,
      "original_column": 13,
      "length": 14,
      "name": "expr_mapping"
    },
    {
      "generated_line": 4,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 5,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 6,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 7,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 8,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 9,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 10,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    }
  ]
}
```

## Architecture Context

Dingo LSP is a proxy that wraps gopls:
1. User edits `.dingo` file in IDE
2. Dingo LSP translates positions to `.go` file positions
3. gopls analyzes the `.go` file
4. gopls returns diagnostics for `.go` file
5. Dingo LSP translates diagnostics back to `.dingo` positions
6. IDE displays error underlines in `.dingo` file

## Relevant Source Code

### SourceMap.MapToOriginal (sourcemap.go)

```go
// MapToOriginal maps a preprocessed position to the original Dingo position
// Returns the mapped position or the input position if no mapping found
func (sm *SourceMap) MapToOriginal(line, col int) (int, int) {
	// CRITICAL FIX C7: Use column information for disambiguation
	// When multiple mappings exist for same generated line, choose closest column
	var bestMatch *Mapping

	for i := range sm.Mappings {
		m := &sm.Mappings[i]
		if m.GeneratedLine == line {
			// Check if position is within this mapping's range
			if col >= m.GeneratedColumn && col < m.GeneratedColumn+m.Length {
				// Exact match within range
				offset := col - m.GeneratedColumn
				return m.OriginalLine, m.OriginalColumn + offset
			}

			// Track closest mapping for fallback
			if bestMatch == nil {
				bestMatch = m
			} else {
				// Closer column match wins
				currDist := abs(m.GeneratedColumn - col)
				bestDist := abs(bestMatch.GeneratedColumn - col)
				if currDist < bestDist {
					bestMatch = m
				}
			}
		}
	}

	// CRITICAL FIX: Smart fallback logic
	// If this line has mappings but position is outside them, AND the position
	// is far from any mapping (> 10 columns), use identity mapping.
	// This handles cases like "ReadFile" on same line as "?" operator.
	if bestMatch != nil {
		dist := abs(bestMatch.GeneratedColumn - col)
		if dist > 10 {
			// Position is far from mapped region - likely unchanged code
			// Use identity mapping instead
			return line, col
		}

		// Close to mapped region - use offset from best match
		offset := col - bestMatch.GeneratedColumn
		return bestMatch.OriginalLine, bestMatch.OriginalColumn + offset
	}

	// No mapping found at all for this line - use identity mapping
	return line, col
}
```

### Diagnostic Translation (handlers.go)

```go
// handlePublishDiagnostics processes diagnostics from gopls and translates to Dingo positions
// This is called when gopls sends diagnostics for .go files
func (s *Server) handlePublishDiagnostics(
	ctx context.Context,
	params protocol.PublishDiagnosticsParams,
) error {
	// Check if this is for a .go file that has a corresponding .dingo file
	goPath := params.URI.Filename()
	dingoPath := goToDingoPath(goPath)

	// If no .dingo file, ignore (this is a pure Go file)
	if dingoPath == goPath {
		return nil
	}

	// Translate diagnostics: Go positions → Dingo positions
	translatedDiagnostics, err := s.translator.TranslateDiagnostics(params.Diagnostics, params.URI, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("Diagnostic translation failed: %v", err)
		return nil
	}

	// Publish diagnostics for the .dingo file
	dingoURI := uri.File(dingoPath)
	translatedParams := protocol.PublishDiagnosticsParams{
		URI:         dingoURI,
		Diagnostics: translatedDiagnostics,
		Version:     params.Version,
	}

	s.config.Logger.Debugf("Publishing %d diagnostics for %s", len(translatedDiagnostics), dingoPath)

	// CRITICAL FIX C1: Actually publish to IDE connection (thread-safe)
	ideConn, serverCtx := s.GetConn()
	if ideConn != nil {
		// Use server context if available, otherwise use provided context
		publishCtx := serverCtx
		if publishCtx == nil {
			publishCtx = ctx
		}
		return ideConn.Notify(publishCtx, "textDocument/publishDiagnostics", translatedParams)
	}

	s.config.Logger.Warnf("No IDE connection available, cannot publish diagnostics")
	return nil
}
```

```go
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
```

## Your Task

**Analyze this bug and provide:**

1. **Root Cause**: Why is the wrong code getting underlined?
   - Consider: What position does gopls report for the error?
   - Consider: How does the source map translation work?
   - Consider: What's wrong with the mapping or translation logic?

2. **Detailed Analysis**:
   - Trace the expected flow of diagnostic translation
   - Identify where the logic breaks down
   - Explain why `e(path)?` gets highlighted instead of `ReadFile`

3. **Fix Design**:
   - Propose specific changes to fix the bug
   - Consider edge cases and alternative approaches
   - Provide pseudocode or detailed implementation guidance

4. **Validation Strategy**:
   - How to verify the fix works correctly
   - What test cases should be added

Think step-by-step through the position mapping logic and identify the exact failure point.
