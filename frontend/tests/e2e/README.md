# Frontend e2e (Playwright)

Browser smoke tests for the SPA: login, the NavRail, and the two Knowledge-Base
pages — Черновик (`/playground`, review-only) and Знаний база
(`/knowledge-base`, the sole creation/edit surface; every write there stages
into the draft, never live — see kb-draft-review-boundaries).

`smoke.spec.ts` covers page chrome; `kb-draft.spec.ts` covers the actual
stage → review → publish/cancel flow end to end, each test owning a unique
natural key and cleaning up in `try/finally` (see that file's own doc
comment for the isolation rules).

`features/kb-full-journey.spec.ts` is a separate, larger feature test — the
KB import & AI assistant feature exercised exactly as a user experiences it
(Settings → import a real URL/PDF → review the draft → delete a card →
author the assistant's identity → publish → ask the simulator real
questions), with every UI action cross-checked against the backend API. It
names no branch or PR — it passes or fails on whether the feature works,
against any deployment that has it. See its own doc comment, and the doc
comments atop `harness/env.ts`/`harness/verify.ts`/`harness/shot.ts`, for the
full design; **Prerequisites** below covers what it additionally needs
beyond the two specs above.

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
   That default now carries a forced first-login password change
   (`0014_force_default_admin_password_change`, per
   `docs/ux/flows/01-onboarding.md`'s friction point 1) — `login()` in
   `helpers.ts` completes that change automatically on a fresh database (to
   a fixed `E2E_PASSWORD_ROTATED` value, `xchat-admin-e2e-rotated-1!` by
   default) and self-heals on every later run against the same,
   already-migrated database by retrying with whichever of the two
   passwords the login form actually accepts. No manual step is needed for
   this on a normal run.
   `E2E_EMAIL`/`E2E_PASSWORD`/`E2E_PASSWORD_ROTATED` only need setting if the
   database the tests point at has had either password changed some other
   way:
   ```bash
   export E2E_EMAIL=admin@xchat.kz
   export E2E_PASSWORD='<your password, if you changed it>'
   export E2E_PASSWORD_ROTATED='<what login() rotated it to, if not the default>'
   ```
3. **Only for `features/kb-full-journey.spec.ts`:**
   - `make up` (the Docker stack — backend `:8080`, frontend `:8081`), not
     `make dev-backend` — the spec sweeps `docker compose logs backend` for
     panics/errors at the end (best-effort: it skips that one check, not the
     rest of the run, if Docker isn't reachable from wherever the suite runs).
   - A `.env` file at the repo root with `FIRECRAWL_API_KEY`,
     `LLAMA_PARSE_API_KEY`, and `LLM_API_KEY`/`LLM_PROVIDER`/
     `LLM_DEFAULT_MODEL` — read only by `harness/env.ts`, to type into the
     Settings UI forms exactly as an operator would; it never reaches the
     backend or Docker directly. Any step whose key is missing is skipped
     (not failed), so the spec still runs meaningfully without any of them.
   - `example.pdf` at the repo root (falls back to `scripts/example.pdf` if
     that's where it currently lives).

## Run

```bash
cd frontend
npm run test:e2e          # headless; auto-starts Vite (reuses one if already up)
npm run test:e2e:ui       # interactive UI mode

# the full journey, against a Docker stack already running on :8081
E2E_BASE_URL=http://localhost:8081 npx playwright test features/kb-full-journey.spec.ts
npx playwright show-report
```

`E2E_BASE_URL` overrides the frontend URL (default `http://localhost:5173`)
— set it and `webServer` is skipped entirely rather than also trying to boot
a local Vite dev server against that URL.

## Notes

- The KB builder uses the deterministic RuleSynthesizer, so `smoke.spec.ts`/
  `kb-draft.spec.ts` need **no LLM key / egress**. The inbox AI-draft flow
  and `features/kb-full-journey.spec.ts` do call the LLM (and, for the
  latter, Firecrawl/LlamaParse) — see its own Prerequisites above.
- Tests share one backend draft and run serially (`fullyParallel: false`,
  `workers: 1`) — this is what keeps `kb-draft.spec.ts`'s two tests, which
  both stage into that same shared draft, from racing each other or
  `smoke.spec.ts`'s read-only assertions; neither file needs its own
  `test.describe.configure({ mode: 'serial' })` on top of that.
  `kb-full-journey.spec.ts` DOES use `mode: 'serial'` itself, for a
  different reason — see its own doc comment.
