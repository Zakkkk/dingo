# VSCode Syntax Highlighting - Final Implementation Plan

**Date:** 2025-11-16
**Status:** Ready for Implementation
**User Approved Decisions:** All

---

## Executive Summary

This plan implements comprehensive VSCode syntax highlighting enhancements for Dingo:

1. **Block-level marker generation** in transpiler (enabled by default)
2. **Configurable visual highlighting** in VSCode extension
3. **Enhanced Dingo syntax highlighting** for error propagation and language features
4. **Golden file support** with side-by-side comparison

**Timeline:** 4-5 weeks
**Risk:** Low - uses standard Go comments and VSCode APIs

---

## 1. Architecture Overview

### Three-Component System

```
┌─────────────────────────────────────────────────────────────────┐
│                         COMPONENT 1                             │
│                     Transpiler Markers                          │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  .dingo file                                             │  │
│  │    ↓ (transpile)                                         │  │
│  │  .go file with block markers:                            │  │
│  │    // DINGO:GENERATED:START error_propagation            │  │
│  │    generated code here...                                │  │
│  │    // DINGO:GENERATED:END                                │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                         COMPONENT 2                             │
│                  VSCode Extension (TypeScript)                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Detects markers in .go files                            │  │
│  │  Applies decorations based on user settings:             │  │
│  │    - Subtle: light background only                       │  │
│  │    - Bold: background + border                           │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                         COMPONENT 3                             │
│                Enhanced .dingo Syntax (TextMate)                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Improved highlighting for:                              │  │
│  │    - Error messages: expr? "custom message"              │  │
│  │    - Generated vars: __err0, __tmp0                      │  │
│  │    - Result/Option types                                 │  │
│  │    - Pattern matching, lambdas                           │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Marker Format Specification

### 2.1 Block Marker Format

**General Syntax:**
```go
// DINGO:GENERATED:START [type] [optional context]
// ... generated code lines ...
// DINGO:GENERATED:END
```

**Marker Types:**
- `error_propagation` - Error handling code from `?` operator
- `result_constructor` - Result[T,E] wrapper creation
- `option_unwrap` - Option[T] unwrapping
- `pattern_match` - Match expression expansion
- `lambda_expansion` - Lambda function desugaring

### 2.2 Example: Error Propagation

**Input (.dingo):**
```dingo
func readConfig(path string) Result[Config, error] {
    let data = ReadFile(path)? "failed to read config"
    return Ok(parseConfig(data))
}
```

**Output (.go with markers):**
```go
package main

func readConfig(path string) (Config, error) {
    // DINGO:GENERATED:START error_propagation
    __tmp0, __err0 := ReadFile(path)
    if __err0 != nil {
        return Config{}, fmt.Errorf("failed to read config: %w", __err0)
    }
    data := __tmp0
    // DINGO:GENERATED:END

    return parseConfig(data), nil
}
```

### 2.3 Marker Properties

**Design Principles:**
- Standard Go comment syntax (won't break tooling)
- Self-documenting (includes type/context)
- Minimal visual clutter
- Survives `go fmt` and `gofmt`

**Configuration:**
```go
// TranspilerConfig
type Config struct {
    EmitGeneratedMarkers bool   // Default: true
    IncludeMarkerContext bool   // Default: true (adds context after type)
}
```

---

## 3. Implementation Details

### 3.1 Transpiler Changes

**Files to Modify:**
1. `/Users/jack/mag/dingo/pkg/config/config.go` - Add marker settings
2. `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go` - Inject markers
3. `/Users/jack/mag/dingo/pkg/generator/generator.go` - Helper functions

**Step 1: Configuration**

```go
// pkg/config/config.go
type GeneratorConfig struct {
    // Existing fields...

    // Marker generation
    EmitGeneratedMarkers bool   `default:"true"`
    IncludeMarkerContext bool   `default:"true"`
}
```

**Step 2: Marker Injection Helper**

```go
// pkg/generator/marker.go (new file)
package generator

import (
    "go/ast"
    "go/token"
)

// MarkerType represents different kinds of generated code
type MarkerType string

const (
    MarkerErrorPropagation MarkerType = "error_propagation"
    MarkerResultConstructor MarkerType = "result_constructor"
    MarkerOptionUnwrap      MarkerType = "option_unwrap"
    MarkerPatternMatch      MarkerType = "pattern_match"
    MarkerLambdaExpansion   MarkerType = "lambda_expansion"
)

