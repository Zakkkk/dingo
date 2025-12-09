# Getting Started with Dingo

Welcome to Dingo! This guide will help you get up and running with Dingo in under 15 minutes.

## What is Dingo?

Dingo is a meta-language for Go that adds modern features like sum types, pattern matching, and error propagation while transpiling to clean, idiomatic Go code. Think TypeScript for Go.

**Key Benefits:**
- 67% less error handling boilerplate with the `?` operator
- 78% code reduction with sum types/enums
- Zero nil pointer panics with Option types
- Same performance - transpiles to pure Go
- Full Go ecosystem compatibility

## Installation

### Prerequisites

- Go 1.21 or later
- Git

### Build from Source

```bash
# Clone the repository
git clone https://github.com/MadAppGang/dingo.git
cd dingo

# Build the compiler
go build -o dingo ./cmd/dingo

# Add to PATH (optional but recommended)
export PATH=$PATH:$(pwd)

# Verify installation
./dingo version
```

You should see output like:
```
Dingo v0.1.0-alpha
Go Transpiler and Meta-Language
```

## Your First Dingo Program

Let's start with a simple "Hello, World!" example.

### Step 1: Create a Dingo File

Create a file named `hello.dingo`:

```go
package main

import "fmt"

func main() {
    let message = "Hello from Dingo!"
    fmt.Println(message)
}
```

### Step 2: Build and Run

```bash
# Transpile to Go and compile to binary (like go build)
dingo build hello.dingo

# This creates hello (binary) and hello.go

# Run with dingo (transpiles + runs)
dingo run hello.dingo

# Or just transpile to Go files (no compilation)
dingo go hello.dingo

# This creates hello.go - inspect it!
cat hello.go
```

**What just happened?**
- `dingo build` transpiled your `.dingo` file and compiled it to a binary
- `dingo run` would transpile and run immediately (like `go run`)
- `dingo go` just transpiles to `.go` files (no compilation)
- The `let` keyword became `var` in Go
- The type annotation `:` syntax was converted to Go's format
- All Go flags work: `dingo build -o myapp -race main.dingo`

## Basic Features Walkthrough

### 1. Sum Types with Enums

Dingo makes it easy to create type-safe enums with associated data:

```go
package main

import "fmt"

enum Status {
    Pending,
    Active,
    Complete,
}

func main() {
    let status = Status_Pending()

    if status.IsPending() {
        fmt.Println("Task is pending")
    }
}
```

**What you get:**
- Type-safe enum variants
- Auto-generated constructor functions (`Status_Pending()`)
- Auto-generated type checking methods (`IsPending()`)
- Full exhaustiveness checking

See [docs/features/sum-types.md](./features/sum-types.md) for more details.

### 2. Error Propagation with `?`

Stop writing `if err != nil` over and over:

```go
package main

import (
    "errors"
    "fmt"
)

func fetchUser(id: int) (string, error) {
    if id <= 0 {
        return "", errors.New("invalid ID")
    }
    return "User123", nil
}

func getEmail(user: string) (string, error) {
    return user + "@example.com", nil
}

func getUserEmail(id: int) (string, error) {
    let user = fetchUser(id)?
    let email = getEmail(user)?
    return email, nil
}

func main() {
    email, err := getUserEmail(42)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Email:", email)
    }
}
```

**The `?` operator:**
- Automatically checks for errors
- Returns early if error is found
- Unwraps the success value if no error
- Dramatically reduces boilerplate

See [docs/features/error-propagation.md](./features/error-propagation.md) for details.

### 3. Result and Option Types

Work with Result and Option types for safer error handling:

```go
package main

import "errors"

enum Result {
    Ok(float64),
    Err(error),
}

func divide(a: float64, b: float64) Result {
    if b == 0.0 {
        return Result_Err(errors.New("division by zero"))
    }
    return Result_Ok(a / b)
}

func main() {
    let result = divide(10.0, 2.0)

    if result.IsOk() {
        println("Result:", *result.ok)
    } else {
        println("Error:", result.err.Error())
    }
}
```

See [docs/features/result-type.md](./features/result-type.md) and [docs/features/option-type.md](./features/option-type.md).

### 4. Pattern Matching

Match on sum types with exhaustiveness checking:

```go
package main

import "fmt"

enum Response {
    Success(int),
    Error(string),
    Pending,
}

func handleResponse(resp: Response) string {
    match resp {
        Success(code) => fmt.Sprintf("Success: %d", code),
        Error(msg) => fmt.Sprintf("Error: %s", msg),
        Pending => "Still waiting..."
    }
}

func main() {
    let resp = Response_Success(200)
    println(handleResponse(resp))
}
```

See [docs/features/pattern-matching.md](./features/pattern-matching.md) for advanced examples.

## Building and Running Code

### Build a Binary (like go build)

```bash
dingo build main.dingo
# Transpiles to main.go and compiles to binary

dingo build -o myapp main.dingo
# Specify output binary name

dingo build -race main.dingo
# Enable race detector (Go flag passthrough)
```

### Run Immediately (like go run)

```bash
dingo run main.dingo
# Transpiles and runs in one step

dingo run main.dingo -- --port 8080
# Pass arguments to your program after --
```

