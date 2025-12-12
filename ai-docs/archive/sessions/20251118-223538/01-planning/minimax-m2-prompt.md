# LSP Source Mapping Bug Investigation

You are an expert at debugging language servers and source map implementations. Analyze this Dingo LSP source mapping bug and provide root cause analysis with a fix design.

## Problem Description

The Dingo language server is underlining the WRONG part of code when there's an error.

**Expected behavior:**
- Should underline `ReadFile` when there's an error in that function call

**Actual behavior:**
- Underlines `e(path)?` instead (specifically the `?` operator position)

**Affected file:** `error_prop_01_simple.dingo`

## Source Code Context

### Original Dingo Source (`error_prop_01_simple.dingo`)
```dingo
package main

func readConfig(path string) ([]byte, error) {
	data := ReadFile(path)?
	return data, nil
}
```

**Line 4 breakdown:**
- `data := ` - Columns 1-12
- `ReadFile(path)` - Columns 13-27 (the function call that has the error)
- `?` - Column 28 (error propagation operator)

### Transpiled Go Output (`error_prop_01_simple.go`)
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

**Generated code breakdown:**
- Line 4: `__tmp0, __err0 := ReadFile(path)` - The actual function call
- Lines 5-9: Error handling boilerplate (generated from `?` operator)
- Line 10: Variable assignment (`var data = __tmp0`)

### Source Map (`error_prop_01_simple.go.map`)
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

**Key observations about the source map:**
1. First mapping (expr_mapping): Maps `ReadFile(path)` correctly
   - Generated: Line 4, Col 20 (length 14)
   - Original: Line 4, Col 13 (length 14)

2. All other mappings (error_prop): Map error handling code to `?` operator
   - All point to Original: Line 4, Col 15 (the `?` position)
   - Problem: Col 15 is WRONG - the `?` is at Col 28!

### Source Map Translation Logic (`pkg/preprocessor/sourcemap.go`)

The `MapToOriginal` function (lines 49-99) uses this algorithm:

```go
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

### LSP Diagnostic Translation (`pkg/lsp/handlers.go`)

The `handlePublishDiagnostics` function (lines 300-345) translates gopls diagnostics:

```go
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

The `TranslateDiagnostics` function (from handlers.go lines 97-133):

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

## The Bug Scenario

When gopls reports an error for `ReadFile` (undefined function):

1. **gopls error**: Points to `ReadFile` in the `.go` file (Line 4, around column 20)
2. **Translation**: LSP calls `MapToOriginal(4, 20)`
3. **Source map lookup**: Finds mapping at Line 4, Col 20 (the expr_mapping)
4. **Expected result**: Should map to Line 4, Col 13 (`ReadFile` in .dingo file)
5. **Actual result**: Maps to Line 4, Col 15 (the `?` position - WRONG!)

## Your Task

Analyze this bug and provide:

1. **Root Cause Analysis**
   - What exactly is causing the wrong position to be underlined?
   - Is it the source map generation that's wrong?
   - Is it the MapToOriginal translation logic?
   - Is it both?

2. **Source Map Issues**
   - The `error_prop` mappings claim Original Col 15, but `?` is at Col 28
   - Why is Col 15 incorrect?
   - Should error handling code map to the `?` position at all?

3. **Translation Logic Issues**
   - Does the "closest column match" fallback work correctly?
   - Does the "> 10 columns" distance check make sense?
   - When gopls reports error at Col 20, which mapping should win?

4. **Fix Design**
   - What should the source map mappings look like?
   - What changes (if any) to MapToOriginal logic?
   - How to ensure `ReadFile` errors underline `ReadFile`, not `?`?

5. **Implementation Priority**
   - Should we fix source map generation first?
   - Should we fix translation logic first?
   - Or both simultaneously?

Please provide a detailed analysis with concrete recommendations for fixing this bug.
