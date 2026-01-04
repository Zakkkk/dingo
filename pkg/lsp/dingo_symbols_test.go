package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
)

// mockLogger implements Logger for testing
type mockLogger struct {
	debugLogs []string
	warnLogs  []string
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	// Silent for tests
}

func (m *mockLogger) Infof(format string, args ...interface{}) {
	// Silent for tests
}

func (m *mockLogger) Warnf(format string, args ...interface{}) {
	// Silent for tests
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	// Silent for tests
}

func (m *mockLogger) Fatalf(format string, args ...interface{}) {
	// Silent for tests
}

func TestDingoSymbolExtractor_EnumDeclaration(t *testing.T) {
	source := `package main

enum Color {
	Red
	Green
	Blue
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	// Should find one enum symbol
	if len(symbols) != 1 {
		t.Fatalf("Expected 1 symbol, got %d", len(symbols))
	}

	enumSym := symbols[0]
	if enumSym.Name != "Color" {
		t.Errorf("Expected enum name 'Color', got %q", enumSym.Name)
	}
	if enumSym.Kind != protocol.SymbolKindEnum {
		t.Errorf("Expected SymbolKindEnum, got %v", enumSym.Kind)
	}

	// Should have 3 variant children
	if len(enumSym.Children) != 3 {
		t.Fatalf("Expected 3 variant children, got %d", len(enumSym.Children))
	}

	expectedVariants := []string{"Red", "Green", "Blue"}
	for i, child := range enumSym.Children {
		if child.Name != expectedVariants[i] {
			t.Errorf("Expected variant %q, got %q", expectedVariants[i], child.Name)
		}
		if child.Kind != protocol.SymbolKindEnumMember {
			t.Errorf("Expected SymbolKindEnumMember for %q, got %v", child.Name, child.Kind)
		}
	}
}

func TestDingoSymbolExtractor_EnumWithTupleVariants(t *testing.T) {
	source := `package main

enum Result {
	Ok(T)
	Err(E)
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	if len(symbols) != 1 {
		t.Fatalf("Expected 1 symbol, got %d", len(symbols))
	}

	enumSym := symbols[0]
	if enumSym.Name != "Result" {
		t.Errorf("Expected enum name 'Result', got %q", enumSym.Name)
	}

	// Should have 2 variant children
	if len(enumSym.Children) != 2 {
		t.Fatalf("Expected 2 variant children, got %d", len(enumSym.Children))
	}

	// Check variants are marked as tuple variants
	for _, child := range enumSym.Children {
		if child.Detail != "tuple variant" {
			t.Errorf("Expected 'tuple variant' detail for %q, got %q", child.Name, child.Detail)
		}
	}
}

func TestDingoSymbolExtractor_EnumWithStructVariants(t *testing.T) {
	source := `package main

enum Event {
	Click { x: int, y: int }
	KeyPress { key: string }
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	if len(symbols) != 1 {
		t.Fatalf("Expected 1 symbol, got %d", len(symbols))
	}

	enumSym := symbols[0]

	// Should have 2 variant children
	if len(enumSym.Children) != 2 {
		t.Fatalf("Expected 2 variant children, got %d", len(enumSym.Children))
	}

	// Check variants are marked as struct variants
	for _, child := range enumSym.Children {
		if child.Detail != "struct variant" {
			t.Errorf("Expected 'struct variant' detail for %q, got %q", child.Name, child.Detail)
		}
	}
}

func TestDingoSymbolExtractor_MatchExpression(t *testing.T) {
	source := `package main

func handle(x int) string {
	return match x {
		1 => "one",
		2 => "two",
		_ => "other",
	}
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	// Should find at least one match symbol
	var matchSym *protocol.DocumentSymbol
	for i := range symbols {
		if symbols[i].Name == "match" {
			matchSym = &symbols[i]
			break
		}
	}

	if matchSym == nil {
		t.Fatalf("Expected to find match symbol")
	}

	if matchSym.Kind != protocol.SymbolKindOperator {
		t.Errorf("Expected SymbolKindOperator for match, got %v", matchSym.Kind)
	}

	// Should have match arm children
	if len(matchSym.Children) == 0 {
		t.Error("Expected match arms as children")
	}
}

func TestDingoSymbolExtractor_RustStyleLambda(t *testing.T) {
	source := `package main

func main() {
	f := |x| x * 2
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	// Should find lambda symbol
	var lambdaSym *protocol.DocumentSymbol
	for i := range symbols {
		if symbols[i].Kind == protocol.SymbolKindFunction && symbols[i].Detail == "lambda" {
			lambdaSym = &symbols[i]
			break
		}
	}

	if lambdaSym == nil {
		t.Fatalf("Expected to find lambda symbol")
	}

	if lambdaSym.Kind != protocol.SymbolKindFunction {
		t.Errorf("Expected SymbolKindFunction for lambda, got %v", lambdaSym.Kind)
	}
}

func TestMergeDingoSymbols_ReplacesBadDecl(t *testing.T) {
	// Simulate gopls returning a BadDecl placeholder for enum
	goplsSymbols := []protocol.DocumentSymbol{
		{
			Name: "package main",
			Kind: protocol.SymbolKindPackage,
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 12},
			},
		},
	}

	// Dingo extractor found an enum
	dingoSymbols := []protocol.DocumentSymbol{
		{
			Name: "Color",
			Kind: protocol.SymbolKindEnum,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 0},
				End:   protocol.Position{Line: 5, Character: 1},
			},
			Children: []protocol.DocumentSymbol{
				{Name: "Red", Kind: protocol.SymbolKindEnumMember},
				{Name: "Green", Kind: protocol.SymbolKindEnumMember},
				{Name: "Blue", Kind: protocol.SymbolKindEnumMember},
			},
		},
	}

	merged := MergeDingoSymbols(goplsSymbols, dingoSymbols)

	// Should have both package and enum
	if len(merged) != 2 {
		t.Fatalf("Expected 2 merged symbols, got %d", len(merged))
	}

	// Check that enum has variants as children
	var enumFound bool
	for _, sym := range merged {
		if sym.Kind == protocol.SymbolKindEnum {
			enumFound = true
			if len(sym.Children) != 3 {
				t.Errorf("Expected 3 enum children, got %d", len(sym.Children))
			}
		}
	}

	if !enumFound {
		t.Error("Expected to find enum symbol in merged result")
	}
}

func TestDingoSymbolExtractor_EmptyFile(t *testing.T) {
	source := ``
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	if len(symbols) != 0 {
		t.Errorf("Expected 0 symbols for empty file, got %d", len(symbols))
	}
}

func TestDingoSymbolExtractor_NoEnumOrMatch(t *testing.T) {
	source := `package main

func main() {
	x := 42
	fmt.Println(x)
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	// Should not find any Dingo-specific symbols
	// (gopls will handle the regular function)
	for _, sym := range symbols {
		if sym.Kind == protocol.SymbolKindEnum || sym.Name == "match" {
			t.Errorf("Unexpected Dingo-specific symbol found: %s", sym.Name)
		}
	}
}

func TestDingoSymbolExtractor_MultipleEnums(t *testing.T) {
	source := `package main

enum Color {
	Red
	Green
}

enum Status {
	Active
	Inactive
}
`
	logger := &mockLogger{}
	extractor := NewDingoSymbolExtractor(logger)

	symbols, err := extractor.ExtractDingoSymbols([]byte(source), "file:///test.dingo")
	if err != nil {
		t.Fatalf("ExtractDingoSymbols failed: %v", err)
	}

	// Should find two enum symbols
	enumCount := 0
	for _, sym := range symbols {
		if sym.Kind == protocol.SymbolKindEnum {
			enumCount++
		}
	}

	if enumCount != 2 {
		t.Errorf("Expected 2 enum symbols, got %d", enumCount)
	}
}
