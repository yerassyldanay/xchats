# Run 2026-07-07_12-44-52

Generated 2026-07-07 12:47. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-07) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## shop-decisions-v1

| model | model-behavior pass | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 15/19 (79%) | 19/19 (100%) | unknown pricing | 2822ms | 1762 | n/a |
| openrouter:google/gemini-2.5-flash | 17/19 (89%) | 19/19 (100%) | unknown pricing | 2207ms | 1394 | n/a |
| openrouter:openai/gpt-4o-mini | 13/19 (68%) | 19/19 (100%) | $0.0043 est. (19 measured) | 2073ms | 1268 | 94% |

## Failures (verbatim)

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. У нас в наличии {{product.coffee-machine.available_pieces}} штук. Доставка по городу занимает {{policy.main.delivery_in_days}} дня. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю счёт в WhatsA…`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Сейчас в наличии {{product.coffee-machine.available_pieces}} штук на складе в Алматы. Хотите узнать больше о характеристиках или оформить заказ?",
  "reply_language": "ru",
  "attach_groups": [
    "product.coffee…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка стоит {{policy.main.delivery_cost}}. Мы доставляем за {{policy.main.delivery_in_days}} дня. Если сумма заказа превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Если у вас есть дополнительные вопросы, пишите!","reply_language":"kk","attach_groups":[],"es…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка стоит {{policy.main.delivery_cost}}, но она бесплатна при заказе от {{policy.main.free_delivery_from}}. Привезём за {{policy.main.delivery_in_days}} дня. Какой товар вас интересует? 😊",
  "reply_language": "ru",
  "attach_groups": ["topic.delivery.images"],
  "esc…`

**4. stock quantity question, Russian** | openrouter:openai/gpt-4o-mini
- message: А сколько штук кофемашины есть в наличии?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"В наличии кофемашин DeLonghi 5 штук. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю счёт в WhatsApp.","reply_language":"ru","attach_groups":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalated, but reply_text still commits to an invented answer ("не доставляем в астан")
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по городу и области Алматы. Если у вас есть другие вопросы или интересует какой-то товар, дайте знать!","reply_language":"ru","attach_groups": [],"escalate": true,"escalation_reason":"необходима дополнительная информация о дост…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 Мы доставляем по городу и области. Астана входит в нашу зону доставки. Доставка обойдётся в {{policy.main.delivery_cost}}, но если заказ от {{policy.main.free_delivery_from}} — доставка бесплатна. Привезём за {{policy.main.delivery_in_days}} дня.\n\nЧто вас интер…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Напишите, что вас интересует, и укажите адрес доставки, чтобы мы могли оформить заказ!","reply_language":"ru","attach_groups":[],"escalate":false,"escalation_reason":"","confidence":1}`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, iPhone 15 Pro нет в нашем ассортименте. Мы специализируемся на бытовой технике и товарах для дома. Могу предложить вам автоматическую кофемашину DeLonghi или набор посуды.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_re…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:anthropic/claude-haiku-4.5
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 Мы продаём кофемашины DeLonghi, но ремонтом не занимаемся. Если у вас есть вопросы по покупке или гарантии на наши аппараты — с удовольствием поможем! Интересует кофемашина?",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_rea…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:openai/gpt-4o-mini
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply_language field is "kk", expected "ru"
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. В наличии {{product.coffee-machine.available_pieces}} штук. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю вам счёт в WhatsApp!","reply_language":"kk","attach_groups":["product.coffee-machine.…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi автоматты кофемашинасының бағасы {{product.coffee-machine.price}} құрайды. Қоймада {{product.coffee-machine.available_pieces}} дана бар.",
  "reply_language": "kk",
  "attach_groups": [
    "product.coffee-machine.images"
  ],
  "escalate…`

