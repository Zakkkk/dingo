# Claude Code Hooks

This directory contains verification hooks for the Dingo project.

## Purpose

These hooks enforce architectural rules that have been violated multiple times.
They run automatically to catch violations before they're committed.

## Available Hooks

### check-forbidden-patterns.sh

Verifies that transformation code doesn't use forbidden byte manipulation patterns.

**What it checks:**
- `pkg/transpiler/*.go` (excluding tests)
- `pkg/codegen/*.go` (excluding tests)
- `pkg/ast/*_codegen.go`

**Forbidden patterns:**
- `bytes.Index`, `bytes.HasPrefix`, `bytes.Contains`
- `strings.Index`, `strings.Contains`, `strings.Split` on source bytes
- `regexp.*` for source scanning
- Character scanning loops (`for i := 0; i < len(src)...`)

**Why these are forbidden:**
Byte offsets are fragile - they shift when go/printer reformats code.
Position tracking must use `token.Pos` and `token.FileSet` which provide
stable logical references that survive reformatting.

**Manual run:**
```bash
.claude/hooks/check-forbidden-patterns.sh
```

## Suppressing False Positives

If a pattern is legitimately needed (rare), add `// OK:` comment:
```go
// OK: This is string matching on identifier names, not position tracking
if strings.Contains(identName, "Dest") { ... }
```

## Integration with Claude Code

To configure as a post-edit hook, add to `.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": ".claude/hooks/check-forbidden-patterns.sh"
          }
        ]
      }
    ]
  }
}
```

## See Also

- `CLAUDE.md` - Full explanation of the architectural principle
- `ai-docs/adr/` - Architectural decision records
