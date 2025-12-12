# Dingo Neovim plugin review (2025-12-10)

## ✅ Strengths
- Modular Lua modules mirror the Dingo toolchain responsibilities: configuration, LSP, formatting, linting, commands, health checks, and treesitter registration are cleanly separated.
- The tree-sitter grammar covers all Dingo-specific syntax (let bindings, enums, match, lambdas, error propagation, safe navigation, null coalescing) and the queries align with the grammar to highlight these constructs.
- Commands, formatters, and lint integrations are documented in `editors/nvim/README.md`, which makes onboarding straightforward.

## ⚠️ Concerns

### CRITICAL
1. **Incorrect root detection (maintainability & correctness)** – `editors/nvim/lua/dingo/lsp.lua` lines 4‑12 call `vim.fs.find(root_files, { path = fname, upward = true })[1]` without checking whether any root file was found. When working outside a `go.mod`/`.git` tree the `find` call returns `nil`, and `vim.fs.dirname(nil)` raises an error so the LSP callback never completes. A simple nil check (e.g., store the result in a local variable, guard it, and fall back to `vim.fs.dirname(fname)` when empty) prevents the hard crash and keeps the language server running in single-file contexts.

### IMPORTANT
1. **Lint configuration options ignored** – `editors/nvim/lua/dingo/lint.lua` exposes `on_insert_leave`, `virtual_text`, and `signs` in `config.options.lint`, but none of these values are used. Diagnostics are always emitted on `BufWritePost` and `BufReadPost`, and `vim.diagnostic.set` is called with the default display settings, so disabling virtual text or enabling `on_insert_leave` has no effect. Respect the configuration by wiring `virtual_text`/`signs` into `vim.diagnostic.config` or by toggling them in `vim.schedule_wrap`, and conditionally register the `InsertLeave` autocmd when `on_insert_leave` is true. Otherwise the plugin misleads users who expect these knobs to work.

### MINOR
1. **Shell quoting missing in build/run commands** – In `editors/nvim/lua/dingo/commands.lua` lines 4‑35 both `M.build` and `M.run` construct the terminal command with `table.concat(args, " ")` and feed it to `vim.cmd("split | terminal " .. ...)`. Filenames containing spaces or shell-sensitive characters will break (`foo bar.dingo` becomes two arguments) and open a command-injection vector if the user can control the file name. Use `vim.fn.fnameescape`, `vim.fn.shellescape`, or `vim.fn.termopen(args)` so each argument remains a single entry, and avoid concatenating unescaped strings.

## 🔍 Questions
- Should the lint module expose `vim.diagnostic.config()` to honor `virtual_text` and `signs` fully, or at least offer a way to disable the default display per-buffer?
- Would it make sense to keep a single LSP client per root directory instead of starting one per buffer so the plugin plays nicer with multiple files in the same project?

## 📊 Summary
- Status: **CHANGES_NEEDED** (CRITICAL: 1 | IMPORTANT: 1 | MINOR: 1)
- Priority ranking: Fix the root detection bug first; then make configuration options for linting meaningful; finally, quote the build/run commands before release.
- Testability: **Medium** – Lua modules are easy to exercise manually, but many paths depend on external `dingo`/`dingo-lsp` binaries and the Neovim runtime; the logic in `format.lua`, `lint.lua`, and `commands.lua` can be tested via `vim.loop`/mocked `vim.system` calls with stubbed results.
