# Run 2026-07-06_18-57-26

Generated 2026-07-06 19:32. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost shows as n/a**: promptfoo has no pricing table for generic `openrouter:` provider IDs, so it reports $0 regardless of real spend — this is not "free", it's "unmeasured". Check OpenRouter's own dashboard for actual cost. Tokens are real and come straight from the API response.

## shop-current

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 9/12 (75%) | 12/12 (100%) | n/a | 6ms | 1802 |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | 12/12 (100%) | n/a | 6ms | 1408 |
| openrouter:openai/gpt-4o-mini | 9/12 (75%) | 12/12 (100%) | n/a | 7ms | 1310 |

## shop-decisions-v1

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 10/12 (83%) | 12/12 (100%) | n/a | 3428ms | 1534 |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | 12/12 (100%) | n/a | 2413ms | 1209 |
| openrouter:openai/gpt-4o-mini | 9/12 (75%) | 12/12 (100%) | n/a | 3486ms | 1085 |

## xpayment-decisions-v1

| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |
|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 3/3 (100%) | 3/3 (100%) | n/a | 3867ms | 1637 |
| openrouter:google/gemini-2.5-flash | 2/3 (67%) | 3/3 (100%) | n/a | 5060ms | 1233 |
| openrouter:openai/gpt-4o-mini | 2/3 (67%) | 3/3 (100%) | n/a | 2752ms | 1114 |

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

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalated, but reply_text still commits to an invented answer ("не доставляем в астан")
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует доставка по Алматы или есть вопросы по товарам, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":true,"escalation_reason":"Запрос на доставку в Астану, информация отсутству…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalated, but reply_text still commits to an invented answer ("за пределами нашей зоны")
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть знакомые в Алматы, можно отправить туда, или напишите нам в Instagram @demoshop.kz — может быть, найдём решение.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она есть в наличии на складе в Алматы.",
  "reply_language": "kk",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. У нас в наличии {{product.coffee-machine.available_pieces}} штук на складе в Алматы. Интересует? Напишите, и мы оформим заказ! 😊",
  "reply_language": "ru",
  "attach_groups": ["product.coffee-machine.images"],
  …`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. У нас в наличии 5 штук. Если вас интересует, напишите, пожалуйста, адрес доставки, и мы подтвердим заказ. Счёт пришлём в WhatsApp.","reply_language":"казахский","attach_groups":[],"escalate":false,"escalation_reason":"","conf…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по городу стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_in_days}} күнде. Если ваш заказ превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Если у вас есть дополнительные вопросы, дайте знать!","reply_language":"kk","attach_groups"…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка стоит {{policy.main.delivery_cost}}, но она бесплатна при заказе от {{policy.main.free_delivery_from}}. Привезём за {{policy.main.delivery_in_days}} дня. Какой товар вас интересует?",
  "reply_language": "ru",
  "attach_groups": ["topic.delivery.images"],
  "escal…`

**10. refund request** | openrouter:google/gemini-2.5-flash
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Чтобы оформить возврат, пожалуйста, свяжитесь с нашим менеджером по телефону {{contact.support.phone}}. Он поможет вам с этим вопросом.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**10. refund request** | openrouter:openai/gpt-4o-mini
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: `{"reply_text":"Здравствуйте! Для возврата средств, пожалуйста, уточните номер вашего заказа, чтобы мы могли помочь вам быстрее. Напоминаем, что срок возврата составляет 14 дней. Ждем вашего ответа!","reply_language":"ru","attach_groups": [],"escalate": false,"escalation_reason": "","confidence": 0.9…`

**14. payment limit, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Бизнес тарифінің төлем лимиті қандай?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Лимит по платежам для тарифа «Бизнес» составляет до {{tariff.business.payment_limit_monthly}} в месяц. Этот тариф оптимален для крупных компаний, которым важен максимальный объём операций и приоритетная поддержка.",
  "reply_language": "ru",
  "attach_groups": [],
  "escal…`

**15. off-KB crypto question** | openrouter:openai/gpt-4o-mini
- message: А вы принимаете оплату криптовалютой?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не принимаем оплату криптовалютой. Мы работаем с традиционными методами оплаты. Если у вас есть другие вопросы или нужна помощь, дайте знать!","reply_language":"ru","attach_groups": [],"escalate": false, "escalation_reason": "", "confidence": 0.9}`

