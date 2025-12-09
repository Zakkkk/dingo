package builtin

import (
	"fmt"
	"strings"
)

// NOTE ON ARCHITECTURAL COMPLIANCE:
// The string manipulation in this file (strings.HasPrefix, strings.Contains, etc.)
// is INTENTIONALLY allowed and does NOT violate the "no string manipulation" rule.
//
// The architectural rule prohibits string/regex manipulation of RAW SOURCE CODE
// (which must go through tokenizer → parser → AST → codegen).
//
// These functions operate on TYPE STRINGS that have ALREADY been extracted from
// the AST by the parser. This is post-AST string manipulation for generating
// Go-idiomatic type names (e.g., "Result[*User, error]" → "ResultPtrUserError").
//
// The distinction:
//   ❌ BAD: strings.Contains(sourceCode, "Result<") - parsing raw source
//   ✅ OK:  strings.HasPrefix(extractedType, "*")   - processing AST-derived data

// SanitizeTypeName converts type name parts to camelCase format
// This is used for Result[T,E] → ResultIntError and Option[T] → OptionString naming
// Examples:
//   ("int", "error") → "IntError"
//   ("string") → "String"
//   ("any", "error") → "InterfaceError"
//   ("*User", "error") → "PtrUserError"
//   ("[]int", "error") → "SliceIntError"
func SanitizeTypeName(parts ...string) string {
	var result strings.Builder
	for _, part := range parts {
		result.WriteString(sanitizeTypeComponent(part))
	}
	return result.String()
}

// Package-level maps for performance (avoid recreating on every call)
var (
	// commonAcronyms maps lowercase acronyms to their canonical Go form.
	// Only include genuine acronyms (HTTP, URL, etc.), not regular words.
	// Regular words are handled by the default capitalization logic.
	commonAcronyms = map[string]string{
		"http":  "HTTP",
		"https": "HTTPS",
		"url":   "URL",
		"uri":   "URI",
		"json":  "JSON",
		"xml":   "XML",
		"api":   "API",
		"id":    "ID",
		"uuid":  "UUID",
		"sql":   "SQL",
		"html":  "HTML",
		"css":   "CSS",
		"tcp":   "TCP",
		"udp":   "UDP",
		"ip":    "IP",
	}

	// builtinTypes contains Go built-in types that should only capitalize the first letter
	builtinTypes = map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"string": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true,
	}
)

// sanitizeTypeComponent sanitizes individual type components for camelCase format
// Handles special prefixes like *, [], map[], chan, arrays, and package-qualified types
// Also converts underscore-separated names to camelCase (Option_int → OptionInt)
func sanitizeTypeComponent(s string) string {
	if s == "" {
		return s
	}

	// Handle pointer types: *T → PtrT
	if strings.HasPrefix(s, "*") {
		return "Ptr" + sanitizeTypeComponent(strings.TrimPrefix(s, "*"))
	}

	// Handle slice types: []T → SliceT
	if strings.HasPrefix(s, "[]") {
		return "Slice" + sanitizeTypeComponent(strings.TrimPrefix(s, "[]"))
	}

	// Handle array types: [N]T → ArrayNT
	if strings.HasPrefix(s, "[") {
		closeBracket := strings.Index(s, "]")
		if closeBracket > 0 {
			size := s[1:closeBracket]
			elemType := s[closeBracket+1:]
			return "Array" + size + sanitizeTypeComponent(elemType)
		}
	}

	// Handle map types: map[K]V → Map (simplified)
	if strings.HasPrefix(s, "map[") {
		// Complex parsing - for now just use "Map"
		return "Map"
	}

	// Handle chan types: chan T → ChanT
	if strings.HasPrefix(s, "chan ") {
		return "Chan" + sanitizeTypeComponent(strings.TrimPrefix(s, "chan "))
	}

	// Handle interface{} → Interface
	if s == "interface{}" {
		return "Interface"
	}

	// Handle any → Interface (Go 1.18+)
	if s == "any" {
		return "Interface"
	}

	// Handle package-qualified types: pkg.Type → PkgType
	// Example: "github.com/user/pkg.Type" → "GithubComUserPkgType"
	if strings.Contains(s, ".") {
		// Split by dots and capitalize each part
		parts := strings.FieldsFunc(s, func(r rune) bool {
			return r == '.' || r == '/'
		})
		var result strings.Builder
		for _, part := range parts {
			result.WriteString(sanitizeTypeComponent(part))
		}
		return result.String()
	}

	// Handle underscore-separated type names: Option_int → OptionInt
	// This converts legacy underscore notation to camelCase
	if strings.Contains(s, "_") {
		parts := strings.Split(s, "_")
		var result strings.Builder
		for _, part := range parts {
			result.WriteString(sanitizeTypeComponent(part))
		}
		return result.String()
	}

	// Check if it's a common acronym - use canonical form (e.g., HTTP, URL)
	lower := strings.ToLower(s)
	if acronym, ok := commonAcronyms[lower]; ok {
		return acronym
	}

	// Check if it's a built-in type - capitalize first letter only
	if builtinTypes[s] {
		return capitalize(s)
	}

	// User-defined type - ensure first letter is capitalized
	return capitalize(s)
}

// NormalizeTypeName converts underscore-separated type names to camelCase
// while preserving basic Go types unchanged.
// This is used for type references in struct fields and method signatures.
// Examples:
//   "Option_int" → "OptionInt"
//   "Result_int_error" → "ResultIntError"
//   "int" → "int" (preserved)
//   "error" → "error" (preserved)
//   "*Option_int" → "*OptionInt"
//   "[]Option_int" → "[]OptionInt"
func NormalizeTypeName(typeName string) string {
	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		return "*" + NormalizeTypeName(strings.TrimPrefix(typeName, "*"))
	}

	// Handle slice types
	if strings.HasPrefix(typeName, "[]") {
		return "[]" + NormalizeTypeName(strings.TrimPrefix(typeName, "[]"))
	}

	// If it contains underscores, it's a generated type name that needs normalization
	if strings.Contains(typeName, "_") {
		parts := strings.Split(typeName, "_")
		var result strings.Builder
		for _, part := range parts {
			result.WriteString(capitalize(part))
		}
		return result.String()
	}

	// Basic types and user types without underscores are returned as-is
	return typeName
}

// GenerateTempVarName generates temporary variable names with optional numbering
// First call returns base name (e.g., "ok"), subsequent calls add numbers ("ok1", "ok2")
// Examples:
//   ("ok", 0) → "ok"
//   ("ok", 1) → "ok1"
//   ("err", 0) → "err"
//   ("err", 1) → "err1"
func GenerateTempVarName(base string, index int) string {
	if index < 0 {
		index = 0 // Defensive: treat negative as zero
	}
	if index == 0 {
		return base // First variable: no number suffix
	}
	return fmt.Sprintf("%s%d", base, index) // ok1, ok2, ok3, ...
}