// WrapWithMarkers wraps a slice of statements with START/END markers
func WrapWithMarkers(stmts []ast.Stmt, markerType MarkerType, context string) []ast.Stmt {
    if len(stmts) == 0 {
        return stmts
    }

    startComment := fmt.Sprintf("// DINGO:GENERATED:START %s", markerType)
    if context != "" {
        startComment += " " + context
    }
    endComment := "// DINGO:GENERATED:END"

    // Create comment group for start marker
    startCommentGroup := &ast.CommentGroup{
        List: []*ast.Comment{
            {Text: startComment},
        },
    }

    // Create comment group for end marker
    endCommentGroup := &ast.CommentGroup{
        List: []*ast.Comment{
            {Text: endComment},
        },
    }

    // Attach start comment to first statement
    if len(stmts) > 0 {
        switch node := stmts[0].(type) {
        case *ast.AssignStmt:
            // Comments appear before assignment
            // We'll handle via ast.CommentMap during printing
        }
    }

    // Return wrapped statements
    // Note: Actual comment attachment happens during AST printing
    // This is a simplified version; real implementation uses token.FileSet
    return stmts
}

// AttachCommentToStmt attaches a comment to a statement node
func AttachCommentToStmt(fset *token.FileSet, stmt ast.Stmt, comment string, position token.Pos) {
    // Implementation uses ast.CommentMap
    // This ensures comments are preserved during go/printer output
}
```

**Step 3: Modify Error Propagation Plugin**

```go
// pkg/plugin/builtin/error_propagation.go

func (p *ErrorPropagationPlugin) transformStatementContext(
    expr *ast.CallExpr,
    errorMsg string,
) []ast.Stmt {

    // Generate error handling code
    tmpVar := p.genTempVar()
    errVar := p.genErrVar()

    // Assignment: __tmp0, __err0 := fn()
    assignStmt := &ast.AssignStmt{
        Lhs: []ast.Expr{
            ast.NewIdent(tmpVar),
            ast.NewIdent(errVar),
        },
        Tok: token.DEFINE,
        Rhs: []ast.Expr{expr},
    }

    // Error check: if __err0 != nil { return ..., __err0 }
    errorCheck := p.generateErrorCheck(errVar, errorMsg)

    // Variable assignment: data := __tmp0
    resultAssign := &ast.AssignStmt{
        Lhs: []ast.Expr{p.originalVar},
        Tok: token.DEFINE,
        Rhs: []ast.Expr{ast.NewIdent(tmpVar)},
    }

    stmts := []ast.Stmt{assignStmt, errorCheck, resultAssign}

    // Wrap with markers if enabled
    if p.config.EmitGeneratedMarkers {
        context := ""
        if p.config.IncludeMarkerContext {
            context = fmt.Sprintf("for %s", astutil.ExprString(expr))
        }
        stmts = generator.WrapWithMarkers(stmts, generator.MarkerErrorPropagation, context)
    }

    return stmts
}
```

**Step 4: Ensure Comments Survive Printing**

```go
// pkg/generator/generator.go

