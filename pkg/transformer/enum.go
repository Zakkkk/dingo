package transformer

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// transformEnumDecl transforms an EnumDecl to multiple Go declarations
// Converts Dingo enum syntax to Go tagged union pattern:
//   - enum Result<T, E> { Ok(T), Err(E) } → Tag type + constants + struct + constructors
//   - enum Color { Red, Green, Blue } → Tag type + constants + struct + constructors
func (t *Transformer) transformEnumDecl(decl *dingoast.EnumDecl) ([]goast.Decl, error) {
	if decl == nil {
		return nil, fmt.Errorf("transformEnumDecl: nil EnumDecl")
	}

	enumName := decl.Name.Name
	tagTypeName := enumName + "Tag"

	var result []goast.Decl

	// 1. Generate tag type declaration: type ResultTag int
	tagTypeDecl := &goast.GenDecl{
		Tok: token.TYPE,
		Specs: []goast.Spec{
			&goast.TypeSpec{
				Name: goast.NewIdent(tagTypeName),
				Type: goast.NewIdent("int"),
			},
		},
	}
	result = append(result, tagTypeDecl)

	// 2. Generate tag constants: const ( ResultTagOk ResultTag = iota; ResultTagErr )
	constSpecs := make([]goast.Spec, len(decl.Variants))
	for i, variant := range decl.Variants {
		tagConstName := tagTypeName + variant.Name.Name

		var value goast.Expr
		if i == 0 {
			// First constant: ResultTagOk ResultTag = iota
			value = goast.NewIdent("iota")
		}
		// Subsequent constants: ResultTagErr (implicit iota increment)

		constSpecs[i] = &goast.ValueSpec{
			Names: []*goast.Ident{goast.NewIdent(tagConstName)},
			Type:  goast.NewIdent(tagTypeName),
			Values: func() []goast.Expr {
				if value != nil {
					return []goast.Expr{value}
				}
				return nil
			}(),
		}
	}

	constDecl := &goast.GenDecl{
		Tok:   token.CONST,
		Lparen: 1, // Indicates parenthesized const block
		Specs: constSpecs,
	}
	result = append(result, constDecl)

	// 3. Generate struct type with tag and variant fields
	structFields := []*goast.Field{
		// tag field
		{
			Names: []*goast.Ident{goast.NewIdent("tag")},
			Type:  goast.NewIdent(tagTypeName),
		},
	}

	// Collect all variant fields
	fieldMap := make(map[string]string) // fieldName -> fieldType
	for _, variant := range decl.Variants {
		if len(variant.Fields) == 0 {
			continue // Unit variant, no fields
		}

		isSingleTupleVariant := len(variant.Fields) == 1 && variant.Kind == dingoast.TupleVariant

		for fieldIdx, field := range variant.Fields {
			var fieldName string

			if isSingleTupleVariant {
				// Single tuple field: use lowercase variant name (ok, err, some)
				fieldName = strings.ToLower(variant.Name.Name)
			} else if variant.Kind == dingoast.TupleVariant {
				// Multiple tuple fields: use baseName, baseName1, baseName2, ...
				baseName := strings.ToLower(variant.Name.Name)
				if fieldIdx == 0 {
					fieldName = baseName
				} else {
					fieldName = fmt.Sprintf("%s%d", baseName, fieldIdx)
				}
			} else {
				// Struct variant: use variantname_fieldname
				fieldName = strings.ToLower(variant.Name.Name) + "_" + field.Name.Name
			}

			// Add to field map (deduplicates if same field used in multiple variants)
			fieldMap[fieldName] = field.Type.Text
		}
	}

	// Add fields to struct in sorted order for consistency
	fieldNames := make([]string, 0, len(fieldMap))
	for name := range fieldMap {
		fieldNames = append(fieldNames, name)
	}
	// Simple sort (alphabetical)
	for i := 0; i < len(fieldNames); i++ {
		for j := i + 1; j < len(fieldNames); j++ {
			if fieldNames[i] > fieldNames[j] {
				fieldNames[i], fieldNames[j] = fieldNames[j], fieldNames[i]
			}
		}
	}

	for _, fieldName := range fieldNames {
		fieldType := fieldMap[fieldName]
		// Pointer type: *T
		structFields = append(structFields, &goast.Field{
			Names: []*goast.Ident{goast.NewIdent(fieldName)},
			Type: &goast.StarExpr{
				X: goast.NewIdent(fieldType),
			},
		})
	}

	// Build struct type spec
	structTypeSpec := &goast.TypeSpec{
		Name: goast.NewIdent(enumName),
		Type: &goast.StructType{
			Fields: &goast.FieldList{
				List: structFields,
			},
		},
	}

	// Handle generics if present
	if decl.TypeParams != nil && len(decl.TypeParams.Params) > 0 {
		// Add type parameters: [T, E any]
		typeParamFields := make([]*goast.Field, 0, len(decl.TypeParams.Params))

		// Build single field with all type param names: T, E any
		names := make([]*goast.Ident, len(decl.TypeParams.Params))
		for i, param := range decl.TypeParams.Params {
			names[i] = goast.NewIdent(param.Name)
		}

		typeParamFields = append(typeParamFields, &goast.Field{
			Names: names,
			Type:  goast.NewIdent("any"),
		})

		structTypeSpec.TypeParams = &goast.FieldList{
			List: typeParamFields,
		}
	}

	structDecl := &goast.GenDecl{
		Tok: token.TYPE,
		Specs: []goast.Spec{structTypeSpec},
	}
	result = append(result, structDecl)

	// 4. Generate constructor functions for each variant
	for _, variant := range decl.Variants {
		constructor := t.generateConstructor(enumName, tagTypeName, variant, decl.TypeParams)
		result = append(result, constructor)
	}

	// 5. Generate Is* methods
	for _, variant := range decl.Variants {
		isMethod := t.generateIsMethod(enumName, tagTypeName, variant, decl.TypeParams)
		result = append(result, isMethod)
	}

	return result, nil
}

