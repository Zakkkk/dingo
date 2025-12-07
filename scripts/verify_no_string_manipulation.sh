#!/bin/bash
# verify_no_string_manipulation.sh
#
# This script MUST be run before any PR that touches codegen code.
# If it finds matches, the code is WRONG and must be rewritten.
#
# See: ai-docs/plans/REMOVE_STRING_MANIPULATION.md

echo "=============================================="
echo "VERIFYING: No String Manipulation in Codegens"
echo "=============================================="
echo ""

FAILED=0

# Helper function to check for patterns
# Excludes:
#   - Test files (*_test.go) - tests need strings.Contains for assertions
#   - Comment lines (// or *) - documentation mentions forbidden patterns
#   - Disabled files (*.disabled)
check_pattern() {
    local description="$1"
    local pattern="$2"

    echo "Checking: $description..."

    # Check pkg/ast/*_codegen.go files (exclude tests, comments, disabled files)
    local matches=""
    if ls pkg/ast/*_codegen.go 1>/dev/null 2>&1; then
        matches=$(grep -rn "$pattern" pkg/ast/*_codegen.go 2>/dev/null | \
            grep -v "_test\.go" | \
            grep -v "\.disabled" | \
            grep -v "//.*$pattern" | \
            grep -v "^\s*\*" || true)
    fi

    # Also check pkg/codegen/ if it exists (exclude tests, comments, disabled files)
    if [ -d "pkg/codegen" ]; then
        local codegen_matches=$(grep -rn "$pattern" pkg/codegen/*.go 2>/dev/null | \
            grep -v "_test\.go" | \
            grep -v "\.disabled" | \
            grep -v "//.*$pattern" | \
            grep -v "^\s*\*" || true)
        matches="$matches$codegen_matches"
    fi

    if [ -n "$matches" ]; then
        echo "$matches"
        echo "❌ FAILED: $description"
        echo ""
        FAILED=1
        return 1
    else
        echo "✅ PASSED"
        echo ""
        return 0
    fi
}

# Run all checks
check_pattern "bytes/strings index/contains" "bytes\.Index\|bytes\.Contains\|strings\.Index\|strings\.Contains"
check_pattern "regexp usage" "regexp\."
check_pattern "Find* functions (byte scanning)" "func Find"

# Check Transform*Source functions - but exclude known transitional exceptions
# These functions are part of the migration to full AST-based pipeline:
# - pkg/ast/enum_codegen.go:TransformEnumSource - uses proper parser internally
# - pkg/ast/transform.go:TransformSource - token-based pipeline entry point
# Both will be deprecated once the full tokenizer → parser → codegen pipeline is complete.
echo "Checking: Transform*Source functions (wrong pattern)..."
TRANSFORM_MATCHES=$(grep -rn "func Transform.*Source" pkg/ast/*.go pkg/codegen/*.go 2>/dev/null | \
    grep -v "enum_codegen.go:.*TransformEnumSource" | \
    grep -v "transform.go:.*TransformSource" | \
    grep -v "transform_stub.go" | \
    grep -v "_test\.go" || true)
if [ -n "$TRANSFORM_MATCHES" ]; then
    echo "$TRANSFORM_MATCHES"
    echo "❌ FAILED: Transform*Source functions (wrong pattern)"
    echo ""
    FAILED=1
else
    echo "✅ PASSED (transitional functions allowed: TransformSource, TransformEnumSource)"
    echo ""
fi

check_pattern "character scanning loops" "for.*i.*<.*len(src)"
check_pattern "regexp import" "^import.*\"regexp\""

# Summary
echo "=============================================="
if [ $FAILED -eq 1 ]; then
    echo "❌ VERIFICATION FAILED"
    echo ""
    echo "The code contains string manipulation patterns."
    echo ""
    echo "REQUIRED ACTIONS:"
    echo "  1. DELETE the offending files"
    echo "  2. Create new codegens that ONLY accept AST nodes"
    echo "  3. Use pkg/tokenizer/ for tokenization"
    echo "  4. Use pkg/parser/ for parsing"
    echo ""
    echo "📖 Read: ai-docs/plans/REMOVE_STRING_MANIPULATION.md"
    exit 1
else
    echo "✅ VERIFICATION PASSED"
    echo ""
    echo "No string manipulation patterns detected."
fi
