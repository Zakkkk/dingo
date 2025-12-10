local M = {}
local config = require("dingo.config")

function M.format(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()

  if not config.options.format.enabled then
    return
  end

  local filename = vim.api.nvim_buf_get_name(bufnr)
  if filename == "" then
    vim.notify("dingo: Buffer has no filename", vim.log.levels.WARN)
    return
  end

  -- Save cursor position and view
  local view = vim.fn.winsaveview()

  -- Get buffer content
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
  local content = table.concat(lines, "\n")

  -- Run formatter on stdin/stdout instead of file
  local result = vim.system(
    { "dingo", "fmt" },
    {
      stdin = content,
      text = true,
      timeout = config.options.format.timeout_ms
    }
  ):wait()

  if result.code == nil then
    vim.notify(
      string.format("dingo fmt timed out after %dms", config.options.format.timeout_ms),
      vim.log.levels.ERROR
    )
  elseif result.code == 0 and result.stdout then
    -- Replace buffer content (preserves undo history)
    local new_lines = vim.split(result.stdout, "\n", { plain = true })
    vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, new_lines)

    -- Restore view (handles cursor position safely)
    pcall(vim.fn.winrestview, view)
  else
    vim.notify(
      string.format("dingo fmt failed: %s", result.stderr or "unknown error"),
      vim.log.levels.ERROR
    )
  end
end

function M.setup()
  if not config.options.format.enabled then
    return
  end

  vim.api.nvim_create_user_command("DingoFormat", function()
    M.format()
  end, { desc = "Format current Dingo buffer" })

  if config.options.format.on_save then
    vim.api.nvim_create_autocmd("BufWritePre", {
      pattern = "*.dingo",
      callback = function(args)
        M.format(args.buf)
      end,
      group = vim.api.nvim_create_augroup("DingoFormat", { clear = true }),
    })
  end
end

return M
