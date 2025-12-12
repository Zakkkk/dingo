# Dingo LSP Source Mapping Bug Investigation

## Problem Statement

The Dingo language server is underlining the WRONG part of code when there's an error.

**Expected behavior:**
- Should underline `ReadFile` when there's an undefined identifier error

**Actual behavior:**
- Underlines `e(path)?` instead (the error propagation operator)

## Context: Dingo Transpiler Architecture

Dingo is a meta-language for Go (like TypeScript for JavaScript) that transpiles `.dingo` files to `.go` files.

**Key features:**
- `?` operator for error propagation
- `let` keyword for variable declarations
- Type annotations with `:` syntax
- Source maps for LSP position translation

**Transpilation example:**

**Input (error_prop_01_simple.dingo):**
```dingo
package main

func readConfig(path string) ([]byte, error) {
	data := ReadFile(path)?
	return data, nil
}
```

**Output (error_prop_01_simple.go):**
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

**Source Map (error_prop_01_simple.go.map):**
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
    ... (lines 6-10 also map to original line 4, col 15)
  ]
}
```

## LSP Architecture

The Dingo LSP server wraps gopls as a proxy:

1. **IDE → Dingo LSP**: Requests with `.dingo` file positions
2. **Dingo LSP**: Translates positions (Dingo → Go) using source maps
3. **Dingo LSP → gopls**: Forwards request with `.go` file positions
4. **gopls → Dingo LSP**: Returns diagnostics/responses with `.go` positions
5. **Dingo LSP**: Translates positions (Go → Dingo) using source maps
6. **Dingo LSP → IDE**: Returns diagnostics/responses with `.dingo` positions

## Source Code

### sourcemap.go (Position Translation Logic)

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

### handlers.go (Diagnostic Translation)

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

// handlePublishDiagnostics processes diagnostics from gopls and translates to Dingo positions
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

	// Publish to IDE connection
	ideConn, serverCtx := s.GetConn()
	if ideConn != nil {
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

## The Bug Scenario

When `ReadFile` is undefined (not imported), gopls reports:

**Go file (line 4):** `__tmp0, __err0 := ReadFile(path)`
- Error position: line 4, column 20 (start of `ReadFile`)

**Expected Dingo translation:**
- Should map to: line 4, column 13 (start of `ReadFile` in Dingo)
- Source map has mapping: `generated_line: 4, generated_column: 20 → original_line: 4, original_column: 13, length: 14`

**Actual Dingo translation:**
- Maps to: line 4, column 15 (the `?` operator instead of `ReadFile`)

## Investigation Questions

1. **Root Cause Analysis:**
   - Why is the position mapping choosing column 15 (the `?` operator) instead of column 13 (`ReadFile`)?
   - Is the issue in the source map generation or the lookup algorithm?
   - Is the "closest column match" logic in `MapToOriginal` flawed?

2. **Source Map Structure:**
   - Is the source map correct? Should there be a separate mapping for `ReadFile` itself?
   - Why do lines 5-10 all map to the same original position (line 4, col 15)?
   - Is the `expr_mapping` entry (line 4, col 20 → line 4, col 13) being selected correctly?

3. **Algorithm Analysis:**
   - When gopls reports an error at Go line 4, column 20, what happens in `MapToOriginal`?
   - Does the "exact match within range" logic (lines 60-64) trigger?
   - Does the "closest column match" fallback (lines 68-76) select the wrong mapping?

4. **Fix Design:**
   - What changes are needed to ensure `ReadFile` errors underline `ReadFile`, not `?`?
   - Should we prioritize mappings by type (prefer `expr_mapping` over `error_prop`)?
   - Should we add more granular mappings for function calls?

## Expected Analysis Deliverables

1. **Root cause identification:** Exact reason why column 15 is chosen over column 13
2. **Algorithm trace:** Step-by-step execution of `MapToOriginal(4, 20)` with current source map
3. **Fix recommendation:** Specific code changes to resolve the issue
4. **Test cases:** What edge cases should be tested after the fix?

## Additional Context

- The `?` operator expands to 7 lines of Go code (lines 4-10)
- All generated lines map back to the same original position (line 4, col 15)
- The `ReadFile(path)` expression itself has a mapping (line 4, col 20 → line 4, col 13)
- The bug manifests when gopls reports errors at positions on the generated line 4

Please provide a deep technical analysis with specific recommendations for fixing this source mapping bug.
