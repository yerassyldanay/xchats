import { defineConfig } from 'vite'

// Used only by the blog e2e suite's webServer (see playwright.blog.config.ts).
// appType: 'mpa' turns off Vite's SPA history-fallback middleware, so a
// request for a path with no matching file gets a real 404 instead of
// index.html — matching what nginx.conf actually does in production for
// /blog/* (see its dedicated locations). The main vite.config.ts deliberately
// keeps the default 'spa' behavior: the authenticated app's client-side router
// depends on that fallback for every other route, in both dev and preview.
export default defineConfig({
  appType: 'mpa',
  build: { outDir: 'dist' },
  preview: { port: 5173 },
})
