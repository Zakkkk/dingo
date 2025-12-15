package semantic

import (
	"go/types"
)

// SemanticKind identifies the type of semantic entity
type SemanticKind int

const (
	KindIdent    SemanticKind = iota // Variable, constant, function name
	KindCall                         // Function call expression
	KindField                        // Field access (x.Field)
	KindType                         // Type expression
	KindOperator                     // Dingo operators (?, ??, ?.)
	KindLambda                       // Lambda expression
	KindMatch                        // Match expression
)

// SemanticEntity represents a typed entity in Dingo source
type SemanticEntity struct {
	// Position in Dingo source (1-indexed)
	Line   int
	Col    int
	EndCol int

	// What kind of entity this is
	Kind SemanticKind

	// Type information (from go/types)
	Object types.Object // For named entities (vars, funcs, etc.)
	Type   types.Type   // For expressions

	// Dingo-specific context
	Context *DingoContext
}

// DingoContext provides Dingo-specific hover information
type DingoContext struct {
	Kind          ContextKind
	OriginalType  types.Type // For error_prop: the Result[T, E] type
	UnwrappedType types.Type // For error_prop: the T type
	Description   string     // Human-readable description
}

// ContextKind identifies the Dingo construct type
type ContextKind int

const (
	ContextNone      ContextKind = iota
	ContextErrorProp             // Error propagation (?)
	ContextSafeNav               // Safe navigation (?.)
	ContextNullCoal              // Null coalescing (??)
	ContextLambda                // Lambda expression
	ContextMatch                 // Match expression
)
