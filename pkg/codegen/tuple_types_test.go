package codegen

import (
	"go/types"
	"strings"
	"testing"
)

func TestTupleTypeResolver_BasicTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		input    string // Marker-infused Go source
		want     string // Expected output
		wantErr  bool
	}{
		{
			name: "simple int tuple",
			input: `package main
func main() {
	x := __tuple2__(10, 20)
}`,
			want: "Tuple2IntInt",
		},
		{
			name: "mixed types",
			input: `package main
func main() {
	x := __tuple2__("hello", 42)
}`,
			want: "Tuple2StringInt",
		},
		{
			name: "three elements",
			input: `package main
func main() {
	x := __tuple3__(1, 2.5, true)
}`,
			want: "Tuple3IntFloat64Bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewTupleTypeResolver([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTupleTypeResolver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			result, err := resolver.Resolve([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := string(result.Output)
			if !strings.Contains(output, tt.want) {
				t.Errorf("Expected output to contain %q, got:\n%s", tt.want, output)
			}
		})
	}
}

func TestTupleTypeResolver_TypeDeduplication(t *testing.T) {
	input := `package main
func main() {
	a := __tuple2__(1, 2)
	b := __tuple2__(3, 4)
	c := __tuple2__(5, 6)
}`

	resolver, err := NewTupleTypeResolver([]byte(input))
	if err != nil {
		t.Fatalf("NewTupleTypeResolver() error = %v", err)
	}

	result, err := resolver.Resolve([]byte(input))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	output := string(result.Output)

	// Should have exactly one Tuple2IntInt definition
	count := strings.Count(output, "type Tuple2IntInt struct")
	if count != 1 {
		t.Errorf("Expected exactly 1 Tuple2IntInt definition, got %d", count)
	}
}

func TestTupleTypeResolver_Destructuring(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantVars  []string // Variables that should be assigned
		skipVars  []string // Wildcards that should NOT be assigned
	}{
		{
			name: "basic destructuring",
			input: `package main
func main() {
	__tupleDest2__("x", "y", point)
}`,
			wantVars: []string{"x", "y"},
		},
		{
			name: "wildcard destructuring",
			input: `package main
func main() {
	__tupleDest3__("x", "_", "z", triple)
}`,
			wantVars: []string{"x", "z"},
			skipVars: []string{"_"},
		},
		{
			name: "all wildcards",
			input: `package main
func main() {
	__tupleDest2__("_", "_", pair)
}`,
			wantVars: []string{},
			skipVars: []string{"_"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewTupleTypeResolver([]byte(tt.input))
			if err != nil {
				t.Fatalf("NewTupleTypeResolver() error = %v", err)
			}

			result, err := resolver.Resolve([]byte(tt.input))
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			output := string(result.Output)

			// Check that expected variables are assigned
			for _, varName := range tt.wantVars {
				if !strings.Contains(output, varName+" :=") && !strings.Contains(output, varName+",") {
					t.Errorf("Expected variable %q to be assigned in output", varName)
				}
			}

			// Check that wildcards are NOT assigned
			for _, wildcard := range tt.skipVars {
				if wildcard == "_" {
					// Should have tmp := expr but no assignments to _
					if strings.Contains(output, "_ :=") {
						t.Errorf("Wildcard should not be assigned, but found '_ :=' in output")
					}
				}
			}

			// Should always have tmp variable
			if !strings.Contains(output, "tmp :=") {
				t.Errorf("Expected 'tmp :=' in destructuring output")
			}
		})
	}
}

func TestTypeToNameComponent(t *testing.T) {
	tests := []struct {
		typeStr  string
		expected string
	}{
		{"int", "Int"},
		{"string", "String"},
		{"bool", "Bool"},
		{"float64", "Float64"},
		{"*int", "PtrInt"},
		{"[]string", "SliceString"},
		{"interface{}", "Any"},
	}

	for _, tt := range tests {
		t.Run(tt.typeStr, func(t *testing.T) {
			// Create a basic type for testing
			var typ types.Type
			switch tt.typeStr {
			case "int":
				typ = types.Typ[types.Int]
			case "string":
				typ = types.Typ[types.String]
			case "bool":
				typ = types.Typ[types.Bool]
			case "float64":
				typ = types.Typ[types.Float64]
			case "*int":
				typ = types.NewPointer(types.Typ[types.Int])
			case "[]string":
				typ = types.NewSlice(types.Typ[types.String])
			case "interface{}":
				typ = types.NewInterfaceType(nil, nil)
			default:
				t.Skip("Type not implemented in test")
			}

			result := typeToNameComponent(typ)
			if result != tt.expected {
				t.Errorf("typeToNameComponent(%s) = %q, want %q", tt.typeStr, result, tt.expected)
			}
		})
	}
}

