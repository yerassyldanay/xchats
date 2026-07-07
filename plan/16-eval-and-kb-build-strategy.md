# Eval and KB Build Strategy

This document captures the recommended next step for AI evaluation. The goal is to move from prompt-only tests toward tests that prove the real product loop works:

```text
raw customer/business material
-> extracted material
-> draft knowledge base rows
-> approval
-> live snapshot
-> prompt build
-> LLM response
-> post-processing
-> customer-ready draft
```

## Current State

The eval harness is useful for prompt and schema experiments:

- It renders prompt text from scenario data instead of hand-maintaining full prompts.
- It validates prompt/catalog consistency before running models.
- It checks output contracts, fact-token injection, unknown media refs, escalation, language, invented digits, and unit issues.
- It supports authored conversation history.
- It reports model behavior separately from contract safety.
- It includes scale scenarios for larger product catalogs.

This is a good playground for comparing prompt wording, schema shapes, model behavior, and token costs.

The limitation is that each test still evaluates one generated response. Even when `history` exists, it is authored fixture text, not a full production conversation loop. Also, prompt scenarios still start from structured `data.yaml`, not from the messy inputs that the real playground accepts, such as URLs, images, audio, video, PDFs, and operator notes.

## Recommendation

Keep two eval layers:

1. Prompt-design evals.
2. End-to-end KB build evals.

They answer different questions and should stay separate. The playground/KB builder has almost nothing to do with the chat response path except that both use the same database schema and the chat path later reads the approved KB snapshot.

## Layer 1: Prompt-Design Evals

Use the existing `evals/harness` flow for fast model and prompt comparisons.

This layer should answer:

- Does the model choose the correct fact token?
- Does the model avoid literal prices, phone numbers, counts, and other exact values?
- Does the model attach only catalog media refs/groups?
- Does the model escalate when the KB is missing an answer?
- Does behavior degrade as product count grows?
- Does the model follow language rules?
- How many tokens does a prompt shape cost?

### Improve Conversation Fixtures

The current `message` plus `history` shape works, but a cleaner format would model a conversation window explicitly:

```yaml
tests:
  - id: "delivery follow-up after price"
    turns:
      - role: customer
        text: "How much is the coffee machine?"
      - role: assistant
        text: "The DeLonghi coffee machine costs {{product.coffee-machine.price}}."
      - role: customer
        text: "How much is delivery?"
    current_turn: 2
    requires:
      - ["policy.main.delivery_cost"]
    language: en
```

The harness should convert this into the same shape production uses:

```go
brain.BuildUser(profile, historyWindow, currentMessage)
```

This makes eval fixtures closer to `assistant.RealDrafter`, where the current inbound message is separated from the prior window.

### Add Rolling Conversation Mode Later

For some cases, test a whole conversation where the model's first answer becomes history for the next turn:

1. Customer asks price.
2. Model answers and post-processing injects the price.
3. Injected answer is appended as assistant history.
4. Customer asks a follow-up.
5. Model must answer using the same KB and current context.

This is more expensive and less deterministic, so it should be a separate mode from authored-history tests.

## Layer 2: End-to-End KB Build Evals

Add a new eval family for the playground/KB builder. This should start from raw materials, not from prebuilt `data.yaml`.

This layer should answer:

- Can the app ingest a URL, image, audio file, video file, PDF, or text note?
- Can it extract useful text and media descriptions?
- Can it request human help when extraction is uncertain or incomplete?
- Can it classify the material as product, tariff, policy, contact, general topic, or media asset?
- Can it create typed draft rows in the correct tables?
- Are exact facts stored in typed columns rather than prose topic bodies?
- Are media files stored as assets with usable refs or URLs?
- After approval, can `kbstore.LoadLive` build a valid `domain.Snapshot`?
- Can `brain.BuildSystem` build a correct prompt from that live snapshot?
- Can the assistant answer from the generated KB using fact tokens and media refs?

### Concrete Fixture Set

The directory `evals/assets` is now the starting fixture blob store. Builder evals should treat these files as already uploaded materials, not fictional media names:

```text
evals/assets/IMG_4155.MOV
evals/assets/drill_edh_01.jpeg
evals/assets/drill_edh_02.jpeg
evals/assets/drill_edh_03.jpeg
evals/assets/fixed_tariff_006.png
evals/assets/how-to-add-cachier.jpg
evals/assets/xpayment-how-it-works.png
evals/assets/xpayment_caledar_ads_kz.mp3
evals/assets/xpayment_caledar_ads_kz_newaudio.mp4
```

Use this URL fixture:

```text
https://xpayment.kz
```

For deterministic CI, record the fetched page text into a fixture and run the URL extractor against a local fixture server. Keep a separate live integration mode that can fetch `https://xpayment.kz` to catch real-world parser drift.

### Expected Xpayment Knowledge

