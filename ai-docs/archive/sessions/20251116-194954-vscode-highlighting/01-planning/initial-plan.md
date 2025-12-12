# VSCode Syntax Highlighting Enhancement - Architectural Plan

## Executive Summary

This document outlines a comprehensive approach to enhance VSCode syntax highlighting for the Dingo project, focusing on:
1. Improved `.dingo` file syntax highlighting (especially `?` operator and error messages)
2. Marking generated code sections in transpiler output
3. Visual highlighting of generated code markers in VSCode
4. Support for `.go.golden` test files

## 1. Current State Analysis

### 1.1 Existing VSCode Extension

**Location:** `/Users/jack/mag/dingo/editors/vscode/`

**Current Capabilities:**
- Basic `.dingo` syntax highlighting via TextMate grammar
- Support for keywords, types, operators, comments, strings
- Pattern matching syntax
- Result/Option type highlighting
- Lambda expressions with `|param|` syntax

**Existing TextMate Grammar Coverage:**
```yaml
- Comments (line and block)
- Keywords (control flow, declarations)
- Result[T,E] and Option[T] types
- Operators including ? (error propagation)
- Enum variants and pattern matching
- Functions and lambdas
- Numeric types, strings, constants
- Attributes (#[...])
```

**Current Limitations:**
1. The `?` operator is highlighted but has no special visual distinction
2. Error message syntax `expr? "message"` is not specifically highlighted
3. No highlighting for `.go.golden` files
4. No mechanism to visualize transpiler-generated code

### 1.2 Transpiler Current State

**Generator:** `/Users/jack/mag/dingo/pkg/generator/generator.go`

**Current Behavior:**
- Uses plugin architecture for AST transformations
- Error propagation plugin generates code like:
  ```go
  data, __err0 := ReadFile(path)
  if __err0 != nil {
      return nil, __err0
  }
  ```
- No markers or comments indicating generated vs original code
- Uses standard Go formatting (go/format, go/printer)

**Observed Generated Patterns:**
- Error variables: `__err0`, `__err1`, `__err2`, etc.
- Temp variables: `__tmp0`, `__tmp1`, etc.
- Error checking blocks: `if __errN != nil { return ... }`
- Error wrapping: `fmt.Errorf("message: %w", __errN)`

### 1.3 Golden Test Files

**Location:** `/Users/jack/mag/dingo/tests/golden/`

**Structure:**
- `.dingo` files: Original Dingo source
- `.go.golden` files: Expected transpiled Go output
- Pairs like:
  - `01_simple_statement.dingo` → `01_simple_statement.go.golden`
  - `04_error_wrapping.dingo` → `04_error_wrapping.go.golden`

**Current Status:**
- No special VSCode handling for `.go.golden` files
- Treated as regular Go files with standard Go highlighting

## 2. Proposed Architecture

### 2.1 Three-Component Solution

We propose a **marker-based approach** with three integrated components:

#### Component 1: Transpiler Marker Generation
- Add special comments to mark generated code sections
- Lightweight, non-invasive to Go output
- Compatible with all Go tools

#### Component 2: Enhanced .dingo Syntax Highlighting
- Improve TextMate grammar for better visual distinction
- Special highlighting for `?` operator and error messages
- Better semantic scoping

#### Component 3: VSCode Extension for Generated Code Highlighting
- Detect marker comments in `.go` and `.go.golden` files
- Apply visual decorations to generated code sections
- Configurable, non-intrusive styling

### 2.2 Marker Format Design

#### Option A: Line-Level Markers (RECOMMENDED)
```go
data, __err0 := ReadFile(path) // DINGO:GENERATED
if __err0 != nil {             // DINGO:GENERATED
    return nil, __err0         // DINGO:GENERATED
}                              // DINGO:GENERATED
```

**Pros:**
- Fine-grained control
- Easy to implement in generator
- No block-level logic needed

**Cons:**
- More verbose (4 comments vs 2)
- Slightly clutters output

#### Option B: Block-Level Markers
```go
// DINGO:GENERATED:START
data, __err0 := ReadFile(path)
if __err0 != nil {
    return nil, __err0
}
// DINGO:GENERATED:END
```

**Pros:**
- Cleaner for multi-line generated blocks
- Less visual clutter

