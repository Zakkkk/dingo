// Package builtin provides lambda type inference plugin
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// LambdaTypeInferencePlugin infers types for lambda parameters from call context
//
// This plugin analyzes function literals with untyped parameters and attempts to
// infer their types from the surrounding context:
// - Method calls: users.filter(func(u) { ... }) -> infer u: User
// - Function arguments: process(func(x) { ... }) -> infer from process signature
//
// Current implementation status:
// - Phase 1 (v1.0): Basic inference for common patterns (map/filter/reduce)
// - Phase 2 (v1.1): Full go/types integration with complex type propagation
//
// For v1.0, explicit type annotations are required when inference fails.
type LambdaTypeInferencePlugin struct {
	ctx *plugin.Context

	// Type inference service for go/types integration
	typeInference *TypeInferenceService

	// Track function literals that need type inference
	untypedLiterals []*funcLiteralContext

	// Track variable declarations needing type inference
	untypedVars []*varDeclContext
}

// funcLiteralContext tracks a function literal needing type inference
type funcLiteralContext struct {
	funcLit  *ast.FuncLit
	callExpr *ast.CallExpr // The call expression containing this literal
	argIndex int           // Position in call arguments
	pos      token.Pos
}

// varDeclContext tracks a variable declaration needing type inference
type varDeclContext struct {
	varSpec *ast.ValueSpec
	varName string // Name of the variable
	pos     token.Pos
}

// NewLambdaTypeInferencePlugin creates a new lambda type inference plugin
func NewLambdaTypeInferencePlugin() *LambdaTypeInferencePlugin {
	return &LambdaTypeInferencePlugin{
		untypedLiterals: make([]*funcLiteralContext, 0),
		untypedVars:     make([]*varDeclContext, 0),
	}
}

// Name returns the plugin name
func (p *LambdaTypeInferencePlugin) Name() string {
	return "lambda_type_inference"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *LambdaTypeInferencePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	// Initialize type inference service with go/types integration
	if ctx != nil && ctx.FileSet != nil {
		// Create type inference service
		service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
		if err != nil {
			ctx.Logger.Warnf("Lambda type inference: Failed to create type inference service: %v", err)
		} else {
			p.typeInference = service

			// Inject go/types.Info if available in context
			if ctx.TypeInfo != nil {
				if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
					service.SetTypesInfo(typesInfo)
					ctx.Logger.Debugf("Lambda type inference: go/types integration enabled")
				}
			}
		}
	}
}

// Process processes AST nodes to find and infer lambda parameter types and variable types
func (p *LambdaTypeInferencePlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Phase 1: Discover function literals with untyped parameters
	p.discoverUntypedLiterals(node)

	// Phase 1b: Discover variable declarations with __TYPE_INFERENCE_NEEDED
	p.discoverUntypedVars(node)

	// Phase 2: Attempt type inference for each literal
	for _, ctx := range p.untypedLiterals {
		p.inferFuncLiteralTypes(ctx)
	}

	// Phase 3: Attempt type inference for each variable
	for _, ctx := range p.untypedVars {
		p.inferVarDeclType(ctx, node)
	}

	return nil
}

// discoverUntypedLiterals walks the AST to find function literals needing type inference
func (p *LambdaTypeInferencePlugin) discoverUntypedLiterals(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			// Check if any arguments are function literals with untyped params
			for i, arg := range n.Args {
				if funcLit, ok := arg.(*ast.FuncLit); ok {
					if p.hasUntypedParams(funcLit) {
						p.untypedLiterals = append(p.untypedLiterals, &funcLiteralContext{
							funcLit:  funcLit,
							callExpr: n,
							argIndex: i,
							pos:      funcLit.Pos(),
						})
					}
				}
			}
		}
		return true
	})
}

// hasUntypedParams checks if a function literal has any parameters without type annotations
func (p *LambdaTypeInferencePlugin) hasUntypedParams(funcLit *ast.FuncLit) bool {
	if funcLit.Type == nil || funcLit.Type.Params == nil {
		return false
	}

	for _, field := range funcLit.Type.Params.List {
		// If Type is nil, parameter is untyped
		if field.Type == nil {
			return true
		}

		// Check for __TYPE_INFERENCE_NEEDED marker added by preprocessor
		// This indicates the preprocessor couldn't determine the type and needs plugin help
		if ident, ok := field.Type.(*ast.Ident); ok {
			if ident.Name == "__TYPE_INFERENCE_NEEDED" {
				return true
			}
		}
	}

	return false
}

// discoverUntypedVars walks the AST to find var declarations with __TYPE_INFERENCE_NEEDED
func (p *LambdaTypeInferencePlugin) discoverUntypedVars(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GenDecl:
			// Only process var declarations
			if n.Tok != token.VAR {
				return true
			}

			// Check each ValueSpec in the declaration
			for _, spec := range n.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					// Check if type is __TYPE_INFERENCE_NEEDED
					if p.needsTypeInference(valueSpec) {
						// Track each variable name in this spec
						for _, name := range valueSpec.Names {
							p.untypedVars = append(p.untypedVars, &varDeclContext{
								varSpec: valueSpec,
								varName: name.Name,
								pos:     name.Pos(),
							})
						}
					}
				}
			}
		}
		return true
	})
}

