package typechecker

import (
	"fmt"
	"strings"
	"unicode"
)

// EnumInfo holds complete information about an enum type.
type EnumInfo struct {
	Name     string        // "Shape"
	Variants []VariantInfo // All variants in declaration order
}

// VariantInfo holds information about a single enum variant.
type VariantInfo struct {
	Name       string   // "Point" (NOT "ShapePoint")
	FullName   string   // "ShapePoint" (as generated in Go)
	Fields     []string // Field names for tuple/struct variants
	FieldTypes []string // Field types for tuple/struct variants
}

// EnumRegistry maps variant names to their enum types.
// It provides efficient lookup for exhaustiveness checking and pattern normalization.
type EnumRegistry struct {
	enumsByName    map[string]*EnumInfo // "Shape" -> EnumInfo
	variantToEnum  map[string]*EnumInfo // "Point" -> EnumInfo (variant name)
	fullNameToEnum map[string]*EnumInfo // "ShapePoint" -> EnumInfo (full name)
}

// NewEnumRegistry creates a new empty EnumRegistry.
func NewEnumRegistry() *EnumRegistry {
	return &EnumRegistry{
		enumsByName:    make(map[string]*EnumInfo),
		variantToEnum:  make(map[string]*EnumInfo),
		fullNameToEnum: make(map[string]*EnumInfo),
	}
}

// RegisterEnum registers an enum type with its variants.
// The variants slice should contain all variants in declaration order.
// Returns an error if a variant name collision is detected.
func (r *EnumRegistry) RegisterEnum(name string, variants []VariantInfo) error {
	info := &EnumInfo{
		Name:     name,
		Variants: variants,
	}

	// Store by enum name
	r.enumsByName[name] = info

	// Index by variant name and full name
	for _, v := range variants {
		// Check for variant name collision
		if existing := r.variantToEnum[v.Name]; existing != nil && existing.Name != name {
			return fmt.Errorf(
				"variant name collision: '%s' exists in both enum %s and enum %s",
				v.Name, existing.Name, name,
			)
		}

		// Map variant name (e.g., "Point") to enum
		r.variantToEnum[v.Name] = info

		// Map full name (e.g., "ShapePoint") to enum
		r.fullNameToEnum[v.FullName] = info
	}
	return nil
}

// GetEnum returns the EnumInfo for the given enum name, or nil if not found.
func (r *EnumRegistry) GetEnum(name string) *EnumInfo {
	return r.enumsByName[name]
}

// GetEnumForVariant returns the EnumInfo that defines the given variant name.
// Returns nil if the variant is not registered.
//
// Example: GetEnumForVariant("Point") -> EnumInfo for "Shape"
func (r *EnumRegistry) GetEnumForVariant(variantName string) *EnumInfo {
	return r.variantToEnum[variantName]
}

// GetEnumForFullName returns the EnumInfo for the given full variant name.
// Returns nil if the full name is not registered.
//
// Example: GetEnumForFullName("ShapePoint") -> EnumInfo for "Shape"
func (r *EnumRegistry) GetEnumForFullName(fullName string) *EnumInfo {
	return r.fullNameToEnum[fullName]
}

// GetAllVariants returns a slice of all variant names for the given enum.
// Returns nil if the enum is not registered.
func (r *EnumRegistry) GetAllVariants(enumName string) []string {
	info := r.enumsByName[enumName]
	if info == nil {
		return nil
	}

	variants := make([]string, len(info.Variants))
	for i, v := range info.Variants {
		variants[i] = v.Name
	}
	return variants
}

// NormalizePatternName resolves a pattern name to its enum and variant names.
// Only supports PascalCase naming (e.g., "ShapePoint" or "Point").
// Rejects underscore syntax (e.g., "Shape_Point") with a clear error.
//
// Returns:
//   - enumName: The enum type name (e.g., "Shape")
//   - variantName: The variant name (e.g., "Point")
//   - ok: true if pattern was successfully normalized, false otherwise
//
// Examples:
//   - "ShapePoint" -> ("Shape", "Point", true)  // Full name lookup
//   - "Point" -> ("Shape", "Point", true)       // Variant name lookup
//   - "Shape_Point" -> ("", "", false)          // Deprecated underscore syntax
//   - "Unknown" -> ("", "", false)              // Not registered
func (r *EnumRegistry) NormalizePatternName(pattern string) (enumName, variantName string, ok bool) {
	// Detect deprecated underscore syntax early
	if strings.Contains(pattern, "_") {
		return "", "", false
	}

	// Validate PascalCase (must start with uppercase letter)
	if len(pattern) == 0 || !unicode.IsUpper(rune(pattern[0])) {
		return "", "", false
	}

	// Strategy 1: Check full name lookup (e.g., "ShapePoint")
	if info := r.fullNameToEnum[pattern]; info != nil {
		// Find the variant with this full name
		for _, v := range info.Variants {
			if v.FullName == pattern {
				return info.Name, v.Name, true
			}
		}
	}

	// Strategy 2: Check variant name only (e.g., "Point")
	// This requires a unique variant name across all enums
	if info := r.variantToEnum[pattern]; info != nil {
		return info.Name, pattern, true
	}

	// Not found
	return "", "", false
}

// ValidatePatternName validates a pattern name and returns a detailed error if invalid.
// This is useful for providing clear error messages in the parser/checker.
//
// Returns nil if the pattern is valid, or an error describing the issue.
func (r *EnumRegistry) ValidatePatternName(pattern string) error {
	// Check for underscore syntax
	if strings.Contains(pattern, "_") {
		return fmt.Errorf("deprecated pattern syntax: use PascalCase '%s' instead of '%s'",
			strings.ReplaceAll(pattern, "_", ""), pattern)
	}

	// Check for PascalCase
	if len(pattern) == 0 {
		return fmt.Errorf("pattern name cannot be empty")
	}

	if !unicode.IsUpper(rune(pattern[0])) {
		return fmt.Errorf("pattern names must be PascalCase (start with uppercase letter): '%s'", pattern)
	}

	// Check if registered
	if _, _, ok := r.NormalizePatternName(pattern); !ok {
		return fmt.Errorf("unknown pattern name: '%s'", pattern)
	}

	return nil
}

// Clone creates a deep copy of the EnumRegistry.
// This is useful for creating isolated registries for testing or parallel processing.
func (r *EnumRegistry) Clone() *EnumRegistry {
	clone := NewEnumRegistry()

	// Copy all enum info
	for name, info := range r.enumsByName {
		// Deep copy variants
		variants := make([]VariantInfo, len(info.Variants))
		for i, v := range info.Variants {
			variants[i] = VariantInfo{
				Name:       v.Name,
				FullName:   v.FullName,
				Fields:     append([]string(nil), v.Fields...),
				FieldTypes: append([]string(nil), v.FieldTypes...),
			}
		}

		if err := clone.RegisterEnum(name, variants); err != nil {
			// Clone should not fail since original registry was valid
			panic("clone failed: " + err.Error())
		}
	}

	return clone
}

// Size returns the number of registered enums.
func (r *EnumRegistry) Size() int {
	return len(r.enumsByName)
}

// VariantCount returns the total number of variants across all enums.
func (r *EnumRegistry) VariantCount() int {
	count := 0
	for _, info := range r.enumsByName {
		count += len(info.Variants)
	}
	return count
}
