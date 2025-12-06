package transpiler

import (
	"bytes"
	"fmt"
	"go/token"
	"sort"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/parser"
)

// transformASTExpressions finds and transforms all Dingo expressions (match, lambda)
// to Go code using the AST-based parser and codegen pipeline.
//
// Process:
// 1. Find all match/lambda expression locations using FindDingoExpressions
// 2. Sort by position descending (transform from end to avoid offset shifts)
// 3. For each expression:
//    a. Parse the expression using pkg/parser
//    b. Set IsExpr on MatchExpr based on context (Assignment/Return/Argument = true)
//    c. Generate Go code using pkg/codegen
//    d. Splice generated code back into result
// 4. Return transformed source and mappings
//
// Returns error immediately on any parse failure with byte offset information.
func transformASTExpressions(src []byte) ([]byte, []ast.SourceMapping, error) {
	// Find all Dingo expressions
	locations, err := ast.FindDingoExpressions(src)
	if err != nil {
		return nil, nil, fmt.Errorf("find expressions: %w", err)
	}

	// If no expressions found, return source unchanged
	if len(locations) == 0 {
		return src, nil, nil
	}

	// Sort by position descending (highest offset first)
	// This allows transformation from end to beginning, avoiding offset shifts
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Start > locations[j].Start
	})

	var mappings []ast.SourceMapping
	result := src

	// Transform each expression from end to beginning
	for _, loc := range locations {
		// Skip error propagation expressions - they're handled at statement level
		if loc.Kind == ast.ExprErrorProp {
			continue
		}

		// Extract expression source
		exprSrc := result[loc.Start:loc.End]

		// Handle expression types based on kind
		switch loc.Kind {
		case ast.ExprMatch, ast.ExprLambdaRust, ast.ExprLambdaTS, ast.ExprNullCoalesce, ast.ExprSafeNav:
			// Expression-level transformation (supported types)
		default:
			// Unknown expression kind - skip
			continue
		}

		// Parse the expression
		fset := token.NewFileSet()
		dingoNode, parseErr := parser.ParseExpr(fset, string(exprSrc))
		if parseErr != nil {
			// Skip if parsing fails for null coalesce/safe nav (may be partial implementation)
			if loc.Kind == ast.ExprNullCoalesce || loc.Kind == ast.ExprSafeNav {
				continue
			}
			return nil, nil, fmt.Errorf("parse expression at byte %d: %w", loc.Start, parseErr)
		}

		// Extract the actual Expr from DingoNode wrapper
		var expr ast.Expr
		if wrapped, ok := dingoNode.(*ast.ExprWrapper); ok {
			expr = wrapped.DingoExpr
		} else if astExpr, ok := dingoNode.(ast.Expr); ok {
			expr = astExpr
		} else {
			return nil, nil, fmt.Errorf("unexpected node type at byte %d: %T", loc.Start, dingoNode)
		}

		// Set IsExpr flag on MatchExpr based on context
		if matchExpr, ok := expr.(*ast.MatchExpr); ok {
			// Context determines if match needs IIFE wrapping
			matchExpr.IsExpr = loc.Context == ast.ContextAssignment ||
				loc.Context == ast.ContextReturn ||
				loc.Context == ast.ContextArgument
		}

		// Generate Go code with context for null coalesce/safe nav (human-like output)
		var genResult ast.CodeGenResult
		if (loc.Kind == ast.ExprNullCoalesce || loc.Kind == ast.ExprSafeNav) &&
			(loc.Context == ast.ContextReturn || loc.Context == ast.ContextAssignment) {
			// Create context for human-like code generation
			ctx := &codegen.GenContext{
				Context:        loc.Context,
				VarName:        loc.VarName,
				StatementStart: loc.StatementStart,
				StatementEnd:   loc.StatementEnd,
			}
			genResult = codegen.GenerateExprWithContext(expr, ctx)
		} else {
			genResult = codegen.GenerateExpr(expr)
		}

		if len(genResult.Output) == 0 && len(genResult.StatementOutput) == 0 {
			return nil, nil, fmt.Errorf("codegen produced no output for expression at byte %d", loc.Start)
		}

		// Determine what to replace and what to use as replacement
		var replaceStart, replaceEnd int
		var replacement []byte
		if len(genResult.StatementOutput) > 0 && loc.StatementStart > 0 {
			// Statement-level replacement (human-like output)
			replaceStart = loc.StatementStart
			replaceEnd = loc.StatementEnd
			replacement = genResult.StatementOutput
		} else {
			// Expression-level replacement (IIFE fallback)
			replaceStart = loc.Start
			replaceEnd = loc.End
			replacement = genResult.Output
		}

		// Splice generated code into result
		oldLen := replaceEnd - replaceStart
		newResult := make([]byte, 0, len(result)-oldLen+len(replacement))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, replacement...)
		newResult = append(newResult, result[replaceEnd:]...)
		result = newResult

		// Convert codegen mappings to SourceMapping
		// Adjust mapping positions based on splice location
		for _, m := range genResult.Mappings {
			mappings = append(mappings, ast.SourceMapping{
				DingoStart: loc.Start + m.DingoStart,
				DingoEnd:   loc.Start + m.DingoEnd,
				GoStart:    replaceStart + m.GoStart,
				GoEnd:      replaceStart + m.GoEnd,
				Kind:       m.Kind,
			})
		}
	}

	return result, mappings, nil
}

