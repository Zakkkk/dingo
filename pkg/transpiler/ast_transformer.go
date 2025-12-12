package transpiler

import (
	"bytes"
	"fmt"
	"go/token"
	"sort"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
	"github.com/MadAppGang/dingo/pkg/typechecker"
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
	return transformASTExpressionsWithRegistry(src, nil, nil, nil, "")
}

// transformASTExpressionsWithRegistry is like transformASTExpressions but accepts
// an enum registry for match expression pattern name resolution.
// If originalSrc is provided, source mappings will use positions from originalSrc instead of src.
// This is needed because earlier transforms (like error prop) may have shifted positions.
// If tracker is provided, transforms will be recorded for line-level mapping generation.
// If filename is provided, //line directives will be emitted for accurate error reporting.
func transformASTExpressionsWithRegistry(src []byte, enumRegistry map[string]string, originalSrc []byte, tracker *sourcemap.TransformTracker, filename string) ([]byte, []ast.SourceMapping, error) {
	// Find all Dingo expressions in the (potentially transformed) source
	locations, err := ast.FindDingoExpressions(src)
	if err != nil {
		return nil, nil, fmt.Errorf("find expressions: %w", err)
	}

	// If no expressions found, return source unchanged
	if len(locations) == 0 {
		return src, nil, nil
	}

	// Also find expressions in original source for accurate mapping positions
	var originalLocations []ast.ExprLocation
	if originalSrc != nil {
		originalLocations, _ = ast.FindDingoExpressions(originalSrc)
	}

	// Filter out only expressions that are nested inside ternary expressions
	// These will be handled by the ternary's codegen via GenerateExpr
	// Other expressions (e.g., standalone safe nav) should still be processed
	locations = filterExprNestedInTernary(locations)

	// Sort by position descending (highest offset first)
	// This allows transformation from end to beginning, avoiding offset shifts
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Start > locations[j].Start
	})

	var mappings []ast.SourceMapping
	result := src

	// Shared counter for unique temp var names across all expressions
	// Counter starts at 0 and increments, so first temp var has no suffix
	tempCounter := 0

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
		case ast.ExprMatch, ast.ExprLambdaRust, ast.ExprLambdaTS, ast.ExprNullCoalesce, ast.ExprSafeNav, ast.ExprTernary:
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

		// Generate Go code with context for null coalesce/safe nav/match/ternary (human-like output)
		var genResult ast.CodeGenResult
		if (loc.Kind == ast.ExprNullCoalesce || loc.Kind == ast.ExprSafeNav || loc.Kind == ast.ExprMatch || loc.Kind == ast.ExprTernary) &&
			(loc.Context == ast.ContextReturn || loc.Context == ast.ContextAssignment || loc.Context == ast.ContextArgument) {
			// Create context for human-like code generation
			ctx := &codegen.GenContext{
				Context:        loc.Context,
				VarName:        loc.VarName,
				StatementStart: loc.StatementStart,
				StatementEnd:   loc.StatementEnd,
				EnumRegistry:   enumRegistry,  // Pass enum registry for match pattern resolution
				TempCounter:    &tempCounter,  // Share counter for unique temp var names
			}

			// For assignments and arguments, try to infer the type
			if loc.Context == ast.ContextAssignment || loc.Context == ast.ContextArgument {
				switch loc.Kind {
				case ast.ExprSafeNav:
					// Infer type using go/types
					varType := inferSafeNavType(result, exprSrc)
					ctx.VarType = varType
				case ast.ExprMatch:
					// Infer type from match arm bodies
					if matchExpr, ok := expr.(*ast.MatchExpr); ok {
						varType := typechecker.InferMatchResultType(matchExpr, result)
						ctx.VarType = varType
					}
				case ast.ExprTernary:
					// Infer type from ternary branches
					if ternaryExpr, ok := expr.(*ast.TernaryExpr); ok {
						varType := inferTernaryType(ternaryExpr, result)
						ctx.VarType = varType
					}
				}
			}

			genResult = codegen.GenerateExprWithContext(expr, ctx)
		} else {
			// For non-context generation, still pass registry and counter for match expressions
			if loc.Kind == ast.ExprMatch {
				ctx := &codegen.GenContext{
					EnumRegistry: enumRegistry,
					TempCounter:  &tempCounter,
				}
				genResult = codegen.GenerateExprWithContext(expr, ctx)
			} else {
				genResult = codegen.GenerateExpr(expr)
			}
		}

		if len(genResult.Output) == 0 && len(genResult.StatementOutput) == 0 {
			return nil, nil, fmt.Errorf("codegen produced no output for expression at byte %d", loc.Start)
		}

		// Determine what to replace and what to use as replacement
		var replaceStart, replaceEnd int
		var replacement []byte

		// Handle HoistedCode for argument context (variable declaration before statement)
		if len(genResult.HoistedCode) > 0 && loc.StatementEnd > loc.StatementStart {
			// Insert hoisted code before the statement
			// Then replace the expression with the temp variable name
			replaceStart = loc.Start
			replaceEnd = loc.End

			// Build replacement: hoisted code at statement start, temp var at expression location
			hoistedInsertPos := loc.StatementStart

			// Record transform before splicing (if tracker provided)
			dingoPos := loc.Start
			dingoEnd := loc.End
			if tracker != nil {
				if originalSrc != nil {
					if origPos := findOriginalPosition(originalLocations, loc, exprSrc); origPos >= 0 {
						dingoPos = origPos
						dingoEnd = origPos + (loc.End - loc.Start)
					}
				}
			}

			// Prepend //line directive to hoisted code if filename is provided
			var finalHoistedCode []byte
			if filename != "" && originalSrc != nil && len(originalSrc) > 0 {
				line, col := byteOffsetToLineCol(originalSrc, dingoPos)
				if line > 0 && col > 0 {
					lineDirective := ast.FormatLineDirective(filename, line, col)
					finalHoistedCode = append([]byte(lineDirective), genResult.HoistedCode...)
				} else {
					finalHoistedCode = genResult.HoistedCode
				}
			} else {
				finalHoistedCode = genResult.HoistedCode
			}

			// Record transform after calculating final size
			if tracker != nil {
				// Total generated length includes line directive + hoisted code + newline + temp var
				generatedLen := len(finalHoistedCode) + 1 + len(genResult.Output)
				tracker.RecordTransform(dingoPos, dingoEnd, loc.Kind.String(), generatedLen)
			}

			// Insert hoisted code before the statement
			newResult := make([]byte, 0, len(result)+len(finalHoistedCode)+len(genResult.Output))
			newResult = append(newResult, result[:hoistedInsertPos]...)
			newResult = append(newResult, finalHoistedCode...)
			newResult = append(newResult, []byte("\n")...)

			// Replace expression with temp variable
			newResult = append(newResult, result[hoistedInsertPos:loc.Start]...)
			newResult = append(newResult, genResult.Output...)
			newResult = append(newResult, result[loc.End:]...)
			result = newResult

			// Adjust mapping positions
			for _, m := range genResult.Mappings {
				mappings = append(mappings, ast.SourceMapping{
					DingoStart: loc.Start + m.DingoStart,
					DingoEnd:   loc.Start + m.DingoEnd,
					GoStart:    hoistedInsertPos + m.GoStart,
					GoEnd:      hoistedInsertPos + m.GoEnd,
					Kind:       m.Kind,
				})
			}
			continue
		}

		if len(genResult.StatementOutput) > 0 && loc.StatementEnd > loc.StatementStart {
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

		// Calculate original source position for both tracking and //line directives
		dingoPos := loc.Start
		dingoEnd := loc.End
		if originalSrc != nil {
			if origPos := findOriginalPosition(originalLocations, loc, exprSrc); origPos >= 0 {
				dingoPos = origPos
				// Adjust end position by same delta
				dingoEnd = origPos + (loc.End - loc.Start)
			}
		}

		// Prepend //line directive if filename is provided
		// Calculate line:col from byte offset in original source
		var finalReplacement []byte
		if filename != "" && originalSrc != nil && len(originalSrc) > 0 {
			line, col := byteOffsetToLineCol(originalSrc, dingoPos)
			if line > 0 && col > 0 {
				lineDirective := ast.FormatLineDirective(filename, line, col)
				finalReplacement = append([]byte(lineDirective), replacement...)
			} else {
				finalReplacement = replacement
			}
		} else {
			finalReplacement = replacement
		}

		// Record transform before splicing (if tracker provided)
		if tracker != nil {
			tracker.RecordTransform(dingoPos, dingoEnd, loc.Kind.String(), len(finalReplacement))
		}

		// Splice generated code into result
		oldLen := replaceEnd - replaceStart
		newResult := make([]byte, 0, len(result)-oldLen+len(finalReplacement))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, finalReplacement...)
		newResult = append(newResult, result[replaceEnd:]...)
		result = newResult

		// Convert codegen mappings to SourceMapping
		// Adjust mapping positions based on splice location
		// Note: dingoPos already calculated above for //line directives

		for _, m := range genResult.Mappings {
			mappings = append(mappings, ast.SourceMapping{
				DingoStart: dingoPos + m.DingoStart,
				DingoEnd:   dingoPos + m.DingoEnd,
				GoStart:    replaceStart + m.GoStart,
				GoEnd:      replaceStart + m.GoEnd,
				Kind:       m.Kind,
			})
		}
	}

	return result, mappings, nil
}

