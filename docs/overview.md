# xchats Product & Architecture Overview

> **xchats** is a self-hosted omnichannel team inbox with zero-hallucination AI assistance. It unifies WhatsApp, Telegram, Instagram, and Messenger into one shared inbox where AI drafts responses grounded exclusively in your verified knowledge base, and human operators retain complete control before any message sends.

---

## 1. Product Vision & Principles

Small sales and support teams routinely juggle multiple messaging apps while answering repetitive customer inquiries about pricing, inventory, specs, and delivery. Fully autonomous bots risk hallucinating incorrect prices or promises, while fully manual inboxes overwhelm operators.

xchats solves this with three foundational principles:

1. **Zero-Hallucination AI**: The response model is never permitted to write raw prices, dates, or contact details directly. Instead, it emits strict semantic placeholders (e.g., `{{token}}`) that backend code resolves against stored, verified database values. If a token cannot be resolved, the draft fails closed and escalates to an operator.
2. **Human-in-the-Loop, Always**: Every AI-generated draft is presented to a teammate in the shared inbox. A human can approve, edit, or discard the message with one click. No automated message reaches a customer without explicit human review.
3. **Self-Hosted Simplicity (One Binary, One File)**: Written in Go with an embedded SQLite transactional database, xchats requires no external Postgres, Redis, or managed message broker. It deploys cleanly via Docker or runs as a native desktop app via Wails.

---

## 2. System Architecture

```text
Customer Channels (WhatsApp, Telegram, Instagram, Messenger)
                           │
             Webhooks / Transport Events
                           ▼
                 ┌───────────────────┐
                 │  Channel Adapters │ (wa_*, tg_*, ig_*, fb_*)
                 └─────────┬─────────┘
                           │ Normalized Events
                           ▼
                 ┌───────────────────┐      HTTPS / SSE       ┌─────────────────┐
                 │    Go Backend     │ ◄────────────────────► │ Vue 3 Frontend  │
                 │ (Modular Monolith)│                        │ (SPA / Wails)   │
                 └────┬─────┬──────┬─┘                        └─────────────────┘
                      │     │      │
          ┌───────────┘     │      └──────────┐
          ▼                 ▼                 ▼
   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
   │   SQLite    │   │ Local Blob  │   │  ModelGateway
   │ (Database)  │   │   Storage   │   │(OpenAI/Claude/
   └─────────────┘   └─────────────┘   │Gemini/Ollama)
                                       └─────────────┘
```

### Core Components

- **Channel Adapters**: Handle transport-specific protocols, authentication, QR pairing, polling/webhooks, and media downloads. Channel tables (`wa_*`, `tg_*`, etc.) isolate provider peculiarities from core domain logic.
  - **WhatsApp**: Direct pairing via `whatsmeow` (no business API fees required) and WhatsApp Cloud API support.
  - **Telegram**: Long-polling or webhook integration via the Telegram Bot API.
  - **Instagram & Messenger**: Webhook synchronization and Graph API messaging.
- **Go Backend**: Modular monolith providing REST APIs, Server-Sent Events (SSE) for real-time inbox synchronization, background worker routines, authentication, and deterministic validation engines.
- **Transactional Database (SQLite)**: Stores accounts, normalized conversations, contacts, campaigns, staged drafts, and live knowledge base tables (`ai_*`).
- **Vue 3 Frontend**: Single-page application built with Vue 3, Tailwind CSS, and Pinia, providing responsive daily inbox operations, knowledge drafting, channel management, CRM, and campaign tracking.
- **Desktop Wrapper (Wails)**: Cross-platform desktop application packaging the frontend and backend into native macOS, Windows, and Linux bundles.
- **MCP Connector**: Model Context Protocol (MCP) server implementing OAuth 2.1 PKCE. Enables external agents (ChatGPT, Claude Desktop) to inspect documents and stage knowledge additions securely.

---

## 3. Core Lifecycles & Data Flow

### A. Customer Response Suggestion Flow

