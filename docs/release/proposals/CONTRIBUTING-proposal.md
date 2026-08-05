# Contributing — proposal

**Status: proposal.** No `CONTRIBUTING.md` exists today. This draft is
based on patterns already visible in the codebase and its git history —
codifying existing practice, not inventing new rules — plus a placeholder
CLA question the repo owner needs to answer (see
[`LICENSE-proposal.md`](LICENSE-proposal.md) for why it matters).

## Before you start

- Check open issues/PRs for overlap first.
- For anything beyond a small fix, open an issue describing the change
  before writing code — this project has a detailed `plan/` directory
  (`plan/DECISIONS.md` is the authoritative design record) and a change that
  cuts against a recorded decision needs that discussed first, not
  discovered at review time.

## Development setup

```bash
git clone <repo>
cd xchats
make dev-backend     # backend on :8080 (Go 1.25+)
make dev-frontend    # frontend on :5173 (Node 22+)
```

No `.env` file, no secrets to set up first — see
[`../installation.md`](../installation.md). `make up` runs the full Docker
stack instead, if you'd rather not run two local processes.

## Before opening a PR

```bash
cd backend  && go build ./... && go vet ./... && go test -race ./...
cd frontend && npm run build && npx vitest run
```

`npm run build` includes the `vue-tsc --noEmit` typecheck — a type error
fails the build, not just a separate lint step. `make test-e2e` runs the
DB-backed integration suites in isolation, if your change touches
`internal/httpapi`, `internal/kbstore`, `internal/responsestore`, or
`internal/store`.

For a **UI-facing change**, run it in an actual browser before calling it
done — the test suites verify code correctness, not that the feature looks
and behaves right; there's no substitute for looking at it.

## Code conventions (observed in the existing codebase)

- **Backend:** Go, `internal/` package boundaries are enforced by
  architecture tests (`internal/dbtest.TestArchitectureBoundary`,
  `response.TestPackageDependencies`) — these fail CI-equivalent local runs
  if a package reaches somewhere it structurally shouldn't (e.g. only
  whitelisted packages may import `internal/dbx` directly). Read the failing
  test's own message before working around it; it usually names the correct
  seam (a wrapper method, a callback field) instead.
- **Comments explain WHY, not WHAT.** A well-named function doesn't need a
  comment restating its name in prose. A comment earns its place by
  recording a non-obvious constraint, an invariant, or the reason a piece of
  code looks less obvious than expected — not by narrating what the next
  three lines do.
- **No speculative abstraction.** Match the shape of the problem in front of
  you; don't build a generic layer for a need you don't have yet.
- **Every new package-level capability gets tests alongside it in the same
  PR** — not "added later." This codebase's own test suite is the guard
  against silent regressions in a fast-moving area (see `internal/httpapi`'s
  140+ test functions as the working example of the expected density).
- **Frontend:** Vue 3 `<script setup>` + TypeScript, Pinia for state,
  Tailwind for styling, `reka-ui` (shadcn-vue pattern) for primitives.
  `vue-i18n` with **strict en/ru key parity** — `locales.test.ts` fails the
  build if a translation key exists in one locale file but not the other;
  add both together, in the same PR, every time.

## Commit messages

This repo's history follows a `type: summary` convention (`feat:`, `fix:`,
`refactor:`, `test:`, `docs:`, `chore:` — broadly [Conventional
Commits](https://www.conventionalcommits.org/)-shaped, though not
mechanically enforced today). Pick the type that matches the *nature* of the
change, write the summary in the imperative mood ("add", not "added" or
"adds"), and keep the subject line focused on one change — a PR that mixes
an unrelated refactor into a bug-fix commit makes both harder to review and
impossible to selectively revert later.

## Pull requests

- Keep PRs scoped to one change — easier to review, easier to revert if
  something's wrong.
- Explain the *why* in the PR description; the diff already shows the *what*.
- A PR that touches `plan/DECISIONS.md`-covered territory should update that
  file in the same PR, not leave it to drift stale — this repo treats stale
  design docs as a real defect (see `docs/release/release-checklist.md`'s
  pre-release check for exactly this).
- Be responsive to review feedback; an abandoned PR with no activity may be
  closed after a while and can always be reopened later.

## Contributor License Agreement (CLA)

`[TODO: repo owner — decide and state here. If AGPL-3.0 is adopted (see
../LICENSE-proposal.md) and there's any chance of wanting a more permissive
license or a dual-licensing/commercial-license model later, requiring a CLA
from every contributor now is much cheaper than trying to get retroactive
relicensing consent from everyone later. If neither applies, explicitly
state "no CLA required" so contributors aren't left guessing.]`

## Code of conduct

`[TODO: repo owner — adopt one (e.g. the Contributor Covenant) and link it
here, or state explicitly that informal norms apply for now.]`
