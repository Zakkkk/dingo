package semantic

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"go.lsp.dev/protocol"
)

// FormatHover creates LSP hover response from semantic entity
func FormatHover(entity *SemanticEntity, pkg *types.Package) *protocol.Hover {
	return FormatHoverWithDocs(entity, pkg, nil)
}

// FormatHoverWithDocs creates LSP hover response with optional documentation.
// If docProvider is non-nil, external symbols will include their documentation.
func FormatHoverWithDocs(entity *SemanticEntity, pkg *types.Package, docProvider *DocProvider) *protocol.Hover {
	if entity == nil {
		return nil
	}

	var content string

	// Handle operators separately
	if entity.Kind == KindOperator && entity.Context != nil {
		content = formatOperatorHover(entity, pkg)
	} else if entity.Kind == KindLambda && entity.Context != nil {
		// Lambda parameter
		content = formatLambdaHover(entity, pkg)
	} else if entity.Context != nil && entity.Context.Kind == ContextErrorProp {
		// Error propagation context on a variable
		content = formatErrorPropHover(entity, pkg)
	} else if entity.Context != nil && entity.Context.Description != "" {
		// Entity with only context description (e.g., Option constructors like None/Some)
		content = entity.Context.Description
	} else if entity.Object != nil {
		// Named entity (variable, function, etc.)
		content = formatObjectHoverWithDocs(entity, pkg, docProvider)
	} else if entity.Type != nil {
		// Expression without object
		content = formatTypeHover(entity, pkg)
	} else {
		return nil
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: content,
		},
	}
}

// formatErrorPropHover formats hover for error propagation variable
// Shows: "var x T (from Result[T, E])"
func formatErrorPropHover(entity *SemanticEntity, pkg *types.Package) string {
	ctx := entity.Context
	var b strings.Builder

	// Variable/expression name and unwrapped type
	b.WriteString("```go\n")
	if entity.Object != nil {
		b.WriteString(fmt.Sprintf("var %s %s",
			entity.Object.Name(),
			formatType(ctx.UnwrappedType, pkg)))
	} else {
		b.WriteString(formatType(ctx.UnwrappedType, pkg))
	}
	b.WriteString("\n```\n\n")

	// Origin information
	b.WriteString(fmt.Sprintf("*(from `%s`)*\n\n",
		formatType(ctx.OriginalType, pkg)))

	// Explanation
	b.WriteString("Error propagation: returns early if result is `Err`")

	return b.String()
}

// formatOperatorHover formats hover for Dingo operators
func formatOperatorHover(entity *SemanticEntity, pkg *types.Package) string {
	ctx := entity.Context
	if ctx == nil {
		return ""
	}

	var b strings.Builder

	switch ctx.Kind {
	case ContextErrorProp:
		b.WriteString("**`?` error propagation**\n\n")
		if ctx.OriginalType != nil && ctx.UnwrappedType != nil {
			// Result[T, E] pattern
			b.WriteString(fmt.Sprintf("Unwraps `%s` to `%s`\n\n",
				formatType(ctx.OriginalType, pkg),
				formatType(ctx.UnwrappedType, pkg)))
			b.WriteString("Returns early with error if result is `Err`")
		} else {
			// Go's (T, error) pattern
			b.WriteString("Unwraps Go's `(T, error)` pattern.\n\n")
			b.WriteString("- If `error` is **non-nil**: returns early from the function\n")
			b.WriteString("- If `error` is **nil**: continues with the unwrapped value\n\n")
			b.WriteString("*Equivalent to:*\n```go\nval, err := expr\nif err != nil {\n    return ..., err\n}\n```")
		}

	case ContextNullCoal:
		b.WriteString("**`??` null coalescing**\n\n")
		if ctx.UnwrappedType != nil {
			b.WriteString(fmt.Sprintf("Type: `%s`\n\n",
				formatType(ctx.UnwrappedType, pkg)))
		}
		b.WriteString("Returns left operand if non-nil, otherwise right operand")

	case ContextSafeNav:
		b.WriteString("**`?.` safe navigation**\n\n")
		if ctx.UnwrappedType != nil {
			b.WriteString(fmt.Sprintf("Type: `%s`\n\n",
				formatType(ctx.UnwrappedType, pkg)))
		}
		b.WriteString("Returns `nil` if receiver is `nil`, otherwise accesses field")

	case ContextTernary:
		b.WriteString("**`? :` ternary conditional**\n\n")
		b.WriteString("```dingo\ncondition ? valueIfTrue : valueIfFalse\n```\n\n")
		b.WriteString("Returns first value if condition is true, otherwise second value")

	case ContextGuard:
		b.WriteString("**`guard` statement**\n\n")
		b.WriteString("```dingo\nguard value := expr else { return ... }\n```\n\n")
		b.WriteString("Unwraps an `Option` or `Result`, executing the else block if `None`/`Err`.\n\n")
		b.WriteString("Similar to Swift's guard-let statement.")

	default:
		if ctx.Description != "" {
			b.WriteString(ctx.Description)
		}
	}

	return b.String()
}

