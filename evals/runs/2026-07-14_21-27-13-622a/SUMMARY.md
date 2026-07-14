# Run 2026-07-14_21-27-13-622a

Generated 2026-07-14 21:35. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-11) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## escalation-canary-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 6/18 (33%) | [16%, 56%] | 18/18 (100%) | $0.0493 est. (18 measured) | 2663ms | 2033 | 91% |
| openrouter:google/gemini-2.5-flash | 5/18 (28%) | [12%, 51%] | 17/18 (94%) | $0.0132 est. (17 measured, 1 cached-borrowed) | 5333ms | 1526 | 93% |
| openrouter:google/gemini-2.5-flash-lite | 3/18 (17%) | [6%, 39%] | 18/18 (100%) | $0.0035 est. (18 measured) | 923ms | 1613 | 93% |
| openrouter:openai/gpt-4o-mini | 7/18 (39%) | [20%, 61%] | 18/18 (100%) | $0.0048 est. (18 measured) | 2723ms | 1510 | 94% |

## escalation-canary-v2

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 12/18 (67%) | [44%, 84%] | 18/18 (100%) | $0.0526 est. (18 measured) | 2587ms | 2249 | 93% |
| openrouter:google/gemini-2.5-flash | 13/18 (72%) | [49%, 88%] | 17/18 (94%) | $0.0133 est. (17 measured, 1 cached-borrowed) | 5926ms | 1664 | 95% |
| openrouter:google/gemini-2.5-flash-lite | 8/18 (44%) | [25%, 66%] | 18/18 (100%) | $0.0038 est. (18 measured) | 1095ms | 1779 | 94% |
| openrouter:openai/gpt-4o-mini | 15/18 (83%) | [61%, 94%] | 18/18 (100%) | $0.0049 est. (18 measured) | 2471ms | 1641 | 96% |

## lang-canary-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 9/21 (43%) | [24%, 63%] | 21/21 (100%) | $0.0550 est. (21 measured) | 3031ms | 2012 | 92% |
| openrouter:google/gemini-2.5-flash | 11/21 (52%) | [32%, 72%] | 21/21 (100%) | $0.0152 est. (21 measured) | 2588ms | 1614 | 93% |
| openrouter:google/gemini-2.5-flash-lite | 9/21 (43%) | [24%, 63%] | 21/21 (100%) | $0.0041 est. (21 measured) | 962ms | 1619 | 93% |
| openrouter:openai/gpt-4o-mini | 6/21 (29%) | [14%, 50%] | 21/21 (100%) | $0.0053 est. (21 measured) | 2178ms | 1496 | 96% |

## lang-canary-v2

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 13/21 (62%) | [41%, 79%] | 19/21 (90%) | $0.0622 est. (21 measured) | 3072ms | 2287 | 93% |
| openrouter:google/gemini-2.5-flash | 13/21 (62%) | [41%, 79%] | 18/21 (86%) | $0.0172 est. (18 measured, 3 cached-borrowed) | 8106ms | 1568 | 93% |
| openrouter:google/gemini-2.5-flash-lite | 14/21 (67%) | [45%, 83%] | 21/21 (100%) | $0.0046 est. (21 measured) | 1055ms | 1831 | 93% |
| openrouter:openai/gpt-4o-mini | 3/21 (14%) | [5%, 35%] | 21/21 (100%) | $0.0059 est. (21 measured) | 1916ms | 1695 | 97% |

## lang-canary-v4-ru

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:anthropic/claude-haiku-4.5 | 9/12 (75%) | [47%, 91%] | 12/12 (100%) | $0.0310 est. (12 measured) | 2365ms | 2007 | 93% |
| openrouter:google/gemini-2.5-flash | 8/12 (67%) | [39%, 86%] | 11/12 (92%) | $0.0086 est. (11 measured, 1 cached-borrowed) | 6811ms | 1481 | 93% |
| openrouter:google/gemini-2.5-flash-lite | 8/12 (67%) | [39%, 86%] | 11/12 (92%) | $0.0023 est. (11 measured, 1 cached-borrowed) | 1058ms | 1477 | 94% |
| openrouter:openai/gpt-4o-mini | 6/12 (50%) | [25%, 75%] | 12/12 (100%) | $0.0030 est. (12 measured) | 1918ms | 1492 | 96% |