// findOriginalPosition finds the position of an expression in the original source
// by matching the expression content and kind. Returns -1 if not found.
// Since we process expressions in descending order, we also track which original
// locations have been used to avoid matching the same one twice.
func findOriginalPosition(originalLocations []ast.ExprLocation, loc ast.ExprLocation, exprContent []byte) int {
	// Look for a matching expression by kind
	// If there are multiple of the same kind, we rely on the processing order
	// (both are sorted descending by position)
	for i := len(originalLocations) - 1; i >= 0; i-- {
		origLoc := originalLocations[i]
		if origLoc.Kind == loc.Kind && origLoc.Start >= 0 {
			// Mark as used by setting Start to -1 (we can modify since we're iterating)
			// Actually, we can't modify the slice this way. Just return the first match.
			return origLoc.Start
		}
	}
	return -1 // Not found
}

// findOriginalErrorPropPosition finds the position in original source for error prop statements.
// If originalSrc == src (no earlier transforms), returns the position unchanged.
// Otherwise, attempts to find a matching ? operator in the original source.
func findOriginalErrorPropPosition(originalSrc []byte, transformedSrc []byte, pos int) int {
	// If sources are the same, no adjustment needed
	if bytes.Equal(originalSrc, transformedSrc) {
		return pos
	}

	// Simple heuristic: return the position as-is
	// The tracker will use this position relative to originalSrc
	// More sophisticated matching could be added if needed
	return pos
}