// formatLambdaHover formats hover for lambda parameters
func formatLambdaHover(entity *SemanticEntity, pkg *types.Package) string {
	var b strings.Builder

	// Get the actual parameter name from context
	name := "x" // fallback
	if entity.Context != nil && entity.Context.Name != "" {
		name = entity.Context.Name
	}

	b.WriteString("```dingo\n")
	b.WriteString(name)
	b.WriteString("\n```\n\n")
	b.WriteString("*Lambda parameter* — type inferred by Go compiler")

	return b.String()
}

// formatObjectHover formats hover for named objects (vars, funcs, etc.)
func formatObjectHover(entity *SemanticEntity, pkg *types.Package) string {
	return formatObjectHoverWithDocs(entity, pkg, nil)
}

// formatObjectHoverWithDocs formats hover with optional documentation.
func formatObjectHoverWithDocs(entity *SemanticEntity, pkg *types.Package, docProvider *DocProvider) string {
	// Check for types that have their own complete hover formatting
	// These return full markdown including code fences
	if typeName, ok := entity.Object.(*types.TypeName); ok {
		// Check for dgo types (Result, Option)
		if dgoInfo := detectDgoType(typeName.Type()); dgoInfo != nil {
			switch dgoInfo.TypeName {
			case "Result":
				return formatDgoResultHover(dgoInfo, pkg)
			case "Option":
				return formatDgoOptionHover(dgoInfo, pkg)
			}
		}
		// Check for Dingo enum types
		// Use the object's package to find variants (not the document package)
		objPkg := typeName.Pkg()
		if objPkg == nil {
			objPkg = pkg // Fall back to document package
		}
		if enumInfo := detectDingoEnumType(typeName.Type(), objPkg); enumInfo != nil {
			return formatDingoEnumHover(enumInfo)
		}
		// Check for Dingo enum variants
		if variantInfo := detectDingoVariantType(typeName.Type(), pkg); variantInfo != nil {
			return formatDingoVariantHover(variantInfo)
		}
	}

	// Check for dgo constructor functions (None, Some, Ok, Err)
	if fn, ok := entity.Object.(*types.Func); ok {
		if hover := formatDgoConstructorHover(fn, pkg); hover != "" {
			return hover
		}
	}

	var b strings.Builder

	b.WriteString("```go\n")
	b.WriteString(formatSignature(entity.Object, pkg))
	b.WriteString("\n```")

	// Add documentation for external symbols
	if docProvider != nil && entity.Object != nil {
		isExternal := IsExternalPackage(entity.Object, pkg)
		if isExternal {
			docStr := docProvider.GetDoc(entity.Object)
			if docStr != "" {
				b.WriteString("\n\n")
				b.WriteString(docStr)
			}
		}
	}

	// Add context description if available
	if entity.Context != nil && entity.Context.Description != "" {
		b.WriteString("\n\n")
		b.WriteString(entity.Context.Description)
	}

	return b.String()
}

// formatTypeHover formats hover for expressions without objects
func formatTypeHover(entity *SemanticEntity, pkg *types.Package) string {
	// Check for dgo types first (Result, Option)
	if dgoInfo := detectDgoType(entity.Type); dgoInfo != nil {
		switch dgoInfo.TypeName {
		case "Result":
			return formatDgoResultHover(dgoInfo, pkg)
		case "Option":
			return formatDgoOptionHover(dgoInfo, pkg)
		}
	}

	// Check for dgo constructor signatures (instantiated generic constructors like Err[User])
	if sig, ok := entity.Type.(*types.Signature); ok {
		if hover := formatDgoConstructorSignatureHover(sig, pkg); hover != "" {
			return hover
		}
	}

	var b strings.Builder

	b.WriteString("```go\n")
	b.WriteString(formatType(entity.Type, pkg))
	b.WriteString("\n```")

	// Add context description if available
	if entity.Context != nil && entity.Context.Description != "" {
		b.WriteString("\n\n")
		b.WriteString(entity.Context.Description)
	}

	return b.String()
}