The xpayment builder scenario should use:

- URL: `https://xpayment.kz`
- Images:
  - `fixed_tariff_006.png`
  - `xpayment-how-it-works.png`
  - `how-to-add-cachier.jpg`
- Audio/video:
  - `xpayment_caledar_ads_kz.mp3`
  - `xpayment_caledar_ads_kz_newaudio.mp4`

Expected draft KB concepts:

- Service category: Kaspi Pay API automation / payment acceptance.
- Value proposition: connect Kaspi Pay payment flows quickly through xpayment.
- Payment methods: QR, deeplink/link, invoice.
- Integration methods: API key, webhooks, merchant website, CRM, Telegram bot, mobile app.
- Security explanation: use a restricted cashier role; xpayment does not need the main Kaspi Pay password.
- Money flow: funds go directly to the merchant's Kaspi Pay account; xpayment should not be described as holding funds.
- Suitable users: online shops, Telegram bots/courses, CRM/sales teams, courier/services, platforms/marketplaces, MVPs.
- Setup flow: register in xpayment, create a restricted cashier in Kaspi Pay, connect device via phone/SMS, receive API key, create first payment.
- Tariffs:
  - Trial: free for 3 days.
  - Start: 10000 KZT per month, up to 250 payments per month.
  - Growth: 25000 KZT per month, up to 2000 payments per month.
  - Scale: 60000 KZT per month, unlimited payments.
- Shared tariff features: up to 5 virtual cashiers, unlimited webhooks, QR/deeplink/invoice, full dashboard access.
- FAQ facts: unofficial Kaspi integration layer, restricted cashier role, refund support, fiscal receipts may require a fiscalization service.

Expected draft assets:

- `fixed_tariff_006.png`: tariff/pricing image.
- `xpayment-how-it-works.png`: payment flow / integration diagram.
- `how-to-add-cachier.jpg`: cashier setup instruction image.
- `xpayment_caledar_ads_kz.mp3`: audio material attached to xpayment marketing or explanation topic after transcription.
- `xpayment_caledar_ads_kz_newaudio.mp4`: video material attached to xpayment marketing or explanation topic after video/audio extraction.

### Expected Drill Knowledge

The drill builder scenario should use:

- Images:
  - `drill_edh_01.jpeg`
  - `drill_edh_02.jpeg`
  - `drill_edh_03.jpeg`
- Video:
  - `IMG_4155.MOV`

Expected draft KB concepts:

- Entity type: physical product.
- Product family: magnetic drill / drilling machine.
- Brand/name: Follis, if extracted confidently.
- Model: ZT-40H, if extracted confidently from the plate image.
- Specs from the plate image, if extracted confidently:
  - Input: AC 230V 50Hz.
  - Max power: 1450W.
  - Speed: 820 r/min.
  - Magnetic attraction: 12500N.
  - Twist drilling: diameter 3-16 mm.
  - Max hollow drilling: diameter 40 mm.
  - Depth: 50 mm.
  - Spindle taper: Weldon 19.05.
- Assets: all drill photos and video should become product media assets.

If any plate value is not confidently readable, the builder should create a confirmation request instead of inventing the spec.

## Builder Eval Shape

Use a fixture schema that models uploaded materials and expected draft rows:

```yaml
id: "builder-xpayment-url-and-media"
kind: builder

materials:
  - type: url
    ref: "https://xpayment.kz"
    fixture_text: "fixtures/xpayment.kz.txt"
  - type: image
    file: "evals/assets/fixed_tariff_006.png"
  - type: image
    file: "evals/assets/xpayment-how-it-works.png"
  - type: image
    file: "evals/assets/how-to-add-cachier.jpg"
  - type: audio
    file: "evals/assets/xpayment_caledar_ads_kz.mp3"
  - type: video
    file: "evals/assets/xpayment_caledar_ads_kz_newaudio.mp4"

expected_build:
  topics:
    - slug: "xpayment-overview"
      contains:
        - "Kaspi Pay"
        - "QR"
        - "webhooks"
      must_not_contain:
        - "{{"
  tariffs:
    - ref: "trial"
      price: "0 KZT"
      period: "3 days"
    - ref: "start"
      price: "10000 KZT/month"
      payment_limit: 250
    - ref: "growth"
      price: "25000 KZT/month"
      payment_limit: 2000
    - ref: "scale"
      price: "60000 KZT/month"
      payment_limit: null
  assets:
    - file: "fixed_tariff_006.png"
      kind: "image"
      role: "pricing"
    - file: "xpayment-how-it-works.png"
      kind: "image"
      role: "flow"
    - file: "how-to-add-cachier.jpg"
      kind: "image"
      role: "instruction"
  requests:
    - type: "confirm_fact"
      optional: true

after_approve_questions:
  - message: "How much is the Growth plan?"
    requires:
      - ["tariff.growth.price"]
    language: en
  - message: "Send me the tariff image."
    requires_media:
      - "fixed_tariff_006.png"
```

