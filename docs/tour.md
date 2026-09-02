<div align="center">

# The xchats visual tour

Every screenshot below comes straight from a real, self-hosted instance running
the built-in demo dataset — `make up && make seed-demo`. Nothing here is a
mockup.

[← Back to the README](../README.md)

</div>

---

## 1. Team Inbox & Omnichannel Sync

![xchats team inbox: a conversation open with the customer's message on the left and the assistant's grounded, ready-to-approve draft reply on the right](images/inbox.png)

Every conversation from WhatsApp, Telegram, Instagram, Messenger and the
WhatsApp Cloud API lands in **one shared inbox**, with the assistant's
suggested reply waiting beside it for a human to approve, edit, or discard.
Nothing sends itself — a teammate always makes the final call before a
customer sees a word of it.

## 2. Grounded Knowledge Base & Strict Token Replacement

![The Knowledge Base products catalog, showing real product photos, names and exact prices](images/knowledge-base.png)

The Knowledge Base holds the exact facts the assistant is allowed to answer
from — products, prices, delivery zones, tariffs and policies, each with real
photos and current values. The model never writes a number itself; it emits a
`{{token}}` placeholder that the backend substitutes with the stored value, so
a wrong price is structurally impossible, not just unlikely — see the
[grounding diagram](images/grounding.svg) for the full five-stage pipeline.

## 3. Staged Knowledge Ingestion & Visual Diff Review

![The Draft page showing a pending product price change as a before/after diff, with Publish all and Discard all actions](images/draft-staging.png)

Every edit — typed by hand, imported from a URL or document, or written by an
LLM over MCP (§7) — lands in a staging area first, showing exactly what will
be **added, changed or removed** with a full before/after diff. Nothing
reaches the live knowledge base, and nothing the assistant can quote, until a
human reviews and publishes it.

## 4. Mini CRM & Time-Grouped Daily Follow-up Board

![The Customers grid, showing profile cards with status, tags and channel identities](images/customers.png)
![The Follow-ups board, grouped into Overdue, Today, Tomorrow and Later, with a completed-tasks history tab](images/followups.png)

Every contact gets a lightweight CRM profile — status, tags, notes and every
channel identity in one place — and every promised next step becomes a
follow-up task, automatically grouped into **Overdue, Today, Tomorrow and
Later** (with a Completed tab for history). It's the minimum structure a
small sales or support team actually needs, without a separate CRM
subscription.

## 5. Campaigns: Bulk Outbound with Live Delivery Tracking

![The Campaigns list showing one running broadcast with a live delivery progress bar and one draft campaign](images/campaigns.png)

Paste or upload a recipient list, write one templated message, and xchats
sends it out rate-limited and confined to a send window — no accidental
floods, no banned numbers. Each campaign tracks **delivery status per
recipient live**, and any reply a customer sends back lands right in the
shared inbox like any other conversation.

## 6. Built-in Channel Simulator & AI Evals

![The Simulator's empty state, with one-click example questions to test the assistant against the live knowledge base or a staged draft](images/simulator.png)

The Simulator lets you test exactly how the assistant would answer a real
customer question — against the **live** knowledge base or a **staged
draft** — without touching a real WhatsApp or Telegram account. Pair it with
the open evaluation harness ([`evals/`](../evals/)) to grade response quality
automatically whenever the prompt, model or knowledge base changes.

## 7. Configuring xchats via ChatGPT / Claude Desktop using MCP

Point ChatGPT or Claude Desktop at your own xchats instance as an MCP
connector (**Draft → ChatGPT / Claude**), authorize once over OAuth 2.1 with
PKCE, and the assistant can read documents you hand it and store structured
facts through 13 `kb_*` tools — products, tariffs, delivery zones, policies
and more. Every write lands in the **same staging area** shown in §3, so an
LLM configuring your knowledge base over chat is exactly as safe as typing it
in yourself.

---

<div align="center">

[Back to the README](../README.md) · `make up && make seed-demo` to see this yourself

</div>
