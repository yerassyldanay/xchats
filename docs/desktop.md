# Desktop app (Wails)

xchats ships as a native desktop application for Windows, macOS and Linux,
built with [Wails v2](https://wails.io). This is the developer guide: how to
run it, how to build it, what CI produces, and what a user ends up with.

For the browser/Docker deployment, see [`release/docker.md`](release/docker.md)
— it is unchanged, and nothing here replaces it.

---

## What the desktop app actually is

**The same `xchats` binary, plus a window.** There is no second application,
no separate service, and no IPC bridge between a "frontend process" and a
"backend process":

```
                      xchats  (one process)
  ┌──────────────────────────────────────────────────────────────┐
  │  cmd/xchats runServe — store, queue, workers, response        │
  │  service, channels, MCP, tunnel … all exactly as before       │
  │                                                              │
  │      ├── http.Server on 127.0.0.1:8080  (unchanged)          │
  │      │                                                       │
  │      └── internal/desktop shell (only in the desktop build)   │
  │            ├── WebView window                                 │
  │            ├── embedded Vue SPA (frontend/dist)               │
  │            ├── /xchats/api/v1/… → the same gin router,        │
  │            │   in-process, same-origin                        │
  │            └── realtime.Hub events → Wails events             │
  └──────────────────────────────────────────────────────────────┘
```

The desktop-only code is compiled behind the `desktop` build tag, so
`go build ./...`, the Docker image and CI produce byte-for-byte the server
binary they always did. Everything the shell needs from the app it receives
from `runServe`; it constructs no application state of its own.

| Concern | Browser deployment | Desktop app |
|---|---|---|
| Where the SPA is served from | nginx (`frontend/Dockerfile`) | embedded in the binary, served by Wails' asset server |
| How the SPA reaches the API | same-origin over the network | same-origin, in-process (`internal/desktop/handler.go`) |
| Live updates | SSE (`GET /xchats/api/v1/realtime`) | Wails events (`internal/desktop/realtime.go`) |
| Session cookie | the browser's cookie jar | the shell's cookie jar (see below) |
| SQLite + blobs | `storage.*` paths from config/env | the same paths, resolved against the OS app-data directory |

### Why realtime is not SSE here

Wails' asset server cannot stream a response on Windows: WebView2's response
writer buffers everything until the handler returns, so an `EventSource`
would connect and then never receive an event — and would hold a WebView
request slot open for the life of the app. The shell forwards the same
`realtime.Hub` events over Wails' own event channel instead, and
`frontend/src/lib/sse.ts` picks the transport at runtime. Event names and
payloads are identical, so no handler downstream of `connectRealtime` can
tell which transport delivered it. `/xchats/api/v1/realtime` is explicitly
refused (501) in the desktop build rather than left to hang.

### Why the shell keeps a cookie jar

On macOS and Linux the window loads from a custom URL scheme
(`wails://wails/`), and neither WKWebView nor WebKitGTK runs its cookie store
for a custom scheme — `Set-Cookie` is dropped and no `Cookie` header comes
back. Session auth would simply never work there.

Rather than change how the backend authenticates for a packaging reason, the
shell does the one job the WebView is not doing: it remembers the cookies the
router sets and replays them on the next request. That is a browser's cookie
jar, scoped to this process and this one origin. Login, logout, session
expiry and org switching behave exactly as they do in a browser. On Windows
the window loads from `http://wails.localhost/` and the WebView *does* manage
cookies; the jar leaves a request that already carries one alone.

---

## Prerequisites

| | Everyone | Windows | macOS | Linux |
|---|---|---|---|---|
| Go | per `backend/go.mod` (1.25+) | | | |
| Node | per `.nvmrc` (22) | | | |
| Wails CLI | `make desktop-tools` | | | |
| Platform SDK | | [WebView2 runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (preinstalled on Windows 10/11) | Xcode command line tools | `libgtk-3-dev`, `libwebkit2gtk-4.1-dev` |

```bash
make desktop-tools          # go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails doctor                # confirms the OS-level pieces are present
```

On Debian/Ubuntu:

```bash
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

> **Linux build tag.** Every Linux `wails` invocation needs `-tags webkit2_41`
> (the Makefile adds it for you). Without it Wails compiles its legacy
> WebKitGTK path, where the WebView hands the asset server every request as a
> bodyless `GET` — every POST the app makes would arrive empty. `webkit2_41`
> is `libwebkit2gtk-4.1`, which means Ubuntu 24.04+, Fedora 39+, Debian 13+.

---

## Run it locally

```bash
make desktop-dev
```

That runs `wails dev` from `backend/cmd/xchats/`, which:

1. runs `npm ci` and `npm run build:desktop` in `frontend/`,
2. starts the Vite dev server on `:5173`,
3. compiles and launches `xchats` with `-tags desktop,dev`.

The window loads from Vite, so editing a `.vue`/`.ts` file hot-reloads the
UI. Go changes trigger a rebuild and relaunch. `Ctrl-C` in the terminal
closes the window and runs the normal shutdown sequence.

The `-skipbindings` flag the Makefile passes is not optional — see
[Gotchas](#gotchas).

To build and run without the dev server:

```bash
make desktop-assets                                 # build the SPA + mirror it for go:embed
cd backend && go build -tags "desktop webkit2_41" -o /tmp/xchats ./cmd/xchats   # drop the webkit tag off Linux
/tmp/xchats
```

### Where your data goes

A desktop launch does **not** read `./config.yaml` from the working directory
— a packaged app's working directory is whatever Finder, Explorer or the
`.desktop` entry handed the process, so picking a file up from there would be
silent and unreproducible. It reads `$XCHATS_CONFIG`, then
`<OS config dir>/xchats/config.yaml`, and falls back to the built-in defaults
if neither exists.

Relative `storage.*` paths are resolved against the OS application data
directory instead of the process working directory. Absolute paths are left
exactly as configured.

| | Config | Data (SQLite, blobs, credentials, `settings.json`) |
|---|---|---|
| Windows | `%APPDATA%\xchats\` | `%LOCALAPPDATA%\xchats\` |
| macOS | `~/Library/Application Support/xchats/` | `~/Library/Application Support/xchats/` |
| Linux | `$XDG_CONFIG_HOME/xchats/` (or `~/.config/xchats/`) | `$XDG_DATA_HOME/xchats/` (or `~/.local/share/xchats/`) |

With the built-in defaults that means `…/xchats/data/xchats.db`,
`…/xchats/data/whatsmeow.db` and `…/xchats/blobdata/`. `XCHATS_DATA_DIR` and
`XCHATS_CONFIG_DIR` redirect the whole profile — that is how you run a
throwaway instance without touching your real one:

```bash
XCHATS_DATA_DIR=/tmp/xchats-scratch XCHATS_CONFIG_DIR=/tmp/xchats-scratch /tmp/xchats
```

`--data-dir <absolute-path>` does the same job as `XCHATS_DATA_DIR` for the
data half, with higher precedence (a flag beats the environment), and with
its own validation up front: the path must be absolute, it is created with
owner-only permissions if missing, and a startup error names the path and
the reason if it turns out not to be writable — rather than that surfacing
later as a confusing failure inside the store or blob layer.

```bash
/tmp/xchats --data-dir /tmp/xchats-scratch
```

There is no equivalent `--config-dir` flag; use `XCHATS_CONFIG_DIR` for that
half, exactly as before. The precedence for the data root is: `--data-dir`,
then `XCHATS_DATA_DIR`, then the OS default above.

The resolved config file path, config directory, data directory, and the
database/blob paths derived from it are shown read-only in **Settings → Data
& backup**, and in `GET /settings`'s `storage_locations` field — for
troubleshooting and for copying into a bug report or a manual backup script.
There is no graphical relocation control; moving the profile means using one
of the mechanisms above and restarting.

See [`release/data-locations-and-privacy.md`](release/data-locations-and-privacy.md)
for what each of those files holds.

### The listener

The desktop app still starts the normal HTTP server, but bound to
**127.0.0.1**, never every interface: the window reaches the backend
in-process, and nothing outside the machine needs to connect. If the
configured port is already taken (a `make dev-backend`, a compose stack) the
app moves to an OS-assigned port rather than refusing to start, and
`api_base_url` follows it so MCP discovery and signed media URLs stay
correct.

`http://127.0.0.1:8080/healthz` and the rest of the API are reachable from a
terminal or an MCP client as usual. The UI itself is **not** — it lives in
the app window, not on that port.

---

## Build it locally

```bash
make desktop-build
```

Output lands in `backend/cmd/xchats/build/bin/`:

| Platform | Artifact |
|---|---|
| Windows | `xchats.exe` |
| macOS | `xchats.app` (a bundle) |
| Linux | `xchats` (a dynamically linked ELF; needs GTK3 + WebKitGTK 4.1 at runtime) |

Wails cross-compiles poorly for desktop targets — each one links against its
platform's own WebView through cgo — so build each platform on that platform
(which is exactly what CI does).

Stamp a version the way the release workflow does:

```bash
cd backend/cmd/xchats
wails build -clean -skipbindings -tags webkit2_41 \
  -ldflags "-X github.com/yerassyldanay/xchats/backend/internal/version.Version=v1.2.3"
```

`make desktop-clean` removes the build output and the mirrored SPA bundle.

---

## What CI produces

[`.github/workflows/desktop-build.yml`](../.github/workflows/desktop-build.yml)
has two very different jobs depending on why it ran — a job-level `if:`
cannot see which matrix entry is running, so "only Linux, only on a PR" is
its own job rather than a condition on one matrix entry of the other:

- **`pr-compile-check`** — pull requests that touch what actually goes into
  the desktop binary (`backend/cmd/xchats/**`, `backend/internal/desktop/**`,
  `backend/go.{mod,sum}`, `frontend/**`, `.nvmrc`, `scripts/package-*.sh`, the
  workflow itself). One `ubuntu-24.04` runner, `wails build` and nothing
  else — no installers, no checksums, no archives, no artifact upload, no
  Playwright browser install, no launching the app. This proves the
  desktop-tagged code still compiles; it is not a packaging dry run.
  Ordinary backend changes are already compiled and tested by `ci.yml`'s
  `backend-test` job, so they do not spend a macOS runner here either.
- **`build`** — `workflow_dispatch` for a build on demand, or `workflow_call`
  so `release.yml` can reuse this job when cutting a release on a `v*.*.*`
  tag. Never a pull request. All three platforms build natively (on
  `windows-latest`, `macos-latest` and `ubuntu-24.04`), each producing both
  the portable archive and an installer format, checksummed and uploaded.

**Each `build` job** installs the platform's WebView toolchain (plus NSIS on
Windows), installs the pinned Wails CLI, runs `wails build` (`-nsis` on
Windows — see below), packages both formats, checksums every file, and
uploads them.

| Job | Uploads |
|---|---|
| `linux` | `xchats-desktop-linux-amd64.tar.gz` + `.sha256`, `xchats-desktop-linux-amd64.deb` + `.sha256` |
| `macos` | `xchats-desktop-macos-universal.zip` + `.sha256`, `xchats-desktop-macos-universal.dmg` + `.sha256` |
| `windows` | `xchats-desktop-windows-amd64.zip` + `.sha256`, `xchats-desktop-windows-amd64-installer.exe` + `.sha256` |

The `.deb` is built by [`scripts/package-deb.sh`](../scripts/package-deb.sh)
(conventional paths — `/usr/bin/xchats`, a `.desktop` entry, a hicolor icon,
declared `libgtk-3-0`/`libwebkit2gtk-4.1-0` runtime deps) and the `.dmg` by
[`scripts/package-dmg.sh`](../scripts/package-dmg.sh) (the `.app` plus an
`/Applications` symlink). The NSIS installer comes from Wails' own `-nsis`
flag — see [Wails' Windows installer guide](https://v2.wails.io/docs/guides/windows-installer/)
— using its default template (`build/windows/installer/project.nsi`,
gitignored and regenerated fresh on every build from `build/appicon.png` and
`wails.json`'s `info` fields, same as `Info.plist`/`icon.ico`).

On a manual `workflow_dispatch` run these are workflow artifacts
(downloadable from the run's summary page, kept for the repo's retention
window).

On a `v*.*.*` tag push, [`.github/workflows/release.yml`](../.github/workflows/release.yml)
orchestrates the full release pipeline: it invokes `desktop-build.yml`'s
`build` job via `workflow_call` alongside container image builds and the
corresponding-source bundle. Because this repository enforces GitHub's
**Immutable Releases** policy (published releases cannot accept subsequent
asset uploads), `release.yml`'s final `publish-release` job collects all 14
release assets (six desktop packages, one source bundle, and their seven
checksums), verifies their names and checksums, uploads them to a draft,
verifies the draft's remote asset list, and only then publishes the release.
A manual run of `release.yml` requires an existing version tag and follows
the same path.

The draft gate covers GitHub Release assets. Container images are pushed to
GHCR by a parallel job and remain independently retriable if a later desktop,
source, or release-publication step fails.

---

## What users receive

Each platform gets an installer (the default, linked first below) and a
portable archive (the alternative, for an arbitrary install location):

| Platform | Installer (default) | Portable (alternative) |
|---|---|---|
| Windows | `xchats-desktop-windows-amd64-installer.exe` — NSIS: pick the install folder, Start Menu/desktop shortcuts, clean uninstall, no administrator rights needed for the default per-user install. | `xchats-desktop-windows-amd64.zip` → `xchats.exe`. Unzip, run — nothing written outside `%APPDATA%`/`%LOCALAPPDATA%`. |
| macOS | `xchats-desktop-macos-universal.dmg` — open it, drag `xchats.app` to `/Applications`. Universal: native on Apple Silicon and Intel. | `xchats-desktop-macos-universal.zip` → `xchats.app`. Unzip, drag to wherever you like. |
| Linux | `xchats-desktop-linux-amd64.deb` — `sudo apt install ./xchats-desktop-linux-amd64.deb`. Installs to `/usr/bin/xchats`, adds it to the application menu (GTK3 + WebKitGTK 4.1 declared as package dependencies). | `xchats-desktop-linux-amd64.tar.gz` → `xchats`. Extract and run anywhere; needs GTK3 and WebKitGTK 4.1 from the distro. |

Uninstalling (either the `.deb`, via `apt remove`, or the NSIS installer's own
uninstaller) removes only what was installed — the binary, icon, `.desktop`
entry, shortcuts and registry uninstall entry. Neither has any knowledge of
`~/.config/xchats`/`~/.local/share/xchats` (or their Windows/macOS
equivalents) — configuration, the database, credentials and uploaded media
are never touched by an uninstall.

Every download — installer or portable — is built from the same single
self-contained executable: the Vue bundle, the migrations and the sample
media are all compiled into it. There is nothing to install alongside it and
no separate server to start — launching it boots the backend and opens the
UI, with the same zero-configuration first launch either way (see
[Where your data goes](#where-your-data-goes) above — no first-launch storage
prompt, no migration wizard).

**Not signed or notarized.** The binaries carry no code-signing certificate,
so Windows SmartScreen will warn on first run and macOS will refuse to open
the app until the user right-clicks → Open (or clears the quarantine
attribute). Signing is tracked with the image-signing work in
[`release/signing.md`](release/signing.md).

**No auto-update.** Deliberately out of scope for now. The app still performs
its existing update *check* (`GET /settings/update-check`, surfaced in
Settings); acting on it means downloading the next release by hand.

---

## Local Executable UI Testing

`XCHATS_DESKTOP_E2E_HTTP=1` is an opt-in switch on the desktop executable
(`backend/internal/desktop/e2e_http.go`) that serves the exact embedded
production SPA, and routes API/media requests to the real in-process
backend, through the app's existing loopback HTTP listener — the same
`NewMiddleware`/`NewSPAHandler` the Wails window itself uses, just also
mounted where an ordinary browser can reach it. This exists because
Playwright cannot drive Wails' own asset server (a custom URL scheme on
macOS/Linux, WebView2's buffered writer on Windows), so there was previously
no way to browser-test the real packaged app end to end. It refuses to
activate on anything but a loopback listener, and every route it adds is
completely absent unless the env var is set — production behavior does not
change.

```bash
make desktop-test-ui
```

Builds the desktop executable for your platform (unless `DESKTOP_E2E_BINARY`
already points at one), launches it in full isolation — a temporary
`--data-dir`/`XCHATS_CONFIG_DIR`, a dynamic loopback port, file-backed
credentials, staged under a temp path with a space in its name — and drives
it with Playwright: log in, create a Knowledge Base product, upload a
checked-in image fixture, attach it as the featured image and gallery media,
save, reload, then **stop and restart the executable against the same data
root** and confirm the product and its image survived. See
[`frontend/tests/desktop-e2e/README.md`](../frontend/tests/desktop-e2e/README.md)
for the full scenario and prerequisites (Linux without a display needs
`xvfb`).

**Never run this in CI.** It is not invoked from any GitHub Actions workflow,
any other Make target, or an npm lifecycle hook, and it never will be — it
builds and launches a real GUI application. Run it locally, on demand, after
a change to the desktop shell, the Knowledge Base media flow, or the storage
paths above.

Native Wails startup, the desktop asset-server middleware, cookie-jar
behavior, and the realtime (SSE vs. Wails-events) transport are deliberately
NOT re-tested by this suite — they already have focused Go/frontend test
coverage (`backend/internal/desktop/*_test.go`). Its job is the one thing
only the real packaged executable can prove: that a user's data actually
survives a save, a reload, and a restart.

---

## Gotchas

**`wails build` without `-skipbindings` hangs.** Wails' binding generator
compiles the module a second time and *runs* the resulting binary to dump JS
wrappers for whatever `options.App.Bind` holds. Nothing is bound here (the
frontend talks to the backend over ordinary same-origin HTTP), and the
generator strips the `desktop` tag before compiling — so the binary it runs
is the plain server, which would boot the whole app and block in
`xchats serve` forever. `backend/cmd/xchats/bindings.go` is a `bindings`-tagged
`init()` that exits immediately, so a bare `wails build` degrades instead of
hanging; every documented invocation passes `-skipbindings` anyway.

**The embedded bundle is a mirror, not a build target.**
`frontend/scripts/sync-desktop-assets.ts` copies `frontend/dist` into
`backend/internal/desktop/dist/`, because `go:embed` cannot reach outside its
own package directory and repointing Vite's `outDir` would have changed the
web and Docker builds. `npm run build:desktop` runs both steps; `wails.json`
wires it to `frontend:build` so `wails dev` and `wails build` never need you
to remember. Only `.gitkeep` is committed there.

**`go build -tags desktop` needs the bundle first.** Without a
`make desktop-assets`, the embed directory holds only `.gitkeep`; the binary
links but the window reports a missing bundle at startup. The `wails`
commands take care of this themselves.

**Linux without a keyring.** The credential store wants an OS keychain
(Secret Service on Linux — GNOME Keyring, KWallet). On a desktop session
that is normally present; in a container or a bare window manager it is not,
and several features degrade loudly at boot. Set
`XCHATS_ALLOW_FILE_CREDENTIALS=1` to use the encrypted file store instead —
see [`release/credentials.md`](release/credentials.md).

**Second launch focuses the first window.** The store takes an exclusive file
lock on the SQLite database, so a second instance could never boot anyway.
Wails' single-instance lock turns a second launch into "focus the window that
is already open".

**The MCP widget's "Review and publish in Xchats" link.** It is built from
`frontend_base_url`, which in a desktop install points at nothing — the UI
lives in the app window, not at a URL. Set `FRONTEND_BASE_URL` only if you
are also running the web frontend against the same backend.

**`/evals-data/`** is served by nginx in the compose stack, so the eval
comparison UI has no data source in the desktop app.

**Third-party notices.** `scripts/notices.sh` reports the untagged
`cmd/xchats` dependency graph, so `THIRD_PARTY_LICENSES.txt` does not yet
cover Wails and its dependencies. Regenerate with the desktop tag in scope
before a release that ships desktop binaries.

---

## Where the code lives

| Path | What it is |
|---|---|
| `backend/internal/desktop/desktop.go` | Config-path and storage-path resolution, loopback binding. No build tag — covered by `go test ./...`. |
| `backend/internal/desktop/handler.go` | The asset-server middleware: API-vs-SPA routing, the cookie jar, the history-mode fallback. No build tag. |
| `backend/internal/desktop/realtime.go` | `realtime.Hub` → Wails events pump. No build tag. |
| `backend/internal/desktop/e2e_http.go` | `XCHATS_DESKTOP_E2E_HTTP=1`'s loopback SPA-over-HTTP mode and readiness endpoint — see [Local Executable UI Testing](#local-executable-ui-testing). No build tag. |
| `backend/internal/appdirs/appdirs.go` | OS default config/data directories, and `--data-dir`'s validation (`ValidateOverrideDir`). No build tag. |
| `backend/internal/desktop/shell.go` | Wails app options and window lifecycle; marks `e2e_http.go`'s `Readiness` window-ready on `OnStartup`. `//go:build desktop`. |
| `backend/internal/desktop/assets.go` | `go:embed` of the mirrored SPA. `//go:build desktop`. |
| `backend/cmd/xchats/desktop{,_on,_off}.go` | The `--data-dir`/E2E-HTTP-wrapping hooks in `runServe`, in both build flavours. |
| `backend/cmd/xchats/bindings.go` | The binding-generator guard. `//go:build bindings`. |
| `backend/cmd/xchats/wails.json` | Wails project config (it lives next to the `main` package it builds). |
| `scripts/package-deb.sh` / `scripts/package-dmg.sh` | Build the `.deb`/`.dmg` from an already-built `wails build` output — see [What CI produces](#what-ci-produces). |
| `frontend/tests/desktop-e2e/` | The Playwright suite `make desktop-test-ui` runs — see its own README. |
| `frontend/src/lib/desktop.ts` | Runtime detection of the Wails shell. |
| `frontend/src/lib/sse.ts` | Picks SSE or Wails events. |