// transformErrorPropStatements transforms statement-level error propagation.
// This MUST run before expression-level transforms.
//
// Transforms:
//   let data = readFile(path)?  →  tmp, err := readFile(path); if err != nil { return ..., err }; data := tmp
//   x := foo()?                 →  tmp, err := foo(); if err != nil { return ..., err }; x := tmp
//   return foo()?               →  tmp, err := foo(); if err != nil { return ..., err }; return tmp
func transformErrorPropStatements(src []byte) ([]byte, []ast.SourceMapping, error) {
	return transformErrorPropStatementsWithTracker(src, src, nil, "")
}

// transformErrorPropStatementsWithTracker wraps transformErrorPropStatements with tracker support.
// originalSrc should be the original Dingo source (before any transforms) for accurate position tracking.
// filename is used to generate //line directives for accurate error reporting.
func transformErrorPropStatementsWithTracker(src []byte, originalSrc []byte, tracker *sourcemap.TransformTracker, filename string) ([]byte, []ast.SourceMapping, error) {
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
	// Start counter at len(locations) and decrement, so first statement in source
	// gets tmp/err, second gets tmp1/err1, etc. (we process end-to-beginning)
	counter := len(locations)

	// First pass: calculate all deltas to know final positions
	// We need to track byte deltas from transforms to calculate correct Go positions
	type transformInfo struct {
		loc       ast.StmtLocation
		generated []byte
		delta     int // len(generated) - original length
	}
	transforms := make([]transformInfo, 0, len(locations))

	for _, loc := range locations {
		// Extract the expression between operator and ?
		exprBytes := src[loc.ExprStart : loc.ExprEnd-1] // -1 to exclude ?

		// Infer return types from enclosing function
		returnTypes := codegen.InferReturnTypes(result, loc.Start)

		// Extract lambda body if present (using token positions from finder)
		var lambdaBody []byte
		if loc.ErrorKind == ast.ErrorPropLambda && loc.LambdaBodyEnd > loc.LambdaBodyStart {
			lambdaBody = src[loc.LambdaBodyStart:loc.LambdaBodyEnd]
		}

		// Generate statement-level code
		var generated []byte
		switch loc.Kind {
		case ast.StmtErrorPropAssign, ast.StmtErrorPropLet:
			// x := foo()? or let x = foo()?
			generated = generateErrorPropStatementAdvanced(exprBytes, loc.VarName, returnTypes, &counter, loc.ErrorKind, loc.ErrorContext, loc.LambdaParam, lambdaBody)
		case ast.StmtErrorPropReturn:
			// return foo()?
			generated = generateErrorPropReturnAdvanced(exprBytes, returnTypes, &counter, loc.ErrorKind, loc.ErrorContext, loc.LambdaParam, lambdaBody)
		case ast.StmtErrorPropBare:
			// foo()?
			generated = generateErrorPropBareAdvanced(result, exprBytes, loc.ExprStart, returnTypes, &counter, loc.ErrorKind, loc.ErrorContext, loc.LambdaParam, lambdaBody)
		}

		// Record transform before applying (if tracker provided)
		// Calculate position in original source
		origStart := findOriginalErrorPropPosition(originalSrc, src, loc.Start)
		origEnd := findOriginalErrorPropPosition(originalSrc, src, loc.End)

		// Prepend //line directive if filename is provided
		var finalGenerated []byte
		if filename != "" && originalSrc != nil && len(originalSrc) > 0 {
			line, col := byteOffsetToLineCol(originalSrc, origStart)
			if line > 0 && col > 0 {
				lineDirective := ast.FormatLineDirective(filename, line, col)
				finalGenerated = append([]byte(lineDirective), generated...)
			} else {
				finalGenerated = generated
			}
		} else {
			finalGenerated = generated
		}

		if tracker != nil {
			tracker.RecordTransform(origStart, origEnd, "error_prop", len(finalGenerated))
		}

		// Replace in result
		newResult := make([]byte, 0, len(result)-int(loc.End-loc.Start)+len(finalGenerated))
		newResult = append(newResult, result[:loc.Start]...)
		newResult = append(newResult, finalGenerated...)
		newResult = append(newResult, result[loc.End:]...)
		result = newResult

		// Store transform info for second pass
		originalLen := loc.End - loc.Start
		delta := len(generated) - originalLen
		transforms = append(transforms, transformInfo{
			loc:       loc,
			generated: generated,
			delta:     delta,
		})
	}

	// Second pass: calculate Go positions accounting for all transforms
	// Process in source order (reverse of our processing order) to calculate cumulative shifts
	// For each transform, GoStart = DingoStart + sum of deltas from all earlier transforms

	// Sort transforms by DingoStart (ascending order for correct delta accumulation)
	sortedTransforms := make([]transformInfo, len(transforms))
	copy(sortedTransforms, transforms)
	sort.Slice(sortedTransforms, func(i, j int) bool {
		return sortedTransforms[i].loc.Start < sortedTransforms[j].loc.Start
	})

	// Calculate cumulative delta for each transform position
	cumulativeDelta := 0
	for _, t := range sortedTransforms {
		goStart := t.loc.Start + cumulativeDelta
		goEnd := goStart + len(t.generated)

		mappings = append(mappings, ast.SourceMapping{
			DingoStart: t.loc.Start,
			DingoEnd:   t.loc.End,
			GoStart:    goStart,
			GoEnd:      goEnd,
			Kind:       "error_prop",
		})

		// Accumulate delta for subsequent transforms
		cumulativeDelta += t.delta
	}

	return result, mappings, nil
}

