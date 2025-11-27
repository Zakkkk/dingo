// Package builtin provides immutability checking plugin for let-declared variables
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// letMarkerPattern matches dingo:let markers with optional trailing comment
// Group 1: variable names (comma-separated, no spaces after colon)
// Group 2: optional trailing comment after variable names
var letMarkerPattern = regexp.MustCompile(`// dingo:let:([a-zA-Z0-9_,]+)(?:\s+(.*))?$`)

// ImmutabilityPlugin detects reassignment to let-declared variables
// and reports compile-time errors for violations.
//
// This plugin enforces Rust-like immutability semantics:
// - Direct reassignment: x = value (error)
// - Compound assignment: x += 1 (error)
// - Increment/decrement: x++, x-- (error)
// - Pointer dereference: *x = value (error)
// - Closure reassignment: func() { x = 10 }() (error)
//
// Interior mutability is explicitly allowed:
// - Field assignment: x.field = value (ok)
// - Index assignment: x[0] = value (ok)
// - Map assignment: x["key"] = value (ok)
type ImmutabilityPlugin struct {
	ctx           *plugin.Context
	immutableVars map[string]token.Pos // varName -> declaration position
	typeInfo      *types.Info          // For pointer type resolution
}

// NewImmutabilityPlugin creates a new immutability checking plugin
func NewImmutabilityPlugin() *ImmutabilityPlugin {
	return &ImmutabilityPlugin{
		immutableVars: make(map[string]token.Pos),
	}
}

// Name returns the plugin name
func (p *ImmutabilityPlugin) Name() string {
	return "immutability"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *ImmutabilityPlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	// Get type info from context if available for precise pointer detection
	if ctx != nil && ctx.TypeInfo != nil {
		if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
			p.typeInfo = typesInfo
		}
	}
}

// Process builds immutable variable set and checks for violations
func (p *ImmutabilityPlugin) Process(node ast.Node) error {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	// Phase 1: Collect immutable variables from markers
	p.collectImmutableVars(file)

	// Phase 2: Check for violations
	p.checkViolations(file)

	return nil
}

// collectImmutableVars scans comments for dingo:let markers and removes them
func (p *ImmutabilityPlugin) collectImmutableVars(file *ast.File) {
	var cleanedComments []*ast.CommentGroup

	for _, cg := range file.Comments {
		var cleanedList []*ast.Comment

		for _, c := range cg.List {
			if matches := letMarkerPattern.FindStringSubmatch(c.Text); matches != nil {
				// matches[1] = variable names (e.g., "x" or "a,b,c")
				// matches[2] = optional trailing comment (empty if none)
				vars := strings.Split(matches[1], ",")
				for _, v := range vars {
					v = strings.TrimSpace(v)
					if v != "" {
						p.immutableVars[v] = c.Pos()
					}
				}

				// Check if there's additional comment text after the marker
				// matches[2] contains any text after the variable names
				if len(matches) > 2 && matches[2] != "" {
					trailingComment := strings.TrimSpace(matches[2])
					if trailingComment != "" {
						// Create new comment with remaining text
						cleanedComment := &ast.Comment{
							Slash: c.Slash,
							Text:  "// " + trailingComment,
						}
						cleanedList = append(cleanedList, cleanedComment)
					}
				}
				// If no additional text, skip this comment entirely (marker-only)
			} else {
				// Not a marker comment, keep it as-is
				cleanedList = append(cleanedList, c)
			}
		}

		// Only keep comment groups that have remaining comments
		if len(cleanedList) > 0 {
			cleanedGroup := &ast.CommentGroup{List: cleanedList}
			cleanedComments = append(cleanedComments, cleanedGroup)
		}
	}

	// Replace file's comments with cleaned version
	file.Comments = cleanedComments
}

// checkViolations walks the AST to find reassignments to immutable variables
func (p *ImmutabilityPlugin) checkViolations(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			// Skip := (variable declarations)
			// Only check = and compound assignments (+=, -=, etc.)
			if stmt.Tok != token.DEFINE {
				p.checkAssignment(stmt)
			}
		case *ast.IncDecStmt:
			// Check ++ and -- operators
			p.checkIncDec(stmt)
		}
		return true
	})
}

