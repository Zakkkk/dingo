# Releasing dingo.nvim

This document describes how to release new versions of the Neovim plugin.

## Distribution Model

The dingo.nvim plugin is distributed as part of the main Dingo repository at `editors/nvim/`. Users can install it in two ways:

### 1. From GitHub (Recommended for Users)

```lua
-- lazy.nvim with subdir
{
  "MadAppGang/dingo",
  subdir = "editors/nvim",
  ft = "dingo",
  config = function()
    require("dingo").setup()
  end,
}
```

### 2. Local Development

```lua
-- For contributors/testing
{
  dir = "~/path/to/dingo/editors/nvim",
  ft = "dingo",
  config = function()
    require("dingo").setup()
  end,
}
```

## Release Checklist

### Before Release

1. **Update version** in `pkg/version/version.go`:
   ```go
   const Version = "0.X.Y"
   ```

2. **Update CHANGELOG.md**:
   - Move items from `[Unreleased]` to new version section
   - Add release date
   - Update comparison links at bottom

3. **Test the plugin**:
   ```bash
   # Test tree-sitter grammar
   cd editors/nvim/tree-sitter-dingo
   npm install
   npm test

   # Test in Neovim
   nvim --clean -u test_init.lua some_file.dingo
   ```

4. **Verify health checks**:
   ```vim
   :checkhealth dingo
   ```

5. **Rebuild tree-sitter parser** (if grammar changed):
   ```bash
   cd editors/nvim/tree-sitter-dingo
   npm run build
   ```

### Release Steps

1. **Commit all changes**:
   ```bash
   git add .
   git commit -m "chore(nvim): prepare release v0.X.Y"
   ```

2. **Create and push tag**:
   ```bash
   git tag v0.X.Y
   git push origin main
   git push origin v0.X.Y
   ```

3. **Create GitHub Release** (optional but recommended):
   - Go to https://github.com/MadAppGang/dingo/releases
   - Click "Draft a new release"
   - Select the tag
   - Copy relevant CHANGELOG section to release notes
   - Publish

### Post-Release

1. **Verify installation works**:
   ```lua
   -- Test with fresh install
   { "MadAppGang/dingo", subdir = "editors/nvim", tag = "v0.X.Y" }
   ```

2. **Update CHANGELOG.md** with `[Unreleased]` section for next cycle

## Tree-sitter Parser Updates

If the grammar changes:

1. **Regenerate parser**:
   ```bash
   cd editors/nvim/tree-sitter-dingo
   npm install
   npx tree-sitter generate
   npm run build
   ```

2. **Test parsing**:
   ```bash
   npx tree-sitter parse ../../examples/01_error_propagation/http_handler.dingo
   ```

3. **Rebuild .so file**:
   ```bash
   # The .so is needed for local testing
   cc -shared -o dingo.so -fPIC src/parser.c -I src
   ```

4. **Commit generated files**:
   ```bash
   git add src/parser.c src/tree_sitter/parser.h dingo.so
   git commit -m "chore(tree-sitter): regenerate parser"
   ```

## nvim-treesitter Registry (Future)

To get `dingo` into the official nvim-treesitter registry:

1. **Ensure grammar is stable** and well-tested

2. **Create PR to nvim-treesitter**:
   - Fork https://github.com/nvim-treesitter/nvim-treesitter
   - Add dingo to `lua/nvim-treesitter/parsers.lua`
   - Add query files to `queries/dingo/`
   - Submit PR

3. **Parser config format**:
   ```lua
   dingo = {
     install_info = {
       url = "https://github.com/MadAppGang/dingo",
       location = "editors/nvim/tree-sitter-dingo",
       files = { "src/parser.c" },
       branch = "main",
     },
     filetype = "dingo",
     maintainers = { "@YourGitHubUsername" },
   }
   ```

4. **Required queries** (copy from `editors/nvim/queries/dingo/`):
   - `highlights.scm` - Syntax highlighting
   - `folds.scm` - Code folding
   - `indents.scm` - Auto-indentation

## Version Numbering

Follow semver aligned with Dingo CLI:

- **MAJOR**: Breaking changes to plugin API or config
- **MINOR**: New features (commands, LSP capabilities)
- **PATCH**: Bug fixes, query improvements

The nvim plugin version matches Dingo CLI version for simplicity.

## Testing Configurations

### Minimal test config (`test_init.lua`)

```lua
vim.opt.rtp:prepend("~/path/to/dingo/editors/nvim")

require("dingo").setup({
  lsp = { enabled = true, log_level = "debug" },
  format = { on_save = true },
  lint = { on_save = true },
})
```

### Test with:
```bash
nvim --clean -u test_init.lua examples/01_error_propagation/http_handler.dingo
```
