package registry

import (
	"go/ast"
	"go/token"
)

// TypeKind represents the category of a type
type TypeKind int

const (
	// TypeKindUnknown represents an unresolved or unknown type
	TypeKindUnknown TypeKind = iota
	// TypeKindResult represents a Result[T, E] type
	TypeKindResult
	// TypeKindOption represents an Option[T] type
	TypeKindOption
	// TypeKindBasic represents a basic Go type (int, string, etc.)
	TypeKindBasic
	// TypeKindNamed represents a named type (struct, interface, etc.)
	TypeKindNamed
	// TypeKindGeneric represents a generic type parameter
	TypeKindGeneric
)

// String returns a human-readable representation of TypeKind
func (k TypeKind) String() string {
	switch k {
	case TypeKindResult:
		return "Result"
	case TypeKindOption:
		return "Option"
	case TypeKindBasic:
		return "Basic"
	case TypeKindNamed:
		return "Named"
	case TypeKindGeneric:
		return "Generic"
	default:
		return "Unknown"
	}
}

// TypeInfo represents complete type information for a value
type TypeInfo struct {
	// Kind is the category of this type
	Kind TypeKind

	// Name is the type name (e.g., "int", "string", "MyStruct")
	Name string

	// ValueType is the inner type for Result[T,E] and Option[T]
	// For Result[string, error], ValueType = "string"
	// For Option[int], ValueType = "int"
	ValueType string

	// ErrorType is the error type for Result[T,E]
	// For Result[string, MyError], ErrorType = "MyError"
	ErrorType string

	// Package is the package path for named types
	Package string

	// IsPointer indicates if this is a pointer type
	IsPointer bool

	// Underlying is the AST type expression (for complex types)
	Underlying ast.Expr
}

// VariableInfo represents information about a declared variable
type VariableInfo struct {
	// Name is the variable identifier
	Name string

	// Type is the complete type information
	Type TypeInfo

	// Scope is the scope level where this variable was declared
	Scope int

	// Position is the AST position of the declaration
	Position token.Pos

	// IsParameter indicates if this variable is a function parameter
	IsParameter bool

	// IsReceiver indicates if this variable is a method receiver
	IsReceiver bool
}

// FunctionInfo represents information about a declared function
type FunctionInfo struct {
	// Name is the function identifier
	Name string

	// Package is the package this function belongs to
	Package string

	// Parameters is the list of parameter types
	Parameters []TypeInfo

	// Results is the list of return types
	Results []TypeInfo

	// Receiver is the method receiver type (nil for functions)
	Receiver *TypeInfo

	// Position is the AST position of the declaration
	Position token.Pos
}

// IsResult returns true if this TypeInfo represents a Result[T,E] type
func (t TypeInfo) IsResult() bool {
	return t.Kind == TypeKindResult
}

// IsOption returns true if this TypeInfo represents an Option[T] type
func (t TypeInfo) IsOption() bool {
	return t.Kind == TypeKindOption
}

// IsMonadic returns true if this TypeInfo represents a monadic type (Result or Option)
func (t TypeInfo) IsMonadic() bool {
	return t.IsResult() || t.IsOption()
}

// HasError returns true if this is a Result type with an error component
func (t TypeInfo) HasError() bool {
	return t.Kind == TypeKindResult && t.ErrorType != ""
}

// String returns a human-readable representation of TypeInfo
func (t TypeInfo) String() string {
	switch t.Kind {
	case TypeKindResult:
		if t.ErrorType != "" {
			return "Result[" + t.ValueType + ", " + t.ErrorType + "]"
		}
		return "Result[" + t.ValueType + "]"
	case TypeKindOption:
		return "Option[" + t.ValueType + "]"
	case TypeKindBasic, TypeKindNamed:
		if t.IsPointer {
			return "*" + t.Name
		}
		return t.Name
	case TypeKindGeneric:
		return t.Name
	default:
		return "unknown"
	}
}
