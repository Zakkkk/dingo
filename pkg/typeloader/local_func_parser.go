package typeloader

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// LocalFuncParser extracts function signatures from Dingo source files
type LocalFuncParser struct{}

// ParseLocalFunctions extracts function declarations from Dingo source
// Handles Dingo-specific syntax by pre-processing before Go parser
func (p *LocalFuncParser) ParseLocalFunctions(source []byte) (map[string]*FunctionSignature, error) {
	// Validate input size to prevent ReDoS attacks
	if len(source) > 1_000_000 { // 1MB limit
		return nil, fmt.Errorf("source file too large for regex processing (>1MB): %d bytes", len(source))
	}

	// Step 1: Normalize Dingo syntax to valid Go for function signatures
	// This handles: type annotations (param: Type -> param Type), etc.
	normalized := p.normalizeFuncDecls(source)

	// Step 2: Try parsing with Go parser
	// Use SkipObjectResolution to be lenient with undefined types
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", normalized, parser.SkipObjectResolution)

	// Step 3: If Go parser fails (due to Dingo syntax in bodies), use regex fallback
	if err != nil {
		return p.extractFuncsRegex(source)
	}

	// Step 4: Extract function signatures from AST
	result := make(map[string]*FunctionSignature)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			// Include all functions (exported and unexported)
			// They can all be called locally
			if fn.Name != nil {
				sig := p.astFuncToSignature(fn)
				if sig != nil {
					result[fn.Name.Name] = sig
				}
			}
		}
	}

	return result, nil
}

// normalizeFuncDecls converts Dingo function declarations to valid Go
// Normalizes both declaration lines and bodies to make them parseable by go/parser
func (p *LocalFuncParser) normalizeFuncDecls(source []byte) []byte {
	// Step 1: Replace '?' with space (error propagation operator)
	// This makes Dingo source parseable while preserving byte positions
	sanitized := make([]byte, len(source))
	copy(sanitized, source)
	for i := 0; i < len(sanitized); i++ {
		if sanitized[i] == '?' {
			sanitized[i] = ' '
		}
	}

	// Step 2: Convert Dingo parameter syntax to Go syntax
	// Pattern: func name(param: Type) ReturnType
	// Convert to: func name(param Type) ReturnType

	// Regex to find parameter type annotations within function signatures
	// This handles "param: Type" -> "param Type"
	typeAnnotPattern := regexp.MustCompile(`\b(\w+):\s+([^,)\s]+)`)

	// Process line by line to avoid modifying non-function-signature colons
	lines := bytes.Split(sanitized, []byte("\n"))
	for i, line := range lines {
		lineStr := string(line)
		// Only process lines that look like function declarations
		if strings.Contains(lineStr, "func ") {
			// Replace "param: Type" with "param Type" in this line
			normalized := typeAnnotPattern.ReplaceAllString(lineStr, "$1 $2")
			lines[i] = []byte(normalized)
		}
	}

	return bytes.Join(lines, []byte("\n"))
}

// astFuncToSignature converts an AST FuncDecl to FunctionSignature
func (p *LocalFuncParser) astFuncToSignature(fn *ast.FuncDecl) *FunctionSignature {
	sig := &FunctionSignature{
		Name: fn.Name.Name,
	}

	// Extract return types
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			ref := p.astTypeToRef(field.Type)
			// Handle multiple names on same type
			if len(field.Names) == 0 {
				sig.Results = append(sig.Results, ref)
			} else {
				for range field.Names {
					sig.Results = append(sig.Results, ref)
				}
			}
		}
	}

	// Extract parameters (less critical but useful for validation)
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			ref := p.astTypeToRef(field.Type)
			if len(field.Names) == 0 {
				sig.Parameters = append(sig.Parameters, ref)
			} else {
				for range field.Names {
					sig.Parameters = append(sig.Parameters, ref)
				}
			}
		}
	}

	// Extract receiver if method
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := fn.Recv.List[0]
		recvRef := p.astTypeToRef(recv.Type)
		sig.Receiver = &recvRef
	}

	return sig
}

// astTypeToRef converts AST type expression to TypeRef
func (p *LocalFuncParser) astTypeToRef(expr ast.Expr) TypeRef {
	ref := TypeRef{}

	switch t := expr.(type) {
	case *ast.Ident:
		ref.Name = t.Name
		ref.IsError = t.Name == "error"

	case *ast.StarExpr:
		inner := p.astTypeToRef(t.X)
		inner.IsPointer = true
		return inner

	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			ref.Package = pkg.Name
		}
		ref.Name = t.Sel.Name

	default:
		// For complex types, use a string representation
		ref.Name = formatType(expr)
	}

	return ref
}

