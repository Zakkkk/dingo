# CommonMark Code Blocks Specification (v0.29)

VS Code uses CommonMark for rendering markdown.

## Indented Code Blocks

### Definition
An indented code block is composed of one or more indented chunks separated by blank lines. An indented chunk is a sequence of non-blank lines, each indented **four or more spaces**.

### Key Rules

1. **Four-space indentation requirement**: Content must be indented at least four spaces to form a code block.

2. **Cannot interrupt paragraphs**: An indented code block cannot interrupt a paragraph, so there must be a blank line between a paragraph and a following indented code block.

3. **Content preservation**: Lines retain their literal text, including trailing line endings, minus the four-space indentation.

4. **No info string**: Indented code blocks cannot have language identifiers.

5. **List item precedence**: When ambiguity exists between code block indentation and list item indentation, list items take priority.

### Examples

```markdown
    code line 1
    code line 2
```

Renders as a code block.

## Fenced Code Blocks

### Definition
A code fence is a sequence of at least three consecutive backtick characters (`) or tildes (~). A fenced code block begins with a code fence, indented no more than three spaces.

### Key Rules

1. **Opening fence**: Must contain 3+ identical characters (backticks or tildes, not mixed)

2. **Info string**: The line with the opening code fence may optionally contain some text following the code fence; this is trimmed of leading and trailing whitespace and called the info string (e.g., `dingo`, `go`, `javascript`).

3. **Info string restrictions**: If using backticks, the info string cannot contain backticks

4. **Closing fence**: May be indented up to three spaces, and may be followed only by spaces

5. **Matching requirement**: Closing fence must use the same character type and have at least as many characters as the opening fence

6. **Unclosed blocks**: If no closing fence is found, the code block contains all lines until the end

7. **Can interrupt paragraphs**: A fenced code block may interrupt a paragraph, and does not require a blank line either before or after.

8. **Indentation handling**: If the opening fence is indented N spaces, up to N spaces of indentation are removed from each content line.

### Examples

```markdown
```dingo
|x int| x * 2
```
```

Renders as syntax-highlighted Dingo code (if language is registered).

## Key Insight for LSP Diagnostics

If VS Code renders CommonMark in diagnostic messages, then:
- Fenced code blocks (```` ```dingo ````) should render with syntax highlighting
- Inline code (`` `code` ``) should render with monospace formatting
- Headers, bold, etc. should also work

Test this by using fenced code blocks in the diagnostic message.