// formatSignature formats the signature of an object
func formatSignature(obj types.Object, pkg *types.Package) string {
	if obj == nil {
		return ""
	}

	switch obj := obj.(type) {
	case *types.Var:
		// Check if variable has dgo type
		if dgoInfo := detectDgoType(obj.Type()); dgoInfo != nil {
			switch dgoInfo.TypeName {
			case "Result":
				return fmt.Sprintf("var %s %s", obj.Name(), formatDgoTypeShort(dgoInfo, pkg))
			case "Option":
				return fmt.Sprintf("var %s %s", obj.Name(), formatDgoTypeShort(dgoInfo, pkg))
			}
		}
		// Check if variable has enum type
		if enumInfo := detectDingoEnumType(obj.Type(), pkg); enumInfo != nil {
			return fmt.Sprintf("var %s %s", obj.Name(), enumInfo.Name)
		}
		if obj.IsField() {
			return fmt.Sprintf("field %s %s", obj.Name(), formatType(obj.Type(), pkg))
		}
		return fmt.Sprintf("var %s %s", obj.Name(), formatType(obj.Type(), pkg))

	case *types.Const:
		return fmt.Sprintf("const %s %s", obj.Name(), formatType(obj.Type(), pkg))

	case *types.Func:
		sig := obj.Type().(*types.Signature)
		return fmt.Sprintf("func %s%s", obj.Name(), formatSignatureType(sig, pkg))

	case *types.TypeName:
		// Note: dgo types, enums, and variants are handled in formatObjectHoverWithDocs
		// before calling formatSignature, so they won't reach here.
		// Check for tuple types (structs with First, Second, Third... fields)
		if tupleStr := formatTupleType(obj.Type().Underlying()); tupleStr != "" {
			return fmt.Sprintf("type %s = %s", obj.Name(), tupleStr)
		}
		return fmt.Sprintf("type %s %s", obj.Name(), formatType(obj.Type().Underlying(), pkg))

	default:
		return obj.String()
	}
}

// formatDgoTypeShort formats a dgo type in short form for variable signatures
func formatDgoTypeShort(info *dgoTypeInfo, pkg *types.Package) string {
	var b strings.Builder
	b.WriteString(info.TypeName)
	b.WriteString("[")
	for i, arg := range info.TypeArgs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatType(arg, pkg))
	}
	b.WriteString("]")
	return b.String()
}

// formatSignatureType formats a function signature
func formatSignatureType(sig *types.Signature, pkg *types.Package) string {
	var b strings.Builder

	// Type parameters (generics)
	if tparams := sig.TypeParams(); tparams != nil && tparams.Len() > 0 {
		b.WriteString("[")
		for i := 0; i < tparams.Len(); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tparams.At(i).String())
		}
		b.WriteString("]")
	}

	// Parameters
	b.WriteString("(")
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		param := params.At(i)
		if name := param.Name(); name != "" {
			b.WriteString(name)
			b.WriteString(" ")
		}
		b.WriteString(formatType(param.Type(), pkg))
	}
	if sig.Variadic() {
		// Replace last parameter type with variadic syntax
		if params.Len() > 0 {
			// Remove the last type and add ... prefix
			lastParam := params.At(params.Len() - 1)
			// The type is already a slice, so we need to show ...element instead
			if sliceType, ok := lastParam.Type().(*types.Slice); ok {
				// Rewrite last parameter
				result := b.String()
				lastTypeStart := strings.LastIndex(result, formatType(lastParam.Type(), pkg))
				if lastTypeStart >= 0 {
					b.Reset()
					b.WriteString(result[:lastTypeStart])
					b.WriteString("...")
					b.WriteString(formatType(sliceType.Elem(), pkg))
				}
			}
		}
	}
	b.WriteString(")")

	// Results
	results := sig.Results()
	switch results.Len() {
	case 0:
		// No return values
	case 1:
		b.WriteString(" ")
		b.WriteString(formatType(results.At(0).Type(), pkg))
	default:
		b.WriteString(" (")
		for i := 0; i < results.Len(); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			result := results.At(i)
			if name := result.Name(); name != "" {
				b.WriteString(name)
				b.WriteString(" ")
			}
			b.WriteString(formatType(result.Type(), pkg))
		}
		b.WriteString(")")
	}

	return b.String()
}