// generateConstructor generates a constructor function for a variant
func (t *Transformer) generateConstructor(enumName, tagTypeName string, variant *dingoast.EnumVariant, typeParams *dingoast.TypeParamList) *goast.FuncDecl {
	constructorName := enumName + variant.Name.Name
	tagConstName := tagTypeName + variant.Name.Name

	// Build type parameters if present
	var funcTypeParams *goast.FieldList
	if typeParams != nil && len(typeParams.Params) > 0 {
		names := make([]*goast.Ident, len(typeParams.Params))
		for i, param := range typeParams.Params {
			names[i] = goast.NewIdent(param.Name)
		}

		funcTypeParams = &goast.FieldList{
			List: []*goast.Field{
				{
					Names: names,
					Type:  goast.NewIdent("any"),
				},
			},
		}
	}

	// Build return type
	var returnType goast.Expr = goast.NewIdent(enumName)
	if typeParams != nil && len(typeParams.Params) > 0 {
		// Result[T, E]
		typeArgs := make([]goast.Expr, len(typeParams.Params))
		for i, param := range typeParams.Params {
			typeArgs[i] = goast.NewIdent(param.Name)
		}
		returnType = &goast.IndexListExpr{
			X:       goast.NewIdent(enumName),
			Indices: typeArgs,
		}
	}

	if len(variant.Fields) == 0 {
		// Unit variant: func ResultOk() Result { return Result{tag: ResultTagOk} }
		return &goast.FuncDecl{
			Name: goast.NewIdent(constructorName),
			Type: &goast.FuncType{
				TypeParams: funcTypeParams,
				Params:     &goast.FieldList{}, // No parameters
				Results: &goast.FieldList{
					List: []*goast.Field{
						{Type: returnType},
					},
				},
			},
			Body: &goast.BlockStmt{
				List: []goast.Stmt{
					&goast.ReturnStmt{
						Results: []goast.Expr{
							&goast.CompositeLit{
								Type: returnType,
								Elts: []goast.Expr{
									&goast.KeyValueExpr{
										Key:   goast.NewIdent("tag"),
										Value: goast.NewIdent(tagConstName),
									},
								},
							},
						},
					},
				},
			},
		}
	}

	// Variant with fields: build parameter list and assignments
	params := make([]*goast.Field, 0, len(variant.Fields))
	assignments := []goast.Expr{
		// tag assignment
		&goast.KeyValueExpr{
			Key:   goast.NewIdent("tag"),
			Value: goast.NewIdent(tagConstName),
		},
	}

	isSingleTupleVariant := len(variant.Fields) == 1 && variant.Kind == dingoast.TupleVariant

	for fieldIdx, field := range variant.Fields {
		// Determine parameter name
		var paramName string
		if variant.Kind == dingoast.TupleVariant {
			paramName = fmt.Sprintf("arg%d", fieldIdx)
		} else {
			paramName = field.Name.Name
		}

		// Determine field name (must match struct field naming)
		var fieldName string
		if isSingleTupleVariant {
			fieldName = strings.ToLower(variant.Name.Name)
		} else if variant.Kind == dingoast.TupleVariant {
			baseName := strings.ToLower(variant.Name.Name)
			if fieldIdx == 0 {
				fieldName = baseName
			} else {
				fieldName = fmt.Sprintf("%s%d", baseName, fieldIdx)
			}
		} else {
			fieldName = strings.ToLower(variant.Name.Name) + "_" + field.Name.Name
		}

		// Add parameter
		params = append(params, &goast.Field{
			Names: []*goast.Ident{goast.NewIdent(paramName)},
			Type:  goast.NewIdent(field.Type.Text),
		})

		// Add assignment: fieldName: &paramName
		assignments = append(assignments, &goast.KeyValueExpr{
			Key: goast.NewIdent(fieldName),
			Value: &goast.UnaryExpr{
				Op: token.AND,
				X:  goast.NewIdent(paramName),
			},
		})
	}

	return &goast.FuncDecl{
		Name: goast.NewIdent(constructorName),
		Type: &goast.FuncType{
			TypeParams: funcTypeParams,
			Params: &goast.FieldList{
				List: params,
			},
			Results: &goast.FieldList{
				List: []*goast.Field{
					{Type: returnType},
				},
			},
		},
		Body: &goast.BlockStmt{
			List: []goast.Stmt{
				&goast.ReturnStmt{
					Results: []goast.Expr{
						&goast.CompositeLit{
							Type: returnType,
							Elts: assignments,
						},
					},
				},
			},
		},
	}
}

