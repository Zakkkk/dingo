#!/bin/bash
# check-forbidden-patterns.sh
# Verifies no forbidden byte manipulation patterns exist in transformation code.
#
# This hook enforces the architectural principle:
# "Position information flows through the token system, NEVER through byte arithmetic."
#
# See CLAUDE.md for full explanation of WHY these patterns are forbidden.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔍 Checking for forbidden byte manipulation patterns..."

# Define the forbidden patterns
# Note: We check for patterns that indicate byte-level position manipulation
FORBIDDEN_PATTERNS=(
    "bytes\.Index"
    "bytes\.HasPrefix"
    "strings\.Index"
    "strings\.Contains.*src"
    "strings\.Split.*src"
    "regexp\.MustCompile"
    "regexp\.Match"
    "regexp\.Find"
    "for.*:=.*0.*<.*len.*src.*\+\+"
)

# Build grep pattern
GREP_PATTERN=$(IFS="|"; echo "${FORBIDDEN_PATTERNS[*]}")

# Directories to check (transformation code, NOT tests)
CHECK_DIRS=(
    "pkg/transpiler/*.go"
    "pkg/codegen/*.go"
    "pkg/ast/*_codegen.go"
    "pkg/ast/stmt_finder.go"
    "pkg/lsp/translator.go"
    "pkg/lsp/handlers.go"
    "pkg/lsp/server.go"
)

# Find violations
VIOLATIONS=""
for dir in "${CHECK_DIRS[@]}"; do
    # Expand glob and check each file
    for file in $dir; do
        if [ -f "$file" ]; then
            # Skip test files
            if [[ "$file" == *"_test.go" ]]; then
                continue
            fi

            # Check for forbidden patterns
            # Filter out:
            # - Lines with "// OK:" marker (legitimate uses)
            # - Lines that are pure comments (start with // after line number)
            # - Lines that are doc comments (start with * in block comments)
            # Pattern handles single-file grep output: "linenum:content"
            MATCHES=$(grep -n -E "$GREP_PATTERN" "$file" 2>/dev/null \
                | grep -v "// OK:" \
                | grep -v "^[0-9]*:[[:space:]]*//" \
                | grep -v "^[0-9]*:[[:space:]]*\*" \
                || true)
            if [ -n "$MATCHES" ]; then
                VIOLATIONS="$VIOLATIONS\n${YELLOW}$file:${NC}\n$MATCHES\n"
            fi
        fi
    done
done

if [ -n "$VIOLATIONS" ]; then
    echo -e "${RED}🚨 FORBIDDEN BYTE MANIPULATION PATTERNS DETECTED${NC}"
    echo ""
    echo -e "The following code violates the token-based architecture:"
    echo -e "$VIOLATIONS"
    echo ""
    echo -e "${YELLOW}The architectural principle:${NC}"
    echo "  Position information flows through the token system, NEVER through byte arithmetic."
    echo ""
    echo -e "${YELLOW}FIX: Use these token-based approaches instead:${NC}"
    echo "  • For Go source: use go/scanner with token.FileSet"
    echo "  • For offset→line:col: use file.SetLinesForContent() + fset.Position()"
    echo "  • Track positions during generation, don't scan output"
    echo ""
    echo "See CLAUDE.md '✅ REQUIRED Approaches' section for code examples."
    exit 1
else
    echo -e "${GREEN}✅ No forbidden patterns found${NC}"
    exit 0
fi
