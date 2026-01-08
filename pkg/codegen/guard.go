package codegen

import (
	"fmt"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// ExprType represents the detected type of a guard expression.
type ExprType int

const (
	TypeUnknown ExprType = iota
	TypeResult           // Result[T, E]
	TypeOption           // Option[T]
)

func (t ExprType) String() string {
	switch t {
	case TypeResult:
		return "Result"
	case TypeOption:
		return "Option"
	default:
		return "Unknown"
	}
}

// InferExprType determines if expr returns Result or Option.
// Deprecated: Use InferExprTypeWithBinding instead which uses semantic signals.
func InferExprType(exprText string, fileContext []byte) (ExprType, error) {
	// Without binding information, we cannot determine type
	// This function exists for API compatibility
	return TypeUnknown, fmt.Errorf("cannot infer type without binding info: use InferExprTypeWithBinding")
}

// InferExprTypeWithBinding determines if expr returns Result or Option.
// Uses the pipe binding |err| as the ONLY type signal (no string heuristics).
//
// Type inference rules:
//   - |err| binding present → Result type (has error value to bind)
//   - No binding → Option type (nothing to bind)
//
// This is semantically correct because:
//   - Result[T, E] has both Ok and Err variants - guard let binds the error
//   - Option[T] has only Some and None - nothing to bind on None
func InferExprTypeWithBinding(exprText string, hasBinding bool) (ExprType, error) {
	if hasBinding {
		// Pipe binding (|err|) means Result type - error value to bind
		return TypeResult, nil
	}
	// No binding means Option type - None has nothing to bind
	return TypeOption, nil
}

// GuardGenerator generates Go code for guard statements.
//
// Transforms:
//
//	guard user := FindUser(id) else |err| { return Err(err) }
//
// Into:
//
//	tmp := FindUser(id)
//	if tmp.IsErr() {
//	    err := *tmp.err
//	    return ResultErr(err)
//	}
//	user := *tmp.ok
//
// Variable naming convention:
//   - First: tmp, err
//   - Second: tmp1, err1
//   - Third: tmp2, err2
type GuardGenerator struct {
	*BaseGenerator
	Location    dingoast.GuardLocation
	ExprType    ExprType
	SourceBytes []byte // Source bytes for extracting else block content
}

// NewGuardGenerator creates a guard code generator.
func NewGuardGenerator(loc dingoast.GuardLocation, exprType ExprType) *GuardGenerator {
	return &GuardGenerator{
		BaseGenerator: NewBaseGenerator(),
		Location:      loc,
		ExprType:      exprType,
	}
}

// Generate produces Go code for the guard statement.
func (g *GuardGenerator) Generate() dingoast.CodeGenResult {
	// Generate unique temp variable
	tmpVar := g.TempVar("tmp")

	// 1. Assign expression to temp
	g.Write(tmpVar)
	g.Write(" := ")
	g.Write(g.Location.ExprText)
	g.WriteByte('\n')

	// 2. Generate if check based on type
	g.generateCheck(tmpVar)

	// 3. Variable binding(s)
	g.generateBindings(tmpVar)

	return g.Result()
}

// generateCheck generates the if check and else block.
func (g *GuardGenerator) generateCheck(tmpVar string) {
	// Generate if condition based on type
	switch g.ExprType {
	case TypeResult:
		g.Write("if ")
		g.Write(tmpVar)
		g.Write(".IsErr() {\n")

		// Bind error if requested
		if g.Location.HasBinding {
			g.Write("\t")
			g.Write(g.Location.BindingName)
			g.Write(" := *")
			g.Write(tmpVar)
			g.Write(".Err\n")
		}

	case TypeOption:
		g.Write("if ")
		g.Write(tmpVar)
		g.Write(".IsNone() {\n")
	}

	// Else block content
	g.generateElseBlock()

	// Close if block
	g.Write("}\n")
}

// generateElseBlock generates the else block content.
func (g *GuardGenerator) generateElseBlock() {
	// If we have source bytes, extract the actual else block content
	if g.SourceBytes != nil && g.Location.ElseStart >= 0 && g.Location.ElseEnd > g.Location.ElseStart {
		elseContent := string(g.SourceBytes[g.Location.ElseStart:g.Location.ElseEnd])
		// Indent each line, preserving blank lines and original formatting
		lines := strings.Split(elseContent, "\n")
		for i, line := range lines {
			g.Write("\t")
			g.Write(line)
			// Don't add extra newline after last line (already has it from split)
			if i < len(lines)-1 || len(line) > 0 {
				g.WriteByte('\n')
			}
		}
	} else {
		// Placeholder for when source bytes are not available (e.g., in unit tests)
		g.Write("\t// GUARD_LET_ELSE_BLOCK\n")
		g.Write("\treturn\n")
	}
}

// generateBindings generates variable bindings for the success case.
func (g *GuardGenerator) generateBindings(tmpVar string) {
	valueField := g.getValueField()

	// Choose := for declaration, = for assignment
	assignOp := " := "
	if !g.Location.IsDecl {
		assignOp = " = "
	}

	if g.Location.IsTuple {
		// Tuple destructuring: name := (*tmp.ok).Item1 or name = (*tmp.ok).Item1
		for i, name := range g.Location.VarNames {
			g.Write(name)
			g.Write(assignOp)
			g.Write("(*")
			g.Write(tmpVar)
			g.Write(".")
			g.Write(valueField)
			g.Write(").Item")
			g.Write(fmt.Sprintf("%d", i+1))
			g.WriteByte('\n')
		}
	} else {
		// Single binding: user := *tmp.ok or user = *tmp.ok
		g.Write(g.Location.VarNames[0])
		g.Write(assignOp)
		g.Write("*")
		g.Write(tmpVar)
		g.Write(".")
		g.Write(valueField)
		g.WriteByte('\n')
	}
}

// getValueField returns the field name based on type (Ok for Result, Some for Option).
func (g *GuardGenerator) getValueField() string {
	if g.ExprType == TypeResult {
		return "Ok"
	}
	return "Some"
}

// GenerateGuard is the main entry point for guard code generation.
// Returns generated Go code and source mappings.
func GenerateGuard(loc dingoast.GuardLocation, exprType ExprType) dingoast.CodeGenResult {
	gen := NewGuardGenerator(loc, exprType)
	return gen.Generate()
}

// GenerateGuardWithSource generates guard code with access to source bytes
// for extracting else block content.
func GenerateGuardWithSource(loc dingoast.GuardLocation, exprType ExprType, src []byte) dingoast.CodeGenResult {
	gen := NewGuardGenerator(loc, exprType)
	gen.SourceBytes = src
	return gen.Generate()
}
