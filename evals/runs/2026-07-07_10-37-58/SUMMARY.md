# Run 2026-07-07_10-37-58

Generated 2026-07-07 12:31. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-07) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## shop-current

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 9/12 (75%) | 12/12 (100%) | unknown pricing | 6ms (cached — not meaningful) | 1994 | n/a |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | 12/12 (100%) | unknown pricing | 6ms (cached — not meaningful) | 1580 | n/a |
| openrouter:openai/gpt-4o-mini | 7/12 (58%) | 12/12 (100%) | unpriceable (cached, no split to borrow) | 8ms (cached — not meaningful) | 1479 | n/a |

## shop-decisions-v1

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 10/12 (83%) | 12/12 (100%) | unknown pricing | 6ms (cached — not meaningful) | 1730 | n/a |
| openrouter:google/gemini-2.5-flash | 11/12 (92%) | 11/12 (92%) | unknown pricing | 6ms (cached — not meaningful) | 1259 | n/a |
| openrouter:openai/gpt-4o-mini | 9/12 (75%) | 12/12 (100%) | unpriceable (cached, no split to borrow) | 8ms (cached — not meaningful) | 1243 | n/a |

## xpayment-decisions-v1

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 3/3 (100%) | 3/3 (100%) | unknown pricing | 11ms (cached — not meaningful) | 1801 | n/a |
| openrouter:google/gemini-2.5-flash | 3/3 (100%) | 3/3 (100%) | unknown pricing | 9ms (cached — not meaningful) | 1383 | n/a |
| openrouter:openai/gpt-4o-mini | 2/3 (67%) | 3/3 (100%) | unpriceable (cached, no split to borrow) | 14ms (cached — not meaningful) | 1255 | n/a |

## Failures (verbatim)

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина стоит {{product.coffee-machine.price}}. Если хотите, могу помочь оформить заказ или ответить на другие вопросы!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите узнать больше о характеристиках или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1"],
  "escalate": false,
  "escalation_reason": "…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если ваш заказ превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной! Есть ли у вас адрес для расчета доставки?","reply_language":"ru","asset_refs":[],"escalate":false,"e…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}, срок {{policy.main.delivery_time}}. Если сумма заказа от {{policy.main.free_delivery_from}}, доставка становится бесплатной. Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. Срок доставки обычно составляет {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escala…`

**4. stock quantity question, Russian** | openrouter:openai/gpt-4o-mini
- message: А сколько штук кофемашины есть в наличии?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Кофемашина DeLonghi в наличии. Если хотите узнать цену или оформить заказ, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**5. out-of-stock product** | openrouter:openai/gpt-4o-mini
- message: Набор посуды есть в наличии?
- reason: invented digits outside any token: 3, 5
- raw output: `{"reply_text":"Набор посуды сейчас под заказ, срок доставки составляет 3–5 дней. Если хотите оформить заказ, напишите, пожалуйста, адрес доставки!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalated, but reply_text still commits to an invented answer ("не доставляем в астан")
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":true,"escalation_reason":"Запрос о доставке в Астану, информация отсутствует в б…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем с заказом. Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "esca…`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. В наличии {{product.coffee-machine.available_pieces}} дана. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю вам счёт в WhatsApp.","reply_language":"kk","attach_groups":[],"escalate":false,"esca…`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. В наличии {{product.coffee-machine.available_pieces}} штук на складе в Алматы. Интересует? Напишите, и мы оформим заказ — доставим за {{policy.main.delivery_in_days}} дня.",
  "reply_language": "ru",
  "attach_gro…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Жеткізу құны {{policy.main.delivery_cost}}. Тапсырысыңыз 1–3 күні жетеді.","reply_language":"kk","attach_groups":[],"escalate":false,"escalation_reason":"","confidence":1}`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по городу стоит {{policy.main.delivery_cost}}, но она бесплатна при заказе от {{policy.main.free_delivery_from}}. Привезём за {{policy.main.delivery_in_days}} дня. Какой товар вас интересует?",
  "reply_language": "ru",
  "attach_groups": ["topic.delivery.images"]…`

**10. refund request** | openrouter:openai/gpt-4o-mini
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: `{"reply_text":"Для возврата средств, пожалуйста, уточните номер вашего заказа, и мы начнем процесс возврата. Срок возврата составляет 14 дней.","reply_language":"ru","attach_groups":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**10. refund request** | openrouter:google/gemini-2.5-flash
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Здравствуйте! Чтобы оформить возврат, пожалуйста, уточните номер вашего заказа или дату покупки. Я уточню у коллеги, как мы можем вам помочь.",
  "reply_language": "ru",
  "attach`

**15. off-KB crypto question** | openrouter:openai/gpt-4o-mini
- message: А вы принимаете оплату криптовалютой?
- reason: escalated, but reply_text still commits to an invented answer ("не принимаем оплату криптовалют")
- raw output: `{"reply_text":"К сожалению, мы не принимаем оплату криптовалютой. Мы работаем с традиционными методами оплаты. Если у вас есть другие вопросы или нужна помощь, пожалуйста, дайте знать!","reply_language":"ru","attach_groups":[],"escalate":true,"escalation_reason":"Нет информации о приёме криптовалюты…`

