# AST Migration Plan

## Overview

**IMPORTANT: All Dingo syntax transformations should use AST-based approaches, NOT regex.**

The current preprocessor stage uses regex-based text transformations, which are fragile and error-prone. The architectural direction is to migrate ALL transformations to use the Dingo AST system (`pkg/ast/`).

## Why AST over Regex?

| Aspect | Regex | AST |
|--------|-------|-----|
| **Correctness** | Fragile, edge cases break easily | Robust, semantic understanding |
| **Maintainability** | Hard to debug, complex patterns | Clear structure, easy to modify |
| **Type Safety** | None | Full Go type checking |
| **Error Messages** | Poor, position often wrong | Accurate positions, helpful messages |
| **Extensibility** | Adding features = more regex hacks | Clean plugin interface |

## Current State

### Regex-Based Preprocessors (TO BE MIGRATED)

| File | Feature | Priority | Complexity | Status |
|------|---------|----------|------------|--------|
| `keywords.go` | `let` declarations | P0 | Medium | TODO |
| `type_annot.go` | Type annotations (`: Type`) | P0 | Medium | TODO |
| `error_prop.go` | Error propagation (`?`) | P1 | High | TODO |
| `enum.go` | Enum/Sum types | P1 | High | TODO |
| `rust_match.go` | Pattern matching | P1 | High | TODO |
| `rust_match_ast.go` | Match expressions | P0 | High | **BROKEN** - Comment handling fails |
| `lambda.go` | Lambda expressions | P2 | Medium | TODO |
| `null_coalesce.go` | Null coalescing (`??`) | P2 | Medium | TODO |
| `safe_nav.go` | Safe navigation (`?.`) | P2 | Medium | TODO |
| `unqualified_imports.go` | Unqualified imports | P3 | Low | TODO |

### Already Using Parsing (Keep)

| File | Feature | Approach |
|------|---------|----------|
| `ternary.go` | Ternary operator | Character-level parsing |
| `tuples.go` | Tuple expressions | Character-level parsing |

### AST Infrastructure (Extend)

| File | Purpose | Status |
|------|---------|--------|
| `pkg/ast/ast.go` | Dingo AST placeholder | Minimal |
| `pkg/ast/file.go` | File wrapper with DingoNode | Minimal |
| `pkg/parser/parser.go` | Parser interface | Complete |
| `pkg/parser/simple.go` | Preprocessing + go/parser | Active |

## Target Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     DINGO COMPILER                          │
│                                                             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐    │
│  │              │   │              │   │              │    │
│  │ Dingo Parser │──>│  Dingo AST   │──>│  Go Codegen  │    │
│  │              │   │              │   │              │    │
│  └──────────────┘   └──────────────┘   └──────────────┘    │
│         │                  │                  │             │
│         v                  v                  v             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐    │
│  │ Tokenizer/   │   │ DingoNodes:  │   │ go/printer   │    │
│  │ Lexer        │   │ - LetDecl    │   │ + source map │    │
│  │              │   │ - EnumDecl   │   │              │    │
│  │              │   │ - MatchExpr  │   │              │    │
│  │              │   │ - LambdaExpr │   │              │    │
│  └──────────────┘   └──────────────┘   └──────────────┘    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Required AST Nodes

### Phase 1: Core Declarations

```go
// pkg/ast/let.go
type LetDecl struct {
    Let      token.Pos     // position of "let" keyword
    Name     *ast.Ident    // variable name
    Type     ast.Expr      // type (nil for inference)
    Value    ast.Expr      // initial value
    Mutable  bool          // false for let, true for var
}

func (d *LetDecl) Node() {} // implements DingoNode
```

### Phase 2: Sum Types

```go
// pkg/ast/enum.go
type EnumDecl struct {
    Enum       token.Pos
    Name       *ast.Ident
    TypeParams *ast.FieldList  // generic parameters
    Variants   []*Variant
}

type Variant struct {
    Name   *ast.Ident
    Kind   VariantKind  // Unit, Tuple, Struct
    Fields *ast.FieldList
}

type VariantKind int
const (
    VariantUnit VariantKind = iota
    VariantTuple
    VariantStruct
)
```

### Phase 3: Pattern Matching

```go
// pkg/ast/match.go
type MatchExpr struct {
    Match    token.Pos
    Subject  ast.Expr
    Arms     []*MatchArm
}

type MatchArm struct {
    Pattern *Pattern
    Guard   ast.Expr  // optional if guard
    Body    ast.Expr
}

type Pattern struct {
    Kind     PatternKind
    Variant  *ast.Ident
    Bindings []*Binding
    Wildcard bool
}
```

### Phase 4: Lambda Expressions

```go
// pkg/ast/lambda.go
type LambdaExpr struct {
    Params     *ast.FieldList
    ReturnType ast.Expr  // optional
    Body       ast.Expr  // single expression or block
}
```

## Migration Steps

### Step 1: Extend pkg/ast/file.go

```go
type File struct {
    *ast.File
    DingoNodes []DingoNode  // All Dingo-specific nodes
}
```

### Step 2: Create Dingo Tokenizer

Simple tokenizer that recognizes:
- `let`, `var` keywords
- `enum` keyword
- `match` keyword
- `=>` arrow
- `|` pipe
- `?` and `??` operators

### Step 3: Create Dingo Pre-Parser

Extracts Dingo constructs before go/parser:
1. Scan for Dingo syntax
2. Parse into DingoNodes
3. Replace with Go-compatible placeholders
4. Let go/parser handle the rest

### Step 4: Modify Codegen

Generate Go code from DingoNodes + go/ast.File

## DO NOT

- **DO NOT** fix bugs in regex-based preprocessors
- **DO NOT** add new regex patterns
- **DO NOT** extend regex-based approaches
- **DO NOT** copy regex patterns to new files

## INSTEAD

- **DO** implement AST nodes for the feature
- **DO** create proper parsing logic
- **DO** use go/ast utilities where possible
- **DO** write tests for AST transformations

## References

- `pkg/parser/sum_types_test.go` - Shows planned AST structure (disabled)
- `ai-docs/wip/IMPLEMENTATION_PLAN.md` - Original architecture vision
- `ai-docs/language/SYNTAX_DESIGN.md` - Dingo syntax specification

---

**Last Updated:** 2025-11-27
**Status:** Planning Phase
**Owner:** Architecture Team
