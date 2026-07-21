# Live Knowledge Base and Customer Prompt

[`DECISIONS.md`](../DECISIONS.md) is authoritative. This document covers only
the customer-response path. Builder prompts and pending knowledge are described
in [`playground.md`](playground.md) and must never cross this trust boundary.

## Prompt composition

The prompt has a stable, cacheable prefix and a dynamic suffix:

```text
stable prefix (changes only after approved KB/assistant changes)
  assistant behavior from ai_assistants
  trusted prose/meaning from approved ai_topics/products/tariffs/contacts/policies
  generated exact-value placeholder catalog (not the stored exact values)
  generated semantic media catalog

dynamic suffix (per suggestion)
  channel and its response constraints
  recent normalized conversation context
  current customer message
```

Only approved rows for the current organization participate. The prompt never
contains `kbd_draft`, pending requests, raw/extracted materials, builder notes,
material UUIDs, filenames, paths, storage keys, or public object URLs.

The default is the full approved KB, not similarity search. Provider prompt
caching lowers price and latency but cannot affect answers. Approval invalidates
and rebuilds the organization's prefix. If a KB later exceeds the as-yet
undecided size threshold, backend code will deterministically shortlist known
categories/entities; free-form vector retrieval is not the fallback.

## Exact business values

Exact values are language-neutral text in purpose-named live columns. Symbols,
ranges, and approximations are valid (`25 000 ₸`, `≈ 5 000 ₸`, `1–3`); words that
need interpretation belong in trusted prose. `in_stock` is the deliberate
boolean exception: code gives the model a semantic in-stock/out-of-stock state
for reasoning and the token `{{product.<ref>.in_stock}}`; code substitutes
reviewed Russian wording for `true`/`false`, and the model never writes the raw
boolean literal to the customer. It must never push an out-of-stock product. A
missing, empty, or whitespace-only exact value generates no placeholder, so a
question that requires it must escalate instead of being answered. A new
exact-fact kind requires a schema column and validator support, not a generic
JSON key.

The response model is shown code-generated placeholders instead of exact stored
values, for example:

```text
У товара sofa-loft токен цены {{product.sofa-loft.price}}.
Токен стоимости доставки {{policy.main.delivery_cost}}.
Токен срока доставки {{policy.main.delivery_in_days}}.
Токен телефона поддержки {{contact.main.phone}}.
```

The placeholder namespace is code-owned and allowlisted from approved rows and
supported exact columns. Entity names are singular (`product`, `tariff`,
`contact`, `policy`); singleton contact/policy rows use the fixed ref `main`.
Natural refs cannot contain dots. The model copies tokens; it does not construct
or resolve them.

Generated `reply_text` must use placeholders rather than model-authored exact
business values. Code validates exact-value literals and every placeholder
against the approved fact contract, then substitutes the stored value verbatim.
It never asks the model to reformat or translate the value. An invented exact
value or an unknown, malformed, stale, or empty placeholder rejects the entire
response suggestion for manual review; partial rendering is not allowed.

V1 has no per-language live rows: trusted KB prose and `reply_text` are Russian,
while exact values are stored once and inserted afterward. Future language
expansion must not duplicate business records.

## Approved media catalog

The prompt builder derives media tokens only from non-empty, currently valid
media columns on approved live rows. The grammar is exactly:

```text
<table>.<natural_ref>.<column>
```

- `table` is one of the model-facing domain sections: `topics`, `products`,
  `tariffs`, `contacts`, or `policies`.
- `natural_ref` is an approved `slug`/`ref`, or fixed `main` for a singleton.
- `column` is one declared semantic media column for that table.
- All three segments are required, none may contain a dot, and no fourth segment
  may select an individual file.

Examples:

```text
topics.delivery.illustration_images
products.sofa-loft.featured_image
products.sofa-loft.certificate_documents
products.sofa-loft.guarantee_documents
policies.main.commerce_policy_documents
```

The catalog includes no empty groups and no invalid material references. It
contains no draft entries, UUIDs, filenames, paths, storage keys, public URLs,
or one-token-per-file entries. The model must copy an exact catalog token; a
syntactically plausible guess is still invalid.

