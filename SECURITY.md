# Security policy

## Reporting a vulnerability

**Do not open a public GitHub issue for a security vulnerability.**

Report privately via [GitHub's private vulnerability reporting](https://github.com/yerassyldanay/xchats/security/advisories/new)
(the **Security** tab on this repository → **Report a vulnerability**).

Include, if you can:

- What kind of issue it is (e.g. credential exposure, auth bypass, injection,
  denial of service).
- Steps to reproduce, or a proof of concept.
- The version/commit you tested against.
- Impact as you understand it — what an attacker could actually do.

## What to expect

xchats is maintained by a small team without dedicated security staff, so
the honest baseline is best-effort, not a staffed SLA:

- **Acknowledgment:** within 5 business days.
- **Initial assessment** (confirmed / not a vulnerability / need more info):
  within 10 business days.
- **Fix timeline:** communicated once severity is assessed — critical
  issues (remote credential/data exposure, auth bypass) are prioritized over
  everything else in flight.

## Scope

**In scope:**

- The `backend/` Go service and `frontend/` Vue app, as built from this
  repository.
- The Docker images in `deploy/` as published (if/when they are — see
  [`docs/release/release-checklist.md`](docs/release/release-checklist.md)).

**Out of scope:**

- Vulnerabilities in third-party dependencies themselves — report those
  upstream (see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) for the
  dependency list), though a report here about how xchats *uses* a
  vulnerable dependency version is welcome.
- A deployment's own operational security (an operator running an
  unpatched OS, an exposed database port, a bootstrap admin password they
  never rotated — see [`docs/release/installation.md`](docs/release/installation.md)
  on why that needs doing) — that's the operator's responsibility, not a
  vulnerability in xchats itself, though documentation gaps that make an
  insecure default more likely are fair game to report.
- Social engineering, physical access, or denial-of-service against
  third-party infrastructure (ngrok, LLM providers, GitHub's API) that
  xchats merely calls into.

## Supported versions

Until a stable release history exists, only the latest released version and
`master` are supported. This section will be updated once tagged releases
exist — see [`docs/release/release-checklist.md`](docs/release/release-checklist.md).

## Known, deliberate security tradeoffs (not vulnerabilities)

Documenting these up front so they aren't re-reported as findings — each is
an explicit, recorded design decision, not an oversight:

- **The ngrok tunnel serves the full application, not just an API
  allowlist** — including the login page — while running. Recorded in
  [`plan/DECISIONS.md`](plan/DECISIONS.md)'s 2026-08 tunnel amendment;
  mitigated by cookies gaining `Secure` over the tunnel's HTTPS origin and
  an explicit UI warning before starting it. See
  [`docs/release/troubleshooting.md`](docs/release/troubleshooting.md).
- **The file-backed credential store is weaker than an OS keychain** — it's
  encrypted at rest, but without OS-level access control, and requires
  explicit opt-in (`XCHATS_ALLOW_FILE_CREDENTIALS=1`) plus an acknowledgment
  in the UI before it's used. See
  [`docs/release/credentials.md`](docs/release/credentials.md).
- **The default admin credential (`admin@xchat.kz` /
  `xchat-admin-change-me`) is a public, shared default** — printed in the
  README and committed in this repo's migration history — restored on every
  fresh install for self-host ergonomics. There is no forced first-login
  change anymore: a deployment that never rotates this password is open to
  anyone who reads the docs, and changing it is the operator's
  responsibility, not something xchats enforces. `xchats
  reset-admin-password` restores this same default if a changed password is
  ever lost. Documented in
  [`docs/release/installation.md`](docs/release/installation.md).

## WhatsApp connectivity

xchats connects to WhatsApp using [whatsmeow](https://github.com/tulir/whatsmeow),
an unofficial, reverse-engineered client — not WhatsApp's official Business
API. This is a product tradeoff, not a vulnerability, but it has real
consequences worth stating plainly: a connected number can be banned by
WhatsApp at their discretion, with no recourse. Don't pair a number you
can't afford to lose.

## Acknowledgments

Once this policy is live and reports come in, reporters will be credited
here (with their permission) as a small, genuine thank-you for good-faith
reports.
