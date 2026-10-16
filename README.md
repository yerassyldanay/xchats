<div align="center">

# xchats

**The self-hosted omnichannel inbox with zero-hallucination AI.**
WhatsApp and Telegram in one shared inbox — an AI assistant drafts every
reply from your own knowledge base, and a human approves it before it sends.
Customer profiles, follow-ups and campaigns included; Instagram, Messenger and
WhatsApp Cloud API are coming soon.

[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](backend)
[![Vue 3](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](frontend)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)](docs/release/data-locations-and-privacy.md)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](deploy)
[![CI](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml/badge.svg)](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml)

**English** · [Русский](README.ru.md) · [Қазақша](README.kk.md)

</div>

![xchats overview](docs/images/social-preview.png)

---

## 60-second quickstart

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make up
make seed
```

Open **http://localhost:8081** — sign in with `admin@xchat.kz` /
`xchat-admin-change-me` (public default; change it after your first login).
`make seed` fills the local instance with the Qazan Home kitchen-appliance
demo and is safe to run again.

## Key superpowers

- **Zero-hallucination replies** — the model never writes a price, date
  or contact detail itself. It emits a `{{token}}`; the backend substitutes
  the exact stored value, or the draft fails closed and escalates to a human.
- **Human-in-the-loop, always** — every draft is a suggestion. Nothing
  reaches a customer until a teammate clicks Send.
- **Choose SQLite or PostgreSQL** — Go + embedded SQLite by default, with
  PostgreSQL available through `storage.database_target`. See [Database architecture and migrations](docs/database.md). No Redis,
  no managed cloud service required to run this.
- **Model-agnostic** — OpenAI, Claude, Gemini, OpenRouter or a local
  Ollama model; swap providers from Settings, not code.
- **Configurable from ChatGPT / Claude** — an MCP connector lets an LLM
  client read your documents and stage knowledge-base edits for your review.

## Visual tour

These are real screens from a seeded Qazan Home instance, not mockups. They are
regenerated from the running app by `make screenshots`.

### 1. Sign-in and first steps

![Sign-in page: what xchats is and the six things it does](docs/images/login.png)
![The Dashboard with unread chats, overdue follow-ups and changes waiting in the Draft](docs/images/dashboard.png)
![The setup wizard asking where customers write to you](docs/images/setup.png)

The sign-in page says what xchats is and what it does. After signing in, the
Dashboard shows what needs attention (unread chats, overdue follow-ups, changes
waiting in the Draft) and, for a new workspace, a "Getting started" checklist;
the setup wizard walks an admin through the AI key, a first channel and the
knowledge base.

### 2. Shared inbox

![A customer conversation beside a grounded AI reply ready for human review](docs/images/inbox.png)
![The customer panel beside a chat: profile, status, tags, next step and notes](docs/images/customer-panel.png)

WhatsApp and Telegram conversations share one queue. AI replies remain
suggestions until a teammate edits, approves or discards them; the customer
panel keeps the profile, status, tags, next step and notes beside the chat.

### 3. Channels and automation

![Channel cards with channel icons, health and automation states; Meta channels marked Coming soon](docs/images/channels.png)
![The channel picker for WhatsApp and Telegram, with the Meta channels marked Coming soon](docs/images/channel-connect.png)
![The per-channel automation dialog with the auto-reply schedule presets](docs/images/automation.png)

Connect WhatsApp by QR code or a Telegram bot. Every account is a card with its
own channel icon, health, an AI-suggestions switch and a schedule for
auto-reply (always, nights, weekends or custom hours). Instagram, Messenger and
WhatsApp Cloud API are implemented in the backend but switched off in the UI
for now ("Coming soon"); build the frontend with
`VITE_ENABLE_META_CHANNELS=true` to bring them back.

### 4. Grounded knowledge base

![Products with photos, names and exact prices in the Knowledge Base](docs/images/knowledge-base.png)
![The Knowledge Base overview: the assistant's persona, mission, guardrails and language policy](docs/images/knowledge-base-overview.png)

Products, tariffs, delivery zones, contacts and policies are the only facts
the assistant may use, together with its persona and guardrails. Numbers
become `{{token}}` placeholders and are filled from stored values; see the
[grounding pipeline](docs/images/grounding.svg).

### 5. Draft: review and ChatGPT / Claude

![A staged price change shown as a before and after card in the Draft](docs/images/draft-staging.png)
![The ChatGPT and Claude MCP connector setup inside the Draft page](docs/images/mcp-connect.png)

The Draft page has the same anatomy as the Knowledge Base. Links, files and the
MCP connector all write to it; ChatGPT or Claude can use 13 `kb_*` tools after
OAuth 2.1 authorization, but every change shows up as a before/after card and
still needs human review before publishing.

### 6. Test the assistant

![The customer-reply simulator with suggested test questions](docs/images/simulator.png)
![The private operator assistant with example questions about live and staged knowledge](docs/images/assistant.png)

Rehearse customer questions against the live Knowledge Base or the Draft — no
real phone number needed — or ask the private operator assistant about live
facts and pending changes. Neither ever messages a customer. Compare prompts
and models with the open [`evals/`](evals/) harness.

### 7. Customers and follow-ups

![Customer profiles with status, tags and channel identities](docs/images/customers.png)
![Follow-ups grouped into Overdue, Today, Tomorrow and Later](docs/images/followups.png)

The mini CRM keeps identities, notes, tags and ownership together; its daily
board tracks overdue, upcoming and completed follow-ups.

### 8. Campaigns

![Campaign delivery progress and draft broadcasts](docs/images/campaigns.png)
![The new-campaign wizard: audience, message and schedule steps](docs/images/campaign-wizard.png)

A three-step wizard — audience, message, schedule — imports recipients,
previews templates and sets rate limits and send windows; then every delivery
and reply is tracked in the shared inbox.

### 9. Settings and team

![Settings for the AI engine, parsers, monitoring, remote access, channels, team and backups](docs/images/settings.png)
![Settings: team members and account security](docs/images/settings-team.png)

Configure model providers, extraction, Langfuse monitoring, remote access,
channels, team access and backups from the UI — no secrets belong in
`config.yaml`.

## How it works

1. A customer messages you on WhatsApp or Telegram.
2. It lands in **one inbox**, shared by your whole team.
3. The assistant drafts a reply from your approved knowledge base only.
4. A person reviews, edits or discards it — then sends.

The full design lives in [`docs/overview.md`](docs/overview.md); the grounding
mechanism is summarized in the [visual tour](#4-grounded-knowledge-base).

## Run it another way

- **From source** (dev): `make dev-backend` + `make dev-frontend` — see
  [`docs/release/installation.md`](docs/release/installation.md).
- **Desktop app** (Wails — Windows/macOS/Linux): `make desktop-build` — see
  [`docs/desktop.md`](docs/desktop.md).
- `make help` lists every target.

> [!WARNING]
> The WhatsApp channel connects like WhatsApp Web (no Business API fee), but
> it is an unofficial client — start with a number you can afford to lose.
> Details in the [visual tour](#3-channels-and-automation).

## Documentation

- [`docs/overview.md`](docs/overview.md) — product and architectural overview
- [`docs/release/installation.md`](docs/release/installation.md) — every install path
- [`docs/desktop.md`](docs/desktop.md) — the desktop app
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — dev setup, conventions, PRs
- [`SECURITY.md`](SECURITY.md) — reporting a vulnerability

## License

[AGPL-3.0-only](LICENSE) — chosen after a GPL-3.0 dependency (pulled in
transitively through the WhatsApp integration) was found statically linked
into the backend; see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