A `NULL` singular column and an empty plural array generate no catalog token. A
record is advertised as having no media only when ALL of its semantic media
columns are empty; if one column is populated and another is empty, only the
populated column is catalogued and the record is not media-less. A non-empty
column containing an invalid or stale material reference fails prompt rendering
outright — it is never silently treated as empty.

A singular token such as `.featured_image` sends one file. A group token such as
`.gallery_images` sends every material ID in that approved array, in stored
order. V1 does not let the model pick individual files from a group.

## Response contract and validation

The model returns channel-neutral control data in the canonical customer-response
JSON contract (DECISIONS.md §"Customer-response JSON contract" holds the
authoritative per-property JSON Schema descriptions; unknown properties are
rejected; the legacy names `asset_refs`, `attach_groups`, and `send` must not
appear anywhere):

```json
{
  "reply_text": "Стоимость доставки — {{policy.main.delivery_cost}}. Отправлю инфографику.",
  "reply_language": "ru",
  "media_files_to_send": ["topics.delivery.illustration_images"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.91
}
```

`media_files_to_send` is never displayed as prose and contains only semantic
media-catalog tokens — never UUIDs, filenames, paths, or storage keys. Before a
suggestion can be approved or sent, the backend performs the following
fail-closed sequence:

| JSON property | Type | Contract description |
|---|---|---|
| `reply_text` | `string` | Russian customer-facing reply; exact values use approved placeholders. |
| `reply_language` | `string` | Must be `ru` in v1. |
| `media_files_to_send` | `string[]` | Ordered tokens copied exactly from the generated media catalog; empty means no media. |
| `escalate` | `boolean` | True when approved live knowledge is insufficient and human review is needed. |
| `escalation_reason` | `string` | Internal Russian reason; empty when not escalating and never customer-visible. |
| `confidence` | `number` | Informational value from `0` to `1`; never a safety gate. |

1. Parse the exact response schema; reject unknown properties and control shapes.
2. Reject invented numeric/contact values and validate every placeholder against
   the current organization's approved exact-value allowlist.
3. Validate every `media_files_to_send` token against the exact catalog supplied
   to that request; deduplicate tokens while preserving order.
4. Re-read each token's approved row and declared column so stale/deleted tokens
   cannot pass merely because they were in an old model response.
5. For every internal material reference, recheck organization, source type,
   storage locator, `customer_visibility`, and MIME compatibility.
6. Substitute exact values into `reply_text` verbatim and resolve material rows
   through the storage adapter.
7. Store/transition `rp_suggestions`; at authorized send time, apply adapter
   constraints such as text length, album support, and MIME support.

Failure in any item blocks the whole suggestion and surfaces a manual-check
reason. `reply_text` is never sent without requested media, and a valid subset
of an invalid response is never silently delivered.

## Evaluation contract

Response evaluations create Russian, schema-shaped approved `ai_*` records and
metadata-only `kbd_materials` records, then invoke the same prompt builder used
by the product. They never start from a committed, already-expanded KB prompt.
Media scenarios use deterministic material UUIDs and a fake storage adapter; no
image, PDF, audio, or video bytes are required. Tests cover populated, partially
populated, empty, invalid, cross-organization, invisible, and stale media
references, and assert that internal material data never leaks into the prompt.

## End-to-end example

Approved storage:

```text
ai_topics.slug = delivery
ai_topics.body_md = Доставляем по всему Алматы.
ai_topics.illustration_images = [internal kbd_materials UUID]
ai_policies.delivery_cost = 2 000 ₸
```

Model-visible live view:

```text
Доставляем по всему Алматы.
Стоимость доставки: {{policy.main.delivery_cost}}
Доступные медиа: topics.delivery.illustration_images
Клиент: Сколько стоит доставка? Можно инфографику?
```

Model result:

```json
{
  "reply_text": "Доставка стоит {{policy.main.delivery_cost}}. Отправлю инфографику.",
  "reply_language": "ru",
  "media_files_to_send": ["topics.delivery.illustration_images"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.93
}
```

Code validates both tokens, substitutes `2 000 ₸`, loads the complete approved
image group through `kbd_materials`, and hands the rendered text and files to the
selected `ChannelAdapter`. The customer model never learns the stored price or
the file location.
