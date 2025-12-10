local M = {}
local config = require("dingo.config")

-- Build current file or project
function M.build(opts)
  opts = opts or {}
  local args = vim.list_extend({}, config.options.build.cmd)

  -- Add current file if specified
  if opts.file then
    table.insert(args, vim.api.nvim_buf_get_name(0))
  end

  -- Escape each argument for shell safety
  local escaped_args = vim.tbl_map(vim.fn.shellescape, args)
  vim.cmd("split | terminal " .. table.concat(escaped_args, " "))
end

-- Run current file
function M.run(opts)
  opts = opts or {}
  local args = vim.list_extend({}, config.options.build.run_cmd)
  local filename = vim.api.nvim_buf_get_name(0)

  if filename ~= "" then
    table.insert(args, filename)
  end

  -- Add any extra arguments
  if opts.args then
    vim.list_extend(args, opts.args)
  end

  -- Escape each argument for shell safety
  local escaped_args = vim.tbl_map(vim.fn.shellescape, args)
  vim.cmd("split | terminal " .. table.concat(escaped_args, " "))
end

-- Transpile to Go (show output)
function M.transpile()
  local filename = vim.api.nvim_buf_get_name(0)

  -- Validate .dingo extension
  if not filename:match("%.dingo$") then
    vim.notify("Current file is not a .dingo file", vim.log.levels.ERROR)
    return
  end

  local result = vim.system({ "dingo", "go", filename }, { text = true }):wait()

  if result.code == 0 then
    -- Open the generated .go file
    local go_file = filename:gsub("%.dingo$", ".go")
    if vim.fn.filereadable(go_file) == 1 then
      vim.cmd("vsplit " .. vim.fn.fnameescape(go_file))
    end
  else
    vim.notify("Transpile failed: " .. (result.stderr or ""), vim.log.levels.ERROR)
  end
end

function M.setup()
  vim.api.nvim_create_user_command("DingoBuild", function(opts)
    M.build({ file = opts.bang })
  end, {
    desc = "Build Dingo project (! for current file)",
    bang = true,
  })

  vim.api.nvim_create_user_command("DingoRun", function(opts)
    M.run({ args = opts.fargs })
  end, {
    desc = "Run current Dingo file",
    nargs = "*",
  })

  vim.api.nvim_create_user_command("DingoTranspile", function()
    M.transpile()
  end, { desc = "Transpile and open generated Go file" })
end

return M
