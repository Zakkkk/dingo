package builtin

import "testing"

func TestSanitizeTypeName(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		// Single built-in types (camelCase)
		{
			name:     "int",
			parts:    []string{"int"},
			expected: "Int",
		},
		{
			name:     "string",
			parts:    []string{"string"},
			expected: "String",
		},
		{
			name:     "error",
			parts:    []string{"error"},
			expected: "Error",
		},
		{
			name:     "bool",
			parts:    []string{"bool"},
			expected: "Bool",
		},
		{
			name:     "any → interface",
			parts:    []string{"any"},
			expected: "Interface",
		},

		// Two-part type names (camelCase)
		{
			name:     "int + error",
			parts:    []string{"int", "error"},
			expected: "IntError",
		},
		{
			name:     "string + option",
			parts:    []string{"string", "option"},
			expected: "StringOption",
		},
		{
			name:     "any + error",
			parts:    []string{"any", "error"},
			expected: "InterfaceError",
		},

		// User-defined types (preserve capitalization)
		{
			name:     "User",
			parts:    []string{"User"},
			expected: "User",
		},
		{
			name:     "CustomError",
			parts:    []string{"CustomError"},
			expected: "CustomError",
		},
		{
			name:     "UserID",
			parts:    []string{"UserID"},
			expected: "UserID",
		},

		// User types in compound names
		{
			name:     "CustomError + int",
			parts:    []string{"CustomError", "int"},
			expected: "CustomErrorInt",
		},
		{
			name:     "int + CustomError",
			parts:    []string{"int", "CustomError"},
			expected: "IntCustomError",
		},
		{
			name:     "UserID + error",
			parts:    []string{"UserID", "error"},
			expected: "UserIDError",
		},

		// Pointer types
		{
			name:     "*User",
			parts:    []string{"*User"},
			expected: "PtrUser",
		},
		{
			name:     "*int + error",
			parts:    []string{"*int", "error"},
			expected: "PtrIntError",
		},

		// Slice types
		{
			name:     "[]string",
			parts:    []string{"[]string"},
			expected: "SliceString",
		},
		{
			name:     "[]int + error",
			parts:    []string{"[]int", "error"},
			expected: "SliceIntError",
		},

		// Three-part names
		{
			name:     "int + string + error",
			parts:    []string{"int", "string", "error"},
			expected: "IntStringError",
		},

		// Edge cases
		{
			name:     "numeric types",
			parts:    []string{"int64", "error"},
			expected: "Int64Error",
		},
		{
			name:     "uint types",
			parts:    []string{"uint32"},
			expected: "Uint32",
		},
		{
			name:     "float types",
			parts:    []string{"float64", "error"},
			expected: "Float64Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeTypeName(tt.parts...)
			if result != tt.expected {
				t.Errorf("SanitizeTypeName(%v) = %q, want %q",
					tt.parts, result, tt.expected)
			}
		})
	}
}

func TestGenerateTempVarName(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		index    int
		expected string
	}{
		// First variable (no number suffix)
		{
			name:     "ok first",
			base:     "ok",
			index:    0,
			expected: "ok",
		},
		{
			name:     "err first",
			base:     "err",
			index:    0,
			expected: "err",
		},
		{
			name:     "tmp first",
			base:     "tmp",
			index:    0,
			expected: "tmp",
		},

		// Second variable (add number)
		{
			name:     "ok second",
			base:     "ok",
			index:    1,
			expected: "ok1",
		},
		{
			name:     "err second",
			base:     "err",
			index:    1,
			expected: "err1",
		},
		{
			name:     "tmp second",
			base:     "tmp",
			index:    1,
			expected: "tmp1",
		},

		// Third variable
		{
			name:     "ok third",
			base:     "ok",
			index:    2,
			expected: "ok2",
		},
		{
			name:     "err third",
			base:     "err",
			index:    2,
			expected: "err2",
		},

		// Higher indices
		{
			name:     "ok tenth",
			base:     "ok",
			index:    9,
			expected: "ok9",
		},
		{
			name:     "err twentieth",
			base:     "err",
			index:    19,
			expected: "err19",
		},

		// Different base names
		{
			name:     "val first",
			base:     "val",
			index:    0,
			expected: "val",
		},
		{
			name:     "val second",
			base:     "val",
			index:    1,
			expected: "val1",
		},
		{
			name:     "result first",
			base:     "result",
			index:    0,
			expected: "result",
		},
		{
			name:     "result second",
			base:     "result",
			index:    1,
			expected: "result1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTempVarName(tt.base, tt.index)
			if result != tt.expected {
				t.Errorf("GenerateTempVarName(%q, %d) = %q, want %q",
					tt.base, tt.index, result, tt.expected)
			}
		})
	}
}