// extractFuncsRegex is fallback when Go parser fails on Dingo syntax
func (p *LocalFuncParser) extractFuncsRegex(source []byte) (map[string]*FunctionSignature, error) {
	result := make(map[string]*FunctionSignature)

	// Pattern to match function declarations with return types
	// Captures everything between ) and { as the return type section
	// This handles: func name(params) ReturnType {
	//               func name(params) (Type1, error) {
	//               func name(params) Result[T, E] {
	pattern := regexp.MustCompile(`(?m)^func\s+(\w+)\s*\(([^)]*)\)\s*([^{]*)\{`)

	matches := pattern.FindAllSubmatch(source, -1)
	for _, match := range matches {
		name := string(match[1])
		sig := &FunctionSignature{Name: name}

		// Parse parameters (match[2])
		if len(match) > 2 && len(match[2]) > 0 {
			params := p.parseParamsRegex(string(match[2]))
			sig.Parameters = params
		}

		// Parse return types (match[3])
		if len(match) > 3 && len(match[3]) > 0 {
			returnSection := strings.TrimSpace(string(match[3]))
			if returnSection != "" {
				sig.Results = p.parseReturnTypes(returnSection)
			}
		}

		result[name] = sig
	}

	return result, nil
}

// parseReturnTypes parses the return type section from a function signature
// Handles: "error", "Result[T, E]", "(Type1, error)", etc.
func (p *LocalFuncParser) parseReturnTypes(returnSection string) []TypeRef {
	returnSection = strings.TrimSpace(returnSection)
	if returnSection == "" {
		return nil
	}

	var results []TypeRef

	// Check if it's a tuple return: (Type1, Type2)
	if strings.HasPrefix(returnSection, "(") && strings.HasSuffix(returnSection, ")") {
		// Remove parentheses and split by comma (but not commas inside brackets)
		inner := returnSection[1 : len(returnSection)-1]
		types := splitByCommaRespectingBrackets(inner)
		for _, t := range types {
			t = strings.TrimSpace(t)
			if t != "" {
				ref := TypeRef{Name: t}
				if t == "error" {
					ref.IsError = true
				}
				results = append(results, ref)
			}
		}
	} else {
		// Single return type: Type, error, Result[T, E], etc.
		ref := TypeRef{Name: returnSection}
		if returnSection == "error" {
			ref.IsError = true
		}
		results = append(results, ref)
	}

	return results
}

// splitByCommaRespectingBrackets splits a string by comma, but ignores commas inside brackets
func splitByCommaRespectingBrackets(s string) []string {
	var result []string
	var current strings.Builder
	bracketDepth := 0

	for _, ch := range s {
		switch ch {
		case '[':
			bracketDepth++
			current.WriteRune(ch)
		case ']':
			bracketDepth--
			current.WriteRune(ch)
		case ',':
			if bracketDepth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Don't forget the last part
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// parseParamsRegex extracts parameter types from a parameter string
func (p *LocalFuncParser) parseParamsRegex(params string) []TypeRef {
	if strings.TrimSpace(params) == "" {
		return nil
	}

	var result []TypeRef

	// Handle both Dingo syntax (name: Type) and Go syntax (name Type)
	// Split by comma to get individual parameters
	parts := strings.Split(params, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Try to extract type (handle both "x: int" and "x int")
		typePattern := regexp.MustCompile(`\w+[:\s]+(\S+)`)
		if matches := typePattern.FindStringSubmatch(part); len(matches) > 1 {
			typeName := strings.TrimSpace(matches[1])
			// Remove trailing colon if present (from Dingo syntax)
			typeName = strings.TrimSuffix(typeName, ":")
			result = append(result, TypeRef{Name: typeName})
		}
	}

	return result
}

// formatType converts an AST type to string representation
func formatType(expr ast.Expr) string {
	// Simplified - could use go/printer for complete implementation
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + formatType(t.Elt)
		}
		return fmt.Sprintf("[%s]%s", formatType(t.Len), formatType(t.Elt))
	case *ast.MapType:
		return "map[" + formatType(t.Key) + "]" + formatType(t.Value)
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.IndexExpr:
		// Generic type with single type param: Result[T]
		return formatType(t.X) + "[" + formatType(t.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple type params: Result[T, E]
		result := formatType(t.X) + "["
		for i, idx := range t.Indices {
			if i > 0 {
				result += ", "
			}
			result += formatType(idx)
		}
		result += "]"
		return result
	default:
		return "unknown"
	}
}
