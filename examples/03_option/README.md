# Option Type (`Option<T>`)

## Scenario
A user settings system where each preference may or may not be configured. This is a common pattern for configuration, user preferences, and optional data.

## The Problem
Go uses `nil` for absence, which causes issues:
1. Nil pointer panics at runtime
2. No distinction between "not set" and "set to zero value"
3. Easy to forget nil checks

```go
type Settings struct {
    Theme *string  // nil means not set
}

// Dangerous: might panic
fmt.Println(*user.Settings.Theme)
```

## Dingo Solution
`Option<T>` makes absence explicit and type-safe:

```dingo
type UserSettings struct {
    Theme    Option<string>
    FontSize Option<int>
}

// Safe: compiler ensures you handle None case
theme := user.Settings.Theme.UnwrapOr("system")
```

## Comparison

| Aspect | Go pointers | Dingo `Option<T>` |
|--------|-------------|-------------------|
| Nil panic risk | High | Zero |
| Zero vs absent | Ambiguous | Clear |
| Method chaining | None | Map, AndThen, etc. |
| Default handling | Manual | UnwrapOr |

## Key Points

### Option Methods
- `IsSome()` / `IsNone()` - Check presence
- `Unwrap()` - Get value (panics if None)
- `UnwrapOr(default)` - Get value or default
- `Map(fn)` - Transform if present
- `AndThen(fn)` - Chain Option-returning functions

### Constructors
```dingo
Some(value)     // Present value
None[T]()       // Absent value (type parameter required)
```

### When to Use
- Optional configuration values
- Database fields that can be NULL
- API responses with optional fields
- Search results (found vs not found)

### When Go's nil is Fine
- Interface types (already have nil semantics)
- Error returns (use Result instead)
- Simple boolean "exists" checks

## Pattern: UnwrapOr for Defaults
Most common usage - provide a fallback:
```dingo
theme := settings.Theme.UnwrapOr("system")
fontSize := settings.FontSize.UnwrapOr(16)
```

## Pattern: Map for Transformation
Transform the value only if present:
```dingo
cssSize := settings.FontSize
    .Map(func(size: int) string { return fmt.Sprintf("%dpx", size) })
    .UnwrapOr("16px")
```

## Generated Code
The transpiler generates:
- Generic `Option` struct with `present` bool
- Type-safe `Some` and `None` constructors
- Methods that enforce presence checking
- Zero runtime overhead
