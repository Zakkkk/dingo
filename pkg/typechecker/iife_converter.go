// Package typechecker provides IIFE to statement conversion.
// The IIFEConverter transforms IIFE patterns into human-readable if statements
// after TypeRewriter has fixed the types.
package typechecker

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// IIFEConverter converts IIFE patterns to human-like if statements.
// It runs AFTER TypeRewriter so we know the actual types.
//
// Before:
//
//	userLang := func() *string {
//	    tmp := user.Settings
//	    if tmp == nil { return nil }
//	    return tmp.Language
//	}()
//
// After:
//
//	var userLang *string
//	if user.Settings != nil {
//	    userLang = user.Settings.Language
//	}
type IIFEConverter struct {
	fset    *token.FileSet
	file    *ast.File
	changed bool
	cmap    ast.CommentMap
}

// NewIIFEConverter creates a new IIFE converter.
func NewIIFEConverter(fset *token.FileSet, file *ast.File) *IIFEConverter {
	return &IIFEConverter{
		fset: fset,
		file: file,
	}
}

// Convert transforms IIFEs to human-like statements.
// Returns true if any changes were made.
func (c *IIFEConverter) Convert() bool {
	c.changed = false

	// Build comment map before transformation for proper comment handling
	c.cmap = ast.NewCommentMap(c.fset, c.file, c.file.Comments)

	// Process each declaration
	for i := 0; i < len(c.file.Decls); i++ {
		decl := c.file.Decls[i]
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			c.processBlock(fn.Body)
		}
	}

	// After transformation, update comment associations
	if c.changed {
		c.file.Comments = c.cmap.Filter(c.file).Comments()
	}

	return c.changed
}

// processBlock processes statements in a block, converting IIFEs.
func (c *IIFEConverter) processBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}

	// We need to iterate carefully since we may insert new statements
	newStmts := make([]ast.Stmt, 0, len(block.List))

	for _, stmt := range block.List {
		// Check for assignment with IIFE
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			if converted := c.tryConvertAssignment(assign); converted != nil {
				// Transfer comments from original statement to first replacement statement
				if comments := c.cmap[stmt]; comments != nil {
					c.cmap[converted[0]] = comments
					delete(c.cmap, stmt)
				}
				newStmts = append(newStmts, converted...)
				c.changed = true
				continue
			}
		}

		// Process nested blocks
		c.processStmt(stmt)
		newStmts = append(newStmts, stmt)
	}

	block.List = newStmts
}

// processStmt recursively processes nested statements.
func (c *IIFEConverter) processStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		c.processBlock(s.Body)
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*ast.BlockStmt); ok {
				c.processBlock(elseBlock)
			} else if elseIf, ok := s.Else.(*ast.IfStmt); ok {
				c.processStmt(elseIf)
			}
		}
	case *ast.ForStmt:
		c.processBlock(s.Body)
	case *ast.RangeStmt:
		c.processBlock(s.Body)
	case *ast.SwitchStmt:
		c.processBlock(s.Body)
	case *ast.TypeSwitchStmt:
		c.processBlock(s.Body)
	case *ast.SelectStmt:
		c.processBlock(s.Body)
	case *ast.BlockStmt:
		c.processBlock(s)
	case *ast.CaseClause:
		// Process statements inside switch case blocks
		s.Body = c.processStmtList(s.Body)
	case *ast.CommClause:
		// Process statements inside select case blocks
		s.Body = c.processStmtList(s.Body)
	}
}

// processStmtList processes a list of statements, returning the (possibly expanded) list.
func (c *IIFEConverter) processStmtList(stmts []ast.Stmt) []ast.Stmt {
	newStmts := make([]ast.Stmt, 0, len(stmts))

	for _, stmt := range stmts {
		// Check for assignment with IIFE
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			if converted := c.tryConvertAssignment(assign); converted != nil {
				// Transfer comments from original statement to first replacement statement
				if comments := c.cmap[stmt]; comments != nil {
					c.cmap[converted[0]] = comments
					delete(c.cmap, stmt)
				}
				newStmts = append(newStmts, converted...)
				c.changed = true
				continue
			}
		}

		// Process nested blocks
		c.processStmt(stmt)
		newStmts = append(newStmts, stmt)
	}

	return newStmts
}

