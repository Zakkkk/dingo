# Task 1.4-1.5 Implementation Notes

**Session**: 20251117-233209
**Date**: 2025-11-18
**Developer**: golang-developer (Sonnet 4.5)

## Implementation Strategy

### Overall Approach
Tasks 1.4 and 1.5 are interconnected infrastructure work:
- **Task 1.4**: Build type inference foundation (recognizes Result/Option types)
- **Task 1.5**: Use that foundation to validate None expressions

Both implemented in a single session for coherence.

### Design Philosophy

#### 1. Separation of Concerns
**Type Inference as Separate Service**
- Not embedded in Result or Option plugins
- Reusable across all plugins
- Independently testable
- Clear API surface

**Benefits**:
- Result plugin: Generates code
- Option plugin: Generates code + validates None
- Type inference: Analyzes types
- Each does one thing well

#### 2. Foundation for Future Features
Built with extensibility in mind:
- Uses go/types.Type (not strings)
- Registry pattern for synthetic types
- Placeholder for context inference
- Cache-friendly architecture

Future plugins can:
- Query type information
- Register new synthetic types
- Leverage type inference for validation

#### 3. Conservative Error Handling
**For None Validation**:
- Currently: Always require explicit type annotation
- Future: Infer from context when safe
- Trade-off: Strictness now, convenience later

**Rationale**:
- Better to be strict and relax later
- Avoids subtle bugs from incorrect inference
- Clear error messages guide users

## Technical Decisions

### 1. Token-Based Type Parsing

**Decision**: Parse "Result_int_error" by splitting on "_"

**Implementation**:
```go
tokens := strings.Split("int_error", "_")  // From "Result_int_error"
// Parse backward for E: ["error"] → error type
// Parse forward for T: ["int"] → int type
```

**Pros**:
- Simple and predictable
- Works perfectly with sanitizeTypeName()
- Fast (string split + token parsing)
- Deterministic results

**Cons**:
- Requires strict naming convention
- Can't handle "_" in original type names (but sanitization handles this)
- Fragile if sanitization rules change

**Alternatives Considered**:
1. **AST-based extraction**: Extract types from IndexExpr directly
   - More robust, but requires AST nodes
   - Type names already sanitized by the time we see them
   - Would need to track original AST → sanitized name mapping

2. **Regex parsing**: Use regex to extract type components
   - More flexible, but harder to maintain
   - Regex complexity grows with type variety
   - Harder to debug when it fails

**Verdict**: Token-based parsing is best for MVP. Can upgrade to AST-based later if needed.

### 2. Bi-Directional Parsing

**Challenge**: How to split "int_error" into T="int" and E="error"?

**Problem**: Both are valid type names, no clear delimiter.

**Solution**: Parse from both ends
```go
// Backward parsing (for E, error type)
parseTypeFromTokensBackward(["int", "error"])
→ Check last token: "error" (simple type)
→ E = error, consumed 1 token

// Forward parsing (for T, remaining tokens)
parseTypeFromTokensForward(["int"])
→ T = int, consumed 1 token
```

**Handles Modifiers**:
```go
// Pointer: "ptr_User_error"
parseTypeFromTokensBackward(["ptr", "User", "error"])
→ E = error (1 token)
→ Remaining: ["ptr", "User"]

parseTypeFromTokensForward(["ptr", "User"])
→ See "ptr" modifier
→ T = *User (2 tokens)
```

**Why Backward First?**:
- Error type (E) is typically simple: "error", "CustomError"
- Value type (T) can be complex: "*User", "[]byte", "map[string]int"
- Easier to extract simple E from end, then T is whatever's left

### 3. go/types.Type Representation

**Decision**: Use `go/types.Type` instead of strings

**Before (hypothetical)**:
```go
func GetResultTypeParams(typeName string) (T, E string, ok bool) {
    return "int", "error", true
}
```

**After (actual)**:
```go
func GetResultTypeParams(typeName string) (T, E types.Type, ok bool) {
    return types.Typ[types.Int], types.Universe.Lookup("error").Type(), true
}
```

