# Privacy policy — proposal

**Status: proposal, and deliberately incomplete.** This is a starting draft
built from the factual inventory in
[`../data-locations-and-privacy.md`](../data-locations-and-privacy.md) — it
is **not** a compliance-ready legal document. Several sections below need a
real decision or a real legal identity filled in by whoever operates a given
xchats deployment (the repo owner, if running one; any other adopter, if
they run their own); this document cannot make those calls generically for
"self-hosted software," only for a specific operator.

This matters more than it might first appear: **xchats is self-hosted
software, not a hosted service this repository operates.** There is no
single "xchats" privacy policy that covers every deployment, the way there
would be for a SaaS product — each operator running their own instance is
the actual data controller for whatever their deployment processes, and each
one needs their own privacy notice for their own users if they have legal
obligations to provide one (GDPR, CCPA, or otherwise). What follows is a
template an operator can adapt, plus the factual baseline of what the
*software itself* does, which is the same regardless of who's running it.

## What this software processes (same for every deployment)

See [`../data-locations-and-privacy.md`](../data-locations-and-privacy.md)
for the full, current inventory. Summary:

- **Message content and media** (WhatsApp/Telegram) — stored in the local
  SQLite database and blob directory this deployment controls.
- **Knowledge-base content** an operator builds — same storage.
- **User accounts** (email, password hash, role) for people who log into
  this deployment's admin/agent UI.
- **Credentials** for connected integrations — stored separately and never
  included in a backup, see [`../credentials.md`](../credentials.md).

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
  [`../data-locations-and-privacy.md`](../data-locations-and-privacy.md) for
  the exact call.

None of these are contacted unless the corresponding feature is configured
— every one is opt-in, not on by default beyond the update check.

## Sections that need an operator's own answers

`[TODO: fill in for your specific deployment before publishing this as a
real policy — this software cannot answer these on your behalf]`

- **Data controller identity** — legal name, contact address/email of
  whoever operates this specific instance and is responsible for the data
  in it.
- **Legal basis for processing** (if GDPR applies) — e.g. contract
  performance for a customer-support tool, legitimate interest, consent.
- **Data retention period** — how long messages/KB content are kept before
  deletion; xchats itself imposes no automatic retention limit or deletion
  schedule today (see [`../data-locations-and-privacy.md`](../data-locations-and-privacy.md)'s
  "Deleting everything" section — it's a manual, filesystem-level
  operation), so a retention policy is entirely the operator's to define and
  enforce.
- **Data subject rights process** — how someone whose messages are stored
  (an end customer messaging the operator's WhatsApp/Telegram number)
  requests access to or deletion of their data, and how the operator
  actually fulfills that given today's manual deletion story.
- **Sub-processor list** — the specific LLM/tracing/tunnel providers *this*
  deployment has configured, each with a link to their own privacy policy —
  this varies per deployment, unlike the generic list above.
- **Jurisdiction and applicable law.**
- **Cookie/session disclosure**, if legally required in the operator's
  jurisdiction — xchats sets a session cookie for login (see
  `internal/httpapi`'s auth middleware); no third-party tracking or
  advertising cookies are set by the software itself.

## Children's data

`[TODO: repo owner/operator — state whether this deployment is intended for
use by/regarding minors, and what applies (e.g. COPPA) if so. Default
assumption for a B2B team-inbox product is "not directed at children," but
that's a business decision to state explicitly, not assume silently.]`

## Changes to this policy

`[TODO: describe how an operator will notify users of material changes —
e.g. a dated changelog entry, an in-app notice.]`