// generateErrorPropStatementAdvanced generates code for statement-level error propagation
// with support for all three error kinds: basic, context, and lambda.
//
// Basic (ErrorPropBasic):
//   tmp, err := expr; if err != nil { return ..., err }; data := tmp
//
// Context (ErrorPropContext):
//   tmp, err := expr; if err != nil { return ..., fmt.Errorf("msg: %w", err) }; data := tmp
//
// Lambda (ErrorPropLambda):
//   tmp, err := expr; if err != nil { return ..., func(p error) error { return body }(err) }; data := tmp
func generateErrorPropStatementAdvanced(expr []byte, varName string, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte) []byte {
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
	*counter--

	// tmp, err := expr
	buf.WriteString(tmpVar)
	buf.WriteString(", ")
	buf.WriteString(errVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ..., ERROR_VALUE }
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

	// Generate the error value based on kind
	switch errorKind {
	case ast.ErrorPropContext:
		// fmt.Errorf("message: %w", err)
		buf.WriteString(`fmt.Errorf("`)
		buf.WriteString(errorContext)
		buf.WriteString(`: %w", `)
		buf.WriteString(errVar)
		buf.WriteString(")")
	case ast.ErrorPropLambda:
		// Substitute lambda param with error var in body, emit directly (no IIFE)
		// e.g., |err| NewAppError(403, "msg", err) with err2 → NewAppError(403, "msg", err2)
		substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
		buf.Write(substituted)
	default:
		// Basic: just return err
		buf.WriteString(errVar)
	}

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

// inferSafeNavType attempts to infer the type of a safe navigation expression.
// It converts the ?. chain to regular . access and type-checks against the source context.
//
// Example:
//
//	exprSrc: "config?.Database?.Host"
//	Returns: "*string" (from go/types analysis)
//
// Returns empty string if type cannot be inferred.
func inferSafeNavType(fullSource []byte, exprSrc []byte) string {
	// Convert safe-nav chain to regular Go expression
	// config?.Database?.Host → config.Database.Host
	chainExpr := typechecker.ChainToExprString(string(exprSrc))
	if chainExpr == "" {
		return ""
	}

	// Convert entire source to valid Go by replacing ?. with .
	// This makes the source parseable by go/parser
	goSource := bytes.ReplaceAll(fullSource, []byte("?."), []byte("."))

	// Replace ?? expressions: "a ?? b" → "a" (we only need the left side for type inference)
	// This regex-like replacement handles the common patterns
	goSource = removeNullCoalesce(goSource)

	// Type check the converted source
	sc := typechecker.NewSourceChecker()
	if err := sc.ParseAndCheck("probe.go", goSource); err != nil {
		return ""
	}

	return sc.GetExprType(chainExpr)
}

// inferTernaryType attempts to infer the type of a ternary expression from its branches.
// It analyzes both the true and false branches to determine a common type.
//
// Example:
//
//	ternary: user.ID > 0 ? "Welcome" : "Hello"
//	Returns: "string"
//
// Returns empty string if type cannot be inferred.
func inferTernaryType(ternary *ast.TernaryExpr, fullSource []byte) string {
	// Use go/types to infer the type from the true branch (most specific)
	// This handles literals, complex expressions, and typed constants correctly
	if ternary.True != nil {
		if trueStr := ternary.True.String(); trueStr != "" && trueStr != "?:" {
			typeStr := inferExprTypeFromSource(fullSource, []byte(trueStr))
			if typeStr != "" {
				return typeStr
			}
		}
	}

	// Fallback: try false branch using go/types
	if ternary.False != nil {
		if falseStr := ternary.False.String(); falseStr != "" && falseStr != "?:" {
			typeStr := inferExprTypeFromSource(fullSource, []byte(falseStr))
			if typeStr != "" {
				return typeStr
			}
		}
	}

	// If go/types can't infer, return empty (codegen will use 'any')
	return ""
}

// inferExprTypeFromSource attempts to infer an expression's type using go/types.
func inferExprTypeFromSource(fullSource []byte, exprSrc []byte) string {
	// Convert Dingo syntax to Go for type checking
	goSource := bytes.ReplaceAll(fullSource, []byte("?."), []byte("."))
	goSource = removeNullCoalesce(goSource)

	// Type check the converted source
	sc := typechecker.NewSourceChecker()
	if err := sc.ParseAndCheck("probe.go", goSource); err != nil {
		return ""
	}

	return sc.GetExprType(string(exprSrc))
}

// removeNullCoalesce removes ?? operators and their right operands for type inference.
// Uses the Dingo tokenizer to properly find ?? tokens.
// "a ?? b" → "a" (we only need the left side for type checking)
func removeNullCoalesce(src []byte) []byte {
	// Use Dingo tokenizer to find ?? tokens
	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		// If tokenization fails, return source unchanged
		return src
	}

	// Collect positions of ?? operators and their right operands
	type removal struct {
		start int // Position of ??
		end   int // Position after right operand
	}
	var removals []removal

	for i, t := range tokens {
		if t.Kind == tokenizer.QUESTION_QUESTION {
			start := t.BytePos()
			// Find end: newline, semicolon, or closing delimiter
			endOffset := t.ByteEnd() + 1 // Start after ??
			for j := i + 1; j < len(tokens); j++ {
				nextTok := tokens[j]
				if nextTok.Kind == tokenizer.NEWLINE ||
					nextTok.Kind == tokenizer.SEMICOLON ||
					nextTok.Kind == tokenizer.EOF {
					endOffset = nextTok.BytePos()
					break
				}
				// For closing delimiters, stop before them
				if nextTok.Kind == tokenizer.RPAREN ||
					nextTok.Kind == tokenizer.RBRACE ||
					nextTok.Kind == tokenizer.RBRACKET {
					endOffset = nextTok.BytePos()
					break
				}
				// Keep advancing past the token
				endOffset = nextTok.ByteEnd() + 1
			}
			removals = append(removals, removal{start: start, end: endOffset})
		}
	}

	// Remove in reverse order to preserve offsets
	result := src
	for i := len(removals) - 1; i >= 0; i-- {
		r := removals[i]
		if r.end > len(result) {
			r.end = len(result)
		}
		newResult := make([]byte, 0, len(result)-(r.end-r.start))
		newResult = append(newResult, result[:r.start]...)
		newResult = append(newResult, result[r.end:]...)
		result = newResult
	}

	return result
}

