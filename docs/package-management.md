# Package Management in Dingo

**Version**: 1.0
**Status**: Stable
**Last Updated**: 2025-11-19

---

## Overview

Dingo uses a **hybrid package management strategy** that balances Go ecosystem compatibility with developer experience:

- **Libraries**: Transpile `.dingo` → `.go` and publish Go files
- **Applications**: Keep `.dingo` files, transpile during development/build

This approach ensures Dingo libraries work seamlessly with the entire Go ecosystem while application developers get the full Dingo development experience.

---

## The Hybrid Strategy

### Why Two Approaches?

**Library Publishing (Transpile-on-Publish)**:
- ✅ **Zero barrier to adoption** - Go projects can use Dingo libraries without installing Dingo
- ✅ **Full Go tooling support** - Works with `go get`, `go mod`, `gopls`, IDEs
- ✅ **Build reliability** - No external dependencies required for consumers
- ✅ **Ecosystem compatibility** - Appears as normal Go package in pkg.go.dev

**Application Development (Direct .dingo Usage)**:
- ✅ **Best developer experience** - Full Dingo syntax, error messages in Dingo code
- ✅ **Faster iteration** - No publish step, instant feedback
- ✅ **Better debugging** - Source maps point to actual `.dingo` files
- ✅ **LSP integration** - IDE features work on `.dingo` files directly

### Decision Matrix

| Project Type | Publish Strategy | Reason |
|--------------|------------------|--------|
| Library (shared package) | Transpile + commit `.go` files | Maximize compatibility |
| CLI application | Keep `.dingo` only | Internal use, best DX |
| Web service | Keep `.dingo` only | Internal use, best DX |
| Internal tool | Keep `.dingo` only | Internal use, best DX |
| Framework/SDK | Transpile + commit `.go` files | Maximize adoption |

**Rule of thumb**: If it's `go get`-able by others → Transpile and publish Go. If it's your application → Keep Dingo.

---

## Library Publishing Workflow

### Step 1: Develop in Dingo

Create your library using `.dingo` files:

```dingo
// mylib/result.dingo
package mylib

// SafeDivide returns Ok(result) or Err(error)
func SafeDivide(a: int, b: int) -> Result[float64, error] {
    if b == 0 {
        return Err(errors.New("division by zero"))
    }
    return Ok(float64(a) / float64(b))
}
```

### Step 2: Transpile for Publishing

Before publishing, transpile all `.dingo` files:

```bash
# Transpile all .dingo files in package
dingo build ./...

# Verify the generated Go compiles
go build ./...

# Run tests
go test ./...
```

This generates:
- `mylib/result.go` - Clean, idiomatic Go code
- `mylib/result.go.golden.map` - Source map (optional, for debugging)

### Step 3: Commit Transpiled Go Files

**Critical**: Commit the `.go` files to your repository:

```bash
# Add transpiled Go files
git add mylib/*.go

# DO NOT gitignore .go files in libraries!
# Remove *.go from .gitignore if present

# Commit
git commit -m "feat: add SafeDivide function

Transpiled from result.dingo"
```

### Step 4: Tag and Publish

Use semantic versioning:

```bash
# Tag release
git tag v1.0.0
git push origin v1.0.0

# Now available via:
# go get github.com/yourname/mylib@v1.0.0
```

### Step 5: Verify Consumption

Test that pure Go projects can consume your library:

```go
// consumer/main.go (pure Go, no Dingo)
package main

import "github.com/yourname/mylib"

func main() {
    result := mylib.SafeDivide(10, 2)
    // Works! Consumer doesn't need Dingo installed
}
```

---

## Application Development Workflow

### Step 1: Create Application

Keep `.dingo` files, gitignore `.go`:

```bash
# Initialize project
mkdir myapp && cd myapp
go mod init myapp

# Create .gitignore
cat > .gitignore << EOF
# Transpiled Go files (application mode)
*.go
!vendor/**/*.go

# Source maps (optional)
*.go.golden.map
EOF
```

### Step 2: Develop in Dingo

Write application code in `.dingo`:

```dingo
// main.dingo
package main

import "fmt"

func main() {
    config := loadConfig()?
    server := startServer(config)?

    fmt.Println("Server started:", server.Addr)
    server.Run()
}
```

