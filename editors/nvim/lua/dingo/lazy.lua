-- lazy.nvim plugin spec for dingo.nvim
-- This file can be used directly as a lazy.nvim plugin spec
--
-- Usage in your lazy.nvim config:
--   require("lazy").setup({
--     { import = "dingo.lazy" },
--     -- your other plugins...
--   })

return {
  -- Dingo language support
  {
    "MadAppGang/dingo",
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
