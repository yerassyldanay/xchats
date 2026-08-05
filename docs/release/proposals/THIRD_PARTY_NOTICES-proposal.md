# Third-party notices — proposal

**Status: proposal, and a starting point only — not a verified legal
document.** This list was compiled from `backend/go.mod`'s direct
requirements and `frontend/package.json`'s `dependencies`, with license
identifiers filled in from general knowledge of these (well-known, widely
used) packages rather than by fetching and checking each project's actual
`LICENSE` file in this pass. **Before publishing a real
`THIRD_PARTY_NOTICES` file, regenerate and verify this with tooling** —
[`google/go-licenses`](https://github.com/google/go-licenses) for the Go
module graph, [`license-checker`](https://github.com/davglass/license-checker)
or `npx license-checker-rseidelsohn` for npm — which read the actual
license files rather than relying on recall, and will also surface the full
*transitive* dependency tree (this list is direct/first-order dependencies
only).

## Why this exists

Redistributing software that bundles third-party open-source code generally
requires reproducing each dependency's license/copyright notice — this file
is that notice, once verified. It's also a practical companion to
[`../sbom-checksums-provenance.md`](../sbom-checksums-provenance.md)'s SBOM:
same dependency graph, different purpose (legal notice vs. security
inventory).

## Backend (Go modules — `backend/go.mod`, direct requires)

| Module | License (unverified — confirm before publishing) |
|---|---|
| `github.com/caarlos0/env/v11` | MIT |
| `github.com/gin-gonic/gin` | MIT |
| `github.com/gofrs/flock` | BSD-3-Clause |
| `github.com/google/uuid` | BSD-3-Clause |
| `github.com/skip2/go-qrcode` | MIT |
| `github.com/zalando/go-keyring` | MIT |
| `go.mau.fi/whatsmeow` | Mozilla Public License 2.0 (MPL-2.0) |
| `go.opentelemetry.io/otel` (+ `sdk`, `trace`, `exporters/otlp/...`) | Apache-2.0 |
| `golang.ngrok.com/ngrok` | MIT |
| `golang.org/x/crypto` | BSD-3-Clause |
| `google.golang.org/protobuf` | BSD-3-Clause |
| `gopkg.in/yaml.v3` | MIT / Apache-2.0 (dual-licensed, historical from go-yaml) |
| `modernc.org/sqlite` | BSD-3-Clause style (Cznic) |

**MPL-2.0 flag:** `go.mau.fi/whatsmeow` is weak-copyleft at the *file* level
— MPL-2.0 requires that modifications to MPL-licensed files themselves stay
under MPL-2.0 and be made available, but does not extend to other files in
the same repository that merely import it (unlike AGPL/GPL). This is
compatible with the [AGPL-3.0 proposal](LICENSE-proposal.md) for the rest of
this codebase — confirm this reading before publishing if the license choice
changes.

## Frontend (npm — `frontend/package.json`, `dependencies` only)

Runtime dependencies — the code that actually ships inside the built bundle.
`devDependencies` (build tooling: Vite, TypeScript, Vitest, Playwright,
Tailwind, ...) are deliberately excluded here since their own code doesn't
ship in the production bundle, only their *output* does — but re-run the npm
tooling above with `--production=false` if a fully exhaustive audit is
wanted regardless.

| Package | License (unverified — confirm before publishing) |
|---|---|
| `vue` | MIT |
| `vue-router` | MIT |
| `pinia` | MIT |
| `vue-i18n` | MIT |
| `reka-ui` | MIT |
| `@vueuse/core` | MIT |
| `class-variance-authority` | Apache-2.0 |
| `clsx` | MIT |
| `gray-matter` | MIT |
| `lucide-vue-next` | ISC |
| `markdown-it` | MIT |
| `nanoid` | MIT |
| `tailwind-merge` | MIT |
| `yaml` | ISC |

## No copyleft-incompatible findings

At a glance, every direct dependency above is permissively licensed
(MIT/BSD/Apache-2.0/ISC) except `whatsmeow`'s file-level MPL-2.0 — none
create an obligation incompatible with shipping xchats itself under a
permissive license, and (per the flag above) none appear incompatible with
the AGPL-3.0 proposal either. This is not a substitute for running the
verification tooling linked above, particularly for the full transitive
tree, which is meaningfully larger than this direct-dependency list (see
`go.mod`'s `// indirect` block and `package-lock.json`).

## Maintaining this file

Regenerate whenever dependencies change meaningfully (a new direct
dependency, a major version bump of an existing one) — ideally as a CI check
that fails when `go.mod`/`package.json` drift from the last-generated
notices file, once release automation exists (see
[`../release-checklist.md`](../release-checklist.md)).
