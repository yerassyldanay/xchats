# SBOM, checksums, and provenance

**Status: partially implemented.** The release workflow publishes SHA-256
checksums for every desktop/source archive and GitHub build-provenance
attestations for both container images. A full backend/frontend SBOM, signed
checksum manifest, and provenance attestations for the downloadable archives
remain planned work.

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

Every published desktop and corresponding-source archive has an adjacent
`.sha256` file. Docker image digests are recorded by GHCR and used as the
subjects of the build-provenance attestations. A signed aggregate checksum
manifest remains a useful follow-up because unsigned checksums detect transport
corruption but do not independently authenticate the publisher:

```bash
sha256sum xchats-linux-amd64 xchats-darwin-arm64 > checksums.txt
docker inspect --format='{{index .RepoDigests 0}}' ghcr.io/yerassyldanay/xchats-backend:vX.Y.Z
```

## Provenance

Beyond "here's a checksum," provenance answers "what commit, what CI run,
what build environment actually produced this artifact" — the difference
between trusting a checksum because you have to, and being able to verify
the whole chain from source to artifact.

The current pipeline uses [GitHub's native artifact
attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
for both GHCR images:

```yaml
- uses: actions/attest-build-provenance@v4
  with:
    subject-name: ghcr.io/OWNER/IMAGE
    subject-digest: ${{ steps.build.outputs.digest }}
    push-to-registry: true
```

Extending the same mechanism to downloadable desktop/source archives is
planned. GitHub's native attestation is preferred over a hand-rolled
[SLSA](https://slsa.dev/spec/v1.2/provenance) statement because it avoids a
custom provenance pipeline.

## Where these attach

The current workflow's 14-asset contract is six desktop packages (a portable
archive and an installer per platform), one corresponding-source archive, and
their seven checksum files. Future SBOMs and archive attestations will expand
that contract deliberately; the publisher validates the exact set before
making a release immutable.
