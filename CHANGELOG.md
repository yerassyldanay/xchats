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
- Operator UI redesigned (xchats 2.0): a light, Manrope-based design
  language with a collapsible left sidebar replacing the icon rail; a new
  sign-in screen; a Главная dashboard (first steps, unread chats, overdue
  tasks, changes awaiting publication) that is now the landing page after
  sign-in; a full-screen setup wizard (AI key, WhatsApp/Telegram, ChatGPT or
  Claude knowledge-base connector, public address / ngrok); restyled Inbox,
  Customers, Tasks, Campaigns (with a three-step wizard), Knowledge Base
  (product photo grid, pending-changes strip), Draft, Channels (per-account
  AI-suggestions switch) and Settings (section nav, technical fields behind
  "Расширенные настройки"). The simulator and the knowledge-base assistant
  are now two modes of one "Проверка ассистента" page. The app's font
  (Manrope) is bundled rather than fetched from a CDN.
- The official Meta channels (WhatsApp Cloud API, Instagram Direct,
  Messenger) are switched off in the UI for now and shown as "Coming soon";
  the backend, stores and API client are untouched. Build the frontend with
  `VITE_ENABLE_META_CHANNELS=true` to bring them back.
- Channels: accounts are cards again, each with its channel's icon and brand
  colour, connection and automation badges and the quick AI switch.
- Draft has the same page anatomy as the Knowledge Base (summary strip, tabs,
  "Section · N" heading, product photo cards with before/after captions) and
  its own sidebar entry with a pending-changes badge.
- Sign-in page and the public landing page now describe the whole product
  (nine app sections, the draft-and-approve flow, supported channels). The
  landing page and the standalone Netlify visit-card use the app's design
  language; `frontend/visit-card/index.html` is generated from the landing
  page's stylesheet and copy by `npm run build:visit-card` (a test fails when
  it is stale).
- `make screenshots` now captures every app page (login, dashboard, setup,
  inbox and customer panel, customers, follow-ups, campaigns and wizard,
  knowledge base and overview, draft and connector, assistant check, channels,
  connect and automation dialogs, settings and team) into `docs/images/`.

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
