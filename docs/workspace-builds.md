# Workspace Builds - Multi-Package Support

**Status**: ✅ Implemented (Phase V - Task E)
**Date**: 2025-11-19
**Version**: 1.0

---

## Overview

Dingo supports building multiple packages in a workspace with intelligent dependency resolution, parallel builds, and incremental compilation. This guide covers workspace structure, build commands, and best practices for multi-package projects.

## Table of Contents

1. [Workspace Structure](#workspace-structure)
2. [Build Commands](#build-commands)
3. [Dependency Resolution](#dependency-resolution)
4. [Build Cache](#build-cache)
5. [Parallel Builds](#parallel-builds)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

---

## Workspace Structure

### Workspace Root Detection

Dingo automatically detects workspace root by searching for:
1. `dingo.toml` (Dingo workspace configuration)
2. `go.work` (Go workspace file)
3. `go.mod` (Go module file, as fallback)

**Example workspace structure:**

```
my-project/                    # Workspace root (has dingo.toml or go.mod)
├── dingo.toml                 # Workspace config (optional)
├── go.mod                     # Go module definition
├── cmd/
│   └── myapp/
│       ├── main.dingo         # Application entry point
│       └── main.go            # Generated (auto-transpiled)
├── pkg/
│   ├── models/
│   │   ├── user.dingo
│   │   ├── user.go           # Generated
│   │   └── product.dingo
│   └── services/
│       ├── auth.dingo
│       └── database.dingo
├── internal/
│   └── utils/
│       └── helpers.dingo
└── .dingoignore              # Files/dirs to ignore
```

### .dingoignore File

Control which files and directories are excluded from workspace scans:

```gitignore
# .dingoignore (similar to .gitignore)

# Build outputs
*.exe
*.test

# Directories
vendor/
node_modules/
.git/
.vscode/

# Specific files
*_backup.dingo
tmp/
```

**Default ignores** (always excluded):
- `.git/`
- `.dingo-cache/`
- `node_modules/`
- `vendor/`
- `.idea/`
- `.vscode/`

---

## Build Commands

### Single File Build

Build a specific `.dingo` file:

```bash
dingo build main.dingo
```

This is the existing behavior (unchanged).

### Workspace-Wide Build

Build all `.dingo` files in the workspace:

```bash
dingo build ./...
```

**What happens:**
1. Scans workspace for all `.dingo` files
2. Groups by package (directory)
3. Resolves dependencies between packages
4. Builds in dependency order
5. Skips unchanged files (with `-incremental`)

### Package Pattern Build

Build specific package patterns:

```bash
# Build everything in pkg/ directory
dingo build ./pkg/...

# Build specific package
dingo build ./cmd/myapp

# Build multiple patterns
dingo build ./pkg/... ./internal/...
```

### Build Options

**Available flags:**

```bash
dingo build ./... [options]

Options:
  -parallel       Enable parallel builds (default: true)
  -incremental    Only rebuild changed files (default: true)
  -jobs N         Number of parallel jobs (default: 4)
  -v              Verbose output (show each file)
  -clean          Clean cache before building
```

**Examples:**

```bash
# Fast incremental build (default)
dingo build ./...

# Full rebuild (ignore cache)
dingo build ./... -clean

# Verbose output to see all files
dingo build ./... -v

# Sequential build (no parallelism)
dingo build ./... -parallel=false

# Use 8 parallel jobs
dingo build ./... -jobs=8
```

---

## Dependency Resolution

### How Dependencies Are Detected

Dingo analyzes `import` statements in `.dingo` files to build a dependency graph:

```dingo
// File: pkg/services/auth.dingo
package services

import "myproject/pkg/models"  // Depends on models package

func Authenticate(user: models.User) Result[string, error] {
    // ...
}
```

### Build Order

Packages are built in **dependency order** (topological sort):

1. Packages with no dependencies build first
2. Packages depending on others wait for dependencies
3. Independent packages build in parallel (if `-parallel` enabled)

**Example build order:**

```
pkg/models/        → (builds first, no dependencies)
pkg/utils/         → (builds first, no dependencies)
↓
pkg/services/      → (builds after models, depends on it)
↓
cmd/myapp/         → (builds last, depends on services and models)
```

### Circular Dependency Detection

Dingo detects circular dependencies at build time:

```
Error: Circular dependencies detected:
  pkg/a → pkg/b → pkg/c → pkg/a
```

**Fix:** Refactor to break the cycle (extract shared code to new package).

---

## Build Cache

### How Caching Works

Dingo uses **content-based caching** to skip rebuilding unchanged files:

1. **Hash-based**: Computes SHA-256 hash of source files
2. **Dependency-aware**: Tracks dependencies (rebuilds if deps change)
3. **Fast**: Scans 1000 files in <100ms
4. **Automatic**: Enabled by default with `-incremental`

**Cache location:** `.dingo-cache/` (in workspace root)

### Cache Files

```
.dingo-cache/
├── build-cache.json      # Metadata (hashes, timestamps)
└── ... (future: compiled artifacts)
```

**build-cache.json structure:**

```json
{
  "/path/to/file.dingo": {
    "source_path": "/path/to/file.dingo",
    "output_path": "/path/to/file.go",
    "source_hash": "a3b2c1...",
    "output_hash": "d4e5f6...",
    "last_built": "2025-11-19T15:30:00Z",
    "dependencies": []
  }
}
```

### When Cache Invalidates

File is rebuilt if:
- ✅ Source file content changed (hash differs)
- ✅ Source file modified after last build (timestamp)
- ✅ Output file missing
- ✅ Any dependency changed
- ✅ Cache manually cleared (`-clean`)

### Cache Management

**Clean cache:**

```bash
# Clean and rebuild everything
dingo build ./... -clean
```

**Manual cache cleanup:**

```bash
rm -rf .dingo-cache/
```

**View cache stats** (future feature):

```bash
dingo cache stats
# Output:
# Cache entries: 42
# Total size: 1.2 MB
# Cache directory: .dingo-cache/
```

---

## Parallel Builds

### How Parallelism Works

Dingo builds independent packages **simultaneously** to speed up builds:

**Sequential build** (no parallelism):
```
pkg/a (5s) → pkg/b (5s) → pkg/c (5s) = 15 seconds total
```

**Parallel build** (3 workers):
```
pkg/a (5s) ┐
pkg/b (5s) ├→ All done = 5 seconds total
pkg/c (5s) ┘
```

**Speedup:** 2-4x faster for workspaces with multiple independent packages.

### Dependency Levels

Packages are grouped by **dependency level**:

- **Level 0**: No dependencies (can build immediately)
- **Level 1**: Depends only on Level 0 (waits for Level 0)
- **Level 2**: Depends on Level 0 or 1 (waits for previous levels)
- ...

**Example:**

```
Level 0:  [pkg/models, pkg/utils]         → Build in parallel
Level 1:  [pkg/services]                  → Build after Level 0
Level 2:  [cmd/myapp, cmd/admin]          → Build in parallel after Level 1
```

### Controlling Parallelism

**Number of parallel jobs:**

```bash
# Use 8 workers (good for large workspaces)
dingo build ./... -jobs=8

# Use 1 worker (sequential, easier debugging)
dingo build ./... -jobs=1

# Auto-detect (default: 4)
dingo build ./...
```

**Disable parallelism:**

```bash
dingo build ./... -parallel=false
```

**When to disable:**
- Debugging build issues
- Low-memory environments
- Ensuring deterministic order

---

## Best Practices

### 1. Organize Packages Logically

```
✅ Good structure:
pkg/
├── models/      # Data structures
├── services/    # Business logic (depends on models)
└── api/         # HTTP handlers (depends on services)

❌ Avoid:
pkg/
├── everything.dingo  # Monolithic file
└── random/           # Unclear dependencies
```

### 2. Use .dingoignore

Exclude unnecessary directories from scans:

```gitignore
# .dingoignore
vendor/
node_modules/
tmp/
*.backup.dingo
```

**Benefits:**
- Faster workspace scans
- Avoid errors from generated/vendored code

### 3. Leverage Incremental Builds

```bash
# Development: Fast incremental builds
dingo build ./... -incremental

# CI/Production: Clean builds for safety
dingo build ./... -clean
```

### 4. Use Parallel Builds Wisely

```bash
# Large workspace (many independent packages)
dingo build ./... -jobs=8

# Small workspace or debugging
dingo build ./... -jobs=1
```

### 5. Structure for Parallel Builds

**Maximize parallelism** by minimizing dependencies:

```
✅ Independent packages (parallel-friendly):
cmd/
├── api/          # Independent
├── worker/       # Independent
└── cli/          # Independent

❌ Linear dependencies (sequential):
pkg/a → pkg/b → pkg/c → pkg/d
(Each must wait for previous)
```

### 6. Monitor Build Performance

Use `-v` flag to see bottlenecks:

```bash
dingo build ./... -v

# Output shows timing:
# Building level 1 (3 packages)
#   Building: pkg/models (2.1s)
#   Building: pkg/utils (0.5s)
#   Building: pkg/config (0.3s)
# Building level 2 (1 package)
#   Building: pkg/services (5.2s)  ← Bottleneck
```

**Fix bottlenecks:**
- Split large packages
- Reduce dependencies
- Optimize slow transformations

---

## Troubleshooting

### Build Fails with "Circular Dependencies"

**Error:**
```
Error: Circular dependencies detected:
  pkg/a → pkg/b → pkg/a
```

**Solution:**
1. Identify the cycle from error message
2. Refactor to break cycle:
   - Extract shared code to new package
   - Use interfaces to reverse dependency
   - Combine cyclic packages if tightly coupled

### Cache Not Working (Always Rebuilds)

**Check:**
```bash
# Verify cache exists
ls -la .dingo-cache/

# Check if -incremental is enabled
dingo build ./... -incremental -v
```

**Common causes:**
- File timestamps reset (e.g., after `git clone`)
- Cache directory deleted
- Source files modified outside editor

**Fix:**
```bash
# Clear and rebuild cache
dingo build ./... -clean
```

### Slow Builds Despite Parallel Flag

**Check:**
```bash
# Verify parallelism is active
dingo build ./... -v

# Output should show "Building level X (N packages)"
# with multiple packages per level
```

**Common causes:**
- Linear dependencies (each pkg depends on previous)
- Too few independent packages
- `-jobs=1` or `-parallel=false` set

**Fix:**
- Refactor to reduce dependencies
- Increase `-jobs` value
- Split large packages

### "No Workspace Root Found"

**Error:**
```
Error: No workspace root found (no dingo.toml, go.work, or go.mod)
```

**Solution:**
Create `go.mod` or `dingo.toml`:

```bash
# Create go.mod (recommended)
go mod init myproject

# Or create dingo.toml
echo "# Dingo workspace" > dingo.toml
```

### Import Paths Not Resolved

**Issue:** Dependencies not detected correctly.

**Check:**
1. Import paths match module name in `go.mod`
2. Packages exist in workspace
3. No typos in import statements

**Example:**

```go
// go.mod
module github.com/user/myproject

// File: pkg/services/auth.dingo
import "github.com/user/myproject/pkg/models"  // ✅ Correct

import "pkg/models"                             // ❌ Wrong
```

---

## Future Enhancements

Planned features for workspace builds:

- [ ] **Watch mode**: `dingo build ./... --watch` (auto-rebuild on change)
- [ ] **Smart cache**: Store transpiled AST for faster rebuilds
- [ ] **Distributed builds**: Build across multiple machines
- [ ] **Build profiles**: Different configs for dev/prod
- [ ] **Cache statistics**: `dingo cache stats` command
- [ ] **Dependency visualization**: `dingo graph` (show dep tree)

---

## Summary

**Key Features:**
- ✅ `dingo build ./...` - Build entire workspace
- ✅ Package patterns - Build specific packages
- ✅ Dependency resolution - Automatic build order
- ✅ Parallel builds - 2-4x faster for independent packages
- ✅ Incremental builds - Skip unchanged files
- ✅ Build cache - Hash-based caching
- ✅ Circular dependency detection
- ✅ `.dingoignore` support

**Quick Reference:**

```bash
# Common commands
dingo build ./...                # Build everything
dingo build ./pkg/...            # Build package pattern
dingo build ./... -v             # Verbose output
dingo build ./... -clean         # Clean rebuild
dingo build ./... -jobs=8        # 8 parallel jobs
```

**Best Practices:**
1. Organize packages logically
2. Use `.dingoignore` for exclusions
3. Enable incremental builds for development
4. Use parallel builds for large workspaces
5. Monitor performance with `-v` flag

For more details, see:
- [Package Management Guide](./package-management.md)
- [Getting Started Guide](./getting-started.md)
- [Feature Documentation](./features/README.md)

---

**Last Updated**: 2025-11-19
**Version**: 1.0
**Status**: ✅ Complete