// transformErrorPropStatements transforms statement-level error propagation.
// This MUST run before expression-level transforms.
//
// Transforms:
//   let data = readFile(path)?  →  tmp, err := readFile(path); if err != nil { return ..., err }; data := tmp
//   x := foo()?                 →  tmp, err := foo(); if err != nil { return ..., err }; x := tmp
//   return foo()?               →  tmp, err := foo(); if err != nil { return ..., err }; return tmp
func transformErrorPropStatements(src []byte) ([]byte, []ast.SourceMapping, error) {
	locations, err := ast.FindErrorPropStatements(src)
	if err != nil {
		return src, nil, err
	}

	if len(locations) == 0 {
		return src, nil, nil
	}

	// Sort by position (descending) to avoid position shifting
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Start > locations[j].Start
	})

	result := src
	var mappings []ast.SourceMapping
	counter := 1 // Counter for unique variable names

	for _, loc := range locations {
		// Extract the expression between operator and ?
		exprBytes := src[loc.ExprStart : loc.ExprEnd-1] // -1 to exclude ?

		// Infer return types from enclosing function
		returnTypes := codegen.InferReturnTypes(result, loc.Start)

		// Generate statement-level code
		var generated []byte
		switch loc.Kind {
		case ast.StmtErrorPropAssign, ast.StmtErrorPropLet:
			// x := foo()? or let x = foo()?
			generated = generateErrorPropStatement(exprBytes, loc.VarName, returnTypes, &counter)
		case ast.StmtErrorPropReturn:
			// return foo()?
			generated = generateErrorPropReturn(exprBytes, returnTypes, &counter)
		}

		// Replace in result
		newResult := make([]byte, 0, len(result)-int(loc.End-loc.Start)+len(generated))
		newResult = append(newResult, result[:loc.Start]...)
		newResult = append(newResult, generated...)
		newResult = append(newResult, result[loc.End:]...)
		result = newResult

		// TODO: Add source mappings
	}

	return result, mappings, nil
}

// generateErrorPropStatement generates code for statement-level error propagation.
//
// Input:  expr = "readFile(path)", varName = "data", returnTypes = ["0", "\"\""], counter = 1
// Output:
//   tmp, err := readFile(path)
//   if err != nil {
//       return 0, "", err
//   }
//   data := tmp
//
// counter is incremented after use (tmp, err for first; tmp1, err1 for second; etc.)
func generateErrorPropStatement(expr []byte, varName string, returnTypes []string, counter *int) []byte {
	var buf bytes.Buffer

	// Generate unique variable names
	var tmpVar, errVar string
	if *counter == 1 {
		tmpVar = "tmp"
		errVar = "err"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", *counter-1)
		errVar = fmt.Sprintf("err%d", *counter-1)
	}
	*counter++

	// tmp, err := expr
	buf.WriteString(tmpVar)
	buf.WriteString(", ")
	buf.WriteString(errVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ..., err }
	buf.WriteString("if ")
	buf.WriteString(errVar)
	buf.WriteString(" != nil {\n\treturn ")
	for i, zv := range returnTypes {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(zv)
	}
	if len(returnTypes) > 0 {
		buf.WriteString(", ")
	}
	buf.WriteString(errVar)
	buf.WriteString("\n}\n")

	// varName := tmpVar (or varName = tmpVar for underscore)
	buf.WriteString(varName)
	if varName == "_" {
		buf.WriteString(" = ")
	} else {
		buf.WriteString(" := ")
	}
	buf.WriteString(tmpVar)

	return buf.Bytes()
}

// generateErrorPropReturn generates code for return statement error propagation.
//
// Input:  expr = "readFile(path)", returnTypes = ["0", "\"\""], counter = 1
// Output:
//   tmp, err := readFile(path)
//   if err != nil {
//       return 0, "", err
//   }
//   return tmp
//
// counter is incremented after use (tmp, err for first; tmp1, err1 for second; etc.)
func generateErrorPropReturn(expr []byte, returnTypes []string, counter *int) []byte {
	var buf bytes.Buffer

	// Generate unique variable names
	var tmpVar, errVar string
	if *counter == 1 {
		tmpVar = "tmp"
		errVar = "err"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", *counter-1)
		errVar = fmt.Sprintf("err%d", *counter-1)
	}
	*counter++

	// tmp, err := expr
	buf.WriteString(tmpVar)
	buf.WriteString(", ")
	buf.WriteString(errVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ..., err }
	buf.WriteString("if ")
	buf.WriteString(errVar)
	buf.WriteString(" != nil {\n\treturn ")
	for i, zv := range returnTypes {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(zv)
	}
	if len(returnTypes) > 0 {
		buf.WriteString(", ")
	}
	buf.WriteString(errVar)
	buf.WriteString("\n}\n")

	// return tmpVar
	buf.WriteString("return ")
	buf.WriteString(tmpVar)

	return buf.Bytes()
}