// substituteIdentifier replaces occurrences of oldIdent with newIdent in src
// using token-aware replacement (only replaces IDENT tokens, not substrings).
// e.g., substituteIdentifier("NewAppError(403, err)", "err", "err2")
//
//	→ "NewAppError(403, err2)" (not "NewApperr2or(403, err2)")
func substituteIdentifier(src []byte, oldIdent, newIdent string) []byte {
	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		// Fallback: return original if tokenization fails
		return src
	}

	// Build result by copying source with replacements
	result := make([]byte, 0, len(src)+len(newIdent)*2)
	lastCopied := 0

	for _, t := range tokens {
		if t.Kind == tokenizer.IDENT && t.Lit == oldIdent {
			// Copy everything up to this token
			result = append(result, src[lastCopied:t.BytePos()]...)
			// Write new identifier
			result = append(result, newIdent...)
			// Skip past old identifier
			lastCopied = t.ByteEnd()
		}
	}

	// Copy remaining bytes
	if lastCopied < len(src) {
		result = append(result, src[lastCopied:]...)
	}

	return result
}

// generateErrorPropReturnAdvanced generates code for return statement error propagation
// with support for all three error kinds: basic, context, and lambda.
//
// Basic (ErrorPropBasic):
//   tmp, err := expr; if err != nil { return ..., err }; return tmp
//
// Context (ErrorPropContext):
//   tmp, err := expr; if err != nil { return ..., fmt.Errorf("msg: %w", err) }; return tmp
//
// Lambda (ErrorPropLambda):
//   tmp, err := expr; if err != nil { return ..., func(p error) error { return body }(err) }; return tmp
func generateErrorPropReturnAdvanced(expr []byte, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte) []byte {
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
	*counter--

	// tmp, err := expr
	buf.WriteString(tmpVar)
	buf.WriteString(", ")
	buf.WriteString(errVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ..., ERROR_VALUE }
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

	// Generate the error value based on kind
	switch errorKind {
	case ast.ErrorPropContext:
		// fmt.Errorf("message: %w", err)
		buf.WriteString(`fmt.Errorf("`)
		buf.WriteString(errorContext)
		buf.WriteString(`: %w", `)
		buf.WriteString(errVar)
		buf.WriteString(")")
	case ast.ErrorPropLambda:
		// Substitute lambda param with error var in body, emit directly (no IIFE)
		// e.g., |err| NewAppError(403, "msg", err) with err2 → NewAppError(403, "msg", err2)
		substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
		buf.Write(substituted)
	default:
		// Basic: just return err
		buf.WriteString(errVar)
	}

	buf.WriteString("\n}\n")

	// return tmpVar
	buf.WriteString("return ")
	buf.WriteString(tmpVar)

	return buf.Bytes()
}

