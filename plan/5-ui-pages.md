# UI Pages

The frontend is a single Vue 3 SPA (see `2-architecture.md`). It talks only to the backend
(`/api` + SSE). Below are the v1 pages. The centerpiece is the **Chatboard** (reference:
`./ui-chatboard.png`).

## Login

Email + password → session. Minimal. Redirects to the Chatboard.

## Chatboard (Inbox) — the main page

A three-pane, Chatwoot-style team inbox. Reference screenshot: `./ui-chatboard.png`.

- **Left nav rail** — Inbox/Conversations, Contacts, WhatsApp accounts, AI assistant, Settings;
  the current org + user at the bottom.
- **Conversation list (pane 1)** — search; filter tabs (unassigned / mine / all, by status);
  each row shows avatar, contact name, last-message preview, time, unread badge, and which
  WhatsApp account it came in on. Live-updates over SSE (`conversation.*`, `message.created`).
- **Chat view (pane 2)** — header with contact name + phone and actions (assign, resolve);
  message timeline with inbound (left) / outbound (right) bubbles, media cards (image / file /
  audio / video), date separators, and **delivery/read ticks** (from `messages.update`); a
  composer at the bottom (text, attachments, send). Messages sent from the phone/WhatsApp Web
  appear here too (outbound `external_account`).
- **Right panel (pane 3)**
  - **Auto-responder** — shows/sets the org auto-response mode (`NEVER / CONFIGURE_TIME / ALWAYS`)
    and time window.
  - **Profile** — the contact's attributes/identities (phone, `@lid`, push name) and metadata.
  - **AI Recommendations** — the AI-suggested reply and suggested media for the open
    conversation (`ai_draft.created`); approve → sends through the normal send pipeline.
  - **Interaction history** — recent activity on the contact.

## Contacts

Searchable contact list; a contact detail view showing identities (phone / `@lid` / push name),
attributes, and the contact's conversations.

## WhatsApp Accounts

A simplified Evolution `/manager` (see `2-architecture.md` → *WhatsApp accounts*):
- Table of **all Evolution instances** with connection status and an **assign / unassign** control
  (only assigned instances are handled by xchats).
- **Add WhatsApp account** → modal: enter an instance name + scan instructions → show the **QR**
  → on successful scan it is **assigned** to the org (with **unassign**).

## AI Assistant

Configure the assistant (ported brain): **persona**, **knowledge/topics**, **prices**, **media
assets**; a **playground** to dry-run a draft against a sample conversation; publish / version the
config. All stored in Postgres.

## Settings

- **Organization** — auto-response mode + time; org profile.
- **Users** — list members; **add user** (email + password); they join the default org.
- **General** — app preferences.

## Out of scope (v1)

Reports/analytics, campaigns, SLA, macros, billing, granular permissions — deferred (per
`1-concept.md`). The nav may show placeholders but they are not built in v1.
