# Reasoning: null_coalesce_06_mixed_types

## Purpose
Test null coalescing (`??`) with mixed Option and pointer types in same chain.

## Test Coverage

### 1. Option -> pointer transition
```dingo
bio := user?.profile?.bio ?? "No bio"
```
**Expected:** Type change in chain
- user: UserOption (enum)
- profile: *Profile (pointer)
- bio: string (field)
- Must handle: IsSome() -> nil check

### 2. Pointer -> Option transition
```dingo
name := userPtr?.name ?? "Anonymous"
```
**Expected:** Pointer to field
- userPtr: *User (pointer)
- name: string (field)
- Nil check, then field access

### 3. Mixed chain with data
```dingo
let user2: UserOption = UserOption_Some(User{...})
profile := user2?.profile?.bio ?? "Default bio"
```
**Expected:** Option with valid data
- Option check (Some)
- Unwrap
- Pointer check
- Field access

### 4. Method returning pointer
```dingo
email := user3?.getEmail() ?? "no-email"
```
**Expected:** Method call transition
- user3: UserOption
- getEmail(): *string (method returns pointer)
- Must dereference result

## Code Generation Strategy

### Type Boundary Handling
When chain crosses Option<->pointer boundary:

**Option to Pointer:**
```go
if user.IsNone() {
    return ""
}
_user := *user.some  // Unwrap Option

if _user.profile == nil {  // Pointer check
    return ""
}
```

**Pointer to value:**
```go
if userPtr == nil {
    return ""
}
return userPtr.name  // Direct access
```

### Return Type Determination
- If chain ends with pointer -> dereference for ??
- If chain ends with value -> use directly
- If chain ends with Option -> unwrap

### Nil -> None Conversion
When crossing boundaries:
- Pointer nil -> empty string/zero value
- Option None -> empty string/zero value
- Both unify to same "empty" representation

## Edge Cases Tested
- Option containing struct with pointer field
- Pointer to struct (from function)
- Option with valid data
- Method returning pointer
- All type transitions

## Integration Points
- Safe navigation type detection
- Option unwrapping
- Pointer dereferencing
- Method calls
- Type promotion rules

**Last Updated**: 2025-11-20
