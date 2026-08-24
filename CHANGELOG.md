# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Campaigns: bulk outbound messaging to a pasted or uploaded recipient
  list, rate-limited per sending account with a scheduled send window,
  automatic retry on transient failures, and auto-pause when the sending
  account disconnects. Replies land in the normal inbox like any other
  conversation. A campaign can be deleted until it has sent anything, so an
  abandoned draft doesn't linger in the list.
- WhatsApp and Telegram team inbox with a draft-and-approve AI assistant
  grounded in a structured knowledge base.
- MCP (Model Context Protocol) server: connect ChatGPT/Claude directly to
  manage the knowledge base and configure the assistant.
- Self-service password change; a per-install, randomly-generated bootstrap
  admin credential replaces the old shared default.
- CI: backend/frontend test suites, CodeQL, secret scanning (gitleaks),
  Dependabot, and a tag-triggered release workflow with build provenance
  attestation.
- Desktop application for Windows, macOS and Linux, built with Wails: the
  same backend binary with a WebView window attached, shipping the Vue UI
  embedded and storing SQLite and all local data under the OS
  application-data directory. Built for all three platforms by a GitHub
  Actions matrix; auto-update is not implemented. See `docs/desktop.md`.

### Changed
- Database layer ported from PostgreSQL to SQLite (no separate database
  service to run).

### Security
- Retired the shared default admin password in favor of a one-time,
  randomly-generated bootstrap credential with forced rotation on first
  login.
- Closed a cross-organization IDOR on the media-serving endpoint.
- Added rate limiting to login and password-change; CORS credential
  handling now only matches exact origins, never a wildcard.
- Media uploads are now MIME-sanity-checked and size-bounded; served back
  with `X-Content-Type-Options`, `Content-Security-Policy`, and
  `Referrer-Policy` headers.
- Telegram webhook requests are now rejected outright when no webhook
  secret is configured, instead of skipping verification.
- Adopted [AGPL-3.0-only](LICENSE) as the project license, after a
  dependency audit found a GPL-3.0 transitive dependency
  (see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)).

## [0.1.0] - Unreleased

Initial public release. See the [Unreleased] section above until this
version is actually tagged, at which point this heading gains a real date
and a fresh `[Unreleased]` section starts above it — see
[`docs/release/release-checklist.md`](docs/release/release-checklist.md).
