// Package rules provides formatting rules for specific Dingo language constructs.
//
// # Overview
//
// The rules package defines specialized formatting logic for Dingo-specific syntax:
//   - match expressions (pattern matching)
//   - enum declarations (sum types)
//   - lambda expressions (closures)
//   - general spacing and whitespace normalization
//
// Each rule is implemented as a separate formatter that operates on token streams
// and writes formatted output through the TokenWriter interface.
//
// # Architecture
//
// The rules package follows a pluggable architecture where each rule is independent:
//
//	┌─────────────────┐
//	│  Main Formatter │
//	└────────┬────────┘
//	         │
//	         ├──────────┬──────────┬──────────┬──────────┐
//	         │          │          │          │          │
//	    ┌────▼───┐ ┌────▼───┐ ┌───▼────┐ ┌───▼──────┐ │
//	    │ Match  │ │  Enum  │ │ Lambda │ │ Spacing  │ │
//	    │  Rule  │ │  Rule  │ │  Rule  │ │   Rules  │ │
//	    └────┬───┘ └────┬───┘ └───┬────┘ └────┬─────┘ │
//	         │          │          │           │       │
//	         └──────────┴──────────┴───────────┴───────┘
//	                          │
//	                   ┌──────▼──────┐
//	                   │ TokenWriter │
//	                   └─────────────┘
//
// # Usage
//
// Rules are typically invoked by the main formatter when specific token patterns
// are detected:
//
//	// In main formatter
//	if tok.Kind == tokenizer.MATCH {
//	    matchFormatter := rules.NewMatchFormatter()
//	    idx = matchFormatter.Format(tokens, idx, writer)
//	}
//
// Each formatter returns the index of the last token consumed, allowing the
// main formatter to continue processing from the correct position.
//
// # TokenWriter Interface
//
// All formatters write output through the TokenWriter interface:
//
//	type TokenWriter interface {
//	    WriteToken(tok tokenizer.Token)      // Write a single token
//	    WriteNewline()                        // Write a newline and handle indentation
//	    WriteSpaces(count int)                // Write N spaces
//	    IncreaseIndent()                      // Increase indentation level
//	    DecreaseIndent()                      // Decrease indentation level
//	}
//
// This abstraction allows rules to be tested independently and composed flexibly.
//
// # Formatting Rules
//
// Match Expressions (match.go)
//
// Formats pattern matching expressions with optional alignment:
//
//	// Input
//	match x{Some(v)=>v*2,None=>0}
//
//	// Output
//	match x {
//	    Some(v) => v * 2
//	    None    => 0
//	}
//
// Configuration:
//   - AlignArms: Align => arrows across all arms (default: true)
//
// Enum Declarations (enum.go)
//
// Formats sum type declarations:
//
//	// Input
//	enum Status{Active,Inactive(string),Pending}
//
//	// Output
//	enum Status {
//	    Active
//	    Inactive(string)
//	    Pending
//	}
//
// Configuration:
//   - OneVariantPerLine: Force one variant per line (default: true)
//   - AlignTypes: Align variant type annotations (default: false)
//
// Lambda Expressions (lambda.go)
//
// Formats closure syntax with configurable spacing:
//
//	// Input
//	items.map(|x,y|x+y)
//
//	// Output
//	items.map(|x, y| => x + y)
//
// Supports multiple lambda syntaxes:
//   - |x, y| => expr  (Rust-style with =>)
//   - |x| -> expr     (Alternative with ->)
//   - x => expr       (Single param shorthand)
//
// Configuration:
//   - SpacingAroundArrow: Add spaces around => or -> (default: true)
//   - SpaceAfterPipe: Add space after opening | (default: false)
//   - SpaceBeforePipe: Add space before closing | (default: false)
//
// Spacing Rules (spacing.go)
//
// General whitespace normalization applied to all tokens:
//
//   - Space around binary operators: a + b, x == y
//   - Space around assignment: x = 5, y := 10
//   - Space after comma: f(a, b, c)
//   - No space inside parens: (expr), not ( expr )
//   - No space in function calls: f(x), not f (x)
//   - No space before error propagation: x?, not x ?
//
// Configuration:
//   - SpaceAroundBinaryOps: Space around +, -, *, /, ==, etc. (default: true)
//   - SpaceAroundAssignment: Space around =, := (default: true)
//   - SpaceAfterComma: Space after comma (default: true)
//   - NoSpaceBeforeQuestion: No space before ? in x? (default: true)
//   - SpaceAroundNullCoal: Space around ?? operator (default: true)
//   - NoSpaceAroundSafeNav: No space around ?. operator (default: true)
//
// # Comment Preservation
//
// All formatters preserve comments in their original positions:
//
//	enum Status {
//	    Active       // User is active
//	    Inactive(string)  // Reason for inactivity
//	    Pending      // Awaiting activation
//	}
//
// # Indentation
//
// Indentation is managed by the Writer through IncreaseIndent/DecreaseIndent.
// Each rule is responsible for increasing indentation when entering nested
// structures (after {) and decreasing when exiting (before }).
//
// # Testing
//
// Each rule should be tested with:
//   - Golden tests: input.dingo -> expected.dingo
//   - Edge cases: empty blocks, single-line vs multi-line, deeply nested
//   - Comment preservation: ensure comments remain in correct positions
//   - Idempotence: Format(Format(x)) == Format(x)
//
// Example test structure:
//
//	func TestMatchFormatter(t *testing.T) {
//	    tests := []struct{
//	        name  string
//	        input string
//	        want  string
//	    }{
//	        {
//	            name: "simple_match",
//	            input: "match x{Some(v)=>v,None=>0}",
//	            want: "match x {\n    Some(v) => v\n    None => 0\n}",
//	        },
//	        // ... more tests
//	    }
//	}
//
// # Future Enhancements
//
// Potential additions to the rules package:
//   - Guard statement formatting (guard let x = ... else { ... })
//   - Ternary operator formatting (a ? b : c)
//   - Tuple destructuring alignment
//   - Error propagation chain formatting (a()?.b()?.c()?)
//   - Pattern matching in function parameters
//   - Multi-line lambda body formatting
package rules