// tupleFieldNames are the expected field names for Dingo tuple types
var tupleFieldNames = []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth"}

// formatTupleType checks if a type is a Dingo tuple (struct with First, Second, etc. fields)
// and returns the Dingo tuple syntax, e.g., "(float64, float64)"
func formatTupleType(t types.Type) string {
	st, ok := t.(*types.Struct)
	if !ok || st.NumFields() == 0 || st.NumFields() > len(tupleFieldNames) {
		return ""
	}

	// Check that all fields match tuple naming pattern
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Name() != tupleFieldNames[i] {
			return ""
		}
	}

	// Format as Dingo tuple: (T1, T2, ...)
	var b strings.Builder
	b.WriteString("(")
	for i := 0; i < st.NumFields(); i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(types.TypeString(st.Field(i).Type(), nil))
	}
	b.WriteString(")")
	return b.String()
}

// formatType formats a type with package qualification
func formatType(t types.Type, pkg *types.Package) string {
	if t == nil {
		return ""
	}

	// Use types.TypeString with custom qualifier
	qualifier := func(other *types.Package) string {
		if other == nil {
			return ""
		}
		// Same package - no qualifier needed
		if pkg != nil && other == pkg {
			return ""
		}
		// Different package - use package name
		return other.Name()
	}

	return types.TypeString(t, qualifier)
}

// dgoTypeInfo holds information about a dgo type (Result or Option)
type dgoTypeInfo struct {
	TypeName string       // "Result" or "Option"
	TypeArgs []types.Type // [T, E] for Result, [T] for Option
}

// detectDgoType checks if a type is dgo.Result or dgo.Option
func detectDgoType(t types.Type) *dgoTypeInfo {
	if t == nil {
		return nil
	}

	named, ok := t.(*types.Named)
	if !ok {
		return nil
	}

	obj := named.Obj()
	if obj == nil {
		return nil
	}

	typeName := obj.Name()
	if typeName != "Result" && typeName != "Option" {
		return nil
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return nil
	}

	// Check for dgo package (handles various import paths)
	if !strings.Contains(pkg.Path(), "dgo") && pkg.Name() != "dgo" {
		return nil
	}

	// Extract type arguments
	var typeArgs []types.Type
	if args := named.TypeArgs(); args != nil {
		for i := 0; i < args.Len(); i++ {
			typeArgs = append(typeArgs, args.At(i))
		}
	}

	return &dgoTypeInfo{
		TypeName: typeName,
		TypeArgs: typeArgs,
	}
}

// formatDgoResultHover formats hover for Result[T, E]
func formatDgoResultHover(info *dgoTypeInfo, pkg *types.Package) string {
	var b strings.Builder

	// Type signature
	b.WriteString("```go\n")
	b.WriteString("Result[")
	tStr := "T"
	eStr := "E"
	if len(info.TypeArgs) >= 1 {
		tStr = formatType(info.TypeArgs[0], pkg)
		b.WriteString(tStr)
	}
	b.WriteString(", ")
	if len(info.TypeArgs) >= 2 {
		eStr = formatType(info.TypeArgs[1], pkg)
		b.WriteString(eStr)
	}
	b.WriteString("]\n```\n\n")

	// Description
	b.WriteString("**Success OR failure container**\n\n")

	// Methods table
	b.WriteString("| Method | Returns |\n")
	b.WriteString("|--------|--------|\n")
	b.WriteString(fmt.Sprintf("| `.IsOk()` | `bool` |\n"))
	b.WriteString(fmt.Sprintf("| `.IsErr()` | `bool` |\n"))
	b.WriteString(fmt.Sprintf("| `.MustOk()` | `%s` |\n", tStr))
	b.WriteString(fmt.Sprintf("| `.MustErr()` | `%s` |\n", eStr))
	b.WriteString(fmt.Sprintf("| `.OkOr(default)` | `%s` |\n", tStr))

	// Constructors
	b.WriteString(fmt.Sprintf("\n*Constructors:* `Ok(value)`, `Err(err)`"))

	return b.String()
}

