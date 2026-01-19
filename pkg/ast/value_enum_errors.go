package ast

import (
	"fmt"
	"go/token"
)

// ValueEnumError represents a validation error for value enums.
// It includes position information for accurate error reporting.
type ValueEnumError struct {
	Pos     token.Pos // Position in source where error occurred
	Message string    // Human-readable error message
	Kind    ValueEnumErrorKind
}

// ValueEnumErrorKind categorizes the type of validation error
type ValueEnumErrorKind int

const (
	// ErrorEmptyEnum: value enum must have at least one variant
	ErrorEmptyEnum ValueEnumErrorKind = iota

	// ErrorInvalidBaseType: base type is not string/int/byte/rune/etc
	ErrorInvalidBaseType

	// ErrorStringRequiresValue: string enum variant requires explicit value
	ErrorStringRequiresValue

	// ErrorDuplicateVariant: variant name already declared
	ErrorDuplicateVariant

	// ErrorTypeMismatch: value type doesn't match base type
	ErrorTypeMismatch

	// ErrorInvalidPrefixArg: @prefix attribute has invalid argument
	ErrorInvalidPrefixArg

	// ErrorMissingPrefixArg: @prefix attribute missing required argument
	ErrorMissingPrefixArg

	// ErrorTooManyPrefixArgs: @prefix attribute has too many arguments
	ErrorTooManyPrefixArgs

	// WarningMixedValues: mixing explicit and auto-increment values
	WarningMixedValues
)

func (e *ValueEnumError) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("pos %d: %s", e.Pos, e.Message)
	}
	return e.Message
}

// ValueEnumValidationResult holds all validation errors and warnings
type ValueEnumValidationResult struct {
	Errors   []*ValueEnumError
	Warnings []*ValueEnumError
}

func NewValidationResult() *ValueEnumValidationResult {
	return &ValueEnumValidationResult{}
}

func (r *ValueEnumValidationResult) AddError(pos token.Pos, kind ValueEnumErrorKind, msg string) {
	r.Errors = append(r.Errors, &ValueEnumError{
		Pos:     pos,
		Message: msg,
		Kind:    kind,
	})
}

func (r *ValueEnumValidationResult) AddWarning(pos token.Pos, kind ValueEnumErrorKind, msg string) {
	r.Warnings = append(r.Warnings, &ValueEnumError{
		Pos:     pos,
		Message: msg,
		Kind:    kind,
	})
}

