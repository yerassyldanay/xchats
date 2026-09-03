# Release checklist

There is no automated release pipeline yet (no `.github/workflows/`) — this
checklist is the manual process until one exists, and the spec for what that
automation should do once it's built.

## Before cutting a release

- [ ] `cd backend && go build ./... && go vet ./... && go test -race ./...`
      all clean.
- [ ] `cd frontend && npm run build && npx vitest run` all clean (`npm run
      build` includes the `vue-tsc --noEmit` typecheck).
- [ ] `make test-e2e` passes (the DB-backed integration suites, run in
      isolation from the rest of `test-backend`).
- [ ] No secrets in the diff — double-check anything touching `config.yaml`,
      `deploy/`, or test fixtures for anything that looks like a real key
      pasted in by mistake.
- [ ] `docs/overview.md`, this `docs/release/` tree, and the README reflect
      reality — a stale doc claiming something the code no longer does is
      worse than no doc.
- [ ] Changelog entry written for the release (see
      `proposals/CHANGELOG-format-proposal.md`).

## Version the build

`internal/version.Version` is the single source every "what version am I
running" surface reads from (the Settings page's update notice, `GET
/settings/update-check`). Stamp it at build time rather than editing the
source constant:

```bash
go build -ldflags "-X github.com/yerassyldanay/xchats/backend/internal/version.Version=X.Y.Z" \
  -o xchats ./cmd/xchats
```

Follow [Semantic Versioning](https://semver.org/): breaking API/schema
changes bump major, backward-compatible features bump minor, fixes bump
patch. Since every migration is forward-only (see
[`upgrade-rollback.md`](upgrade-rollback.md)), a schema-changing migration
in a release is itself a signal to consider whether that's a minor or major
bump for your own compatibility policy.

## Build artifacts

- [ ] `docker build -f backend/Dockerfile -t xchats-backend:vX.Y.Z backend/`
- [ ] `docker build -f frontend/Dockerfile -t xchats-frontend:vX.Y.Z frontend/`
- [ ] (if publishing native binaries) cross-compile with the version ldflags
      above for each target platform.

## Tag and publish

- [ ] `git tag -s vX.Y.Z -m "vX.Y.Z"` (signed — see
      [`signing.md`](signing.md)) and `git push origin vX.Y.Z`.
- [ ] Push the built images to the registry; sign them (`cosign sign`, see
      [`signing.md`](signing.md)).
- [ ] Generate and attach an SBOM, checksums, and a provenance attestation
      (see [`sbom-checksums-provenance.md`](sbom-checksums-provenance.md)).
- [ ] Confirm `.github/workflows/release.yml`'s `source-bundle` job attached
      `xchats-vX.Y.Z-corresponding-source.tar.gz` (+ `.sha256`) to the GitHub
      Release — the AGPL-3.0/GPL-3.0 §6 corresponding-source obligation for
      the images just published (see
      [`THIRD_PARTY_NOTICES.md`](../../THIRD_PARTY_NOTICES.md)'s libsignal
      section). Runs automatically on the tag push; this is a confirm step,
      not a manual one.
- [ ] Publish GitHub release notes from the changelog entry, linking the
      images/checksums/SBOM as release assets.

## After publishing

- [ ] Confirm `GET /settings/update-check` against a still-running previous
      version now reports the new release (GitHub's releases API + the
      1-hour cache means this may take up to an hour to reflect — see
      [`upgrade-rollback.md`](upgrade-rollback.md)).
- [ ] Smoke-test a fresh install against the published image
      ([`installation.md`](installation.md)'s verification steps): boots,
      migrations apply, seeded admin login works, first-run wizard appears.
- [ ] Smoke-test an upgrade from the prior version against a copy of real
      (or realistic) data: backup → upgrade → verify → note anything
      surprising for the next release's checklist.
