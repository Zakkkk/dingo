// Package parser provides error types and recovery mechanisms for the Dingo parser
package parser

import (
	"fmt"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// Error codes for documentation and tooling
const (
	ErrUnexpectedToken   = "E001"
	ErrMissingExpression = "E002"
	ErrUnclosedDelimiter = "E003"
	ErrAmbiguousQuestion = "E004"
	ErrInvalidPattern    = "E005"
	ErrMissingArrow      = "E006"
	ErrMissingColon      = "E007"
	ErrEmptyMatchArms    = "E008"
	ErrInvalidLambda     = "E009"
	ErrMissingOperand    = "E010"
	// ErrMatchArmStatement is reported when a statement (like assignment) is used
	// in a braceless match arm body, which requires an expression
	ErrMatchArmStatement = "E011"
)

// SpanError represents a parse error with full span information
// This extends the basic ParseError with end position, hints, and error codes
type SpanError struct {
	Pos     token.Pos // Start position
	EndPos  token.Pos // End position (for highlighting)
	Line    int       // Line number (1-based)
	Column  int       // Column number (1-based)
	EndCol  int       // End column (for span highlighting)
	Message string    // Human-readable message
	Hint    string    // Suggestion for fixing (optional)
	Code    string    // Error code for docs (e.g., E001)
	Context string    // Context info (e.g., "in match expression")
}

// Error implements the error interface
func (e SpanError) Error() string {
	msg := e.Message
	if e.Hint != "" {
		msg += "\n  Hint: " + e.Hint
	}
	return msg
}

// ErrorWithSpan creates a SpanError with start/end positions
func ErrorWithSpan(pos, endPos token.Pos, line, col, endCol int, code, msg, hint, context string) SpanError {
	return SpanError{
		Pos:     pos,
		EndPos:  endPos,
		Line:    line,
		Column:  col,
		EndCol:  endCol,
		Message: msg,
		Hint:    hint,
		Code:    code,
		Context: context,
	}
}

// errorBuilder provides fluent API for constructing errors
type errorBuilder struct {
	pos     token.Pos
	endPos  token.Pos
	line    int
	col     int
	endCol  int
	code    string
	context string
}

// NewErrorBuilder starts building an error at the given position
func NewErrorBuilder(pos token.Pos, line, col int) *errorBuilder {
	return &errorBuilder{
		pos:  pos,
		line: line,
		col:  col,
	}
}

// EndPos sets the end position for the error span
func (b *errorBuilder) EndPos(endPos token.Pos, endCol int) *errorBuilder {
	b.endPos = endPos
	b.endCol = endCol
	return b
}

// Code sets the error code
func (b *errorBuilder) Code(code string) *errorBuilder {
	b.code = code
	return b
}

// Context sets the context information
func (b *errorBuilder) Context(ctx string) *errorBuilder {
	b.context = ctx
	return b
}

// Build constructs the SpanError with the given message and hint
func (b *errorBuilder) Build(msg, hint string) SpanError {
	return SpanError{
		Pos:     b.pos,
		EndPos:  b.endPos,
		Line:    b.line,
		Column:  b.col,
		EndCol:  b.endCol,
		Message: msg,
		Hint:    hint,
		Code:    b.code,
		Context: b.context,
	}
}

// Common error constructors

// UnexpectedTokenError creates an error for unexpected tokens
func UnexpectedTokenError(got, expected string, pos token.Pos, line, col int) SpanError {
	return SpanError{
		Pos:     pos,
		Line:    line,
		Column:  col,
		Code:    ErrUnexpectedToken,
		Message: fmt.Sprintf("unexpected '%s', expected %s", got, expected),
	}
}

// AmbiguousQuestionError creates an error for ambiguous ? operator usage
func AmbiguousQuestionError(pos token.Pos, line, col int) SpanError {
	return SpanError{
		Pos:     pos,
		Line:    line,
		Column:  col,
		Code:    ErrAmbiguousQuestion,
		Message: "ambiguous '?' operator",
		Hint:    "use 'expr?' for error propagation, or 'cond ? a : b' for ternary",
	}
}

// MissingArrowError creates an error for missing => in lambdas or match arms
func MissingArrowError(context string, pos token.Pos, line, col int) SpanError {
	return SpanError{
		Pos:     pos,
		Line:    line,
		Column:  col,
		Code:    ErrMissingArrow,
		Message: fmt.Sprintf("missing '=>' %s", context),
		Context: context,
	}
}

// MissingColonError creates an error for missing colon in ternary expressions
func MissingColonError(pos token.Pos, line, col int) SpanError {
	return SpanError{
		Pos:     pos,
		Line:    line,
		Column:  col,
		Code:    ErrMissingColon,
		Message: "ternary operator missing ':'",
		Hint:    "ternary syntax: condition ? trueValue : falseValue",
	}
}

// MissingOperandError creates an error for missing operands
func MissingOperandError(operator string, pos token.Pos, line, col int) SpanError {
	return SpanError{
		Pos:     pos,
		Line:    line,
		Column:  col,
		Code:    ErrMissingOperand,
		Message: fmt.Sprintf("missing operand for '%s' operator", operator),
	}
}

// UnclosedDelimiterError creates an error for unclosed delimiters
func UnclosedDelimiterError(delimiter string, pos token.Pos, line, col int) SpanError {
	return SpanError{
		Pos:     pos,
		Line:    line,
		Column:  col,
		Code:    ErrUnclosedDelimiter,
		Message: fmt.Sprintf("unclosed '%s'", delimiter),
		Hint:    fmt.Sprintf("add matching closing delimiter for '%s'", delimiter),
	}
}

// SyncSet defines tokens that can be used as synchronization points for error recovery
type SyncSet map[tokenizer.TokenKind]bool

// Pre-defined sync sets for different parsing contexts

// StatementSync contains synchronization points for statement parsing
var StatementSync = SyncSet{
	tokenizer.SEMICOLON: true,
	tokenizer.RBRACE:    true,
	tokenizer.EOF:       true,
}

// ExpressionSync contains synchronization points for expression parsing
var ExpressionSync = SyncSet{
	tokenizer.SEMICOLON: true,
	tokenizer.COMMA:     true,
	tokenizer.RPAREN:    true,
	tokenizer.RBRACE:    true,
	tokenizer.RBRACKET:  true,
	tokenizer.COLON:     true,
	tokenizer.EOF:       true,
}

// MatchArmSync contains synchronization points for match arm parsing
var MatchArmSync = SyncSet{
	tokenizer.ARROW:  true,
	tokenizer.COMMA:  true,
	tokenizer.RBRACE: true,
	tokenizer.EOF:    true,
}

// LambdaSync contains synchronization points for lambda parsing
var LambdaSync = SyncSet{
	tokenizer.ARROW:  true, // => for TS-style
	tokenizer.PIPE:   true, // | for Rust-style
	tokenizer.RPAREN: true,
	tokenizer.EOF:    true,
}

// Contains checks if a token kind is in the sync set
func (s SyncSet) Contains(kind tokenizer.TokenKind) bool {
	return s[kind]
}

// errorHints provides context-aware hints for common error situations
var errorHints = map[tokenizer.TokenKind]string{
	tokenizer.QUESTION: "the '?' operator requires an expression before it (e.g., 'result?')",
	tokenizer.ARROW:    "'=>' is used in lambdas and match arms, not as a standalone operator",
	tokenizer.COLON:    "':' is used for type annotations and ternary operators",
	tokenizer.PIPE:     "'|' starts a Rust-style lambda parameter list",
}

// GetHintForToken returns a helpful hint for the given token kind
func GetHintForToken(kind tokenizer.TokenKind) string {
	if hint, ok := errorHints[kind]; ok {
		return hint
	}
	return ""
}
