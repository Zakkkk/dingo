# Migration Guide: From Go to Dingo

This guide helps you migrate existing Go code to Dingo or decide when migration makes sense.

## Should You Migrate?

### When Dingo Shines

✅ **Use Dingo when your code has:**
- Heavy error handling (`if err != nil` everywhere)
- Nil pointer risks (pointer-heavy APIs)
- State machines or workflow logic
- Tagged unions or discriminated types
- Complex nested error checking

✅ **Good candidates:**
- API handlers and HTTP services
- Data processing pipelines
- Configuration parsers
- Database interaction layers
- Business logic with complex states

### When to Stick with Go

❌ **Stay with pure Go when:**
- Simple scripts or small tools
- Performance-critical tight loops (until you benchmark)
- Heavy use of reflection or code generation
- Team unfamiliar with Dingo/Rust-like syntax
- Library meant for Go community (unless you commit transpiled `.go` files)

### Gradual Migration Strategy

You don't need to migrate everything at once:

**Phase 1: New code only**
- Write new features in Dingo
- Keep existing Go code untouched
- Mix `.dingo` and `.go` files in same project

**Phase 2: Hot spots**
- Migrate files with heavy error handling
- Migrate complex state management
- Migrate error-prone nil handling

**Phase 3: Full migration**
- Convert remaining files if ROI is clear
- Keep Go for truly simple utilities

## Feature Mapping

### Error Handling

#### Pattern: `if err != nil`

**Go:**
```go
func processData(input string) (Result, error) {
    validated, err := validate(input)
    if err != nil {
        return Result{}, fmt.Errorf("validation failed: %w", err)
    }

    parsed, err := parse(validated)
    if err != nil {
        return Result{}, fmt.Errorf("parsing failed: %w", err)
    }

    transformed, err := transform(parsed)
    if err != nil {
        return Result{}, fmt.Errorf("transformation failed: %w", err)
    }

    return transformed, nil
}
```

**Dingo:**
```go
func processData(input: string) (Result, error) {
    validated := validate(input)?
    parsed := parse(validated)?
    transformed := transform(parsed)?
    return transformed, nil
}
```

**Savings: 67% fewer lines**

#### Pattern: Nil Checks

**Go:**
```go
func getUserCity(userID int) (string, error) {
    user, err := db.GetUser(userID)
    if err != nil {
        return "", err
    }

    if user == nil {
        return "", errors.New("user not found")
    }

    if user.Address == nil {
        return "", errors.New("no address")
    }

    if user.Address.City == nil {
        return "", errors.New("no city")
    }

    return *user.Address.City, nil
}
```

**Dingo:**
```go
enum UserOption {
    Some(User),
    None,
}

func getUserCity(userID: int) (string, error) {
    userOpt := db.GetUserSafe(userID)?

    if userOpt.IsNone() {
        return "", errors.New("user not found")
    }

    user := *userOpt.some0

    if user.Address == nil || user.Address.City == nil {
        return "", errors.New("no address/city")
    }

    return *user.Address.City, nil
}
```

**Safer with Option types and pattern matching**

### State Management

#### Pattern: String-based states

**Go:**
```go
type Order struct {
    ID     string
    Status string  // "pending", "paid", "shipped", etc.
    // Problem: Can set invalid states!
}

func handleOrder(order Order) error {
    switch order.Status {
    case "pending":
        return processPending(order)
    case "paid":
        return processPaid(order)
    case "shipped":
        return processShipped(order)
    default:
        return errors.New("unknown status")
    }
}
```

**Dingo:**
```go
enum OrderStatus {
    Pending(string),              // order ID
    Paid { id: string, amount: float64 },
    Shipped { id: string, tracking: string },
}

func handleOrder(status: OrderStatus) error {
    match status {
        Pending(id) => processPending(id),
        Paid(id, amount) => processPaid(id, amount),
        Shipped(id, tracking) => processShipped(id, tracking)
        // Compile error if you add new status and forget to handle it!
    }
}
```

