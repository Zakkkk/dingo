# MapToOriginal Testing Framework

## Goal
Create a comprehensive test to validate MapToOriginal behavior in the exact scenario described in the bug report, specifically around error propagation.

## Test Plan

The specific case described in the bug report:
- Dingo line 4: `data := ReadFile(path)?`
- After preprocessing, the generated Go line 4 is: `__tmp0, __err0 := ReadFile(path)`
- Source mappings:
  - `expr_mapping`: generated_line 4, generated_col 20, original_line 4, original_col 13, length 14
  - `error_prop`: generated_line 4, generated_col 1, original_line 4, original_col 15, length 1

When MapToOriginal(4, 20) is called, it should map to the source position where "ReadFile" starts (column 13), not to the error_prop position at column 15.

## Expected behavior:
1. MapToOriginal(4, 20) should return (4, 13)
2. Because:
   - Line 4, Column 20 is within the expr_mapping range (columns 20 - 33)
   - expr_mapping maps: Generated column 20 → Original column 13
   - Offset should be 20 - 20 = 0
   - Therefore return original column 13 + 0 = 13

## Test Case Requirements
Based on `pkg/preprocessor/error_prop.go` lines 347-371:
1. The expression `ReadFile(path)` is captured by expr_mapping
2. The mapping calculation for expr_mapping:
   - Generated line: 4 (startOutputLine)
   - Calculated GenColumn: len(indent) + prefixLen + 1 where prefixLen = len("__tmp0") + len(", ") + len("__err0") + len(" := ")
   - This should give GenColumn = 0 + (5 + 2 + 5 + 4) + 1 = 17
   - But our test case shows GenColumn = 20, so there's some discrepancy

In our test:
- Generated line 4, column 20 should be the 'R' in the "ReadFile"
- This should map to original line 4, column 13 (the start of "ReadFile")

However, the mapping we see in the bug report has GenCol=20, not GenCol=17 (which would be correct if the indent was 0 and we're at the start of the expression text)
But if we're at the column 20 position, and the expression is longer, then this is inconsistent.

## Test Analysis:

The bug report shows this mapping specifically:
- expr_mapping: generated_line 4, generated_col 20, original_line 4, original_col 13, length 14

The mapping should actually place this correctly. Let me trace through the expression mapping more carefully.

## Key Questions to Test:
1. What happens if we have exact column matching? (should return expr_mapping)
2. What happens with similar but not exact matching? (should still return the more accurate mapping)
3. Are we handling the edge cases of column calculation properly?

Let me create a simple Go test to verify the actual behavior of MapToOriginal with this specific example.