// formatDgoOptionHover formats hover for Option[T]
func formatDgoOptionHover(info *dgoTypeInfo, pkg *types.Package) string {
	var b strings.Builder

	// Type signature
	b.WriteString("```go\n")
	b.WriteString("Option[")
	tStr := "T"
	if len(info.TypeArgs) >= 1 {
		tStr = formatType(info.TypeArgs[0], pkg)
		b.WriteString(tStr)
	}
	b.WriteString("]\n```\n\n")

	// Description
	b.WriteString("**Optional value container** (Some or None)\n\n")

	// Methods table
	b.WriteString("| Method | Returns |\n")
	b.WriteString("|--------|--------|\n")
	b.WriteString("| `.IsSome()` | `bool` |\n")
	b.WriteString("| `.IsNone()` | `bool` |\n")
	b.WriteString(fmt.Sprintf("| `.MustSome()` | `%s` |\n", tStr))
	b.WriteString(fmt.Sprintf("| `.SomeOr(default)` | `%s` |\n", tStr))

	// Constructors
	b.WriteString(fmt.Sprintf("\n*Constructors:* `Some(value)`, `None[%s]()`", tStr))

	return b.String()
}

// formatDgoConstructorHover formats hover for dgo constructor functions (None, Some, Ok, Err)
func formatDgoConstructorHover(fn *types.Func, pkg *types.Package) string {
	// Check if this is a dgo package function
	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return ""
	}
	// Accept both the original dgo package and our local copy
	pkgPath := fnPkg.Path()
	if pkgPath != "github.com/nicksrandall/dgo" && pkgPath != "github.com/MadAppGang/dingo/pkg/dgo" {
		return ""
	}

	sig := fn.Type().(*types.Signature)
	results := sig.Results()

	switch fn.Name() {
	case "None":
		// func None[T]() Option[T]
		var b strings.Builder
		tStr := "T"
		if results.Len() > 0 {
			if dgoInfo := detectDgoType(results.At(0).Type()); dgoInfo != nil && len(dgoInfo.TypeArgs) > 0 {
				tStr = formatType(dgoInfo.TypeArgs[0], pkg)
			}
		}

		b.WriteString("```go\n")
		b.WriteString(fmt.Sprintf("func None[%s]() Option[%s]\n", tStr, tStr))
		b.WriteString("```\n\n")
		b.WriteString("**Creates an empty Option** (no value present)\n\n")
		b.WriteString("| Method | Returns |\n")
		b.WriteString("|--------|--------|\n")
		b.WriteString("| `.IsSome()` | `false` |\n")
		b.WriteString("| `.IsNone()` | `true` |\n")
		b.WriteString(fmt.Sprintf("| `.MustSome()` | *panics* |\n"))
		b.WriteString(fmt.Sprintf("| `.SomeOr(default)` | `default` |\n"))
		return b.String()

	case "Some":
		// func Some[T](value T) Option[T]
		var b strings.Builder
		tStr := "T"
		if results.Len() > 0 {
			if dgoInfo := detectDgoType(results.At(0).Type()); dgoInfo != nil && len(dgoInfo.TypeArgs) > 0 {
				tStr = formatType(dgoInfo.TypeArgs[0], pkg)
			}
		}

		b.WriteString("```go\n")
		b.WriteString(fmt.Sprintf("func Some[%s](value %s) Option[%s]\n", tStr, tStr, tStr))
		b.WriteString("```\n\n")
		b.WriteString("**Creates an Option containing a value**\n\n")
		b.WriteString("| Method | Returns |\n")
		b.WriteString("|--------|--------|\n")
		b.WriteString("| `.IsSome()` | `true` |\n")
		b.WriteString("| `.IsNone()` | `false` |\n")
		b.WriteString(fmt.Sprintf("| `.MustSome()` | `%s` |\n", tStr))
		b.WriteString(fmt.Sprintf("| `.SomeOr(default)` | `%s` |\n", tStr))
		return b.String()

	case "Ok":
		// func Ok[T, E](value T) Result[T, E]
		var b strings.Builder
		tStr := "T"
		eStr := "E"
		if results.Len() > 0 {
			if dgoInfo := detectDgoType(results.At(0).Type()); dgoInfo != nil {
				if len(dgoInfo.TypeArgs) > 0 {
					tStr = formatType(dgoInfo.TypeArgs[0], pkg)
				}
				if len(dgoInfo.TypeArgs) > 1 {
					eStr = formatType(dgoInfo.TypeArgs[1], pkg)
				}
			}
		}

		b.WriteString("```go\n")
		b.WriteString(fmt.Sprintf("func Ok[%s, %s](value %s) Result[%s, %s]\n", tStr, eStr, tStr, tStr, eStr))
		b.WriteString("```\n\n")
		b.WriteString("**Creates a successful Result**\n\n")
		b.WriteString("| Method | Returns |\n")
		b.WriteString("|--------|--------|\n")
		b.WriteString("| `.IsOk()` | `true` |\n")
		b.WriteString("| `.IsErr()` | `false` |\n")
		b.WriteString(fmt.Sprintf("| `.MustOk()` | `%s` |\n", tStr))
		b.WriteString("| `.MustErr()` | *panics* |\n")
		b.WriteString(fmt.Sprintf("| `.OkOr(default)` | `%s` |\n", tStr))
		return b.String()

	case "Err":
		// func Err[T, E](err E) Result[T, E]
		var b strings.Builder
		tStr := "T"
		eStr := "E"
		if results.Len() > 0 {
			if dgoInfo := detectDgoType(results.At(0).Type()); dgoInfo != nil {
				if len(dgoInfo.TypeArgs) > 0 {
					tStr = formatType(dgoInfo.TypeArgs[0], pkg)
				}
				if len(dgoInfo.TypeArgs) > 1 {
					eStr = formatType(dgoInfo.TypeArgs[1], pkg)
				}
			}
		}

		b.WriteString("```go\n")
		b.WriteString(fmt.Sprintf("func Err[%s, %s](err %s) Result[%s, %s]\n", tStr, eStr, eStr, tStr, eStr))
		b.WriteString("```\n\n")
		b.WriteString("**Creates a failed Result**\n\n")
		b.WriteString("| Method | Returns |\n")
		b.WriteString("|--------|--------|\n")
		b.WriteString("| `.IsOk()` | `false` |\n")
		b.WriteString("| `.IsErr()` | `true` |\n")
		b.WriteString("| `.MustOk()` | *panics* |\n")
		b.WriteString(fmt.Sprintf("| `.MustErr()` | `%s` |\n", eStr))
		b.WriteString(fmt.Sprintf("| `.OkOr(default)` | `default` |\n"))
		return b.String()
	}

	return ""
}