// tryConvertAssignment tries to convert an IIFE assignment to human-like code.
// Returns the replacement statements or nil if not applicable.
func (c *IIFEConverter) tryConvertAssignment(assign *ast.AssignStmt) []ast.Stmt {
	// Must be a single := assignment
	if assign.Tok != token.DEFINE || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil
	}

	// LHS must be an identifier (the variable name)
	varIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil
	}

	// RHS must be an IIFE call
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil
	}

	funcLit, ok := call.Fun.(*ast.FuncLit)
	if !ok || funcLit.Body == nil {
		return nil
	}

	// Get return type (must have exactly one result)
	if funcLit.Type.Results == nil || len(funcLit.Type.Results.List) != 1 {
		return nil
	}
	returnType := funcLit.Type.Results.List[0].Type

	// Don't convert if return type is still interface{} (type inference failed)
	if c.isInterfaceType(returnType) {
		return nil
	}

	// Check if this is a safe navigation IIFE pattern
	return c.convertSafeNavIIFE(varIdent, returnType, funcLit.Body, assign.Pos())
}

// convertSafeNavIIFE converts a safe navigation IIFE to human-like code.
//
// Pattern:
//
//	func() *string {
//	    tmp := user.Settings
//	    if tmp == nil { return nil }
//	    return tmp.Language
//	}()
//
// Converts to:
//
//	var userLang *string
//	if user.Settings != nil {
//	    userLang = user.Settings.Language
//	}
func (c *IIFEConverter) convertSafeNavIIFE(varIdent *ast.Ident, returnType ast.Expr, body *ast.BlockStmt, origPos token.Pos) []ast.Stmt {
	// Pattern: tmp := baseExpr; if tmp == nil { return nil }; return tmp.Field
	if len(body.List) < 2 {
		return nil
	}

	// First statement: tmp := baseExpr
	tmpAssign, ok := body.List[0].(*ast.AssignStmt)
	if !ok || tmpAssign.Tok != token.DEFINE || len(tmpAssign.Lhs) != 1 {
		return nil
	}
	tmpIdent, ok := tmpAssign.Lhs[0].(*ast.Ident)
	if !ok || !isTmpVar(tmpIdent.Name) {
		return nil
	}
	baseExpr := tmpAssign.Rhs[0]

	// Collect all tmp variable mappings
	tmpExprs := map[string]ast.Expr{
		tmpIdent.Name: baseExpr,
	}

	// Scan body for additional tmp assignments
	for _, stmt := range body.List[1:] {
		if assign, ok := stmt.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
			if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok && isTmpVar(ident.Name) {
					// Resolve the RHS using existing mappings
					resolved := c.resolveExpr(assign.Rhs[0], tmpExprs)
					tmpExprs[ident.Name] = resolved
				}
			}
		}
	}

	// Find the final return statement with the value expression
	var valueExpr ast.Expr
	for i := len(body.List) - 1; i >= 0; i-- {
		if ret, ok := body.List[i].(*ast.ReturnStmt); ok {
			if len(ret.Results) == 1 {
				// Skip "return nil" - we want the value return
				if ident, ok := ret.Results[0].(*ast.Ident); ok && ident.Name == "nil" {
					continue
				}
				valueExpr = ret.Results[0]
				break
			}
		}
	}

	if valueExpr == nil {
		return nil
	}

	// Resolve the value expression
	fullValueExpr := c.resolveExpr(valueExpr, tmpExprs)

	// Build nil checks for all intermediate expressions
	// For user.Settings?.Language: check user.Settings != nil
	conditions := c.buildNilChecks(tmpExprs)
	if len(conditions) == 0 {
		return nil
	}

	var fullCondition ast.Expr
	if len(conditions) == 1 {
		fullCondition = conditions[0]
	} else {
		fullCondition = conditions[0]
		for i := 1; i < len(conditions); i++ {
			fullCondition = &ast.BinaryExpr{
				X:     fullCondition,
				OpPos: origPos,
				Op:    token.LAND,
				Y:     conditions[i],
			}
		}
	}

	// Generate: var varName Type
	// All nodes inherit position from original assignment for proper ordering
	varDecl := &ast.DeclStmt{
		Decl: &ast.GenDecl{
			TokPos: origPos,
			Tok:    token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{{
						NamePos: origPos,
						Name:    varIdent.Name,
					}},
					Type: c.cloneExprWithPos(returnType, origPos),
				},
			},
		},
	}

	// Generate: if condition { varName = valueExpr }
	// Position the if statement slightly after the var declaration
	// to ensure correct ordering in go/printer output
	ifPos := origPos + 1
	ifStmt := &ast.IfStmt{
		If:   ifPos,
		Cond: c.cloneExprWithPos(fullCondition, ifPos),
		Body: &ast.BlockStmt{
			Lbrace: ifPos + 1,
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs:    []ast.Expr{c.positionedIdent(varIdent.Name, ifPos+2)},
					TokPos: ifPos + 2,
					Tok:    token.ASSIGN,
					Rhs:    []ast.Expr{c.cloneExprWithPos(fullValueExpr, ifPos+2)},
				},
			},
			Rbrace: ifPos + 3,
		},
	}

	return []ast.Stmt{varDecl, ifStmt}
}

