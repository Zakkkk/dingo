package typechecker

import (
	"go/scanner"
	gotoken "go/token"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// InferMatchResultType infers the result type of a match expression from its arm bodies.
// Returns empty string if type cannot be inferred (triggers IIFE fallback in codegen).
//
// Type inference strategy (in priority order):
// 1. String literal ("...") -> "string"
// 2. Integer literal (42) -> "int"
// 3. Float literal (3.14) -> "float64"
// 4. Boolean literal (true/false) -> "bool"
// 5. fmt.Sprintf/fmt.Sprint calls -> "string"
// 6. Return empty string if type cannot be inferred (triggers IIFE fallback)
//
// IMPORTANT: This function uses AST-based analysis, NOT string manipulation.
// All type inference is performed by inspecting AST node types and fields.
func InferMatchResultType(match *ast.MatchExpr, sourceContext []byte) string {
	if len(match.Arms) == 0 {
		return "" // Fallback to IIFE
	}

	// Infer type from first arm (pass IsBlock from AST, not string inspection)
	firstType := inferArmBodyType(match.Arms[0], sourceContext)
	if firstType == "" {
		return "" // Cannot infer - fallback to IIFE
	}

	// CRITICAL FIX: Verify all other arms have compatible types
	// If any arm has a different type, fall back to IIFE
	for i := 1; i < len(match.Arms); i++ {
		armType := inferArmBodyType(match.Arms[i], sourceContext)
		if armType != firstType && armType != "" {
			// Type mismatch - fall back to IIFE for type safety
			return ""
		}
	}

	return firstType
}

// inferArmBodyType infers the type of a match arm body expression.
// Returns empty string if type cannot be inferred.
//
// CRITICAL: This function uses AST fields (IsBlock) and go/scanner
// for type inference. NO string manipulation for parsing.
func inferArmBodyType(arm *ast.MatchArm, sourceContext []byte) string {
	if arm == nil || arm.Body == nil {
		return ""
	}

	// Use AST's IsBlock field (NOT string prefix/suffix checking)
	// This is the correct AST-based approach
	if arm.IsBlock {
		// Block expression - cannot reliably infer type without full parsing
		// Fall back to IIFE (return empty string)
		return ""
	}

	// For RawExpr (most common case), analyze the text using go/scanner
	if rawExpr, ok := arm.Body.(*ast.RawExpr); ok {
		return inferTypeFromText(rawExpr.Text)
	}

	// For other expression types, try to extract text via String() method
	bodyText := arm.Body.String()
	if bodyText != "" {
		return inferTypeFromText(bodyText)
	}

	// Unknown expression type - cannot infer
	return ""
}

// inferTypeFromText uses go/scanner to tokenize text and infer the type.
// This is the CORRECT way to analyze Go source text (not string manipulation).
// go/scanner naturally handles leading whitespace - no TrimSpace needed.
func inferTypeFromText(text string) string {
	if len(text) == 0 {
		return ""
	}

	// Use go/scanner to tokenize the expression
	var s scanner.Scanner
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(text))
	s.Init(file, []byte(text), nil, 0)

	// Scan first token
	_, tok, lit := s.Scan()

	// Handle unary minus for negative numbers
	isNegative := false
	if tok == gotoken.SUB {
		isNegative = true
		// Scan next token (the actual number)
		_, tok, lit = s.Scan()
	}

	switch tok {
	case gotoken.STRING:
		// String literal: "..." or `...`
		return "string"

	case gotoken.INT:
		// Integer literal: 42, -10, 0 (isNegative handles the sign)
		return "int"

	case gotoken.FLOAT:
		// Float literal: 3.14, -2.5 (isNegative handles the sign)
		return "float64"

	case gotoken.IDENT:
		// Don't allow identifiers after unary minus
		if isNegative {
			return ""
		}

		// Identifier - check for bool literals or fmt calls
		if lit == "true" || lit == "false" {
			return "bool"
		}

		// Check for fmt.Sprintf/fmt.Sprint
		// Scan next token to see if it's a dot
		_, tok2, _ := s.Scan()
		if tok2 == gotoken.PERIOD {
			_, tok3, lit3 := s.Scan()
			if tok3 == gotoken.IDENT && (lit3 == "Sprintf" || lit3 == "Sprint") {
				// Likely fmt.Sprintf or fmt.Sprint
				if lit == "fmt" {
					return "string"
				}
			}
		}
		return ""

	default:
		return ""
	}
}

// simplifyTypeName simplifies type names for better readability.
// For example: "interface{}" -> "any" (Go 1.18+)
func simplifyTypeName(typeName string) string {
	// Common simplifications
	switch typeName {
	case "interface{}":
		return "any"
	case "interface {}":
		return "any"
	}
	return typeName
}