Add a second builder fixture for the drill product:

```yaml
id: "builder-drill-product-media"
kind: builder

materials:
  - type: image
    file: "evals/assets/drill_edh_01.jpeg"
  - type: image
    file: "evals/assets/drill_edh_02.jpeg"
  - type: image
    file: "evals/assets/drill_edh_03.jpeg"
  - type: video
    file: "evals/assets/IMG_4155.MOV"

expected_build:
  products:
    - ref: "follis-zt-40h"
      name_contains:
        - "Follis"
        - "ZT-40H"
      specs:
        max_power: "1450W"
        speed: "820 r/min"
        magnetic_attraction: "12500N"
  assets:
    - file: "drill_edh_01.jpeg"
      kind: "image"
    - file: "drill_edh_02.jpeg"
      kind: "image"
    - file: "drill_edh_03.jpeg"
      kind: "image"
    - file: "IMG_4155.MOV"
      kind: "video"
```

## Extraction Architecture

Separate the builder pipeline from chat response generation:

```go
type MaterialExtractor interface {
    Extract(ctx context.Context, material Material) (ExtractedMaterial, error)
}

type KBSynthesizer interface {
    BuildDraft(ctx context.Context, extracted []ExtractedMaterial) (DraftPlan, error)
}

type DraftWriter interface {
    ApplyDraft(ctx context.Context, plan DraftPlan) (DraftResult, error)
}
```

The extraction stage should normalize each source:

- URL: fetched page text, title, canonical URL, links, image references.
- Image: OCR text, visual description, detected product/tariff/instruction role.
- Audio: transcription, language, confidence, timestamps when available.
- Video: sampled frame descriptions plus audio transcription.
- PDF/document later: text extraction plus page-level image descriptions.

Current backend code already has useful pieces for URL and image extraction. Audio and video should not remain `needs_human` if these materials are a required playground input. For video, extract both sampled frames and audio; otherwise the builder will miss visual facts and spoken facts.

## Production Prompt Build Requirement

Production should not rely on predefined full prompts with KB already injected.

The source of truth should be database rows:

- `ai_topics`
- `ai_assets`
- `ai_tariffs`
- `ai_products`
- `ai_contacts`
- `ai_policies`

These rows load into:

```text
kbstore.LoadLive
-> domain.Snapshot
-> brain.BuildSystem
-> brain.BuildUser
-> LLM
-> brain.PostProcess
```

Eval coverage should include this path directly, not only the standalone `evals/harness` renderer.

## Suggested Implementation Plan

1. Add an asset manifest for `evals/assets` with file path, material type, expected role, and deterministic blob ID.
2. Add a `builder` eval mode separate from chat prompt scenarios.
3. Implement deterministic extractor fakes for CI using recorded URL text, image captions, audio transcripts, and video frame descriptions.
4. Add optional live extractor runs that call the real URL, vision, transcription, and video-frame parsers.
5. Add an LLM-backed `KBSynthesizer` that returns strict JSON draft operations.
6. Keep `RuleSynthesizer` only as a low-risk fallback and unit-test baseline; it is not enough for real media/product/tariff extraction.
7. Apply draft operations into the same tables used by production.
8. Approve the draft into live KB rows.
9. Load the live snapshot and run normal chat eval questions against it.
10. Report build pass/fail separately from chat response pass/fail.

## Acceptance Criteria

The eval system should be considered strong enough when it can prove all of this:

- A prompt-only scenario can test fresh messages, authored history, and scale.
- A rolling conversation scenario can test multiple model turns.
- A raw URL can become a live KB topic or typed fact.
- The xpayment URL plus xpayment media can produce topics, tariffs, FAQ facts, and real assets.
- Drill images and video can produce a product draft with real media assets and confirmed/speculative specs separated.
- Image, audio, and video materials are parsed, not just accepted as opaque files.
- Ambiguous or low-confidence facts create human confirmation requests.
- Approved KB rows can be loaded from the database into `domain.Snapshot`.
- Exact values are injected from typed facts, not copied from prose.
- Unknown or malformed fact tokens fail closed.
- Returned media refs resolve to real stored assets in pipeline tests.
- Reports show build pass, chat behavior pass, contract pass, estimated cost basis, and token growth.

## Bottom Line

The current eval harness is good for prompt-design work. The next step is a separate builder eval path that starts from the real assets in `evals/assets` and the real xpayment URL, extracts material, builds draft KB rows, approves them, then verifies that the chat assistant can answer from the generated KB. This tests the actual product promise: users can drop messy business material into the playground, the app builds a safe knowledge base, and the assistant later answers correctly from that knowledge base during an ongoing conversation.
