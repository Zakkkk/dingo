-- lazy.nvim plugin spec for dingo.nvim
-- This file provides the complete plugin specification for lazy.nvim
--
-- Usage in your lazy.nvim config:
--   require("lazy").setup({
--     {
--       "MadAppGang/dingo",
--       subdir = "editors/nvim",
--       import = "dingo.lazy",  -- Import this spec for keybindings and integration
--     },
--     -- your other plugins...
--   })

return {
  -- Main Dingo plugin configuration
  -- Note: The plugin itself should be specified with subdir in user config
  {
    "MadAppGang/dingo",
    subdir = "editors/nvim",
    name = "dingo.nvim",
    ft = "dingo",
    dependencies = {
      -- Optional but recommended dependencies
      { "williamboman/mason.nvim", optional = true },
      { "nvim-treesitter/nvim-treesitter", optional = true },
    },
    opts = {
      lsp = {
        enabled = true,
        log_level = "info",
      },
      format = {
        enabled = true,
        on_save = true,
      },
      lint = {
        enabled = true,
        on_save = true,
      },
    },
    config = function(_, opts)
      require("dingo").setup(opts)
    end,
    keys = {
      { "<leader>db", "<cmd>DingoBuild<cr>", ft = "dingo", desc = "Build (Dingo)" },
      { "<leader>dr", "<cmd>DingoRun<cr>", ft = "dingo", desc = "Run (Dingo)" },
      { "<leader>df", "<cmd>DingoFormat<cr>", ft = "dingo", desc = "Format (Dingo)" },
      { "<leader>dl", "<cmd>DingoLint<cr>", ft = "dingo", desc = "Lint (Dingo)" },
      { "<leader>dt", "<cmd>DingoTranspile<cr>", ft = "dingo", desc = "Transpile (Dingo)" },
    },
    cmd = {
      "DingoBuild",
      "DingoRun",
      "DingoFormat",
      "DingoLint",
      "DingoLintClear",
      "DingoTranspile",
    },
  },

  -- Mason configuration for gopls (required by dingo-lsp)
  {
    "williamboman/mason.nvim",
    optional = true,
    opts = function(_, opts)
      opts.ensure_installed = opts.ensure_installed or {}
      vim.list_extend(opts.ensure_installed, { "gopls" })
    end,
  },

  -- Tree-sitter configuration for dingo
  {
    "nvim-treesitter/nvim-treesitter",
    optional = true,
    opts = function(_, opts)
      opts.ensure_installed = opts.ensure_installed or {}
      -- Dingo parser will be available after it's published to nvim-treesitter
      -- For now, use local installation via TSInstallFromGrammar
      if type(opts.ensure_installed) == "table" then
        vim.list_extend(opts.ensure_installed, { "go" })
      end
    end,
  },
}
