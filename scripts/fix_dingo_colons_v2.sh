#!/bin/bash

# Script to fix colon syntax in .dingo files (v2 - fixed)
# Converts ONLY function parameters: func name(param: Type) -> func name(param Type)
# Does NOT touch struct literals, lambda params, struct tags, etc.

set -e

REPORT_FILE="/Users/jack/mag/dingo/ai-docs/sessions/20251208-082719/04-testing/dingo-fixes-v2.md"

echo "# Dingo Colon Syntax Fix Report (v2 - Corrected)" > "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "Date: $(date)" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "## Summary" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

count=0
modified_files=()

# Find all .dingo files
while IFS= read -r file; do
    # Create backup
    cp "$file" "$file.bak"

    # Use perl for precise regex
    # Only match function/method parameter declarations
    # Pattern: func name(param: Type) or func (receiver: Type) method
    perl -i -pe '
        # Match function parameters: func name(..., param: Type, ...)
        # Only within function declarations (after "func")
        s/\bfunc\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)/handle_func_params($1, $2)/ge;

        # Match method receivers: func (r: Type) method(...)
        s/\bfunc\s+\(([^)]+)\)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)/handle_method($1, $2, $3)/ge;

        sub handle_func_params {
            my ($name, $params) = @_;
            $params =~ s/([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*[a-zA-Z_])/$1 $2/g;
            return "func $name($params)";
        }

        sub handle_method {
            my ($receiver, $name, $params) = @_;
            $receiver =~ s/([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*[a-zA-Z_])/$1 $2/;
            $params =~ s/([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*[a-zA-Z_])/$1 $2/g;
            return "func ($receiver) $name($params)";
        }
    ' "$file"

    # Check if file was actually modified
    if ! diff -q "$file" "$file.bak" > /dev/null 2>&1; then
        count=$((count + 1))
        modified_files+=("$file")
        echo "Modified: $file"
        rm "$file.bak"
    else
        # No changes, restore backup
        mv "$file.bak" "$file"
    fi
done < <(find /Users/jack/mag/dingo -name "*.dingo" -type f)

echo "Total files modified: $count" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "## Modified Files" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

for file in "${modified_files[@]}"; do
    echo "- $file" >> "$REPORT_FILE"
done

echo ""
echo "Report written to: $REPORT_FILE"
echo "Total files modified: $count"
