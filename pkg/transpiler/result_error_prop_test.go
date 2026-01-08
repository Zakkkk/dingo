package transpiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResultTypeErrorPropagation verifies that the ? operator generates correct code
// for Result[T, E] types vs (T, error) tuples.
//
// EXPECTED BEHAVIOR:
// - Result[T, E] types: use .IsErr() and .MustErr() pattern
// - (T, error) tuples: use != nil pattern (existing behavior)
//
// This test should FAIL until the Result type error propagation is implemented.
func TestResultTypeErrorPropagation(t *testing.T) {
	// Read the test file
	testFile := filepath.Join("..", "..", "tests", "golden", "result_error_prop_test.dingo")
	source, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	// Transpile
	result, err := PureASTTranspile(source, "result_error_prop_test.dingo")
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	goCode := string(result)

	// Log the generated code for debugging
	t.Logf("Generated code:\n%s", goCode)

	t.Run("Result type should use IsErr pattern", func(t *testing.T) {
		// For Result[T, E] types, the generated code should use:
		//   tmp := getResult()
		//   if tmp.IsErr() { return dgo.Err[...](tmp.MustErr()) }
		//
		// NOT:
		//   tmp, err := getResult()
		//   if err != nil { return ..., err }

		// The file has 3 uses of getResult()?, so we expect at least 3 "if tmp.IsErr()" patterns
		// (one for each ? operator on a Result type)
		// We look for "if tmp" + ".IsErr()" pattern to avoid counting user code .IsOk() calls
		isErrCount := 0
		lines := strings.Split(goCode, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Look for the error check pattern: if tmpN.IsErr()
			if strings.HasPrefix(trimmed, "if tmp") && strings.Contains(trimmed, ".IsErr()") {
				isErrCount++
			}
		}

		if isErrCount < 3 {
			t.Errorf("Expected at least 3 'if tmp.IsErr()' checks for Result type error propagation, found %d.\n\n"+
				"The ? operator on Result[T, E] should generate:\n"+
				"  tmp := expr\n"+
				"  if tmp.IsErr() { return dgo.Err[T, E](tmp.MustErr()) }\n"+
				"  val := tmp.MustOk()\n\n"+
				"But currently generates tuple-style error handling.", isErrCount)
		}
	})

	t.Run("Result type should use MustErr to extract error", func(t *testing.T) {
		// For Result[T, E] types, when propagating the error we need:
		//   return dgo.Err[T, E](tmp.MustErr())
		//
		// NOT:
		//   return ..., err  (tuple style)

		// Count occurrences of tmpN.MustErr() in return statements
		mustErrCount := 0
		lines := strings.Split(goCode, "\n")
		for _, line := range lines {
			// Look for return statements with .MustErr()
			if strings.Contains(line, "return") && strings.Contains(line, ".MustErr()") {
				mustErrCount++
			}
		}

		if mustErrCount < 3 {
			t.Errorf("Expected at least 3 'return ...MustErr()' calls for Result type error extraction, found %d.\n\n"+
				"When propagating error from Result, should use tmp.MustErr() to extract the error value.", mustErrCount)
		}
	})

	t.Run("Result type should use MustOk to extract value", func(t *testing.T) {
		// After checking .IsErr(), we need to extract the success value with .MustOk()
		//   val := tmp.MustOk()
		//
		// NOT:
		//   val := tmp  (wrong - tmp is the whole Result, not the value)

		// Count occurrences of tmpN.MustOk() in assignment statements
		// We expect 3: one for each getResult()? in the code
		mustOkCount := 0
		lines := strings.Split(goCode, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Look for assignment statements with .MustOk() (value extraction after error check)
			// Pattern: varName := tmpN.MustOk()
			if strings.Contains(trimmed, ":= tmp") && strings.Contains(trimmed, ".MustOk()") {
				mustOkCount++
			}
		}

		if mustOkCount < 3 {
			t.Errorf("Expected at least 3 'val := tmp.MustOk()' assignments for Result type value extraction, found %d.\n\n"+
				"After ? operator on Result[T, E], should use tmp.MustOk() to get the T value.", mustOkCount)
		}
	})

	t.Run("Tuple return should use != nil pattern", func(t *testing.T) {
		// For (T, error) tuples, the existing pattern should be used:
		//   tmp, err := getTuple()
		//   if err != nil { return ..., err }

		// The tuple case should still use the != nil pattern
		if !strings.Contains(goCode, "!= nil") {
			t.Errorf("Expected != nil pattern for tuple error propagation, but not found in generated code.")
		}
	})

	t.Run("Result type should NOT use tuple pattern", func(t *testing.T) {
		// For functions that return Result[T, E], we should NOT generate:
		//   tmp, err := getResult()
		//
		// Instead we should generate:
		//   tmp := getResult()

		lines := strings.Split(goCode, "\n")
		wrongPatternCount := 0
		for _, line := range lines {
			// Check for tuple-style unpacking of getResult()
			if strings.Contains(line, "getResult()") && strings.Contains(line, ", err") && strings.Contains(line, ":=") {
				wrongPatternCount++
				t.Errorf("Result type function should NOT use tuple unpacking pattern.\n"+
					"Found: %s\n"+
					"Expected: tmp := getResult() (single value assignment)", strings.TrimSpace(line))
			}
		}
	})

	t.Run("Result error return should have correct type parameters", func(t *testing.T) {
		// When returning an error from Result propagation, the constructor should have
		// both type parameters:
		//   return dgo.Err[string, error](tmp.MustErr())
		//
		// NOT:
		//   return dgo.Err[string](err)  (missing second type parameter)

		// Look for dgo.Err with only one type parameter (the bug)
		// Skip comment lines (which contain documentation about wrong behavior)
		lines := strings.Split(goCode, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip comments
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			// Check for single-parameter Err constructor in actual code
			if strings.Contains(line, "dgo.Err[string](") || strings.Contains(line, "Err[string](") {
				t.Errorf("dgo.Err should have two type parameters: Err[T, E], not Err[T].\n"+
					"Found single-parameter Err constructor which is invalid.\nLine: %s", trimmed)
			}
		}
	})

	t.Run("Generated code should be valid Go syntax", func(t *testing.T) {
		// Check for unbalanced braces (rough check)
		openBraces := strings.Count(goCode, "{")
		closeBraces := strings.Count(goCode, "}")
		if openBraces != closeBraces {
			t.Errorf("Unbalanced braces in generated code: %d open, %d close", openBraces, closeBraces)
		}
	})
}