// isTmpVar checks if a name is a temp variable (tmp, tmp1, tmp2, etc.)
func isTmpVar(name string) bool {
	if name == "tmp" {
		return true
	}
	if len(name) > 3 && name[:3] == "tmp" {
		for _, c := range name[3:] {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	return false
}

// buildNilChecks builds nil check conditions for all mapped expressions.
// Returns unique conditions in order: tmp first, then tmp1, tmp2, etc.
// Uses deduplication to avoid generating `x != nil && x != nil`.
func (c *IIFEConverter) buildNilChecks(tmpExprs map[string]ast.Expr) []ast.Expr {
	// Collect unique conditions preserving order
	seen := make(map[string]bool)
	var conditions []ast.Expr

	// Process "tmp" first (the base)
	if expr, ok := tmpExprs["tmp"]; ok {
		exprStr := c.exprToString(expr)
		if !seen[exprStr] {
			seen[exprStr] = true
			conditions = append(conditions, c.makeNilCheck(expr))
		}
	}

	// Process tmp1, tmp2, etc. in order
	// Use a simple counter that stops when no more tmp variables are found
	for i := 1; ; i++ {
		name := fmt.Sprintf("tmp%d", i)
		expr, ok := tmpExprs[name]
		if !ok {
			break // No more tmp variables
		}
		exprStr := c.exprToString(expr)
		if !seen[exprStr] {
			seen[exprStr] = true
			conditions = append(conditions, c.makeNilCheck(expr))
		}
	}

	return conditions
}

// makeNilCheck creates an expr != nil check.
func (c *IIFEConverter) makeNilCheck(expr ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:  c.cloneExpr(expr),
		Op: token.NEQ,
		Y:  &ast.Ident{Name: "nil"},
	}
}

// positionedIdent creates an identifier with a specific position.
func (c *IIFEConverter) positionedIdent(name string, pos token.Pos) *ast.Ident {
	return &ast.Ident{
		NamePos: pos,
		Name:    name,
	}
}

// resolveExpr resolves tmp variable references in an expression.
func (c *IIFEConverter) resolveExpr(expr ast.Expr, tmpExprs map[string]ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.Ident:
		if resolved, ok := tmpExprs[e.Name]; ok {
			return c.cloneExpr(resolved)
		}
		return &ast.Ident{Name: e.Name}

	case *ast.SelectorExpr:
		return &ast.SelectorExpr{
			X:   c.resolveExpr(e.X, tmpExprs),
			Sel: &ast.Ident{Name: e.Sel.Name},
		}

	case *ast.StarExpr:
		return &ast.StarExpr{
			X: c.resolveExpr(e.X, tmpExprs),
		}

	case *ast.CallExpr:
		// Handle method calls: tmp.Method() → resolved.Method()
		args := make([]ast.Expr, len(e.Args))
		for i, arg := range e.Args {
			args[i] = c.resolveExpr(arg, tmpExprs)
		}
		return &ast.CallExpr{
			Fun:    c.resolveExpr(e.Fun, tmpExprs),
			Lparen: e.Lparen,
			Args:   args,
			Rparen: e.Rparen,
		}

	case *ast.IndexExpr:
		return &ast.IndexExpr{
			X:     c.resolveExpr(e.X, tmpExprs),
			Index: c.resolveExpr(e.Index, tmpExprs),
		}

	case *ast.ParenExpr:
		return &ast.ParenExpr{
			Lparen: e.Lparen,
			X:      c.resolveExpr(e.X, tmpExprs),
			Rparen: e.Rparen,
		}

	case *ast.UnaryExpr:
		return &ast.UnaryExpr{
			OpPos: e.OpPos,
			Op:    e.Op,
			X:     c.resolveExpr(e.X, tmpExprs),
		}

	case *ast.BasicLit:
		// Literals don't need resolution
		return &ast.BasicLit{
			ValuePos: e.ValuePos,
			Kind:     e.Kind,
			Value:    e.Value,
		}

	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			X:     c.resolveExpr(e.X, tmpExprs),
			OpPos: e.OpPos,
			Op:    e.Op,
			Y:     c.resolveExpr(e.Y, tmpExprs),
		}

	default:
		return expr
	}
}

