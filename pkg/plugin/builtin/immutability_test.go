package builtin

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

func TestImmutabilityPlugin_DirectReassignment(t *testing.T) {
	code := `package main

func main() {
	x := 42 // dingo:let:x
	x = 10
}
`
	ctx, fset := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected error for direct reassignment to let variable")
	}

	errs := ctx.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	errMsg := errs[0].Error()
	if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
		t.Errorf("Error message should mention immutable variable 'x', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "reassignment") {
		t.Errorf("Error message should mention reassignment, got: %s", errMsg)
	}

	_ = fset // Unused but may be needed for position checking
}

func TestImmutabilityPlugin_CompoundAssignment(t *testing.T) {
	testCases := []struct {
		name     string
		operator string
		code     string
	}{
		{"add_assign", "+=", `package main
func main() {
	x := 42 // dingo:let:x
	x += 1
}`},
		{"sub_assign", "-=", `package main
func main() {
	x := 42 // dingo:let:x
	x -= 1
}`},
		{"mul_assign", "*=", `package main
func main() {
	x := 42 // dingo:let:x
	x *= 2
}`},
		{"div_assign", "/=", `package main
func main() {
	x := 42 // dingo:let:x
	x /= 2
}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := setupImmutabilityTest(t, tc.code)
			if !ctx.HasErrors() {
				t.Fatalf("Expected error for compound assignment (%s) to let variable", tc.operator)
			}

			errs := ctx.GetErrors()
			if len(errs) != 1 {
				t.Fatalf("Expected 1 error, got %d", len(errs))
			}

			errMsg := errs[0].Error()
			if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
				t.Errorf("Error message should mention immutable variable 'x', got: %s", errMsg)
			}
			if !strings.Contains(errMsg, "compound assignment") {
				t.Errorf("Error message should mention compound assignment, got: %s", errMsg)
			}
		})
	}
}

func TestImmutabilityPlugin_Increment(t *testing.T) {
	testCases := []struct {
		name string
		code string
	}{
		{"increment", `package main
func main() {
	x := 42 // dingo:let:x
	x++
}`},
		{"decrement", `package main
func main() {
	x := 42 // dingo:let:x
	x--
}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := setupImmutabilityTest(t, tc.code)
			if !ctx.HasErrors() {
				t.Fatalf("Expected error for %s on let variable", tc.name)
			}

			errs := ctx.GetErrors()
			if len(errs) != 1 {
				t.Fatalf("Expected 1 error, got %d", len(errs))
			}

			errMsg := errs[0].Error()
			if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
				t.Errorf("Error message should mention immutable variable 'x', got: %s", errMsg)
			}
		})
	}
}

func TestImmutabilityPlugin_ClosureReassignment(t *testing.T) {
	code := `package main

func main() {
	x := 42 // dingo:let:x
	func() {
		x = 10
	}()
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected error for closure reassignment to let variable")
	}

	errs := ctx.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	errMsg := errs[0].Error()
	if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
		t.Errorf("Error message should mention immutable variable 'x', got: %s", errMsg)
	}
}

func TestImmutabilityPlugin_PointerDereference(t *testing.T) {
	code := `package main

func main() {
	y := 100
	x := &y // dingo:let:x
	*x = 10
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected error for pointer dereference mutation on let variable")
	}

	errs := ctx.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	errMsg := errs[0].Error()
	if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
		t.Errorf("Error message should mention immutable variable 'x', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pointer dereference") {
		t.Errorf("Error message should mention pointer dereference, got: %s", errMsg)
	}
}

