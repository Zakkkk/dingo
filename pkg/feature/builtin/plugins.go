// Package builtin provides the built-in Dingo language feature plugins.
// These plugins wrap the existing transform functions from pkg/goparser/parser.
//
// The plugin system uses a function pointer pattern to delegate to existing
// implementations. This allows gradual migration without breaking changes.
package builtin

import (
	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/feature"
)

// --- Transform function pointers ---
// These are set by the parser package to avoid import cycles

// TransformFuncs holds pointers to the actual transform functions
type TransformFuncs struct {
	Enum              func(src []byte) []byte
	Match             func(src []byte) []byte
	EnumConstructors  func(src []byte) []byte
	ErrorProp         func(src []byte) []byte
	Guard             func(src []byte) []byte
	SafeNavStatements func(src []byte) []byte
	SafeNav           func(src []byte) []byte
	NullCoalesce      func(src []byte) []byte
	Lambdas           func(src []byte) []byte
}

// Transforms holds the registered transform functions
var Transforms TransformFuncs

// --- Enum Plugin ---

type EnumPlugin struct{}

func (p *EnumPlugin) Name() string             { return "enum" }
func (p *EnumPlugin) Version() string          { return "1.0.0" }
func (p *EnumPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *EnumPlugin) Priority() int            { return 10 }
func (p *EnumPlugin) Dependencies() []string   { return nil }
func (p *EnumPlugin) Conflicts() []string      { return nil }
func (p *EnumPlugin) Detect(src []byte) []feature.SyntaxLocation {
	// Detection removed: if feature is disabled, user will get a compile error anyway.
	// Regex-based detection was removed to eliminate false positives and architectural violations.
	return nil
}
func (p *EnumPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	// Call the AST-based enum transformer directly
	// Pass empty filename since this is the plugin path (not full transpile with position tracking)
	transformedSrc, enumRegistry := dingoast.TransformEnumSource(src, "")

	// Store enum registry in context for enum_constructors plugin
	if ctx != nil && ctx.Registry != nil && enumRegistry != nil {
		ctx.Registry.Set("enum_registry", enumRegistry)
	}

	return transformedSrc, nil
}

// --- Match Plugin ---

type MatchPlugin struct{}

func (p *MatchPlugin) Name() string             { return "match" }
func (p *MatchPlugin) Version() string          { return "1.0.0" }
func (p *MatchPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *MatchPlugin) Priority() int            { return 20 }
func (p *MatchPlugin) Dependencies() []string   { return []string{"enum"} }
func (p *MatchPlugin) Conflicts() []string      { return nil }
func (p *MatchPlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *MatchPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.Match == nil {
		return src, nil
	}
	return Transforms.Match(src), nil
}

// --- Enum Constructors Plugin ---

type EnumConstructorsPlugin struct{}

func (p *EnumConstructorsPlugin) Name() string             { return "enum_constructors" }
func (p *EnumConstructorsPlugin) Version() string          { return "1.0.0" }
func (p *EnumConstructorsPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *EnumConstructorsPlugin) Priority() int            { return 30 }
func (p *EnumConstructorsPlugin) Dependencies() []string   { return []string{"enum"} }
func (p *EnumConstructorsPlugin) Conflicts() []string      { return nil }
func (p *EnumConstructorsPlugin) Detect(src []byte) []feature.SyntaxLocation {
	// This is harder to detect without enum registry
	// For now, return nil (detection not supported)
	return nil
}
func (p *EnumConstructorsPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	// Get enum registry from context (populated by EnumPlugin)
	var enumRegistry map[string]string
	if ctx != nil && ctx.Registry != nil {
		if regVal, ok := ctx.Registry.Get("enum_registry"); ok {
			if reg, ok := regVal.(map[string]string); ok {
				enumRegistry = reg
			}
		}
	}

	// If no registry, nothing to transform
	if len(enumRegistry) == 0 {
		return src, nil
	}

	// Call the AST-based enum constructor transformer
	return dingoast.TransformEnumConstructors(src, enumRegistry), nil
}

// --- Error Propagation Plugin ---

type ErrorPropPlugin struct{}

func (p *ErrorPropPlugin) Name() string             { return "error_prop" }
func (p *ErrorPropPlugin) Version() string          { return "1.0.0" }
func (p *ErrorPropPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *ErrorPropPlugin) Priority() int            { return 40 }
func (p *ErrorPropPlugin) Dependencies() []string   { return nil }
func (p *ErrorPropPlugin) Conflicts() []string      { return nil }
func (p *ErrorPropPlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *ErrorPropPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.ErrorProp == nil {
		return src, nil
	}
	return Transforms.ErrorProp(src), nil
}

