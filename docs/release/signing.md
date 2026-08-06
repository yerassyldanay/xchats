# Signing releases

**Status: not implemented yet.** No CI pipeline exists in this repo today
(no `.github/workflows/`), so nothing currently signs a build automatically.
This page is a recommendation for when release automation is set up, not a
description of an existing process — track its adoption in
[`release-checklist.md`](release-checklist.md) once it exists.

## Why it matters here specifically

xchats is self-hosted and handles real credentials (LLM provider keys,
Telegram bot tokens) and message content. An operator pulling a Docker image
or downloading a binary release has no way to confirm it actually came from
this repository's build process, rather than a tampered fork or a
compromised registry, unless something verifiable says so.

## Recommended approach

**1. Signed git tags.** Every release tag (`vX.Y.Z`) should be an annotated,
GPG-signed tag (`git tag -s vX.Y.Z -m "..."`), not a lightweight one. This is
the cheapest step and the foundation everything else can point back to —
`git tag -v vX.Y.Z` lets anyone confirm a checkout matches a tag the
maintainer actually signed.

**2. Container image signing (Docker Hub / GHCR).** [Sigstore
cosign](https://docs.sigstore.dev/cosign/signing/overview/), keyless mode via OIDC
from GitHub Actions — no long-lived signing key to manage or leak. Once CI
exists:

```bash
cosign sign ghcr.io/yerassyldanay/xchats-backend:vX.Y.Z
cosign sign ghcr.io/yerassyldanay/xchats-frontend:vX.Y.Z
```

An operator verifies with:

```bash
cosign verify ghcr.io/yerassyldanay/xchats-backend:vX.Y.Z \
  --certificate-identity-regexp 'https://github.com/yerassyldanay/xchats/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

**3. Checksums for any binary artifact.** If native binaries are ever
published (beyond the Docker images), publish a `checksums.txt`
(`sha256sum xchats-* > checksums.txt`) alongside the release and sign *that
file* with cosign or a minisign key — signing the checksum manifest instead
of every binary individually is the standard pattern and scales to however
many platform builds a release produces.

**4. Provenance.** See
[`sbom-checksums-provenance.md`](sbom-checksums-provenance.md) for
attaching a build-provenance attestation (what commit, what toolchain
versions, what CI run produced this artifact) — `cosign attest` or GitHub's
native [artifact attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
both fit a GitHub Actions-based pipeline without extra infrastructure.

## What this is not proposing

Not proposing a custom PKI, a paid code-signing certificate, or
platform-specific signing (macOS notarization, Windows Authenticode) —
xchats ships as a Docker image and a Go binary, not a distributed desktop
app, so none of those apply today. Revisit if that changes.
