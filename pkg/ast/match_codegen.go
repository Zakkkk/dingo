package ast

import (
	"bytes"
	"fmt"
	"go/token"
	"strings"
)

// MatchCodeGen generates Go code from MatchExpr AST nodes.
// Transforms match expressions to inline type switches (NO IIFE).
type MatchCodeGen struct {
	buf      bytes.Buffer
	mappings *MappingBuilder
	enumReg  map[string]string // Variant name → Enum name mapping
}

// NewMatchCodeGen creates a new match code generator.
func NewMatchCodeGen(enumRegistry map[string]string) *MatchCodeGen {
	return &MatchCodeGen{
		mappings: NewMappingBuilder(),
		enumReg:  enumRegistry,
	}
}

// Generate produces Go code for a MatchExpr.
// Returns the generated Go code and source mappings.
//
// Transformation:
//   match result {
//       Ok(x) => x * 2,
//       Err(e) => 0,
//   }
//
// →
//   var matchResult int
//   switch v := result.(type) {
//   case ResultOk:
//       x := v.ok
//       matchResult = x * 2
//   case ResultErr:
//       e := v.err
//       matchResult = 0
//   default:
//       panic("non-exhaustive match")
//   }
func (g *MatchCodeGen) Generate(expr *MatchExpr) CodeGenResult {
	g.buf.Reset()
	g.mappings.Reset()

	dingoStart := int(expr.Pos())
	dingoEnd := int(expr.End())
	goStart := 0

	// Result variable name
	resultVar := fmt.Sprintf("matchResult%d", expr.MatchID)

	// 1. Generate result variable declaration (var matchResultN T)
	// Type inference needed - for now use empty interface
	varDecl := fmt.Sprintf("var %s interface{}\n", resultVar)
	g.buf.WriteString(varDecl)

	// 2. Generate type switch
	scrutineeStr := expr.Scrutinee.String()
	switchHeader := fmt.Sprintf("switch v := %s.(type) {\n", scrutineeStr)
	g.buf.WriteString(switchHeader)

	// 3. Generate case arms
	for _, arm := range expr.Arms {
		g.generateArm(arm, resultVar)
	}

	// 4. Generate default case (exhaustiveness check)
	defaultCase := "default:\n\tpanic(\"non-exhaustive match\")\n"
	g.buf.WriteString(defaultCase)

	// 5. Close switch
	g.buf.WriteString("}\n")

	// Create mapping for entire match expression
	goEnd := g.buf.Len()
	mapping := NewSourceMapping(dingoStart, dingoEnd, goStart, goEnd, "match")

	return CodeGenResult{
		Output:   g.buf.Bytes(),
		Mappings: []SourceMapping{mapping},
	}
}

// generateArm generates code for a single match arm
func (g *MatchCodeGen) generateArm(arm *MatchArm, resultVar string) {
	// Generate case label based on pattern type
	switch p := arm.Pattern.(type) {
	case *ConstructorPattern:
		g.generateConstructorArm(p, arm, resultVar)
	case *TuplePattern:
		g.generateTupleArm(p, arm, resultVar)
	case *WildcardPattern:
		g.generateWildcardArm(arm, resultVar)
	case *LiteralPattern:
		g.generateLiteralArm(p, arm, resultVar)
	case *VariablePattern:
		// Variable pattern matches anything and binds to name
		g.buf.WriteString("default:\n")
		g.buf.WriteString(fmt.Sprintf("\t%s := v\n", p.Name))
		g.generateArmBody(arm, resultVar)
	}
}