**Benefits:**
- Impossible to set invalid states
- Exhaustiveness checking
- Type-safe data access

#### Pattern: Interface-based unions

**Go:**
```go
type Shape interface {
    Area() float64
}

type Circle struct {
    Radius float64
}

func (c Circle) Area() float64 {
    return math.Pi * c.Radius * c.Radius
}

type Rectangle struct {
    Width, Height float64
}

func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

func process(s Shape) {
    switch shape := s.(type) {
    case Circle:
        fmt.Println("Circle radius:", shape.Radius)
    case Rectangle:
        fmt.Println("Rectangle:", shape.Width, "x", shape.Height)
    // Easy to add new type and forget to handle it!
    }
}
```

**Dingo:**
```go
enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
}

func area(s: Shape) float64 {
    match s {
        Circle(r) => math.Pi * r * r,
        Rectangle(w, h) => w * h
    }
}

func process(s: Shape) {
    match s {
        Circle(r) => println("Circle radius:", r),
        Rectangle(w, h) => println("Rectangle:", w, "x", h)
        // Compiler error if you add Triangle and don't handle it!
    }
}
```

**78% less boilerplate**

### Optional Values

#### Pattern: Pointer for "maybe nil"

**Go:**
```go
type Config struct {
    Port     int
    Host     string
    Database *DatabaseConfig  // Optional
}

func loadConfig() (*Config, error) {
    config := &Config{
        Port: 8080,
        Host: "localhost",
    }

    dbConfig, err := loadDatabaseConfig()
    if err != nil {
        // Database config is optional, ignore error
        dbConfig = nil
    }

    config.Database = dbConfig
    return config, nil
}

func getDBHost(config *Config) string {
    if config == nil {
        return ""
    }

    if config.Database == nil {
        return ""
    }

    return config.Database.Host
}
```

**Dingo:**
```go
enum DatabaseOption {
    Some(DatabaseConfig),
    None,
}

type Config struct {
    Port     int
    Host     string
    Database DatabaseOption
}

func loadConfig() (Config, error) {
    dbOpt := loadDatabaseConfig()
        .unwrapOr(DatabaseOptionNone())

    return Config{
        Port:     8080,
        Host:     "localhost",
        Database: dbOpt,
    }, nil
}

func getDBHost(config: Config) string {
    match config.Database {
        Some(db) => db.Host,
        None => ""
    }
}
```

**Explicit handling, no nil panics**

## Migration Workflow

### Step 1: Set Up Mixed Environment

```bash
# Your project structure
myproject/
├── go.mod
├── main.go           # Existing Go
├── handler.go        # Existing Go
├── newfeature.dingo  # New Dingo code
└── utils.dingo       # Migrated Dingo code
```

### Step 2: Transpile Dingo Files

```bash
# Add to your build script
dingo build *.dingo
go build .
```

### Step 3: Call Dingo from Go

Dingo transpiles to regular Go, so no special imports needed:

**handler.dingo:**
```go
package main

enum Result {
    Ok(string),
    Err(string),
}

func ProcessRequest(input: string) Result {
    if input == "" {
        return ResultErr("empty input")
    }
    return ResultOk("processed: " + input)
}
```

**main.go:**
```go
package main

func main() {
    result := ProcessRequest("hello")

    if result.IsOk() {
        println("Success:", *result.Ok0)
    } else {
        println("Error:", *result.Err0)
    }
}
```

### Step 4: Call Go from Dingo

Import and use any Go package:

**mycode.dingo:**
```go
package main

import (
    "fmt"
    "encoding/json"
    "github.com/yourusername/yourlib"
)

func processData(data: []byte) (Result, error) {
    var obj Object
    _ := json.Unmarshal(data, &obj)?

    result := yourlib.DoSomething(obj)?

    return result, nil
}
```

## Common Migration Patterns

### API Handlers

