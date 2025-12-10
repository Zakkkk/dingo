local M = {}

function M.check()
  local health = vim.health

  health.start("dingo.nvim")

  -- Check Neovim version
  if vim.fn.has("nvim-0.8") == 1 then
    health.ok("Neovim version >= 0.8")
  else
    health.error("Neovim 0.8+ required")
  end

  -- Check dingo CLI
  if vim.fn.executable("dingo") == 1 then
    local result = vim.system({ "dingo", "--version" }, { text = true }):wait()
    if result.code == 0 then
      health.ok("dingo executable found: " .. vim.trim(result.stdout or ""))
    else
      health.warn("dingo found but --version failed: " .. (result.stderr or ""))
    end
  else
    health.error("dingo executable not found", {
      "Install: go install github.com/MadAppGang/dingo/cmd/dingo@latest",
    })
  end

  -- Check dingo-lsp binary
  if vim.fn.executable("dingo-lsp") == 1 then
    health.ok("dingo-lsp executable found")
  else
    health.error("dingo-lsp executable not found", {
      "Build: go build -o $GOPATH/bin/dingo-lsp ./cmd/dingo-lsp",
    })
  end

  -- Check gopls (required by dingo-lsp)
  if vim.fn.executable("gopls") == 1 then
    health.ok("gopls found (required by dingo-lsp)")
  else
    health.error("gopls not found", {
      "Install: go install golang.org/x/tools/gopls@latest",
    })
  end

  -- Check tree-sitter parser
  local ok, parsers = pcall(require, "nvim-treesitter.parsers")
  if ok and parsers.has_parser("dingo") then
    health.ok("Tree-sitter dingo parser installed")
  else
    health.info("Tree-sitter dingo parser not installed", {
      "Run :TSInstall dingo (after registering parser)",
    })
  end
end

return M
