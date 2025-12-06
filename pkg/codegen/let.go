package codegen

import (
	"github.com/MadAppGang/dingo/pkg/ast"
)

// LetCodeGen generates Go variable declarations from Dingo let declarations.
//
// Transforms:
//   - With type annotation:
//     let x: int = 5 → var x int = 5
//   - Without type, with init:
//     let x = 5 → x := 5 (short declaration)
//   - Multiple names:
//     let a, b = getValues() → a, b := getValues()
//   - Declaration without init:
//     let x: int → var x int
//
// Handles:
//   - LetDecl.Names (single or multiple)
//   - LetDecl.TypeAnnot (optional, includes colon ": int")
//   - LetDecl.Value (expression as string)
//   - LetDecl.HasInit (whether = expr is present)
type LetCodeGen struct {
	*BaseGenerator
	decl *ast.LetDecl
}

// Generate produces Go code for the let declaration.
//
// Output formats:
//   - var x int = 5       (with type annotation)
//   - x := 5              (short declaration, no type)
//   - a, b := getValues() (multiple names)
//   - var x int           (no initialization)
//
// Source mappings track:
//   - Let declaration position → entire generated declaration
func (g *LetCodeGen) Generate() ast.CodeGenResult {
	if g.decl == nil {
		return ast.CodeGenResult{}
	}

	// Track start position
	dingoStart := int(g.decl.Pos())
	dingoEnd := int(g.decl.End())
	goStart := g.Buf.Len()

	// Use LetDecl.ToGo() for transformation logic
	g.Write(g.decl.ToGo())

	// Create mapping from let declaration to generated code
	goEnd := g.Buf.Len()

	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		goStart,
		goEnd,
		"let_decl",
	))

	return result
}
