// Flat config (ESLint 9+). Deliberately conservative on a first introduction
// to a mature, un-linted codebase — recommended rule sets only, no custom
// tightening — matching .golangci.yml's own "start conservative" approach on
// the backend. CI runs this with only new/changed lines gated (see
// .github/workflows/ci.yml), not as a whole-repo pass/fail.
import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import globals from 'globals'

export default tseslint.config(
  {
    // The app itself — everything Vite ships to the browser.
    files: ['src/**/*.ts', 'src/**/*.vue'],
    languageOptions: {
      globals: globals.browser,
    },
  },
  {
    // Build/test tooling that runs under Node, not the browser: Vite/
    // Vitest/Playwright/Tailwind configs, the blog build/scaffold scripts,
    // and the Playwright e2e specs themselves.
    files: ['*.config.{js,ts}', 'scripts/**/*.ts', '*.mjs', 'tests/**/*.ts'],
    languageOptions: {
      globals: globals.node,
    },
  },
  {
    ignores: [
      'dist/**',
      'dist-ssr/**',
      'node_modules/**',
      'test-results/**',
      'playwright-report/**',
      'blob-report/**',
      'playwright/.cache/**',
      'coverage/**',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  // 'essential' (correctness only), not 'recommended'/'strongly-recommended'
  // — those tiers pull in formatting/style preferences (attribute-per-line,
  // self-closing tags, ...) that are Prettier's job, not a first-pass
  // linter's; on this un-linted codebase they alone produced ~1800 warnings
  // with zero actual bugs among them.
  ...pluginVue.configs['flat/essential'],
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
  },
  {
    rules: {
      // The codebase leans on `any` at a few real interop boundaries (API
      // envelope decoding, third-party type gaps) — a hard ban here would
      // need per-call-site suppressions rather than fixing real issues.
      '@typescript-eslint/no-explicit-any': 'off',
      // Vue SFCs commonly have a single root/page component per file; this
      // rule is aimed at reusable component libraries, not app views.
      'vue/multi-word-component-names': 'off',
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          // The reka-ui/shadcn-vue wrapper components consistently destructure
          // and discard forwarded props/slots as `_`/`__`/`_a`/`_k` — an
          // established, intentional convention throughout src/components/ui/,
          // not an oversight.
          args: 'all',
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          // `catch (err) { log.warn('thing failed') }` — logging a fixed
          // message without the caught value is a deliberate, repeated
          // pattern here (see src/lib/sse.ts, src/stores/inbox.ts), not
          // something worth forcing a rename/suppression for.
          caughtErrors: 'none',
        },
      ],
    },
  },
  {
    // env.d.ts's `DefineComponent<{}, {}, any>` is Vite's own generated
    // *.vue module declaration (every `npm create vite` Vue+TS project
    // ships this exact shape) — not something to hand-edit for a lint rule.
    // Placed last: tseslint.configs.recommended above is what turns this
    // rule on, so an override has to come after it to win.
    files: ['**/*.d.ts'],
    rules: {
      '@typescript-eslint/no-empty-object-type': 'off',
    },
  },
  {
    // kb-full-journey.spec.ts shares ONE page across many test() blocks
    // (test.beforeAll/afterAll, not the per-test `page` fixture — see its
    // own doc comment on why) via `test('...', async ({}, testInfo) => ...)`
    // — Playwright's own test collector requires that first argument to be
    // an object destructuring pattern (even an empty one) to statically
    // read which fixtures a test needs; `no-empty-pattern` doesn't know
    // that and would otherwise flag every one of these as a mistake.
    files: ['tests/e2e/features/kb-full-journey.spec.ts'],
    rules: {
      'no-empty-pattern': 'off',
    },
  },
)
