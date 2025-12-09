# Hybrid Parser Approaches for Dingo

This document analyzes the optimal split between a text-based preprocessor and a more robust, context-aware parser for Dingo features. The goal is to balance development complexity, correctness, maintainability, and error reporting.

## 1. Dingo Features Requiring a Full, Context-Aware Parser

Features that involve nested structures, semantic dependencies, or require a deep understanding of Go's type system inherently demand a full, context-aware parser. Implementing these as simple text transformations would lead to brittleness, incorrectness, and poor error reporting.

**Examples:**

*   **Pattern Matching (`match` expression):**
    *   **Reasoning:** Pattern matching involves intricate syntax (e.g., `match value { Some(v) => ..., None => ... }`, `match err { MyError(msg) => ..., _ => ... }`) and deeply depends on the structure and types of the matched value. A preprocessor cannot reliably handle nested patterns, type destructuring, or ensure exhaustive checks without full AST context and type information.
    *   **Trade-offs:**
        *   **Complexity:** High in a full parser, but necessary for correctness.
        *   **Correctness:** Guaranteed with a full parser; impossible with a preprocessor for complex patterns.
        *   **Maintainability:** Easier to maintain and extend within an AST-based system where patterns are represented as structured data.
        *   **Error Reporting:** A full parser can provide precise diagnostics for non-exhaustive patterns, type mismatches, and syntax errors within the `match` block.

*   **Advanced Error Propagation (`?` operator with contextual return types):**
    *   **Reasoning:** While basic `x?` can be preprocessed, handling complex scenarios where the `?` operator needs to infer or adapt to different return types of the enclosing function (e.g., `Result[T, E]` vs. `(T, error)`) requires semantic analysis. A preprocessor can only perform simple text substitution, which might break in edge cases or require overly complex regex patterns.
    *   **Trade-offs:**
        *   **Complexity:** Moderate to high for a full parser, especially for type inference.
        *   **Correctness:** A full parser, especially with `go/types` integration, ensures correct type propagation and error handling.
        *   **Maintainability:** Easier to manage complex type rules within a parser/type-checker.
        *   **Error Reporting:** Accurate errors for type mismatches (e.g., `?` on a non-Result/error type in a function not returning one).

*   **Sum Types (`enum` keyword with data variants):**
    *   **Reasoning:** While the basic `enum Name { Variant }` can be preprocessed to structs, handling data variants (e.g., `enum Result { Ok(T), Err(E) }`) requires understanding the types within the variants and generating appropriate Go code (e.g., tagged unions with switch statements). This involves type extraction and structural transformation beyond simple text replacement.
    *   **Trade-offs:**
        *   **Complexity:** High for a full parser to generate robust, idiomatic Go.
        *   **Correctness:** Ensures type safety and correct Go struct generation.
        *   **Maintainability:** Changes to sum type generation logic are localized to the parser.
        *   **Error Reporting:** Can report errors for malformed enum definitions or incorrect variant usage.

## 2. Dingo Features Remaining as Text Transformations (Preprocessor)

Features that are localized, stateless, and do not require deep semantic understanding can effectively and safely remain implemented as simple text transformations. These offer quick wins and minimal complexity in the preprocessor.

**Examples:**

*   **Type Annotations (`param: Type` → `param Type`):**
    *   **Reasoning:** This is a purely syntactic transformation that changes the order of tokens without affecting the overall structure or semantics of the code in a way that requires deep context. A simple regex can reliably convert `: Type` to `Type` in argument lists or variable declarations.
    *   **Trade-offs:**
        *   **Complexity:** Very low in a preprocessor.
        *   **Correctness:** High, as it's a direct, unambiguous replacement.
        *   **Maintainability:** Very easy to maintain.
        *   **Error Reporting:** Any resulting Go syntax errors would be caught by `go/parser` in the next stage.

*   **Basic Keyword Replacements (e.g., `func` → `func` if Dingo proposed a different keyword here):**
    *   **Reasoning:** Direct 1:1 keyword mapping that doesn't alter surrounding syntax or semantics.
    *   **Trade-offs:** Similar to type annotations – low complexity, high correctness, easy to maintain.

*   **IIFE Pattern for Literals (`Ok(42)` → `dingo.Ok(42)` wrapped in IIFE if needed):**
    *   **Reasoning:** For simple literal transformations that act like function calls, a preprocessor can effectively enclose them in IIFEs. While this involves a bit more structure, it's still pattern-based and doesn't require deep semantic analysis before `go/parser` takes over. The actual IIFE generation for complex cases would likely be handled by a later AST transform.
    *   **Trade-offs:**
        *   **Complexity:** Low to moderate, depending on the complexity of the IIFE pattern.
        *   **Correctness:** High for well-defined patterns.
        *   **Maintainability:** Reasonable.
        *   **Error Reporting:** Errors would surface if the preprocessor generates invalid Go that `go/parser` cannot handle.

## 3. Analysis of Trade-offs (Preprocessor vs. Full Parser)

| Aspect             | Preprocessor (Text-based)                                   | Full Parser (AST-based)                                         |
| :----------------- | :---------------------------------------------------------- | :-------------------------------------------------------------- |
| **Complexity**     | Low for simple, localized transformations. High for complex, contextual patterns (regex become unmanageable). | High initially (parser/AST/type-checker implementation). Lower for individual feature logic. |
| **Correctness**    | High for simple transformations. Low for contextual or nested features (prone to subtle bugs). | High for all features if implemented correctly; handles edge cases and semantic rules. |
| **Maintainability**| Easy for simple rules. Very difficult for complex or interdependent rules (regex spaghetti). | Easier to reason about and modify individual feature logic within a structured AST. |
| **Error Reporting**| Generally poor; errors are often reported by the subsequent Go compiler, far from the Dingo source. Difficult to map back. | Precise and context-aware error messages, directly mapping to Dingo source locations. |
| **Performance**    | Fast for text processing.                                   | Slower due to full parsing, AST construction, and semantic analysis. |
| **Feasibility**    | Only for purely syntactic, localized transformations.       | Essential for features requiring semantic understanding, type checking, or structural transformations. |
| **Iteration Speed**| Faster for initial basic features.                          | Slower for initial setup, but faster for adding complex, robust features. |
| **Debugging**      | Difficult due to indirect errors and lack of context.       | Far easier with AST and semantic information available.         |

## Conclusion

The optimal hybrid parser approach for Dingo involves a clear division of labor:

*   **Preprocessor:** Should be reserved for **simple, purely syntactic, and localized transformations**. Examples include basic type annotations (`param: Type` → `param Type`) and straightforward keyword mappings. These provide quick wins and keep the preprocessor lean and efficient.
*   **Full Parser (AST-based):** Crucial for **complex features that require deep semantic understanding, context awareness, nested structures, or type inference**. This includes features like pattern matching, advanced error propagation (where return types are inferred), and sum types with data variants. While more complex to implement, a full parser ensures correctness, robustness, and superior error reporting essential for a reliable language.

This hybrid model leverages the strengths of both approaches: rapid prototyping for simple syntax changes via the preprocessor, and robust, correct implementation of core language features via a proper parser and AST transformation pipeline. This minimizes overall complexity by avoiding "regex abuse" for features that fundamentally require structured analysis.
