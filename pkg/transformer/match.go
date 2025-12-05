package transformer

import (
	"fmt"
	goast "go/ast"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// transformMatch transforms a MatchExpr to Go switch statement
// Converts Dingo match expressions to idiomatic Go switch statements:
//   - match result { Ok(x) => x, Err(e) => 0 } → switch result.tag { case ResultTagOk: ..., case ResultTagErr: ... }
//   - Handles constructor patterns, literals, wildcards, and guards
//   - Supports both expression and statement contexts
func (t *Transformer) transformMatch(node goast.Node, ctx *TransformContext) (goast.Node, error) {
	matchExpr, ok := node.(*dingoast.MatchExpr)
	if !ok {
		return nil, fmt.Errorf("transformMatch: expected *dingoast.MatchExpr, got %T", node)
	}

	// Generate temp variable for scrutinee if needed
	scrutineeVar := ctx.FreshTempVar()

	// Create statements for the switch
	var stmts []goast.Stmt

	// Assign scrutinee to temp variable: tmp := scrutinee
	scrutineeAssign := &goast.AssignStmt{
		Lhs: []goast.Expr{
			&goast.Ident{
				NamePos: matchExpr.Scrutinee.Pos(),
				Name:    scrutineeVar,
			},
		},
		TokPos: matchExpr.Scrutinee.Pos(),
		Tok:    token.DEFINE,
		Rhs: []goast.Expr{
			t.convertDingoExprToGoExpr(matchExpr.Scrutinee),
		},
	}
	stmts = append(stmts, scrutineeAssign)

	// Build switch statement
	switchStmt := &goast.SwitchStmt{
		Switch: matchExpr.Match,
		Body: &goast.BlockStmt{
			Lbrace: matchExpr.OpenBrace,
			List:   make([]goast.Stmt, 0, len(matchExpr.Arms)),
			Rbrace: matchExpr.CloseBrace,
		},
	}

	// Determine if we need a result variable (match as expression)
	var resultVar string
	if matchExpr.IsExpr {
		resultVar = ctx.FreshTempVar()
	}

	// Process each match arm
	for _, arm := range matchExpr.Arms {
		caseClause, err := t.transformMatchArm(arm, scrutineeVar, resultVar, ctx)
		if err != nil {
			return nil, fmt.Errorf("transformMatch: failed to transform arm: %w", err)
		}
		switchStmt.Body.List = append(switchStmt.Body.List, caseClause)
	}

	stmts = append(stmts, switchStmt)

	// If match is expression, wrap in IIFE that returns result
	if matchExpr.IsExpr {
		// Build IIFE: func() T { var result T; switch ... { ... }; return result }()
		iife := t.buildMatchIIFE(resultVar, stmts, matchExpr.Pos())
		return iife, nil
	}

	// For statement context, return block statement
	return &goast.BlockStmt{
		Lbrace: matchExpr.Match,
		List:   stmts,
		Rbrace: matchExpr.CloseBrace,
	}, nil
}

// transformMatchArm transforms a single match arm to a case clause
func (t *Transformer) transformMatchArm(
	arm *dingoast.MatchArm,
	scrutineeVar string,
	resultVar string,
	ctx *TransformContext,
) (*goast.CaseClause, error) {
	// Generate case value(s)
	caseExprs, bindings, err := t.transformPattern(arm.Pattern, scrutineeVar)
	if err != nil {
		return nil, fmt.Errorf("transformMatchArm: failed to transform pattern: %w", err)
	}

	// Build case body
	var bodyStmts []goast.Stmt

	// Add binding extractions
	for _, binding := range bindings {
		bodyStmts = append(bodyStmts, binding)
	}

	// Add guard check if present
	if arm.Guard != nil {
		guardIf := &goast.IfStmt{
			If:   arm.GuardPos,
			Cond: t.convertDingoExprToGoExpr(arm.Guard),
			Body: &goast.BlockStmt{
				List: t.buildArmBody(arm, resultVar),
			},
		}
		bodyStmts = append(bodyStmts, guardIf)
	} else {
		// No guard: add body directly
		bodyStmts = append(bodyStmts, t.buildArmBody(arm, resultVar)...)
	}

	return &goast.CaseClause{
		Case:  arm.PatternPos,
		List:  caseExprs,
		Colon: arm.Arrow,
		Body:  bodyStmts,
	}, nil
}

// transformPattern transforms a pattern to case expression(s) and binding statements
// Returns: case expressions, binding statements, error
func (t *Transformer) transformPattern(
	pattern dingoast.Pattern,
	scrutineeVar string,
) ([]goast.Expr, []goast.Stmt, error) {
	switch p := pattern.(type) {
	case *dingoast.ConstructorPattern:
		return t.transformConstructorPattern(p, scrutineeVar)
	case *dingoast.LiteralPattern:
		return t.transformLiteralPattern(p)
	case *dingoast.WildcardPattern:
		return t.transformWildcardPattern()
	case *dingoast.VariablePattern:
		return t.transformVariablePattern(p, scrutineeVar)
	default:
		return nil, nil, fmt.Errorf("unsupported pattern type: %T", pattern)
	}
}

// transformConstructorPattern transforms constructor pattern: Ok(x), Err(e)
// Returns: case expression for tag field, binding statements
func (t *Transformer) transformConstructorPattern(
	p *dingoast.ConstructorPattern,
	scrutineeVar string,
) ([]goast.Expr, []goast.Stmt, error) {
	// Generate tag name: ResultTagOk, ResultTagErr, OptionTagSome, OptionTagNone
	// The tag name is based on the constructor name (Ok → ResultTagOk, Some → OptionTagSome)
	tagName := t.constructorToTagName(p.Name)

	// Case expression: scrutinee.tag (for accessing the tag field)
	// But in switch, we just use the tag constant
	caseExpr := &goast.Ident{
		NamePos: p.NamePos,
		Name:    tagName,
	}

	// Generate binding statements for parameters
	var bindings []goast.Stmt
	for i, param := range p.Params {
		if varPat, ok := param.(*dingoast.VariablePattern); ok {
			// Extract binding: x := *tmp.result_ok_0
			fieldName := t.constructorFieldName(p.Name, i)
			binding := &goast.AssignStmt{
				Lhs: []goast.Expr{
					&goast.Ident{
						NamePos: varPat.NamePos,
						Name:    varPat.Name,
					},
				},
				TokPos: varPat.NamePos,
				Tok:    token.DEFINE,
				Rhs: []goast.Expr{
					&goast.StarExpr{
						Star: varPat.NamePos,
						X: &goast.SelectorExpr{
							X: &goast.Ident{
								Name: scrutineeVar,
							},
							Sel: &goast.Ident{
								Name: fieldName,
							},
						},
					},
				},
			}
			bindings = append(bindings, binding)
		}
	}

	return []goast.Expr{caseExpr}, bindings, nil
}

// transformLiteralPattern transforms literal pattern: 1, "hello", true
func (t *Transformer) transformLiteralPattern(p *dingoast.LiteralPattern) ([]goast.Expr, []goast.Stmt, error) {
	// Create literal expression based on kind
	var lit goast.Expr
	switch p.Kind {
	case dingoast.IntLiteral:
		lit = &goast.BasicLit{
			ValuePos: p.ValuePos,
			Kind:     token.INT,
			Value:    p.Value,
		}
	case dingoast.FloatLiteral:
		lit = &goast.BasicLit{
			ValuePos: p.ValuePos,
			Kind:     token.FLOAT,
			Value:    p.Value,
		}
	case dingoast.StringLiteral:
		lit = &goast.BasicLit{
			ValuePos: p.ValuePos,
			Kind:     token.STRING,
			Value:    p.Value,
		}
	case dingoast.BoolLiteral:
		lit = &goast.Ident{
			NamePos: p.ValuePos,
			Name:    p.Value,
		}
	default:
		return nil, nil, fmt.Errorf("unsupported literal kind: %v", p.Kind)
	}

	return []goast.Expr{lit}, nil, nil
}

// transformWildcardPattern transforms wildcard pattern: _
func (t *Transformer) transformWildcardPattern() ([]goast.Expr, []goast.Stmt, error) {
	// Wildcard becomes default case (nil case expression list)
	return nil, nil, nil
}

// transformVariablePattern transforms variable pattern (binding without constructor)
// This is treated as default case with binding: x => default: x := scrutinee
func (t *Transformer) transformVariablePattern(
	p *dingoast.VariablePattern,
	scrutineeVar string,
) ([]goast.Expr, []goast.Stmt, error) {
	// Variable pattern is default case with binding
	binding := &goast.AssignStmt{
		Lhs: []goast.Expr{
			&goast.Ident{
				NamePos: p.NamePos,
				Name:    p.Name,
			},
		},
		TokPos: p.NamePos,
		Tok:    token.DEFINE,
		Rhs: []goast.Expr{
			&goast.Ident{
				Name: scrutineeVar,
			},
		},
	}
	return nil, []goast.Stmt{binding}, nil
}

// buildArmBody builds the body statements for a match arm
func (t *Transformer) buildArmBody(arm *dingoast.MatchArm, resultVar string) []goast.Stmt {
	var stmts []goast.Stmt

	if arm.IsBlock {
		// Block body: the body is a RawExpr containing block text
		// For now, we'll create a placeholder comment indicating the block needs parsing
		// Full implementation would parse the block text to AST
		stmts = append(stmts, &goast.ExprStmt{
			X: &goast.Ident{
				Name: "/* block body: " + arm.Body.String() + " */",
			},
		})
	} else {
		// Expression body
		bodyExpr := t.convertDingoExprToGoExpr(arm.Body)

		if resultVar != "" {
			// Assign to result variable: result = bodyExpr
			stmts = append(stmts, &goast.AssignStmt{
				Lhs: []goast.Expr{
					&goast.Ident{Name: resultVar},
				},
				TokPos: arm.Arrow,
				Tok:    token.ASSIGN,
				Rhs:    []goast.Expr{bodyExpr},
			})
		} else {
			// Statement context: use as expression statement or return
			stmts = append(stmts, &goast.ExprStmt{
				X: bodyExpr,
			})
		}
	}

	return stmts
}

// buildMatchIIFE builds an IIFE for match expression
// Pattern: func() T { var result T; switch ... { ... }; return result }()
func (t *Transformer) buildMatchIIFE(resultVar string, stmts []goast.Stmt, pos token.Pos) *goast.CallExpr {
	// Declare result variable: var result T
	resultDecl := &goast.DeclStmt{
		Decl: &goast.GenDecl{
			Tok: token.VAR,
			Specs: []goast.Spec{
				&goast.ValueSpec{
					Names: []*goast.Ident{
						{Name: resultVar},
					},
					// Type would be inferred from assignments
				},
			},
		},
	}

	// Return result
	returnStmt := &goast.ReturnStmt{
		Results: []goast.Expr{
			&goast.Ident{Name: resultVar},
		},
	}

	// Build function body
	bodyStmts := []goast.Stmt{resultDecl}
	bodyStmts = append(bodyStmts, stmts...)
	bodyStmts = append(bodyStmts, returnStmt)

	// Build function literal
	funcLit := &goast.FuncLit{
		Type: &goast.FuncType{
			Func:   pos,
			Params: &goast.FieldList{},
			// Results would need type inference
		},
		Body: &goast.BlockStmt{
			List: bodyStmts,
		},
	}

	// Call immediately: (func() T { ... })()
	return &goast.CallExpr{
		Fun:    funcLit,
		Lparen: pos,
		Rparen: pos,
	}
}

// constructorToTagName converts constructor name to tag constant name
// Ok → ResultTagOk, Err → ResultTagErr, Some → OptionTagSome, None → OptionTagNone
func (t *Transformer) constructorToTagName(constructor string) string {
	switch constructor {
	case "Ok":
		return "ResultTagOk"
	case "Err":
		return "ResultTagErr"
	case "Some":
		return "OptionTagSome"
	case "None":
		return "OptionTagNone"
	default:
		// For custom enums: EnumName_Variant → EnumNameTagVariant
		// This is simplified - full implementation would use type info
		return constructor + "Tag"
	}
}

// constructorFieldName generates the field name for accessing constructor data
// Ok, 0 → result_ok_0, Err, 0 → result_err_0
func (t *Transformer) constructorFieldName(constructor string, paramIndex int) string {
	switch constructor {
	case "Ok":
		return fmt.Sprintf("result_ok_%d", paramIndex)
	case "Err":
		return fmt.Sprintf("result_err_%d", paramIndex)
	case "Some":
		return fmt.Sprintf("option_some_%d", paramIndex)
	default:
		// For custom enums, use lowercase constructor name
		return fmt.Sprintf("%s_%d", constructor, paramIndex)
	}
}

// convertDingoExprToGoExpr converts a Dingo expression to Go expression
// This is a helper for handling RawExpr and other Dingo-specific expressions
func (t *Transformer) convertDingoExprToGoExpr(expr dingoast.Expr) goast.Expr {
	// If it's a RawExpr, we need to parse it
	// For now, we'll create a placeholder identifier
	// Full implementation would parse the expression text
	if rawExpr, ok := expr.(*dingoast.RawExpr); ok {
		// For simple cases, create an identifier
		// This is simplified - full implementation would parse
		return &goast.Ident{
			NamePos: rawExpr.Pos(),
			Name:    rawExpr.Text,
		}
	}

	// For other expressions, assume they're already valid Go AST
	if goExpr, ok := expr.(goast.Expr); ok {
		return goExpr
	}

	// Fallback: create placeholder
	return &goast.Ident{
		Name: expr.String(),
	}
}

// RegisterMatchTransformer registers the match expression transformer
func (t *Transformer) RegisterMatchTransformer() {
	t.RegisterNodeTransformer("MatchExpr", t.transformMatch)
}
