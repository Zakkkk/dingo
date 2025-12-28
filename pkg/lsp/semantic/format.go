package semantic

import (
	"fmt"
	"go/types"
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
			b.WriteString("Returns early if error is non-nil")
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

	b.WriteString("```go\n")
	b.WriteString("var err error")
	b.WriteString("\n```\n\n")
	b.WriteString("*Lambda parameter for error transformation*")

	return b.String()
}

// formatObjectHover formats hover for named objects (vars, funcs, etc.)
func formatObjectHover(entity *SemanticEntity, pkg *types.Package) string {
	return formatObjectHoverWithDocs(entity, pkg, nil)
}

// formatObjectHoverWithDocs formats hover with optional documentation.
func formatObjectHoverWithDocs(entity *SemanticEntity, pkg *types.Package, docProvider *DocProvider) string {
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
		// Check for dgo types (Result, Option)
		if dgoInfo := detectDgoType(obj.Type()); dgoInfo != nil {
			switch dgoInfo.TypeName {
			case "Result":
				return formatDgoResultHover(dgoInfo, pkg)
			case "Option":
				return formatDgoOptionHover(dgoInfo, pkg)
			}
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
