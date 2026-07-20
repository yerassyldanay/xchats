# Live Knowledge Base and Customer Prompt

[`DECISIONS.md`](../DECISIONS.md) is authoritative. This document covers only
the customer-response path. Builder prompts and pending knowledge are described
in [`playground.md`](playground.md) and must never cross this trust boundary.

## Prompt composition

The prompt has a stable, cacheable prefix and a dynamic suffix:

```text
stable prefix (changes only after approved KB/config changes)
  assistant behavior from ai_assistants
  trusted prose/meaning from approved ai_topics/products/tariffs/contacts/policies
  generated exact-value placeholder catalog (not the stored exact values)
  generated semantic media send catalog

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
ranges, and approximations are valid (`25 000 ₸`, `от 5 000`, `1–3`); words that
need interpretation belong in trusted prose. A new exact-fact kind requires a
schema column and validator support, not a generic JSON key.

The response model is shown code-generated placeholders instead of exact stored
values, for example:

```text
Product sofa-loft has price token {{product.sofa-loft.price}}.
Delivery has cost token {{policy.main.delivery_cost}}.
Delivery duration has token {{policy.main.delivery_in_days}}.
Support phone has token {{contact.main.phone}}.
```

The placeholder namespace is code-owned and allowlisted from approved rows and
supported exact columns. Entity names are singular (`product`, `tariff`,
`contact`, `policy`); singleton contact/policy rows use the fixed ref `main`.
Natural refs cannot contain dots. The model copies tokens; it does not construct
or resolve them.

Generated `text` must use placeholders rather than model-authored exact business
values. Code validates exact-value literals and every placeholder against the
approved fact contract, then substitutes the stored value verbatim. It never
asks the model to reformat or translate the value. An invented exact value or an
unknown, malformed, stale, or empty placeholder rejects the entire response
suggestion for manual review; partial rendering is not allowed.

This is why exact columns are stored once rather than duplicated by reply
language: the model phrases the sentence in Russian, Kazakh, or another allowed
language, while the same approved value is inserted afterward.

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
topics.delivery.images
products.sofa-loft.featured_image
products.sofa-loft.certificates
products.sofa-loft.guarantee_documents
policies.main.documents
```

The catalog includes no empty groups and no invalid material references. It
contains no draft entries, UUIDs, filenames, paths, storage keys, public URLs,
or one-token-per-file entries. The model must copy an exact catalog token; a
syntactically plausible guess is still invalid.

A singular token such as `.featured_image` sends one file. A group token such as
`.images` sends every material ID in that approved array, in stored order. V1
does not let the model pick individual files from a group.

## Response contract and validation

The model returns channel-neutral control data:

```json
{
  "text": "Delivery costs {{policy.main.delivery_cost}}. I will send the infographic.",
  "send": ["topics.delivery.images"]
}
```

`send` is never displayed as prose. Before a suggestion can be approved or sent,
the backend performs the following fail-closed sequence:

1. Parse the exact response schema; reject unknown control shapes as configured.
2. Reject invented numeric/contact values and validate every placeholder against
   the current organization's approved exact-value allowlist.
3. Validate every send token against the exact catalog supplied to that request;
   deduplicate tokens while preserving order.
4. Re-read each token's approved row and declared column so stale/deleted tokens
   cannot pass merely because they were in an old model response.
5. For every internal material reference, recheck organization, source type,
   storage locator, visibility, and MIME compatibility.
6. Substitute exact values into `text` verbatim and resolve material rows through
   the storage adapter.
7. Store/transition `rp_suggestions`; at authorized send time, apply adapter
   constraints such as text length, album support, and MIME support.

Failure in any item blocks the whole suggestion/send and surfaces a manual-check
reason. Text is never sent without requested media, and a valid subset of an
invalid response is never silently delivered.

## End-to-end example

Approved storage:

```text
ai_topics.slug = delivery
ai_topics.body_md = We deliver throughout Almaty.
ai_topics.images = [internal kbd_materials UUID]
ai_policies.delivery_cost = 2 000 ₸
```

Model-visible live view:

```text
Delivery is available throughout Almaty.
Delivery cost: {{policy.main.delivery_cost}}
Available media: topics.delivery.images
Customer: What does delivery cost? Can I see the infographic?
```

Model result:

```json
{
  "text": "Delivery costs {{policy.main.delivery_cost}}. I’ll send the infographic.",
  "send": ["topics.delivery.images"]
}
```

Code validates both tokens, substitutes `2 000 ₸`, loads the complete approved
image group through `kbd_materials`, and hands the rendered text and files to the
selected `ChannelAdapter`. The customer model never learns the stored price or
the file location.