// cloneExpr creates a copy of an expression to avoid sharing AST nodes.
func (c *IIFEConverter) cloneExpr(expr ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.Ident:
		return &ast.Ident{Name: e.Name}

	case *ast.SelectorExpr:
		return &ast.SelectorExpr{
			X:   c.cloneExpr(e.X),
			Sel: &ast.Ident{Name: e.Sel.Name},
		}

	case *ast.StarExpr:
		return &ast.StarExpr{
			X: c.cloneExpr(e.X),
		}

	case *ast.CallExpr:
		args := make([]ast.Expr, len(e.Args))
		for i, arg := range e.Args {
			args[i] = c.cloneExpr(arg)
		}
		return &ast.CallExpr{
			Fun:    c.cloneExpr(e.Fun),
			Lparen: e.Lparen,
			Args:   args,
			Rparen: e.Rparen,
		}

	case *ast.IndexExpr:
		return &ast.IndexExpr{
			X:     c.cloneExpr(e.X),
			Index: c.cloneExpr(e.Index),
		}

	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			X:     c.cloneExpr(e.X),
			OpPos: e.OpPos,
			Op:    e.Op,
			Y:     c.cloneExpr(e.Y),
		}

	case *ast.ParenExpr:
		return &ast.ParenExpr{
			Lparen: e.Lparen,
			X:      c.cloneExpr(e.X),
			Rparen: e.Rparen,
		}

	case *ast.UnaryExpr:
		return &ast.UnaryExpr{
			OpPos: e.OpPos,
			Op:    e.Op,
			X:     c.cloneExpr(e.X),
		}

	case *ast.BasicLit:
		return &ast.BasicLit{
			ValuePos: e.ValuePos,
			Kind:     e.Kind,
			Value:    e.Value,
		}

	default:
		// Unknown type - return original (may cause AST sharing, but safe for read-only)
		return expr
	}
}