// generateIsMethod generates an Is* method for a variant
// Example: func (e Result) IsOk() bool { return e.tag == ResultTagOk }
func (t *Transformer) generateIsMethod(enumName, tagTypeName string, variant *dingoast.EnumVariant, typeParams *dingoast.TypeParamList) *goast.FuncDecl {
	methodName := "Is" + variant.Name.Name
	tagConstName := tagTypeName + variant.Name.Name

	// Build receiver type
	var receiverType goast.Expr = goast.NewIdent(enumName)
	if typeParams != nil && len(typeParams.Params) > 0 {
		// Result[T, E]
		typeArgs := make([]goast.Expr, len(typeParams.Params))
		for i, param := range typeParams.Params {
			typeArgs[i] = goast.NewIdent(param.Name)
		}
		receiverType = &goast.IndexListExpr{
			X:       goast.NewIdent(enumName),
			Indices: typeArgs,
		}
	}

	return &goast.FuncDecl{
		Recv: &goast.FieldList{
			List: []*goast.Field{
				{
					Names: []*goast.Ident{goast.NewIdent("e")},
					Type:  receiverType,
				},
			},
		},
		Name: goast.NewIdent(methodName),
		Type: &goast.FuncType{
			Params: &goast.FieldList{}, // No parameters
			Results: &goast.FieldList{
				List: []*goast.Field{
					{Type: goast.NewIdent("bool")},
				},
			},
		},
		Body: &goast.BlockStmt{
			List: []goast.Stmt{
				&goast.ReturnStmt{
					Results: []goast.Expr{
						&goast.BinaryExpr{
							X:  &goast.SelectorExpr{
								X:   goast.NewIdent("e"),
								Sel: goast.NewIdent("tag"),
							},
							Op: token.EQL,
							Y:  goast.NewIdent(tagConstName),
						},
					},
				},
			},
		},
	}
}

// RegisterEnumTransformer registers the enum transformer with the transformer framework
func (t *Transformer) RegisterEnumTransformer() {
	t.RegisterNodeTransformer("EnumDecl", func(node goast.Node, ctx *TransformContext) (goast.Node, error) {
		enumDecl, ok := node.(*dingoast.EnumDecl)
		if !ok {
			return nil, fmt.Errorf("expected *dingoast.EnumDecl, got %T", node)
		}

		// Transform enum to multiple declarations
		decls, err := t.transformEnumDecl(enumDecl)
		if err != nil {
			return nil, err
		}

		// Note: We return the first declaration as the node
		// The caller needs to handle the multiple declarations appropriately
		// This is a limitation of the current transformer architecture
		if len(decls) > 0 {
			return decls[0], nil
		}

		return nil, fmt.Errorf("enum transformation produced no declarations")
	})
}