**Cons:**
- Requires tracking block boundaries
- More complex generator logic
- Mixed generated/original code harder to handle

#### Option C: Hybrid Approach (RECOMMENDED FINAL)
Use block markers for multi-statement generated code, line markers for single expressions:

```go
// DINGO:GENERATED:START error propagation for ReadFile(path)?
data, __err0 := ReadFile(path)
if __err0 != nil {
    return nil, fmt.Errorf("failed to read user config: %w", __err0)
}
// DINGO:GENERATED:END

// Original user code continues...
return data, nil
```

**Pros:**
- Best of both worlds
- Self-documenting (includes context)
- Minimal clutter for simple cases

### 2.3 Marker Taxonomy

Define different marker types for different generated code:

```go
// DINGO:GENERATED:START error_propagation
// DINGO:GENERATED:START result_constructor
// DINGO:GENERATED:START option_unwrap
// DINGO:GENERATED:START pattern_match
// DINGO:GENERATED:START lambda_expansion
// DINGO:GENERATED:END
```

This allows future type-specific styling (e.g., error propagation in blue, pattern matching in purple).

## 3. Implementation Strategy

### 3.1 Phase 1: Transpiler Marker Generation

**Goal:** Modify generator to emit marker comments

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- `/Users/jack/mag/dingo/pkg/generator/generator.go`

**Implementation Approach:**

1. **Add Marker Configuration:**
```go
// pkg/config/config.go
type GeneratorConfig struct {
    EmitGeneratedMarkers bool   // Enable/disable markers
    MarkerStyle         string  // "line", "block", "hybrid"
    IncludeContext      bool    // Add context info to markers
}
```

2. **Modify Error Propagation Plugin:**
```go
// In transformStatementContext()
func (p *ErrorPropagationPlugin) transformStatementContext(...) {
    // After generating error check statements
    errorCheck := &ast.IfStmt{...}

    // Wrap with marker comments
    if p.config.EmitGeneratedMarkers {
        startMarker := &ast.Comment{
            Text: "// DINGO:GENERATED:START error_propagation",
        }
        endMarker := &ast.Comment{
            Text: "// DINGO:GENERATED:END",
        }

        // Attach comments to AST nodes
        p.attachComment(assignStmt, startMarker)
        p.attachComment(errorCheck, endMarker)
    }
}
```

3. **Helper Functions:**
```go
// Utility to inject comments into AST
func (g *Generator) injectMarkerComments(node ast.Node, markers ...string) {
    // Use token.FileSet and ast.CommentMap to attach comments
    // Ensure comments appear in correct positions during printing
}
```

**Testing:**
- Update golden files to include markers
- Verify markers don't affect Go compilation
- Ensure markers survive `go fmt`

### 3.2 Phase 2: Enhanced .dingo Syntax Highlighting

**Goal:** Improve visual distinction for Dingo-specific syntax

**Files to Modify:**
- `/Users/jack/mag/dingo/editors/vscode/syntaxes/dingo.tmLanguage.yaml`

**Improvements:**

1. **Enhanced Error Propagation Operator:**
```yaml
# CURRENT (line 159-162)
- name: keyword.operator.error-propagation.dingo
  match: \?
  comment: Error propagation operator

# ENHANCED
operators:
  patterns:
    # Error propagation with message: expr? "message"
    - name: meta.error-propagation.with-message.dingo
      begin: \?
      beginCaptures:
        0: { name: keyword.operator.error-propagation.dingo }
      end: (?=;|,|\)|})
      patterns:
        - include: '#strings'
        - name: string.quoted.double.error-message.dingo
          begin: "\""
          end: "\""
          beginCaptures:
            0: { name: punctuation.definition.error-message.begin.dingo }
          endCaptures:
            0: { name: punctuation.definition.error-message.end.dingo }

    # Standalone error propagation: expr?
    - name: keyword.operator.error-propagation.standalone.dingo
      match: \?(?!\")
```

2. **Add Scope for Generated Variable Patterns:**
```yaml
# Highlight __err0, __tmp0 style variables (helpful for understanding transpiled code)
variables:
  patterns:
    - name: variable.other.generated.error.dingo
      match: \b__err\d+\b

    - name: variable.other.generated.temp.dingo
      match: \b__tmp\d+\b
```