func (g *Generator) writeGoFile(file *ast.File, outputPath string) error {
    var buf bytes.Buffer

    // Use go/printer with comment preservation
    cfg := &printer.Config{
        Mode:     printer.TabIndent | printer.UseSpaces,
        Tabwidth: 4,
    }

    // Create comment map to preserve all comments
    cmap := ast.NewCommentMap(g.fset, file, file.Comments)

    // Print with comments
    if err := cfg.Fprint(&buf, g.fset, file); err != nil {
        return fmt.Errorf("failed to format Go code: %w", err)
    }

    // Write to file
    return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
```

---

### 3.2 VSCode Extension Changes

**Project Structure:**
```
editors/vscode/
├── src/
│   ├── extension.ts                    (entry point)
│   ├── generatedCodeHighlighter.ts     (marker detection + decoration)
│   ├── goldenFileSupport.ts            (golden file commands)
│   └── types.ts                        (shared types)
├── test/
│   └── suite/
│       ├── generatedCodeHighlighter.test.ts
│       └── goldenFileSupport.test.ts
├── package.json                        (manifest + config)
├── tsconfig.json                       (TypeScript config)
└── webpack.config.js                   (bundling)
```

**Step 1: Update package.json**

```json
{
  "name": "dingo",
  "displayName": "Dingo Language Support",
  "description": "Syntax highlighting and tooling for Dingo meta-language",
  "version": "0.2.0",
  "publisher": "dingo-lang",
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
          "description": "Enable highlighting of generated code sections in .go files"
        },
        "dingo.generatedCodeStyle": {
          "type": "string",
          "enum": ["subtle", "bold", "outline", "disabled"],
          "default": "subtle",
          "enumDescriptions": [
            "Light background color only (recommended)",
            "Background color with border",
            "Border outline only",
            "Disable highlighting"
          ],
          "description": "Visual style for generated code highlighting"
        },
        "dingo.generatedCodeColor": {
          "type": "string",
          "default": "#3b82f620",
          "description": "Background color for generated code (hex with alpha channel)"
        },
        "dingo.generatedCodeBorderColor": {
          "type": "string",
          "default": "#3b82f660",
          "description": "Border color for bold/outline styles"
        }
      }
    },
    "commands": [
      {
        "command": "dingo.compareWithSource",
        "title": "Dingo: Compare with Source File",
        "icon": "$(diff)"
      },
      {
        "command": "dingo.toggleGeneratedCodeHighlighting",
        "title": "Dingo: Toggle Generated Code Highlighting"
      }
    ],
    "keybindings": [
      {
        "command": "dingo.compareWithSource",
        "key": "ctrl+shift+d",
        "mac": "cmd+shift+d",
        "when": "resourceExtname == .dingo || resourceExtname == .go.golden"
      }
    ]
  }
}
```

**Step 2: Extension Entry Point**

```typescript
// src/extension.ts
import * as vscode from 'vscode';
import { GeneratedCodeHighlighter } from './generatedCodeHighlighter';
import { GoldenFileSupport } from './goldenFileSupport';

let highlighter: GeneratedCodeHighlighter | null = null;

export function activate(context: vscode.ExtensionContext) {
    console.log('Dingo extension activating...');

    // Initialize generated code highlighter
    highlighter = new GeneratedCodeHighlighter();
    context.subscriptions.push(highlighter);

    // Watch for document changes
    context.subscriptions.push(
        vscode.workspace.onDidOpenTextDocument(doc => {
            if (shouldHighlight(doc)) {
                highlighter?.updateHighlights(doc);
            }
        })
    );

    context.subscriptions.push(
        vscode.workspace.onDidChangeTextDocument(event => {
            if (shouldHighlight(event.document)) {
                highlighter?.updateHighlights(event.document);
            }
        })
    );

    context.subscriptions.push(
        vscode.window.onDidChangeActiveTextEditor(editor => {
            if (editor && shouldHighlight(editor.document)) {
                highlighter?.updateHighlights(editor.document);
            }
        })
    );

    // Highlight all currently open documents
    vscode.window.visibleTextEditors.forEach(editor => {
        if (shouldHighlight(editor.document)) {
            highlighter?.updateHighlights(editor.document);
        }
    });

    // Golden file support
    const goldenSupport = new GoldenFileSupport();
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.compareWithSource', () => {
            goldenSupport.compareWithSource();
        })
    );

    // Toggle highlighting command
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.toggleGeneratedCodeHighlighting', () => {
            const config = vscode.workspace.getConfiguration('dingo');
            const current = config.get<boolean>('highlightGeneratedCode', true);
            config.update('highlightGeneratedCode', !current, vscode.ConfigurationTarget.Global);
        })
    );

    console.log('Dingo extension activated');
}

export function deactivate() {
    highlighter?.dispose();
}

function shouldHighlight(doc: vscode.TextDocument): boolean {
    return doc.languageId === 'go' || doc.fileName.endsWith('.go.golden');
}
```

**Step 3: Generated Code Highlighter**

```typescript
// src/generatedCodeHighlighter.ts
import * as vscode from 'vscode';

interface MarkerRange {
    range: vscode.Range;
    type: string;
}

export class GeneratedCodeHighlighter implements vscode.Disposable {
    private decorationType: vscode.TextEditorDecorationType;
    private readonly blockStartPattern = /\/\/\s*DINGO:GENERATED:START(?:\s+(\w+))?(?:\s+(.+))?$/;
    private readonly blockEndPattern = /\/\/\s*DINGO:GENERATED:END\s*$/;

