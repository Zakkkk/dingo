package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// OptionResultAnalyzer validates safe usage of Option[T] and Result[T,E] methods.
//
// Rules:
// - D003: unchecked-unwrap - .MustSome()/.MustOk() without prior .IsSome()/.IsOk() check
// - D004: unreachable-arm - Match arm can never be reached
//
// Category: correctness
type OptionResultAnalyzer struct{}

func (a *OptionResultAnalyzer) Name() string {
	return "option-result-safety"
}

func (a *OptionResultAnalyzer) Doc() string {
	return "Validates safe usage of Option[T] and Result[T,E] methods (unchecked unwraps, unreachable match arms)"
}

func (a *OptionResultAnalyzer) Category() string {
	return "correctness"
}

func (a *OptionResultAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// D003: Check for unchecked unwraps
	diagnostics = append(diagnostics, a.checkUncheckedUnwraps(fset, file)...)

	// D004: Check for unreachable match arms
	diagnostics = append(diagnostics, a.checkUnreachableArms(fset, file)...)

	return diagnostics
}

// checkUncheckedUnwraps looks for .MustSome()/.MustOk() calls without prior checks
// D003: unchecked-unwrap
func (a *OptionResultAnalyzer) checkUncheckedUnwraps(fset *token.FileSet, file *dingoast.File) []Diagnostic {
	var diagnostics []Diagnostic

	// Strategy:
	// 1. Find all .MustSome() and .MustOk() calls in the AST
	// 2. For each call, check if it's guarded by .IsSome()/.IsOk() check
	// 3. Look backwards in the same function/block for the guard

	// Walk the Go AST to find function declarations
	ast.Inspect(file.File, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// Analyze function body for unwrap patterns
			if node.Body != nil {
				diagnostics = append(diagnostics, a.analyzeBlockForUnwraps(fset, node.Body)...)
			}
		}
		return true
	})

	return diagnostics
}

// analyzeBlockForUnwraps checks a block statement for unchecked unwraps
func (a *OptionResultAnalyzer) analyzeBlockForUnwraps(fset *token.FileSet, block *ast.BlockStmt) []Diagnostic {
	var diagnostics []Diagnostic

	// Track checked variables (var -> true if checked with .IsSome()/.IsOk())
	checked := make(map[string]token.Pos)

	for _, stmt := range block.List {
		// Look for if statements with .IsSome()/.IsOk() checks
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			// Check if condition is .IsSome() or .IsOk()
			if varName := extractCheckCall(ifStmt.Cond); varName != "" {
				checked[varName] = ifStmt.Pos()
				// Variables are safe within the if body
				// For MVP, we don't track scopes precisely, just mark as checked
			}
		}

		// Look for .MustSome() and .MustOk() calls
		ast.Inspect(stmt, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					methodName := sel.Sel.Name
					if methodName == "MustSome" || methodName == "MustOk" {
						// Extract the receiver variable
						varName := extractVarName(sel.X)
						if varName == "" {
							// Can't determine variable, skip
							return true
						}

						// Check if this variable was checked
						if _, isChecked := checked[varName]; !isChecked {
							// Unchecked unwrap!
							diagnostics = append(diagnostics, Diagnostic{
								Pos:      fset.Position(sel.Pos()),
								End:      fset.Position(call.End()),
								Message:  buildUncheckedUnwrapMessage(methodName, varName),
								Severity: SeverityWarning,
								Code:     "D003",
								Category: "correctness",
								Related: []RelatedInfo{
									{
										Pos:     fset.Position(sel.Pos()),
										Message: "Consider checking with .IsSome()/.IsOk() before unwrapping",
									},
								},
							})
						}
					}
				}
			}
			return true
		})
	}

	return diagnostics
}

// extractCheckCall checks if an expression is a .IsSome() or .IsOk() call
// Returns the variable name if it is, empty string otherwise
func extractCheckCall(expr ast.Expr) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return ""
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	methodName := sel.Sel.Name
	if methodName != "IsSome" && methodName != "IsOk" && methodName != "IsNone" && methodName != "IsErr" {
		return ""
	}

	return extractVarName(sel.X)
}

// extractVarName extracts the base variable name from an expression
func extractVarName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		// For x.y.IsSome(), return "x.y"
		base := extractVarName(e.X)
		if base != "" {
			return base + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.IndexExpr:
		// For arr[i].IsSome(), return base variable
		return extractVarName(e.X)
	default:
		return ""
	}
}

// buildUncheckedUnwrapMessage constructs a helpful diagnostic message
func buildUncheckedUnwrapMessage(methodName, varName string) string {
	checkMethod := ".IsSome()"
	if methodName == "MustOk" {
		checkMethod = ".IsOk()"
	}

	return "Unchecked unwrap: ." + methodName + "() used without prior " + checkMethod + " check on " + varName
}