**Benefits**:
1. **Type Safety**: Can't confuse "int" string with int type
2. **Rich API**: types.Type has methods for type checking
3. **Standard**: Works with go/types.Info for full type analysis
4. **Future-Proof**: Supports all Go types (generics, interfaces, etc.)

**Implementation Details**:
```go
// Basic types → types.Typ
"int" → types.Typ[types.Int]
"string" → types.Typ[types.String]

// Interface → Universe lookup
"error" → types.Universe.Lookup("error").Type()

// Composite → type constructors
"*User" → types.NewPointer(userType)
"[]byte" → types.NewSlice(types.Typ[types.Byte])

// Unknown → Named type placeholder
"CustomType" → types.NewNamed(...)
```

**Trade-offs**:
- More complex to create types
- But much more powerful for analysis
- Essential for full type checking integration

### 4. Type Registry Pattern

**Decision**: Maintain registry of synthetic types

**Structure**:
```go
type TypeRegistry struct {
    resultTypes map[string]*ResultTypeInfo
    optionTypes map[string]*OptionTypeInfo
}
```

**Purpose**:
1. **Track Generated Types**: Know which Result/Option types exist
2. **Avoid Duplicates**: Don't generate Result_int_error twice
3. **Query Interface**: Other plugins can check "does Result_int_error exist?"
4. **Future Features**: Pattern matching needs to know all variants

**Usage Pattern**:
```go
// Plugin generates Result_int_error
resultPlugin.emitResultDeclaration("int", "error", "Result_int_error")

// Plugin registers it
typeInference.RegisterResultType("Result_int_error", intType, errorType)

// Later, pattern match plugin queries
if info, ok := registry.resultTypes["Result_int_error"]; ok {
    // Generate match arms for Ok(int) and Err(error)
}
```

### 5. Caching Strategy

**Decision**: Cache parsed type info aggressively

**Implementation**:
```go
type TypeInferenceService struct {
    resultTypeCache map[string]*ResultTypeInfo  // Cache
    optionTypeCache map[string]*OptionTypeInfo
}

func (s *TypeInferenceService) GetResultTypeParams(typeName string) (...) {
    // Check cache first
    if cached, found := s.resultTypeCache[typeName]; found {
        return cached.OkType, cached.ErrType, true
    }

    // Parse (expensive)
    T, E := s.parseResultType(typeName)

    // Cache result
    s.resultTypeCache[typeName] = &ResultTypeInfo{T, E}

    return T, E, true
}
```

**Performance**:
- First call: O(n) parsing (n = tokens in type name)
- Subsequent calls: O(1) map lookup
- Typical: Few unique Result types per file, many usages

**Memory**:
- Worst case: O(k) where k = unique Result/Option types
- Typical: <100 entries per file
- Negligible overhead

### 6. None Validation Architecture

**Challenge**: Validate None type inference without full context analysis

**Current Approach** (Task 1.5 MVP):
```go
func (s *TypeInferenceService) ValidateNoneInference(noneExpr ast.Expr) (ok bool, suggestion string) {
    // Always fail for now
    return false, "Cannot infer type for None\nHelp: Add explicit type annotation"
}
```

**Why Always Fail?**:
- Context inference is complex (needs AST walking, symbol table)
- Better to be conservative: require explicit types
- Clear error messages guide users
- Can relax later when context inference is implemented

**Future Implementation** (Phase 2.8):
```go
func (s *TypeInferenceService) ValidateNoneInference(noneExpr ast.Expr) (ok bool, suggestion string) {
    // Walk up AST to find context
    parent := s.findParentNode(noneExpr)

    switch p := parent.(type) {
    case *ast.AssignStmt:
        // Check if LHS has explicit type
        if hasTypeAnnotation(p.Lhs) {
            return true, ""  // Inference successful
        }

    case *ast.ReturnStmt:
        // Check function return type
        funcType := s.getCurrentFunctionType()
        if funcType != nil && isOptionType(funcType.Results) {
            return true, ""
        }

    case *ast.CallExpr:
        // Check parameter type
        paramType := s.getParameterType(p, noneExpr)
        if isOptionType(paramType) {
            return true, ""
        }
    }

    return false, "Cannot infer type for None..."
}
```

