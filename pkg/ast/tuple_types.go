package ast

import "fmt"

// TupleKind represents the type of tuple syntax
type TupleKind int

const (
	TupleKindLiteral     TupleKind = iota // (a, b)
	TupleKindDestructure                  // let (x, y) = expr
	TupleKindTypeAlias                    // type Point = (int, int)
	TupleKindFuncReturn                   // func foo() (int, int)
)

// String returns string representation of TupleKind
func (k TupleKind) String() string {
	switch k {
	case TupleKindLiteral:
		return "literal"
	case TupleKindDestructure:
		return "destructure"
	case TupleKindTypeAlias:
		return "type_alias"
	case TupleKindFuncReturn:
		return "func_return"
	default:
		return fmt.Sprintf("TupleKind(%d)", k)
	}
}
