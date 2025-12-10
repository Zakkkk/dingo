local M = {}

-- Register dingo parser with nvim-treesitter
function M.register()
  local ok, parsers = pcall(require, "nvim-treesitter.parsers")
  if not ok then
    return false
  end

  local parser_config = parsers.get_parser_configs()

  parser_config.dingo = {
    install_info = {
      url = "https://github.com/MadAppGang/dingo",
      files = { "editors/nvim/tree-sitter-dingo/src/parser.c" },
      branch = "main",
      generate_requires_npm = true,
    },
    filetype = "dingo",
    maintainers = { "@MadAppGang" },
  }

  -- For local development, use:
  -- :TSInstallFromGrammar dingo

  return true
end

return M