### Step 3: Build and Run

Transpile during development:

```bash
# Development: transpile and run
dingo build main.dingo && go run main.go

# OR use watch mode (future):
# dingo dev main.dingo
```

### Step 4: Production Build

For deployment, transpile and build binary:

```bash
# Transpile all files
dingo build ./...

# Build production binary
go build -o myapp main.go

# Deploy binary (no Dingo needed in production)
./myapp
```

**Note**: The binary is pure Go, no Dingo runtime required!

---

## Consuming Dingo Libraries from Go

### Pure Go Consumer

No Dingo installation required:

```go
// main.go (pure Go project)
package main

import (
    "fmt"
    "github.com/someauthor/dingo-http-utils" // Dingo library
)

func main() {
    // Use Dingo library types naturally
    result := httputils.Fetch("https://api.example.com")

    // Result[T,E] is just a normal Go struct
    if result.IsOk() {
        fmt.Println("Response:", result.Unwrap())
    } else {
        fmt.Println("Error:", result.UnwrapErr())
    }
}
```

### Key Points

1. **No Dingo required**: Consumers only need `go get`
2. **Type safety**: All Dingo types are normal Go structs
3. **Zero overhead**: Transpiled code is idiomatic Go
4. **Full tooling**: Works with gopls, delve, pprof, etc.

---

## Interoperability Patterns

### Dingo Application → Go Library

Seamless, works out of the box:

```dingo
// app.dingo
package main

import "database/sql" // Standard Go library

func getUser(id: int) -> Option[User] {
    var user User
    err := db.QueryRow("SELECT * FROM users WHERE id = ?", id).Scan(&user)

    if err == sql.ErrNoRows {
        return None[User]() // Dingo syntax
    }

    if err != nil {
        panic(err) // Or handle differently
    }

    return Some(user)
}
```

### Dingo Application → Dingo Library

Also seamless, full Dingo syntax:

```dingo
// app.dingo
package main

import "github.com/someauthor/dingo-http-utils"

func fetchData(url: string) -> Result[Data, error] {
    // Both app and library use Dingo syntax
    response := httputils.Fetch(url)?
    data := parseData(response)?
    return Ok(data)
}
```

### Go Application → Dingo Library

Works perfectly (library published as Go):

```go
// main.go (pure Go)
package main

import "github.com/someauthor/dingo-http-utils"

func main() {
    result := httputils.Fetch("https://example.com")

    // Use the Result type
    if result.IsOk() {
        fmt.Println(result.Unwrap())
    }
}
```

---

## Mixed Codebases

### Scenario: Migrating Existing Go Project

You can incrementally adopt Dingo:

**Step 1**: Start with new files in Dingo:

```
myproject/
├── go.mod
├── legacy.go           # Existing Go code
├── legacy_test.go      # Existing Go tests
├── newfeature.dingo    # New code in Dingo
└── newfeature_test.go  # Tests in Go (or .dingo)
```

**Step 2**: Transpile Dingo files:

```bash
dingo build newfeature.dingo
# Generates newfeature.go
```

**Step 3**: Build normally:

```bash
go build ./...
# Both legacy.go and newfeature.go compile together
```

**Step 4**: Gradually migrate more files to Dingo as needed.

### .gitignore Strategy for Mixed Codebases

```gitignore
# Ignore transpiled .go files from .dingo sources
# (List them individually)
newfeature.go
anotherfeature.go

# Keep hand-written Go files
!legacy.go
!legacy_test.go
```

Or use a naming convention:

```gitignore
# Ignore all .go files that have a corresponding .dingo file
*.dingo.go
```

And transpile with:
```bash
dingo build -o feature.dingo.go feature.dingo
```

---

## Publishing Checklist

### For Library Maintainers

- [ ] Transpile all `.dingo` files: `dingo build ./...`
- [ ] Verify Go compilation: `go build ./...`
- [ ] Run all tests: `go test ./...`
- [ ] Run Go linters: `go vet ./...`, `golangci-lint run`
- [ ] Commit `.go` files: `git add *.go`
- [ ] Update README with Go import path
- [ ] Update CHANGELOG
- [ ] Tag version: `git tag v1.x.x`
- [ ] Push: `git push --tags`
- [ ] Verify on pkg.go.dev (appears within ~10 minutes)

