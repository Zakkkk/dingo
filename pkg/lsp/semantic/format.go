package semantic

import (
	"fmt"
	"go/types"
	"strings"

	"go.lsp.dev/protocol"
)

// FormatHover creates LSP hover response from semantic entity
func FormatHover(entity *SemanticEntity, pkg *types.Package) *protocol.Hover {
	if entity == nil {
		return nil
	}

	var content string

	// Handle operators separately
	if entity.Kind == KindOperator && entity.Context != nil {
		content = formatOperatorHover(entity, pkg)
	} else if entity.Context != nil && entity.Context.Kind == ContextErrorProp {
		// Error propagation context on a variable
		content = formatErrorPropHover(entity, pkg)
	} else if entity.Object != nil {
		// Named entity (variable, function, etc.)
		content = formatObjectHover(entity, pkg)
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
			b.WriteString(fmt.Sprintf("Unwraps `%s` to `%s`\n\n",
				formatType(ctx.OriginalType, pkg),
				formatType(ctx.UnwrappedType, pkg)))
		}
		b.WriteString("Returns early with error if result is `Err`")

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

// formatObjectHover formats hover for named objects (vars, funcs, etc.)
func formatObjectHover(entity *SemanticEntity, pkg *types.Package) string {
	var b strings.Builder

	b.WriteString("```go\n")
	b.WriteString(formatSignature(entity.Object, pkg))
	b.WriteString("\n```")

	// Add context description if available
	if entity.Context != nil && entity.Context.Description != "" {
		b.WriteString("\n\n")
		b.WriteString(entity.Context.Description)
	}

	return b.String()
}

// formatTypeHover formats hover for expressions without objects
func formatTypeHover(entity *SemanticEntity, pkg *types.Package) string {
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
		return fmt.Sprintf("type %s %s", obj.Name(), formatType(obj.Type().Underlying(), pkg))

	default:
		return obj.String()
	}
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
		// Same package - no qualifier needed
		if pkg != nil && other == pkg {
			return ""
		}
		// Different package - use package name
		return other.Name()
	}

	return types.TypeString(t, qualifier)
}