// checkAssignment checks an assignment statement for violations
func (p *ImmutabilityPlugin) checkAssignment(stmt *ast.AssignStmt) {
	for _, lhs := range stmt.Lhs {
		// Case 1: Direct variable assignment (x = value)
		if ident, ok := lhs.(*ast.Ident); ok {
			if declPos, exists := p.immutableVars[ident.Name]; exists {
				operation := "reassignment"
				if stmt.Tok != token.ASSIGN {
					operation = fmt.Sprintf("compound assignment (%s)", stmt.Tok.String())
				}
				p.reportError(ident.Name, operation, stmt.TokPos, declPos)
			}
			continue
		}

		// Case 2: Pointer dereference (*x = value)
		if star, ok := lhs.(*ast.StarExpr); ok {
			p.checkPointerDeref(star, stmt.TokPos)
			continue
		}

		// Case 3: Field/Index - ALLOWED (interior mutability)
		// SelectorExpr (x.field) and IndexExpr (x[i]) are explicitly permitted
		// These are allowed because they mutate the contents, not the binding itself
	}
}

// checkPointerDeref checks if a pointer dereference violates immutability
func (p *ImmutabilityPlugin) checkPointerDeref(star *ast.StarExpr, pos token.Pos) {
	// Extract the base identifier from the star expression
	// *x -> check if x is immutable
	// *(*x) -> walk down to find base identifier
	baseIdent := p.extractBaseIdent(star.X)
	if baseIdent == nil {
		return
	}

	// Check if base variable is immutable
	if declPos, exists := p.immutableVars[baseIdent.Name]; exists {
		// Additional check: Is this actually a pointer type?
		// Use go/types if available for precise checking
		if p.isPointerType(star.X) {
			p.reportError(baseIdent.Name, "pointer dereference mutation", pos, declPos)
		}
	}
}

// extractBaseIdent recursively extracts the base identifier from an expression
// *x -> x
// *(*x) -> x
// *(parens) -> extract from parens
func (p *ImmutabilityPlugin) extractBaseIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.ParenExpr:
		return p.extractBaseIdent(e.X)
	case *ast.StarExpr:
		return p.extractBaseIdent(e.X)
	default:
		return nil
	}
}

// isPointerType checks if an expression has a pointer type
func (p *ImmutabilityPlugin) isPointerType(expr ast.Expr) bool {
	// If we have type info, use it for precise checking
	if p.typeInfo != nil && p.typeInfo.Types != nil {
		if tv, ok := p.typeInfo.Types[expr]; ok {
			_, isPtr := tv.Type.Underlying().(*types.Pointer)
			return isPtr
		}
	}

	// Fallback: Assume pointer (conservative - may have false positives)
	// This is acceptable because *x = value on non-pointer won't compile anyway
	// The Go compiler will catch the error if our check is overly conservative
	return true
}

// checkIncDec checks increment/decrement statements for violations
func (p *ImmutabilityPlugin) checkIncDec(stmt *ast.IncDecStmt) {
	if ident, ok := stmt.X.(*ast.Ident); ok {
		if declPos, exists := p.immutableVars[ident.Name]; exists {
			op := "++"
			if stmt.Tok == token.DEC {
				op = "--"
			}
			p.reportError(ident.Name, op, stmt.TokPos, declPos)
		}
	}
}

// reportError reports an immutability violation error
func (p *ImmutabilityPlugin) reportError(varName, operation string, pos, declPos token.Pos) {
	msg := fmt.Sprintf("cannot modify immutable variable '%s' (%s): declared with 'let'",
		varName, operation)
	p.ctx.ReportError(msg, pos)
}

// Transform is no-op - this plugin only reports errors
func (p *ImmutabilityPlugin) Transform(node ast.Node) (ast.Node, error) {
	return node, nil
}

// GetPendingDeclarations returns empty - no declarations to inject
func (p *ImmutabilityPlugin) GetPendingDeclarations() []ast.Decl {
	return nil
}

// ClearPendingDeclarations is no-op - no declarations to clear
func (p *ImmutabilityPlugin) ClearPendingDeclarations() {
}
