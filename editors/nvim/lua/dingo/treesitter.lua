local M = {}

-- Register dingo parser with nvim-treesitter
function M.register()
  -- Try new nvim-treesitter API first (main branch)
  local ok, parsers = pcall(require, "nvim-treesitter.parsers")
  if ok and parsers.dingo == nil then
    -- New API: directly assign to parsers table
    parsers.dingo = {
      install_info = {
        url = "https://github.com/MadAppGang/dingo",
        location = "editors/nvim/tree-sitter-dingo",
        files = { "src/parser.c" },
        branch = "main",
        generate_requires_npm = true,
      },
      filetype = "dingo",
      maintainers = { "@MadAppGang" },
    }
  end

  -- Try legacy API (master branch)
  if ok and type(parsers.get_parser_configs) == "function" then
    local parser_config = parsers.get_parser_configs()
    if parser_config.dingo == nil then
      parser_config.dingo = {
        install_info = {
          url = "https://github.com/MadAppGang/dingo",
          location = "editors/nvim/tree-sitter-dingo",
          files = { "src/parser.c" },
          branch = "main",
          generate_requires_npm = true,
        },
        filetype = "dingo",
        maintainers = { "@MadAppGang" },
      }
    end
  end

  -- Register language mapping (parser name -> filetype)
  pcall(vim.treesitter.language.register, "dingo", "dingo")

  return ok
end

return M