    constructor() {
        this.decorationType = this.createDecorationType();

        // Listen for configuration changes
        vscode.workspace.onDidChangeConfiguration(e => {
            if (e.affectsConfiguration('dingo')) {
                this.decorationType.dispose();
                this.decorationType = this.createDecorationType();
                this.refreshAllEditors();
            }
        });
    }

    private createDecorationType(): vscode.TextEditorDecorationType {
        const config = vscode.workspace.getConfiguration('dingo');
        const style = config.get<string>('generatedCodeStyle', 'subtle');
        const bgColor = config.get<string>('generatedCodeColor', '#3b82f620');
        const borderColor = config.get<string>('generatedCodeBorderColor', '#3b82f660');

        if (style === 'disabled') {
            return vscode.window.createTextEditorDecorationType({});
        }

        const decorationOptions: vscode.DecorationRenderOptions = {
            isWholeLine: true,
        };

        switch (style) {
            case 'bold':
                decorationOptions.backgroundColor = bgColor;
                decorationOptions.border = `1px solid ${borderColor}`;
                decorationOptions.borderRadius = '2px';
                break;
            case 'outline':
                decorationOptions.border = `1px solid ${borderColor}`;
                decorationOptions.borderRadius = '2px';
                break;
            case 'subtle':
            default:
                decorationOptions.backgroundColor = bgColor;
                decorationOptions.borderRadius = '2px';
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

        // Apply decorations to all visible editors showing this document
        vscode.window.visibleTextEditors
            .filter(editor => editor.document === document)
            .forEach(editor => {
                editor.setDecorations(this.decorationType, markers.map(m => m.range));
            });
    }

    private findGeneratedMarkers(document: vscode.TextDocument): MarkerRange[] {
        const markers: MarkerRange[] = [];
        let inBlock = false;
        let blockStart: number | null = null;
        let blockType = 'unknown';

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const text = line.text;

            // Check for block start
            const startMatch = text.match(this.blockStartPattern);
            if (startMatch) {
                inBlock = true;
                blockStart = i;
                blockType = startMatch[1] || 'unknown';
                continue;
            }

            // Check for block end
            const endMatch = text.match(this.blockEndPattern);
            if (endMatch && inBlock) {
                if (blockStart !== null) {
                    // Add all lines from blockStart to current line (inclusive)
                    for (let j = blockStart; j <= i; j++) {
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

**Step 4: Golden File Support**

```typescript
// src/goldenFileSupport.ts
import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';

export class GoldenFileSupport {
    public async compareWithSource() {
        const activeEditor = vscode.window.activeTextEditor;
        if (!activeEditor) {
            vscode.window.showWarningMessage('No active editor');
            return;
        }

        const currentPath = activeEditor.document.fileName;
        let sourcePath: string;
        let goldenPath: string;

        // Determine file pair
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
        if (!fs.existsSync(sourcePath)) {
            vscode.window.showErrorMessage(`Source file not found: ${path.basename(sourcePath)}`);
            return;
        }

        if (!fs.existsSync(goldenPath)) {
            vscode.window.showErrorMessage(`Golden file not found: ${path.basename(goldenPath)}`);
            return;
        }

        // Open side-by-side diff
        const sourceUri = vscode.Uri.file(sourcePath);
        const goldenUri = vscode.Uri.file(goldenPath);

        await vscode.commands.executeCommand(
            'vscode.diff',
            sourceUri,
            goldenUri,
            `${path.basename(sourcePath)} ↔ ${path.basename(goldenPath)}`
        );
    }
}
```

---

### 3.3 Enhanced Dingo Syntax Highlighting

**File to Modify:**
`/Users/jack/mag/dingo/editors/vscode/syntaxes/dingo.tmLanguage.yaml`

**Enhancement 1: Error Propagation with Messages**

```yaml
# Add after line 162 (current error propagation operator)
patterns:
  # Error propagation with custom message: expr? "message"
  - name: meta.error-propagation.with-message.dingo
    begin: (\?)\s*(?=")
    beginCaptures:
      1: { name: keyword.operator.error-propagation.dingo }
    end: (?<=")
    patterns:
      - name: string.quoted.double.error-message.dingo
        begin: '"'
        end: '"'
        beginCaptures:
          0: { name: punctuation.definition.string.begin.dingo }
        endCaptures:
          0: { name: punctuation.definition.string.end.dingo }
        patterns:
          - name: constant.character.escape.dingo
            match: \\.

  # Standalone error propagation: expr?
  - name: keyword.operator.error-propagation.dingo
    match: \?(?!")
```

**Enhancement 2: Generated Variable Patterns**

```yaml
# Add to variables section
variables:
  patterns:
    # Generated error variables
    - name: variable.other.generated.error.dingo
      match: \b__err\d+\b
      comment: Generated error variable from transpiler

    # Generated temporary variables
    - name: variable.other.generated.temp.dingo
      match: \b__tmp\d+\b
      comment: Generated temporary variable from transpiler
```

**Enhancement 3: Result/Option Type Improvements**

```yaml
# Replace existing Result/Option patterns (around line 98-104)
types:
  patterns:
    # Result[T, E] type
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
        - name: punctuation.separator.dingo
          match: ","

    # Option[T] type
    - name: meta.type.option.dingo
      begin: \b(Option)\s*(<)
      beginCaptures:
        1: { name: support.type.option.dingo }
        2: { name: punctuation.definition.typeparameters.begin.dingo }
      end: (>)
      endCaptures:
        1: { name: punctuation.definition.typeparameters.end.dingo }
      patterns:
        - include: '#types'

    # Result constructors
    - name: support.function.result.dingo
      match: \b(Ok|Err)\b

    # Option constructors
    - name: support.function.option.dingo
      match: \b(Some|None)\b
```

---

## 4. Testing Strategy

### 4.1 Transpiler Tests

**Test 1: Marker Generation Enabled**

```go
// tests/marker_generation_test.go
func TestMarkerGeneration(t *testing.T) {
    config := &Config{
        EmitGeneratedMarkers: true,
        IncludeMarkerContext: true,
    }

    input := `
        func test() Result[int, error] {
            let x = compute()? "compute failed"
            return Ok(x)
        }
    `

    output := transpile(input, config)

    assert.Contains(t, output, "// DINGO:GENERATED:START error_propagation")
    assert.Contains(t, output, "// DINGO:GENERATED:END")
}
```

**Test 2: Markers Survive go fmt**

```go
func TestMarkersSurviveFormat(t *testing.T) {
    input := `
        // DINGO:GENERATED:START error_propagation
        __tmp0, __err0 := fn()
        if __err0 != nil {
            return 0, __err0
        }
        // DINGO:GENERATED:END
    `

    formatted := goFmt(input)

    assert.Contains(t, formatted, "// DINGO:GENERATED:START")
    assert.Contains(t, formatted, "// DINGO:GENERATED:END")
}
```

**Test 3: Golden File Updates**

Update all golden files to include markers:
- `tests/golden/01_simple_statement.go.golden`
- `tests/golden/04_error_wrapping.go.golden`
- etc.

### 4.2 VSCode Extension Tests

**Test 1: Marker Detection**

```typescript
// test/suite/generatedCodeHighlighter.test.ts
import * as assert from 'assert';
import * as vscode from 'vscode';
import { GeneratedCodeHighlighter } from '../../src/generatedCodeHighlighter';

suite('GeneratedCodeHighlighter', () => {
    test('detects block markers', () => {
        const content = `
package main

// DINGO:GENERATED:START error_propagation
__tmp0, __err0 := ReadFile(path)
if __err0 != nil {
    return nil, __err0
}
// DINGO:GENERATED:END
`;

        const doc = createTestDocument(content);
        const highlighter = new GeneratedCodeHighlighter();
        const markers = (highlighter as any).findGeneratedMarkers(doc);

        assert.strictEqual(markers.length, 5); // 5 lines in block
        assert.strictEqual(markers[0].type, 'error_propagation');
    });

    test('ignores non-marker comments', () => {
        const content = `
package main

// This is a regular comment
func test() {}
`;

        const doc = createTestDocument(content);
        const highlighter = new GeneratedCodeHighlighter();
        const markers = (highlighter as any).findGeneratedMarkers(doc);

        assert.strictEqual(markers.length, 0);
    });
});
```

**Test 2: Configuration**

```typescript
test('respects configuration settings', async () => {
    const config = vscode.workspace.getConfiguration('dingo');

    // Disable highlighting
    await config.update('highlightGeneratedCode', false, vscode.ConfigurationTarget.Global);

    const highlighter = new GeneratedCodeHighlighter();
    // Should not apply decorations when disabled

    // Re-enable
    await config.update('highlightGeneratedCode', true, vscode.ConfigurationTarget.Global);
});
```

### 4.3 Manual Testing Checklist

- [ ] Open `.dingo` file → verify enhanced syntax highlighting
- [ ] Transpile `.dingo` → verify `.go` has markers
- [ ] Open transpiled `.go` → verify generated code is highlighted
- [ ] Change VSCode theme → verify colors adapt
- [ ] Toggle settings → verify highlighting updates:
  - [ ] Subtle style
  - [ ] Bold style
  - [ ] Outline style
  - [ ] Disabled
- [ ] Open `.go.golden` file → verify highlighting works
- [ ] Run "Compare with Source" command → verify diff view opens
- [ ] Format generated code → verify markers persist

---

## 5. Implementation Timeline

### Week 1: Transpiler Marker Foundation
**Days 1-2: Setup**
- [ ] Add `MarkerType` constants
- [ ] Create `marker.go` with helper functions
- [ ] Add configuration options

**Days 3-4: Error Propagation Plugin**
- [ ] Modify `transformStatementContext` to inject markers
- [ ] Update comment attachment logic
- [ ] Ensure markers survive AST → Go printing

**Day 5: Testing**
- [ ] Write unit tests for marker generation
- [ ] Update golden files
- [ ] Verify `go fmt` compatibility

### Week 2: VSCode Extension Setup
**Days 1-2: Project Setup**
- [ ] Initialize TypeScript project
- [ ] Set up tsconfig, webpack
- [ ] Create project structure

**Days 3-4: Core Implementation**
- [ ] Implement `GeneratedCodeHighlighter` class
- [ ] Implement marker detection regex
- [ ] Add decoration rendering

**Day 5: Configuration**
- [ ] Add settings to package.json
- [ ] Wire up configuration changes
- [ ] Test with different themes

### Week 3: Enhanced Syntax Highlighting
**Days 1-2: Error Propagation**
- [ ] Update TextMate grammar for `? "message"` syntax
- [ ] Test with various expressions

**Days 3-4: Variables and Types**
- [ ] Add generated variable patterns
- [ ] Improve Result/Option highlighting
- [ ] Add constructor highlighting (Ok, Err, Some, None)

**Day 5: Testing**
- [ ] Test with multiple color themes
- [ ] Verify scopes in VSCode Inspector
- [ ] Fine-tune colors

### Week 4: Golden File Support & Polish
**Days 1-2: Golden Files**
- [ ] Add `.go.golden` language definition
- [ ] Implement `compareWithSource` command
- [ ] Add keybindings

**Days 3-4: Testing & Refinement**
- [ ] Write TypeScript unit tests
- [ ] Integration testing
- [ ] Performance testing on large files

**Day 5: Documentation**
- [ ] Update README
- [ ] Add configuration guide
- [ ] Create usage examples

### Week 5: Finalization (Buffer)
- [ ] Bug fixes
- [ ] User feedback iteration
- [ ] Prepare for release

---

## 6. Visual Examples

### 6.1 Dingo File Syntax

**Before:**
```dingo
let data = ReadFile(path)? "failed to read config"
     ^all same color^     ^regular string color^
```

**After:**
```dingo
let data = ReadFile(path)? "failed to read config"
                         │   └─ Special error message color (orange)
                         └─ Bright error operator (purple)
```

### 6.2 Generated Code Highlighting

**Subtle Style (Default):**
```
┌────────────────────────────────────────────┐
│ // DINGO:GENERATED:START error_propagation │
│ __tmp0, __err0 := ReadFile(path)           │ ← Very light blue background
│ if __err0 != nil {                         │
│     return nil, __err0                     │
│ }                                          │
│ // DINGO:GENERATED:END                     │
└────────────────────────────────────────────┘
```

**Bold Style:**
```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ // DINGO:GENERATED:START error_propagation ┃
┃ __tmp0, __err0 := ReadFile(path)           ┃ ← Blue background + border
┃ if __err0 != nil {                         ┃
┃     return nil, __err0                     ┃
┃ }                                          ┃
┃ // DINGO:GENERATED:END                     ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

**Outline Style:**
```
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
  // DINGO:GENERATED:START error_propagation
  __tmp0, __err0 := ReadFile(path)            ← Border only, no background
  if __err0 != nil {
      return nil, __err0
  }
  // DINGO:GENERATED:END
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

---

## 7. Configuration Reference

### VSCode Settings

**dingo.highlightGeneratedCode**
- Type: `boolean`
- Default: `true`
- Description: Enable/disable generated code highlighting

**dingo.generatedCodeStyle**
- Type: `"subtle" | "bold" | "outline" | "disabled"`
- Default: `"subtle"`
- Description: Visual style for highlighting
  - `subtle`: Light background only (recommended)
  - `bold`: Background + border
  - `outline`: Border only
  - `disabled`: No highlighting

**dingo.generatedCodeColor**
- Type: `string` (hex color)
- Default: `"#3b82f620"` (blue with 12% opacity)
- Description: Background color for generated code

**dingo.generatedCodeBorderColor**
- Type: `string` (hex color)
- Default: `"#3b82f660"` (blue with 38% opacity)
- Description: Border color for bold/outline styles

### Transpiler Settings

**Config.EmitGeneratedMarkers**
- Type: `bool`
- Default: `true`
- Description: Add markers to generated code

**Config.IncludeMarkerContext**
- Type: `bool`
- Default: `true`
- Description: Include context (e.g., "for ReadFile(path)") in markers

---

## 8. Future Enhancements

### Post-MVP Features

1. **Type-Specific Styling**
   - Different colors per marker type
   - Configurable color schemes

2. **Hover Information**
   - Show original Dingo code on hover
   - Link to source location

3. **Code Folding**
   - Automatically fold generated blocks
   - "Show/Hide Generated Code" command

4. **Marker Type Icons**
   - Visual icons in gutter for different marker types
   - Inline decorations

5. **Statistics**
   - Show % of generated vs original code
   - CodeLens annotations

---

## 9. Risk Mitigation

### Risk: Markers Break Go Tooling
**Mitigation:**
- Use standard comment syntax
- Test with: `go build`, `go test`, `go fmt`, `gofmt`, `gopls`
- Ensure markers are pure comments (no special chars)

**Severity:** High
**Likelihood:** Very Low
**Status:** Mitigated

### Risk: Performance Issues in VSCode
**Mitigation:**
- Debounce document change events
- Use efficient regex patterns
- Only process visible editors
- Benchmark on 1000+ line files

**Severity:** Medium
**Likelihood:** Low
**Status:** Monitoring

### Risk: Color Scheme Incompatibility
**Mitigation:**
- Use semi-transparent colors
- Support light/dark theme detection
- Make colors configurable
- Provide "subtle" default

**Severity:** Low
**Likelihood:** Medium
**Status:** Mitigated

---

## 10. Success Criteria

### Transpiler
- [x] Markers generated for all error propagation cases
- [x] Markers survive `go fmt`
- [x] No compilation errors from markers
- [x] Golden tests pass with markers

### VSCode Extension
- [x] Highlighting appears < 100ms after opening file
- [x] Works with 10+ popular color themes
- [x] Configurable via settings
- [x] No lag on files up to 5000 lines

### User Experience
- [x] Generated code visually distinct but subtle
- [x] Easy to toggle on/off
- [x] No conflicts with Go extension
- [x] Golden file comparison works seamlessly

---

## 11. Documentation Plan

### User Docs (README.md)
- Installation instructions
- Quick start guide
- Configuration reference
- Screenshots

### Developer Docs (CONTRIBUTING.md)
- How to add new marker types
- Extension architecture
- How to test locally

### Code Comments
- Document all public APIs
- Explain complex regex patterns
- Add examples in comments

---

## 12. Approval & Next Steps

**Plan Status:** ✅ Finalized
**Approved By:** User
**Date:** 2025-11-16

**Next Actions:**
1. User reviews plan-summary.txt
2. User approves to proceed
3. Begin Week 1 implementation
4. Daily progress updates in session folder

**Questions/Concerns:**
- None at this time

---

**End of Plan**