3. **Enhanced Result/Option Highlighting:**
```yaml
result-type:
  patterns:
    # Result type with generics
    - name: meta.type.result.dingo
      begin: \b(Result)\s*(<)
      beginCaptures:
        1: { name: support.type.result.dingo }
        2: { name: punctuation.definition.typeparameters.begin.dingo }
      end: (>)
      endCaptures:
        1: { name: punctuation.definition.typeparameters.end.dingo }
      patterns:
        - include: '#types'
```

### 3.3 Phase 3: VSCode Extension Enhancement

**Goal:** Create TypeScript-based extension to highlight generated code

**Files to Create:**
- `/Users/jack/mag/dingo/editors/vscode/src/extension.ts`
- `/Users/jack/mag/dingo/editors/vscode/src/generatedCodeHighlighter.ts`
- `/Users/jack/mag/dingo/editors/vscode/src/goldenFileSupport.ts`

**Current Package.json Structure:**
```json
{
  "name": "dingo",
  "contributes": {
    "languages": [...],
    "grammars": [...]
  }
}
```

**Enhanced Package.json:**
```json
{
  "name": "dingo",
  "main": "./out/extension.js",
  "activationEvents": [
    "onLanguage:dingo",
    "onLanguage:go"
  ],
  "contributes": {
    "languages": [
      {
        "id": "dingo",
        "extensions": [".dingo"],
        "configuration": "./language-configuration.json"
      },
      {
        "id": "go-golden",
        "aliases": ["Go Golden Test"],
        "extensions": [".go.golden"],
        "configuration": "./language-configuration.json"
      }
    ],
    "grammars": [
      {
        "language": "dingo",
        "scopeName": "source.dingo",
        "path": "./syntaxes/dingo.tmLanguage.json"
      },
      {
        "language": "go-golden",
        "scopeName": "source.go",
        "path": "./syntaxes/go.tmLanguage.json"
      }
    ],
    "configuration": {
      "title": "Dingo",
      "properties": {
        "dingo.highlightGeneratedCode": {
          "type": "boolean",
          "default": true,
          "description": "Highlight generated code sections in transpiled .go files"
        },
        "dingo.generatedCodeStyle": {
          "type": "string",
          "enum": ["subtle", "bold", "outline"],
          "default": "subtle",
          "description": "Visual style for generated code highlighting"
        },
        "dingo.generatedCodeColor": {
          "type": "string",
          "default": "#3b82f620",
          "description": "Background color for generated code (with alpha)"
        }
      }
    }
  }
}
```

**Extension Architecture:**

```typescript
// src/extension.ts
import * as vscode from 'vscode';
import { GeneratedCodeHighlighter } from './generatedCodeHighlighter';
import { GoldenFileSupport } from './goldenFileSupport';

export function activate(context: vscode.ExtensionContext) {
    console.log('Dingo extension activated');

    // Initialize highlighter
    const highlighter = new GeneratedCodeHighlighter();

    // Register for Go files (including .go.golden)
    context.subscriptions.push(
        vscode.workspace.onDidOpenTextDocument(doc => {
            if (doc.languageId === 'go' || doc.fileName.endsWith('.go.golden')) {
                highlighter.updateHighlights(doc);
            }
        })
    );

    context.subscriptions.push(
        vscode.workspace.onDidChangeTextDocument(event => {
            highlighter.updateHighlights(event.document);
        })
    );

    // Golden file support
    const goldenSupport = new GoldenFileSupport();
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.compareWithSource', () => {
            goldenSupport.compareWithSource();
        })
    );
}
```

