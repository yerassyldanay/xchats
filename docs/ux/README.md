# xchats UX Flows & Implementation Roadmap

> **Verified baseline:** 2026-08-30 against the Vue components, Vue Router guards, Pinia stores, and relevant backend handlers.
>
> **Purpose:** These documents are living implementation roadmaps. They describe the current user-visible flow, preserve already implemented audit history, and define the remaining work with stable IDs and acceptance criteria.

## How to Use These Documents

1. Open the relevant flow and read the Mermaid diagram for current behavior.
2. Pick a stable issue ID such as `INB-13` or `CAM-09`.
3. Implement only that issue’s target behavior and acceptance criteria.
4. Keep state ownership explicit: route/store logic belongs in the router or Pinia store; presentational feedback belongs in the focused component.
5. Add tests named after the issue ID where practical.
6. Update the document in the same change:
   - change the issue status to Implemented;
   - record the final behavior and test coverage;
   - update the current-flow diagram if behavior changed;
   - retain the ID so code comments, commits, and later discussion remain traceable.

Do not delete implemented findings merely to reduce the count. Move them into an implemented-history section so the product does not regress.

## Priority Definitions

| Priority | Meaning |
|---|---|
| `P0` | Can expose one customer’s data in another customer’s context or send to an audience the operator did not approve. |
| `P1` | Can lose customer work, hide data, trigger an irreversible action, break a core workflow, or prevent secure administration. |
| `P2` | Significant recurring friction, unclear recovery, accessibility, or missing workflow guidance. |
| `P3` | Useful refinement with lower operational risk. |

## Flows Index

| Journey | Current state | Open roadmap items | File |
|---|---|---:|---|
| First-Time Onboarding | Baseline refactor implemented; no blocking wizard | 4 | [01-onboarding.md](flows/01-onboarding.md) |
| WhatsApp QR | Main pairing and recovery refactor implemented | 3 | [02-connect-whatsapp-qr.md](flows/02-connect-whatsapp-qr.md) |
| Telegram | Main connection flow implemented; delivery semantics/localization remain | 3 | [03-connect-telegram.md](flows/03-connect-telegram.md) |
| Instagram & Messenger | Guided setup and OAuth recovery implemented; Page selection/durability remain | 4 | [03b-connect-instagram-messenger.md](flows/03b-connect-instagram-messenger.md) |
| Knowledge Base | Existing lifecycle remains; safety and simplicity work is open | 14 | [04-knowledge-base.md](flows/04-knowledge-base.md) |
| Campaigns | Existing creation/monitoring flow remains | 13 | [05-campaigns.md](flows/05-campaigns.md) |
| Daily Inbox | Core workflow remains; contains the highest-risk state races | 16 | [06-daily-inbox.md](flows/06-daily-inbox.md) |
| CRM, Follow-ups & Settings | Core workflows remain; configuration, pagination, and recovery work is open | 19 | [07-crm-settings.md](flows/07-crm-settings.md) |
| **Total** | 27 legacy findings already implemented; 1 duplicate merged | **76** | |

The open total is not directly comparable to the original 86-point audit: implemented findings are retained as history, duplicated work was merged, and newly discovered issues were added.

## Product Decisions Already Made

These are settled unless the product direction explicitly changes:

- Fresh installs use known default credentials for a simple first login.
- `must_change_password` is mandatory for the bootstrap admin.
- The login helper fills the known credentials but never auto-submits.
- There is no blocking Setup Wizard.
- Administrators configure AI provider, channels, and Knowledge Base along the way through a persistent checklist.
- Password changes remain available later through Account Security.
- Direct/simple channels and advanced Meta-backed channels are presented separately.
- Connection success stays visible until the user acknowledges it.

## Current Top Priorities

### 1. INB-13 — Bind Every Send to Its Original Chat

Attachment upload currently allows the active chat to change before the final message request is built. A message intended for chat A can be posted to chat B.

**Roadmap:** [Daily Inbox → INB-13](flows/06-daily-inbox.md#inb-13--p0-attachment-upload-can-send-to-the-wrong-customer)

### 2. INB-08 — Reject Stale Cross-Chat Responses

Messages, drafts, customer profiles, timelines, and filtered lists can finish out of order and overwrite current state.

**Roadmap:** [Daily Inbox → INB-08](flows/06-daily-inbox.md#inb-08--p0-rapid-chat-switching-can-render-the-previous-chats-data)

### 3. CAM-09 — Bind Recipient Approval to Exact Input

Creation and replacement can display a valid preview for old text/file input while saving a different audience.

**Roadmap:** [Campaigns → CAM-09](flows/05-campaigns.md#cam-09--p0-reachability-preview-can-become-stale-before-saving-recipients)

### 4. INB-09 — Preserve Failed Sends and Attachments

Manual send failures are invisible, and attachments are cleared before upload/send success.

**Roadmap:** [Daily Inbox → INB-09](flows/06-daily-inbox.md#inb-09--p1-manual-send-failures-are-invisible-and-attachments-are-lost)

### 5. KB-08 — Confirm Publishing to Live Knowledge

**Publish all** immediately changes the live assistant’s knowledge without a summary or confirmation.

**Roadmap:** [Knowledge Base → KB-08](flows/04-knowledge-base.md#kb-08--p1-no-confirmation-guard-on-publish-all-and-inconsistent-discard-all-dialog)

**Next:** `KB-12` simulator data isolation, `SET-07` teammate offboarding, `TG-07` localized Telegram failures, `CRM-10` request ordering, and hard-list pagination across Inbox, CRM, campaigns, imports, and team management.

## Implementation Ownership Map

| Concern | Primary owner | Rendering owner |
|---|---|---|
| Auth and forced password state | auth store, router, backend auth gate | Login, Change Password, Account Security |
| Channel connection lifecycle | accounts/channel-setup stores and backend handlers | Add Account dialog, Accounts page, setup cards |
| Import/draft/live KB state | playground and import stores, KB APIs | Knowledge Base, Draft review, import status |
| Campaign lifecycle and recipient validity | campaigns store and campaign APIs | Campaign wizard/detail/preview table |
| Conversation identity, messages, drafts, sending | inbox store and messaging APIs | Chat list, thread, composer, assistant panel |
| Customer identity, profile, timeline, follow-ups | CRM store and CRM APIs | Customer panel/directory/follow-up views |
| Settings, credentials, users | settings store, router guard, auth/user APIs | Settings tabs and Account Security |

## Definition of Done for UX Roadmap Work

- Current flow and target behavior agree with each other.
- Async requests cannot mutate state for an obsolete selection.
- Destructive or irreversible actions use confirmation or a recoverable undo window.
- Loading, empty, failed, and successful states are distinct.
- Stateful tabs, filters, pagination, and selected records use the URL when deep linking is valuable.
- Forms have associated labels, meaningful names/autocomplete, inline recovery, visible focus, and accessible async status.
- Icon-only actions have localized accessible names.
- Large server-backed lists expose all records through pagination, incremental loading, or virtualization.
- User-facing backend failures use stable codes and localized recovery copy.
- Unit/DOM tests cover state transitions; P0/P1 cross-route workflows receive end-to-end coverage.