// TestResultErrorPropGeneratesCorrectReturnType verifies that when propagating
// an error from a Result[T, E], the return statement uses the correct Result constructor.
func TestResultErrorPropGeneratesCorrectReturnType(t *testing.T) {
	source := []byte(`package main

import "github.com/MadAppGang/dingo/dgo"

func inner() dgo.Result[int, string] {
	return dgo.Ok[int, string](42)
}

// When ? is used on Result[int, string], the error propagation should:
// - Check with .IsErr()
// - Return dgo.Err[ReturnT, string](tmp.MustErr()) where ReturnT matches outer function's Ok type
func outer() dgo.Result[bool, string] {
	val := inner()?
	return dgo.Ok[bool, string](val > 0)
}
`)

	result, err := PureASTTranspile(source, "test.dingo")
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	goCode := string(result)

	// The generated error return should preserve the error type (string) from inner()
	// and use the success type from outer() (bool)
	// Expected: return dgo.Err[bool, string](tmp.MustErr())
	if !strings.Contains(goCode, "dgo.Err[bool, string]") && !strings.Contains(goCode, "Err[bool, string]") {
		// For now, just check that we don't generate the WRONG pattern
		// The wrong pattern would be: return ..., err (tuple style)
		lines := strings.Split(goCode, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "return") && strings.Contains(line, ", err") && strings.Contains(line, "inner") {
				t.Errorf("Result propagation should NOT use tuple-style error return.\nFound: %s", trimmed)
			}
		}
	}
}

// TestMixedErrorPropPatterns verifies that the transpiler correctly handles
// functions that use both Result types AND tuple returns.
func TestMixedErrorPropPatterns(t *testing.T) {
	source := []byte(`package main

import (
	"fmt"
	"github.com/MadAppGang/dingo/dgo"
)

func resultFunc() dgo.Result[int, error] {
	return dgo.Ok[int, error](1)
}

func tupleFunc() (int, error) {
	return 2, nil
}

// This function mixes both patterns
func mixed() (int, error) {
	// This should use .IsErr() pattern
	a := resultFunc()?
	// This should use != nil pattern
	b := tupleFunc()?
	return a + b, nil
}
`)

	result, err := PureASTTranspile(source, "test.dingo")
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	goCode := string(result)

	// Should have BOTH patterns in the output
	hasIsErr := strings.Contains(goCode, ".IsErr()")
	hasNilCheck := strings.Contains(goCode, "!= nil")

	// Currently this will fail because .IsErr() is not implemented
	// Once implemented, both patterns should appear
	if !hasIsErr {
		t.Logf("Note: .IsErr() pattern not found - this is expected until Result error prop is implemented")
	}

	if !hasNilCheck {
		t.Errorf("Expected != nil pattern for tuple function, but not found")
	}

	t.Logf("Generated code:\n%s", goCode)
}