```typescript
// src/generatedCodeHighlighter.ts
import * as vscode from 'vscode';

interface MarkerRange {
    range: vscode.Range;
    type: string; // 'error_propagation', 'pattern_match', etc.
}

export class GeneratedCodeHighlighter {
    private decorationType: vscode.TextEditorDecorationType;
    private markerPattern = /\/\/\s*DINGO:GENERATED:(\w+)(?:\s+(.+))?$/;
    private blockStartPattern = /\/\/\s*DINGO:GENERATED:START(?:\s+(\w+))?/;
    private blockEndPattern = /\/\/\s*DINGO:GENERATED:END/;

    constructor() {
        this.decorationType = this.createDecorationType();

        // Listen for configuration changes
        vscode.workspace.onDidChangeConfiguration(e => {
            if (e.affectsConfiguration('dingo.generatedCode')) {
                this.decorationType.dispose();
                this.decorationType = this.createDecorationType();
                this.refreshAllEditors();
            }
        });
    }

    private createDecorationType(): vscode.TextEditorDecorationType {
        const config = vscode.workspace.getConfiguration('dingo');
        const style = config.get<string>('generatedCodeStyle', 'subtle');
        const color = config.get<string>('generatedCodeColor', '#3b82f620');

        const decorationOptions: vscode.DecorationRenderOptions = {
            isWholeLine: true,
        };

        switch (style) {
            case 'bold':
                decorationOptions.backgroundColor = color;
                decorationOptions.border = '1px solid #3b82f640';
                break;
            case 'outline':
                decorationOptions.border = '1px solid #3b82f680';
                break;
            case 'subtle':
            default:
                decorationOptions.backgroundColor = color;
                break;
        }

        return vscode.window.createTextEditorDecorationType(decorationOptions);
    }

    public updateHighlights(document: vscode.TextDocument) {
        const config = vscode.workspace.getConfiguration('dingo');
        if (!config.get<boolean>('highlightGeneratedCode', true)) {
            return;
        }

        const markers = this.findGeneratedMarkers(document);

        // Apply decorations to all visible editors
        vscode.window.visibleTextEditors.forEach(editor => {
            if (editor.document === document) {
                editor.setDecorations(this.decorationType, markers.map(m => m.range));
            }
        });
    }

    private findGeneratedMarkers(document: vscode.TextDocument): MarkerRange[] {
        const markers: MarkerRange[] = [];
        inBlock := false;
        let blockStart: number | null = null;
        blockType := 'unknown';

        for (i := 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const text = line.text;

            // Check for block start
            const blockStartMatch = text.match(this.blockStartPattern);
            if (blockStartMatch) {
                inBlock = true;
                blockStart = i;
                blockType = blockStartMatch[1] || 'unknown';
                continue;
            }

            // Check for block end
            if (inBlock && text.match(this.blockEndPattern)) {
                if (blockStart !== null) {
                    // Add all lines from blockStart to current line
                    for (j := blockStart; j <= i; j++) {
                        markers.push({
                            range: document.lineAt(j).range,
                            type: blockType
                        });
                    }
                }
                inBlock = false;
                blockStart = null;
                continue;
            }

            // Check for inline marker
            const inlineMatch = text.match(this.markerPattern);
            if (inlineMatch) {
                markers.push({
                    range: line.range,
                    type: inlineMatch[1]
                });
            }
        }

        return markers;
    }

    private refreshAllEditors() {
        vscode.window.visibleTextEditors.forEach(editor => {
            this.updateHighlights(editor.document);
        });
    }

    public dispose() {
        this.decorationType.dispose();
    }
}
```

```typescript
// src/goldenFileSupport.ts
import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';

export class GoldenFileSupport {
    public async compareWithSource() {
        const activeEditor = vscode.window.activeTextEditor;
        if (!activeEditor) {
            return;
        }

        const currentPath = activeEditor.document.fileName;
        let sourcePath: string;
        let goldenPath: string;

        // Determine if we're in .dingo or .go.golden file
        if (currentPath.endsWith('.dingo')) {
            sourcePath = currentPath;
            goldenPath = currentPath.replace('.dingo', '.go.golden');
        } else if (currentPath.endsWith('.go.golden')) {
            goldenPath = currentPath;
            sourcePath = currentPath.replace('.go.golden', '.dingo');
        } else {
            vscode.window.showErrorMessage('Not a Dingo or golden file');
            return;
        }

        // Check if both files exist
        if (!fs.existsSync(sourcePath) || !fs.existsSync(goldenPath)) {
            vscode.window.showErrorMessage('Could not find matching source/golden file');
            return;
        }

        // Open side-by-side
        const sourceUri = vscode.Uri.file(sourcePath);
        const goldenUri = vscode.Uri.file(goldenPath);

        await vscode.commands.executeCommand('vscode.diff',
            sourceUri,
            goldenUri,
            `${path.basename(sourcePath)} ↔ ${path.basename(goldenPath)}`
        );
    }
}
```