## Failures (verbatim)

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [
    "deliver…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог назвать точные условия.",
  "reply_language": "ru",
  "asset_refs": [],
…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану, только по Алматы и области. Если у вас есть вопросы по товарам или доставке в пределах Алматы, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем с заказом. Хотите узнать про доставку в ваш район?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть вопросы по товарам или доставке по этим регионам, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точные сроки и стоимость.",
  "reply_language":…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_rea…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [
    "deliver…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [
    "deliver…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_rea…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите узнать точную стоимость доставки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с выбором!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Я понимаю, что вы хотите вернуть деньги за прошлый заказ. Для решения этого вопроса, пожалуйста, свяжитесь с нашей службой поддержки по телефону {{contact.support.phone}} или напишите нам в Instagram @demoshop.kz. Они смогут вам помочь.",
  "reply_language": …`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Чтобы оформить возврат, пожалуйста, напишите номер вашего заказа и причину возврата. Мы проверим информацию и свяжемся с вами.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**10. refund request** | openrouter:google/gemini-2.5-flash
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Здравствуйте! Я — ассистент интернет-магазина «Demo Shop».`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Чтобы оформить возврат, пожалуйста, сообщите номер вашего прошлого заказа. Мы проверим информацию и свяжемся с вами для уточнения деталей.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
``…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! В нашем каталоге есть техника для дома, например, кофемашины и наборы посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить посмотреть другие товары?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason"…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate":…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. К сожалению, смартфоны мы не продаём. Может быть, вас интересует что-то из того, что у нас есть? Или я помогу вам с чем-то ещё?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalat…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! У нас в каталоге есть техника и товары для дома, например, кофемашина и набор посуды. iPhone 15 Pro, к сожалению, сейчас нет в наличии. Могу предложить посмотреть другие товары?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalatio…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone у нас не продаём. Может быть, вас интересует что-то из техники для кухни? Расскажите, что ищете!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate":…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! В нашем каталоге есть техника для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить вам посмотреть другие товары?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reas…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Если интересует что-то из того, что у нас есть, или у вас другой вопрос — напишите, помогу! 😊",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fal…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. iPhone 15 Pro, к сожалению, нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate"…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:openai/gpt-4o-mini
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не занимаемся ремонтом кофемашин. Мы можем предложить вам купить новую кофемашину. Если вас интересует, я могу сообщить цену и наличие. Какой товар вас интересует?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**19. wrong understanding, asks about a service we don't offer** | openrouter:anthropic/claude-haiku-4.5
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы — интернет-магазин, поэтому ремонтом не занимаемся. Но если вам нужна новая кофемашина, у нас есть DeLonghi по цене {{product.coffee-machine.price}}. На технику действует {{policy.main.warranty}}. Интересует?",
  "reply_language": "ru",
  "asset_refs"…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  …`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить новые. В нашем каталоге есть отличная кофемашина DeLonghi. Хотите узнать о ней подробнее?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reaso…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:anthropic/claude-haiku-4.5
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы — интернет-магазин, поэтому ремонтом не занимаемся. Но если вам нужна новая кофемашина, у нас есть DeLonghi по цене {{product.coffee-machine.price}}. На технику даём {{policy.main.warranty}}.\n\nЕсли у вас вопрос по гарантии или сервису — свяжитесь с …`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  …`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить новые модели. В нашем каталоге есть отличная кофемашина DeLonghi. Хотите узнать о ней подробнее?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confid…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:openai/gpt-4o-mini
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не занимаемся ремонтом кофемашин. Мы можем предложить вам купить новую. Если вас интересует кофемашина DeLonghi, она стоит {{product.coffee-machine.price}} и сейчас в наличии. Хотите узнать больше?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_rea…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:anthropic/claude-haiku-4.5
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! Мы продаём кофемашины, но ремонтом не занимаемся. Если у вас есть вопрос по гарантии или неисправности купленного у нас товара, свяжитесь с поддержкой: {{contact.support.phone}}, Пн–Сб {{contact.support.working_hours}}. Или напишите в Instagram {{contact.support.in…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы не занимаемся ремонтом техники. Могу помочь с выбором новой кофемашины или другого товара для дома из нашего каталога.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.8
}
````

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом, но можем предложить вам новые кофемашины. Например, модель DeLonghi сейчас в наличии. Вас интересует покупка или есть другие вопросы?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "es…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по Алматы и области. Астана — это за пределами нашей зоны доставки. Если вы в Алматы или близко, с удовольствью поможем. Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а при заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_time}}.",
  "reply_language": "ru",
…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точнее по вашему адресу?",
  "reply_language": "ru",
  "asset_refs": [
    "delivery-zones"
  ],
  "escala…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не осуществляем доставку в Астану. Доставляем только по Алматы и области. Если вам нужно что-то из нашего каталога, дайте знать, и я помогу вам с заказом!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а при заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_time}}.",
  "reply_language": "ru",
…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по Алматы и области. К сожалению, точную информацию по Астане мне нужно уточнить с командой. Напишите, пожалуйста, свой адрес, и я проверю возможность доставки именно туда. Или свяжитесь с поддержкой: {{contact.support.phone}}, {{contact.su…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Могу я узнать ваш адрес, чтобы точнее рассчитать стоимость и сроки?",
  "reply_language": "ru"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть вопросы о товарах или доставке по Алматы, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по Алматы и области. Астана входит в нашу зону доставки — срок {{policy.main.delivery_time}}, стоимость {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатна.\n\nЧто вас интересует? Помогу подобр…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — {{policy.main.delivery_cost}}, а при заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_time}}.",
  "reply_language": "ru",
  "asset_…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. Уточните, пожалуйста, ваш адрес, чтобы я мог проверить возможность доставки и её условия.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  …`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной! Хотите узнать что-то еще или оформить заказ?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки. Также, при заказе на сумму от 20 000 ₸ доставка будет бесплатной! Какой товар вас интересует?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confiden…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Какой товар вы хотите заказать?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка осуществляется по городу и области. Стоимость и сроки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка по Алматы будет бесплатной. Уточните, пожалуйста, ваш город, чтобы я мог назвать точные условия.",
  "reply_la…`

**10. refund request** | openrouter:google/gemini-2.5-flash
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Чтобы я мог помочь вам с возвратом денег, пожалуйста, уточните номер вашего заказа или дату покупки. Я проверю информацию и постараюсь решить ваш вопрос.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.8
}
`…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! У нас в каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro в списке товаров пока нет. Могу предложить посмотреть другие варианты техники или посуды?",
  "reply_language": "ru",
  "asset_refs": [],
  "escal…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может, интересует что-то из того, что у нас есть? Или ищете что-то конкретное?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalati…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! У нас в каталоге есть техника и товары для дома. Например, кофемашина DeLonghi стоит {{product.coffee-machine.price}} и есть в наличии. Также есть набор посуды за {{product.cookware-set.price}}, он под заказ на 3-5 дней. Скажите, что именно вас интересует, и …`

**18. wrong understanding, asks about a product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может быть, вас интересует что-то из того, что у нас есть? Или расскажите, что ещё ищете — постараюсь помочь!",
  "reply_language": "ru",
  "asset_refs": [],
 …`

**18. wrong understanding, asks about a product we don't sell** | openrouter:anthropic/claude-haiku-4.5
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может, вас интересует что-то из того, что у нас есть? Или уточню информацию по другому товару?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! У нас в каталоге есть техника и товары для дома. Например, кофемашина DeLonghi стоит {{product.coffee-machine.price}} и есть в наличии. Также есть набор посуды за {{product.cookware-set.price}}, он поставляется под заказ в течение 3–5 дней. Если вас интересуе…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Я уточню информацию и вернусь к вам с точным ответом.",
  "reply_language": "ru",
  "asset_refs":`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Вас интересует кофемашина DeLonghi? Она сейчас в наличии и стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate":…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин. Могу предложить вам новую кофемашину DeLonghi. Она сейчас в наличии, цена — {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reaso…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка по Алматы будет бесплатной. Хотите узнать точнее про доставку в Астану?",
  "reply_language": "ru",
  "asset_refs…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по городу и области. Точные сроки и стоимость доставки зависят от адреса. Уточните, пожалуйста, ваш адрес в Астане — и я назову точную стоимость и срок доставки для вас.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
 …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Доставка в Астану не указана в базе знаний, есть только по городу и области.",
  "confidence": 0.8
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка по городу и области зависит от адреса. Скажите, пожалуйста, куда именно вы хотели бы оформить доставку, и я смогу уточнить сроки и стоимость.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidenc…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по городу и области. Точные сроки и стоимость доставки зависят от адреса. Уточню информацию по Астане и вернусь с точным ответом.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Delivery to Astana…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. Уточните, пожалуйста, ваш адрес в Астане, чтобы я могла рассчитать точнее.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по городу и области. Точные сроки и стоимость доставки зависят от адреса. Уточню информацию по Астане и вернусь с точным ответом.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Delivery to Astana…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка по Алматы будет бесплатной. Хотите узнать подробнее о доставке в Астану?",
  "reply_language": "ru",
  "asset_ref…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Я уточню информацию и вернусь к вам с точным ответом.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "The knowledge base states delivery is within the city and region, but does not specify if Astana is included in the delivery re…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

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

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Если хотите, могу рассказать о её характеристиках или помочь с оформлением заказа!","reply_language":"ru","asset_refs":["coffee-photo-1","coffee-photo-2","coffee-video"],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите узнать больше или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-video"],
  "escalate": false,
  "escalation_reason": "",…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она сейчас в наличии. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-photo-2",
    "coffee-photo-3"
  ],
  "escalate": false,
  "escalation_rea…`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Если вас интересует, как оформить заказ или нужна дополнительная информация, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите узнать больше о характеристиках или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-video"],
  "escalate": false,
  "escal…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она сейчас в наличии. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она есть {{product.coffee-machine.availability}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-video"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

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

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Если хотите, могу помочь с оформлением заказа!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите узнать больше о характеристиках или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-video"],
  "escalate": false,
  "escal…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она сейчас в наличии. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-photo-2",
    "coffee-photo-3"
  ],
  "escalate": false,
  "escalation_rea…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если ваш заказ превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной! Есть ли у вас конкретный адрес для расчета?","reply_language":"ru","asset_refs":[],"escalate":false,…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_time}}. Если сумма заказа от {{policy.main.free_delivery_from}}, доставка бесплатная. Точная стоимость и время зависят от вашего адреса. Какой товар вас интересует?",
  "reply_language": …`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите оформить заказ или узнать больше о товарах?",
  "reply_language": "ru",
  "asset_refs": [],
 …`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка по Алматы стоит {{policy.main.delivery_cost}}, а срок доставки составляет {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если сумма заказа превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Если у вас есть еще вопросы или хотите сделать заказ, дайте знать!","reply_language":"ru","asset_…`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_time}}. Если сумма заказа от {{policy.main.free_delivery_from}}, доставка будет бесплатной. Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а срок доставки — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_ref…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка по Алматы стоит {{policy.main.delivery_cost}}, а срок доставки составляет {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка по Алматы стоит {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки составляет {{policy.main.delivery_time}}.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  …`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. Обычно мы привозим заказы в течение {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите оформить заказ или узнать больше о товарах?",
  "reply_language": "ru",
 …`

**3. delivery cost + time, Kazakh** | openrouter:anthropic/claude-haiku-4.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}, срок {{policy.main.delivery_time}}. Если заказ на сумму {{policy.main.free_delivery_from}} и выше — доставка бесплатная! 🚚\n\nКакой товар вас интересует? Помогу подобрать и оформить заказ.",
  "reply_language": "ru",
…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если сумма заказа превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Если у вас есть еще вопросы или вы хотите оформить заказ, дайте знать!","reply_language":"ru","as…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите узнать точную стоимость для Астаны?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reaso…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_rea…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [
    "deliver…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_rea…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с выбором!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точные условия.",
  "reply_language": "ru",
  "asset_re…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точнее.",
  "reply_language": "ru",
  "asset_refs": [],
  "es…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_rea…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с выбором!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы доставляем только по Алматы и области. Если у вас есть вопросы по товарам или условиям доставки, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — {{policy.main.delivery_cost}}, а срок — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать подробнее о товарах или оформить…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Могу уточнить информацию по вашему адресу?",
  "reply_language": "ru",
  "asset_refs": [
    "delivery-zon…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по Алматы и области. Астана — это за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием поможем с заказом. Или свяжитесь с нашей поддержкой — может быть, найдём решение: {{contact.support.phone}}, {{contact.suppo…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — {{policy.main.delivery_cost}}, а срок — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчит…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу Алматы и области. Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. К сожалению, в Астану пока не доставля…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы доставляем только по Алматы и области. Если у вас есть вопросы по товарам или доставке в Алматы, с удовольствием помогу!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Спасибо за вопрос! Мы доставляем по Алматы и области. Астана входит в нашу зону доставки — точную стоимость и срок уточним по вашему адресу при оформлении заказа. Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Можете уточнить ваш адрес, чтобы я рассчитал точную стоимость и сроки?",
  "reply_language": "…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а срок — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Скажите, какой товар вас интересует?"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану, только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. К сожалению, в Knowledge Base нет информации о доставке в Астану. Уточню для вас — свяжитесь с нашей поддержкой по телефону {{contact.support.phone}} или в Instagram {{contact.support.instagram}}. График работы: {{contact.support.working_…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить сроки доставки. Также, при заказе на сумму от 20 000 ₸ доставка будет бесплатной! Какой товар вас интересует?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confide…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi тұрады {{product.coffee-machine.price}}.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**2. price question, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если ваш заказ превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Чем могу помочь еще?","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"",…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если сумма заказа превысит {{policy.main.free_delivery_from}}, доставка будет бесплатной. Есть ли что-то еще, с чем я могу помочь?","reply_language":"kk","asset_refs":[],"escalate":false,"…`

**3. delivery cost + time, Kazakh** | openrouter:openai/gpt-4o-mini
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{"reply_text":"Доставка по Алматы стоит {{policy.main.delivery_cost}} и занимает {{policy.main.delivery_time}}. Если сумма заказа превышает {{policy.main.free_delivery_from}}, доставка будет бесплатной. Чем могу помочь еще?","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если вы в Алматы, с удовольствью поможем! Доставка стоит {{policy.main.delivery_cost}}, срок {{policy.main.delivery_time}}. А при заказе от {{policy.main.free_delivery_from}} доставка бесплатна. …`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите узнать точную стоимость доставки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "esc…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпарат біздің білім базамызда жоқ, сондықтан бұл сұрақты нақтылау үшін бізге қосымша ақпарат қажет болады. Сізге қалай көмектесе аламын?",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": …`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставка доступна только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с заказом!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпарат біздің білім базамызда жоқ, сондықтан бұл сұрақты нақтылау үшін эскалация қажет.",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Доставка в Астану н…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если вы в Алматы, с удовольствью поможем! Доставка стоит {{policy.main.delivery_cost}}, срок {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатна. Чт…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка бесплатная.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если вы в Алматы, с удовольствью поможем! Доставка стоит {{policy.main.delivery_cost}}, срок {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатна. Чт…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Біз Алматы қаласы мен облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} жоғары болса, жеткізу тегін болады. Сізге жеткізу туралы толығырақ ақпарат беру үшін мекенжайыңызды нақтылай …`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:openai/gpt-4o-mini
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply_language field is "kk", expected "ru"
- raw output: `{"reply_text":"Кофемашина DeLonghi тұрады {{product.coffee-machine.price}}.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi {{product.coffee-machine.price}} тұрады. Ол қазіргі уақытта {{product.coffee-machine.availability}}. Сізге оны қалай жеткізуге болады?",
  "reply_language": "kk",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalat…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:anthropic/claude-haiku-4.5
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi {{product.coffee-machine.price}} тұрады. {{product.coffee-machine.availability}}. Қызығушылық танытсаңыз, заказ оформдап беремін — адресіңізді айтыңыз, ал қалғанын біз ойластырамыз.",
  "reply_language": "kk",
  "asset_refs": [
    "coffe…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы {{product.coffee-machine.price}} тұрады. Ол қазір {{product.coffee-machine.availability}}.",
  "reply_language": "kk",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence":…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:openai/gpt-4o-mini
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply_language field is "kk", expected "ru"
- raw output: `{"reply_text":"Кофемашина DeLonghi бағасы {{product.coffee-machine.price}}.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi {{product.coffee-machine.price}} тұрады. Ол қазіргі уақытта {{product.coffee-machine.availability}}. Сізге осы тауарды сатып алуға көмектесуімді қалайсыз ба?",
  "reply_language": "kk",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:anthropic/claude-haiku-4.5
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi тұрады {{product.coffee-machine.price}}. Ол қазір {{product.coffee-machine.availability}}. Қызығушылық танытсаңыз, фотосын көрсетіп беремін немесе заказ оформдау бойынша көмектесемін!",
  "reply_language": "kk",
  "asset_refs": [
    "cof…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы {{product.coffee-machine.price}} тұрады. Ол қазір {{product.coffee-machine.availability}} бар.",
  "reply_language": "kk",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confiden…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:openai/gpt-4o-mini
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply_language field is "kk", expected "ru"
- raw output: `{"reply_text":"Кофемашина DeLonghi бағасы {{product.coffee-machine.price}}.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi {{product.coffee-machine.price}} тұрады. Ол қазіргі уақытта {{product.coffee-machine.availability}}. Сізге басқа қандай да бір ақпарат керек пе?",
  "reply_language": "kk",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-photo-2",
   …`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:anthropic/claude-haiku-4.5
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi {{product.coffee-machine.price}} және {{product.coffee-machine.availability}}. Қызығушылық танытсаңыз, біз сізге толық ақпарат беріп, тапсырысты ресімдеуге көмектесе аламыз. Басқа сұрақтарыңыз бар ма?",
  "reply_language": "kk",
  "asset_…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы {{product.coffee-machine.price}}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"Иә, біз Астанаға да жеткіземіз. Жеткізу құны мен мерзімі мекен-жайыңызға байланысты. Сізге қандай өнім қызықтырады?","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы бойынша және облысқа жеткіземіз. Астанаға жеткізу шарттарын тексеру қажет. Сіз қандай тауарды алғыңыз келеді?",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу мүмкінді`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"Иә, біз Астанаға жеткіземіз. Жеткізу мерзімі мен құны сіздің мекен-жайыңызға байланысты. Қосымша ақпарат алу үшін, мекен-жайыңызды жазыңыз.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлем! Біз Алматы қаласы және облысына жеткіземіз. Өкінішке орай, Астанаға әзірге жеткізе алмаймыз. Бірақ сіз басқа қалаларға тапсырыс беруге болады. Қосымша ақпарат алу үшін {{contact.support.phone}} номеріне хабарласыңыз — {{contact.support.working_hours}} ұстап тұрамыз.…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:openai/gpt-4o-mini
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"Иә, біз Астанаға жеткіземіз. Жеткізу құны мен мерзімі мекенжайыңызға байланысты. Егер сіз нақты мекенжайыңызды айтсаңыз, мен сізге ақпарат бере аламын.","reply_language":"kk","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:anthropic/claude-haiku-4.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлем! Біз Алматы қаласы және облысына жеткіземіз. Өкінішке орай, Астанаға әзірге жеткізе алмаймыз. Бірақ сіз басқа қалаларға тапсырыс беруге болады. Сізге қандай товар қажет екенін айтсаңыз, көмектесе аламын! 😊",
  "reply_language": "kk",
  "asset_refs": [
    "delivery-z…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:anthropic/claude-haiku-4.5
- message: А доставка сколько будет стоить?
- reason: unknown token(s), draft would be BLOCKED: {{policy.main.delivery_from}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. Срок доставки {{policy.main.delivery_time}}. Если сумма заказа будет {{policy.main.delivery_from}}, доставка станет бесплатной. Какой адрес указать для доставки?",
  "reply_language": "ru",
  "asset_refs": [],
  "esca…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить сроки доставки. Какой товар вы хотите заказать?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:anthropic/claude-haiku-4.5
- message: А доставка сколько будет стоить?
- reason: unknown token(s), draft would be BLOCKED: {{policy.main.delivery_from}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. Срок доставки {{policy.main.delivery_time}}. Если сумма заказа будет {{policy.main.delivery_from}}, доставка станет бесплатной. Какой адрес указать для доставки?",
  "reply_language": "ru",
  "asset_refs": [],
  "esca…`

**16. follow-up with history, needs delivery cost** | openrouter:google/gemini-2.5-flash
- message: А доставка сколько будет стоить?
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Какой`

**1. price question, Russian** | openrouter:google/gemini-2.5-flash-lite
- message: Сколько стоит кофемашина?
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она сейчас в наличии. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escal`

**1. price question, Russian** | openrouter:google/gemini-2.5-flash
- message: Сколько стоит кофемашина?
- reason: could not parse JSON output
- raw output: ````json
{
  "reply_text": "Кофемашина стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}.",
  "reply_language": "ru`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не осуществляем доставку в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с выбором!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать подробнее о доставке или вас интересует конкретный товар?",
  "reply_language": "ru",
  "…`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием оформим заказ! Что вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": …`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Могу уточнить информацию по вашему адресу?",
  "reply_language": "ru",
  "asset_refs": [
    "delivery-zones…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с выбором!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_rea…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точную стоимость и сроки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [
    …`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точнее для Астаны?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escala…`

**9. off-KB city coverage** | openrouter:openai/gpt-4o-mini
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{"reply_text":"К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с выбором!","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**9. off-KB city coverage** | openrouter:anthropic/claude-haiku-4.5
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием оформим заказ! Что вас интересует?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": …`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите узнать точную стоимость для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalatio…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [
    "deliver…`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Хотите узнать что-то еще или оформить заказ?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной! Какой товар вы хотите заказать?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

**16. follow-up with history, needs delivery cost** | openrouter:openai/gpt-4o-mini
- message: А доставка сколько будет стоить?
- reason: did not use the required fact token(s)
- raw output: `{"reply_text":"Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?","reply_language":"ru","asset_refs":[],"escalate":false,"escalation_reason":"","confidence":1}`

