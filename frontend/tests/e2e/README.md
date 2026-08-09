# Frontend e2e (Playwright)

Browser smoke tests for the SPA: login, the NavRail, and the two Knowledge-Base
pages — Черновик (`/playground`, review-only) and Знаний база
(`/knowledge-base`, the sole creation/edit surface; every write there stages
into the draft, never live — see kb-draft-review-boundaries).

`smoke.spec.ts` covers page chrome; `kb-draft.spec.ts` covers the actual
stage → review → publish/cancel flow end to end, each test owning a unique
natural key and cleaning up in `try/finally` (see that file's own doc
comment for the isolation rules).

## Prerequisites

1. **A browser.** Playwright downloads Chromium from its CDN:
   ```bash
   npm run e2e:install        # = playwright install chromium
   ```
   In a locked-down sandbox this needs egress to `cdn.playwright.dev` and
   `playwright.download.prss.microsoft.com` (allowlist them, or run where a
   browser is available).
2. **The backend stack running** (Playwright only starts the frontend; no
   separate database service to start — SQLite migrates and seeds itself
   on boot):
   ```bash
   # from repo root:
   make dev-backend     # :8080
   ```
   The tests log in as the migration-seeded admin, `admin@xchat.kz`, whose
   password defaults to the documented public default,
   `xchat-admin-change-me` (see the root [`README.md`](../../../README.md)).
   `E2E_EMAIL`/`E2E_PASSWORD` only need setting if the database the tests
   point at has since had that password changed:
   ```bash
   export E2E_EMAIL=admin@xchat.kz
   export E2E_PASSWORD='<your password, if you changed it>'
   ```

## Run

```bash
cd frontend
npm run test:e2e          # headless; auto-starts Vite (reuses one if already up)
npm run test:e2e:ui       # interactive UI mode
```

`E2E_BASE_URL` overrides the frontend URL (default `http://localhost:5173`).

## Notes

- The KB builder uses the deterministic RuleSynthesizer, so these tests need **no
  LLM key / egress**. The inbox AI-draft flow does call the LLM
  (`openrouter.ai`) — out of scope for this smoke suite.
- Tests share one backend draft and run serially (`fullyParallel: false`);
  `kb-draft.spec.ts` additionally forces `test.describe.configure({ mode:
  'serial' })` since its two tests both stage into that same shared draft.
