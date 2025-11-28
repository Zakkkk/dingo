package preprocessor

import (
	"bytes"
	"fmt"
)

// This file contains IIFE template generation utilities for functional transformations.
// Templates follow the same pattern as error_prop.go and ternary.go for consistency.

// IIFETemplate represents a code generation template for functional operations
type IIFETemplate struct {
	ReturnType string   // Return type of the IIFE (e.g., "[]int", "bool", "Option[User]")
	TempVar    string   // Temporary variable name (e.g., "tmp", "tmp1", "acc")
	LoopVar    string   // Loop iteration variable (e.g., "x", "item")
	Receiver   string   // The slice/array being operated on
	Body       []string // Lines of code inside the IIFE
}

// GenerateIIFE generates the complete IIFE code from a template
// Format:
//   func() ReturnType {
//       [Body lines with proper indentation]
//   }()
func (t *IIFETemplate) GenerateIIFE(indent string) string {
	var buf bytes.Buffer

	// Opening: func() ReturnType {
	buf.WriteString("func() ")
	buf.WriteString(t.ReturnType)
	buf.WriteString(" {\n")

	// Body lines (each indented with one extra level)
	for _, line := range t.Body {
		buf.WriteString(indent)
		buf.WriteString("\t")
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	// Closing: }()
	buf.WriteString(indent)
	buf.WriteString("}()")

	return buf.String()
}

// MapTemplate generates IIFE for map() operation
// Pattern:
//   func() []R {
//       tmp := make([]R, 0, len(receiver))
//       for _, x := range receiver {
//           tmp = append(tmp, transform(x))
//       }
//       return tmp
//   }()
func MapTemplate(receiver, loopVar, transformFunc, returnType string) *IIFETemplate {
	// Extract element type from slice type []T → T
	// For now, use placeholder that Go will infer
	// elemType will be inferred by Go compiler

	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    "tmp",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("tmp := make(%s, 0, len(%s))", returnType, receiver),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\ttmp = append(tmp, (%s)(%s))", transformFunc, loopVar),
			"}",
			"return tmp",
		},
	}
}

// FilterTemplate generates IIFE for filter() operation
// Pattern:
//   func() []T {
//       tmp := make([]T, 0, len(receiver))
//       for _, x := range receiver {
//           if predicate(x) {
//               tmp = append(tmp, x)
//           }
//       }
//       return tmp
//   }()
func FilterTemplate(receiver, loopVar, predicateFunc, returnType string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    "tmp",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("tmp := make(%s, 0, len(%s))", returnType, receiver),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif (%s)(%s) {", predicateFunc, loopVar),
			fmt.Sprintf("\t\ttmp = append(tmp, %s)", loopVar),
			"\t}",
			"}",
			"return tmp",
		},
	}
}

// ReduceTemplate generates IIFE for reduce() operation
// Pattern:
//   func() T {
//       acc := initialValue
//       for _, x := range receiver {
//           acc = reducerFunc(acc, x)
//       }
//       return acc
//   }()
func ReduceTemplate(receiver, loopVar, accVar, initialValue, reducerFunc, returnType string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    accVar,
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("%s := %s", accVar, initialValue),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\t%s = (%s)(%s, %s)", accVar, reducerFunc, accVar, loopVar),
			"}",
			fmt.Sprintf("return %s", accVar),
		},
	}
}

// AllTemplate generates IIFE for all() operation with early exit
// Pattern:
//   func() bool {
//       for _, x := range receiver {
//           if !(predicate(x)) {
//               return false
//           }
//       }
//       return true
//   }()
func AllTemplate(receiver, loopVar, predicateFunc string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: "bool",
		TempVar:    "",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif !((%s)(%s)) {", predicateFunc, loopVar),
			"\t\treturn false",
			"\t}",
			"}",
			"return true",
		},
	}
}

// AnyTemplate generates IIFE for any() operation with early exit
// Pattern:
//   func() bool {
//       for _, x := range receiver {
//           if predicate(x) {
//               return true
//           }
//       }
//       return false
//   }()
func AnyTemplate(receiver, loopVar, predicateFunc string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: "bool",
		TempVar:    "",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif (%s)(%s) {", predicateFunc, loopVar),
			"\t\treturn true",
			"\t}",
			"}",
			"return false",
		},
	}
}

// FindTemplate generates IIFE for find() operation → Option<T>
// Pattern:
//   func() Option[T] {
//       for _, x := range receiver {
//           if predicate(x) {
//               return Some(x)
//           }
//       }
//       return None[T]()
//   }()
func FindTemplate(receiver, loopVar, predicateFunc, elemType string) *IIFETemplate {
	returnType := fmt.Sprintf("Option[%s]", elemType)
	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    "",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif (%s)(%s) {", predicateFunc, loopVar),
			fmt.Sprintf("\t\treturn Some(%s)", loopVar),
			"\t}",
			"}",
			fmt.Sprintf("return None[%s]()", elemType),
		},
	}
}