// checkUnreachableArms looks for match arms that can never be reached
// D004: unreachable-arm
func (a *OptionResultAnalyzer) checkUnreachableArms(fset *token.FileSet, file *dingoast.File) []Diagnostic {
	var diagnostics []Diagnostic

	// Walk Dingo AST looking for MatchExpr nodes
	for _, node := range file.DingoNodes {
		if wrapper, ok := node.(*dingoast.ExprWrapper); ok {
			if matchExpr, ok := wrapper.DingoExpr.(*dingoast.MatchExpr); ok {
				diagnostics = append(diagnostics, a.analyzeMatchExpr(fset, matchExpr)...)
			}
		}
	}

	// Also recursively search within Go AST for embedded match expressions
	// (e.g., match inside function bodies)
	diagnostics = append(diagnostics, a.findMatchExprsInAST(fset, file)...)

	return diagnostics
}

// analyzeMatchExpr checks a single match expression for unreachable arms
func (a *OptionResultAnalyzer) analyzeMatchExpr(fset *token.FileSet, matchExpr *dingoast.MatchExpr) []Diagnostic {
	var diagnostics []Diagnostic

	// Track which patterns we've seen
	seenPatterns := make(map[string]bool)
	wildcardSeen := false
	wildcardPos := token.NoPos

	for i, arm := range matchExpr.Arms {
		pattern := arm.Pattern
		if pattern == nil {
			continue
		}

		// Check if this is a wildcard pattern
		if _, isWildcard := pattern.(*dingoast.WildcardPattern); isWildcard {
			if wildcardSeen {
				// Duplicate wildcard (unreachable)
				diagnostics = append(diagnostics, Diagnostic{
					Pos:      fset.Position(pattern.Pos()),
					End:      fset.Position(pattern.End()),
					Message:  "Unreachable match arm: wildcard pattern after another wildcard",
					Severity: SeverityWarning,
					Code:     "D004",
					Category: "correctness",
					Related: []RelatedInfo{
						{
							Pos:     fset.Position(wildcardPos),
							Message: "First wildcard pattern here",
						},
					},
				})
			} else {
				wildcardSeen = true
				wildcardPos = pattern.Pos()

				// Any arms after wildcard are unreachable
				if i < len(matchExpr.Arms)-1 {
					for _, unreachable := range matchExpr.Arms[i+1:] {
						diagnostics = append(diagnostics, Diagnostic{
							Pos:      fset.Position(unreachable.Pattern.Pos()),
							End:      fset.Position(unreachable.Pattern.End()),
							Message:  "Unreachable match arm: appears after wildcard pattern",
							Severity: SeverityWarning,
							Code:     "D004",
							Category: "correctness",
							Related: []RelatedInfo{
								{
									Pos:     fset.Position(wildcardPos),
									Message: "Wildcard pattern here catches all remaining cases",
								},
							},
						})
					}
					break // No need to check further arms
				}
			}
			continue
		}

		// Check for duplicate patterns
		patternStr := pattern.String()
		if seenPatterns[patternStr] {
			diagnostics = append(diagnostics, Diagnostic{
				Pos:      fset.Position(pattern.Pos()),
				End:      fset.Position(pattern.End()),
				Message:  "Unreachable match arm: duplicate pattern '" + patternStr + "'",
				Severity: SeverityWarning,
				Code:     "D004",
				Category: "correctness",
			})
		} else {
			seenPatterns[patternStr] = true
		}

		// Check for patterns after wildcard (already handled above)
		if wildcardSeen {
			diagnostics = append(diagnostics, Diagnostic{
				Pos:      fset.Position(pattern.Pos()),
				End:      fset.Position(pattern.End()),
				Message:  "Unreachable match arm: appears after wildcard pattern",
				Severity: SeverityWarning,
				Code:     "D004",
				Category: "correctness",
				Related: []RelatedInfo{
					{
						Pos:     fset.Position(wildcardPos),
						Message: "Wildcard pattern here",
					},
				},
			})
		}
	}

	return diagnostics
}

// findMatchExprsInAST recursively finds match expressions in the Go AST
func (a *OptionResultAnalyzer) findMatchExprsInAST(fset *token.FileSet, file *dingoast.File) []Diagnostic {
	var diagnostics []Diagnostic

	// For now, rely on DingoNodes being populated
	// A full implementation would walk the Go AST and look for comment markers
	// indicating embedded Dingo expressions

	return diagnostics
}

// isOptionOrResultCheck checks if an if condition is checking Option/Result state
func isOptionOrResultCheck(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	methodName := sel.Sel.Name
	return methodName == "IsSome" || methodName == "IsNone" ||
		methodName == "IsOk" || methodName == "IsErr"
}

// normalizePatternString normalizes pattern representation for comparison
func normalizePatternString(pattern dingoast.Pattern) string {
	if pattern == nil {
		return ""
	}

	// Use the String() method, but normalize whitespace
	str := pattern.String()
	return strings.TrimSpace(str)
}
