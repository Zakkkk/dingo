-- Dingo filetype settings (matches Go conventions + Dingo specifics)
vim.bo.commentstring = "// %s"
vim.bo.comments = "s1:/*,mb:*,ex:*/,://"
vim.bo.tabstop = 4
vim.bo.shiftwidth = 4
vim.bo.expandtab = false  -- Use tabs like Go
vim.bo.smartindent = true

-- Set up tree-sitter if available
if vim.treesitter.language.require_language then
  pcall(vim.treesitter.language.require_language, "dingo")
end