// needsTypeInference checks if a ValueSpec needs type inference
func (p *LambdaTypeInferencePlugin) needsTypeInference(spec *ast.ValueSpec) bool {
	// Check if Type is __TYPE_INFERENCE_NEEDED marker
	if spec.Type == nil {
		return false
	}

	if ident, ok := spec.Type.(*ast.Ident); ok {
		return ident.Name == "__TYPE_INFERENCE_NEEDED"
	}

	return false
}

// inferVarDeclType attempts to infer the type for a variable declaration
func (p *LambdaTypeInferencePlugin) inferVarDeclType(ctx *varDeclContext, root ast.Node) {
	// Strategy 1: Try go/types first (most accurate)
	if p.typeInference != nil && p.typeInference.typesInfo != nil {
		// If go/types is available, try to get type from assignments
		varType := p.findVarTypeFromAssignments(ctx.varName, root)
		if varType != nil {
			// Update the ValueSpec with the inferred type
			ctx.varSpec.Type = p.typeToAST(varType)
			p.ctx.Logger.Debugf("Inferred type for var %s: %s", ctx.varName, varType)
			return
		}
	}

	// Strategy 2: Fallback to literal type inference (for basic cases)
	inferredType := p.inferVarTypeFromLiterals(ctx.varName, root)
	if inferredType != nil {
		ctx.varSpec.Type = inferredType
		p.ctx.Logger.Debugf("Inferred type for var %s from literals", ctx.varName)
		return
	}

	// Strategy 3: If no type found, report error
	p.ctx.Logger.Warnf("Could not infer type for var %s at %v",
		ctx.varName, p.ctx.FileSet.Position(ctx.pos))
}

// findVarTypeFromAssignments finds the type of a variable by analyzing assignments
func (p *LambdaTypeInferencePlugin) findVarTypeFromAssignments(varName string, root ast.Node) types.Type {
	var inferredType types.Type

	ast.Inspect(root, func(n ast.Node) bool {
		// Look for assignment statements
		if assign, ok := n.(*ast.AssignStmt); ok {
			// Check each LHS to see if it's our variable
			for i, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
					// Found an assignment to our variable
					// Get the RHS expression
					if i < len(assign.Rhs) {
						rhs := assign.Rhs[i]
						// Try to infer type from RHS using go/types
						if p.typeInference != nil {
							if typ, ok := p.typeInference.InferType(rhs); ok {
								inferredType = typ
								return false // Stop searching
							}
						}
					}
				}
			}
		}
		return true
	})

	return inferredType
}

// inferVarTypeFromLiterals infers variable type from literal assignments (fallback)
func (p *LambdaTypeInferencePlugin) inferVarTypeFromLiterals(varName string, root ast.Node) ast.Expr {
	var inferredType ast.Expr

	ast.Inspect(root, func(n ast.Node) bool {
		// Look for assignment statements
		if assign, ok := n.(*ast.AssignStmt); ok {
			// Check each LHS to see if it's our variable
			for i, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
					// Found an assignment to our variable
					if i < len(assign.Rhs) {
						rhs := assign.Rhs[i]
						// Infer from basic literal types
						if lit, ok := rhs.(*ast.BasicLit); ok {
							inferredType = p.literalToType(lit)
							return false // Stop searching
						}
					}
				}
			}
		}
		return true
	})

	return inferredType
}

// literalToType converts a basic literal to its type expression
func (p *LambdaTypeInferencePlugin) literalToType(lit *ast.BasicLit) ast.Expr {
	switch lit.Kind {
	case token.INT:
		return &ast.Ident{Name: "int"}
	case token.FLOAT:
		return &ast.Ident{Name: "float64"}
	case token.STRING:
		return &ast.Ident{Name: "string"}
	case token.CHAR:
		return &ast.Ident{Name: "rune"}
	default:
		return &ast.Ident{Name: "interface{}"}
	}
}

// inferFuncLiteralTypes attempts to infer parameter types for a function literal
func (p *LambdaTypeInferencePlugin) inferFuncLiteralTypes(ctx *funcLiteralContext) {
	if p.typeInference == nil || p.typeInference.typesInfo == nil {
		// No go/types info available - cannot infer types
		// This is expected for v1.0 - require explicit types
		p.reportTypeInferenceRequired(ctx)
		return
	}

	// Attempt to infer from call context
	inferred := p.inferFromCallContext(ctx)
	if !inferred {
		p.reportTypeInferenceRequired(ctx)
	}
}

