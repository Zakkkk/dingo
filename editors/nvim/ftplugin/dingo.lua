-- Dingo filetype settings (matches Go conventions + Dingo specifics)
vim.bo.commentstring = "// %s"
vim.bo.comments = "s1:/*,mb:*,ex:*/,://"
vim.bo.tabstop = 4
vim.bo.shiftwidth = 4
vim.bo.expandtab = false  -- Use tabs like Go
vim.bo.smartindent = true

-- Use Dingo tree-sitter for syntax highlighting
-- Falls back to Go parser if Dingo parser not available
local ok = pcall(function()
  vim.treesitter.start(0, "dingo")
end)
if not ok then
  pcall(function()
    vim.treesitter.start(0, "go")
  end)
end
