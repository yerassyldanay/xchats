import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// Three projects, opted into by filename suffix:
//   - unit: pure TypeScript logic (no Vue components, no DOM) — unchanged
//     from before the Черновик/Knowledge Base component-test infra existed.
//   - dom: Vue SFC compilation + jsdom, for *.dom.test.ts files that mount
//     real components with @vue/test-utils.
//   - scripts: tests of the build scripts under scripts/ (e.g. the generated
//     visit-card staying in sync with the landing page).
// Kept as ONE config (rather than two separate vitest.config files) so
// `vitest run`/`vitest` picks up both without extra scripting.
export default defineConfig({
  test: {
    projects: [
      {
        resolve: {
          alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
        },
        test: {
          name: 'unit',
          environment: 'node',
          include: ['src/**/*.test.ts'],
          exclude: ['src/**/*.dom.test.ts'],
        },
      },
      {
        test: {
          name: 'scripts',
          environment: 'node',
          include: ['scripts/**/*.test.ts'],
        },
      },
      {
        plugins: [vue()],
        resolve: {
          alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
        },
        test: {
          name: 'dom',
          environment: 'jsdom',
          include: ['src/**/*.dom.test.ts'],
          setupFiles: ['src/test/setup.ts'],
        },
      },
    ],
  },
})
