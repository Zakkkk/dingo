## Architecture Recommendations for Dingo's Parser

Based on the investigation into Go's native parser extension, third-party Go parsers, meta-language precedents, and hybrid approaches, here are the recommendations for Dingo's parser architecture:

### 1. Recommended Approach: Hybrid (Preprocessor + `go/parser` + `tree-sitter` for LSP)

The current two-stage approach (regex-based preprocessor transforming Dingo syntax to valid Go, followed by `go/parser` and AST transformation) is effective for the core transpilation. However, to enhance LSP capabilities and future-proof the parser for more complex syntax, a hybrid approach integrating `tree-sitter` is recommended.

**Detailed Breakdown:**

*   **Stage 1: Preprocessor (Text-based)**
    *   **Purpose**: Handle simple syntactic sugar and transformations that don't require deep contextual understanding and can be reliably and efficiently converted to valid Go syntax via regex or simple string manipulations.
    *   **Features**: Type annotations (`param: Type` → `param Type`), error propagation (`x?` → `if err != nil { ... }`), `let` bindings, and other keyword replacements.
    *   **Pros**: Fast, simple to implement for specific patterns, leverages existing Go tooling (gopls) once converted.
    *   **Cons**: Limited context, can be fragile for complex or nested structures, debugging can be harder (errors in generated Go).
    *   **Complexity**: Low for simple rules, medium for more intricate regex patterns.
    *   **Maintainability**: Good for isolated, simple patterns. Can become a "regex nightmare" for complex, interconnected syntax.
    *   **Correctness**: Reliant on exhaustive testing for all edge cases.
    *   **Go Version Tracking**: Less sensitive to Go syntax specifics, but must be aware of new keywords or syntax that might clash.

*   **Stage 2: `go/parser` and AST Transformation**
    *   **Purpose**: Parse the preprocessed, now valid Go code into an AST and apply semantic transformations that require contextual understanding and structural manipulation.
    *   **Features**: `enum` keyword (sum types), `Result[T,E]` and `Option[T]` types (including IIFE for literals), pattern matching (`match` expressions), and lambda functions. These features involve significant AST restructuring and type-aware transformations.
    *   **Pros**: Full contextual understanding (with `go/types`), robust for complex structural changes, higher correctness, generates idiomatic Go, leverages official Go tools.
    *   **Cons**: Higher complexity (requires `go/ast` and `go/types` expertise), performance overhead compared to direct text manipulation.
    *   **Complexity**: High.
    *   **Maintainability**: High due to structured approach.
    *   **Correctness**: Very high, especially with `go/types` integration.
    *   **Go Version Tracking**: Highly coupled to `go/parser`, `go/ast`, and `go/types`. Requires careful alignment with Go releases and potential updates to AST transformation logic.

*   **LSP Enhancement: `tree-sitter-go` (Optional but Recommended for Future)**
    *   **Purpose**: While `gopls` wrapping (with source maps) will handle basic LSP features, `tree-sitter` provides a robust, fault-tolerant parsing infrastructure that can be directly extended for Dingo's custom syntax. This would enable richer, more accurate LSP features (e.g., semantic highlighting, context-aware autocompletion, refactoring) even *before* transpilation.
    *   **Integration**: Develop a `tree-sitter-dingo` grammar that extends `tree-sitter-go`. This grammar would understand Dingo-specific syntax, allowing the LSP to operate on the Dingo AST directly. Source maps would then bridge the `tree-sitter-dingo` AST to the `go/parser` AST.
    *   **Pros**: Superior error recovery, incremental parsing (faster for LSP), language-agnostic API, robust for complex syntax, excellent for advanced IDE features.
    *   **Cons**: Adds another dependency and parsing layer, requires developing a `tree-sitter-dingo` grammar.
    *   **Complexity**: Medium (learning `tree-sitter` grammar, integrating with existing LSP).
    *   **Maintainability**: High, `tree-sitter` is well-maintained and widely adopted.
    *   **Correctness**: High.
    *   **Go Version Tracking**: `tree-sitter-go` tracks upstream changes to Go, isolating Dingo's parser from direct Go parser updates for LSP purposes.

### 2. Analysis of Other Approaches:

*   **Go Native Parser Extension**: Directly extending `go/parser` is not feasible as it's not designed for external extension. The recommended hybrid approach effectively "extends" it by providing valid Go code as input, rather than modifying the parser itself.
*   **Third-Party Go Parsers (`participle`, etc.)**: While `participle` is a powerful parser combinator library, constructing a full Go parser (even a subset) and then extending it for Dingo syntax would be a massive undertaking with high maintenance costs as Go evolves. Relying on `go/parser` for the Go portion is critical. `tree-sitter` is the most promising external option for its specific benefits for LSP.
*   **Meta-Language Precedents**:
    *   **TypeScript**: Uses a similar two-stage approach (parser for TS syntax to produce JS, then JS engine processes). It maintains its own parser optimized for the superset language. This reinforces the idea of having a Dingo-aware front-end and a Go-aware back-end.
    *   **Borgo**: Also transpile Rust-like syntax to Go. Investigating its specific implementation details (e.g., regex vs. custom parser for the initial stage) would be highly beneficial for Dingo's preprocessor design.
    *   **CoffeeScript/Elm**: Primarily use custom parsers to translate their syntax to JavaScript. While providing full control, this approach is deemed too high complexity for Dingo given the strong desire to leverage Go's native tooling.

### 3. Pros/Cons and Metrics

| Aspect              | Preprocessor (Current) | `go/parser` (Current)    | `tree-sitter` (Recommended Addition) |
| :------------------ | :--------------------- | :----------------------- | :----------------------------------- |
| **Pros**            | Simple, fast           | Robust, idiomatic Go     | Fault-tolerant, incremental, LSP-ready |
| **Cons**            | Fragile, limited context | High complexity          | New dependency, grammar dev          |
| **Complexity (1-10)** | 3-6 (depending on rule) | 8                      | 5 (initial) + 7 (grammar dev)        |
| **Maintainability** | Medium                 | High                     | High                                 |
| **Correctness**     | Medium                 | Very High                | High                                 |
| **Go Version Track.** | Low impact             | High impact (updates)    | Low impact (via `tree-sitter-go`)    |

### 4. Biggest Risk/Concern

The biggest risk is maintaining robust source map generation and management across three potential layers (Dingo source -> Preprocessor output -> `go/parser` AST -> `tree-sitter` AST for LSP). Accurate source maps are absolutely critical for a good developer experience, especially for debugging and error reporting in the IDE.

### 5. Key Insight/Surprising Finding

The `tree-sitter` project, especially `tree-sitter-go`, offers a surprisingly elegant and powerful solution for enhancing the LSP experience beyond what a purely `gopls`-wrapping approach can provide. It allows Dingo to have its own contextual understanding for IDE features *before* transpilation, while still leveraging `gopls` for the underlying Go.

### 6. Implementation Effort (1-10 scale)

*   **Continue current hybrid (Preprocessor + `go/parser`):** 6 (for new Dingo features)
*   **Integrate `tree-sitter-go` and build `tree-sitter-dingo` grammar:** 8 (initial setup + ongoing grammar development)