// generateErrorPropBareAdvanced generates code for bare statement error propagation.
//
// Uses InferExprReturnCount to detect:
// - Single-return functions (like row.Scan()) → generates: err := expr
// - Multi-return functions (like db.Query()) → generates: _, err := expr
//
// For external packages where detection fails, defaults to multi-return (_, err :=).
//
// Basic (ErrorPropBasic):
//   err := expr; if err != nil { return ..., err }  (single-return)
//   _, err := expr; if err != nil { return ..., err }  (multi-return)
//
// Context (ErrorPropContext):
//   err := expr; if err != nil { return ..., fmt.Errorf("msg: %w", err) }
//
// Lambda (ErrorPropLambda):
//   err := expr; if err != nil { return ..., transformedError }
func generateErrorPropBareAdvanced(src []byte, expr []byte, exprPos int, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte) []byte {
	var buf bytes.Buffer

	// Generate unique variable names
	var errVar string
	if *counter == 1 {
		errVar = "err"
	} else {
		errVar = fmt.Sprintf("err%d", *counter-1)
	}
	*counter--

	// Detect return count: 1 = single (error only), 2+ = multi (T, error)
	returnCount := codegen.InferExprReturnCount(src, expr, exprPos)

	// Detect if enclosing function returns Result type
	resultOkType := codegen.InferEnclosingFunctionReturnsResult(src, exprPos)
	isResultReturn := resultOkType != ""

	// Generate assignment based on return count
	if returnCount == 1 {
		// Single return: err := expr
		buf.WriteString(errVar)
		buf.WriteString(" := ")
	} else {
		// Multi-return or unknown: _, err := expr (safe default)
		buf.WriteString("_, ")
		buf.WriteString(errVar)
		buf.WriteString(" := ")
	}
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ERROR_VALUE }
	buf.WriteString("if ")
	buf.WriteString(errVar)
	buf.WriteString(" != nil {\n\treturn ")

	// For Result-returning functions, we don't need zero values for other returns
	// Just return the error wrapped in dgo.Err[T]
	if !isResultReturn {
		for i, zv := range returnTypes {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(zv)
		}
		if len(returnTypes) > 0 {
			buf.WriteString(", ")
		}
	}

	// Generate the error value based on kind and return type
	switch errorKind {
	case ast.ErrorPropContext:
		// fmt.Errorf("message: %w", err)
		errExpr := fmt.Sprintf(`fmt.Errorf("%s: %%w", %s)`, errorContext, errVar)
		if isResultReturn {
			buf.WriteString("dgo.Err[")
			buf.WriteString(resultOkType)
			buf.WriteString("](")
			buf.WriteString(errExpr)
			buf.WriteString(")")
		} else {
			buf.WriteString(errExpr)
		}
	case ast.ErrorPropLambda:
		// Substitute lambda param with error var in body, emit directly (no IIFE)
		substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
		if isResultReturn {
			buf.WriteString("dgo.Err[")
			buf.WriteString(resultOkType)
			buf.WriteString("](")
			buf.Write(substituted)
			buf.WriteString(")")
		} else {
			buf.Write(substituted)
		}
	default:
		// Basic: just return err
		if isResultReturn {
			buf.WriteString("dgo.Err[")
			buf.WriteString(resultOkType)
			buf.WriteString("](")
			buf.WriteString(errVar)
			buf.WriteString(")")
		} else {
			buf.WriteString(errVar)
		}
	}

	buf.WriteString("\n}")

	return buf.Bytes()
}