// --- Guard Plugin ---

type GuardPlugin struct{}

func (p *GuardPlugin) Name() string             { return "guard" }
func (p *GuardPlugin) Version() string          { return "1.0.0" }
func (p *GuardPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *GuardPlugin) Priority() int            { return 50 }
func (p *GuardPlugin) Dependencies() []string   { return []string{"error_prop"} }
func (p *GuardPlugin) Conflicts() []string      { return nil }
func (p *GuardPlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *GuardPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.Guard == nil {
		return src, nil
	}
	return Transforms.Guard(src), nil
}

// --- Safe Nav Statements Plugin ---

type SafeNavStatementsPlugin struct{}

func (p *SafeNavStatementsPlugin) Name() string             { return "safe_nav_statements" }
func (p *SafeNavStatementsPlugin) Version() string          { return "1.0.0" }
func (p *SafeNavStatementsPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *SafeNavStatementsPlugin) Priority() int            { return 55 }
func (p *SafeNavStatementsPlugin) Dependencies() []string   { return nil }
func (p *SafeNavStatementsPlugin) Conflicts() []string      { return nil }
func (p *SafeNavStatementsPlugin) Detect(src []byte) []feature.SyntaxLocation {
	// This is part of safe_nav, so use same detection
	return nil
}
func (p *SafeNavStatementsPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.SafeNavStatements == nil {
		return src, nil
	}
	return Transforms.SafeNavStatements(src), nil
}

// --- Safe Nav Plugin ---

type SafeNavPlugin struct{}

func (p *SafeNavPlugin) Name() string             { return "safe_nav" }
func (p *SafeNavPlugin) Version() string          { return "1.0.0" }
func (p *SafeNavPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *SafeNavPlugin) Priority() int            { return 60 }
func (p *SafeNavPlugin) Dependencies() []string   { return nil }
func (p *SafeNavPlugin) Conflicts() []string      { return nil }
func (p *SafeNavPlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *SafeNavPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.SafeNav == nil {
		return src, nil
	}
	return Transforms.SafeNav(src), nil
}

// --- Null Coalesce Plugin ---

type NullCoalescePlugin struct{}

func (p *NullCoalescePlugin) Name() string             { return "null_coalesce" }
func (p *NullCoalescePlugin) Version() string          { return "1.0.0" }
func (p *NullCoalescePlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *NullCoalescePlugin) Priority() int            { return 70 }
func (p *NullCoalescePlugin) Dependencies() []string   { return []string{"safe_nav"} }
func (p *NullCoalescePlugin) Conflicts() []string      { return nil }
func (p *NullCoalescePlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *NullCoalescePlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.NullCoalesce == nil {
		return src, nil
	}
	return Transforms.NullCoalesce(src), nil
}

// --- Lambdas Plugin ---

type LambdasPlugin struct{}

func (p *LambdasPlugin) Name() string             { return "lambdas" }
func (p *LambdasPlugin) Version() string          { return "1.0.0" }
func (p *LambdasPlugin) Type() feature.PluginType { return feature.CharacterLevel }
func (p *LambdasPlugin) Priority() int            { return 80 }
func (p *LambdasPlugin) Dependencies() []string   { return nil }
func (p *LambdasPlugin) Conflicts() []string      { return nil }
func (p *LambdasPlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *LambdasPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	if Transforms.Lambdas == nil {
		return src, nil
	}
	return Transforms.Lambdas(src), nil
}

// --- Token-Level Plugins ---

// GenericsPlugin transforms `<T>` to `[T]`
type GenericsPlugin struct{}

func (p *GenericsPlugin) Name() string             { return "generics" }
func (p *GenericsPlugin) Version() string          { return "1.0.0" }
func (p *GenericsPlugin) Type() feature.PluginType { return feature.TokenLevel }
func (p *GenericsPlugin) Priority() int            { return 110 }
func (p *GenericsPlugin) Dependencies() []string   { return nil }
func (p *GenericsPlugin) Conflicts() []string      { return nil }
func (p *GenericsPlugin) Detect(src []byte) []feature.SyntaxLocation {
	return nil // Detection removed: compile errors suffice for disabled features
}
func (p *GenericsPlugin) Transform(src []byte, ctx *feature.Context) ([]byte, error) {
	// Transforms handled by AST pipeline (pkg/ast.TransformSource)
	return src, nil
}
