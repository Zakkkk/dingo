# Issue Triage Bot Setup

The Dingo project uses an automated issue triage bot powered by [Claude Code](https://github.com/anthropics/claude-code) (Opus 4.5) to categorize and respond to new GitHub issues.

## How It Works

When a new issue is opened:

1. **Checkout**: Full repository is checked out
2. **Claude Code Agent**: Runs with full codebase access via claudish
3. **Exploration**: Agent reads `features/INDEX.md`, checks `pkg/` implementations, looks at `examples/`
4. **Analysis**: Determines if feature exists, is planned, or is new
5. **Response**: Posts a conversational reply with specific file references

## Key Difference: Full Codebase Access

Unlike simple API-based bots, this triage bot runs Claude Code with full access to:
- All source code in `pkg/`
- Feature documentation in `features/*.md`
- Working examples in `examples/`
- Implementation status in `CLAUDE.md`

This means it can give accurate answers like "that's already implemented in `pkg/ast/lambda_codegen.go`" or "see `examples/04_pattern_matching/` for usage."

## Labels Used

| Label | Description |
|-------|-------------|
| `bug` | Something broken in existing feature |
| `enhancement` | New feature or improvement |
| `question` | User needs help/clarification |
| `discussion` | Open-ended topic for feedback |
| `duplicate` | Already exists as issue/feature |
| `P0-critical` | Critical - blocking users |
| `P1-high` | High - significant impact |
| `P2-medium` | Medium - quality of life |
| `P3-low` | Low - nice to have |
| `already-implemented` | Feature already exists |
| `planned` | Feature is on the roadmap |

## Setup Requirements

Add this secret to your repository:

| Secret | Required | Description |
|--------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Anthropic API key for Claude Code (Opus 4.5) |

## Response Style

The bot uses a conversational, specific response style:
- 2-4 sentences max
- References specific files/examples from the codebase
- No generic phrases like "Thanks for sharing!"
- Points to `features/*.md` for planned features
- Willing to push back respectfully when needed

## Example Responses

**Already implemented:**
> The `?.` safe navigation you're describing is already in place - check out `pkg/ast/safe_nav_codegen.go` and the example in `examples/07_null_safety/`. The current implementation uses marker-based transformation, which handles most common cases.

**Planned feature:**
> Default parameters are on our radar but haven't started yet. There's a design doc at `features/default-parameters.md` that outlines two approaches we're considering. Would be good to hear which strategy you'd prefer.

**New idea:**
> Interesting angle on operator overloading for matrix types. We've got a basic design in `features/operator-overloading.md` but hadn't considered the scientific computing use case specifically. Converting this to a discussion to gather more input.
