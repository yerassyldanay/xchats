# Privacy

**xchats is self-hosted software, not a hosted service this repository
operates.** There is no single "xchats" privacy policy that covers every
deployment, the way there would be for a SaaS product — each operator
running their own instance is the data controller for whatever their
deployment processes, and each one needs their own privacy notice for their
own end users if they have legal obligations to provide one (GDPR, CCPA, or
otherwise). What follows is the factual baseline of what the *software
itself* does — the same regardless of who runs it — plus a template an
operator can adapt into their own real policy.

## What this software processes (same for every deployment)

See [`docs/release/data-locations-and-privacy.md`](docs/release/data-locations-and-privacy.md)
for the full, current inventory. Summary:

- **Message content and media** (WhatsApp/Telegram) — stored in the local
  SQLite database and blob directory this deployment controls.
- **Knowledge-base content** an operator builds — same storage.
- **User accounts** (email, password hash, role) for people who log into
  this deployment's admin/agent UI.
- **Credentials** for connected integrations — stored separately and never
  included in a backup, see
  [`docs/release/credentials.md`](docs/release/credentials.md).

## What leaves a deployment, and where it goes

- **LLM providers** (OpenRouter/OpenAI/Gemini, whichever is configured) —
  message/KB content an operator's AI features act on is sent to whichever
  provider is configured, subject to that provider's own privacy policy and
  data-retention practices, entirely outside this software's control.
- **Langfuse** (only if an operator opts into tracing) — LLM call traces,
  which may include prompt/response content, sent to whatever Langfuse host
  is configured (their cloud, or a self-hosted Langfuse instance).
- **ngrok** (only if an operator starts a tunnel) — traffic to the tunnel's
  public URL is relayed through ngrok's infrastructure while the tunnel is
  running.
- **GitHub's public releases API** — an unauthenticated version check with
  no deployment-identifying data sent; see
  [`docs/release/data-locations-and-privacy.md`](docs/release/data-locations-and-privacy.md)
  for the exact call.

None of these are contacted unless the corresponding feature is configured
— every one is opt-in, not on by default beyond the update check.

## Template: sections an operator fills in for their own deployment

If you operate an xchats deployment that processes real customers' WhatsApp
or Telegram messages, you likely need your own privacy notice for those
customers. This software cannot answer the following on your behalf —
they depend on who you are, where you operate, and what you've configured:

- **Data controller identity** — your legal name and a contact
  address/email, as the operator responsible for the data in your instance.
- **Legal basis for processing** (if GDPR applies) — e.g. contract
  performance for a customer-support tool, legitimate interest, consent.
- **Data retention period** — xchats itself imposes no automatic retention
  limit or deletion schedule (see
  [`docs/release/data-locations-and-privacy.md`](docs/release/data-locations-and-privacy.md)'s
  "Deleting everything" section — it's a manual, filesystem-level
  operation), so a retention policy is entirely yours to define and enforce.
- **Data subject rights process** — how someone whose messages you store
  (an end customer messaging your WhatsApp/Telegram number) can request
  access to or deletion of their data, and how you actually fulfill that
  given today's manual deletion story.
- **Sub-processor list** — the specific LLM/tracing/tunnel providers *your*
  deployment has configured, each with a link to their own privacy policy.
- **Jurisdiction and applicable law.**
- **Cookie/session disclosure**, if legally required in your jurisdiction —
  xchats sets a session cookie for login (see `internal/httpapi`'s auth
  middleware); no third-party tracking or advertising cookies are set by
  the software itself.

## Children's data

xchats is built as a B2B team-inbox tool and is not directed at children.
If your deployment is used in a context involving minors' data (e.g. COPPA
applies to you), that's a determination and a compliance obligation that's
yours to make and meet — this software doesn't make it for you.

## Changes to this document

Material changes to what the *software itself* processes or sends
externally will be noted in [`CHANGELOG.md`](CHANGELOG.md) under a
`Security` or `Changed` entry. This document does not track any specific
operator's own policy changes — that's the operator's responsibility to
communicate to their own end users.
