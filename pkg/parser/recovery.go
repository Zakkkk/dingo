// Package parser provides error recovery mechanisms for the Dingo parser
package parser

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// Note: ParseError is defined in pratt.go and used throughout the parser
// We extend it here with additional recovery functionality

// ErrorList is a collection of ParseErrors
type ErrorList []ParseError

// Add appends a new error to the list
func (l *ErrorList) Add(pos token.Pos, line, column int, msg string) {
	*l = append(*l, ParseError{
		Pos:     pos,
		Line:    line,
		Column:  column,
		Message: msg,
	})
}

// Error returns a formatted string containing all errors
func (l ErrorList) Error() string {
	if len(l) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d error(s):\n", len(l)))
	for i, err := range l {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("  %d. %s", i+1, err.Error()))
	}
	return sb.String()
}

// Err returns nil if the list is empty, otherwise returns the ErrorList itself
func (l ErrorList) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

// RecoveryPoint represents a token type that can be used as a synchronization point
type RecoveryPoint token.Token

// Common recovery points for synchronizing after parse errors
var (
	// Statement-level recovery points
	StatementRecoveryPoints = []token.Token{
		token.SEMICOLON,
		token.IF,
		token.FOR,
		token.RETURN,
		token.BREAK,
		token.CONTINUE,
		token.GO,
		token.DEFER,
		token.SWITCH,
		token.SELECT,
	}

	// Declaration-level recovery points
	DeclarationRecoveryPoints = []token.Token{
		token.FUNC,
		token.TYPE,
		token.CONST,
		token.VAR,
		token.IMPORT,
		token.PACKAGE,
	}

	// Dingo-specific recovery points
	DingoRecoveryPoints = []token.Token{
		// token.MATCH,  // Will be added when match keyword exists
		// token.ENUM,   // Will be added when enum keyword exists
		// token.GUARD,  // Will be added when guard keyword exists
	}
)

// RecoveryStrategy defines different strategies for error recovery
type RecoveryStrategy int

const (
	// RecoverToStatement skips tokens until a statement boundary
	RecoverToStatement RecoveryStrategy = iota

	// RecoverToDeclaration skips tokens until a declaration boundary
	RecoverToDeclaration

	// RecoverToBlock skips tokens until a block boundary (}, {, ;)
	RecoverToBlock

	// RecoverToExpression skips tokens until an expression boundary
	RecoverToExpression
)

// RecoveryContext holds state during error recovery
type RecoveryContext struct {
	Errors    ErrorList
	Strategy  RecoveryStrategy
	MaxErrors int // Maximum errors before giving up (0 = unlimited)
}

// NewRecoveryContext creates a new recovery context with default settings
func NewRecoveryContext() *RecoveryContext {
	return &RecoveryContext{
		Errors:    make(ErrorList, 0),
		Strategy:  RecoverToStatement,
		MaxErrors: 100, // Reasonable default to prevent infinite loops
	}
}

// ShouldContinue returns false if max errors reached
func (rc *RecoveryContext) ShouldContinue() bool {
	if rc.MaxErrors == 0 {
		return true
	}
	return len(rc.Errors) < rc.MaxErrors
}

// RecoveryHelper provides utility methods for error recovery during parsing
type RecoveryHelper struct {
	tokens  []tokenizer.Token // Token stream
	pos     int               // Current position in token stream
	fset    *token.FileSet
	context *RecoveryContext
}

// NewRecoveryHelper creates a new recovery helper
func NewRecoveryHelper(tokens []tokenizer.Token, fset *token.FileSet, ctx *RecoveryContext) *RecoveryHelper {
	if ctx == nil {
		ctx = NewRecoveryContext()
	}
	return &RecoveryHelper{
		tokens:  tokens,
		pos:     0,
		fset:    fset,
		context: ctx,
	}
}

// Synchronize skips tokens until a recovery point is found
// Returns the number of tokens skipped
func (rh *RecoveryHelper) Synchronize(recoveryPoints []token.Token) int {
	skipped := 0
	startPos := rh.pos

	// Advance at least once to avoid getting stuck
	if rh.pos < len(rh.tokens) {
		rh.pos++
		skipped++
	}

	// Skip until we find a recovery point or reach end
	for rh.pos < len(rh.tokens) {
		current := rh.tokens[rh.pos]

		// Check if current token is a recovery point
		// Compare token.Kind since we're comparing tokenizer.Token with go/token.Token
		for _, rp := range recoveryPoints {
			// Map tokenizer token to go/token and compare
			if mapTokenizerToGoToken(current.Kind) == rp {
				return skipped
			}
		}

		rh.pos++
		skipped++
	}

	// Reset if we went too far
	if skipped > 100 {
		rh.pos = startPos + 1
		return 1
	}

	return skipped
}