```text
Customer Message
       │
       ▼
1. Channel Adapter receives and authenticates incoming webhook or socket event
2. Event is normalized and committed to the database
3. Backend checks conversation context and loads approved live knowledge (ai_* only)
4. PromptBuilder generates prompt with approved KB context and strict formatting rules
5. ModelGateway calls LLM provider, returning structured JSON (draft text, tokens, media)
6. ResponseValidator verifies that all placeholders and media tokens exist in live KB
7. Exact business values are substituted into the template
8. rp_suggestions records the suggested draft
9. Operator reviews suggestion in UI: Approve (Send), Edit, or Discard
10. Channel Adapter sends message to customer channel upon approval
```

### B. Knowledge Authoring & Staged Ingestion Flow

```text
Operator Input (Files, URLs, Text, MCP Tools)
       │
       ▼
1. Ingestion creates durable kbd_materials record and persists raw bytes
2. Pass 1 (Extraction): Document/URL content is parsed into structured evidence
3. Pass 2 (Synthesis): Evidence is synthesized into validated candidate entries
4. Validation: Schemas, entity handles, and natural keys are strictly validated
5. Staged in kbd_draft: Delta preview displayed with visual before/after diffs
6. Operator Review: Operator inspects staged changes and clicks "Publish All"
7. Atomic Promotion: Selected changes transactionally update live ai_* tables
8. Brain Reload: Prompt cache updates; new knowledge is immediately active
```

---

## 4. Non-Negotiable Invariants

1. **Isolation of Live vs. Staged Knowledge**: Customer-facing models only ever see approved rows from `ai_*`. Drafts (`kbd_draft`), raw uploaded files, extraction debug notes, and internal system IDs are never injected into response prompts.
2. **Placeholders for Exact Numbers**: Prices, tariffs, contact numbers, and dates must be emitted as placeholders by the model and resolved by deterministic backend code from verified table columns. An unrecognized placeholder aborts the suggestion.
3. **Durable Materials Before Extraction**: User-uploaded source files are durably stored in blob storage before background extraction begins. Extraction failure never results in data loss.
4. **Controlled Media Distribution**: Files and product images can only be sent to customers if registered under an approved `ai_*` media column. Models refer to media exclusively via generated semantic tokens (e.g., `products.sofa.gallery`), never direct URLs or file paths.
5. **Fail-Closed Safety**: Any ambiguity, validation error, schema mismatch, or missing data immediately fails closed—suppressing automated sending and prompting manual human intervention.

---

## 5. Key Feature Areas

- **Shared Team Inbox**: Centralized message threads across all channels with real-time SSE updates, operator assignments, and one-click draft approvals.
- **Visual Draft Staging**: Side-by-side diff review of knowledge base updates before publishing.
- **Mini CRM & Follow-up Board**: Contact profiles, custom tags, and scheduled follow-up tasks organized into *Overdue*, *Today*, *Tomorrow*, and *Later*.
- **Outbound Campaigns**: Rate-limited broadcast messaging with live per-recipient delivery statuses and send-window enforcement.
- **Channel Simulator & Evals**: Built-in test sandbox to simulate customer conversations against live or staged knowledge bases without connecting real channel accounts.

---

## 6. Repository Structure

```text
xchats/
├── backend/            # Go modular monolith (handlers, services, DB migrations)
│   ├── cmd/xchats/     # CLI commands and server entrypoint
│   ├── internal/       # Core domain packages (channels, brain, store, httpapi)
│   └── migrations/     # SQLite database migrations
├── frontend/           # Vue 3 SPA (Tailwind CSS, Pinia, Vue Router)
│   ├── src/components/ # Reusable UI components (inbox, channels, CRM, simulator)
│   └── src/views/      # Top-level route views
├── deploy/             # Docker Compose configurations and deployment scripts
├── desktop/            # Wails desktop application configuration
├── docs/               # Technical and operational documentation
│   ├── release/        # Production setup, backups, Docker, security
│   ├── images/         # Architecture diagrams and UI screenshots
│   └── overview.md     # This document
├── evals/              # AI quality evaluation harness
└── Makefile            # Primary build and development targets
```