// formatDgoConstructorSignatureHover formats hover for instantiated dgo constructor signatures
// This handles cases like Err[User] which become func(err string) Result[User, string]
func formatDgoConstructorSignatureHover(sig *types.Signature, pkg *types.Package) string {
	results := sig.Results()
	if results.Len() != 1 {
		return ""
	}

	dgoInfo := detectDgoType(results.At(0).Type())
	if dgoInfo == nil {
		return ""
	}

	params := sig.Params()

	switch dgoInfo.TypeName {
	case "Option":
		tStr := "T"
		if len(dgoInfo.TypeArgs) > 0 {
			tStr = formatType(dgoInfo.TypeArgs[0], pkg)
		}

		// Detect if this is None (no params) or Some (one param)
		if params.Len() == 0 {
			// None[T]() signature
			var b strings.Builder
			b.WriteString("```go\n")
			b.WriteString(fmt.Sprintf("func None[%s]() Option[%s]\n", tStr, tStr))
			b.WriteString("```\n\n")
			b.WriteString("**Creates an empty Option** (no value present)\n\n")
			b.WriteString("| Method | Returns |\n")
			b.WriteString("|--------|--------|\n")
			b.WriteString("| `.IsSome()` | `false` |\n")
			b.WriteString("| `.IsNone()` | `true` |\n")
			b.WriteString("| `.MustSome()` | *panics* |\n")
			b.WriteString("| `.SomeOr(default)` | `default` |\n")
			return b.String()
		} else if params.Len() == 1 {
			// Some[T](value T) signature
			var b strings.Builder
			b.WriteString("```go\n")
			b.WriteString(fmt.Sprintf("func Some[%s](value %s) Option[%s]\n", tStr, tStr, tStr))
			b.WriteString("```\n\n")
			b.WriteString("**Creates an Option containing a value**\n\n")
			b.WriteString("| Method | Returns |\n")
			b.WriteString("|--------|--------|\n")
			b.WriteString("| `.IsSome()` | `true` |\n")
			b.WriteString("| `.IsNone()` | `false` |\n")
			b.WriteString(fmt.Sprintf("| `.MustSome()` | `%s` |\n", tStr))
			b.WriteString(fmt.Sprintf("| `.SomeOr(default)` | `%s` |\n", tStr))
			return b.String()
		}

	case "Result":
		tStr := "T"
		eStr := "E"
		if len(dgoInfo.TypeArgs) > 0 {
			tStr = formatType(dgoInfo.TypeArgs[0], pkg)
		}
		if len(dgoInfo.TypeArgs) > 1 {
			eStr = formatType(dgoInfo.TypeArgs[1], pkg)
		}

		if params.Len() == 1 {
			paramType := formatType(params.At(0).Type(), pkg)
			// Determine if this is Ok or Err based on parameter type
			if paramType == tStr {
				// Ok[T, E](value T) signature
				var b strings.Builder
				b.WriteString("```go\n")
				b.WriteString(fmt.Sprintf("func Ok[%s, %s](value %s) Result[%s, %s]\n", tStr, eStr, tStr, tStr, eStr))
				b.WriteString("```\n\n")
				b.WriteString("**Creates a successful Result**\n\n")
				b.WriteString("| Method | Returns |\n")
				b.WriteString("|--------|--------|\n")
				b.WriteString("| `.IsOk()` | `true` |\n")
				b.WriteString("| `.IsErr()` | `false` |\n")
				b.WriteString(fmt.Sprintf("| `.MustOk()` | `%s` |\n", tStr))
				b.WriteString("| `.MustErr()` | *panics* |\n")
				b.WriteString(fmt.Sprintf("| `.OkOr(default)` | `%s` |\n", tStr))
				return b.String()
			} else if paramType == eStr {
				// Err[T, E](err E) signature
				var b strings.Builder
				b.WriteString("```go\n")
				b.WriteString(fmt.Sprintf("func Err[%s, %s](err %s) Result[%s, %s]\n", tStr, eStr, eStr, tStr, eStr))
				b.WriteString("```\n\n")
				b.WriteString("**Creates a failed Result**\n\n")
				b.WriteString("| Method | Returns |\n")
				b.WriteString("|--------|--------|\n")
				b.WriteString("| `.IsOk()` | `false` |\n")
				b.WriteString("| `.IsErr()` | `true` |\n")
				b.WriteString("| `.MustOk()` | *panics* |\n")
				b.WriteString(fmt.Sprintf("| `.MustErr()` | `%s` |\n", eStr))
				b.WriteString("| `.OkOr(default)` | `default` |\n")
				return b.String()
			}
		}
	}

	return ""
}