func TestGenerateStructName(t *testing.T) {
	tests := []struct {
		name      string
		elemTypes []types.Type
		expected  string
	}{
		{
			name:      "two ints",
			elemTypes: []types.Type{types.Typ[types.Int], types.Typ[types.Int]},
			expected:  "Tuple2IntInt",
		},
		{
			name:      "string and int",
			elemTypes: []types.Type{types.Typ[types.String], types.Typ[types.Int]},
			expected:  "Tuple2StringInt",
		},
		{
			name: "pointer and slice",
			elemTypes: []types.Type{
				types.NewPointer(types.Typ[types.Int]),
				types.NewSlice(types.Typ[types.String]),
			},
			expected: "Tuple2PtrIntSliceString",
		},
		{
			name: "three elements",
			elemTypes: []types.Type{
				types.Typ[types.Int],
				types.Typ[types.Float64],
				types.Typ[types.Bool],
			},
			expected: "Tuple3IntFloat64Bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &TupleTypeResolver{}
			result := resolver.generateStructName(tt.elemTypes)
			if result != tt.expected {
				t.Errorf("generateStructName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateStructDefinitions(t *testing.T) {
	resolver := &TupleTypeResolver{
		structs: []structDefinition{
			{
				name:     "Tuple2IntString",
				elemTypes: []string{"int", "string"},
			},
			{
				name:     "Tuple3BoolFloat64Int",
				elemTypes: []string{"bool", "float64", "int"},
			},
		},
	}

	result := resolver.generateStructDefinitions()

	// Check that both structs are defined
	if !strings.Contains(result, "type Tuple2IntString struct") {
		t.Error("Expected Tuple2IntString definition")
	}
	if !strings.Contains(result, "type Tuple3BoolFloat64Int struct") {
		t.Error("Expected Tuple3BoolFloat64Int definition")
	}

	// Check field definitions
	if !strings.Contains(result, "_0 int") {
		t.Error("Expected _0 int field in Tuple2IntString")
	}
	if !strings.Contains(result, "_1 string") {
		t.Error("Expected _1 string field in Tuple2IntString")
	}
	if !strings.Contains(result, "_2 int") {
		t.Error("Expected _2 int field in Tuple3BoolFloat64Int")
	}
}

func TestGetTypeSignature(t *testing.T) {
	tests := []struct {
		name      string
		elemTypes []types.Type
		expected  string
	}{
		{
			name:      "two ints",
			elemTypes: []types.Type{types.Typ[types.Int], types.Typ[types.Int]},
			expected:  "int,int",
		},
		{
			name:      "string and bool",
			elemTypes: []types.Type{types.Typ[types.String], types.Typ[types.Bool]},
			expected:  "string,bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &TupleTypeResolver{}
			result := resolver.getTypeSignature(tt.elemTypes)
			if result != tt.expected {
				t.Errorf("getTypeSignature() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExprToString(t *testing.T) {
	// This is a simplified test since we don't have a full parser setup
	// In practice, exprToString would be tested via integration tests
	t.Skip("exprToString requires full AST context - tested via integration")
}

func TestTupleTypeResolver_ComplexExpressions(t *testing.T) {
	input := `package main
func main() {
	result := __tuple3__(foo.Bar(), baz[0], someMap["key"])
}`

	resolver, err := NewTupleTypeResolver([]byte(input))
	if err != nil {
		t.Fatalf("NewTupleTypeResolver() error = %v", err)
	}

	result, err := resolver.Resolve([]byte(input))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	output := string(result.Output)

	// Should have a Tuple3 type (exact types depend on type inference)
	if !strings.Contains(output, "Tuple3") {
		t.Errorf("Expected Tuple3 type in output, got:\n%s", output)
	}

	// Should preserve the expressions
	if !strings.Contains(output, "foo.Bar()") {
		t.Errorf("Expected 'foo.Bar()' expression in output")
	}
	if !strings.Contains(output, "baz[0]") {
		t.Errorf("Expected 'baz[0]' expression in output")
	}
	if !strings.Contains(output, `someMap["key"]`) {
		t.Errorf("Expected 'someMap[\"key\"]' expression in output")
	}
}

func TestTupleTypeResolver_NestedTuples(t *testing.T) {
	input := `package main
func main() {
	nested := __tuple2__(__tuple2__(1, 2), 3)
}`

	resolver, err := NewTupleTypeResolver([]byte(input))
	if err != nil {
		t.Fatalf("NewTupleTypeResolver() error = %v", err)
	}

	result, err := resolver.Resolve([]byte(input))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	output := string(result.Output)

	// Should have both Tuple2IntInt (inner) and Tuple2Tuple2IntIntInt (outer)
	// Note: Exact naming depends on implementation details
	if !strings.Contains(output, "Tuple2") {
		t.Errorf("Expected Tuple2 types in output for nested tuples")
	}
}

func TestBasicTypeName(t *testing.T) {
	tests := []struct {
		kind     types.BasicKind
		expected string
	}{
		{types.Bool, "Bool"},
		{types.Int, "Int"},
		{types.Int8, "Int8"},
		{types.Int16, "Int16"},
		{types.Int32, "Int32"},
		{types.Int64, "Int64"},
		{types.Uint, "Uint"},
		{types.Uint8, "Uint8"},
		{types.Uint16, "Uint16"},
		{types.Uint32, "Uint32"},
		{types.Uint64, "Uint64"},
		{types.Float32, "Float32"},
		{types.Float64, "Float64"},
		{types.String, "String"},
		{types.UntypedInt, "Int"},
		{types.UntypedFloat, "Float64"},
		{types.UntypedString, "String"},
		{types.UntypedBool, "Bool"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			basicType := types.Typ[tt.kind]
			result := basicTypeName(basicType)
			if result != tt.expected {
				t.Errorf("basicTypeName(%v) = %q, want %q", tt.kind, result, tt.expected)
			}
		})
	}
}
