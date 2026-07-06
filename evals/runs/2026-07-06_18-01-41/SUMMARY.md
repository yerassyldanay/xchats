# Run 2026-07-06_18-01-41

Generated 2026-07-06 18:04. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost shows as n/a**: promptfoo has no pricing table for generic `openrouter:` provider IDs, so it reports $0 regardless of real spend — this is not "free", it's "unmeasured". Check OpenRouter's own dashboard for actual cost. Tokens are real and come straight from the API response.

## shop-current

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 10/12 (83%) | 12/12 (100%) | n/a | 6ms | 1802 |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | 12/12 (100%) | n/a | 6ms | 1408 |
| openrouter:openai/gpt-4o-mini | 9/12 (75%) | 12/12 (100%) | n/a | 7ms | 1310 |

## shop-decisions-v1

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 8/12 (67%) | 12/12 (100%) | n/a | 2812ms | 1397 |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | 12/12 (100%) | n/a | 2523ms | 1088 |
| openrouter:openai/gpt-4o-mini | 8/12 (67%) | 12/12 (100%) | n/a | 2950ms | 971 |

## xpayment-decisions-v1

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 3/3 (100%) | 3/3 (100%) | n/a | 3095ms | 1489 |
| openrouter:google/gemini-2.5-flash | 2/3 (67%) | 3/3 (100%) | n/a | 5695ms | 1118 |
| openrouter:openai/gpt-4o-mini | 2/3 (67%) | 3/3 (100%) | n/a | 2667ms | 1008 |

## Failures (verbatim)

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина стоит {{product.coffee-machine.price}}.","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она в наличии и готова к отправке. Хотите узнать больше или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-video"],
  "escalate": false,
  "escalation_reason": "",
  "confid…`

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
  "confidence": 1
}
````

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает от {{policy.main.delivery_time}} дней. Если сумма заказа превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Есть ли что-то конкретное, что вы хотите заказать?","reply_language":"ru","asset_refs":[]…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_time}}. Если сумма заказа от {{policy.main.free_delivery_from}}, доставка бесплатная. Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": ["delivery-zones"],
  "escala…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confide…`

**5. out-of-stock product** | openrouter:openai/gpt-4o-mini
- message: Набор посуды есть в наличии?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Набор посуды под заказ, срок доставки составляет 3–5 дней. Если вас это устраивает, дайте знать, и я помогу оформить заказ!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. У нас в наличии 5 штук. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я пришлю счёт в WhatsApp.","reply_language":"kk","attach_groups":["coffee-machine.images"],"escalate":false,"escalation_reason":"","…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она есть в наличии на складе в Алматы.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она в наличии на складе в Алматы — {{product.coffee-machine.available_pieces}} штук. Хотите заказать?",
  "reply_language": "ru",
  "attach_groups": ["coffee-machine.images"],
  "escalate": false,
  "escalation_re…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка стоит {{policy.main.delivery_cost}}, но она бесплатна при заказе от {{policy.main.free_delivery_from}}. Привезём за {{policy.main.delivery_in_days}} күнде по Алматы и области.\n\nЧто вас интересует? 😊",
  "reply_language": "kk",
  "attach_groups": ["delivery.image…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка стоит {{policy.main.delivery_cost}} ₸. Мы доставим за {{policy.main.delivery_in_days}} дня / күнде. Если у вас есть вопросы или хотите сделать заказ, дайте знать!","reply_language":"kk","attach_groups":[],"escalate":false,"escalation_reason":"","confidence":1}`

**5. out-of-stock product** | openrouter:openai/gpt-4o-mini
- message: Набор посуды есть в наличии?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"К сожалению, набор посуды сейчас под заказ, в наличии нет. Мы можем привезти его за {{policy.main.delivery_in_days}} дня / күнде. Если вас это устраивает, напишите, пожалуйста, адрес доставки, и я оформлю заказ!","reply_language":"ru","attach_groups":[],"escalate":true,"escalation_rea…`

**5. out-of-stock product** | openrouter:google/gemini-2.5-flash
- message: Набор посуды есть в наличии?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Набор посуды сейчас доступен под заказ. Мы привезём его за несколько дней. Хотите оформить заказ?",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**5. out-of-stock product** | openrouter:anthropic/claude-haiku-4.5
- message: Набор посуды есть в наличии?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "К сожалению, набор посуды сейчас закончился на складе. Но мы можем заказать его для вас — привезём за {{policy.main.delivery_in_days}} дня. Цена {{product.cookware-set.price}}. Интересует?",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalatio…`

**10. refund request** | openrouter:anthropic/claude-haiku-4.5
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: invented digits outside any token: 00, 19, 00
- raw output: ````json
{
  "reply_text": "Здравствуйте! Я помогу разобраться с возвратом. Уточню у коллеги детали вашего заказа — номер, дату и причину возврата. Свяжемся с вами в течение рабочего дня (9:00–19:00). Спасибо за терпение!",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": true,
  "escal…`

**10. refund request** | openrouter:openai/gpt-4o-mini
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: `{"reply_text":"Здравствуйте! Для возврата средств, пожалуйста, уточните номер вашего заказа и причину возврата. Мы обработаем ваш запрос в течение 14 дней. Если у вас есть дополнительные вопросы, не стесняйтесь спрашивать!","reply_language":"ru","attach_groups":[],"escalate":false,"escalation_reason…`

**14. payment limit, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Бизнес тарифінің төлем лимиті қандай?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Лимит по тарифу «Бизнес» составляет {{tariff.business.payment_limit_monthly}} тенге в месяц. Этот тариф оптимален для максимального объёма и приоритетной поддержки.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confid…`

**15. off-KB crypto question** | openrouter:openai/gpt-4o-mini
- message: А вы принимаете оплату криптовалютой?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не принимаем оплату криптовалютой. Мы работаем с традиционными методами оплаты. Если у вас есть другие вопросы или нужна помощь, пожалуйста, дайте знать!","reply_language":"ru","attach_groups": [],"escalate": false,"escalation_reason": "Нет информации о приеме криптова…`

