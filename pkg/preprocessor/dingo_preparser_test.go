package preprocessor

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

func TestDingoPreParser_SimpleLetDeclaration(t *testing.T) {
	source := `package main

func main() {
	let x = 5
	println(x)
}
`

	expected := `package main

func main() {
	x := 5
	println(x)
}
`

	parser := NewDingoPreParser()
	result, mappings, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(result))
	}

	// DingoPreParser doesn't generate mappings (uses metadata-based system)
	if mappings != nil {
		t.Errorf("Expected nil mappings, got %d mappings", len(mappings))
	}

	// Check nodes
	nodes := parser.GetNodes()
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
}

func TestDingoPreParser_TypedLetDeclaration(t *testing.T) {
	source := `package main

func main() {
	let x: int = 5
	println(x)
}
`

	expected := `package main

func main() {
	var x int = 5
	println(x)
}
`

	parser := NewDingoPreParser()
	result, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(result))
	}
}

func TestDingoPreParser_MultipleNames(t *testing.T) {
	source := `package main

func main() {
	let a, b = getValues()
	println(a, b)
}
`

	expected := `package main

func main() {
	a, b := getValues ()
	println(a, b)
}
`

	parser := NewDingoPreParser()
	result, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(result))
	}
}

func TestDingoPreParser_DeclarationOnly(t *testing.T) {
	source := `package main

func main() {
	let x: int
	x = 10
	println(x)
}
`

	expected := `package main

func main() {
	var x int
	x = 10
	println(x)
}
`

	parser := NewDingoPreParser()
	result, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(result))
	}
}

func TestDingoPreParser_ComplexTypes(t *testing.T) {
	source := `package main

func main() {
	let opt: Option<int> = None
	let result: Result<string, Error> = Ok("hello")
	println(opt, result)
}
`

	expected := `package main

func main() {
	var opt Option < int > = None
	var result Result < string , Error > = Ok ("hello")
	println(opt, result)
}
`

	parser := NewDingoPreParser()
	result, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Normalize whitespace for comparison (lexer adds spaces around tokens)
	normalizeWS := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}

	resultNorm := normalizeWS(string(result))
	expectedNorm := normalizeWS(expected)

	if resultNorm != expectedNorm {
		t.Errorf("Expected (normalized):\n%s\n\nGot (normalized):\n%s", expectedNorm, resultNorm)
	}
}

func TestDingoPreParser_PreservesNonLetCode(t *testing.T) {
	source := `package main

import "fmt"

func main() {
	let x = 5
	fmt.Println(x)
	y := 10 // Regular Go code
	fmt.Println(y)
}
`

	parser := NewDingoPreParser()
	result, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	resultStr := string(result)

	// Check that non-let code is preserved
	if !strings.Contains(resultStr, "import \"fmt\"") {
		t.Error("Import statement not preserved")
	}
	if !strings.Contains(resultStr, "y := 10") {
		t.Error("Regular Go code not preserved")
	}
	if !strings.Contains(resultStr, "// Regular Go code") {
		t.Error("Comment not preserved")
	}

	// Check that let was transformed
	if strings.Contains(resultStr, "let x") {
		t.Error("let keyword not transformed")
	}
	if !strings.Contains(resultStr, "x := 5") {
		t.Error("let x = 5 not transformed to x := 5")
	}
}

func TestDingoPreParser_IndentedLet(t *testing.T) {
	source := `package main

func main() {
	if true {
		let x = 5
		println(x)
	}
}
`

	expected := `package main

func main() {
	if true {
		x := 5
		println(x)
	}
}
`

	parser := NewDingoPreParser()
	result, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(result) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(result))
	}
}

func TestDingoPreParser_Mappings(t *testing.T) {
	source := `package main

let x = 5
let y = 10
`

	parser := NewDingoPreParser()
	_, mappings, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// DingoPreParser doesn't generate mappings (uses metadata-based system)
	// Simple transformations like let → var don't need source maps
	if mappings != nil {
		t.Errorf("Expected nil mappings, got %d mappings", len(mappings))
	}
}

func TestDingoPreParser_MappingType(t *testing.T) {
	source := `let x = 5`

	parser := NewDingoPreParser()
	_, mappings, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// DingoPreParser doesn't generate mappings (uses metadata-based system)
	if mappings != nil {
		t.Errorf("Expected nil mappings, got %d mappings", len(mappings))
	}
}

func TestDingoPreParser_GetNodes(t *testing.T) {
	source := `package main

let x = 5
let y: int = 10
`

	parser := NewDingoPreParser()
	_, _, err := parser.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	nodes := parser.GetNodes()
	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	// Both nodes should be LetDecl
	for i, node := range nodes {
		if _, ok := node.(*ast.LetDecl); !ok {
			t.Errorf("Node %d is not a *ast.LetDecl", i)
		}
	}
}
