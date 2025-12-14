package main

import (
	"net/url"
	"strings"
	"unicode/utf16"
)

// PathToURI converts an absolute file path to a proper file:// URI
// On Unix: /path/to/file -> file:///path/to/file (3 slashes)
// Handles path escaping for special characters
func PathToURI(path string) string {
	// URL-encode the path, but preserve slashes
	var builder strings.Builder
	builder.WriteString("file://")

	for _, c := range path {
		switch {
		case c == '/':
			builder.WriteRune(c)
		case isUnreserved(c):
			builder.WriteRune(c)
		default:
			// Percent-encode
			encoded := url.PathEscape(string(c))
			builder.WriteString(encoded)
		}
	}

	return builder.String()
}

// isUnreserved returns true if c is an "unreserved" character per RFC 3986
func isUnreserved(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

// ByteOffsetToUTF16 converts a byte offset in a UTF-8 string to UTF-16 code units
// This is needed because LSP positions use UTF-16 code units, not bytes
func ByteOffsetToUTF16(text string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(text) {
		byteOffset = len(text)
	}

	// Count UTF-16 code units up to byteOffset
	utf16Units := 0
	bytePos := 0

	for _, r := range text {
		if bytePos >= byteOffset {
			break
		}
		// Count how many UTF-16 code units this rune needs
		if r <= 0xFFFF {
			utf16Units++ // BMP character = 1 code unit
		} else {
			utf16Units += 2 // Supplementary character = surrogate pair
		}
		bytePos += len(string(r)) // Advance by UTF-8 byte length
	}

	return utf16Units
}

// UTF16ToByteOffset converts UTF-16 code units to byte offset
// Inverse of ByteOffsetToUTF16
func UTF16ToByteOffset(text string, utf16Offset int) int {
	if utf16Offset <= 0 {
		return 0
	}

	utf16Units := 0
	bytePos := 0

	for _, r := range text {
		if utf16Units >= utf16Offset {
			break
		}
		bytePos += len(string(r))
		if r <= 0xFFFF {
			utf16Units++
		} else {
			utf16Units += 2
		}
	}

	return bytePos
}

// RuneToUTF16Length returns how many UTF-16 code units a rune requires
func RuneToUTF16Length(r rune) int {
	encoded := utf16.Encode([]rune{r})
	return len(encoded)
}

// IsWordBoundary checks if the character at position pos is at a word boundary
// (i.e., preceded and followed by non-identifier characters)
func IsWordBoundary(text string, pos int, tokenLen int) bool {
	// Check character before
	if pos > 0 {
		prevRune := rune(0)
		bytePos := 0
		for _, r := range text {
			if bytePos == pos {
				break
			}
			prevRune = r
			bytePos += len(string(r))
		}
		if isIdentChar(prevRune) {
			return false
		}
	}

	// Check character after
	endPos := pos + tokenLen
	if endPos < len(text) {
		bytePos := 0
		for _, r := range text {
			if bytePos == endPos {
				// r is the character after the token
				if isIdentChar(r) {
					return false
				}
				break
			}
			bytePos += len(string(r))
		}
	}

	return true
}

// isIdentChar returns true if r is a valid identifier character
func isIdentChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_'
}
