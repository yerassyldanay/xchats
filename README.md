<div align="center">

# xchats

**The self-hosted omnichannel inbox with zero-hallucination AI.**
WhatsApp, Telegram, Instagram and Messenger in one shared inbox — an AI
assistant drafts every reply from your own knowledge base, and a human
always approves before it sends.

[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](backend)
[![Vue 3](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](frontend)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)](docs/release/data-locations-and-privacy.md)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](deploy)
[![CI](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml/badge.svg)](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml)

**English** · [Русский](README.ru.md) · [Қазақша](README.kk.md)

</div>

![xchats overview](docs/images/overview-bento.png)

**[Explore the Complete Visual Tour & Screenshots →](docs/tour.md)**

---

## 60-second quickstart

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make up && make seed-demo
```

Open **http://localhost:8081** — sign in with `admin@xchat.kz` /
`xchat-admin-change-me` (public default; change it after your first login).

## Key superpowers

- **Zero-hallucination replies** — the model never writes a price, date
  or contact detail itself. It emits a `{{token}}`; the backend substitutes
  the exact stored value, or the draft fails closed and escalates to a human.
- **Human-in-the-loop, always** — every draft is a suggestion. Nothing
  reaches a customer until a teammate clicks Send.
- **One binary, one file** — Go + embedded SQLite. No Postgres, no Redis,
  no managed cloud service required to run this.
- **Model-agnostic** — OpenAI, Claude, Gemini, OpenRouter or a local
  Ollama model; swap providers from Settings, not code.
- **Configurable from ChatGPT / Claude** — an MCP connector lets an LLM
  client read your documents and stage knowledge-base edits for your review.

## How it works

1. A customer messages you on WhatsApp, Telegram, Instagram or Messenger.
2. It lands in **one inbox**, shared by your whole team.
3. The assistant drafts a reply from your approved knowledge base only.
4. A person reviews, edits or discards it — then sends.

The full design lives in [`plan/architecture.md`](plan/architecture.md); the
grounding mechanism above is diagrammed in the
[visual tour](docs/tour.md#2-grounded-knowledge-base--strict-token-replacement).

## Run it another way

- **From source** (dev): `make dev-backend` + `make dev-frontend` — see
  [`docs/release/installation.md`](docs/release/installation.md).
- **Desktop app** (Wails — Windows/macOS/Linux): `make desktop-build` — see
  [`docs/desktop.md`](docs/desktop.md).
- `make help` lists every target.

> [!WARNING]
> The WhatsApp channel connects like WhatsApp Web (no Business API fee), but
> it is an unofficial client — start with a number you can afford to lose.
> Details in the [visual tour](docs/tour.md#1-team-inbox--omnichannel-sync).

## Documentation

- [Visual tour](docs/tour.md) — every screen, explained
- [`docs/release/installation.md`](docs/release/installation.md) — every install path
- [`docs/desktop.md`](docs/desktop.md) — the desktop app
- [`plan/`](plan/) — the design record, starting at [`plan/README.md`](plan/README.md)
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — dev setup, conventions, PRs
- [`SECURITY.md`](SECURITY.md) — reporting a vulnerability

## License

[AGPL-3.0-only](LICENSE) — chosen after a GPL-3.0 dependency (pulled in
transitively through the WhatsApp integration) was found statically linked
into the backend; see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
