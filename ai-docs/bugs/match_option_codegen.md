# Bug Report: Match Expression Generates Invalid Code for Option Types

## Summary
`match` expressions on `Option[T]` types generate invalid Go code with undefined identifiers `OptionSome` and `OptionNone`, and incorrect type assertions.

## Severity
**High** - Match expressions with Option types are completely broken.

## Reproduction

### Minimal Example
```dingo
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func main() {
    opt := dgo.Some(42)

    match opt {
        Some(v) => fmt.Println("Got:", v)
        None => fmt.Println("Nothing")
    }
}
```

### Generated Go Code (WRONG)
```go
val := dgo.Some(42)
switch val2 := val.(type) {  // ERROR: val is not an interface
case OptionSome:              // ERROR: undefined: OptionSome
    v := val2.value
    fmt.Println("Got:", v)
case OptionNone:              // ERROR: undefined: OptionNone
    fmt.Println("Nothing")
}
```

### Expected Go Code
```go
val := dgo.Some(42)
if val.IsSome() {
    v := val.Unwrap()
    fmt.Println("Got:", v)
} else {
    fmt.Println("Nothing")
}
```

### Error Messages
```
val2 (variable of struct type dgo.Option[Product]) is not an interface
undefined: OptionSome
undefined: OptionNone
```

## Analysis
The match codegen is treating `Option[T]` as a sum type with variants `OptionSome`/`OptionNone`, but:

1. `Option[T]` is a struct, not an interface - type switches don't work
2. `OptionSome`/`OptionNone` are not defined types - they're method-based

The codegen should detect `dgo.Option[T]` types and generate method-based code using `IsSome()`/`IsNone()`/`Unwrap()`.

## Related Files
- `pkg/ast/match_codegen.go` - Match expression code generation
- `pkg/dgo/option.go` - Option type definition

## Date Reported
2025-12-09
