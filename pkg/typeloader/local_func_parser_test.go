package typeloader

import (
	"testing"
)

func TestParseLocalFunctions_SimpleFunction(t *testing.T) {
	source := []byte(`
package main

func readFile(path string) ([]byte, error) {
	return nil, nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(funcs))
	}

	sig, ok := funcs["readFile"]
	if !ok {
		t.Fatal("Function 'readFile' not found")
	}

	if sig.Name != "readFile" {
		t.Errorf("Expected name 'readFile', got '%s'", sig.Name)
	}

	if len(sig.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(sig.Results))
	}

	if sig.Results[0].Name != "[]byte" {
		t.Errorf("Expected first result '[]byte', got '%s'", sig.Results[0].Name)
	}

	if !sig.Results[1].IsError {
		t.Error("Expected second result to be error type")
	}
}

func TestParseLocalFunctions_DingoSyntax(t *testing.T) {
	source := []byte(`
package main

func getUserData(id int) (User, error) {
	data := fetchFromDB(id)?
	return data, nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["getUserData"]
	if !ok {
		t.Fatal("Function 'getUserData' not found")
	}

	if len(sig.Parameters) != 1 {
		t.Fatalf("Expected 1 parameter, got %d", len(sig.Parameters))
	}

	if sig.Parameters[0].Name != "int" {
		t.Errorf("Expected parameter type 'int', got '%s'", sig.Parameters[0].Name)
	}

	if len(sig.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(sig.Results))
	}

	if sig.Results[0].Name != "User" {
		t.Errorf("Expected first result 'User', got '%s'", sig.Results[0].Name)
	}

	if !sig.Results[1].IsError {
		t.Error("Expected second result to be error type")
	}
}

func TestParseLocalFunctions_MultipleParameters(t *testing.T) {
	source := []byte(`
package main

func processData(name: string, age: int, active: bool) string {
	return name
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["processData"]
	if !ok {
		t.Fatal("Function 'processData' not found")
	}

	if len(sig.Parameters) != 3 {
		t.Fatalf("Expected 3 parameters, got %d", len(sig.Parameters))
	}

	expectedParams := []string{"string", "int", "bool"}
	for i, expected := range expectedParams {
		if sig.Parameters[i].Name != expected {
			t.Errorf("Parameter %d: expected '%s', got '%s'", i, expected, sig.Parameters[i].Name)
		}
	}

	if len(sig.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(sig.Results))
	}

	if sig.Results[0].Name != "string" {
		t.Errorf("Expected result 'string', got '%s'", sig.Results[0].Name)
	}
}

func TestParseLocalFunctions_NoReturnValue(t *testing.T) {
	source := []byte(`
package main

func printMessage(msg: string) {
	println(msg)
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["printMessage"]
	if !ok {
		t.Fatal("Function 'printMessage' not found")
	}

	if len(sig.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(sig.Results))
	}
}

func TestParseLocalFunctions_PointerTypes(t *testing.T) {
	source := []byte(`
package main

func getUser(id: int) (*User, error) {
	return &User{}, nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["getUser"]
	if !ok {
		t.Fatal("Function 'getUser' not found")
	}

	if len(sig.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(sig.Results))
	}

	if !sig.Results[0].IsPointer {
		t.Error("Expected first result to be pointer type")
	}

	if sig.Results[0].Name != "User" {
		t.Errorf("Expected first result name 'User', got '%s'", sig.Results[0].Name)
	}
}

func TestParseLocalFunctions_PackageQualifiedTypes(t *testing.T) {
	source := []byte(`
package main

import "encoding/json"

func parseJSON(data: []byte) (json.RawMessage, error) {
	return nil, nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["parseJSON"]
	if !ok {
		t.Fatal("Function 'parseJSON' not found")
	}

	if len(sig.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(sig.Results))
	}

	if sig.Results[0].Package != "json" {
		t.Errorf("Expected result package 'json', got '%s'", sig.Results[0].Package)
	}

	if sig.Results[0].Name != "RawMessage" {
		t.Errorf("Expected result name 'RawMessage', got '%s'", sig.Results[0].Name)
	}
}

func TestParseLocalFunctions_MethodWithReceiver(t *testing.T) {
	source := []byte(`
package main

type Server struct{}

func (s *Server) Start(port: int) error {
	return nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["Start"]
	if !ok {
		t.Fatal("Method 'Start' not found")
	}

	if sig.Receiver == nil {
		t.Fatal("Expected receiver, got nil")
	}

	if sig.Receiver.Name != "Server" {
		t.Errorf("Expected receiver type 'Server', got '%s'", sig.Receiver.Name)
	}

	if !sig.Receiver.IsPointer {
		t.Error("Expected receiver to be pointer type")
	}
}

func TestParseLocalFunctions_MultipleFunctions(t *testing.T) {
	source := []byte(`
package main

func funcA() string {
	return ""
}

func funcB(x: int) (int, error) {
	return x, nil
}

func funcC() {
	// no return
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	if len(funcs) != 3 {
		t.Fatalf("Expected 3 functions, got %d", len(funcs))
	}

	expectedFuncs := []string{"funcA", "funcB", "funcC"}
	for _, name := range expectedFuncs {
		if _, ok := funcs[name]; !ok {
			t.Errorf("Expected function '%s' not found", name)
		}
	}
}

func TestParseLocalFunctions_ErrorOnlyReturn(t *testing.T) {
	source := []byte(`
package main

func validateData(input: string) error {
	return nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig, ok := funcs["validateData"]
	if !ok {
		t.Fatal("Function 'validateData' not found")
	}

	if len(sig.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(sig.Results))
	}

	if !sig.Results[0].IsError {
		t.Error("Expected result to be error type")
	}
}

func TestNormalizeFuncDecls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple type annotation",
			input:    "func test(x: int) string {",
			expected: "func test(x int) string {",
		},
		{
			name:     "multiple parameters",
			input:    "func test(x: int, y: string) bool {",
			expected: "func test(x int, y string) bool {",
		},
		{
			name:     "no type annotations",
			input:    "func test(x int) string {",
			expected: "func test(x int) string {",
		},
		{
			name:     "colon in body should not be changed",
			input:    "func test() {\n\tx := map[string]int{\"key\": 1}\n}",
			expected: "func test() {\n\tx := map[string]int{\"key\": 1}\n}",
		},
	}

	parser := &LocalFuncParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(parser.normalizeFuncDecls([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestExtractFuncsRegex_Fallback(t *testing.T) {
	// Test regex fallback when Go parser fails
	source := []byte(`
package main

func validFunc() error {
	return nil
}

func withDingoSyntax() (User, error) {
	x := doSomething()?
	return x, nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.extractFuncsRegex(source)
	if err != nil {
		t.Fatalf("extractFuncsRegex failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Fatalf("Expected 2 functions, got %d", len(funcs))
	}

	sig1, ok := funcs["validFunc"]
	if !ok {
		t.Fatal("Function 'validFunc' not found")
	}

	if len(sig1.Results) != 1 || !sig1.Results[0].IsError {
		t.Error("Expected 'validFunc' to return error")
	}

	sig2, ok := funcs["withDingoSyntax"]
	if !ok {
		t.Fatal("Function 'withDingoSyntax' not found")
	}

	if len(sig2.Results) != 2 {
		t.Errorf("Expected 'withDingoSyntax' to have 2 results, got %d", len(sig2.Results))
	}
}

func TestParseLocalFunctions_SliceAndMapTypes(t *testing.T) {
	source := []byte(`
package main

func getItems() []string {
	return nil
}

func getConfig() map[string]int {
	return nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	sig1, ok := funcs["getItems"]
	if !ok {
		t.Fatal("Function 'getItems' not found")
	}

	if sig1.Results[0].Name != "[]string" {
		t.Errorf("Expected result '[]string', got '%s'", sig1.Results[0].Name)
	}

	sig2, ok := funcs["getConfig"]
	if !ok {
		t.Fatal("Function 'getConfig' not found")
	}

	if sig2.Results[0].Name != "map[string]int" {
		t.Errorf("Expected result 'map[string]int', got '%s'", sig2.Results[0].Name)
	}
}

func TestParseLocalFunctions_UnexportedFunctions(t *testing.T) {
	source := []byte(`
package main

func ExportedFunc() error {
	return nil
}

func unexportedFunc() error {
	return nil
}
`)

	parser := &LocalFuncParser{}
	funcs, err := parser.ParseLocalFunctions(source)
	if err != nil {
		t.Fatalf("ParseLocalFunctions failed: %v", err)
	}

	// Both exported and unexported functions should be included
	// (they can be called locally)
	if len(funcs) != 2 {
		t.Fatalf("Expected 2 functions, got %d", len(funcs))
	}

	if _, ok := funcs["ExportedFunc"]; !ok {
		t.Error("ExportedFunc not found")
	}

	if _, ok := funcs["unexportedFunc"]; !ok {
		t.Error("unexportedFunc not found")
	}
}