// generateConstructorArm generates code for constructor patterns like Ok(x), Err(e)
func (g *MatchCodeGen) generateConstructorArm(p *ConstructorPattern, arm *MatchArm, resultVar string) {
	// Resolve enum type name from variant
	enumName := g.resolveEnumName(p.Name)
	if enumName == "" {
		// Fallback: assume constructor is the full type name
		enumName = ""
	}

	// Generate case label
	caseName := g.getCaseTypeName(p.Name, enumName)
	g.buf.WriteString(fmt.Sprintf("case %s:\n", caseName))

	// Extract bindings from variant
	if len(p.Params) > 0 {
		bindings := p.GetBindings()
		for _, binding := range bindings {
			fieldName := g.getFieldName(p, binding)
			g.buf.WriteString(fmt.Sprintf("\t%s := v.%s\n", binding.Name, fieldName))
		}
	}

	// Generate guard if present
	if arm.Guard != nil {
		g.buf.WriteString(fmt.Sprintf("\tif %s {\n", arm.Guard.String()))
		g.buf.WriteString(fmt.Sprintf("\t\t%s = %s\n", resultVar, arm.Body.String()))
		g.buf.WriteString("\t} else {\n")
		g.buf.WriteString("\t\tpanic(\"match guard failed\")\n")
		g.buf.WriteString("\t}\n")
	} else {
		// Direct assignment
		g.generateArmBody(arm, resultVar)
	}
}

// generateTupleArm generates code for tuple patterns like (a, b)
func (g *MatchCodeGen) generateTupleArm(p *TuplePattern, arm *MatchArm, resultVar string) {
	// Tuple patterns require type assertion to tuple struct
	// For now, treat as default with binding extraction
	g.buf.WriteString("default:\n")

	// Extract tuple elements
	for i, elem := range p.Elements {
		if varPat, ok := elem.(*VariablePattern); ok {
			g.buf.WriteString(fmt.Sprintf("\t%s := v._%d\n", varPat.Name, i))
		}
	}

	g.generateArmBody(arm, resultVar)
}

// generateWildcardArm generates code for wildcard pattern _
func (g *MatchCodeGen) generateWildcardArm(arm *MatchArm, resultVar string) {
	g.buf.WriteString("default:\n")
	g.generateArmBody(arm, resultVar)
}

// generateLiteralArm generates code for literal patterns like 1, "hello", true
func (g *MatchCodeGen) generateLiteralArm(p *LiteralPattern, arm *MatchArm, resultVar string) {
	// Literal patterns need equality check, not type switch
	// For now, generate case with type assertion
	var typeName string
	switch p.Kind {
	case IntLiteral:
		typeName = "int"
	case FloatLiteral:
		typeName = "float64"
	case StringLiteral:
		typeName = "string"
	case BoolLiteral:
		typeName = "bool"
	}

	g.buf.WriteString(fmt.Sprintf("case %s:\n", typeName))
	g.buf.WriteString(fmt.Sprintf("\tif v == %s {\n", p.Value))
	g.buf.WriteString(fmt.Sprintf("\t\t%s = %s\n", resultVar, arm.Body.String()))
	g.buf.WriteString("\t}\n")
}

// generateArmBody generates the body assignment for a match arm
func (g *MatchCodeGen) generateArmBody(arm *MatchArm, resultVar string) {
	bodyStr := arm.Body.String()
	if arm.IsBlock {
		// Block body - execute statements and assign last expression
		g.buf.WriteString(fmt.Sprintf("\t{\n\t\t%s\n\t}\n", bodyStr))
	} else {
		// Expression body - direct assignment
		g.buf.WriteString(fmt.Sprintf("\t%s = %s\n", resultVar, bodyStr))
	}

	// Preserve trailing comment if present
	if arm.Comment != nil {
		g.buf.WriteString(fmt.Sprintf("\t// %s\n", arm.Comment.Text))
	}
}

// resolveEnumName looks up the enum name for a variant constructor
func (g *MatchCodeGen) resolveEnumName(variantName string) string {
	if g.enumReg == nil {
		return ""
	}
	return g.enumReg[variantName]
}

// getCaseTypeName returns the Go type name for a case statement
func (g *MatchCodeGen) getCaseTypeName(variantName, enumName string) string {
	if enumName != "" {
		return enumName + variantName
	}
	// Fallback: use variant name as-is (might be qualified like ResultOk)
	return variantName
}

