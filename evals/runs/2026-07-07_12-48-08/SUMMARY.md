# Run 2026-07-07_12-48-08

Generated 2026-07-07 12:53. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-07) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## shop-scale-10

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 3/5 (60%) | 5/5 (100%) | unknown pricing | 2793ms | 3500 | n/a |
| openrouter:google/gemini-2.5-flash | 4/5 (80%) | 5/5 (100%) | unknown pricing | 2537ms | 2762 | n/a |
| openrouter:openai/gpt-4o-mini | 4/5 (80%) | 5/5 (100%) | $0.0021 est. (5 measured) | 2853ms | 2590 | 97% |

## shop-scale-20

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 4/6 (67%) | 6/6 (100%) | unknown pricing | 2946ms | 5189 | n/a |
| openrouter:google/gemini-2.5-flash | 5/6 (83%) | 6/6 (100%) | unknown pricing | 1819ms | 4130 | n/a |
| openrouter:openai/gpt-4o-mini | 5/6 (83%) | 6/6 (100%) | $0.0037 est. (6 measured) | 3134ms | 3869 | 98% |

## shop-scale-30

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 4/7 (57%) | 7/7 (100%) | unknown pricing | 2639ms | 6852 | n/a |
| openrouter:google/gemini-2.5-flash | 6/7 (86%) | 7/7 (100%) | unknown pricing | 5762ms | 5490 | n/a |
| openrouter:openai/gpt-4o-mini | 5/7 (71%) | 7/7 (100%) | $0.0057 est. (7 measured) | 2977ms | 5135 | 98% |

## Scale comparison (shop-scale-N)

Model-behavior pass % and avg total tokens per answer at each catalog size — the
direct answer to "does answer quality hold up as the product list grows" and what
that growth costs in tokens (avg tokens here is the raw API count, always
available regardless of whether this model's cost is priced — unlike the est.
cost column above).

| model | shop-scale-10 (behavior / avg tokens) | shop-scale-20 (behavior / avg tokens) | shop-scale-30 (behavior / avg tokens) |
|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 60% / 3500 | 67% / 5189 | 57% / 6852 |
| openrouter:google/gemini-2.5-flash | 80% / 2762 | 83% / 4130 | 86% / 5490 |
| openrouter:openai/gpt-4o-mini | 80% / 2590 | 83% / 3869 | 71% / 5135 |

## Failures (verbatim)

**2. product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! К сожалению, посудомоечных машин в нашем каталоге нет. У нас есть техника для дома и кухни: кофемашины, блендеры, чайники, тостеры, мультиварки, микроволновые печи, пылесосы, утюги и наборы посуды. Что из этого вас интересует?",
  "reply_language": "ru",…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечных машин Bosch нет в нашем каталоге. В наличии есть техника для дома и кухни: кофемашины, мелкая бытовая техника, посуда. Могу предложить что-то из этого.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason":…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:openai/gpt-4o-mini
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук. Если вас интересует покупка, напишите адрес доставки, и я отправлю счёт в WhatsApp!","reply_language":"kk","attach_groups":[],"escalate":false,"escalati…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:anthropic/claude-haiku-4.5
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук на складе в Алматы. Интересует? Напишите, и мы оформим заказ — доставим за {{policy.main.delivery_in_days}} дня или заберёте со склада.",
  "…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечных машин Bosch сейчас нет в наличии. В нашем каталоге представлена техника для дома и кухни: кофемашины, мелкая бытовая техника, посуда. Могу предложить что-то из этого?",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "esca…`

**2. product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! К сожалению, посудомоечных машин в нашем каталоге нет. У нас есть техника для дома и кухни: кофемашины, блендеры, чайники, мультиварки, микроволновые печи и многое другое. Могу помочь с чем-то ещё? 😊",
  "reply_language": "ru",
  "attach_groups": [],
  "…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:openai/gpt-4o-mini
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук. Доставка по городу занимает {{policy.main.delivery_in_days}} дня. Если вас интересует, как заказать, дайте знать!","reply_language":"kk","attach_groups"…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:anthropic/claude-haiku-4.5
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук на складе в Алматы. Хотите заказать или нужна дополнительная информация?",
  "reply_language": "ru",
  "attach_groups": ["product.microwave-s…`

**2. product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! К сожалению, посудомоечных машин в нашем каталоге нет. У нас есть техника для дома и кухни: кофемашины, блендеры, чайники, мультиварки, микроволновые печи и многое другое. Могу ли я помочь вам с чем-то из имеющегося ассортимента?",
  "reply_language": "r…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечных машин Bosch сейчас нет в наличии. Могу предложить вам другую технику для дома и кухни из нашего каталога. Что-то конкретное интересует?",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confiden…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:openai/gpt-4o-mini
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук. Если вас интересует доставка, уточните адрес, и я помогу оформить заказ!","reply_language":"kk","attach_groups":[],"escalate":false,"escalation_reason":…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:anthropic/claude-haiku-4.5
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук на складе в Алматы. Интересует? Помогу оформить заказ! 😊",
  "reply_language": "ru",
  "attach_groups": [
    "product.microwave-samsung.imag…`

**7. deep-list product (#25 of 30), Kazakh price question** | openrouter:openai/gpt-4o-mini
- message: Кофемолка Bosch қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемолка Bosch стоит {{product.coffee-grinder-bosch.price}}. У нас в наличии {{product.coffee-grinder-bosch.available_pieces}} штук. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я подготовлю счёт!","reply_language":"kk","attach_groups":[],"escalate":false,"esc…`

**7. deep-list product (#25 of 30), Kazakh price question** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемолка Bosch қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемолка Bosch стоит {{product.coffee-grinder-bosch.price}}. У нас в наличии {{product.coffee-grinder-bosch.available_pieces}} штук. Интересует? Могу помочь с заказом или ответить на вопросы 😊",
  "reply_language": "ru",
  "attach_groups": [
    "product.coffee-grinder-bo…`