func TestSanitizeTypeComponent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Built-in types (capitalize for camelCase)
		{
			name:     "int",
			input:    "int",
			expected: "Int",
		},
		{
			name:     "string",
			input:    "string",
			expected: "String",
		},
		{
			name:     "error",
			input:    "error",
			expected: "Error",
		},
		{
			name:     "any → Interface",
			input:    "any",
			expected: "Interface",
		},
		{
			name:     "interface{} → Interface",
			input:    "interface{}",
			expected: "Interface",
		},

		// Pointer types
		{
			name:     "*User",
			input:    "*User",
			expected: "PtrUser",
		},
		{
			name:     "*int",
			input:    "*int",
			expected: "PtrInt",
		},

		// Slice types
		{
			name:     "[]string",
			input:    "[]string",
			expected: "SliceString",
		},
		{
			name:     "[]int",
			input:    "[]int",
			expected: "SliceInt",
		},

		// User types (preserve case)
		{
			name:     "User",
			input:    "User",
			expected: "User",
		},
		{
			name:     "CustomError",
			input:    "CustomError",
			expected: "CustomError",
		},

		// Underscore-separated names (convert to camelCase)
		{
			name:     "Option_int → OptionInt",
			input:    "Option_int",
			expected: "OptionInt",
		},
		{
			name:     "Result_int_error → ResultIntError",
			input:    "Result_int_error",
			expected: "ResultIntError",
		},
		{
			name:     "Option_string → OptionString",
			input:    "Option_string",
			expected: "OptionString",
		},
		{
			name:     "Option_User → OptionUser",
			input:    "Option_User",
			expected: "OptionUser",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTypeComponent(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTypeComponent(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Underscore-separated names get converted to camelCase
		{
			name:     "Option_int → OptionInt",
			input:    "Option_int",
			expected: "OptionInt",
		},
		{
			name:     "Result_int_error → ResultIntError",
			input:    "Result_int_error",
			expected: "ResultIntError",
		},
		{
			name:     "Option_User → OptionUser",
			input:    "Option_User",
			expected: "OptionUser",
		},

		// Basic types are preserved
		{
			name:     "int preserved",
			input:    "int",
			expected: "int",
		},
		{
			name:     "string preserved",
			input:    "string",
			expected: "string",
		},
		{
			name:     "error preserved",
			input:    "error",
			expected: "error",
		},

		// User types without underscores are preserved
		{
			name:     "User preserved",
			input:    "User",
			expected: "User",
		},
		{
			name:     "CustomError preserved",
			input:    "CustomError",
			expected: "CustomError",
		},

		// Pointer types
		{
			name:     "*Option_int → *OptionInt",
			input:    "*Option_int",
			expected: "*OptionInt",
		},
		{
			name:     "*int preserved",
			input:    "*int",
			expected: "*int",
		},

		// Slice types
		{
			name:     "[]Option_int → []OptionInt",
			input:    "[]Option_int",
			expected: "[]OptionInt",
		},
		{
			name:     "[]int preserved",
			input:    "[]int",
			expected: "[]int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTypeName(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}