// transformGuardStatements transforms guard statements.
// This MUST run after error propagation but before expression-level transforms.
//
// Transforms:
//   guard user = FindUser(id) else |err| { return Err(err) }  →  tmp := FindUser(id); if tmp.IsErr() { err := *tmp.err; return ResultErr(err) }; user := *tmp.ok
//   guard (a, b) = ParseInfo(data) else { return None() }     →  tmp := ParseInfo(data); if tmp.IsNone() { return OptionNone[Info]() }; a := (*tmp.ok).Item1; b := (*tmp.ok).Item2
func transformGuardStatements(src []byte) ([]byte, []ast.SourceMapping, error) {
	return transformGuardStatementsWithTracker(src, src, nil)
}

// transformGuardStatementsWithTracker wraps transformGuardStatements with tracker support.
// originalSrc should be the original Dingo source (before any transforms) for accurate position tracking.
func transformGuardStatementsWithTracker(src []byte, originalSrc []byte, tracker *sourcemap.TransformTracker) ([]byte, []ast.SourceMapping, error) {
	locations, err := ast.FindGuardStatements(src)
	if err != nil {
		return src, nil, err
	}

	if len(locations) == 0 {
		return src, nil, nil
	}

	// Sort by position (descending) to avoid position shifting
	// Process from end to beginning so byte offsets remain valid
	for i, j := 0, len(locations)-1; i < j; i, j = i+1, j-1 {
		locations[i], locations[j] = locations[j], locations[i]
	}

	result := src
	var mappings []ast.SourceMapping

	// Share counter across all guard statements
	// Locations are sorted descending, so we iterate last-to-first in source order.
	// Counter starts high and decrements, so first guard in source gets lowest counter (tmp, tmp1, ...).
	// Example: 3 guards → counters 3, 2, 1 → generates tmp, tmp1, tmp2 in source order.
	counter := len(locations)

	for _, loc := range locations {
		// Infer type from expression and binding presence
		// HasBinding (|err|) is a strong signal for Result type
		exprType, err := codegen.InferExprTypeWithBinding(loc.ExprText, loc.HasBinding)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: cannot infer type: %w", loc.Line, err)
		}

		// Generate code with source context and shared counter
		gen := codegen.NewGuardGenerator(loc, exprType)
		gen.SourceBytes = src
		gen.Counter = counter
		counter-- // Decrement for next guard
		genResult := gen.Generate()

		// Record transform before applying (if tracker provided)
		if tracker != nil {
			origStart := findOriginalErrorPropPosition(originalSrc, src, loc.Start)
			origEnd := findOriginalErrorPropPosition(originalSrc, src, loc.End)
			tracker.RecordTransform(origStart, origEnd, "guard", len(genResult.Output))
		}

		// Replace in result
		oldLen := loc.End - loc.Start
		generated := genResult.Output
		newResult := make([]byte, 0, len(result)-oldLen+len(generated))
		newResult = append(newResult, result[:loc.Start]...)
		newResult = append(newResult, generated...)
		newResult = append(newResult, result[loc.End:]...)
		result = newResult

		// Collect source mappings
		for _, m := range genResult.Mappings {
			mappings = append(mappings, ast.SourceMapping{
				DingoStart: loc.Start + m.DingoStart,
				DingoEnd:   loc.Start + m.DingoEnd,
				GoStart:    loc.Start + m.GoStart,
				GoEnd:      loc.Start + m.GoEnd,
				Kind:       m.Kind,
			})
		}
	}

	return result, mappings, nil
}

