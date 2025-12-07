#!/usr/bin/env python3
"""
Fix colon syntax in .dingo function/method parameters.
Converts: func name(param: Type) -> func name(param Type)

This script:
- Modifies ONLY function parameters and method receivers
- Preserves lambda parameters (|x: int|, (x: string))
- Preserves struct field assignments (AppError{Code: code})
- Preserves struct tags (json:"name")
- Preserves map/slice literals
"""

import re
import sys
from pathlib import Path

def fix_function_declaration(line):
    """
    Fix function declarations: func name(param: Type, ...) -> func name(param Type, ...)
    Only modifies parameters inside function declarations, not lambda params or struct literals.
    """

    # Pattern 1: func (receiver: Type) method(params)
    # Capture groups: func (receiver: Type) method(params)
    pattern1 = r'\bfunc\s+\(([^)]+)\)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\('

    def fix_receiver(match):
        receiver_str = match.group(1)
        method_name = match.group(2)

        # Fix colon in receiver: "r: Type" -> "r Type"
        # Only match: identifier: Type (not in struct literals or maps)
        receiver_fixed = re.sub(r'([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*[a-zA-Z_])', r'\1 \2', receiver_str)

        return f'func ({receiver_fixed}) {method_name}('

    line = re.sub(pattern1, fix_receiver, line)

    # Pattern 2: func name(params) - regular function
    # We need to be more careful here - only fix parameters within the function signature
    # Strategy: Find function declarations, extract param list, fix colons, reassemble

    pattern2 = r'\bfunc\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)'

    def fix_params(match):
        func_name = match.group(1)
        params_str = match.group(2)

        # Fix colons in parameters
        # Match: identifier: Type (but NOT Field: value in struct literals)
        # Key insight: Function params come after 'func', struct fields come after '{'
        params_fixed = re.sub(r'([a-zA-Z_][a-zA-Z0-9_]*): (\*?(?:\[\])*(?:func\(|[a-zA-Z_]))', r'\1 \2', params_str)

        return f'func {func_name}({params_fixed})'

    line = re.sub(pattern2, fix_params, line)

    return line

def should_fix_line(line):
    """
    Determine if line should be processed.
    Only process lines with function declarations.
    """
    # Skip if line contains struct literal (has '{' after an identifier)
    if re.search(r'[a-zA-Z_][a-zA-Z0-9_]*\s*{', line):
        # Check if it's actually a function declaration with a struct literal in it
        if not line.strip().startswith('func '):
            return False

    # Only process lines that start with 'func ' (possibly with leading whitespace)
    return line.lstrip().startswith('func ')

def fix_file(filepath):
    """Fix a single .dingo file."""
    with open(filepath, 'r', encoding='utf-8') as f:
        lines = f.readlines()

    modified = False
    new_lines = []

    for line in lines:
        if should_fix_line(line):
            fixed_line = fix_function_declaration(line)
            if fixed_line != line:
                modified = True
            new_lines.append(fixed_line)
        else:
            new_lines.append(line)

    if modified:
        with open(filepath, 'w', encoding='utf-8') as f:
            f.writelines(new_lines)
        return True

    return False

def main():
    """Process all .dingo files."""
    import subprocess

    # Find all .dingo files
    result = subprocess.run(
        ['find', '/Users/jack/mag/dingo', '-name', '*.dingo', '-type', 'f'],
        capture_output=True,
        text=True
    )

    files = [Path(f.strip()) for f in result.stdout.split('\n') if f.strip()]

    modified_files = []
    for filepath in files:
        if fix_file(filepath):
            modified_files.append(str(filepath))
            print(f"Modified: {filepath}")

    # Write report
    report_path = Path('/Users/jack/mag/dingo/ai-docs/sessions/20251208-082719/04-testing/dingo-fixes-final.md')
    report_path.parent.mkdir(parents=True, exist_ok=True)

    with open(report_path, 'w', encoding='utf-8') as f:
        f.write("# Dingo Colon Syntax Fix Report (Final)\n\n")
        f.write(f"**Total files scanned**: {len(files)}\n")
        f.write(f"**Total files modified**: {len(modified_files)}\n\n")
        f.write("## Modified Files\n\n")
        for filepath in modified_files:
            f.write(f"- {filepath}\n")

    print(f"\nTotal files modified: {len(modified_files)}")
    print(f"Report: {report_path}")

if __name__ == '__main__':
    main()