### 3.4 Phase 4: Golden File Support

**Goal:** Treat `.go.golden` files specially for better UX

**Implementation:**

1. **Add Language Association:**
```json
// package.json
"languages": [
  {
    "id": "go-golden",
    "aliases": ["Go Golden Test", "go-golden"],
    "extensions": [".go.golden"],
    "configuration": "./language-configuration.json",
    "icon": {
      "light": "./icons/golden-light.png",
      "dark": "./icons/golden-dark.png"
    }
  }
]
```

2. **Add Commands:**
```json
"commands": [
  {
    "command": "dingo.compareWithSource",
    "title": "Dingo: Compare with Source File",
    "icon": "$(diff)"
  },
  {
    "command": "dingo.openCorrespondingFile",
    "title": "Dingo: Open Corresponding .dingo/.go.golden File"
  }
]
```

3. **Add Keybindings:**
```json
"keybindings": [
  {
    "command": "dingo.compareWithSource",
    "key": "ctrl+shift+d",
    "mac": "cmd+shift+d",
    "when": "resourceExtname == .dingo || resourceExtname == .go.golden"
  }
]
```

## 4. Visual Design Mockups

### 4.1 .dingo File Highlighting

**Before (Current):**
```dingo
data := ReadFile(path)? "failed to read user config"
           ^(basic operator color)
```

**After (Enhanced):**
```dingo
data := ReadFile(path)? "failed to read user config"
           ^(bright keyword) ^(special error message color)
```

Color Scheme:
- `?` operator: Bright keyword color (e.g., purple/magenta)
- Error message string: Special scope (e.g., warning yellow)
- Generated variables (`__err0`): Muted/gray color

### 4.2 Generated Code Highlighting

**Subtle Style (Default):**
```go
func readUserConfig(username string) ([]byte, error) {
    path := "/home/" + username + "/config.json"
┌──────────────────────────────────────────────────────────┐
│ data, __err0 := ReadFile(path)                          │ ← Light blue background
│ if __err0 != nil {                                       │
│     return nil, fmt.Errorf("failed to read...: %w", __err0) │
│ }                                                        │
└──────────────────────────────────────────────────────────┘
    return data, nil
}
```

**Bold Style:**
```go
func readUserConfig(username string) ([]byte, error) {
    path := "/home/" + username + "/config.json"
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ data, __err0 := ReadFile(path)                          ┃ ← Blue bg + border
┃ if __err0 != nil {                                       ┃
┃     return nil, fmt.Errorf("failed to read...: %w", __err0) ┃
┃ }                                                        ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
    return data, nil
}
```

**Outline Style:**
```go
func readUserConfig(username string) ([]byte, error) {
    path := "/home/" + username + "/config.json"
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
  data, __err0 := ReadFile(path)                            ← Border only
  if __err0 != nil {
      return nil, fmt.Errorf("failed to read...: %w", __err0)
  }
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
    return data, nil
}
```

### 4.3 Color Palette

**Light Theme:**
- Generated code background: `#3b82f615` (blue, 15% opacity)
- Generated code border: `#3b82f640` (blue, 40% opacity)
- Error operator: `#9333ea` (purple)
- Error message: `#ea580c` (orange)

**Dark Theme:**
- Generated code background: `#3b82f625` (blue, 25% opacity)
- Generated code border: `#3b82f660` (blue, 60% opacity)
- Error operator: `#c084fc` (light purple)
- Error message: `#fb923c` (light orange)

## 5. Testing Strategy

### 5.1 Transpiler Marker Tests

**Test Files:**
- Update existing golden files to include markers
- Add new test: `09_marker_generation.dingo` / `09_marker_generation.go.golden`

**Test Cases:**
1. Simple error propagation generates markers
2. Multiple error propagations get unique markers
3. Error wrapping with messages includes context
4. Markers survive `go fmt` and `gofmt`
5. Markers don't affect compilation or execution
6. Markers can be disabled via config

