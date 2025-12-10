local M = {}

M.defaults = {
  lsp = {
    enabled = true,
    cmd = { "dingo-lsp" },           -- Standalone binary (not dingo lsp)
    filetypes = { "dingo" },
    root_patterns = { "go.mod", ".git" },
    settings = {},
    log_level = "info",              -- DINGO_LSP_LOG env var
  },

  format = {
    enabled = true,
    on_save = true,
    timeout_ms = 2000,
    cmd = { "dingo", "fmt", "-w" },
  },

  lint = {
    enabled = true,
    on_save = true,
    on_insert_leave = false,
    cmd = { "dingo", "lint", "--json" },
    virtual_text = true,
    signs = true,
  },

  build = {
    cmd = { "dingo", "build", "--no-mascot" },
    run_cmd = { "dingo", "run" },
  },
}

M.options = vim.deepcopy(M.defaults)

function M.setup(opts)
  M.options = vim.tbl_deep_extend("force", M.defaults, opts or {})
end

return M
