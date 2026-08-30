# xchats — UX Status Quo Audit

> **What this is:** A complete visual audit of every user-facing journey in xchats,
> traced directly from the codebase. Each flow documents what the user sees,
> clicks, and experiences — including every friction point found.
>
> **How to use it:**
> 1. **For visual review:** Open any flow file — the Mermaid diagrams render in
>    GitHub, VS Code, and most Markdown viewers.
> 2. **For a coding agent:** Point the agent at the specific flow file and say
>    "fix friction point #N" — each one has the exact component, behavior, and
>    suggested change.
>
> **Verification baseline:** Re-checked against the Vue components, router,
> Pinia stores, and relevant backend handlers on 2026-08-30. These are still
> point-in-time snapshots; re-run the audit or convert high-priority fixes into
> Playwright tests for living documentation.

---

## Flows Index

| # | Journey | File | Friction Points |
|---|---|---|---|
| 01 | **First-Time Onboarding** — Login, conditional password change, setup wizard, empty inbox | [01-onboarding.md](flows/01-onboarding.md) | 10 |
| 02 | **Connect WhatsApp (QR)** — Channel picker, QR scan, timeout/retry, success | [02-connect-whatsapp-qr.md](flows/02-connect-whatsapp-qr.md) | 7 |
| 03 | **Connect Telegram Bot** — BotFather token, polling/webhook delivery, card management | [03-connect-telegram.md](flows/03-connect-telegram.md) | 8 |
| 03b | **Connect Instagram & Messenger (OAuth)** — Prerequisites, Meta consent, redirect handling | [03b-connect-instagram-messenger.md](flows/03b-connect-instagram-messenger.md) | 10 |
| 04 | **Knowledge Base Lifecycle** — Upload materials, extraction, draft review, simulator testing | [04-knowledge-base.md](flows/04-knowledge-base.md) | 12 |
| 05 | **Launching a Campaign** — Creation wizard, recipients, reachability, scheduling, dispatch | [05-campaigns.md](flows/05-campaigns.md) | 11 |
| 06 | **Daily Inbox Triage** — Chat list, AI draft approval, composer, customer panel | [06-daily-inbox.md](flows/06-daily-inbox.md) | 12 |
| 07 | **CRM, Follow-ups & Settings** — Customer directory, tasks, LLM keys, team management | [07-crm-settings.md](flows/07-crm-settings.md) | 15 |
| | | **Total** | **85** |

---

## Top Priority Friction Points (Cross-Journey)

Ranked by severity, likelihood, and breadth of user impact:

### 1. Public Default Admin Password Is Not Forced to Change

**Appears in:** Onboarding

The current migration restores a repository-public password and explicitly clears the
forced-change flag. A reachable fresh deployment can be taken over before configuration.

### 2. Rapid Chat Switching Can Mix Conversation Data

**Appears in:** Daily Inbox, CRM sidebar

Uncancelled async requests can apply messages, drafts, or a customer profile from the
previous selection to the currently active chat. This is a trust and data-integrity failure
inside the product's primary workflow.

### 3. Campaign Reachability Preview Can Become Stale

**Appears in:** Campaigns

Changing pasted text or the selected file does not invalidate an earlier preview. The final
save reparses the new input, so an operator can launch to recipients they did not review.

### 4. Message Send Failures Are Invisible and Clear Attachments

**Appears in:** Daily Inbox

The main composer clears files before upload/send succeeds, while the thread exposes no
send error or retry state. Operators can believe a reply was sent and lose their staged files.

### 5. Simulator Traffic Pollutes the Live Inbox and CRM

**Appears in:** Knowledge Base, Simulator, Daily Inbox, CRM

The testing surface persists and broadcasts synthetic customers, chats, messages, and AI
drafts through the operational data path despite copy that frames it as simulation.

**Next highest:** No persistent onboarding checklist; hard list caps without pagination;
unlocalized backend errors; misleading presence indicators; and missing responsive collapse.

---

## How to Act on This Audit

1. **Pick the highest-impact flow** — Onboarding (#01) or Daily Inbox (#06)
2. **Open the flow file** and review the Mermaid diagram + friction points
3. **Create an implementation plan** targeting specific friction points
4. **Point a coding agent** at the flow file for implementation context
