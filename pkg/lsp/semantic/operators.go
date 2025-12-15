package semantic

import (
	"go/token"
	"go/types"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

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
