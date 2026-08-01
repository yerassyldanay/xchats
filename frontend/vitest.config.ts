import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// Two projects, opted into by filename suffix:
//  - 'unit': pure TypeScript logic (no Vue plugin, no DOM) — unchanged from
//    before component-test infra existed.
//  - 'dom': Vue SFC compilation + jsdom, for src/**/*.dom.test.ts (component
//    tests via @vue/test-utils — see src/test/mount.ts).
// Kept as one config (not split into two files) so `vitest run` still runs
// both with a single command.
export default defineConfig({
  test: {
    projects: [
      {
        resolve: {
          alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url)),
          },
        },
        test: {
          name: 'unit',
          environment: 'node',
          include: ['src/**/*.test.ts'],
          exclude: ['src/**/*.dom.test.ts'],
        },
      },
      {
        plugins: [vue()],
        resolve: {
          alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url)),
          },
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
