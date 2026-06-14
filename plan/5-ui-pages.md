# UI Pages

The frontend is a single Vue 3 SPA (see `2-architecture.md`). It talks only to the backend
(`/api` + SSE). **v1 ships exactly two pages — Login and Chatboard.** Everything below the
Chatboard (Contacts, WhatsApp Accounts, AI Assistant, Settings) is **deferred** — designed here but
shown only as nav placeholders in v1. The centerpiece is the **Chatboard** (reference:
`./ui-chatboard.png`, which is **aspirational** — it shows deferred panels).

## Login

Email + password → session. Minimal. Redirects to the Chatboard.

## Chatboard (Inbox) — the main page

A three-pane, Chatwoot-style team inbox. Reference screenshot: `./ui-chatboard.png`.

- **Left nav rail** — Inbox/Conversations (the only live destination in v1); Contacts, WhatsApp
  accounts, AI assistant, Settings appear as **deferred placeholders**; the current org + user at
  the bottom.
- **Conversation list (pane 1)** — search; filter tabs (unassigned / mine / all, by status);
  each row shows avatar, contact name, last-message preview, time, unread badge, and which
  WhatsApp account it came in on. Live-updates over SSE (`conversation.*`, `message.created`).
- **Chat view (pane 2)** — header with contact name + phone and actions (assign, resolve);
  message timeline with inbound (left) / outbound (right) bubbles, media cards (image / file /
  audio / video), date separators, and **delivery/read ticks** (from `messages.update`); a
  composer at the bottom (text, attachments, send). Messages sent from the phone/WhatsApp Web
  appear here too (outbound `external_account`).
- **Right panel (pane 3)** — v1 has **two sections only** (the reference screenshot's Auto-responder
  and Interaction-history panels are **deferred**):
  - **AI draft card** — the suggested reply for the open conversation. A **"Suggest reply"** button
    triggers a draft on demand (v1 trigger; auto-draft-on-inbound is a fast-follow). When a draft
    exists:
    - **Approve (one tap)** sends the unedited text through the normal send pipeline
      (`sender_kind='ai'`). **Edit** loads the draft text into the **pane-2 composer** (Enter to
      send) — the familiar path, not a second editor.
    - **Escalation must be legible:** when the brain has no KB answer it returns a holding reply —
      the card shows *"AI doesn't know this — reply manually"* + the `escalation_reason`, never a
      confident-looking wrong draft. Low confidence shows a badge. (Suggested **media** is **not** rendered in v1.)
    - Approve is idempotent: once sent (or superseded by a newer inbound), the card greys out via
      `ai_draft.updated` so a second agent can't double-send.
  - **Contact mini-profile** — the contact's key attributes/identities (phone, `@lid`, push name).

## Deferred pages (v2+ — designed, not built in v1)

These are kept as future design; v1 shows them only as nav placeholders.

### Contacts (deferred)

Searchable contact list; a contact detail view showing identities (phone / `@lid` / push name),
attributes, and the contact's conversations.

### WhatsApp Accounts (deferred — v1 uses one pre-connected account from config)

A simplified Evolution `/manager` (see `2-architecture.md` → *WhatsApp accounts*):
- Table of **all Evolution instances** with connection status and an **assign / unassign** control
  (only assigned instances are handled by xchats).
- **Add WhatsApp account** → modal: enter an instance name + scan instructions → show the **QR**
  → on successful scan it is **assigned** to the org (with **unassign**).

### AI Assistant (deferred — v1 seeds the KB from `0002_seed.sql`/markdown, no UI)

Configure the assistant (ported brain): **persona**, **knowledge/topics**, **prices**, **media
assets**; a **playground** to dry-run a draft against a sample conversation; publish / version the
config. All stored in Postgres.

### Settings (deferred)

- **Organization** — auto-response mode + time; org profile.
- **Users** — list members; **add user** (email + password); they join the default org.
- **General** — app preferences.

## Out of scope (v1)

Reports/analytics, campaigns, SLA, macros, billing, granular permissions — deferred (per
`1-concept.md`). The nav may show placeholders but they are not built in v1.
