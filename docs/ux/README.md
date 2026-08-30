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
> **Important:** These are one-time snapshots. They will go stale as the code
> changes. Re-run the audit or convert high-priority fixes into Playwright
> tests for living documentation.

---

## Flows Index

| # | Journey | File | Friction Points |
|---|---|---|---|
| 01 | **First-Time Onboarding** — Login, forced password change, setup wizard, empty inbox | [01-onboarding.md](flows/01-onboarding.md) | 9 |
| 02 | **Connect WhatsApp (QR)** — Channel picker, QR scan, timeout/retry, success | [02-connect-whatsapp-qr.md](flows/02-connect-whatsapp-qr.md) | 6 |
| 03 | **Connect Telegram Bot** — BotFather token, webhook errors, card management | [03-connect-telegram.md](flows/03-connect-telegram.md) | 7 |
| 03b | **Connect Instagram & Messenger (OAuth)** — Prerequisites, Meta consent, redirect handling | [03b-connect-instagram-messenger.md](flows/03b-connect-instagram-messenger.md) | 8 |
| 04 | **Knowledge Base Lifecycle** — Upload materials, extraction, draft review, simulator testing | [04-knowledge-base.md](flows/04-knowledge-base.md) | 10 |
| 05 | **Launching a Campaign** — Creation wizard, recipients, reachability, scheduling, dispatch | [05-campaigns.md](flows/05-campaigns.md) | 8 |
| 06 | **Daily Inbox Triage** — Chat list, AI draft approval, composer, customer panel | [06-daily-inbox.md](flows/06-daily-inbox.md) | 7 |
| 07 | **CRM, Follow-ups & Settings** — Customer directory, tasks, LLM keys, team management | [07-crm-settings.md](flows/07-crm-settings.md) | 12 |
| | | **Total** | **67** |

---

## Top Priority Friction Points (Cross-Journey)

These issues appear repeatedly across multiple flows and have the highest
impact on user experience:

### 1. No Persistent Onboarding Checklist
**Appears in:** Onboarding, all channel flows, Knowledge Base
The setup wizard is skippable, one-time, admin-only, and never returns.
A new user who skips it has no path back to understanding what steps remain.

### 2. Missing "What's Next?" Guidance After Every Major Action
**Appears in:** WhatsApp QR, Telegram, Instagram/Messenger, Knowledge Base, Campaigns
After completing a setup step (connecting a channel, publishing KB, launching a
campaign), the user is left on the page with no pointer to the logical next action.

### 3. Cryptic Error Messages
**Appears in:** WhatsApp QR (timeout/expired), Telegram (webhook failure),
Instagram/Messenger (hardcoded Russian errors), Knowledge Base (validation conflicts)
Error messages describe what failed but not why or what the user should do.

### 4. Auto-Close Modals Too Fast (900ms)
**Appears in:** WhatsApp QR, Telegram, Instagram/Messenger
Success confirmations disappear before users can read them.

### 5. Icon-Only Buttons Without Labels
**Appears in:** Telegram (retry/check/replace), Daily Inbox (resolve, emoji),
CRM (follow-up actions)
Critical actions are represented by small unlabeled icons that new users cannot discover.

### 6. Fixed Layout Without Responsive Collapse
**Appears in:** Daily Inbox
The 3-column layout (68px + 340px + flex + 340px) compresses the main content
area to under 550px on standard laptops.

---

## How to Act on This Audit

1. **Pick the highest-impact flow** — Onboarding (#01) or Daily Inbox (#06)
2. **Open the flow file** and review the Mermaid diagram + friction points
3. **Create an implementation plan** targeting specific friction points
4. **Point a coding agent** at the flow file for implementation context