func (r *ValueEnumValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

func (r *ValueEnumValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// CombinedErrors returns all errors as a single formatted string
func (r *ValueEnumValidationResult) CombinedErrors() string {
	if !r.HasErrors() {
		return ""
	}
	msgs := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		msgs[i] = err.Error()
	}
	if len(msgs) == 1 {
		return msgs[0]
	}
	return fmt.Sprintf("multiple errors: %v", msgs)
}

// ValidateValueEnumDecl performs comprehensive validation on a ValueEnumDecl.
// It checks for:
//   - Empty enums (no variants)
//   - Invalid base types
//   - String enums without explicit values
//   - Duplicate variant names
//   - Type mismatches between values and base type
//   - Mixed explicit/auto-increment values (warning)
func ValidateValueEnumDecl(decl *ValueEnumDecl, fset *token.FileSet) *ValueEnumValidationResult {
	result := NewValidationResult()

	// 1. Check for empty enum
	if len(decl.Variants) == 0 {
		result.AddError(decl.LBrace, ErrorEmptyEnum,
			fmt.Sprintf("value enum %q must have at least one variant", decl.Name.Name))
	}

	// 2. Validate base type
	if !isValidValueEnumBaseType(decl.BaseType.Text) {
		result.AddError(decl.BaseType.StartPos, ErrorInvalidBaseType,
			fmt.Sprintf("invalid value enum base type: %q (use string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, byte, or rune)", decl.BaseType.Text))
	}

	// 3. Check for duplicates and validate variants
	seenNames := make(map[string]token.Pos)
	requiresExplicitValue := decl.BaseType.Text == "string"
	hasExplicitValue := false
	hasAutoValue := false

	for _, variant := range decl.Variants {
		// Check for duplicate variant name
		if prevPos, exists := seenNames[variant.Name.Name]; exists {
			var posInfo string
			if fset != nil {
				posInfo = fset.Position(prevPos).String()
			} else {
				posInfo = fmt.Sprintf("position %d", prevPos)
			}
			result.AddError(variant.Name.NamePos, ErrorDuplicateVariant,
				fmt.Sprintf("duplicate variant %q (first declared at %s)", variant.Name.Name, posInfo))
		}
		seenNames[variant.Name.Name] = variant.Name.NamePos

		// Check for explicit value requirement (string enums)
		if variant.Value != nil {
			hasExplicitValue = true
			// Validate value type matches base type
			if err := validateVariantValue(variant, decl.BaseType.Text); err != nil {
				result.AddError(variant.Value.Pos(), ErrorTypeMismatch, err.Error())
			}
		} else {
			hasAutoValue = true
			if requiresExplicitValue {
				result.AddError(variant.Name.NamePos, ErrorStringRequiresValue,
					fmt.Sprintf("string enum variant %q requires explicit value (e.g., %s = \"value\")",
						variant.Name.Name, variant.Name.Name))
			}
		}
	}

	// 4. Warn about mixing explicit and auto values (for non-string enums)
	if hasExplicitValue && hasAutoValue && !requiresExplicitValue {
		result.AddWarning(decl.Enum, WarningMixedValues,
			fmt.Sprintf("enum %q mixes explicit and auto-increment values; this may lead to unexpected iota behavior",
				decl.Name.Name))
	}

	return result
}

// validateVariantValue checks if a variant's value matches the expected base type
func validateVariantValue(variant *ValueEnumVariant, baseType string) error {
	if variant.Value == nil {
		return nil
	}

	rawExpr, ok := variant.Value.(*RawExpr)
	if !ok {
		return nil // Can't validate non-RawExpr
	}

	text := rawExpr.Text
	if len(text) == 0 {
		return fmt.Errorf("empty value for variant %q", variant.Name.Name)
	}

	switch baseType {
	case "string":
		// Must be a string literal
		if !isStringLiteral(text) {
			return fmt.Errorf("string enum variant %q requires string literal value, got %q", variant.Name.Name, text)
		}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune":
		// Must be numeric literal, iota, or constant reference
		if !isValidIntegerValue(text) {
			return fmt.Errorf("integer enum variant %q requires integer literal, iota, or constant, got %q", variant.Name.Name, text)
		}
	}

	return nil
}

// isStringLiteral checks if text is a valid string literal
func isStringLiteral(text string) bool {
	if len(text) < 2 {
		return false
	}
	// Regular string: "..."
	if text[0] == '"' && text[len(text)-1] == '"' {
		return true
	}
	// Raw string: `...`
	if text[0] == '`' && text[len(text)-1] == '`' {
		return true
	}
	return false
}

// isValidIntegerValue checks if text is a valid integer value
func isValidIntegerValue(text string) bool {
	if len(text) == 0 {
		return false
	}

	// Numeric literal (possibly negative)
	if isDigitChar(text[0]) {
		return true
	}
	if text[0] == '-' && len(text) > 1 && isDigitChar(text[1]) {
		return true
	}

	// Hex/octal/binary prefixes
	if len(text) >= 2 && text[0] == '0' {
		prefix := text[1]
		if prefix == 'x' || prefix == 'X' || prefix == 'o' || prefix == 'O' || prefix == 'b' || prefix == 'B' {
			return true
		}
	}

	// Identifier (iota, constants like math.MaxInt)
	if isAlphaChar(text[0]) {
		return true
	}

	return false
}

func isDigitChar(b byte) bool {
	return b >= '0' && b <= '9'
}

func isAlphaChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// ValidatePrefixAttributeWithPos validates @prefix attribute with detailed position info.
// Returns (usePrefix, error) where error contains position information.
func ValidatePrefixAttributeWithPos(attrs []*Attribute) (bool, *ValueEnumError) {
	for _, attr := range attrs {
		if attr.Name.Name != "prefix" {
			continue
		}

		// @prefix without arguments
		if len(attr.Args) == 0 {
			return false, &ValueEnumError{
				Pos:     attr.At,
				Kind:    ErrorMissingPrefixArg,
				Message: "@prefix requires a boolean argument: @prefix(true) or @prefix(false)",
			}
		}

		// @prefix with too many arguments
		if len(attr.Args) > 1 {
			return false, &ValueEnumError{
				Pos:     attr.Args[1].Pos(),
				Kind:    ErrorTooManyPrefixArgs,
				Message: fmt.Sprintf("@prefix accepts only one argument, got %d", len(attr.Args)),
			}
		}

		// Check if argument is a boolean literal
		rawExpr, ok := attr.Args[0].(*RawExpr)
		if !ok {
			return false, &ValueEnumError{
				Pos:     attr.Args[0].Pos(),
				Kind:    ErrorInvalidPrefixArg,
				Message: "@prefix argument must be a boolean literal (true or false)",
			}
		}

		switch rawExpr.Text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return false, &ValueEnumError{
				Pos:     rawExpr.StartPos,
				Kind:    ErrorInvalidPrefixArg,
				Message: fmt.Sprintf("@prefix argument must be 'true' or 'false', got %q", rawExpr.Text),
			}
		}
	}

	return true, nil // Default: use prefix
}

// FormatValidationErrors formats validation result for display
func FormatValidationErrors(result *ValueEnumValidationResult, fset *token.FileSet) string {
	if result == nil || (!result.HasErrors() && !result.HasWarnings()) {
		return ""
	}

	var output string

	for _, err := range result.Errors {
		if fset != nil && err.Pos.IsValid() {
			pos := fset.Position(err.Pos)
			output += fmt.Sprintf("error: %s: %s\n", pos, err.Message)
		} else {
			output += fmt.Sprintf("error: %s\n", err.Message)
		}
	}

	for _, warn := range result.Warnings {
		if fset != nil && warn.Pos.IsValid() {
			pos := fset.Position(warn.Pos)
			output += fmt.Sprintf("warning: %s: %s\n", pos, warn.Message)
		} else {
			output += fmt.Sprintf("warning: %s\n", warn.Message)
		}
	}

	return output
}
