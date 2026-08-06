# Third-party notices — proposal

**Superseded.** The real, tool-generated file now lives at
[`../../../THIRD_PARTY_NOTICES.md`](../../../THIRD_PARTY_NOTICES.md),
produced with `google/go-licenses` (Go) and `license-checker-rseidelsohn`
(npm) reading actual `LICENSE` files from the module cache rather than from
recall. **This draft's own manual, direct-dependency-only approach is what
caused the error below** — it never walked the transitive tree, so it
missed `go.mau.fi/libsignal`, and the tooling-generated file is kept as
current going forward. This document is retained for its historical record
of the mistake, not as a source of truth.

<details>
<summary>Original draft text (superseded, kept for context)</summary>

This list was compiled from `backend/go.mod`'s direct
requirements and `frontend/package.json`'s `dependencies`, with license
identifiers filled in from general knowledge of these (well-known, widely
used) packages rather than by fetching and checking each project's actual
`LICENSE` file in this pass.

</details>

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

## Correction: this finding was wrong

This section originally claimed every dependency was permissively licensed
and that nothing here was incompatible with shipping xchats under a
permissive (MIT/Apache-2.0) license. **That was wrong**, and it was wrong
specifically because this draft only looked at *direct* dependencies:
`go.mau.fi/libsignal` — pulled in transitively through `go.mau.fi/whatsmeow`
— is **GPL-3.0**, and roughly two dozen of its packages
(`go.mau.fi/libsignal/{ecc,groups,kdf,cipher,...}`) are statically linked
into `cmd/xchats` the moment WhatsApp connectivity is built in. Confirmed via
`go list -deps ./cmd/xchats | grep libsignal` and reading
`go.mau.fi/libsignal@v0.2.2/LICENSE` directly from the module cache.

That finding is why the project license is AGPL-3.0-only rather than
MIT/Apache-2.0 — see [`LICENSE-proposal.md`](LICENSE-proposal.md)'s own
correction. GPL-3.0 and AGPL-3.0 are explicitly compatible with each other
(the [FSF's GPL/AGPL FAQ
entry](https://www.gnu.org/licenses/gpl-faq.en.html#AGPLGPL)), so this is
not a conflict blocking the project's own license — it's the reason
MIT/Apache-2.0 were never actually on the table for the distributed binary.

The generated [`THIRD_PARTY_NOTICES.md`](../../../THIRD_PARTY_NOTICES.md)
lists `go.mau.fi/libsignal` (GPL-3.0) and `go.mau.fi/util` (MPL-2.0)
explicitly, alongside `whatsmeow`'s own file-level MPL-2.0, and is generated
from the real transitive build graph of the one distributed binary
(`./cmd/xchats`), not hand-maintained.

## Maintaining this file

Regenerate whenever dependencies change meaningfully (a new direct
dependency, a major version bump of an existing one) — ideally as a CI check
that fails when `go.mod`/`package.json` drift from the last-generated
notices file, once release automation exists (see
[`../release-checklist.md`](../release-checklist.md)).
