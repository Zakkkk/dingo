package preprocessor

import (
	"go/types"
	"strings"
)

// OptionDetector identifies Option types using dual strategy:
// 1. Naming convention: types ending in "Option" or matching "Option[T]"
// 2. Method signatures: types with Option-specific methods
type OptionDetector struct {
	// Required methods for method-based detection
	// At least 2 of these must be present
	requiredMethods []string
}

// NewOptionDetector creates a new Option type detector
func NewOptionDetector() *OptionDetector {
	return &OptionDetector{
		requiredMethods: []string{
			"IsNone",
			"IsSome",
			"Unwrap",
			"UnwrapOr",
		},
	}
}

// IsOption checks if a type is an Option using dual strategy
// Returns true if EITHER naming convention OR method signatures match
func (od *OptionDetector) IsOption(t types.Type) bool {
	if t == nil {
		return false
	}

	// Strategy 1: Naming convention check
	if od.IsOptionByName(typeNameFromType(t)) {
		return true
	}

	// Strategy 2: Method signature check
	if od.IsOptionByMethods(t) {
		return true
	}

	return false
}

// IsOptionByName checks if a type name follows Option naming convention
// Matches: Option, *Option, Option[T], UserOption, StringOption, HTTPOption, URLOption, etc.
// Does NOT match: Optional, Optionable, NotAnOption, Proportional (substring only)
func (od *OptionDetector) IsOptionByName(typeName string) bool {
	if typeName == "" {
		return false
	}

	// Remove pointer prefix for checking
	name := strings.TrimPrefix(typeName, "*")

	// Check for generic syntax first: Option[T] or XOption[T]
	if idx := strings.Index(name, "Option["); idx != -1 {
		// Verify "Option" is at start or preceded by valid prefix
		if idx == 0 {
			return true // Option[T]
		}
		// XOption[T] - has valid prefix before "Option["
		return idx > 0
	}

	// Must end with "Option" for non-generic cases
	// This prevents false positives like "Proportional"
	if !strings.HasSuffix(name, "Option") {
		return false
	}

	// Blacklist common false positives
	blacklist := []string{"Optional", "Optionable", "NotAnOption"}
	for _, bad := range blacklist {
		if name == bad || strings.HasSuffix(name, bad) {
			return false
		}
	}

	// Valid patterns:
	// - "Option" (exact match)
	// - "XOption" (suffix, X is any valid prefix including acronyms like HTTP, URL, IO)
	if name == "Option" {
		return true
	}

	// Has valid prefix before "Option" suffix
	prefix := name[:len(name)-6] // Remove "Option" suffix (6 chars)
	return len(prefix) > 0        // Has non-empty prefix
}

// IsOptionByMethods checks if a type has Option-like method signatures
// Requires at least 2 of the standard Option methods to be present
func (od *OptionDetector) IsOptionByMethods(t types.Type) bool {
	if t == nil {
		return false
	}

	// Get method set for the type
	methodSet := types.NewMethodSet(t)
	if methodSet.Len() == 0 {
		return false
	}

	// Count how many required methods are present
	matchCount := 0
	for _, methodName := range od.requiredMethods {
		if hasMethod(methodSet, methodName) {
			matchCount++
		}
	}

	// Require at least 2 methods to reduce false positives
	return matchCount >= 2
}

// GetInnerType extracts the inner type T from Option[T]
// Returns (innerType, true) if successful, (nil, false) otherwise
func (od *OptionDetector) GetInnerType(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}

	// Handle pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	// Check if it's a named type
	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}

	// Check if it has type parameters (generic Option[T])
	typeArgs := named.TypeArgs()
	if typeArgs != nil && typeArgs.Len() > 0 {
		// Return the first type argument (T in Option[T])
		return typeArgs.At(0), true
	}

	// For non-generic Option types, try to infer from Unwrap method
	methodSet := types.NewMethodSet(t)
	unwrapMethod := findMethod(methodSet, "Unwrap")
	if unwrapMethod != nil {
		// Get the return type of Unwrap()
		sig, ok := unwrapMethod.Type().(*types.Signature)
		if ok && sig.Results().Len() > 0 {
			return sig.Results().At(0).Type(), true
		}
	}

	return nil, false
}

// Helper: Extract type name as string from types.Type
func typeNameFromType(t types.Type) string {
	if t == nil {
		return ""
	}

	// Handle pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		return "*" + typeNameFromType(ptr.Elem())
	}

	// Handle named types
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj == nil {
			return ""
		}

		name := obj.Name()

		// Include type arguments if present (e.g., "Option[string]")
		typeArgs := named.TypeArgs()
		if typeArgs != nil && typeArgs.Len() > 0 {
			var args []string
			for i := 0; i < typeArgs.Len(); i++ {
				args = append(args, typeNameFromType(typeArgs.At(i)))
			}
			name += "[" + strings.Join(args, ",") + "]"
		}

		return name
	}

	// Handle basic types
	if basic, ok := t.(*types.Basic); ok {
		return basic.Name()
	}

	return ""
}

// Helper: Check if method set contains a method with given name
func hasMethod(methodSet *types.MethodSet, name string) bool {
	for i := 0; i < methodSet.Len(); i++ {
		if methodSet.At(i).Obj().Name() == name {
			return true
		}
	}
	return false
}

// Helper: Find method by name in method set
func findMethod(methodSet *types.MethodSet, name string) *types.Selection {
	for i := 0; i < methodSet.Len(); i++ {
		sel := methodSet.At(i)
		if sel.Obj().Name() == name {
			return sel
		}
	}
	return nil
}
