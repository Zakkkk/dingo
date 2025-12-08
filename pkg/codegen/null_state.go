package codegen

import "github.com/MadAppGang/dingo/pkg/ast"

// SafeNavPath represents a safe navigation chain like "env?.Region?.Name"
// Used for null-state inference to optimize redundant nil checks in ternary expressions.
//
// Example: For `env?.Region?.Name`, the path is:
//   - Base: "env"
//   - Sels: ["Region", "Name"]
type SafeNavPath struct {
	Base string   // Base identifier (e.g., "env")
	Sels []string // Selector chain (e.g., ["Region", "Name"])
}

// ExtractSafeNavPath extracts the path from a SafeNavExpr.
// Returns nil if the expression is nil or cannot be converted to a path.
//
// Example:
//
//	env?.Region?.Name → SafeNavPath{Base: "env", Sels: ["Region", "Name"]}
func ExtractSafeNavPath(e *ast.SafeNavExpr) *SafeNavPath {
	if e == nil {
		return nil
	}

	path := &SafeNavPath{Sels: []string{e.Sel.Name}}
	current := e.X

	for {
		switch x := current.(type) {
		case *ast.SafeNavExpr:
			// Prepend to maintain order: a?.b?.c → Base="a", Sels=["b", "c"]
			path.Sels = append([]string{x.Sel.Name}, path.Sels...)
			current = x.X
		case *ast.DingoIdent:
			path.Base = x.Name
			return path
		case *ast.RawExpr:
			path.Base = x.Text
			return path
		default:
			// Fallback for complex bases
			if stringer, ok := current.(interface{ String() string }); ok {
				path.Base = stringer.String()
			}
			return path
		}
	}
}

// Equals checks if two paths are structurally identical.
// Used to match safe-nav expressions in condition and true branch.
func (p *SafeNavPath) Equals(other *SafeNavPath) bool {
	if p == nil || other == nil {
		return false
	}
	if p.Base != other.Base || len(p.Sels) != len(other.Sels) {
		return false
	}
	for i := range p.Sels {
		if p.Sels[i] != other.Sels[i] {
			return false
		}
	}
	return true
}

// ToDerefExpr generates "*base.sel1.sel2" for direct dereference access.
// Used when we've proven the path is non-nil and want to dereference the final pointer.
//
// Example:
//
//	SafeNavPath{Base: "env", Sels: ["Region"]} → "*env.Region"
func (p *SafeNavPath) ToDerefExpr() string {
	result := "*" + p.Base
	for _, sel := range p.Sels {
		result += "." + sel
	}
	return result
}

// ToDirectExpr generates "base.sel1.sel2" without dereference.
// Used when we've proven the path is non-nil but don't need to dereference.
//
// Example:
//
//	SafeNavPath{Base: "env", Sels: ["Region"]} → "env.Region"
func (p *SafeNavPath) ToDirectExpr() string {
	result := p.Base
	for _, sel := range p.Sels {
		result += "." + sel
	}
	return result
}

// isPositiveLenComparison checks if op+right indicates a positive length check.
// Returns true for patterns like: > 0, >= 1, != 0
func isPositiveLenComparison(op string, right ast.Expr) bool {
	rightText := ""
	switch r := right.(type) {
	case *ast.RawExpr:
		rightText = r.Text
	case *ast.DingoIdent:
		rightText = r.Name
	}

	switch op {
	case ">":
		return rightText == "0"
	case ">=":
		return rightText == "1"
	case "!=":
		return rightText == "0"
	}
	return false
}

// DetectLenSafeNavPattern checks if an expression is `len(x?.y) > 0` pattern.
// Returns the SafeNavPath if pattern matches, nil otherwise.
//
// This enables null-state inference: if len(x?.y) > 0, then x?.y is proven non-nil.
func DetectLenSafeNavPattern(cond ast.Expr) *SafeNavPath {
	// Must be BinaryExpr
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok {
		return nil
	}

	// Check for positive comparison: >, >=, !=
	if !isPositiveLenComparison(binary.Op, binary.Y) {
		return nil
	}

	// Left side must be BuiltinCallExpr(len or cap)
	builtin, ok := binary.X.(*ast.BuiltinCallExpr)
	if !ok || (builtin.Func != "len" && builtin.Func != "cap") {
		return nil
	}

	// Single argument must be SafeNavExpr
	if len(builtin.Args) != 1 {
		return nil
	}
	safeNav, ok := builtin.Args[0].(*ast.SafeNavExpr)
	if !ok {
		return nil
	}

	return ExtractSafeNavPath(safeNav)
}

// MatchesSafeNavPath checks if an expression is a SafeNavExpr matching the given path.
// Used to determine if the true branch of a ternary matches the condition's safe-nav.
func MatchesSafeNavPath(expr ast.Expr, pattern *SafeNavPath) bool {
	if pattern == nil {
		return false
	}

	safeNav, ok := expr.(*ast.SafeNavExpr)
	if !ok {
		return false
	}

	exprPath := ExtractSafeNavPath(safeNav)
	return pattern.Equals(exprPath)
}
