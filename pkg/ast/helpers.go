package ast

// Shared helper functions used across multiple codegen files.
// This file centralizes common utilities to avoid code duplication.

// isIdentChar checks if byte is valid in Go identifier (alphanumeric or underscore).
func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// isWhitespace checks if a byte is whitespace (space, tab, carriage return, or newline).
func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

// skipWhitespace advances position past whitespace characters.
// Returns the new position after skipping all consecutive whitespace.
func skipWhitespace(src []byte, pos int) int {
	for pos < len(src) {
		ch := src[pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			pos++
		} else {
			break
		}
	}
	return pos
}