**Go:**
```go
func HandleUserUpdate(w http.ResponseWriter, r *http.Request) {
    userID := r.URL.Query().Get("id")
    if userID == "" {
        http.Error(w, "missing user ID", 400)
        return
    }

    var updateReq UpdateRequest
    if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
        http.Error(w, "invalid JSON", 400)
        return
    }

    user, err := db.GetUser(userID)
    if err != nil {
        http.Error(w, "database error", 500)
        return
    }

    if user == nil {
        http.Error(w, "user not found", 404)
        return
    }

    if err := db.UpdateUser(user.ID, &updateReq); err != nil {
        http.Error(w, "update failed", 500)
        return
    }

    json.NewEncoder(w).Encode(user)
}
```

**Dingo:**
```go
func HandleUserUpdate(w: http.ResponseWriter, r: http.Request) {
    result := processUpdate(r)

    match result {
        Ok(user) => json.NewEncoder(w).Encode(user),
        Err(e) => http.Error(w, e.Message, e.Code)
    }
}

enum ApiError {
    BadRequest(string),
    NotFound(string),
    Internal(string),
}

func processUpdate(r: http.Request) (User, ApiError) {
    userID := r.URL.Query().Get("id")
    if userID == "" {
        return User{}, ApiErrorBadRequest("missing user ID")
    }

    var updateReq UpdateRequest
    err := json.NewDecoder(r.Body).Decode(&updateReq)
    if err != nil {
        return User{}, ApiErrorBadRequest("invalid JSON")
    }

    user := db.GetUser(userID)?
        .okOr(ApiErrorNotFound("user not found"))?

    _ := db.UpdateUser(user.ID, updateReq)?

    return user, nil
}
```

**Benefits: 64% less code, clearer error types**

### Data Pipelines

**Go:**
```go
func ProcessPipeline(input []Item) ([]ProcessedItem, error) {
    var processed []ProcessedItem

    for _, item := range input {
        if !item.IsValid() {
            continue
        }

        enriched, err := enrichItem(item)
        if err != nil {
            log.Printf("failed to enrich item %s: %v", item.ID, err)
            continue
        }

        validated, err := validateItem(enriched)
        if err != nil {
            log.Printf("validation failed for item %s: %v", item.ID, err)
            continue
        }

        transformed, err := transformItem(validated)
        if err != nil {
            return nil, fmt.Errorf("transform failed: %w", err)
        }

        processed = append(processed, transformed)
    }

    if len(processed) == 0 {
        return nil, errors.New("no items processed")
    }

    return processed, nil
}
```

**Dingo:**
```go
func ProcessPipeline(input: []Item) ([]ProcessedItem, error) {
    processed := input
        .filter { it.isValid() }
        .mapWithLog { enrichItem(it) }
        .mapWithLog { validateItem(it) }
        .map { transformItem(it) }
        .collect()?

    if processed.isEmpty() {
        return []ProcessedItem{}, errors.New("no items processed")
    }

    return processed, nil
}
```

**Functional style makes data flow obvious**

## Interoperability

### Package Publishing

#### For Applications

`.gitignore`:
```
*.go
!go.mod
!go.sum
```

**Workflow:**
1. Write `.dingo` files
2. Transpile locally with `dingo build`
3. Run with `go run`
4. Don't commit generated `.go` files

#### For Libraries

**Workflow:**
1. Write `.dingo` files
2. Transpile with `dingo build`
3. **Commit both `.dingo` and `.go` files**
4. Tag release

**Why?** Go users can import your library without installing Dingo:

```bash
go get github.com/youruser/yourlib
```

They'll use the committed `.go` files.

### Consuming Go Packages

Any Go package works in Dingo:

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    "google.golang.org/grpc"
)

func setupServer() {
    r := gin.Default()

    r.GET("/users/:id", func(c *gin.Context) {
        id := c.Param("id")
        user := fetchUser(id)?
        c.JSON(200, user)
    })

    _ := r.Run(":8080")?
}
```

### Calling Dingo from Go

**No special imports needed.** Transpiled Dingo is just Go:

**utils.dingo:**
```go
package myapp

enum Result {
    Ok(string),
    Err(string),
}