**Example Test:**
```go
// tests/golden/09_marker_generation.go.golden
package main

func readConfig(path string) ([]byte, error) {
    // DINGO:GENERATED:START error_propagation for ReadFile(path)?
    data, __err0 := ReadFile(path)
    if __err0 != nil {
        return nil, __err0
    }
    // DINGO:GENERATED:END
    return data, nil
}
```

### 5.2 VSCode Extension Tests

**Unit Tests:**
```typescript
// test/suite/generatedCodeHighlighter.test.ts
describe('GeneratedCodeHighlighter', () => {
    it('should detect block markers', () => {
        const document = createTestDocument(`
            // DINGO:GENERATED:START error_propagation
            data, __err0 := ReadFile(path)
            if __err0 != nil {
                return nil, __err0
            }
            // DINGO:GENERATED:END
        `);

        const highlighter = new GeneratedCodeHighlighter();
        const markers = highlighter.findGeneratedMarkers(document);

        expect(markers).toHaveLength(5); // All 5 lines
        expect(markers[0].type).toBe('error_propagation');
    });

    it('should detect inline markers', () => {
        const document = createTestDocument(`
            data, __err0 := ReadFile(path) // DINGO:GENERATED
        `);

        const highlighter = new GeneratedCodeHighlighter();
        const markers = highlighter.findGeneratedMarkers(document);

        expect(markers).toHaveLength(1);
    });
});
```

**Manual Tests:**
1. Open `.dingo` file → verify enhanced highlighting
2. Open `.go.golden` file → verify generated code highlighting
3. Toggle highlighting setting → verify decorations update
4. Change color scheme → verify colors adapt
5. Run `dingo.compareWithSource` → verify side-by-side diff opens

### 5.3 Integration Tests

**Scenarios:**
1. Transpile `.dingo` → verify `.go` has markers
2. Open transpiled `.go` in VSCode → verify highlighting appears
3. Modify `.dingo` → re-transpile → verify markers update
4. Format generated `.go` → verify markers persist
5. Use generated code in another project → verify it compiles

## 6. Implementation Roadmap

### Week 1: Transpiler Marker Foundation
- [ ] Add configuration for marker generation
- [ ] Implement marker injection in error_propagation.go
- [ ] Add helper functions for comment attachment
- [ ] Update golden files with markers
- [ ] Write unit tests for marker generation

### Week 2: VSCode Extension Setup
- [ ] Initialize TypeScript project in editors/vscode/src/
- [ ] Set up build pipeline (tsc, esbuild)
- [ ] Implement GeneratedCodeHighlighter class
- [ ] Add configuration properties
- [ ] Write unit tests for marker detection

### Week 3: Golden File Support
- [ ] Add .go.golden language definition
- [ ] Implement compareWithSource command
- [ ] Add keybindings and commands
- [ ] Create golden file icons
- [ ] Write integration tests

### Week 4: Enhanced .dingo Syntax
- [ ] Improve ? operator highlighting
- [ ] Add error message special scope
- [ ] Add generated variable patterns
- [ ] Test with various color themes
- [ ] Update documentation

### Week 5: Polish and Testing
- [ ] End-to-end testing with real projects
- [ ] Performance optimization for large files
- [ ] Documentation and examples
- [ ] Package and publish extension
- [ ] Gather user feedback

## 7. Alternative Approaches Considered

### Alternative 1: Source Map-Based Highlighting

**Approach:** Use source maps to determine which Go code corresponds to Dingo code, highlight the rest as generated.

**Pros:**
- No comment markers needed
- Perfectly accurate
- Can show exact Dingo→Go correspondence

**Cons:**
- Requires parsing source maps in VSCode
- More complex implementation
- Doesn't work for manually viewing .go files without .map files
- Harder to debug

**Verdict:** Rejected for initial implementation. Consider for future enhancement.

### Alternative 2: Semantic Highlighting with Language Server

**Approach:** Implement custom LSP server that provides semantic tokens for generated code.

**Pros:**
- Professional, integrated approach
- Works with any LSP-capable editor
- More powerful (can provide hover info, etc.)

**Cons:**
- Much more complex implementation
- Requires running Dingo LSP server
- Overkill for simple highlighting

**Verdict:** Rejected. Reserve LSP work for full IDE support later.

### Alternative 3: AST-Based Detection (No Markers)

