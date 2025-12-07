#!/bin/bash
# Test the issue triage bot locally
# Usage: ./scripts/test-issue-triage.sh "Issue title" "Issue body"

set -e

TITLE="${1:-Add support for async/await syntax}"
BODY="${2:-I think it would be great if Dingo supported async/await like JavaScript. This would make writing concurrent code much easier.}"
AUTHOR="${3:-testuser}"

echo "=== Testing Issue Triage Bot ==="
echo "Title: $TITLE"
echo "Author: @$AUTHOR"
echo ""

# Create triage directory
mkdir -p .triage

# Write test issue
cat > .triage/issue.md << EOF
# Test Issue

**Title:** $TITLE

**Author:** @$AUTHOR

**Body:**
$BODY
EOF

echo "Issue written to .triage/issue.md"
echo ""

# Check if ANTHROPIC_API_KEY is set
if [ -z "$ANTHROPIC_API_KEY" ]; then
  echo "ERROR: ANTHROPIC_API_KEY not set"
  echo "Run: export ANTHROPIC_API_KEY=your-key"
  exit 1
fi

# Check if claude is installed
if ! command -v claude &> /dev/null; then
  echo "Installing Claude Code..."
  npm install -g @anthropic-ai/claude-code@latest
fi

echo "Running Claude Code (Opus 4.5)..."
echo ""

# Run the triage
claude --model opus -p --dangerously-skip-permissions \
  --system-prompt "$(cat .github/prompts/issue-triage-system.md)" \
  "Triage the GitHub issue in .triage/issue.md. Read it, explore the codebase for context, then write your triage result to .triage/result.json"

echo ""
echo "=== Triage Result ==="

if [ -f .triage/result.json ]; then
  cat .triage/result.json | jq .

  echo ""
  echo "=== Response Preview ==="
  cat .triage/result.json | jq -r '.response'
else
  echo "ERROR: result.json not created"
  exit 1
fi

# Cleanup
echo ""
echo "Cleaning up..."
rm -rf .triage

echo "Done!"
