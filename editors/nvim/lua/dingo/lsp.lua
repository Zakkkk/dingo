local M = {}
local config = require("dingo.config")

local function find_root(fname)
  -- Use vim.fs.find for Neovim 0.8+
  local root_files = config.options.lsp.root_patterns
  local found = vim.fs.find(root_files, {
    path = fname,
    upward = true,
  })

  if #found > 0 then
    return vim.fs.dirname(found[1])
  end

  -- Fallback: use file's directory for single files
  return vim.fs.dirname(fname)
end

function M.setup()
  if not config.options.lsp.enabled then
    return
  end

  -- Use built-in LSP client (no lspconfig dependency required)
  vim.api.nvim_create_autocmd("FileType", {
    pattern = "dingo",
    callback = function(args)
      local bufnr = args.buf
      local bufname = vim.api.nvim_buf_get_name(bufnr)
      local root = find_root(bufname)

      -- Check if client already exists for this buffer
      local clients = vim.lsp.get_clients({ bufnr = bufnr, name = "dingo-lsp" })
      if #clients > 0 then
        return
      end

      -- Check by root directory to avoid duplicates
      local all_clients = vim.lsp.get_clients({ name = "dingo-lsp" })
      for _, client in ipairs(all_clients) do
        if client.config.root_dir == root then
          vim.lsp.buf_attach_client(bufnr, client.id)
          return
        end
      end

      -- No existing client, start new one
      vim.lsp.start({
        name = "dingo-lsp",
        cmd = config.options.lsp.cmd,
        root_dir = root,
        settings = config.options.lsp.settings,
        -- Pass log level via environment
        cmd_env = {
          DINGO_LSP_LOG = config.options.lsp.log_level,
        },
      })
    end,
    group = vim.api.nvim_create_augroup("DingoLsp", { clear = true }),
  })
end

-- Optional: lspconfig integration for users who prefer it
M.lspconfig = {
  default_config = {
    cmd = { "dingo-lsp" },
    filetypes = { "dingo" },
    root_dir = function(fname)
      local util = require("lspconfig.util")
      return util.root_pattern("go.mod", ".git")(fname)
    end,
    single_file_support = true,
  },
  docs = {
    description = [[
Dingo Language Server - LSP proxy for the Dingo meta-language for Go.
Wraps gopls and translates positions via source maps.
https://github.com/MadAppGang/dingo
]],
  },
}

return M