// SynchronizeToStatement skips to the next statement boundary
func (rh *RecoveryHelper) SynchronizeToStatement() int {
	return rh.Synchronize(StatementRecoveryPoints)
}

// SynchronizeToDeclaration skips to the next declaration boundary
func (rh *RecoveryHelper) SynchronizeToDeclaration() int {
	allRecoveryPoints := append(DeclarationRecoveryPoints, DingoRecoveryPoints...)
	return rh.Synchronize(allRecoveryPoints)
}

// SynchronizeToBlock skips to the next block boundary
func (rh *RecoveryHelper) SynchronizeToBlock() int {
	blockPoints := []token.Token{
		token.LBRACE,
		token.RBRACE,
		token.SEMICOLON,
	}
	return rh.Synchronize(blockPoints)
}

// AddError adds a parse error to the context
func (rh *RecoveryHelper) AddError(pos token.Pos, msg string) {
	position := rh.fset.Position(pos)
	rh.context.Errors.Add(pos, position.Line, position.Column, msg)
}

// ShouldContinue checks if parsing should continue
func (rh *RecoveryHelper) ShouldContinue() bool {
	return rh.context.ShouldContinue()
}

// Errors returns all collected errors
func (rh *RecoveryHelper) Errors() ErrorList {
	return rh.context.Errors
}

// ParseWithRecovery wraps a parsing function with error recovery
// The parseFunc should return (result, error). On error, recovery will be attempted.
type ParseFunc func() (interface{}, error)

// TryParse attempts to execute a parse function with error recovery
func (rh *RecoveryHelper) TryParse(parseFunc ParseFunc, recoverTo RecoveryStrategy) (result interface{}, recovered bool) {
	result, err := parseFunc()

	if err == nil {
		return result, false
	}

	// Add error to context
	if parseErr, ok := err.(ParseError); ok {
		rh.context.Errors = append(rh.context.Errors, parseErr)
	} else {
		// Generic error - create ParseError with current position
		pos := token.NoPos
		if rh.pos < len(rh.tokens) && rh.fset != nil {
			pos = token.Pos(rh.pos)
		}
		rh.AddError(pos, err.Error())
	}

	// Don't recover if we've hit max errors
	if !rh.ShouldContinue() {
		return nil, false
	}

	// Synchronize based on strategy
	switch recoverTo {
	case RecoverToStatement:
		rh.SynchronizeToStatement()
	case RecoverToDeclaration:
		rh.SynchronizeToDeclaration()
	case RecoverToBlock:
		rh.SynchronizeToBlock()
	case RecoverToExpression:
		// For expressions, just skip one token
		if rh.pos < len(rh.tokens) {
			rh.pos++
		}
	}

	return nil, true
}

// IsAtRecoveryPoint checks if current token is a recovery point
func (rh *RecoveryHelper) IsAtRecoveryPoint(points []token.Token) bool {
	if rh.pos >= len(rh.tokens) {
		return false
	}

	current := rh.tokens[rh.pos]
	for _, point := range points {
		if mapTokenizerToGoToken(current.Kind) == point {
			return true
		}
	}
	return false
}

// CurrentPos returns the current position in the token stream
func (rh *RecoveryHelper) CurrentPos() int {
	return rh.pos
}

// SetPos sets the current position in the token stream
func (rh *RecoveryHelper) SetPos(pos int) {
	if pos >= 0 && pos < len(rh.tokens) {
		rh.pos = pos
	}
}

// mapTokenizerToGoToken maps tokenizer.TokenKind to go/token.Token
func mapTokenizerToGoToken(tk tokenizer.TokenKind) token.Token {
	switch tk {
	case tokenizer.SEMICOLON:
		return token.SEMICOLON
	case tokenizer.IF:
		return token.IF
	case tokenizer.CONST:
		return token.CONST
	case tokenizer.VAR:
		return token.VAR
	case tokenizer.LBRACE:
		return token.LBRACE
	case tokenizer.RBRACE:
		return token.RBRACE
	// TODO: Add more token mappings as tokenizer is expanded
	// For now, many Go keywords (FOR, RETURN, BREAK, etc.) are not yet in tokenizer
	default:
		return token.ILLEGAL
	}
}
