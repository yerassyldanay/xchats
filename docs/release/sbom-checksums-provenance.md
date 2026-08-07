# SBOM, checksums, and provenance

**Status: not implemented yet** — like [`signing.md`](signing.md), this
describes a recommended process for when release automation exists, not
something this repo currently produces. Nothing here requires deviating from
the existing toolchain (Go modules, npm) or adding infrastructure beyond a
CI pipeline.

## Software Bill of Materials (SBOM)

An SBOM lists every dependency (direct and transitive) a release artifact
was built from — what a downstream user needs to check their own exposure
when a CVE lands in some library, without re-deriving the dependency tree
themselves.

xchats has two dependency trees to cover: `backend/go.mod` (Go modules) and
`frontend/package.json` (npm). Recommended tool:
[Anchore's `syft`](https://github.com/anchore/syft), which understands both
ecosystems and emits [CycloneDX](https://cyclonedx.org/) or
[SPDX](https://spdx.dev/) format:

```bash
syft dir:backend  -o cyclonedx-json > sbom-backend.json
syft dir:frontend -o cyclonedx-json > sbom-frontend.json
# or, against the built Docker images directly:
syft ghcr.io/yerassyldanay/xchats-backend:vX.Y.Z -o cyclonedx-json > sbom-backend.json
```

Until that's wired into CI, the cheap manual equivalent for a quick
dependency inventory:

```bash
cd backend  && go list -m all > deps-backend.txt
cd frontend && npm ls --all --json > deps-frontend.json
```

— not a real SBOM (no license data, no CPE/PURL identifiers a scanner can
match CVEs against), but accurate as a starting point, and exactly what
`proposals/THIRD_PARTY_NOTICES-proposal.md`
was generated from.

## Checksums

Every published artifact (Docker image digest, and any binary if native
builds are ever published — see [`signing.md`](signing.md)) should have a
`sha256sum` recorded and published in the release notes or a `checksums.txt`
file, so a downstream user can confirm what they downloaded matches what was
published without needing to trust the transport (a mirror, a CDN) it came
through:

```bash
sha256sum xchats-linux-amd64 xchats-darwin-arm64 > checksums.txt
docker inspect --format='{{index .RepoDigests 0}}' ghcr.io/yerassyldanay/xchats-backend:vX.Y.Z
```

## Provenance

Beyond "here's a checksum," provenance answers "what commit, what CI run,
what build environment actually produced this artifact" — the difference
between trusting a checksum because you have to, and being able to verify
the whole chain from source to artifact.

For a GitHub Actions-based pipeline (the natural fit once one exists — see
[`release-checklist.md`](release-checklist.md)), [GitHub's native artifact
attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
cover this with no extra infrastructure:

```yaml
- uses: actions/attest-build-provenance@v1
  with:
    subject-path: 'xchats-*'
```

An operator verifies with `gh attestation verify xchats-linux-amd64 --owner
yerassyldanay`. This is the recommended starting point over a hand-rolled
[SLSA](https://slsa.dev/) attestation — it produces a SLSA-compatible
provenance statement without a custom pipeline to maintain.

## Where these attach

Once produced, all three (SBOM, checksums, provenance attestations) should
be uploaded as release assets alongside the versioned Docker images/binaries
themselves — see [`release-checklist.md`](release-checklist.md)'s publish
step.
