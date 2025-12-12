# LSP Source Mapping Bug Investigation

## Problem Statement

The Dingo language server is underlining the WRONG part of code when reporting errors.

**Expected behavior:**
- Error in `ReadFile(path)` should underline `ReadFile`
- This is the actual function call that's undefined

**Actual behavior:**
- Error underlines `e(path)?` instead
- This is the error propagation operator, NOT the source of the error

**Affected code (error_prop_01_simple.dingo):**
```dingo
package main

func readConfig(path string) ([]byte, error) {
	data := ReadFile(path)?
	return data, nil
}
```

**After preprocessing (error_prop_01_simple.go):**
```go
package main

func readConfig(path string) ([]byte, error) {
	data, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}
```

## Architecture Overview

### Dingo LSP Proxy
- **Role**: Wraps gopls, translates positions between .dingo ↔ .go files
- **Key components**:
  1. `pkg/lsp/server.go` - Main server, handles LSP lifecycle
  2. `pkg/lsp/handlers.go` - Request/response handlers with translation
  3. `pkg/preprocessor/sourcemap.go` - Position mapping logic

### Transpilation Pipeline
1. **Preprocessor** transforms Dingo syntax → valid Go
2. **Source map** tracks position mappings
3. **LSP** uses source maps to translate positions

### Diagnostic Flow
```
gopls detects error in .go file (line 4, col X)
    ↓
handlePublishDiagnostics() receives diagnostic
    ↓
TranslateDiagnostics() maps Go position → Dingo position
    ↓
MapToOriginal() finds closest mapping
    ↓
IDE shows diagnostic at translated position
```

## Relevant Source Code

### 1. Source Mapping Logic (sourcemap.go)

```go
// MapToOriginal maps a preprocessed position to the original Dingo position
func (sm *SourceMap) MapToOriginal(line, col int) (int, int) {
	// CRITICAL FIX C7: Use column information for disambiguation
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

### 2. Diagnostic Translation (handlers.go)

```go
func (s *Server) handlePublishDiagnostics(
	ctx context.Context,
	params protocol.PublishDiagnosticsParams,
) error {
	goPath := params.URI.Filename()
	dingoPath := goToDingoPath(goPath)

	if dingoPath == goPath {
		return nil // Not a Dingo file
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

	ideConn, serverCtx := s.GetConn()
	if ideConn != nil {
		publishCtx := serverCtx
		if publishCtx == nil {
			publishCtx = ctx
		}
		return ideConn.Notify(publishCtx, "textDocument/publishDiagnostics", translatedParams)
	}

	return nil
}
```

```go
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

## Analysis Questions

Please provide a root cause analysis focusing on:

1. **Source Map Generation Issue?**
   - Is the preprocessor creating incorrect mappings?
   - What mappings should exist for `ReadFile(path)?` → expanded error handling?

2. **Position Translation Logic?**
   - Is `MapToOriginal()` selecting the wrong mapping?
   - The "closest column" logic might be choosing the `?` operator mapping instead of `ReadFile`

3. **Diagnostic Range Handling?**
   - gopls reports error on `ReadFile` in .go file
   - What's the exact position gopls reports?
   - How should that translate to the .dingo file?

4. **Multi-Line Expansion Issue?**
   - Dingo: `ReadFile(path)?` (single line)
   - Go: Multi-line if statement
   - Are mappings handling the expansion correctly?

## Expected Output

Please provide:

1. **Root Cause**: Specific bug in code (line numbers, function)
2. **Why This Happens**: Detailed explanation of the failure path
3. **Fix Design**: High-level approach to fix (not implementation)
4. **Test Strategy**: How to validate the fix works

## Context Notes

- This is a critical LSP bug affecting user experience
- Source maps were recently added (comments reference "CRITICAL FIX C7")
- The 10-column threshold in `MapToOriginal` seems arbitrary
- Error propagation `?` expands to 4 lines of Go code
