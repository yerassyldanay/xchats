# Signing releases

**Status: partially implemented.** The release workflow publishes immutable
GitHub Releases, records SHA-256 checksums for every desktop/source archive,
and creates GitHub build-provenance attestations for the container images.
Cosign image signing, a signed checksum manifest, macOS notarization, and
Windows Authenticode signing are not implemented yet.

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

**2. Container image signing (GHCR).** [Sigstore
cosign](https://docs.sigstore.dev/cosign/signing/overview/), keyless mode via OIDC
from GitHub Actions — no long-lived signing key to manage or leak. This remains
planned work beyond the existing provenance attestations:

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

**3. Checksums for binary artifacts.** Native desktop and corresponding-source
archives are now published with individual `.sha256` files. A future signing
stage should consolidate or sign those checksums with cosign or minisign so
users can authenticate the checksum values, not merely detect corruption.

**4. Provenance.** See
[`sbom-checksums-provenance.md`](sbom-checksums-provenance.md) for
attaching a build-provenance attestation (what commit, what toolchain
versions, what CI run produced this artifact) — `cosign attest` or GitHub's
native [artifact attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
both fit a GitHub Actions-based pipeline without extra infrastructure.

## What this is not proposing

Not proposing a custom PKI. Platform-specific signing does now apply to the
distributed desktop app, but macOS notarization and Windows Authenticode need
separate credentials and are intentionally deferred from the current unsigned
portable archives.
