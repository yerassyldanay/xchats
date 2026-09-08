# Desktop executable e2e (Playwright)

Drives the REAL packaged Wails executable end to end — not the Vite dev
server, not a mock. See `docs/desktop.md`'s "Local Executable UI Testing"
section for the full design rationale; this file is the practical how-to.

**Never run this in CI.** It is not wired into any GitHub Actions workflow,
any other Make target, or any npm lifecycle hook, and it never will be — it
builds and launches a real GUI application, which CI has no display for and
no business doing on every push anyway. You run it locally, on demand:

```bash
make desktop-test-ui
```

## What it does

1. Builds the desktop executable for your current platform (`wails build`,
   via `make desktop-build`) unless `DESKTOP_E2E_BINARY` already points at
   one.
2. Copies that executable into a fresh temp directory whose name contains a
   space (`/tmp/xchats e2e .../xchats`), to prove the app and this harness
   both handle that correctly.
3. Launches it with its own isolated `--data-dir`/`XCHATS_CONFIG_DIR`, a
   dynamically chosen loopback port, `XCHATS_ALLOW_FILE_CREDENTIALS=1` (no
   OS keyring needed), and `XCHATS_DESKTOP_E2E_HTTP=1` — the opt-in mode
   that serves the exact embedded production SPA and routes API/media
   requests to the real in-process backend over that loopback port, so a
   normal browser (Chromium, via Playwright) can drive it. See
   `backend/internal/desktop/e2e_http.go`.
4. Waits for the readiness endpoint (both the backend listener and the
   native Wails window itself have started), then logs in as the
   migration-seeded sentinel admin (`admin@xchat.kz` — same as
   `tests/e2e/`, see its README) and drives the Knowledge Base: create a
   product, upload `fixtures/test-image.png`, attach it as both the
   featured image and gallery media, save, reload, then **stop and restart
   the executable against the same data root** and confirm the product and
   its image are still there — the exact path
   `backend/internal/httpapi/kb_live.go`'s live-media-persistence fix
   covers.
5. Confirms the media endpoint serves the right content type and nonzero
   bytes, and that the SQLite database and blob files only ever appeared
   under the configured temporary data root.

## Prerequisites

- Everything `docs/desktop.md`'s own Prerequisites table lists for building
  the desktop app on your platform (Go, Node, the Wails CLI, and on Linux
  `libgtk-3-dev libwebkit2gtk-4.1-dev`).
- **Linux without a display** (a headless dev box, a container): install
  `xvfb` (`sudo apt-get install -y xvfb`) — the harness detects a missing
  `$DISPLAY` and wraps the launch in `xvfb-run -a` automatically. macOS and
  Windows runners normally have a display session already.
- A browser for Playwright itself — same as `tests/e2e/`:
  ```bash
  cd frontend && npm run e2e:install
  ```

## Run

```bash
make desktop-test-ui                          # builds for this platform, then runs
DESKTOP_E2E_BINARY=/path/to/xchats make desktop-test-ui   # skip the build, use this binary
```

Or directly from `frontend/` once a binary exists:

```bash
DESKTOP_E2E_BINARY=../backend/cmd/xchats/build/bin/xchats npm run test:e2e:desktop
```

Application logs land in `frontend/test-results/desktop-e2e-logs/`;
Playwright's own trace/screenshots/HTML report land under
`frontend/test-results/desktop-e2e/` and `frontend/desktop-e2e-report/` —
`npx playwright show-report desktop-e2e-report` opens the last run.

## Notes

- `harness/desktop-app.ts` owns the whole process lifecycle (start, wait
  for readiness, restart, stop) and terminates only the exact process it
  started — never anything found by port or process name.
- The scenario is one long, serial test (`test.describe.configure({ mode:
  'serial' })`) rather than several independent ones: the restart step only
  makes sense as a continuation of the same run, against the same data
  root the earlier steps wrote to.
- Native Wails startup, the desktop asset-server middleware, cookie-jar
  behavior, and the realtime (SSE vs. Wails-events) transport are
  deliberately NOT re-tested here — they already have focused Go/frontend
  test coverage (see `backend/internal/desktop/*_test.go`). This suite's
  job is the one thing only the real packaged executable can prove: that a
  user's data actually survives a save, a reload, and a restart.