// dingoEnumInfo holds information about a Dingo enum type
type dingoEnumInfo struct {
	Name     string   // Enum name (e.g., "Event")
	Variants []string // Variant names (e.g., ["UserCreated", "UserDeleted", ...])
}

// dingoVariantInfo holds information about a Dingo enum variant
type dingoVariantInfo struct {
	EnumName    string            // Parent enum name (e.g., "Event")
	VariantName string            // Variant name without prefix (e.g., "UserCreated")
	Fields      []dingoFieldInfo  // Variant fields
}

// dingoFieldInfo holds information about a variant field
type dingoFieldInfo struct {
	Name string
	Type string
}

// detectDingoEnumType checks if a type is a Dingo enum (interface with is<Name>() marker)
func detectDingoEnumType(t types.Type, pkg *types.Package) *dingoEnumInfo {
	if t == nil {
		return nil
	}

	// Get the named type
	named, ok := t.(*types.Named)
	if !ok {
		return nil
	}

	// Get the underlying interface
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		return nil
	}

	// Check for the marker method pattern: is<EnumName>()
	// Dingo enums have exactly one method: is<Name>()
	if iface.NumMethods() != 1 {
		return nil
	}

	method := iface.Method(0)
	methodName := method.Name()

	// Check if method name matches pattern "is<Name>"
	enumName := named.Obj().Name()
	expectedMethodName := "is" + enumName
	if methodName != expectedMethodName {
		return nil
	}

	// Verify method signature: no params, no returns
	sig := method.Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
		return nil
	}

	// Find all variants by scanning the package for structs implementing this interface
	variants := findEnumVariants(enumName, pkg)

	return &dingoEnumInfo{
		Name:     enumName,
		Variants: variants,
	}
}

