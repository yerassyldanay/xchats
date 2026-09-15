# Unified GHCR Server Image

## Summary

Create `ghcr.io/yerassyldanay/xchats` as a self-contained, multi-architecture server image containing the Go service and embedded Vue bundle. Release publishing switches from the two component packages to this unified package, while the existing two-container Compose development workflow remains unchanged.

## Implementation Changes

- Add a non-desktop `go:embed` asset definition using the existing `backend/internal/desktop/dist` mirror. The Docker build copies `frontend/dist` there before compiling; ordinary backend development remains API-only when `index.html` is absent.
- Add a reusable HTTP handler that:
  - Sends `/xchats/`, `/mcp`, `/mcp/`, `/oauth/`, `/telegram/`, `/meta/`, `/.well-known/`, `/healthz`, `/readyz`, and `/playground/review-handoff` to Gin unchanged, including SSE.
  - Serves real static assets with correct MIME types and `GET`/`HEAD` support.
  - Falls back to `index.html` for Vue history routes.
  - Preserves prerendered blog directory indexes, real blog 404 responses, HTML cache policy, and applicable Nginx security headers.
- Use the unified handler for both the HTTP listener and embedded tunnel in server builds. Desktop builds retain their Wails middleware, cookie jar, and realtime transport without linking Wails/Cgo into the server binary.
- Add a root `Dockerfile` and `.dockerignore`:
  - Node 24 frontend stage: `npm ci && npm run build`.
  - Go 1.27 stage: copy the built SPA, cross-compile with `CGO_ENABLED=0`, `TARGETOS`/`TARGETARCH`, `-trimpath`, stripped symbols, and the existing `VERSION` linker value.
  - Distroless Debian 12 non-root runtime exposing `8080`, declaring `/data`, and ensuring UID 65532 can write the initial named volume.
- Add an image-specific `/config.yaml` with database, WhatsApp state, blobs, settings, and credentials rooted beneath `/data`. Set only `XCHATS_CONFIG`, `XCHATS_ALLOW_FILE_CREDENTIALS`, `XCHATS_DATA_DIR`, and `XCHATS_CONFIG_DIR` as image defaults so mounted YAML values are not shadowed by Docker environment defaults.
- Support custom configuration by replacing the baked file:
  `-v /absolute/path/config.yaml:/config.yaml:ro`.
  Environment variables continue to override YAML through the existing configuration precedence.
- Update CI to build the root image when backend, frontend, image config, or root Docker inputs change; retain current component-image CI checks because local Compose still uses their Dockerfiles.
- Change `release.yml` to publish only `ghcr.io/yerassyldanay/xchats` for `linux/amd64` and `linux/arm64`, with provenance and tags `vX.Y.Z`, `X.Y.Z`, `X.Y`, `X`, and `latest`. Per the selected policy, prereleases also update `latest`. Buildx produces one multi-platform manifest using its standard target-platform arguments. [Docker multi-platform guidance](https://docs.docker.com/build/building/multi-platform/), [metadata-action tagging](https://github.com/docker/metadata-action)
- Keep the existing desktop/source release assets and atomic release gate unchanged. Authenticate to GHCR with the scoped `GITHUB_TOKEN`, `packages: write`, and existing attestation permissions. [GitHub GHCR publishing guidance](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)
- Add minimal documentation for the pull/run command, tag scheme, `/data` persistence, custom `/config.yaml` mount, production origin/cookie overrides, and the manual first-push package visibility step. Do not change Compose or source-development instructions.

## Interfaces

- Container: port `8080`, persistent volume `/data`, optional configuration mount `/config.yaml`.
- Registry: `ghcr.io/yerassyldanay/xchats:<tag>`.
- HTTP API schemas and endpoint paths remain unchanged; only non-API paths gain embedded static/SPA handling.
- Existing `xchats-backend` and `xchats-frontend` packages remain available at their old versions but receive no new release tags.

## Test Plan

- Unit-test backend-path classification, including exact `/mcp`, Meta routes, health/readiness, OAuth, webhooks, and review handoff.
- Test static files, MIME types, `HEAD`, non-GET rejection, Vue deep-link fallback, missing-bundle development fallback, blog indexes/404s, cache headers, and API responses never becoming SPA HTML.
- Run frontend typecheck/tests/build and Go tests with `CGO_ENABLED=0`; verify the default dependency graph contains no Wails/WebView packages.
- CI-build both target architectures and smoke-test the amd64 image using the documented command:
  - `/healthz`, `/`, a hashed asset, and a Vue deep link succeed.
  - `/mcp` and representative webhook/API paths reach Gin rather than the SPA.
  - The non-root process initializes and reopens the same named `/data` volume.
  - A bind-mounted config using a different internal port/storage path demonstrably changes runtime behavior.
- Record final compressed image size and target approximately 35 MB without making an architecture-sensitive byte limit a release blocker.

## Assumptions

- Current repository pins—Node 24 and Go 1.27—supersede the older handover versions.
- Local Compose, its component Dockerfiles, Makefile commands, and two-service behavior remain intact.
- The first successful push creates the GHCR package; the owner then makes it public manually.
- Applied skills: `multi-stage-dockerfile` for image structure/security, `golang-patterns` for HTTP composition, `golang-continuous-integration` for publishing controls, and `vue-best-practices` to confirm no Vue component changes are required.
