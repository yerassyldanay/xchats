import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Separate from vite.config.ts on purpose — the unit suite (src/lib/**/*.test.ts) is
// pure TypeScript logic (no Vue components, no DOM), so it needs neither the Vue
// plugin nor a browser-like environment; keeping it standalone avoids coupling the
// test runner's config to the dev-server-focused settings in vite.config.ts.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