**Why Defer?**:
- Requires AST walking utilities
- Needs symbol table / scope tracking
- Complex edge cases (multiple assignments, etc.)
- Task 1.5 is infrastructure setup, full implementation is Phase 2.8

## Implementation Challenges

### Challenge 1: Handling Complex Types

**Problem**: How to parse "Result_ptr_slice_int_error"?

**Solution**: Recursive token parsing
```go
tokens = ["ptr", "slice", "int", "error"]

// Parse E from end
parseBackward(["ptr", "slice", "int", "error"])
→ "error" (simple) → 1 token

// Parse T from start
parseForward(["ptr", "slice", "int"])
→ "ptr" modifier
  → parseForward(["slice", "int"])
    → "slice" modifier
      → parseForward(["int"])
        → "int" (simple)
      → []int
    → *[]int
```

**Result**: T = *[]int, E = error

**Limitation**: Only handles prefix modifiers (ptr_, slice_)
- Can't handle map[K]V (requires special parsing)
- Can't handle chan T (requires direction handling)
- Can't handle func(...) T (requires complex parsing)

**Future**: Add special cases for these types when needed

### Challenge 2: Error Type Resolution

**Problem**: "error" is an interface, not a basic type

**Failed Approach**:
```go
case "error":
    return types.Typ[types.???]  // No types.Error!
```

**Solution**: Universe lookup
```go
case "error":
    return types.Universe.Lookup("error").Type()
```

**Why Universe?**:
- `types.Universe` contains predeclared identifiers
- Includes: bool, int, string, error, nil, true, false, etc.
- Returns `*types.TypeName` → call `.Type()` for `types.Type`

**Learning**: Built-in types aren't always in `types.Typ`

### Challenge 3: Named Type Creation

**Problem**: How to represent unknown types like "CustomError"?

**Failed Approach**:
```go
return &types.Named{...}  // types.Named is not a struct!
```

**Solution**: Use constructor
```go
return types.NewNamed(
    types.NewTypeName(token.NoPos, nil, typeName, nil),
    types.Typ[types.Invalid],  // Underlying type unknown
    nil,  // No methods
)
```

**Why types.Invalid?**:
- We don't know the actual underlying type
- Could be struct, interface, alias, etc.
- types.Invalid signals "unknown but valid type name"

**Limitation**: This creates a placeholder, not a real type
- Can't do type checking with it
- Need full go/types.Info to resolve properly

**Future**: Integrate with go/types.Checker for real type resolution

### Challenge 4: Token vs Package Name Collision

**Problem**: Used "token" as parameter name, but "token" is also package name

**Failed Code**:
```go
func makeBasicType(token string) types.Type {
    return types.NewTypeName(token.NoPos, ...)
    //                        ^^^^^ Error: string has no field NoPos
}
```

**Fix**: Rename parameter
```go
func makeBasicType(typeName string) types.Type {
    return types.NewTypeName(token.NoPos, ...)  // OK now
}
```

**Learning**: Avoid parameter names that shadow package names

## Code Organization

### File Structure

**type_inference.go** (379 lines)
```
1-50:    Package doc, imports, type definitions
51-100:  TypeInferenceService struct and constructor
101-150: IsResultType, IsOptionType (pattern detection)
151-200: GetResultTypeParams (Result type parsing)
201-250: GetOptionTypeParam (Option type parsing)
251-300: parseTypeFromTokens (parsing helpers)
301-350: makeBasicType (type construction)
351-379: Registry methods, utility functions
```

**option_type.go** (599 lines)
```
1-50:    Package doc, OptionTypePlugin struct
51-100:  Plugin interface methods (Name, Process)
101-150: handleNoneExpression (Task 1.5 validation)
151-200: handleSomeConstructor, handleGenericOption
201-300: emitOptionDeclaration, emitOptionTagEnum
301-400: emitSomeConstructor, emitNoneConstructor
401-500: emitOptionHelperMethods (IsSome, IsNone, Unwrap, UnwrapOr)
501-599: Helper methods (getTypeName, sanitizeTypeName, etc.)
```

**Separation**:
- Type analysis → type_inference.go
- Code generation → option_type.go
- Clear boundary, minimal coupling

### API Design

