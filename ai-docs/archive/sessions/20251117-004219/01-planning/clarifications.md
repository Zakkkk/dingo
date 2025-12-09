# User Clarifications

## 1. Null Coalescing Operator (??) Scope
**Decision**: With both Option[T] and Go pointers (*T)

The `??` operator should work with both Dingo's Option[T] types and native Go pointer types (*T) for better interoperability with existing Go code.

## 2. Safe Navigation Operator (?.) Return Type
**Decision**: Smart unwrapping based on context

The `?.` operator should intelligently unwrap when the context expects the unwrapped type, making it more intuitive for simple cases.

## 3. Lambda Syntax Style
**Decision**: Configuration-based switching between all three styles

Should be a settings option that allows switching between:
- Rust-style: `|x| expr`
- Arrow functions: `(x) => expr`
- Both styles supported simultaneously

## 4. Operator Precedence Conflicts
**Decision**: Both options in settings

Provide configuration options for:
- Following TypeScript/C# conventions (standard precedence)
- Requiring parentheses for complex combinations (explicit mode)

## Additional Notes
The user wants maximum flexibility through configuration rather than choosing a single approach. This means implementing all variants with runtime/compile-time settings to switch between behaviors.
