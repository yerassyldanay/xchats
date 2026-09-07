# Release checklist

The automated [release workflow](../../.github/workflows/release.yml) builds
and pushes the backend and frontend container images, builds native desktop
archives on all three operating systems, creates the corresponding-source
bundle, and publishes the eight downloadable assets through a verified draft.
This checklist covers the human preparation and post-publication checks around
that automation.

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

- [ ] Optionally preflight the container builds locally:
      `docker build -f backend/Dockerfile -t xchats-backend:vX.Y.Z backend/`
      and `docker build -f frontend/Dockerfile -t xchats-frontend:vX.Y.Z frontend/`.
- [ ] Confirm the desktop workflow is green on the release commit. Its Linux,
      macOS, and Windows builds are native because Wails links against each
      platform's WebView toolchain; see [`../desktop.md`](../desktop.md).

## Tag and publish

- [ ] `git tag -s vX.Y.Z -m "vX.Y.Z"` (signed — see
      [`signing.md`](signing.md)) and `git push origin vX.Y.Z`.
- [ ] Watch `.github/workflows/release.yml` through `prepare`, the parallel
      image/desktop/source builds, and `publish-release`. For a manual retry,
      dispatch the workflow with the existing version tag; branch-name manual
      releases are deliberately rejected.
- [ ] Confirm both versioned container images exist in GHCR with their build
      provenance attestations. Cosign image signing and SBOM generation remain
      planned follow-ups; see [`signing.md`](signing.md) and
      [`sbom-checksums-provenance.md`](sbom-checksums-provenance.md).
- [ ] Confirm `.github/workflows/release.yml`'s `publish-release` job attached
      all 14 release assets to the GitHub Release:
      - Six native desktop packages + their `.sha256` checksums: Linux portable
        (`.tar.gz`) and installer (`.deb`), macOS portable (`.zip`) and installer
        (`.dmg`), Windows portable (`.zip`) and installer (NSIS `.exe`).
      - `xchats-vX.Y.Z-corresponding-source.tar.gz` (+ `.sha256`) — the AGPL-3.0/GPL-3.0 §6
        corresponding-source obligation for the published images and binaries (see
        [`THIRD_PARTY_NOTICES.md`](../../THIRD_PARTY_NOTICES.md)'s libsignal
        section).
      Runs automatically on the tag push; this is a confirm step, not a manual one.
- [ ] Review the generated GitHub release notes, verify all 14 downloadable
      assets are listed, and add the two versioned GHCR image references if they
      are not already documented. Release notes remain editable, but under the
      repository's Immutable Releases policy the tag and assets cannot be replaced.
- [ ] Manually confirm (see [`desktop.md`](../desktop.md)'s own Test and
      Acceptance Criteria): the `.deb` installs on a supported Ubuntu, appears
      in the application launcher, and opens without a terminal; the Windows
      NSIS installer permits choosing an install directory and cleanly
      uninstalls without deleting user data; the macOS `.dmg` supports the
      conventional drag-install flow.

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
