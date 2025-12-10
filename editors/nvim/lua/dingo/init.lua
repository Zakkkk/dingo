local M = {}

function M.setup(opts)
  local config = require("dingo.config")
  config.setup(opts)

  require("dingo.lsp").setup()
  require("dingo.format").setup()
  require("dingo.lint").setup()
  require("dingo.commands").setup()

  -- Attempt treesitter registration
  local ts_ok = require("dingo.treesitter").register()
  if not ts_ok then
    vim.notify(
      "dingo.nvim: nvim-treesitter not found, tree-sitter features disabled",
      vim.log.levels.INFO
    )
  end
end

return M
