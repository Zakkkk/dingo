# Lambda Parser Implementation

## Overview
Implemented lambda expression parsing in the Pratt parser to support both TypeScript and Rust lambda syntaxes.

## Files Changed

### Created
- **pkg/parser/lambda.go** (367 lines)
  - `parseLambda()` - Main entry point, detects lambda style
  - `parseRustLambda()` - Parses `|params| body` and `|params| -> Type { body }`
  - `parseTSLambda()` - Parses `(params) => body`
  - `parseTSSingleParamLambda()` - Parses `x => body` (no parens)
  - `isTypeScriptLambda()` - Lookahead to distinguish `(expr)` from `(params) =>`
  - `parseLambdaParams()` - Parses parameter list with optional type annotations
  - `parseLambdaParam()` - Parses single parameter: `x` or `x: Type`
  - `parseLambdaBody()` - Routes to expression or block body parser
  - `parseLambdaBlock()` - Parses `{ statements }` block body
  - `parseLambdaExpression()` - Parses expression body with proper boundary detection

### Modified
- **pkg/parser/pratt.go**
  - Updated `NewPrattParser()` to register lambda prefix parsers:
    - `PIPE` token → `parseLambda()` for Rust-style lambdas
    - `LPAREN` token → `parseGroupedOrLambda()` for TypeScript-style or grouped expressions
  - Modified `parseIdentifier()` to detect single-param TypeScript lambdas (`x => expr`)
  - Added `parseGroupedOrLambda()` to handle ambiguity between `(expr)` and `(params) => body`

## Implementation Details

### Lambda Detection Strategy

**TypeScript Style:**
1. **Single-param without parens**: `IDENT` followed by `ARROW` (e.g., `x => x * 2`)
2. **Multi-param or typed**: `LPAREN` followed by lookahead check (e.g., `(x, y) => x + y`)

**Rust Style:**
- Detected by `PIPE` token (e.g., `|x| x * 2`)

### Lookahead Algorithm

The `isTypeScriptLambda()` function performs non-destructive lookahead to distinguish:
- `(expr)` - Grouped expression
- `(params) => body` - TypeScript lambda

Detected patterns:
1. `() =>` - Empty param list
2. `(x) =>` - Single param, no type
3. `(x: Type) =>` - Single param with type
4. `(x, y) =>` - Multiple params (detected by comma)
5. `(x: Type, y: Type) =>` - Multiple params with types

### Body Parsing

**Expression bodies:**
- Collect tokens until natural boundary: unmatched `)`, `}`, `,`, `;`, or EOF
- Track nesting depth to handle nested parens/brackets/braces
- Return raw token string (unparsed)

**Block bodies:**
- Start with `{`, collect until matching `}`
- Track brace depth for nested blocks
- Return raw block text including braces

### Type Annotation Support

**TypeScript:**
```dingo
(x: int, y: int) => x + y
```

**Rust:**
```dingo
|x: int, y: int| -> int { x + y }
```

Parameters with type annotations are parsed as:
```go
ast.LambdaParam{Name: "x", Type: "int"}
```

Parameters without types:
```go
ast.LambdaParam{Name: "x", Type: ""} // Type inference needed
```

## AST Node Structure

Lambda expressions are represented as:

```go
&ast.LambdaExpr{
    LambdaPos:  token.Pos,           // Start position
    Style:      ast.TypeScriptStyle, // or ast.RustStyle
    Params:     []ast.LambdaParam{   // Parameter list
        {Name: "x", Type: "int"},
        {Name: "y", Type: ""},       // Type inference needed
    },
    ReturnType: "int",               // Optional (Rust only)
    Body:       "x + y",             // Unparsed expression or block
    IsBlock:    false,               // true if {}, false if expression
}
```

## Supported Syntax Examples

### TypeScript Style

**Single parameter (no parens):**
```dingo
x => x * 2
x => { return x * 2 }
```

**Multiple parameters:**
```dingo
(x, y) => x + y
(x, y) => { return x + y }
```

**With type annotations:**
```dingo
(x: int) => x * 2
(x: int, y: int) => x + y
```

### Rust Style

**Single parameter:**
```dingo
|x| x * 2
|x| { return x * 2 }
```

**Multiple parameters:**
```dingo
|x, y| x + y
|x, y| { return x + y }
```

**With type annotations:**
```dingo
|x: int| x * 2
|x: int, y: int| x + y
```

**With return type:**
```dingo
|x: int| -> int { x * 2 }
|x: int, y: int| -> int { x + y }
```

## Edge Cases Handled

1. **Ambiguity resolution**: `(x)` vs `(x) =>` - uses lookahead
2. **Nested expressions**: Properly tracks paren/bracket/brace depth
3. **Empty param list**: `() => expr` supported
4. **Type annotations**: Optional types on parameters
5. **Return types**: Rust-style `-> Type` before body
6. **Block vs expression**: Detects `{` to determine body type

## Integration Points

- **Tokenizer**: Uses existing tokens (`PIPE`, `LPAREN`, `RPAREN`, `ARROW`, `THIN_ARROW`, `LBRACE`, `RBRACE`, `IDENT`, `COLON`, `COMMA`)
- **AST**: Constructs `ast.LambdaExpr` nodes (defined in `pkg/ast/lambda.go`)
- **Pratt Parser**: Registered as prefix parsers for `PIPE`, `LPAREN`, and conditionally for `IDENT`

## Next Steps

1. **Type inference**: Implement type inference for parameters without explicit types
2. **Body transformation**: Parse lambda body into proper AST expressions (currently stored as raw string)
3. **Code generation**: Use `ast.LambdaExpr.ToGo()` to generate Go function literals
4. **Testing**: Create comprehensive test suite for all lambda syntax variations

## Known Limitations

1. **Body is unparsed**: Lambda body is stored as raw token string, not parsed AST
2. **No semantic validation**: Type checking and scope validation not implemented
3. **No optimization**: No dead code elimination or constant folding
4. **Basic boundary detection**: Expression boundary detection could be more sophisticated

## Metrics

- **New code**: 367 lines (lambda.go)
- **Modified code**: ~15 lines (pratt.go)
- **Functions added**: 10 parser functions
- **Syntax variants**: 2 styles × 4 forms = 8 total lambda patterns supported