// getFieldName determines the field name for a binding in a constructor
func (g *MatchCodeGen) getFieldName(p *ConstructorPattern, binding Binding) string {
	// Simple case: single parameter uses lowercase variant name
	if len(p.Params) == 1 && len(binding.Path) == 1 && binding.Path[0] == 0 {
		return strings.ToLower(p.Name)
	}

	// Multiple parameters or nested: use path-based naming
	if len(binding.Path) > 0 {
		idx := binding.Path[0]
		if idx == 0 {
			return strings.ToLower(p.Name)
		}
		return fmt.Sprintf("%s%d", strings.ToLower(p.Name), idx)
	}

	// Fallback
	return binding.Name
}

// FindMatchExpressions finds all positions of 'match' keyword in source.
// Returns byte positions where match expressions start.
func FindMatchExpressions(src []byte) []int {
	var positions []int
	keyword := []byte("match ")

	for i := 0; i < len(src); {
		idx := bytes.Index(src[i:], keyword)
		if idx == -1 {
			break
		}

		pos := i + idx

		// Check if 'match' is a standalone keyword (not part of identifier)
		if pos > 0 {
			prev := src[pos-1]
			if isIdentChar(prev) {
				i = pos + 1
				continue
			}
		}

		// Check character after 'match '
		endPos := pos + len(keyword)
		if endPos < len(src) && isIdentStart(src[endPos]) {
			positions = append(positions, pos)
		}

		i = pos + len(keyword)
	}

	return positions
}

// isIdentStart checks if byte can start Go identifier
func isIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// TransformMatchSource transforms Dingo source containing match expressions to Go.
// This is the main entry point for match expression transformation.
//
// Returns:
// - Transformed source with match expressions replaced by type switches
// - Source mappings for LSP integration
func TransformMatchSource(src []byte, enumRegistry map[string]string) ([]byte, []SourceMapping) {
	matchPositions := FindMatchExpressions(src)
	if len(matchPositions) == 0 {
		return src, nil
	}

	result := make([]byte, 0, len(src)+1000) // Estimate extra space
	var allMappings []SourceMapping
	lastPos := 0

	// Parse and transform each match expression
	parser := NewMatchParser(src, 0)
	codegen := NewMatchCodeGen(enumRegistry)

	for _, matchStart := range matchPositions {
		// Copy source before this match
		result = append(result, src[lastPos:matchStart]...)

		// Parse the match expression
		matchExpr, err := parser.ParseMatch(matchStart)
		if err != nil {
			// Parsing failed, keep original source
			result = append(result, src[matchStart:matchStart+5]...) // "match"
			lastPos = matchStart + 5
			continue
		}

		// Generate Go code
		genResult := codegen.Generate(matchExpr)

		// Adjust mappings to account for already-generated code
		goOffset := len(result)
		for _, mapping := range genResult.Mappings {
			adjusted := mapping
			adjusted.GoStart += goOffset
			adjusted.GoEnd += goOffset
			allMappings = append(allMappings, adjusted)
		}

		result = append(result, genResult.Output...)
		lastPos = int(matchExpr.End())
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, allMappings
}

// NewMatchParser creates a match expression parser.
// Defined here as a placeholder - actual implementation in match_parser.go
func NewMatchParser(src []byte, offset int) *MatchParser {
	return &MatchParser{
		src:    src,
		offset: offset,
		pos:    0,
	}
}

// MatchParser is a minimal placeholder for match expression parsing.
// Full implementation will be in match_parser.go
type MatchParser struct {
	src    []byte
	offset int
	pos    int
}

// ParseMatch is a placeholder that returns an error.
// Real implementation will be in match_parser.go
func (p *MatchParser) ParseMatch(start int) (*MatchExpr, error) {
	// Placeholder: Create a minimal match expression
	return &MatchExpr{
		Match:      token.Pos(start),
		Scrutinee:  &RawExpr{Text: "placeholder"},
		Arms:       []*MatchArm{},
		CloseBrace: token.Pos(start + 5),
		MatchID:    0,
	}, fmt.Errorf("match parser not yet implemented - use match_parser.go")
}
