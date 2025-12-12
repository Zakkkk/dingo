# Neovim Plugin Implementation Review - Dingo Language

**Review Date**: December 10, 2025
**Reviewer**: Internal Code Reviewer
**Project**: Dingo transpiler Neovim plugin
**Files Reviewed**: 11 files (8 Lua modules + 1 grammar.js + 3 query files)

---

## ✅ Strengths

### 1. **Clean Architecture**
- Modular design with separated concerns (config, lsp, format, lint, commands, health, treesitter)
- Proper module pattern usage throughout (`local M = {}` pattern)
- Good use of dependency injection via config module

### 2. **Neovim 0.8+ API Usage**
- Correctly uses `vim.system` instead of deprecated `vim.fn.jobstart`
- Uses `vim.api.nvim_create_autocmd` and `vim.api.nvim_create_user_command`
- Proper use of `vim.fs.find` for root pattern detection
- Appropriate use of `vim.schedule` for async callbacks

### 3. **Comprehensive Feature Coverage**
- Full LSP integration with proper environment variable handling
- Asynchronous linting with JSON parsing and diagnostics display
- Format on save with timeout handling
- Build/run commands with terminal integration
- Health check system for dependency validation
- Tree-sitter parser registration with nvim-treesitter

### 4. **Tree-sitter Grammar Quality**
- Complete 530-line grammar covering all Dingo features
- Proper precedence handling for operators
- Conflicting patterns (lambda vs binary, generics vs comparison) are declared in `conflicts` array
- Extends Go grammar correctly with Dingo extensions (enum, match, lambdas, ?, ?., ??)

### 5. **Good Error Handling Practices**
- Uses `pcall` for parsing JSON in lint results
- Graceful handling of missing dependencies (treesitter, CLI tools)
- Buffer validation checks before operations

---

## ⚠️ Concerns

### CRITICAL Issues

#### 1. **Command Injection Vulnerability - commands.lua:15, 34**
**File**: `lua/dingo/commands.lua` (lines 15, 34)
**Issue**: Commands are built using `table.concat(args, " ")` without shell escaping. This is unsafe with filenames containing spaces or special characters.

**Impact**: Critical security vulnerability; malicious filenames could execute arbitrary shell commands.

**Example Problem**:
```lua
-- Current (unsafe):
vim.cmd("split | terminal " .. table.concat(args, " "))
-- If filename is: "test; rm -rf /"
-- Results in: terminal test; rm -rf /  # Oops!
```

**Recommendation**:
```lua
-- Safe approach: escape each argument
local function term_cmd(args)
  local parts = {}
  for _, arg in ipairs(args) do
    table.insert(parts, vim.fn.shellescape(arg))
  end
  vim.cmd("split | terminal " .. table.concat(parts, " "))
end

-- Or use vim.system directly:
vim.system(args, { stdin = "null", stdout = "buffer", stderr = "buffer" }, function(result)
  -- Handle result
end)
```

#### 2. **Potential Panic in LSP Root Finding - lsp.lua:7**
**File**: `lua/dingo/lsp.lua` (line 7)
**Issue**: `vim.fs.find()` returns an array. Accessing `[1]` without checking if the array is empty can cause nil dereference.

**Impact**: May crash Neovim when no root files are found.

**Recommendation**:
```lua
-- Current:
local root = vim.fs.dirname(vim.fs.find(root_files, {
  path = fname,
  upward = true,
})[1])

-- Fixed:
local found = vim.fs.find(root_files, { path = fname, upward = true })
local root = found[1] and vim.fs.dirname(found[1]) or vim.fs.dirname(fname)
```

### IMPORTANT Issues

#### 3. **Error Output Not Displayed - lint.lua:61**
**File**: `lua/dingo/lint.lua` (line 61)
**Issue**: When lint command fails, diagnostics are cleared but stderr is not shown to user.

**Impact**: Users see no feedback when linter fails; difficult to debug configuration issues.

**Recommendation**:
```lua
-- In the vim.system callback:
if result.code ~= 0 then
  vim.diagnostic.set(ns, bufnr, {})
  if result.stderr then
    vim.notify("dingo lint failed: " .. result.stderr, vim.log.levels.ERROR)
  end
end
```

#### 4. **Dingo CLI Semantics Not Aligned - commands.lua:10**
**File**: `lua/dingo/commands.lua` (lines 10-12)
**Issue**: Build command is called `opts.file` but doesn't use `opts.file` in the build command. `dingo build` with a file argument should build that file, but current code passes file arguments incorrectly.

**Impact**: `:DingoBuild!` may not work as expected.

**Recommendation**: Clarify semantics - does `dingo build` work on single files or should it use `dingo run`?

#### 5. **Parser Installation Path Incorrect - treesitter.lua:15**
**File**: `lua/dingo/treesitter.lua` (line 15)
**Issue**: Parser path points to `src/parser.c` but tree-sitter grammars compile to `src/parser.c` and `src/scanner.c`. The actual installed location varies.

**Impact**: `:TSInstall dingo` would fail.

**Recommendation**:
```lua
-- For grammar-based installation:
install_info = {
  url = "https://github.com/MadAppGang/dingo",
  files = {
    "editors/nvim/tree-sitter-dingo/src/parser.c",
    "editors/nvim/tree-sitter-dingo/src/scanner.c",
  },
  generate_requires_npm = true,
}

-- Or recommend manual installation:
" To install parser, run: :TSInstallFromGrammar dingo"
```

#### 6. **Empty Filename Handling - format.lua:11**
**File**: `lua/dingo/format.lua` (lines 11-15)
**Issue**: Warns but doesn't return early, continuing to execute format command.

**Impact**: Warns about missing filename but still tries to format, may show confusing error messages.