### Transpile to Go Only

```bash
dingo go main.dingo
# Creates main.go without compiling

dingo go file1.dingo file2.dingo file3.dingo
# Transpile multiple files
```

### Build/Run Package Mode

```bash
dingo build ./cmd/myapp
# Build all .dingo files in directory

dingo run ./cmd/myapp
# Run package
```

### Check Version

```bash
dingo version
```

### Useful Flags

| Flag | Description |
|------|-------------|
| `--verbose` | Show the underlying go command |
| `--no-mascot` | Disable animated mascot (for CI/scripts) |
| `-o <path>` | Output binary path (build only) |
| `-race` | Enable race detector (Go flag) |
| Any Go flag | Passed through to `go build`/`go run` |

## IDE Setup

### VS Code (Recommended)

**Note:** Full IDE support with the Dingo language server is planned but not yet available. For now, you can work with Dingo files using Go syntax highlighting.

**Current workflow:**
1. Write `.dingo` files using Go syntax highlighting
2. Transpile with `dingo build`
3. Use Go's tooling on the generated `.go` files

**Coming soon:**
- Dingo language server with gopls integration
- Full autocomplete, go-to-definition, and diagnostics
- Auto-transpile on save
- Error messages pointing to `.dingo` files (not generated Go)

### Other Editors

For Vim, Neovim, Emacs, or other editors:
- Use Go syntax highlighting for `.dingo` files
- Configure file associations: `*.dingo` → Go syntax
- Set up build commands to run `dingo build`

## Working with Go Packages

Dingo has 100% compatibility with the Go ecosystem.

### Importing Go Packages

```go
package main

import (
    "fmt"
    "encoding/json"
    "net/http"
)

// Use any Go package as normal
func main() {
    let data = map[string]string{"hello": "world"}
    json, _ := json.Marshal(data)
    fmt.Println(string(json))
}
```

### Mixing `.dingo` and `.go` Files

You can have both `.dingo` and `.go` files in the same package:

```
myproject/
├── main.dingo      # Uses Dingo features
├── utils.go        # Plain Go code
└── handlers.dingo  # More Dingo code
```

Just use dingo build which handles everything:

```bash
# Build the entire project (transpiles .dingo files first)
dingo build ./myproject

# Or transpile separately then use go build
dingo go *.dingo
go build .
```

## Common Patterns

### Error Handling Pipeline

```go
func processData(input: string) (Result, error) {
    let validated = validate(input)?
    let parsed = parse(validated)?
    let transformed = transform(parsed)?
    return Ok(transformed), nil
}
```

### Type-Safe State Machine

```go
enum State {
    Idle,
    Loading(string),
    Success(Data),
    Error(string),
}

func render(state: State) {
    match state {
        Idle => println("Ready"),
        Loading(msg) => println("Loading:", msg),
        Success(data) => println("Data:", data),
        Error(err) => println("Error:", err)
    }
}
```

### Safe Optional Values

```go
enum Option {
    Some(string),
    None,
}

func findUser(id: int) Option {
    if id > 0 {
        return Option_Some("User123")
    }
    return Option_None()
}
```

## Next Steps

Now that you have Dingo running, explore these resources:

📚 **Feature Documentation**
- [Result Type](./features/result-type.md) - Type-safe error handling
- [Option Type](./features/option-type.md) - Null safety
- [Error Propagation](./features/error-propagation.md) - The `?` operator
- [Sum Types](./features/sum-types.md) - Enums with data
- [Pattern Matching](./features/pattern-matching.md) - Match expressions

🔄 **Migration**
- [Migration from Go](./migration-from-go.md) - Convert existing Go code to Dingo

💡 **Examples**
- Browse `examples/` directory for real-world code
- Check `tests/golden/` for working feature examples

🌟 **Community**
- GitHub: [github.com/MadAppGang/dingo](https://github.com/MadAppGang/dingo)
- Issues: Report bugs or request features
- Discussions: Ask questions and share ideas

## Troubleshooting

### Build Errors

**Problem:** `dingo: command not found`
**Solution:** Make sure `dingo` is in your PATH or use `./dingo`

**Problem:** Transpilation errors
**Solution:** Check that your `.dingo` syntax is correct. Run `dingo build -v` for verbose output.

**Problem:** Generated Go code doesn't compile
**Solution:** This is a bug! Please report it with your `.dingo` file and error message.

### Generated Code

**Where is the generated `.go` file?**
It's created in the same directory as your `.dingo` file with the same name.

**Can I edit the generated `.go` file?**
You can, but your changes will be overwritten next time you run `dingo build`. Always edit the `.dingo` file.

**Should I commit `.go` files to git?**
- For **applications**: No, add `*.go` to `.gitignore` (except `go.mod`)
- For **libraries**: Yes, commit transpiled `.go` files so Go users can import your package

## Getting Help

- Check the [documentation](./README.md)
- Browse [examples](../examples/)
- Search [GitHub issues](https://github.com/MadAppGang/dingo/issues)
- Open a new issue for bugs or feature requests

Welcome to the Dingo community! 🎉