// inferFromCallContext attempts to infer lambda parameter types from the call expression
func (p *LambdaTypeInferencePlugin) inferFromCallContext(ctx *funcLiteralContext) bool {
	// Get the function being called
	var funcType *types.Signature

	switch fun := ctx.callExpr.Fun.(type) {
	case *ast.SelectorExpr:
		// Method call: obj.method(lambda)
		funcType = p.inferFromMethodCall(fun, ctx)
	case *ast.Ident:
		// Function call: function(lambda)
		funcType = p.inferFromFunctionCall(fun, ctx)
	default:
		return false
	}

	if funcType == nil {
		return false
	}

	// Extract parameter types from function signature
	if ctx.argIndex >= funcType.Params().Len() {
		return false
	}

	param := funcType.Params().At(ctx.argIndex)
	paramType := param.Type()

	// Check if parameter type is a function type
	if sig, ok := paramType.Underlying().(*types.Signature); ok {
		// Apply inferred types to function literal parameters
		return p.applyInferredTypes(ctx.funcLit, sig)
	}

	return false
}

// inferFromMethodCall infers types from method call context
func (p *LambdaTypeInferencePlugin) inferFromMethodCall(sel *ast.SelectorExpr, ctx *funcLiteralContext) *types.Signature {
	if p.typeInference.typesInfo == nil {
		return nil
	}

	// Get the type of the receiver (X in X.method)
	recvType := p.typeInference.typesInfo.TypeOf(sel.X)
	if recvType == nil {
		return nil
	}

	// Look up the method
	methodName := sel.Sel.Name
	method := p.lookupMethod(recvType, methodName)
	if method == nil {
		return nil
	}

	if sig, ok := method.Type().(*types.Signature); ok {
		return sig
	}

	return nil
}

// inferFromFunctionCall infers types from function call context
func (p *LambdaTypeInferencePlugin) inferFromFunctionCall(ident *ast.Ident, ctx *funcLiteralContext) *types.Signature {
	if p.typeInference.typesInfo == nil {
		return nil
	}

	// Get the function type
	obj := p.typeInference.typesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}

	if sig, ok := obj.Type().(*types.Signature); ok {
		return sig
	}

	return nil
}

// lookupMethod finds a method by name on a type
func (p *LambdaTypeInferencePlugin) lookupMethod(typ types.Type, name string) *types.Func {
	// Handle pointer types
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// Handle named types
	if named, ok := typ.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			method := named.Method(i)
			if method.Name() == name {
				return method
			}
		}
	}

	return nil
}

// applyInferredTypes updates function literal parameters with inferred types
func (p *LambdaTypeInferencePlugin) applyInferredTypes(funcLit *ast.FuncLit, sig *types.Signature) bool {
	if funcLit.Type == nil || funcLit.Type.Params == nil {
		return false
	}

	params := funcLit.Type.Params.List
	sigParams := sig.Params()

	// Ensure parameter counts match
	paramCount := 0
	for _, field := range params {
		paramCount += len(field.Names)
	}

	if paramCount != sigParams.Len() {
		return false
	}

	// Apply types to each parameter
	sigIndex := 0
	for _, field := range params {
		// Check if this parameter needs type inference
		needsInference := field.Type == nil
		if !needsInference {
			if ident, ok := field.Type.(*ast.Ident); ok {
				needsInference = ident.Name == "__TYPE_INFERENCE_NEEDED"
			}
		}

		if !needsInference {
			// Skip parameters that already have concrete types
			sigIndex += len(field.Names)
			continue
		}

		// Get the type from the signature
		if sigIndex >= sigParams.Len() {
			return false
		}

		paramType := sigParams.At(sigIndex).Type()
		field.Type = p.typeToAST(paramType)
		sigIndex += len(field.Names)
	}

	// Apply return type if the signature has one and funcLit doesn't
	if sig.Results() != nil && sig.Results().Len() > 0 {
		if funcLit.Type.Results == nil || funcLit.Type.Results.NumFields() == 0 {
			// Add return type from signature
			funcLit.Type.Results = &ast.FieldList{
				List: make([]*ast.Field, sig.Results().Len()),
			}
			for i := 0; i < sig.Results().Len(); i++ {
				resultType := sig.Results().At(i).Type()
				funcLit.Type.Results.List[i] = &ast.Field{
					Type: p.typeToAST(resultType),
				}
			}
		}
	}

	return true
}

// typeToAST converts a go/types.Type to an ast.Expr
func (p *LambdaTypeInferencePlugin) typeToAST(typ types.Type) ast.Expr {
	typeName := typ.String()

	// Handle basic types
	if basic, ok := typ.(*types.Basic); ok {
		return &ast.Ident{Name: basic.Name()}
	}

	// Handle named types
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		if obj.Pkg() == nil {
			// Predeclared type
			return &ast.Ident{Name: obj.Name()}
		}
		// Qualified type
		return &ast.SelectorExpr{
			X:   &ast.Ident{Name: obj.Pkg().Name()},
			Sel: &ast.Ident{Name: obj.Name()},
		}
	}

	// Fallback: create identifier from string representation
	// This is not perfect but works for simple cases
	return &ast.Ident{Name: typeName}
}

// reportTypeInferenceRequired reports that type inference failed and explicit types are required
func (p *LambdaTypeInferencePlugin) reportTypeInferenceRequired(ctx *funcLiteralContext) {
	// For v1.0, we require explicit type annotations
	// This will be caught by the preprocessor's validation or later compilation
	p.ctx.Logger.Debugf("Lambda at %v: type inference not available, explicit types required",
		p.ctx.FileSet.Position(ctx.pos))
}