**Approach:** VSCode extension parses Go code, detects patterns like `__err0`, infers generated sections.

**Pros:**
- Works on existing code without re-transpiling
- No changes to transpiler needed

**Cons:**
- Fragile (heuristic-based)
- Won't detect all generated code
- False positives possible
- Requires full Go parser in extension

**Verdict:** Rejected. Too unreliable.

### Alternative 4: Separate .dingo.go Extension

**Approach:** Transpile to `.dingo.go` instead of `.go`, use extension to indicate transpiled file.

**Pros:**
- Clear file type distinction
- Easy to add special handling

**Cons:**
- Breaks Go tooling (go build, gopls, etc.)
- Non-standard file extension
- Poor ecosystem compatibility

**Verdict:** Rejected. Violates core compatibility principle.

## 8. Future Enhancements

### Post-MVP Features

1. **Type-Specific Styling:**
   - Different colors for error propagation, pattern matching, lambda expansion
   - Configurable per-type color schemes

2. **Hover Information:**
   - Show original Dingo code on hover over generated section
   - Link to source map position

3. **Code Folding:**
   - Automatically fold generated code sections
   - Toggle command to show/hide all generated code

4. **Source Map Integration:**
   - Use source maps for precise highlighting
   - Navigate from Go back to Dingo source

5. **Generated Code Statistics:**
   - Show metrics: X% generated, Y% original
   - CodeLens annotations with generation details

6. **Multi-File Diff View:**
   - Compare entire directories: dingo/ vs generated/
   - Batch view all .dingo ↔ .go.golden pairs

7. **Customizable Markers:**
   - User-defined marker format
   - Plugin system for custom generators

## 9. Documentation Requirements

### User-Facing Docs

1. **Installation Guide:**
   - How to install VSCode extension
   - How to configure highlighting preferences

2. **Feature Guide:**
   - What generated code highlighting looks like
   - How to use golden file comparison
   - Keyboard shortcuts

3. **Configuration Reference:**
   - All settings explained
   - Color customization examples

### Developer Docs

1. **Transpiler Marker API:**
   - How to add markers in new plugins
   - Marker format specification

2. **Extension Architecture:**
   - How highlighting works
   - How to extend with new marker types

## 10. Success Metrics

### Transpiler
- [ ] Markers correctly generated for all error propagation cases
- [ ] Markers don't affect compilation or formatting
- [ ] Golden tests pass with markers

### VSCode Extension
- [ ] Highlighting appears within 100ms of opening file
- [ ] Works with light and dark themes
- [ ] Configurable via settings
- [ ] No noticeable performance impact on large files (>1000 LOC)

### User Experience
- [ ] Generated code visually distinct but not distracting
- [ ] Golden file comparison works seamlessly
- [ ] Users can easily toggle highlighting on/off
- [ ] Works alongside Go extension without conflicts

## 11. Risk Assessment

### Technical Risks

**Risk:** Markers interfere with Go tooling
- **Mitigation:** Use standard comment syntax, test with all major tools
- **Severity:** High
- **Likelihood:** Low

**Risk:** VSCode extension performance issues
- **Mitigation:** Debounce updates, use efficient regex patterns
- **Severity:** Medium
- **Likelihood:** Medium

**Risk:** Color scheme incompatibility
- **Mitigation:** Use semi-transparent colors, provide presets
- **Severity:** Low
- **Likelihood:** Medium

### UX Risks

**Risk:** Users find highlighting distracting
- **Mitigation:** Default to subtle style, make easily disableable
- **Severity:** Medium
- **Likelihood:** Low

**Risk:** Confusion about what's generated vs original
- **Mitigation:** Clear documentation, hover tooltips
- **Severity:** Low
- **Likelihood:** Low

## 12. Conclusion

This architecture provides a robust, extensible approach to enhancing Dingo's VSCode experience. The marker-based approach balances simplicity with functionality, while the modular design allows for future enhancements without major refactoring.

The implementation is pragmatic, leveraging existing VSCode APIs and standard Go comment syntax to achieve visual distinction without breaking ecosystem compatibility.

**Recommended Starting Point:** Phase 1 (Transpiler Markers) to validate the approach, then Phase 2 (VSCode Extension) to deliver immediate user value.
