# Hover Test Failures Summary

Generated: 2025-12-30

## Test Results

### VS Code Extension Tests (text-based positioning)
**Total: 51 passed, 4 failing**

Failing tests:
1. DBError type param in `Result[User, DBError]` - second type parameter
2. DBError struct literal in `return DBError{...}`
3. Code field in struct literal
4. email destructured variable in match arm

### lsp-hovercheck (token search on line)
**Total: 26 passed, 21 failed**

## Failures by Category

### 1. Type Parameters in Generics (6 failures)

These failures occur on the **second type parameter** in generic types:

| File | Line | Token | Issue |
|------|------|-------|-------|
| repository.dingo | 54 | `DBError` in `Result[User, DBError]` | No hover on second type param |
| repository.dingo | 70 | `bool` in `Result[bool, DBError]` | No hover (bool is first, but still fails) |
| user_settings.dingo | 35 | `int` in `Option[int]` | No hover on type param |
| user_service.dingo (11) | 91 | `ServiceError` in `Result[User, ServiceError]` | No hover on second type param |
| user_service.dingo (12) | 39 | `Point` in `Result[Point, string]` | No hover on first type param |

**Pattern**: The first `User` type parameter works, but subsequent type params often fail.

### 2. Struct Literals in Transformed Returns (3 failures)

When return statements are transformed to wrap values in `dgo.Err()`:

| File | Line | Token | Issue |
|------|------|-------|-------|
| repository.dingo | 60 | `DBError` | Struct name in `DBError{...}` literal |
| repository.dingo | 60 | `Code` | Field name in struct literal |
| repository.dingo | 60 | `Message` | Field name in struct literal |

**Source**: `return DBError{Code: "NOT_FOUND", ...}`
**Generated**: `return dgo.Err[User](DBError{Code: "NOT_FOUND", ...})`

The `dgo.Err[...]()` wrapper shifts positions.

### 3. Match Arm Destructured Variables (2 failures)

Pattern-matched variables in match arms:

| File | Line | Token | Issue |
|------|------|-------|-------|
| event_handler.dingo | 30 | `userID` | Destructured in `UserCreated(userID, email)` |
| event_handler.dingo | 30 | `email` | Destructured in `UserCreated(userID, email)` |

**Source**: `UserCreated(userID, email) => ...`
**Generated**: Type switch with local variable bindings

### 4. Lambda Parameters (1 failure)

| File | Line | Token | Issue |
|------|------|-------|-------|
| data_pipeline.dingo | 40 | `acc` | First param in `(acc, u) => {...}` |

Note: `u` params on lines 32 and 36 work, but `acc` fails.

### 5. Safe Navigation Fields (3 failures)

| File | Line | Token | Issue |
|------|------|-------|-------|
| config.dingo | 63 | `Database` | Field in `config?.Database` |
| config.dingo | 63 | `Host` | Field in `?.Host` |
| config.dingo | 68 | `CertPath` | Field in `?.CertPath` |

Note: `SSL` on line 68 works, but `CertPath` after it fails.

### 6. Null Coalesce (3 failures)

| File | Line | Token | Issue |
|------|------|-------|-------|
| defaults.dingo | 37 | `Host` | Field before `??` |
| defaults.dingo | 42 | `Port` | Field before `??` |
| defaults.dingo | 98 | `primary` | Variable in chained `??` |

## Root Causes

1. **Position drift** - When code transforms add wrapper functions (`dgo.Err`, `dgo.Ok`), positions shift
2. **Column tracking** - The `//line` directives only track line numbers, not columns
3. **Multi-token constructs** - Generic type params and struct literals span multiple tokens

## Recommended Fixes

1. **Track column in sourcemap** - Use `//line file.dingo:line:col` format for column precision
2. **Calculate position offsets** - When inserting wrapper code, adjust subsequent positions
3. **Test before commit** - Run `./lsp-hovercheck --spec "ai-docs/hover-specs/dingo_transforms.yaml"` before any LSP/sourcemap changes
