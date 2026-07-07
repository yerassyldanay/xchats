# Run 2026-07-07_10-37-58

Generated 2026-07-07 10:38. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost shows as n/a**: promptfoo has no pricing table for generic `openrouter:` provider IDs, so it reports $0 regardless of real spend — this is not "free", it's "unmeasured". Check OpenRouter's own dashboard for actual cost. Tokens are real and come straight from the API response.

## shop-current

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 9/12 (75%) | 12/12 (100%) | n/a | 6ms | 1994 |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | 12/12 (100%) | n/a | 6ms | 1580 |
| openrouter:openai/gpt-4o-mini | 8/12 (67%) | 12/12 (100%) | n/a | 8ms | 1479 |

## shop-decisions-v1

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 10/12 (83%) | 12/12 (100%) | n/a | 6ms | 1730 |
| openrouter:google/gemini-2.5-flash | 11/12 (92%) | 11/12 (92%) | n/a | 6ms | 1259 |
| openrouter:openai/gpt-4o-mini | 9/12 (75%) | 12/12 (100%) | n/a | 8ms | 1243 |

## xpayment-decisions-v1

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 3/3 (100%) | 3/3 (100%) | n/a | 11ms | 1801 |
| openrouter:google/gemini-2.5-flash | 3/3 (100%) | 3/3 (100%) | n/a | 9ms | 1383 |
| openrouter:openai/gpt-4o-mini | 2/3 (67%) | 3/3 (100%) | n/a | 14ms | 1255 |

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

