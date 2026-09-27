return {
  {
    "LazyVim/LazyVim",
    init = function()
      local group = vim.api.nvim_create_augroup("AidMarkdownDiagnostics", { clear = true })
      vim.api.nvim_create_autocmd("LspAttach", {
        group = group,
        callback = function(args)
          local bufnr = args.buf
          if vim.bo[bufnr].filetype == "markdown" then
            vim.schedule(function()
              vim.diagnostic.enable(false, { bufnr = bufnr })
            end)
          end
        end,
      })
    end,
  },
}
