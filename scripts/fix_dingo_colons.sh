#!/bin/bash

# Script to fix colon syntax in .dingo files
# Converts: func name(param: Type) -> func name(param Type)

set -e

REPORT_FILE="/Users/jack/mag/dingo/ai-docs/sessions/20251208-082719/04-testing/dingo-fixes.md"

echo "# Dingo Colon Syntax Fix Report" > "$REPORT_FILE"
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

    # Apply fixes using sed
    # Pattern 1: Function parameters (param: Type) -> (param Type)
    # Pattern 2: Method receivers (r: Type) -> (r Type)
    # We need to be careful not to touch struct tags or map literals

    # Use perl for more powerful regex
    perl -i -pe '
        # Fix function parameters and method receivers
        # Match: (identifier: type) where type can be *, [], func, or identifier
        s/\(([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*(?:func\(|[a-zA-Z_]))/($1 $2/g;

        # Fix additional parameters in same line
        s/, ([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*(?:func\(|[a-zA-Z_]))/, $1 $2/g;
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
