local M = {}
local config = require("dingo.config")

local ns = vim.api.nvim_create_namespace("dingo_lint")

local severity_map = {
  hint = vim.diagnostic.severity.HINT,
  info = vim.diagnostic.severity.INFO,
  warning = vim.diagnostic.severity.WARN,
  error = vim.diagnostic.severity.ERROR,
}

local function parse_diagnostics(json_str, bufnr)
  local ok, data = pcall(vim.json.decode, json_str)
  if not ok or type(data) ~= "table" then
    return {}
  end

  local diagnostics = {}
  local bufname = vim.api.nvim_buf_get_name(bufnr)

  for _, item in ipairs(data) do
    if item.file == bufname or vim.fn.fnamemodify(item.file, ":p") == bufname then
      table.insert(diagnostics, {
        lnum = (item.line or 1) - 1,
        col = (item.column or 1) - 1,
        end_lnum = (item.end_line or item.line or 1) - 1,
        end_col = (item.end_column or item.column or 1) - 1,
        severity = severity_map[item.severity] or vim.diagnostic.severity.WARN,
        message = item.message or "",
        source = "dingo-lint",
        code = item.code,
      })
    end
  end

  return diagnostics
end

function M.lint(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()

  if not config.options.lint.enabled then
    return
  end

  local filename = vim.api.nvim_buf_get_name(bufnr)
  if filename == "" then
    return
  end

  local cmd = vim.list_extend({}, config.options.lint.cmd)
  table.insert(cmd, filename)

  vim.system(cmd, { text = true }, function(result)
    vim.schedule(function()
      if not vim.api.nvim_buf_is_valid(bufnr) then
        return
      end

      if result.code == 0 and result.stdout then
        local diagnostics = parse_diagnostics(result.stdout, bufnr)
        vim.diagnostic.set(ns, bufnr, diagnostics)
      else
        vim.diagnostic.set(ns, bufnr, {})

        -- Notify user of linter error
        if result.stderr and result.stderr ~= "" then
          vim.notify(
            string.format("dingo lint failed: %s", result.stderr),
            vim.log.levels.ERROR
          )
        elseif result.code ~= 0 then
          vim.notify(
            string.format("dingo lint exited with code %d", result.code),
            vim.log.levels.ERROR
          )
        end
      end
    end)
  end)
end

function M.clear(bufnr)
  bufnr = bufnr or vim.api.nvim_get_current_buf()
  vim.diagnostic.set(ns, bufnr, {})
end

function M.setup()
  if not config.options.lint.enabled then
    return
  end

  -- Configure diagnostic display based on user options
  vim.diagnostic.config({
    virtual_text = config.options.lint.virtual_text,
    signs = config.options.lint.signs,
  }, ns)

  vim.api.nvim_create_user_command("DingoLint", function()
    M.lint()
  end, { desc = "Lint current Dingo buffer" })

  vim.api.nvim_create_user_command("DingoLintClear", function()
    M.clear()
  end, { desc = "Clear Dingo lint diagnostics" })

  local group = vim.api.nvim_create_augroup("DingoLint", { clear = true })

  if config.options.lint.on_save then
    vim.api.nvim_create_autocmd("BufWritePost", {
      pattern = "*.dingo",
      callback = function(args)
        M.lint(args.buf)
      end,
      group = group,
    })
  end

  -- Wire up on_insert_leave option
  if config.options.lint.on_insert_leave then
    vim.api.nvim_create_autocmd("InsertLeave", {
      pattern = "*.dingo",
      callback = function(args)
        M.lint(args.buf)
      end,
      group = group,
    })
  end

  -- Initial lint on buffer open
  vim.api.nvim_create_autocmd("BufReadPost", {
    pattern = "*.dingo",
    callback = function(args)
      vim.defer_fn(function()
        if vim.api.nvim_buf_is_valid(args.buf) then
          M.lint(args.buf)
        end
      end, 100)
    end,
    group = group,
  })
end

return M
