# Follis pilot rehearsal

A full-day rehearsal of xchats for a real client before going live:
**Follis** — магазин автоинструментов и оборудования для автосервиса,
ул. Жибек жолы, 114/2, Шымкент (opens in October 2026).

One command builds the organization on an empty instance, checks that every
piece landed, plays ten WhatsApp customers and six staff members through a
working day, takes screenshots, and writes a PASS/FAIL report.

> **Test data.** The catalog, prices, staff, phone numbers, photos and videos
> are made up for the rehearsal. They are not Follis's price list. Replace
> `data/kb.json` and `media/` with the real catalog before going live.

## Run it

```bash
scenarios/follis/run.sh
```

It needs Go, Node 22 and a Chromium for Playwright. Ports 8080 and 5173 must be free.
Every run starts from a fresh SQLite database in `scenarios/follis/out/`
(gitignored), so it is safe to repeat.

The AI replies come from the built-in mock by default (`-mock-externals`): no
API key, no network, the same answers every run. The mock reads the
catalog from the real rendered prompt and answers with real `{{placeholders}}`,
so the KB → prompt → substitution path is exercised end to end
(`backend/internal/llmmock/catalog.go`).

To run the same day against a real model:

```bash
LLM_PROVIDER=openrouter LLM_DEFAULT_MODEL=google/gemini-2.5-flash LLM_API_KEY=... scenarios/follis/run.sh --llm=real
```

In real mode the wording checks become warnings, because a model phrases its
answers its own way. Language, escalation and leftover-placeholder checks stay strict.

## What it does

| Phase | What happens | What is checked |
|---|---|---|
| setup | Rotates the default admin password, renames the org to Follis (Asia/Almaty), creates 2 admins + 4 members, CRM tags, and fills the KB: assistant persona, contacts, policies, 6 topics, 3 delivery zones, 22 products with 28 photos and 5 demo videos | — |
| draft | The store manager stages an opening price in the Черновик; the owner reviews (screenshot) and publishes | The live price stays put until the draft is published |
| verify-org / staff | — | Name and timezone; every staff member and role; every login; members get 403 on team, org and AI settings |
| verify-kb | — | Every field of every product, topic, zone, contact and policy; each photo and video sits on the right product and is served back with the right type |
| verify-prompt | — | The AI prompt (`GET /kb/prompt`) lists every available product with its price placeholder, photo and video refs, every topic and zone; unavailable products appear by name only; no raw price leaks into the prompt |
| customers | 10 customers write through the WhatsApp simulator (26 messages, Russian and Kazakh, including Kazakh typed in Russian letters); staff take chats, approve AI drafts, attach product media, and answer escalations themselves | Reply language, escalation where expected (wholesale, invoice, call-back, product not in the catalog), the real KB price in the reply, no unresolved placeholders |
| staff-crm | Staff name customers, tag them, write notes, schedule follow-ups, resolve finished chats | Every action succeeds; the inbox, assignments, customer list and follow-up board add up |
| screenshots | Login, dashboard, team, KB, draft review, simulator, inbox (sent photo, Kazakh draft, escalation), customer card, customers, follow-ups, member view | — |

Output: `out/REPORT.md` (checks, findings, full transcripts), `out/results.json`,
`out/screenshots/*.png`, `out/logs/`.

## Current result

On the current code the mock run ends with 246 checks passing, 2 warnings and
9 failures, so `run.sh` exits non-zero. The failures and warnings are xchats
findings, not scenario bugs:

- **Short facts reject valid answers.** Shymkent delivery takes "1" day, so
  the "1" inside product names like «Динамометрический ключ 1/2"» trips the
  `exact_value_literal` leak check (`backend/aiprompt/contract.go`). The
  customer gets the holding reply instead of a price, in Russian even when
  they wrote in Kazakh (`response.HoldingText`). This causes all 9 failures.
- **AI-chosen photos and videos never reach the draft.** `media_files_to_send`
  is validated and then dropped (`backend/response/engine.go`), so staff
  attach media by hand.
- **Contacts address and policy prose are not in the shop-kb@v7 prompt.**
  Only topics carry them to the model.

## Files

- `data/org.json` — organization, staff (roles, test passwords), CRM tags
- `data/kb.json` — the knowledge base, plus the price change staged in the Draft
- `data/customers.json` — customers, their messages, the expected reply, and what staff do
- `media/generate.py` — draws the product cards and renders the demo MP4s (Pillow + ffmpeg); its output is committed in `media/out/`
- `simulate.mjs` — the rehearsal itself (API + Playwright)
- `run.sh` — build, start, run, stop