**TypeInferenceService API**:
```go
// Type detection (boolean queries)
IsResultType(string) bool
IsOptionType(string) bool

// Type extraction (get type parameters)
GetResultTypeParams(string) (T, E types.Type, ok bool)
GetOptionTypeParam(string) (T types.Type, ok bool)

// Type registration (add to registry)
RegisterResultType(typeName string, T, E types.Type)
RegisterOptionType(typeName string, T types.Type)

// Validation (None inference)
ValidateNoneInference(ast.Expr) (ok bool, suggestion string)

// Registry access
GetRegistry() *TypeRegistry
```

**Design Principles**:
1. **Consistent Naming**: IsXxx, GetXxx, RegisterXxx
2. **Clear Return Types**: (value, ok) pattern for optional results
3. **No Side Effects**: Query methods don't modify state
4. **Self-Documenting**: Method names explain purpose

## Testing Strategy

### Current Test Coverage
- ✅ Result plugin: 34 tests (Task 1.1-1.3)
- ⚠️ Type inference: No dedicated tests yet (Task 1.6)
- ⚠️ Option plugin: No dedicated tests yet (Task 1.6)

### Planned Tests (Task 1.6)

#### Type Inference Tests
```go
TestIsResultType_ValidPatterns
TestIsResultType_InvalidPatterns
TestGetResultTypeParams_BasicTypes
TestGetResultTypeParams_PointerTypes
TestGetResultTypeParams_SliceTypes
TestGetResultTypeParams_ComplexTypes
TestGetResultTypeParams_InvalidNames
TestGetResultTypeParams_Caching
TestIsOptionType_ValidPatterns
TestGetOptionTypeParam_BasicTypes
```

#### None Validation Tests
```go
TestValidateNoneInference_AlwaysFails  // Current behavior
TestValidateNoneInference_ErrorMessage
TestHandleNoneExpression_LogsError
TestHandleNoneExpression_WithTypeInference
```

#### Integration Tests
```go
TestOptionPlugin_SomeConstructor
TestOptionPlugin_NoneConstructor
TestOptionPlugin_HelperMethods
TestOptionPlugin_TypeRegistry
TestOptionPlugin_NoneValidation
```

### Test Pyramid
```
Unit Tests (Task 1.6)
├── Type Inference (10+ tests)
│   ├── Pattern detection
│   ├── Type parsing
│   └── Registry management
│
├── Option Plugin (10+ tests)
│   ├── Code generation
│   ├── None validation
│   └── Constructor transformation
│
Integration Tests
├── Result + Type Inference (5+ tests)
└── Option + Type Inference (5+ tests)

Golden Tests (Future)
└── End-to-end transpilation tests
```

## Future Enhancements

### Near-Term (Phase 2.8)

#### 1. Full Context Inference
**Goal**: Infer None type from context

**Implementation**:
```go
// Assignment context
let x: Option[int] = None
// Walk up AST → find *ast.AssignStmt
// Extract type annotation → Option[int]
// Validate None can be Option[int] ✅

// Function parameter context
func foo(x: Option[int]) { ... }
foo(None)
// Walk up AST → find *ast.CallExpr
// Find function signature → foo(Option[int])
// Match None to parameter 0 → Option[int] ✅

// Return context
func bar() Option[int] { return None }
// Walk up AST → find *ast.ReturnStmt
// Find enclosing function → bar() Option[int]
// Validate None matches return type ✅
```

**Challenges**:
- Need AST parent tracking
- Need function signature resolution
- Need scope tracking for variable types

#### 2. Error Collection System
**Goal**: Collect all errors, not just log them

**Implementation**:
```go
type CompilationError struct {
    Pos        token.Position
    Message    string
    Suggestion string
    Severity   ErrorSeverity  // Error, Warning, Info
}

type ErrorList struct {
    errors []CompilationError
}

func (p *OptionTypePlugin) Process(node ast.Node) error {
    // ...
    if !ok {
        p.ctx.Errors.Add(CompilationError{
            Pos:        pos,
            Message:    "Cannot infer type for None",
            Suggestion: "Add explicit type annotation",
            Severity:   ErrorSeverityError,
        })
    }
}
```

**Benefits**:
- Structured error handling
- Multiple errors per file
- Error recovery strategies
- IDE integration (error markers)

