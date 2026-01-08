package builtin

import (
	"go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// UnusedVarsPlugin adds `_ = varName` statements for unused variables
// This is needed because the transpiler generates code that may have intentionally
// unused variables (e.g., unwrapped Result/Option values in examples)
type UnusedVarsPlugin struct {
	ctx *plugin.Context
}

// NewUnusedVarsPlugin creates a new unused variable handling plugin
func NewUnusedVarsPlugin() *UnusedVarsPlugin {
	return &UnusedVarsPlugin{}
}

// Name returns the plugin name
func (p *UnusedVarsPlugin) Name() string {
	return "unused-vars"
}

// Priority returns the plugin priority (run LAST, after all other transformations)
func (p *UnusedVarsPlugin) Priority() int {
	return 1000
}

// SetContext sets the plugin context
func (p *UnusedVarsPlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx
}

// Process is the discovery phase (no-op for this plugin)
func (p *UnusedVarsPlugin) Process(node ast.Node) error {
	return nil
}

// Transform adds `_ = varName` for unused variables
func (p *UnusedVarsPlugin) Transform(node ast.Node) (ast.Node, error) {
	file, ok := node.(*ast.File)
	if !ok {
		return node, nil
	}
	// Track which variables are declared but not used
	// We'll analyze scopes and add blank assignments where needed

	ast.Inspect(file, func(n ast.Node) bool {
		// Look for if statements with Result/Option unwrapping pattern
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			p.handleIfStatement(ifStmt)
		}
		return true
	})

	return file, nil
}

// handleIfStatement checks if an if statement has the Result/Option unwrapping pattern
// Pattern: if result.IsOk() { v := *result.ok }
func (p *UnusedVarsPlugin) handleIfStatement(ifStmt *ast.IfStmt) {
	if ifStmt.Body == nil || len(ifStmt.Body.List) == 0 {
		return
	}

	// Check if the first statement is an assignment to a variable
	// that looks like it's unwrapping a Result/Option
	firstStmt := ifStmt.Body.List[0]
	assignStmt, ok := firstStmt.(*ast.AssignStmt)
	if !ok || assignStmt.Tok != token.DEFINE {
		return
	}

	// Check if RHS is a dereference of a field like result.ok
	if len(assignStmt.Rhs) != 1 {
		return
	}

	starExpr, ok := assignStmt.Rhs[0].(*ast.StarExpr)
	if !ok {
		return
	}

	selectorExpr, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Check if the selector is for a Result/Option field (ok, err, some)
	fieldName := selectorExpr.Sel.Name
	isResultField := fieldName == "ok" || fieldName == "err"
	isOptionField := fieldName == "some"

	if !isResultField && !isOptionField {
		return
	}

	// This looks like unwrapping - check if the variable is used after declaration
	if len(assignStmt.Lhs) != 1 {
		return
	}

	varIdent, ok := assignStmt.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	varName := varIdent.Name

	// Check if variable is used in subsequent statements
	isUsed := false
	for i := 1; i < len(ifStmt.Body.List); i++ {
		stmt := ifStmt.Body.List[i]
		ast.Inspect(stmt, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == varName {
				isUsed = true
				return false
			}
			return true
		})
		if isUsed {
			break
		}
	}

	// If variable is not used, add `_ = varName` statement
	if !isUsed {
		blankAssign := &ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("_")},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent(varName)},
		}

		// Insert after the variable declaration
		newList := make([]ast.Stmt, 0, len(ifStmt.Body.List)+1)
		newList = append(newList, ifStmt.Body.List[0])     // Original assignment
		newList = append(newList, blankAssign)             // Add _ = v
		newList = append(newList, ifStmt.Body.List[1:]...) // Rest of statements
		ifStmt.Body.List = newList
	}
}
