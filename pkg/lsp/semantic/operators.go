package semantic

import (
	"go/token"
	"go/types"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// LambdaParamInfo holds information about a lambda parameter
type LambdaParamInfo struct {
	Line   int    // 1-indexed
	Col    int    // 1-indexed
	EndCol int    // 1-indexed (exclusive)
	Name   string // Parameter name
}

// OperatorInfo holds information about a Dingo operator
type OperatorInfo struct {
	Line   int // 1-indexed
	Col    int // 1-indexed
	EndCol int // 1-indexed (exclusive)
	Kind   ContextKind
	// The expression this operator applies to (for type lookup)
	// This will be populated by the builder when correlating with AST
	ExprType types.Type
}

// DetectOperators finds all Dingo operators in source using tokenizer
// FOLLOWS CLAUDE.md RULES: Uses token.FileSet for position tracking (NOT byte scanning)
func DetectOperators(dingoSource []byte, fset *token.FileSet, filename string) []OperatorInfo {
	// Use Dingo tokenizer which properly tracks positions via token.FileSet
	tok := tokenizer.NewWithFileSet(dingoSource, fset, filename)
	tokens, err := tok.Tokenize()
	if err != nil {
		// Tokenization error - return empty (graceful degradation)
		return nil
	}

	var operators []OperatorInfo

	for _, t := range tokens {
		var kind ContextKind

		switch t.Kind {
		case tokenizer.QUESTION:
			// Single ? (error propagation)
			// Only if not followed by ? or . (handled by tokenizer as separate tokens)
			kind = ContextErrorProp

		case tokenizer.QUESTION_QUESTION:
			// ?? (null coalescing)
			kind = ContextNullCoal

		case tokenizer.QUESTION_DOT:
			// ?. (safe navigation)
			kind = ContextSafeNav

		default:
			// Not a Dingo operator we care about
			continue
		}

		// SAFE column arithmetic: t.Column comes from token.FileSet (not calculated).
		// We add literal width which is immutable and doesn't shift with go/printer.
		// This is acceptable per CLAUDE.md because we're using token-tracked positions
		// as the base, not calculating positions from raw bytes.
		endCol := t.Column + len(t.Lit)
		if t.Lit == "" {
			// For operators, literal may be empty - use kind to determine width
			switch kind {
			case ContextErrorProp:
				endCol = t.Column + 1 // "?" is 1 char
			case ContextNullCoal:
				endCol = t.Column + 2 // "??" is 2 chars
			case ContextSafeNav:
				endCol = t.Column + 2 // "?." is 2 chars
			}
		}

		operators = append(operators, OperatorInfo{
			Line:   t.Line,
			Col:    t.Column,
			EndCol: endCol,
			Kind:   kind,
		})
	}

	return operators
}

// DetectLambdaParams finds all lambda parameters and their usages in Dingo source
// Detects patterns: |param| (Rust-style) and param => (TypeScript-style)
// Also finds usages of the parameter in the lambda body (same line)
// FOLLOWS CLAUDE.md RULES: Uses token.FileSet for position tracking
func DetectLambdaParams(dingoSource []byte, fset *token.FileSet, filename string) []LambdaParamInfo {
	tok := tokenizer.NewWithFileSet(dingoSource, fset, filename)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil
	}

	var params []LambdaParamInfo

	// Track lambda params by line so we can find usages
	type lambdaScope struct {
		name     string
		line     int
		startIdx int // token index where lambda body starts
	}
	var activeScopes []lambdaScope

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		// Pattern 1: |param| (Rust-style lambda)
		// Look for: PIPE IDENT PIPE
		if t.Kind == tokenizer.PIPE && i+2 < len(tokens) {
			next := tokens[i+1]
			nextNext := tokens[i+2]
			if next.Kind == tokenizer.IDENT && nextNext.Kind == tokenizer.PIPE {
				params = append(params, LambdaParamInfo{
					Line:   next.Line,
					Col:    next.Column,
					EndCol: next.Column + len(next.Lit),
					Name:   next.Lit,
				})
				// Track this scope for finding usages
				activeScopes = append(activeScopes, lambdaScope{
					name:     next.Lit,
					line:     next.Line,
					startIdx: i + 3, // After closing |
				})
				i += 2 // Skip past |param|
				continue
			}
		}

		// Pattern 2: param => (TypeScript-style, no parens)
		// Look for: IDENT ARROW (where IDENT is not a keyword)
		if t.Kind == tokenizer.IDENT && i+1 < len(tokens) {
			next := tokens[i+1]
			if next.Kind == tokenizer.ARROW {
				params = append(params, LambdaParamInfo{
					Line:   t.Line,
					Col:    t.Column,
					EndCol: t.Column + len(t.Lit),
					Name:   t.Lit,
				})
				// Track this scope for finding usages
				activeScopes = append(activeScopes, lambdaScope{
					name:     t.Lit,
					line:     t.Line,
					startIdx: i + 2, // After =>
				})
			}
		}

		// Pattern 3: (param) => (TypeScript-style with parens)
		// Look for: LPAREN IDENT RPAREN ARROW
		if t.Kind == tokenizer.LPAREN && i+3 < len(tokens) {
			ident := tokens[i+1]
			rparen := tokens[i+2]
			arrow := tokens[i+3]
			if ident.Kind == tokenizer.IDENT && rparen.Kind == tokenizer.RPAREN && arrow.Kind == tokenizer.ARROW {
				params = append(params, LambdaParamInfo{
					Line:   ident.Line,
					Col:    ident.Column,
					EndCol: ident.Column + len(ident.Lit),
					Name:   ident.Lit,
				})
				// Track this scope for finding usages
				activeScopes = append(activeScopes, lambdaScope{
					name:     ident.Lit,
					line:     ident.Line,
					startIdx: i + 4, // After =>
				})
				i += 3 // Skip past (param) =>
				continue
			}
		}

		// Check if this identifier is a usage of an active lambda param
		if t.Kind == tokenizer.IDENT {
			for _, scope := range activeScopes {
				// Lambda params are scoped to the same line in error prop expressions
				if t.Line == scope.line && t.Lit == scope.name && i >= scope.startIdx {
					params = append(params, LambdaParamInfo{
						Line:   t.Line,
						Col:    t.Column,
						EndCol: t.Column + len(t.Lit),
						Name:   t.Lit,
					})
					break
				}
			}
		}

		// Clear scopes when we move to a new line
		if len(activeScopes) > 0 && t.Line > activeScopes[len(activeScopes)-1].line {
			activeScopes = nil
		}
	}

	return params
}