### For Application Developers

- [ ] Add `*.go` to `.gitignore` (if not sharing transpiled code)
- [ ] Document build process in README
- [ ] Provide pre-built binaries (optional)
- [ ] Include source maps for debugging (optional)

---

## Module Structure Best Practices

### Recommended Structure for Libraries

```
dingo-mylib/
├── go.mod                    # Go module definition
├── go.sum
├── README.md                 # Include Go usage examples
├── LICENSE
├── .github/
│   └── workflows/
│       └── ci.yml            # CI: transpile + test
├── mylib.dingo               # Source code
├── mylib.go                  # Transpiled (committed)
├── mylib_test.go             # Tests (Go or Dingo)
└── examples/
    ├── basic/
    │   └── main.go           # Pure Go example
    └── advanced/
        └── main.dingo        # Dingo example
```

### Recommended Structure for Applications

```
myapp/
├── go.mod
├── go.sum
├── README.md
├── .gitignore                # Ignore *.go
├── cmd/
│   └── myapp/
│       └── main.dingo        # Entry point
├── internal/
│   ├── config/
│   │   └── config.dingo
│   ├── handlers/
│   │   └── http.dingo
│   └── models/
│       └── user.dingo
├── Makefile                  # Build automation
└── deployments/
    └── Dockerfile            # Include `dingo build` step
```

---

## FAQ

### Q: Can I publish both `.dingo` and `.go` files?

**A**: Yes! This is recommended for libraries. It allows:
- Go consumers to use the library without Dingo
- Dingo consumers to see the original source for better error messages
- Contributors to modify `.dingo` files instead of generated `.go`

### Q: Should I commit source maps (`.go.golden.map`)?

**A**: Optional. Benefits:
- ✅ Better debugging for consumers using Dingo
- ✅ LSP can show original `.dingo` positions
- ❌ Increases repository size slightly

### Q: Can I mix `.dingo` and `.go` files in the same package?

**A**: Yes! The Go compiler doesn't care about file extensions. You can have:

```
mypackage/
├── legacy.go        # Hand-written Go
├── feature.dingo    # Dingo source
└── feature.go       # Transpiled from feature.dingo
```

Just ensure they don't have conflicting declarations.

### Q: How do I handle version updates?

**Library**: Transpile, commit `.go`, tag version, push.

**Application**: Transpile locally, deploy binary (no versioning of transpiled code needed).

### Q: What about CI/CD?

**Library CI**:
```yaml
- name: Transpile Dingo
  run: dingo build ./...

- name: Test
  run: go test ./...

- name: Check for changes
  run: git diff --exit-code  # Fail if .go files not committed
```

**Application CI**:
```yaml
- name: Transpile Dingo
  run: dingo build ./...

- name: Build binary
  run: go build -o myapp cmd/myapp/main.go

- name: Upload artifact
  uses: actions/upload-artifact@v3
  with:
    name: myapp-binary
    path: myapp
```

### Q: Can I use Dingo libraries in production?

**A**: Yes! The transpiled Go code is production-ready:
- ✅ Zero runtime overhead (it's just Go)
- ✅ No dependencies on Dingo tooling
- ✅ Same performance as hand-written Go
- ✅ Works with all Go profiling/debugging tools

---

## Examples

See the `examples/` directory for working demonstrations:

- **`library-example/`** - How to build and publish a Dingo library
- **`app-example/`** - How to build a Dingo application
- **`hybrid-example/`** - App consuming a published Dingo library

---

## Summary

| Aspect | Library (Publish .go) | Application (Keep .dingo) |
|--------|----------------------|---------------------------|
| **Target users** | Other developers | Internal/end users |
| **Commit .go files** | ✅ Yes | ❌ No |
| **Dingo required** | Development only | Development only |
| **Go ecosystem** | Full compatibility | N/A |
| **Developer experience** | Good | Excellent |
| **Use when** | Sharing code | Building apps |

**Remember**: The hybrid strategy gives you the best of both worlds—Go's compatibility for libraries, Dingo's developer experience for applications.

---

**Next Steps**:
- [Getting Started Guide](getting-started.md)
- [Publishing Packages Guide](publishing-packages.md)
- [Migration Guide](migration-guide.md)
