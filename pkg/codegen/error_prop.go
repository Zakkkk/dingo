package codegen

import (
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// ErrorPropGenerator generates Go code for error propagation expressions (expr?).
//
// Transforms:
//   let data = readFile(path)?
//
// Into:
//   tmp, err := readFile(path)
//   if err != nil {
//       return zeroVal1, zeroVal2, ..., err
//   }
//   data := tmp
//
// Variable naming convention:
//   - First: tmp, err
//   - Second: tmp1, err1
//   - Third: tmp2, err2
type ErrorPropGenerator struct {
	*BaseGenerator
	Expr        *ast.ErrorPropExpr
	ReturnTypes []string // Zero values for non-error return types
	Counter     int      // For generating unique variable names
}

// NewErrorPropGenerator creates a new error propagation code generator.
func NewErrorPropGenerator(expr *ast.ErrorPropExpr, returnTypes []string) *ErrorPropGenerator {
	return &ErrorPropGenerator{
		BaseGenerator: NewBaseGenerator(),
		Expr:          expr,
		ReturnTypes:   returnTypes,
		Counter:       1,
	}
}

// Generate produces Go code for error propagation.
//
// Pattern:
//   1. Generate unique temp variable names
//   2. Call operand and capture result + error
//   3. Check error and return with zero values if non-nil
//   4. Return temp variable for success case
//
// Example:
//   Input:  readFile(path)?
//   Output: tmp, err := readFile(path)
//           if err != nil {
//               return 0, "", err  // zero values based on return types
//           }
//           tmp
func (g *ErrorPropGenerator) Generate() ast.CodeGenResult {
	// Convert Dingo operand AST to Go source code
	operandSrc := g.dingoExprToString(g.Expr.Operand)

	// Generate unique variable names (camelCase, no underscores)
	var tmpVar, errVar string
	if g.Counter == 1 {
		tmpVar = "tmp"
		errVar = "err"
		g.Counter++
	} else {
		suffix := strconv.Itoa(g.Counter - 1)
		tmpVar = "tmp" + suffix
		errVar = "err" + suffix
		g.Counter++
	}

	// Generate the error propagation pattern:
	//   tmp, err := operand
	//   if err != nil {
	//       return zeroVal1, zeroVal2, ..., err
	//   }
	//   tmp

	// Line 1: tmp, err := operand
	g.Write(tmpVar)
	g.Write(", ")
	g.Write(errVar)
	g.Write(" := ")
	g.Write(operandSrc)
	g.WriteByte('\n')

	// Line 2: if err != nil {
	g.Write("if ")
	g.Write(errVar)
	g.Write(" != nil {\n")

	// Line 3: return zeroVal1, zeroVal2, ..., err
	g.Write("\treturn ")
	for i, zeroVal := range g.ReturnTypes {
		if i > 0 {
			g.Write(", ")
		}
		g.Write(zeroVal)
	}
	if len(g.ReturnTypes) > 0 {
		g.Write(", ")
	}
	g.Write(errVar)
	g.WriteByte('\n')

	// Line 4: }
	g.Write("}\n")

	// Line 5: tmp (the extracted value)
	g.Write(tmpVar)

	return g.Result()
}

// dingoExprToString converts a Dingo ast.Expr to Go source code string.
func (g *ErrorPropGenerator) dingoExprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.DingoIdent:
		return e.Name
	case *ast.RawExpr:
		return e.Text
	default:
		// For other expression types, use String() method
		return expr.String()
	}
}

// argsToString converts a slice of Dingo expressions to comma-separated Go source.
func (g *ErrorPropGenerator) argsToString(args []ast.Expr) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = g.dingoExprToString(arg)
	}
	return strings.Join(parts, ", ")
}