func TestImmutabilityPlugin_InteriorMutability_FieldAssignment(t *testing.T) {
	code := `package main

type Person struct {
	Name string
	Age  int
}

func main() {
	x := Person{Name: "Alice", Age: 30} // dingo:let:x
	x.Name = "Bob"  // ALLOWED - interior mutability
	x.Age = 31      // ALLOWED - interior mutability
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if ctx.HasErrors() {
		t.Fatalf("Expected no errors for field assignment (interior mutability), got: %v", ctx.GetErrors())
	}
}

func TestImmutabilityPlugin_InteriorMutability_IndexAssignment(t *testing.T) {
	code := `package main

func main() {
	arr := make([]int, 5) // dingo:let:arr
	arr[0] = 10            // ALLOWED - interior mutability
	arr[4] = 99            // ALLOWED - interior mutability
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if ctx.HasErrors() {
		t.Fatalf("Expected no errors for index assignment (interior mutability), got: %v", ctx.GetErrors())
	}
}

func TestImmutabilityPlugin_InteriorMutability_MapAssignment(t *testing.T) {
	code := `package main

func main() {
	m := make(map[string]int) // dingo:let:m
	m["key"] = 100             // ALLOWED - interior mutability
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if ctx.HasErrors() {
		t.Fatalf("Expected no errors for map assignment (interior mutability), got: %v", ctx.GetErrors())
	}
}

func TestImmutabilityPlugin_MultipleVars(t *testing.T) {
	code := `package main

func main() {
	a, b := 1, 2 // dingo:let:a,b
	a = 3
	b = 4
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected errors for reassignment to multiple let variables")
	}

	errs := ctx.GetErrors()
	if len(errs) != 2 {
		t.Fatalf("Expected 2 errors (one for a, one for b), got %d", len(errs))
	}

	// Check both errors mention the correct variables
	errMsgs := make([]string, len(errs))
	for i, err := range errs {
		errMsgs[i] = err.Error()
	}

	hasErrorForA := false
	hasErrorForB := false
	for _, msg := range errMsgs {
		if strings.Contains(msg, "cannot modify immutable variable 'a'") {
			hasErrorForA = true
		}
		if strings.Contains(msg, "cannot modify immutable variable 'b'") {
			hasErrorForB = true
		}
	}

	if !hasErrorForA {
		t.Errorf("Expected error for variable 'a', got: %v", errMsgs)
	}
	if !hasErrorForB {
		t.Errorf("Expected error for variable 'b', got: %v", errMsgs)
	}
}

func TestImmutabilityPlugin_Shadowing(t *testing.T) {
	code := `package main

func main() {
	x := 42 // dingo:let:x
	{
		x := 10  // ALLOWED - shadowing (new variable)
		_ = x
	}
	_ = x
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if ctx.HasErrors() {
		t.Fatalf("Expected no errors for shadowing (new variable declaration), got: %v", ctx.GetErrors())
	}
}

func TestImmutabilityPlugin_VarReassignment(t *testing.T) {
	code := `package main

func main() {
	var x = 42  // NOT let, no marker
	x = 10       // ALLOWED - var is mutable
	x++          // ALLOWED - var is mutable
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if ctx.HasErrors() {
		t.Fatalf("Expected no errors for var reassignment (not let), got: %v", ctx.GetErrors())
	}
}

func TestImmutabilityPlugin_MixedLetAndVar(t *testing.T) {
	code := `package main

func main() {
	x := 42     // dingo:let:x
	var y = 100 // NOT let
	x = 1       // ERROR - x is immutable
	y = 200     // OK - y is mutable
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected error for reassignment to let variable (x)")
	}

	errs := ctx.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error (only for x), got %d", len(errs))
	}

	errMsg := errs[0].Error()
	if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
		t.Errorf("Error should be for variable 'x', got: %s", errMsg)
	}
}

func TestImmutabilityPlugin_NoMarkerNoError(t *testing.T) {
	code := `package main

func main() {
	x := 42  // No dingo:let marker
	x = 10   // ALLOWED - not declared with let
	x++      // ALLOWED - not declared with let
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if ctx.HasErrors() {
		t.Fatalf("Expected no errors when no let marker is present, got: %v", ctx.GetErrors())
	}
}

func TestImmutabilityPlugin_NestedPointerDereference(t *testing.T) {
	code := `package main

func main() {
	z := 100
	y := &z
	x := &y // dingo:let:x
	**x = 10
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected error for nested pointer dereference mutation")
	}

	errs := ctx.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	errMsg := errs[0].Error()
	if !strings.Contains(errMsg, "cannot modify immutable variable 'x'") {
		t.Errorf("Error message should mention immutable variable 'x', got: %s", errMsg)
	}
}

func TestImmutabilityPlugin_ComplexExpression(t *testing.T) {
	code := `package main

type Point struct {
	X, Y int
}

func main() {
	p := Point{X: 10, Y: 20} // dingo:let:p
	p.X = 30                  // ALLOWED - field mutation (interior mutability)
	p = Point{X: 40, Y: 50}   // ERROR - reassignment to p itself
}
`
	ctx, _ := setupImmutabilityTest(t, code)
	if !ctx.HasErrors() {
		t.Fatal("Expected error for reassignment to let variable")
	}

	errs := ctx.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error (reassignment to p), got %d", len(errs))
	}

	errMsg := errs[0].Error()
	if !strings.Contains(errMsg, "cannot modify immutable variable 'p'") {
		t.Errorf("Error message should mention immutable variable 'p', got: %s", errMsg)
	}
}

// setupImmutabilityTest parses code and runs ImmutabilityPlugin
func setupImmutabilityTest(t *testing.T, code string) (*plugin.Context, *token.FileSet) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:  fset,
		TypeInfo: nil, // Basic tests don't need type info
		Config: &plugin.Config{
			EmitGeneratedMarkers: false,
		},
		Logger: plugin.NewNoOpLogger(),
	}

	// Build parent map if needed (required for some analysis)
	ctx.BuildParentMap(file)

	p := NewImmutabilityPlugin()
	p.SetContext(ctx)

	if err := p.Process(file); err != nil {
		t.Fatalf("Plugin Process failed: %v", err)
	}

	return ctx, fset
}
