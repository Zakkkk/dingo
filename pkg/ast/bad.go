// Package ast provides Dingo-specific AST extensions
//
// Note: For error recovery, we use the standard go/ast.BadExpr, go/ast.BadStmt,
// and go/ast.BadDecl nodes. This file provides documentation and helper functions
// for working with these nodes in the Dingo parser context.
package ast

import (
	"go/ast"
	"go/token"
)

// NewBadExpr creates a new BadExpr node for error recovery.
// BadExpr is a placeholder for invalid or incomplete expressions.
//
// Example usage:
//
//	// When parser encounters invalid expression syntax
//	if err := p.parseExpr(); err != nil {
//	    return &ast.BadExpr{From: startPos, To: currentPos}
//	}
func NewBadExpr(from, to token.Pos) *ast.BadExpr {
	return &ast.BadExpr{From: from, To: to}
}

// NewBadStmt creates a new BadStmt node for error recovery.
// BadStmt is a placeholder for invalid or incomplete statements.
//
// Example usage:
//
//	// When parser encounters invalid statement syntax
//	if err := p.parseStmt(); err != nil {
//	    return &ast.BadStmt{From: startPos, To: currentPos}
//	}
func NewBadStmt(from, to token.Pos) *ast.BadStmt {
	return &ast.BadStmt{From: from, To: to}
}

// NewBadDecl creates a new BadDecl node for error recovery.
// BadDecl is a placeholder for invalid or incomplete declarations.
//
// Example usage:
//
//	// When parser encounters invalid declaration syntax
//	if err := p.parseDecl(); err != nil {
//	    return &ast.BadDecl{From: startPos, To: currentPos}
//	}
func NewBadDecl(from, to token.Pos) *ast.BadDecl {
	return &ast.BadDecl{From: from, To: to}
}

// IsBadNode checks if a node is a Bad* node (error recovery placeholder)
func IsBadNode(node ast.Node) bool {
	if node == nil {
		return false
	}

	switch node.(type) {
	case *ast.BadExpr, *ast.BadStmt, *ast.BadDecl:
		return true
	default:
		return false
	}
}

// HasBadNodes recursively checks if an AST contains any Bad* nodes
// This is useful for determining if a parse was completely successful
func HasBadNodes(node ast.Node) bool {
	if node == nil {
		return false
	}

	found := false

	ast.Inspect(node, func(n ast.Node) bool {
		if IsBadNode(n) {
			found = true
			return false // Stop traversing
		}
		return true
	})

	return found
}

// CollectBadNodes recursively collects all Bad* nodes in an AST
// Returns positions of all error recovery points
func CollectBadNodes(node ast.Node) []ast.Node {
	if node == nil {
		return nil
	}

	var badNodes []ast.Node

	ast.Inspect(node, func(n ast.Node) bool {
		if IsBadNode(n) {
			badNodes = append(badNodes, n)
		}
		return true
	})

	return badNodes
}