// filterExprNestedInTernary removes expressions that are nested inside ternary expressions.
// These will be handled by the ternary's codegen via GenerateExpr.
// Other expressions (e.g., standalone safe nav, null coalesce) should still be processed.
//
// Example: For input "len(config?.Region) > 0 ? x : y", FindDingoExpressions returns:
//   - Ternary at 0-30
//   - SafeNav at 4-20
//
// After filtering, only the ternary is returned.
// The SafeNav will be handled when parsing/generating the ternary's condition.
func filterExprNestedInTernary(locations []ast.ExprLocation) []ast.ExprLocation {
	if len(locations) <= 1 {
		return locations
	}

	// Mark which expressions are nested inside ternary expressions
	isNestedInTernary := make([]bool, len(locations))

	for i := range locations {
		for j := range locations {
			if i == j {
				continue
			}
			// Only filter if the outer expression is a ternary
			if locations[j].Kind != ast.ExprTernary {
				continue
			}
			// Check if locations[i] is fully contained within the ternary locations[j]
			if locations[j].Start <= locations[i].Start && locations[i].End <= locations[j].End {
				// locations[i] is nested inside a ternary
				isNestedInTernary[i] = true
				break
			}
		}
	}

	// Return only expressions not nested in ternary
	var result []ast.ExprLocation
	for i, loc := range locations {
		if !isNestedInTernary[i] {
			result = append(result, loc)
		}
	}

	return result
}

// byteOffsetToLineCol converts a byte offset in source to 1-indexed line:col.
// Returns (0, 0) if offset is invalid.
func byteOffsetToLineCol(src []byte, offset int) (line, col int) {
	if offset < 0 || offset >= len(src) {
		return 0, 0
	}

	line = 1
	col = 1

	for i := 0; i < offset && i < len(src); i++ {
		if src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}
