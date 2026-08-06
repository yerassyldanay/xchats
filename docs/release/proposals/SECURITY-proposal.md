# Security policy — proposal

**Status: proposal.** No `SECURITY.md` exists in this repo today, which
means there's currently no documented way for someone who finds a
vulnerability to report it responsibly — they either have to guess, or
default to a public issue, which is the worst outcome for everyone
(the report, and any exploit detail in it, becomes public before a fix
ships). Adopting a policy like this — with a real contact channel filled
in — closes that gap.

## Reporting a vulnerability

**Do not open a public GitHub issue for a security vulnerability.**

Report privately via:
`[TODO: repo owner — fill in a real security contact: a dedicated email
address, or GitHub's private "Report a vulnerability" flow under this
repo's Security tab, which requires enabling GitHub's private vulnerability
reporting feature first]`.

Include, if you can:

- What kind of issue it is (e.g. credential exposure, auth bypass, injection,
  denial of service).
- Steps to reproduce, or a proof of concept.
- The version/commit you tested against.
- Impact as you understand it — what an attacker could actually do.

## What to expect

`[TODO: repo owner — commit to real numbers here once there's someone
positioned to honor them; a solo-maintained project should set expectations
accordingly rather than promise an SLA nobody's staffed to meet]`. A
reasonable starting point for a project this size:

- **Acknowledgment:** within 5 business days.
- **Initial assessment** (confirmed / not a vulnerability / need more info):
  within 10 business days.
- **Fix timeline:** communicated once severity is assessed — critical
  issues (remote credential/data exposure, auth bypass) prioritized over
  everything else in flight.

## Scope

**In scope:**

- The `backend/` Go service and `frontend/` Vue app, as built from this
  repository.
- The Docker images in `deploy/` as published (if/when they are — see
  [`../release-checklist.md`](../release-checklist.md)).

**Out of scope:**

- Vulnerabilities in third-party dependencies themselves — report those
  upstream (see [`THIRD_PARTY_NOTICES-proposal.md`](THIRD_PARTY_NOTICES-proposal.md)
  for the dependency list), though a report here about how xchats *uses* a
  vulnerable dependency version is welcome.
- A deployment's own operational security (an operator running an
  unpatched OS, an exposed database port, a weak admin password they never
  changed from the seeded default — see
  [`../installation.md`](../installation.md) on why that default needs
  changing) — that's the operator's responsibility, not a vulnerability in
  xchats itself, though documentation gaps that make an insecure default
  more likely are fair game to report.
- Social engineering, physical access, or denial-of-service against
  third-party infrastructure (ngrok, LLM providers, GitHub's API) that
  xchats merely calls into.

## Supported versions

`[TODO: repo owner — until a real versioning/support policy exists, the
honest default is: only the latest released version, and `main`, are
supported. State that explicitly here once releases exist — see
../release-checklist.md.]`

## Known, deliberate security tradeoffs (not vulnerabilities)

Documenting these up front so they aren't re-reported as findings — each is
an explicit, recorded design decision, not an oversight:

- **The ngrok tunnel serves the full application, not just an API
  allowlist** — including the login page — while running. Recorded in
  `plan/DECISIONS.md`'s 2026-08 tunnel amendment; mitigated by cookies
  gaining `Secure` over the tunnel's HTTPS origin and an explicit UI warning
  before starting it. See [`../troubleshooting.md`](../troubleshooting.md).
- **The file-backed credential store is weaker than an OS keychain** — it's
  encrypted at rest, but without OS-level access control, and requires
  explicit opt-in (`XCHATS_ALLOW_FILE_CREDENTIALS=1`) plus an acknowledgment
  in the UI before it's used. See [`../credentials.md`](../credentials.md).
- **The seeded admin account** (`admin@xchat.kz` / a well-known default
  password) is a deliberate bootstrap credential for first login, documented
  in [`../installation.md`](../installation.md) with an explicit instruction
  to replace it — not a hidden backdoor.

## Acknowledgments

`[TODO: repo owner — once this policy is live and reports come in, credit
reporters here (with their permission) — a "hall of fame" section is a
cheap, genuine way to encourage future good-faith reports.]`
