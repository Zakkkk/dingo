# Match-in-Assignment Alternatives

## 1. Variable Hoisting + Switch Assignment
- **Idea**: Detect `let/var x = match ...` and emit `var x <type>` before the switch. Each arm assigns to `x` and the original statement becomes a simple reference.
- **Example**:
  ```go
  var result Option[int]
  __match_0 := opt
  switch __match_0.tag {
  case OptionTagSome:
      x := *__match_0.some_0
      result = Some(x * 2)
  case OptionTagNone:
      result = None[int]()
  }
  ```
- **Pros**: Idiomatic Go, no closures or heap allocations, easy to read, plays nicely with debugging. Keeps match expression semantics by reusing the declared identifier.
- **Cons**: Requires knowing the destination type ahead of time (type inference or explicit annotation). When match sits inside larger expressions (e.g., `foo(match …)`), we still need a temporary variable and parentheses.
- **Complexity**: Moderate. Need to capture the inferred type from context (parser already tracks Option/Result types during preprocessing) or fall back to `var result interface{}` when unknown. Implementation mostly contained in preprocessor with small adjustments to assignment detection logic.
- **Edge Cases**: Works for nested matches and tuple destructuring because assignments stay in arms. Need to ensure `panic` fallback still satisfies the compiler when arms don’t cover all cases.

## 2. Scoped Block Expression
- **Idea**: Replace IIFE with an immediately evaluated block `{ ... }()` but without function literal—just braces that end with the desired value assigned to an identifier defined outside.
- **Example**:
  ```go
  var result Option[int]
  {
      __match_0 := opt
      switch __match_0.tag {
      case OptionTagSome:
          x := *__match_0.some_0
          result = Some(x * 2)
      case OptionTagNone:
          result = None[int]()
      }
  }
  ```
- **Pros**: Removes anonymous function/return boilerplate yet keeps single-evaluation semantics. Minimal change from current transformation.
- **Cons**: Still requires hoisting the target variable. Extra braces may look odd and do not restore expression semantics; you still can’t write `x := { ... }` without prior declaration.
- **Complexity**: Low. Only need to wrap generated switch in braces when match appeared in expression context. No type inference beyond hoisted variable.
- **Edge Cases**: Same as hoisting; braces are syntactic no-ops but must ensure formatting and source maps remain consistent.

## 3. Synthetic Helper Function per Match (Predeclared)
- **Idea**: Generate a tiny helper function ahead of time (`func __matchResult_0() T { ... }`) and call it where the expression sits: `result := __matchResult_0()`.
- **Example**:
  ```go
  result := func() Option[int] {
      __match_0 := opt
      switch __match_0.tag {
      case OptionTagSome:
          x := *__match_0.some_0
          return Some(x * 2)
      case OptionTagNone:
          return None[int]()
      }
  }()
  ```
- **Pros**: Maintains true expression form, no need for destination variable. Closest to functional languages.
- **Cons**: Essentially current IIFE pattern; stylistic complaints persist. Also introduces stack frames and possible heap captures.
- **Complexity**: Already implemented; no work but doesn’t meet cleanliness goal.

## 4. Temporary Struct + Deferred Extraction
- **Idea**: Create a lightweight struct `{ value T; set bool }`, assign to it inside the switch, and after switch read `temp.value`. Mimics Go’s comma-ok idiom.
- **Example**:
  ```go
  var __match_result struct {
      value Option[int]
      set   bool
  }
  __match_0 := opt
  switch __match_0.tag {
  case OptionTagSome:
      x := *__match_0.some_0
      __match_result.value = Some(x * 2)
      __match_result.set = true
  case OptionTagNone:
      __match_result.value = None[int]()
      __match_result.set = true
  }
  if !__match_result.set {
      panic("non-exhaustive match")
  }
  result := __match_result.value
  ```
- **Pros**: Preserves expression evaluation order and supports future optional “match may not assign” detection via the `set` flag.
- **Cons**: Verbose, introduces extra struct and boolean flag, harder to read than simple hoist. Adds runtime checks duplicated with compile-time exhaustiveness.
- **Complexity**: Medium-high; must inject struct definition uniquely per expression and ensure no naming clashes. Limited cleanliness gain.

## 5. Assignment Rewriting to If/Else Chain
- **Idea**: When match appears inside assignment, rewrite to sequential `if/else` statements testing tag equality, ending with default panic.
- **Example**:
  ```go
  var result Option[int]
  __match_0 := opt
  if __match_0.tag == OptionTagSome {
      x := *__match_0.some_0
      result = Some(x * 2)
  } else if __match_0.tag == OptionTagNone {
      result = None[int]()
  } else {
      panic("non-exhaustive match")
  }
  ```
- **Pros**: Possibly more familiar than switch for simple cases, avoids nested breaks.
- **Cons**: Harder to support guards and tuple destructuring; loses fall-through semantics leveraged for exhaustiveness. Duplicate logic versus existing switch transformation.
- **Complexity**: High for minimal benefit; would need parallel implementation path for match arms vs switch.

## Recommendation
Variable hoisting (Option 1) best balances cleanliness and effort: generated Go looks natural, no closures, minimal runtime overhead, and logic stays inside existing switch lowering. Implementation steps: (1) Detect expression/assignment contexts earlier in preprocessor; (2) Emit `var <ident> <type>` before switch using inferred type or fallback `var <ident> interface{}` with later type assertion when necessary; (3) Rewrite each arm body to `target = <expr>` and ensure final statement uses the hoisted identifier; (4) Update source mapping and tests, especially golden cases in `pattern_match_01_*`. Estimated effort: ~0.5 day for code changes + 0.5 day for exhaustive testing and new golden coverage. Risks: type inference for the hoisted variable (mitigated by leveraging existing type annotation info) and ensuring nested expression contexts still compile (can temporarily generate temporary identifiers and replace original expression with that identifier).