func ValidateEmail(email: string) Result {
    if !strings.Contains(email, "@") {
        return ResultErr("invalid email")
    }
    return ResultOk(email)
}
```

**main.go:**
```go
package myapp

func main() {
    result := ValidateEmail("test@example.com")

    if result.IsOk() {
        email := *result.Ok0
        println("Valid:", email)
    }
}
```

## Common Pitfalls

### 1. Forgetting to Transpile

**Problem:**
```bash
go build .
# Error: cannot find utils.go
```

**Solution:**
```bash
dingo build *.dingo
go build .
```

**Or use Makefile:**
```makefile
build:
    dingo build *.dingo
    go build .

run: build
    ./myapp
```

### 2. Editing Generated `.go` Files

**Problem:** Your changes get overwritten next time you run `dingo build`

**Solution:** Always edit `.dingo` files, not generated `.go`

### 3. Nil Pointers in Sum Types

**Problem:**
```go
result := ResultOk(42)
if result.IsErr() {
    err := *result.Err0  // PANIC: Err0 is nil!
}
```

**Solution:** Always check variant before accessing:
```go
result := ResultOk(42)
if result.IsOk() {  // Check BEFORE accessing
    value := *result.Ok0
}
```

### 4. Mixing Error Styles

**Problem:**
```go
func mixed(input: string) (Output, error) {
    validated := validate(input)?  // Dingo style

    parsed, err := parse(validated)
    if err != nil {
        return Output{}, err
    }

    transformed := transform(parsed)?  // Dingo style

    return transformed, nil
}
```

**Better:** Pick one style per function:
```go
// All Dingo style
func allDingo(input: string) (Output, error) {
    validated := validate(input)?
    parsed := parse(validated)?
    transformed := transform(parsed)?
    return transformed, nil
}

// Or all Go style (if error wrapping needed)
func allGo(input: string) (Output, error) {
    validated, err := validate(input)
    if err != nil {
        return Output{}, fmt.Errorf("validation: %w", err)
    }

    parsed, err := parse(validated)
    if err != nil {
        return Output{}, fmt.Errorf("parsing: %w", err)
    }

    return transform(parsed)
}
```

## Migration Checklist

### Before Migration

- [ ] Identify high-value targets (lots of error handling)
- [ ] Review Dingo feature set vs your needs
- [ ] Set up build process (`dingo build` before `go build`)
- [ ] Add `.go` to `.gitignore` (for apps) or commit plan (for libraries)

### During Migration

- [ ] Convert one file at a time
- [ ] Run tests after each file
- [ ] Update imports if package structure changes
- [ ] Document any Dingo-specific patterns for team

### After Migration

- [ ] Update CI/CD to run `dingo build`
- [ ] Update README with Dingo setup instructions
- [ ] Monitor generated `.go` code quality
- [ ] Gather metrics (lines of code, bugs, etc.)

## ROI Calculation

### Before Migrating, Ask:

1. **How much error handling code do we have?**
   - Run: `grep -r "if err != nil" . | wc -l`
   - If >100 instances → High ROI for `?` operator

2. **How many nil panics in production?**
   - If frequent → High ROI for Option types

3. **Do we use string-based states?**
   - If yes → High ROI for sum types

4. **How complex is our state management?**
   - Multiple states with different data → High ROI

### Expected Savings

- **Error handling:** 60-70% fewer lines
- **State machines:** 75-80% less boilerplate
- **Nil checks:** 50-60% reduction

## See Also

- [Getting Started](./getting-started.md) - Quick start guide
- [Result Type](./features/result-type.md) - Error handling
- [Option Type](./features/option-type.md) - Null safety
- [Sum Types](./features/sum-types.md) - Enums
- [Pattern Matching](./features/pattern-matching.md) - Match expressions

## Resources

- [Examples](../examples/) - Migration examples
- [Golden Tests](../tests/golden/) - Before/after comparisons
- [TypeScript Migration Guide](https://www.typescriptlang.org/docs/handbook/migrating-from-javascript.html) - Similar migration patterns