// findEnumVariants scans the package for structs that implement the enum interface
func findEnumVariants(enumName string, pkg *types.Package) []string {
	if pkg == nil {
		return nil
	}

	var variants []string
	markerMethod := "is" + enumName

	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}

		// Check if it's a struct with the expected prefix
		if !strings.HasPrefix(name, enumName) || name == enumName {
			continue
		}

		// Must be a named type with a struct underlying
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, ok := named.Underlying().(*types.Struct); !ok {
			continue
		}

		// Check if it has the marker method
		for i := 0; i < named.NumMethods(); i++ {
			method := named.Method(i)
			if method.Name() == markerMethod {
				// Verify signature: no params, no returns
				sig := method.Type().(*types.Signature)
				if sig.Params().Len() == 0 && sig.Results().Len() == 0 {
					// Extract variant name by stripping enum prefix
					variantName := name[len(enumName):]
					if variantName != "" {
						variants = append(variants, variantName)
					}
					break
				}
			}
		}
	}

	// Sort variants for consistent display
	sort.Strings(variants)
	return variants
}

// formatDingoEnumHover formats hover for a Dingo enum type
func formatDingoEnumHover(info *dingoEnumInfo) string {
	var b strings.Builder

	// Dingo-style enum declaration with variants
	b.WriteString("```dingo\n")
	if len(info.Variants) > 0 {
		b.WriteString(fmt.Sprintf("enum %s {\n", info.Name))
		for _, variant := range info.Variants {
			b.WriteString(fmt.Sprintf("    %s\n", variant))
		}
		b.WriteString("}\n")
	} else {
		b.WriteString(fmt.Sprintf("enum %s\n", info.Name))
	}
	b.WriteString("```\n\n")

	// Description
	b.WriteString("**Sum type** (tagged union)\n\n")

	// Usage hint
	b.WriteString("Use `match` for exhaustive pattern matching")

	return b.String()
}

// detectDingoVariantType checks if a type is a Dingo enum variant
// Variants are structs with names like <Enum><Variant> that have a method is<Enum>()
func detectDingoVariantType(t types.Type, pkg *types.Package) *dingoVariantInfo {
	if t == nil {
		return nil
	}

	// Get the named type
	named, ok := t.(*types.Named)
	if !ok {
		return nil
	}

	// Must be a struct
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	// Check if it has a marker method is<Something>()
	// This indicates it's an enum variant
	var enumName string
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if strings.HasPrefix(method.Name(), "is") && len(method.Name()) > 2 {
			// Verify method signature: no params, no returns
			sig := method.Type().(*types.Signature)
			if sig.Params().Len() == 0 && sig.Results().Len() == 0 {
				enumName = method.Name()[2:] // Strip "is" prefix
				break
			}
		}
	}

	if enumName == "" {
		return nil
	}

	// Variant name is the type name with enum prefix stripped
	typeName := named.Obj().Name()
	if !strings.HasPrefix(typeName, enumName) {
		return nil
	}
	variantName := typeName[len(enumName):]
	if variantName == "" {
		return nil
	}

	// Collect fields
	var fields []dingoFieldInfo
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		fields = append(fields, dingoFieldInfo{
			Name: field.Name(),
			Type: formatType(field.Type(), pkg),
		})
	}

	return &dingoVariantInfo{
		EnumName:    enumName,
		VariantName: variantName,
		Fields:      fields,
	}
}

// formatDingoVariantHover formats hover for a Dingo enum variant
func formatDingoVariantHover(info *dingoVariantInfo) string {
	var b strings.Builder

	// Dingo-style variant declaration
	b.WriteString("```dingo\n")
	b.WriteString(fmt.Sprintf("%s.%s", info.EnumName, info.VariantName))
	if len(info.Fields) > 0 {
		b.WriteString(" { ")
		for i, f := range info.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s: %s", f.Name, f.Type))
		}
		b.WriteString(" }")
	}
	b.WriteString("\n```\n\n")

	// Description
	b.WriteString(fmt.Sprintf("Variant of `enum %s`", info.EnumName))

	return b.String()
}