**Recommendation**:
```lua
if filename == "" then
  vim.notify("dingo: Buffer has no filename", vim.log.levels.WARN)
  return  -- Add return here
end
```

#### 7. **Missing Timeout Handling - format.lua:25**
**File**: `lua/dingo/format.lua` (line 25)
**Issue**: If formatter times out, result.code is nil and result.stderr is nil, potentially causing confusing behavior.

**Impact**: Timeout errors are not handled gracefully.

**Recommendation**:
```lua
if result.code == nil then
  vim.notify("dingo fmt timed out after " .. config.options.format.timeout_ms .. "ms", vim.log.levels.ERROR)
  return
end

if result.code == 0 then
  -- success
else
  vim.notify("dingo fmt failed: " .. (result.stderr or "exit code " .. result.code), vim.log.levels.ERROR)
end
```

### MINOR Issues

#### 8. **Filename Concatenation Without Extension Check - commands.lua:44**
**File**: `lua/dingo/commands.lua` (line 44)
**Issue**: Assumes filename ends with `.dingo` when generating Go filename.

**Impact**: Would fail on files like `main` without extension.

**Recommendation**:
```lua
local go_file = filename:gsub("%.dingo$", ".go")
if go_file == filename then
  go_file = filename .. ".go"  -- Add .go if no .dingo extension
end
```

#### 9. **Import Path Capture Missing - highlights.scm:137**
**File**: `queries/dingo/highlights.scm` (line 137)
**Issue**: Import path highlighting uses `path:` as field name, but grammar uses `path` without colon.

**Impact**: Import paths won't be highlighted as `@string.special`.

**Recommendation**:
```diff
- (import_spec path: (interpreted_string_literal) @string.special)
+ (import_spec (path) (interpreted_string_literal) @string.special)
```

Or adjust grammar to use field:
```diff
import_spec: $ => seq(
  optional(field('alias', $.identifier)),
  field('path', $.interpreted_string_literal),
),
```

#### 10. **Inconsistent Tree-sitter Module Export**
**File**: `lua/dingo/treesitter.lua`
**Issue**: Module exports a function `register()` but other modules use `setup()`. Inconsistent naming.

**Impact**: Minor maintainability issue.

**Recommendation**: Rename `register()` to `setup()` for consistency.

---

## 🔍 Questions

1. **Terminal vs Job API** (commands.lua): Is using `vim.cmd("split | terminal ...")` the best approach? `vim.system` with stdout/stderr capture provides better control.

2. **Lint on Insert Leave** (lint.lua:87): The config has `on_insert_leave = false` but no implementation. Is this intended for future use?

3. **Health Check API**: Are we sure `vim.health.start()` works in all Neovim 0.8+ versions? Should we check for deprecation warnings?

4. **Treesitter Parser Location** (treesitter.lua:15): What's the actual installation location after `:TSInstall`? The path may need to be `editors/nvim/tree-sitter-dingo/src/`.

5. **Buffer Reload** (format.lua:29): Does `vim.cmd("edit")` properly reload the buffer in all Neovim versions? Sometimes `vim.cmd("checktime")` is needed.

6. **Build Command Semantics** (commands.lua): What should `:DingoBuild!` do? Current implementation passes file to `dingo build`, but maybe it should use `dingo run` instead?

---

## 📊 Summary

### Overall Assessment: **CHANGES_NEEDED**

**Priority Ranking**:
1. **CRITICAL**: Fix command injection vulnerability (commands.lua:15, 34)
2. **CRITICAL**: Fix potential nil dereference in LSP root finding (lsp.lua:7)
3. **IMPORTANT**: Display error output on lint failures (lint.lua:61)
4. **IMPORTANT**: Fix parser installation paths (treesitter.lua:15)
5. **IMPORTANT**: Fix empty filename handling (format.lua:12)
6. **IMPORTANT**: Add timeout handling for format command (format.lua:25)
7. **MINOR**: Fix import path highlighting (highlights.scm:137)
8. **MINOR**: Fix Go filename generation (commands.lua:44)
9. **MINOR**: Standardize module interface (treesitter.lua:register → setup)
10. **MINOR**: Clarify build command semantics (commands.lua:10)

### Testability Score: **HIGH**
- Each module can be tested independently
- Async operations use proper callbacks
- Configuration is externalized and overridable
- Health checks provide diagnostic information

### Code Quality Score: **GOOD** (8/10)
**Strengths**:
- Clean separation of concerns
- Proper Neovim 0.8+ API usage
- Good async handling
- Comprehensive feature coverage

**Areas for Improvement**:
- Security: command injection vulnerability
- Error handling: incomplete in some async flows
- Edge cases: empty filenames, timeouts, missing files

### Recommendations

1. **Immediate Action Required**:
   - Fix command injection in commands.lua
   - Fix nil dereference in lsp.lua
   - Add error display in lint.lua

2. **Before Release**:
   - Fix treesitter parser installation paths
   - Add timeout handling
   - Fix empty filename handling
   - Test build/run commands thoroughly

3. **Future Enhancements**:
   - Add `on_insert_leave` linting option implementation
   - Consider using `vim.system` for all commands (better error handling)
   - Add user-configurable keybindings for all commands
   - Add completion support for commands

### Integration Notes

The plugin integrates well with the Dingo ecosystem:
- ✅ Uses correct CLI commands (dingo fmt, lint, build, run)
- ✅ Passes proper flags (--no-mascot, --json, -w)
- ✅ LSP binary name correct (dingo-lsp)
- ✅ Environment variable handling (DINGO_LSP_LOG)
- ✅ File extension handling (*.dingo)

**Ready for release after critical security fix and error handling improvements.**