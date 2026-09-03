# Contributing

## Before you start

- Check open issues/PRs for overlap first.
- For anything beyond a small fix, open an issue describing the change
  before writing code — this project has a detailed overview document
  ([`docs/overview.md`](docs/overview.md) is the authoritative architectural
  record) and a change that cuts against a recorded design needs that
  discussed first, not discovered at review time.

## Development setup

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make dev-backend     # backend on :8080 (Go 1.25+)
make dev-frontend    # frontend on :5173 (Node 22+, see .nvmrc)
```

No `.env` file, no secrets to set up first — see
[`docs/release/installation.md`](docs/release/installation.md). `make up`
runs the full Docker stack instead, if you'd rather not run two local
processes. Log in with the default admin account, `admin@xchat.kz` /
`xchat-admin-change-me` — it's a public, documented default (see
installation.md's first-run walkthrough for how to change it).

## Before opening a PR

```bash
cd backend  && go build ./... && go vet ./... && go test -race -count=1 ./...
cd frontend && npm run typecheck && npm run test:unit && npm run build
make lint     # golangci-lint + eslint
```

`make test-e2e` runs the DB-backed integration suites in isolation, if your
change touches `internal/httpapi`, `internal/kbstore`, `internal/responsestore`,
or `internal/store`.

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
  PR** — not "added later." This codebase's test suite is the guard against
  silent regressions in a fast-moving area.
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

## Sign off your commits (DCO)

Every commit must include a `Signed-off-by` trailer, certifying you wrote
it or otherwise have the right to submit it under this project's license
(the [Developer Certificate of Origin](https://developercertificate.org/)):

```bash
git commit -s -m "fix: ..."
```

There is no separate CLA. `libsignal`, a GPL-3.0 dependency statically
linked into the backend (see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)),
already rules out ever relicensing the distributed binary under a more
permissive license regardless of what contributors sign — so a
CLA-for-future-relicensing would buy nothing here. The DCO's purpose is
narrower and still real: a clear, lightweight record that every line was
contributed with the right to do so under [AGPL-3.0](LICENSE).

## Pull requests

- Keep PRs scoped to one change — easier to review, easier to revert if
  something's wrong.
- Explain the *why* in the PR description; the diff already shows the *what*.
- A PR that touches [`docs/overview.md`](docs/overview.md)-covered
  territory should update that file in the same PR, not leave it to drift
  stale.
- Every PR needs one approving review from a code owner before it can merge
  ([`CODEOWNERS`](.github/CODEOWNERS)) — neither maintainer can self-merge.
- Be responsive to review feedback; an abandoned PR with no activity may be
  closed after a while and can always be reopened later.

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md).
