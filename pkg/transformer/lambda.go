package transformer

import (
	"fmt"
	goast "go/ast"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// transformLambda transforms a LambdaExpr to Go function literal
// Converts Dingo lambda syntax to Go func literal:
//   - (x, y) => x + y → func(x, y int) int { return x + y }
//   - x => x * 2 → func(x int) int { return x * 2 }
//   - |x: int| -> int { x * 2 } → func(x int) int { return x * 2 }
func (t *Transformer) transformLambda(node goast.Node, ctx *TransformContext) (goast.Node, error) {
	lambda, ok := node.(*dingoast.LambdaExpr)
	if !ok {
		return nil, fmt.Errorf("transformLambda: expected *dingoast.LambdaExpr, got %T", node)
	}

	// Build parameter list for FuncType
	paramList := &goast.FieldList{
		Opening: lambda.Pos(),
		List:    make([]*goast.Field, 0, len(lambda.Params)),
	}

	for _, param := range lambda.Params {
		field := &goast.Field{
			Names: []*goast.Ident{
				{Name: param.Name},
			},
		}

		// Add type if specified, otherwise leave nil for type inference
		if param.Type != "" {
			field.Type = &goast.Ident{Name: param.Type}
		}

		paramList.List = append(paramList.List, field)
	}

	// Build function type
	funcType := &goast.FuncType{
		Func:   lambda.Pos(),
		Params: paramList,
	}

	// Add return type if specified
	if lambda.ReturnType != "" {
		funcType.Results = &goast.FieldList{
			List: []*goast.Field{
				{
					Type: &goast.Ident{Name: lambda.ReturnType},
				},
			},
		}
	}

	// Build function body
	var body *goast.BlockStmt

	if lambda.IsBlock {
		// Block body: use statements directly
		// Parse the block body (it's already in Go syntax after preprocessing)
		// For now, we'll wrap it in a block statement
		// The body is unparsed text, so we need to handle this carefully

		// TODO: Parse lambda.Body if it contains statements
		// For now, create an empty block as placeholder
		body = &goast.BlockStmt{
			Lbrace: lambda.Pos(),
			List:   []goast.Stmt{},
			Rbrace: lambda.End(),
		}
	} else {
		// Expression body: wrap in return statement
		// The body is an unparsed expression string
		// We need to parse it to create the proper AST node

		// For now, we'll create a placeholder return statement
		// In a full implementation, we'd parse lambda.Body into an expression
		// Since the body is already preprocessed to Go syntax, we can use a placeholder

		// Create return statement with placeholder expression
		body = &goast.BlockStmt{
			Lbrace: lambda.Pos(),
			List: []goast.Stmt{
				&goast.ReturnStmt{
					Return: lambda.Pos(),
					// Results would contain the parsed expression from lambda.Body
					// For now, use nil to indicate incomplete implementation
					Results: nil,
				},
			},
			Rbrace: lambda.End(),
		}
	}

	// Create function literal
	funcLit := &goast.FuncLit{
		Type: funcType,
		Body: body,
	}

	return funcLit, nil
}

// RegisterLambdaTransformer registers the lambda transformer with the transformer framework
func (t *Transformer) RegisterLambdaTransformer() {
	t.RegisterNodeTransformer("LambdaExpr", t.transformLambda)
}