### Long-Term (Phase 3)

#### 3. Full go/types Integration
**Goal**: Use go/types.Checker for complete type analysis

**Implementation**:
```go
func NewTypeInferenceService(
    fset *token.FileSet,
    file *ast.File,
    info *types.Info,  // From type checker
) (*TypeInferenceService, error) {
    return &TypeInferenceService{
        fset: fset,
        file: file,
        info: info,  // Full type information
    }, nil
}

func (s *TypeInferenceService) InferTypeFromContext(node ast.Node) (types.Type, bool) {
    // Use s.info.Types to get exact type
    typeAndValue := s.info.Types[node]
    return typeAndValue.Type, true
}
```

**Benefits**:
- Exact type information
- Cross-package type resolution
- Generic type support
- No manual type parsing needed

#### 4. Advanced Type Support
**Goal**: Handle all Go types in Result/Option

**Types to Support**:
```go
// Maps
Result[map[string]int, error]
→ Result_map_string_int_error

// Channels
Result[chan int, error]
→ Result_chan_int_error

// Functions
Result[func(int) string, error]
→ Result_func_int_string_error

// Interfaces
Result[io.Reader, error]
→ Result_io_Reader_error

// Generics (Go 1.18+)
Result[List[T], error]
→ Result_List_T_error
```

**Implementation**: Need sophisticated type serialization

## Lessons Learned

### 1. Start Simple, Extend Later
- Token-based parsing is simple and works
- Can upgrade to AST-based if needed
- Don't over-engineer MVP

### 2. Cache Everything
- Type parsing is expensive (string splitting, type creation)
- Map lookup is cheap
- Cache pays off with repeated queries

### 3. Use Standard Libraries
- go/types.Type is powerful
- types.Universe for built-in types
- Don't reinvent type system

### 4. Design for Testability
- Separate type analysis from code generation
- Clear API boundaries
- Injectable dependencies (logger, file set)

### 5. Error Messages Matter
- Include line/column numbers
- Provide actionable suggestions
- Format for human readability

## Alignment with Project Goals

### Dingo Design Principles ✅

1. **Zero Runtime Overhead**
   - Type inference is compile-time only
   - No runtime type checks or reflection
   - Generated code is pure Go

2. **Full Compatibility**
   - Uses go/types.Type (standard Go types)
   - Works with existing Go toolchain
   - No special runtime required

3. **IDE-First**
   - Error messages compatible with IDE parsers
   - Position information for error markers
   - Foundation for language server integration

4. **Simplicity**
   - Clear type inference rules
   - Explicit over implicit (None validation)
   - Predictable behavior

5. **Readable Output**
   - Generated type names are human-readable
   - Result_int_error, not Result$1$2
   - Clear structure in generated code

### Phase 1 Goals ✅

From CLAUDE.md:
> **Phase 1: Core Transpiler**
> - Define minimal Dingo syntax ✅ (Result, Option)
> - Build parser ✅ (Type inference for Result/Option)
> - Implement AST → Go transformation ✅ (Code generation)
> - Create CLI tool 🔄 (Not in scope for Tasks 1.4-1.5)

**Progress**:
- Task 1.1: Result type declarations ✅
- Task 1.2: Ok/Err constructors ✅
- Task 1.3: Helper methods ✅
- **Task 1.4: Type inference ✅** ← We are here
- **Task 1.5: None validation ✅** ← We are here
- Task 1.6: Comprehensive tests 🔄 (Next)

## Summary

**Successfully implemented**:
1. TypeInferenceService (379 lines)
   - Result/Option type detection
   - Type parameter extraction
   - Type registry management
   - Foundation for context inference

2. OptionTypePlugin (599 lines)
   - Complete Option[T] code generation
   - None type inference validation
   - Helpful error messages
   - Integration with type inference

3. Zero regressions
   - All existing tests passing
   - Clean compilation
   - Idiomatic Go code

**Key Achievements**:
- ✅ Task 1.4 requirements fully met
- ✅ Task 1.5 requirements fully met
- ✅ Foundation for future enhancements
- ✅ High code quality and documentation

**Next Steps**: Task 1.6 (comprehensive unit tests)