// cloneExprWithPos clones an expression and sets its position.
func (c *IIFEConverter) cloneExprWithPos(expr ast.Expr, pos token.Pos) ast.Expr {
	switch e := expr.(type) {
	case *ast.Ident:
		return &ast.Ident{NamePos: pos, Name: e.Name}

	case *ast.SelectorExpr:
		return &ast.SelectorExpr{
			X:   c.cloneExprWithPos(e.X, pos),
			Sel: &ast.Ident{NamePos: pos, Name: e.Sel.Name},
		}

	case *ast.StarExpr:
		return &ast.StarExpr{
			Star: pos,
			X:    c.cloneExprWithPos(e.X, pos),
		}

	case *ast.CallExpr:
		args := make([]ast.Expr, len(e.Args))
		for i, arg := range e.Args {
			args[i] = c.cloneExprWithPos(arg, pos)
		}
		return &ast.CallExpr{
			Fun:    c.cloneExprWithPos(e.Fun, pos),
			Lparen: pos,
			Args:   args,
			Rparen: pos,
		}

	case *ast.IndexExpr:
		return &ast.IndexExpr{
			X:      c.cloneExprWithPos(e.X, pos),
			Lbrack: pos,
			Index:  c.cloneExprWithPos(e.Index, pos),
			Rbrack: pos,
		}

	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			X:     c.cloneExprWithPos(e.X, pos),
			OpPos: pos,
			Op:    e.Op,
			Y:     c.cloneExprWithPos(e.Y, pos),
		}

	case *ast.ArrayType:
		var elt ast.Expr
		if e.Elt != nil {
			elt = c.cloneExprWithPos(e.Elt, pos)
		}
		var length ast.Expr
		if e.Len != nil {
			length = c.cloneExprWithPos(e.Len, pos)
		}
		return &ast.ArrayType{
			Lbrack: pos,
			Len:    length,
			Elt:    elt,
		}

	case *ast.MapType:
		return &ast.MapType{
			Map:   pos,
			Key:   c.cloneExprWithPos(e.Key, pos),
			Value: c.cloneExprWithPos(e.Value, pos),
		}

	case *ast.InterfaceType:
		// Properly clone method list to preserve interface definitions like interface{ Close() }
		var methods *ast.FieldList
		if e.Methods != nil && len(e.Methods.List) > 0 {
			fields := make([]*ast.Field, len(e.Methods.List))
			for i, f := range e.Methods.List {
				fields[i] = c.cloneFieldWithPos(f, pos)
			}
			methods = &ast.FieldList{
				Opening: pos,
				List:    fields,
				Closing: pos,
			}
		} else {
			methods = &ast.FieldList{Opening: pos, Closing: pos}
		}
		return &ast.InterfaceType{
			Interface: pos,
			Methods:   methods,
		}

	case *ast.ParenExpr:
		return &ast.ParenExpr{
			Lparen: pos,
			X:      c.cloneExprWithPos(e.X, pos),
			Rparen: pos,
		}

	case *ast.UnaryExpr:
		return &ast.UnaryExpr{
			OpPos: pos,
			Op:    e.Op,
			X:     c.cloneExprWithPos(e.X, pos),
		}

	case *ast.BasicLit:
		return &ast.BasicLit{
			ValuePos: pos,
			Kind:     e.Kind,
			Value:    e.Value,
		}

	default:
		return expr
	}
}

// exprToString converts an expression to a string for deduplication.
func (c *IIFEConverter) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name

	case *ast.SelectorExpr:
		return c.exprToString(e.X) + "." + e.Sel.Name

	case *ast.StarExpr:
		return "*" + c.exprToString(e.X)

	case *ast.CallExpr:
		args := make([]string, len(e.Args))
		for i, arg := range e.Args {
			args[i] = c.exprToString(arg)
		}
		return c.exprToString(e.Fun) + "(" + strings.Join(args, ",") + ")"

	case *ast.IndexExpr:
		return c.exprToString(e.X) + "[" + c.exprToString(e.Index) + "]"

	case *ast.BinaryExpr:
		return c.exprToString(e.X) + e.Op.String() + c.exprToString(e.Y)

	case *ast.ParenExpr:
		return "(" + c.exprToString(e.X) + ")"

	case *ast.UnaryExpr:
		return e.Op.String() + c.exprToString(e.X)

	case *ast.BasicLit:
		return e.Value

	default:
		// Return a unique identifier for unknown types to prevent false deduplication
		return fmt.Sprintf("<%T@%p>", expr, expr)
	}
}

// isInterfaceType checks if an expression represents interface{}.
func (c *IIFEConverter) isInterfaceType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.InterfaceType:
		return t.Methods == nil || len(t.Methods.List) == 0
	case *ast.Ident:
		return t.Name == "any"
	}
	return false
}

// cloneFieldWithPos clones a field with the given position.
// Used for interface method cloning.
func (c *IIFEConverter) cloneFieldWithPos(f *ast.Field, pos token.Pos) *ast.Field {
	if f == nil {
		return nil
	}

	// Clone names
	var names []*ast.Ident
	if f.Names != nil {
		names = make([]*ast.Ident, len(f.Names))
		for i, n := range f.Names {
			names[i] = &ast.Ident{NamePos: pos, Name: n.Name}
		}
	}

	// Clone type
	var typ ast.Expr
	if f.Type != nil {
		typ = c.cloneExprWithPos(f.Type, pos)
	}

	return &ast.Field{
		Names: names,
		Type:  typ,
	}
}