// FindIndexTemplate generates IIFE for findIndex() operation → Option<int>
// Pattern:
//   func() Option[int] {
//       for i, x := range receiver {
//           if predicate(x) {
//               return Some(i)
//           }
//       }
//       return None[int]()
//   }()
func FindIndexTemplate(receiver, loopVar, predicateFunc string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: "Option[int]",
		TempVar:    "",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("for i, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif (%s)(%s) {", predicateFunc, loopVar),
			"\t\treturn Some(i)",
			"\t}",
			"}",
			"return None[int]()",
		},
	}
}

// SumTemplate generates IIFE for sum() operation
// Pattern:
//   func() T {
//       result := T(0)
//       for _, x := range receiver {
//           result += x
//       }
//       return result
//   }()
func SumTemplate(receiver, loopVar, returnType string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    "result",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("result := %s(0)", returnType),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tresult += %s", loopVar),
			"}",
			"return result",
		},
	}
}

// CountTemplate generates IIFE for count() operation
// Pattern:
//   func() int {
//       return len(receiver)
//   }()
//
// Note: This is a simple wrapper but follows IIFE pattern for consistency
func CountTemplate(receiver string) *IIFETemplate {
	return &IIFETemplate{
		ReturnType: "int",
		TempVar:    "",
		LoopVar:    "",
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("return len(%s)", receiver),
		},
	}
}

// PartitionTemplate generates IIFE for partition() operation → ([]T, []T)
// Pattern:
//   func() ([]T, []T) {
//       trueSlice := make([]T, 0, len(receiver))
//       falseSlice := make([]T, 0, len(receiver))
//       for _, x := range receiver {
//           if predicate(x) {
//               trueSlice = append(trueSlice, x)
//           } else {
//               falseSlice = append(falseSlice, x)
//           }
//       }
//       return trueSlice, falseSlice
//   }()
func PartitionTemplate(receiver, loopVar, predicateFunc, elemType string) *IIFETemplate {
	sliceType := fmt.Sprintf("[]%s", elemType)
	returnType := fmt.Sprintf("(%s, %s)", sliceType, sliceType)

	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    "trueSlice",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("trueSlice := make(%s, 0, len(%s))", sliceType, receiver),
			fmt.Sprintf("falseSlice := make(%s, 0, len(%s))", sliceType, receiver),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif (%s)(%s) {", predicateFunc, loopVar),
			fmt.Sprintf("\t\ttrueSlice = append(trueSlice, %s)", loopVar),
			"\t} else {",
			fmt.Sprintf("\t\tfalseSlice = append(falseSlice, %s)", loopVar),
			"\t}",
			"}",
			"return trueSlice, falseSlice",
		},
	}
}

// MapResultTemplate generates IIFE for mapResult() operation → Result<[]R, E>
// Pattern:
//   func() Result[[]R, error] {
//       tmp := make([]R, 0, len(receiver))
//       for _, x := range receiver {
//           res := mapFunc(x)
//           if res.IsErr() {
//               return Err[[]R](res.UnwrapErr())
//           }
//           tmp = append(tmp, res.Unwrap())
//       }
//       return Ok(tmp)
//   }()
func MapResultTemplate(receiver, loopVar, mapFunc, elemType string) *IIFETemplate {
	sliceType := fmt.Sprintf("[]%s", elemType)
	returnType := fmt.Sprintf("Result[%s, error]", sliceType)

	return &IIFETemplate{
		ReturnType: returnType,
		TempVar:    "tmp",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("tmp := make(%s, 0, len(%s))", sliceType, receiver),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tres := (%s)(%s)", mapFunc, loopVar),
			"\tif res.IsErr() {",
			fmt.Sprintf("\t\treturn Err[%s](res.UnwrapErr())", sliceType),
			"\t}",
			"\ttmp = append(tmp, res.Unwrap())",
			"}",
			"return Ok(tmp)",
		},
	}
}

// FilterMapTemplate generates IIFE for filterMap() operation
// Pattern:
//   func() []R {
//       tmp := make([]R, 0, len(receiver))
//       for _, x := range receiver {
//           if opt := mapFunc(x); opt.IsSome() {
//               tmp = append(tmp, opt.Unwrap())
//           }
//       }
//       return tmp
//   }()
func FilterMapTemplate(receiver, loopVar, mapFunc, elemType string) *IIFETemplate {
	sliceType := fmt.Sprintf("[]%s", elemType)

	return &IIFETemplate{
		ReturnType: sliceType,
		TempVar:    "tmp",
		LoopVar:    loopVar,
		Receiver:   receiver,
		Body: []string{
			fmt.Sprintf("tmp := make(%s, 0, len(%s))", sliceType, receiver),
			fmt.Sprintf("for _, %s := range %s {", loopVar, receiver),
			fmt.Sprintf("\tif opt := (%s)(%s); opt.IsSome() {", mapFunc, loopVar),
			"\t\ttmp = append(tmp, opt.Unwrap())",
			"\t}",
			"}",
			"return tmp",
		},
	}
}
