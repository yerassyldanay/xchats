# Run 2026-07-19_02-50-37-ef6e

Generated 2026-07-19 04:29. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-11) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## escalation-canary-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 3/6 (50%) | [19%, 81%] | 6/6 (100%) | unpriceable (cached, no split to borrow) | 21ms (cached — not meaningful) | 1741 | n/a |
| openrouter:google/gemini-2.5-flash | 3/6 (50%) | [19%, 81%] | 6/6 (100%) | $0.0045 est. (6 measured) | 2009ms | 1625 | 92% |
| openrouter:google/gemini-2.5-flash-lite | 1/6 (17%) | [3%, 56%] | 6/6 (100%) | $0.0012 est. (6 measured) | 1880ms | 1609 | 93% |

## escalation-canary-v2

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 5/6 (83%) | [44%, 97%] | 6/6 (100%) | $0.0031 est. (6 measured) | 4384ms | 1878 | 93% |
| openrouter:google/gemini-2.5-flash | 5/6 (83%) | [44%, 97%] | 6/6 (100%) | $0.0045 est. (6 measured) | 1837ms | 1763 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 2/6 (33%) | [10%, 70%] | 6/6 (100%) | $0.0013 est. (6 measured) | 1754ms | 1780 | 94% |

## lang-canary-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 3/7 (43%) | [16%, 75%] | 7/7 (100%) | unpriceable (cached, no split to borrow) | 9ms (cached — not meaningful) | 1735 | n/a |
| openrouter:google/gemini-2.5-flash | 3/7 (43%) | [16%, 75%] | 7/7 (100%) | $0.0028 est. (4 measured, 3 unpriceable) | 1102ms | 1618 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 3/7 (43%) | [16%, 75%] | 7/7 (100%) | $0.0008 est. (4 measured, 3 unpriceable) | 1221ms | 1612 | 93% |

## lang-canary-v2

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 4/7 (57%) | [25%, 84%] | 7/7 (100%) | $0.0040 est. (7 measured) | 5193ms | 2041 | 94% |
| openrouter:google/gemini-2.5-flash | 4/7 (57%) | [25%, 84%] | 7/7 (100%) | $0.0056 est. (7 measured) | 1856ms | 1821 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 4/7 (57%) | [25%, 84%] | 7/7 (100%) | $0.0015 est. (7 measured) | 1793ms | 1830 | 93% |

## lang-canary-v3

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 5/7 (71%) | [36%, 92%] | 7/7 (100%) | $0.0048 est. (7 measured) | 3947ms | 2460 | 95% |
| openrouter:google/gemini-2.5-flash | 5/7 (71%) | [36%, 92%] | 7/7 (100%) | $0.0061 est. (7 measured) | 1027ms | 2197 | 96% |
| openrouter:google/gemini-2.5-flash-lite | 3/7 (43%) | [16%, 75%] | 7/7 (100%) | $0.0018 est. (7 measured) | 1356ms | 2206 | 95% |

## lang-canary-v4-kk

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 2/3 (67%) | [21%, 94%] | 3/3 (100%) | $0.0021 est. (3 measured) | 5536ms | 2495 | 94% |
| openrouter:google/gemini-2.5-flash | 2/3 (67%) | [21%, 94%] | 3/3 (100%) | $0.0030 est. (3 measured) | 1408ms | 2171 | 93% |
| openrouter:google/gemini-2.5-flash-lite | 1/3 (33%) | [6%, 79%] | 3/3 (100%) | $0.0008 est. (3 measured) | 2074ms | 2158 | 93% |

## lang-canary-v4-ru

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 2/4 (50%) | [15%, 85%] | 4/4 (100%) | unpriceable (cached, no split to borrow) | 19ms (cached — not meaningful) | 1703 | n/a |
| openrouter:google/gemini-2.5-flash | 3/4 (75%) | [30%, 95%] | 4/4 (100%) | unpriceable (cached, no split to borrow) | 19ms (cached — not meaningful) | 1617 | n/a |
| openrouter:google/gemini-2.5-flash-lite | 3/4 (75%) | [30%, 95%] | 4/4 (100%) | unpriceable (cached, no split to borrow) | 28ms (cached — not meaningful) | 1610 | n/a |

## shop-current

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 21/29 (72%) | [54%, 85%] | 29/29 (100%) | $0.0005 est. (1 measured, 28 unpriceable) | 215ms | 1754 | 94% |
| openrouter:google/gemini-2.5-flash | 20/29 (69%) | [51%, 83%] | 29/29 (100%) | $0.0135 est. (19 measured, 10 unpriceable) | 646ms | 1621 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 14/29 (48%) | [31%, 66%] | 27/29 (93%) | $0.0037 est. (19 measured, 10 unpriceable) | 709ms | 1623 | 93% |

## shop-decisions-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 25/29 (86%) | [69%, 95%] | 29/29 (100%) | $0.0125 est. (29 measured) | 4987ms | 1512 | 90% |
| openrouter:google/gemini-2.5-flash | 25/29 (86%) | [69%, 95%] | 29/29 (100%) | $0.0190 est. (29 measured) | 1104ms | 1400 | 92% |
| openrouter:google/gemini-2.5-flash-lite | 17/29 (59%) | [41%, 74%] | 27/29 (93%) | $0.0052 est. (29 measured) | 1050ms | 1416 | 91% |

## shop-history-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 10/12 (83%) | [55%, 95%] | 12/12 (100%) | $0.0061 est. (12 measured) | 5303ms | 1803 | 93% |
| openrouter:google/gemini-2.5-flash | 10/12 (83%) | [55%, 95%] | 12/12 (100%) | $0.0086 est. (12 measured) | 1097ms | 1663 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 8/12 (67%) | [39%, 86%] | 12/12 (100%) | $0.0024 est. (12 measured) | 1127ms | 1670 | 94% |

## shop-scale-10

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 5/6 (83%) | [44%, 97%] | 6/6 (100%) | $0.0049 est. (6 measured) | 5313ms | 2944 | 95% |
| openrouter:google/gemini-2.5-flash | 5/6 (83%) | [44%, 97%] | 6/6 (100%) | $0.0064 est. (6 measured) | 1043ms | 2771 | 96% |
| openrouter:google/gemini-2.5-flash-lite | 4/6 (67%) | [30%, 90%] | 6/6 (100%) | $0.0019 est. (6 measured) | 1235ms | 2787 | 96% |

## shop-scale-20

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 6/7 (86%) | [49%, 97%] | 7/7 (100%) | $0.0084 est. (7 measured) | 4949ms | 4345 | 96% |
| openrouter:google/gemini-2.5-flash | 6/7 (86%) | [49%, 97%] | 7/7 (100%) | $0.0103 est. (7 measured) | 1154ms | 4128 | 97% |
| openrouter:google/gemini-2.5-flash-lite | 5/7 (71%) | [36%, 92%] | 7/7 (100%) | $0.0032 est. (7 measured) | 1273ms | 4148 | 97% |

## shop-scale-30

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 6/8 (75%) | [41%, 93%] | 7/8 (88%) | $0.0126 est. (8 measured) | 5245ms | 5745 | 97% |
| openrouter:google/gemini-2.5-flash | 7/8 (88%) | [53%, 98%] | 8/8 (100%) | $0.0150 est. (8 measured) | 1175ms | 5493 | 98% |
| openrouter:google/gemini-2.5-flash-lite | 4/8 (50%) | [22%, 78%] | 8/8 (100%) | $0.0047 est. (8 measured) | 1494ms | 5520 | 98% |

## xpayment-decisions-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 7/8 (88%) | [53%, 98%] | 8/8 (100%) | $0.0035 est. (8 measured) | 4888ms | 1540 | 92% |
| openrouter:google/gemini-2.5-flash | 6/8 (75%) | [41%, 93%] | 8/8 (100%) | $0.0053 est. (8 measured) | 1063ms | 1409 | 92% |
| openrouter:google/gemini-2.5-flash-lite | 3/8 (38%) | [14%, 69%] | 8/8 (100%) | $0.0014 est. (8 measured) | 1168ms | 1410 | 92% |

## xpayment-history-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 9/10 (90%) | [60%, 98%] | 10/10 (100%) | $0.0045 est. (10 measured) | 4418ms | 1612 | 91% |
| openrouter:google/gemini-2.5-flash | 7/10 (70%) | [40%, 89%] | 10/10 (100%) | $0.0066 est. (10 measured) | 1113ms | 1456 | 93% |
| openrouter:google/gemini-2.5-flash-lite | 9/10 (90%) | [60%, 98%] | 10/10 (100%) | $0.0018 est. (10 measured) | 970ms | 1456 | 93% |

## Scale comparison (shop-scale-N)

Model-behavior pass % and avg total tokens per answer at each catalog size — the
direct answer to "does answer quality hold up as the product list grows" and what
that growth costs in tokens (avg tokens here is the raw API count, always
available regardless of whether this model's cost is priced — unlike the est.
cost column above).

| model | shop-scale-10 (behavior / avg tokens) | shop-scale-20 (behavior / avg tokens) | shop-scale-30 (behavior / avg tokens) |
|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 83% / 2944 | 86% / 4345 | 75% / 5745 |
| openrouter:google/gemini-2.5-flash | 83% / 2771 | 86% / 4128 | 88% / 5493 |
| openrouter:google/gemini-2.5-flash-lite | 67% / 2787 | 71% / 4148 | 50% / 5520 |

## Failures (verbatim)

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если вас интересует конкретный товар, я могу назвать его цену и сроки доставки по Алматы.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalat…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать стоимость доставки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fal…`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Чтобы оформить возврат, пожалуйста, напишите номер вашего заказа и причину возврата. Мы обязательно разберемся!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**18. wrong understanding, asks about a product we don't sell** | openrouter:deepseek/deepseek-v3.2-exp
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! К сожалению, в нашем каталоге сейчас нет iPhone. У нас представлена техника для дома, например, кофемашина DeLonghi за {{product.coffee-machine.price}} и набор посуды за {{product.cookware-set.price}}. Может, вас заинтересует что-то из этого?",
  "reply_language": "r…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! В нашем каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить что-то другое из нашего ассортимента?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
 …`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу ли я помочь вам с выбором чего-то другого?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalat…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин. Наш магазин специализируется на продаже техники и товаров для дома. Могу я помочь вам с выбором новой кофемашины или другого товара?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason…`

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

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! Да, мы доставляем в Астану. Стоимость доставки — {{policy.main.delivery_cost}}. Срок доставки — {{policy.main.delivery_time}}. Если сумма заказа превысит {{policy.main.free_delivery_from}}, доставка будет бесплатной. Какой товар вас интересует?",
  "reply_language": …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точнее для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Могу я узнать ваш адрес, чтобы точнее рассчитать стоимость и сроки?",
  "reply_language": "ru"…`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Чтобы я мог уточнить информацию по вашему заказу и помочь с возвратом средств, пожалуйста, напишите номер заказа или дату его оформления.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**10. refund request** | openrouter:deepseek/deepseek-v3.2-exp
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, я не могу обработать ваш запрос на возврат средств через этот чат. Чтобы уточнить информацию по вашему прошлому заказу и помочь с возвратом, пожалуйста, свяжитесь с нашей поддержкой по телефону {{contact.support.phone}}. Они работают {{contact.support.working_…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! У нас в каталоге есть техника и товары для дома. Например, кофемашина DeLonghi стоит {{product.coffee-machine.price}} и есть в наличии. Также есть набор посуды за {{product.cookware-set.price}}, он поставляется под заказ в течение 3–5 дней. Что-то конкретное …`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi стоит {{product.coffee-machine.price}} и сейчас есть в наличии. Вас интересует покупка?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка по городу и области осуществляется. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка становится бесплатной. Хотите узнать точнее про доставку в Астану?",
  "reply_language": "ru",
  "asset_refs": …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Доставка в Астану не указана в базе знаний.",
  "confidence": 0.8
}
````

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

**2. price question, Kazakh** | openrouter:deepseek/deepseek-v3.2-exp
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите, покажу фото или расскажу про условия доставки?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1…`

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

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать стоимость доставки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fal…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если вас интересует конкретный товар, я могу назвать его цену и сроки доставки по Алматы.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalat…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: `{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите, я покажу несколько фото или расскажу подробнее?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-photo-2"],
  "escalate": false,
  …`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она есть {{product.coffee-machine.availability}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точнее для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Могу я узнать ваш адрес, чтобы точнее рассчитать стоимость и сроки?",
  "reply_language": "ru"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! Да, мы доставляем в Астану. Стоимость доставки — {{policy.main.delivery_cost}}. Срок доставки — {{policy.main.delivery_time}}. Если сумма заказа превысит {{policy.main.free_delivery_from}}, доставка будет бесплатной. Какой товар вас интересует?",
  "reply_language": …`

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
  "confidence": 1.0
}
````

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпарат біздің білім базамызда жоқ. Сізге қалай көмектесе аламын?",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Delivery to Astana is not covered in the k…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Вы можете уточнить стоимость и сроки доставки по конкретному адресу в Алматы, или я могу рассказать о наших товарах.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "es…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — {{product.coffee-machine.price}}.",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi {{product.coffee-machine.price}} тұрады. Ол қазіргі уақытта {{product.coffee-machine.availability}}. Сізді басқа қандай да бір тауарлар қызықтыра ма?",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation_reaso…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы {{product.coffee-machine.price}}. Ол қазір {{product.coffee-machine.availability}}. Қосымша суреттерді көргіңіз келе ме, әлде тапсырысты рәсімдеуге көшейік?",
  "reply_language": "kk",
  "asset_refs": ["coffee-photo-1", "coffe…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы бойынша және облысқа жеткіземіз. Астанаға жеткізу шарттарын нақтылау үшін, сізге {{contact.support.phone}} нөміріне хабарласуыңызды немесе {{contact.support.instagram}} парақшамызға жазуыңызды сұраймыз. Олар сізге барлық ақпаратты бере алад…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} жоғары болса, жеткізу тегін болады.",
  "reply_language": "kk",
  "asset_refs": [],
  "escala…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Иә, біз Астанаға да жеткіземіз. Жеткізу мерзімі {{policy.main.delivery_time}}, ал құны {{policy.main.delivery_cost}}. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} және одан жоғары болса, жеткізу тегін болады. Қандай тауарды тапсырыс бергіңіз келеді?",
  "reply_langu…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она сейчас в наличии.",
  "reply_language": "kk",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-photo-2",
    "coffee-photo-3"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по городу Алматы стоит {{policy.main.delivery_cost}}. Обычно мы доставляем заказы в течение {{policy.main.delivery_time}}. Если сумма вашего заказа превысит {{policy.main.free_delivery_from}}, доставка будет бесплатной. Хотите оформить заказ?",
  "reply_language":…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставляем по городу и области. Стоимость и сроки зависят от адреса. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, мы доставляем только по Алматы и области. В Астану пока доставки нет.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы {{product.coffee-machine.price}} тұрады.",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi құны {{product.coffee-machine.price}}. Қазір {{product.coffee-machine.availability}}. Қосымша суреттерді көргіңіз келе ме?",
  "reply_language": "kk",
  "asset_refs": ["coffee-photo-1", "coffee-photo-2"],
  "escalate": false,
  "escalatio…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. Хотите узнать точнее?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Иә, біз Астанаға жеткіземіз! Жеткізу мерзімі {{policy.main.delivery_time}}, ал құны {{policy.main.delivery_cost}}. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} және одан жоғары болса, жеткізу тегін болады. Қандай тауарды тапсырмақшысыз?",
  "reply_language": "kk",
 …`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы бойынша жеткізу қызметін көрсетеміз. Жеткізу құны {{policy.main.delivery_cost}} болады. Әдетте, тауар 1–3 күн ішінде жетеді. Сізді қызықтыратын тауар бар ма?",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы мен облысына жеткіземіз. Астанаға жеткізу шарттарын нақтылау үшін, сізге жеткізу аймақтары картасын жіберейін. Сізді қандай тауарлар қызықтырады?",
  "reply_language": "kk",
  "asset_refs": [
    "delivery-zones"
  ],
  "escalate": false,
 …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз тек қала мен облыс бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады. Егер үлкен сомаға тапсырыс берсеңіз, жеткізу тегін болады. Тапсырыс беру үшін қандай тауарға қызығушылық танытып отырсыз?",
  "reply_language": "kk",
  "asset_r…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
    "reply_text": "Сәлеметсіз бе! Иә, біз Астанаға да жеткізіп береміз. Жеткізу құны мен мерзімі {{policy.main.delivery_cost}} және {{policy.main.delivery_time}} болады. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} және одан жоғары болса, жеткізу тегін болады. Қандай тауарға тапсыры…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать стоимость доставки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fal…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если вас интересует конкретный товар, я могу назвать его цену и сроки доставки по Алматы.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalat…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она есть {{product.coffee-machine.availability}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: `{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите, я покажу несколько фото или расскажу подробнее?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-photo-2"],
  "escalate": false,
  …`

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

**2. price question, Kazakh** | openrouter:deepseek/deepseek-v3.2-exp
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите, покажу фото или расскажу про условия доставки?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1…`

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

**5. out-of-stock product** | openrouter:google/gemini-2.5-flash-lite
- message: Набор посуды есть в наличии?
- reason: invented digits outside any token: 3, 5
- raw output: ````json
{
  "reply_text": "Здравствуйте! Набор посуды у нас под заказ, срок поставки 3–5 дней. Хотите оформить заказ или узнать цену?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**7. request certificate** | openrouter:google/gemini-2.5-flash-lite
- message: А сертификат качества на кофемашину есть? Пришлите.
- reason: unknown token(s), draft would be BLOCKED: {{coffee-cert}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Да, конечно! Сертификат качества на кофемашину DeLonghi доступен. Вот он: {{coffee-cert}}",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-cert"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**8. delivery zones question** | openrouter:google/gemini-2.5-flash-lite
- message: Куда вы вообще доставляете, есть карта зон?
- reason: unknown token(s), draft would be BLOCKED: {{delivery-zones}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Вот карта зон доставки по Алматы:\n\n{{delivery-zones}}",
  "reply_language": "ru",
  "asset_refs": [
    "delivery-zones"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать стоимость доставки для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fal…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если вас интересует конкретный товар, я могу назвать его цену и сроки доставки по Алматы.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalat…`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Чтобы оформить возврат, пожалуйста, напишите номер вашего заказа и причину возврата. Мы обязательно разберемся!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**12. long rambling message with the price question buried inside** | openrouter:deepseek/deepseek-v3.2-exp
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она в {{product.coffee-machine.availability}}. Уход за ней несложный: достаточно регулярно промывать контейнер для отходов и использовать очищающие таблетки раз в месяц. Хотите,…`

**12. long rambling message with the price question buried inside** | openrouter:google/gemini-2.5-flash-lite
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! Рада, что вы обратились к нам. Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она в наличии, и уход за ней несложный. Хотите узнать больше о ней или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-ph…`

**12. long rambling message with the price question buried inside** | openrouter:google/gemini-2.5-flash
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Уход за ней довольно прост, могу прислать видео-обзор, чтобы вы могли оценить. Хотите посмотреть?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-video"
  ],
 …`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! В нашем каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить что-то другое из нашего ассортимента?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
 …`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу ли я помочь вам с выбором чего-то другого?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalat…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:deepseek/deepseek-v3.2-exp
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! К сожалению, в нашем каталоге сейчас нет iPhone. У нас представлена техника для дома, например, кофемашина DeLonghi за {{product.coffee-machine.price}} и набор посуды за {{product.cookware-set.price}}. Может, вас заинтересует что-то из этого?",
  "reply_language": "r…`

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
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин. Наш магазин специализируется на продаже техники и товаров для дома. Могу я помочь вам с выбором новой кофемашины или другого товара?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она есть {{product.coffee-machine.availability}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: `{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите, я покажу несколько фото или расскажу подробнее?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-photo-2"],
  "escalate": false,
  …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точнее для вашего адреса?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка будет бесплатной. Могу я узнать ваш адрес, чтобы точнее рассчитать стоимость и сроки?",
  "reply_language": "ru"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! Да, мы доставляем в Астану. Стоимость доставки — {{policy.main.delivery_cost}}. Срок доставки — {{policy.main.delivery_time}}. Если сумма заказа превысит {{policy.main.free_delivery_from}}, доставка будет бесплатной. Какой товар вас интересует?",
  "reply_language": …`

**23. topic switch to media: history about the cookware set, then asks for photos** | openrouter:google/gemini-2.5-flash-lite
- message: А как он выглядит? Пришлите фото, пожалуйста.
- reason: invented digits outside any token: 3, 5
- raw output: ````json
{
  "reply_text": "Конечно! Вот фото нашего набора посуды. Он доступен под заказ, доставка займёт 3–5 дней. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "cookware-photo-1",
    "cookware-photo-2"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence":…`

**24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up** | openrouter:google/gemini-2.5-flash-lite
- message: А запасные части для неё у вас продаются?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, запасные части для кофемашины DeLonghi мы не продаем. Могу предложить вам саму кофемашину, она сейчас в наличии!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**25. pronoun resolution across history: availability of the cookware set** | openrouter:google/gemini-2.5-flash-lite
- message: А он сейчас в наличии?
- reason: invented digits outside any token: 3, 5
- raw output: ````json
{
  "reply_text": "Набор посуды сейчас под заказ, срок доставки 3–5 дней. Если хотите оформить заказ, просто напишите адрес доставки, и мы пришлём счёт в WhatsApp.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**28. asks for a media type that does not exist (cookware video)** | openrouter:google/gemini-2.5-flash-lite
- message: Пришлите видео набора посуды, пожалуйста
- reason: invented digits outside any token: 3, 5
- raw output: ````json
{
  "reply_text": "Здравствуйте! Видео набора посуды пока нет, но могу показать фото. У нас есть отличный набор посуды, который сейчас под заказ — доставка займет 3–5 дней. Цена {{product.cookware-set.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "cookware-photo-1",
    "cookwar…`

**29. media request, Kazakh** | openrouter:deepseek/deepseek-v3.2-exp
- message: Кофемашинаның суреттерін жіберіңізші
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Конечно! Вот несколько фотографий кофемашины DeLonghi. Она сейчас {{product.coffee-machine.availability}} и стоит {{product.coffee-machine.price}}. Хотите посмотреть видео-обзор или узнать подробнее о доставке?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1",…`

**29. media request, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашинаның суреттерін жіберіңізші
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Конечно! Вот фото нашей кофемашины DeLonghi. Она сейчас в наличии, а её цена — {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-photo-2",
    "coffee-photo-3"
  ],
  "escalate": false,
  "escalation_reason": …`

**29. media request, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Кофемашинаның суреттерін жіберіңізші
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Конечно, вот несколько фото кофемашины. Если у вас возникнут вопросы, с радостью на них отвечу!",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1",
    "coffee-photo-2",
    "coffee-photo-3"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence…`

**30. follow-up with history, Kazakh** | openrouter:deepseek/deepseek-v3.2-exp
- message: Ал жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. Если сумма заказа превышает {{policy.main.free_delivery_from}}, доставка бесплатная. Срок доставки — {{policy.main.delivery_time}}. Хотите оформить заказ на кофемашину?",
  "reply_language": "ru",
  "asset_refs": [],
  "escal…`

**30. follow-up with history, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Ал жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_time}}.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": ""…`

**30. follow-up with history, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Ал жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. У нас в наличии {{product.coffee-machine.available_pieces}} штук. Хотите оформить заказ?",
  "reply_language": "ru",
  "attach_groups": [
    "product.coffee-machine.images"
  ],
  "escalate": false,…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Стоимость доставки по городу составляет {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Срок доставки — {{policy.main.delivery_in_days}} дня.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate":…`

**8. delivery zones question** | openrouter:google/gemini-2.5-flash-lite
- message: Куда вы вообще доставляете, есть карта зон?
- reason: unknown token(s), draft would be BLOCKED: {{how_to_order}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Стоимость доставки — {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатна. Вот карта зон доставки:\n\n{{how_to_order}}",
  "reply_language": "ru",
  "attach_groups": [
    "topic.delivery.imag…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки составляет {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатна. Уточните, пожалуйста, какой товар вас интересует, чтобы я мог рассчитать точные сроки и стоимо…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, мы доставляем только по городу и области. Для заказов от {{policy.main.free_delivery_from}} доставка по городу бесплатна, в остальных случаях её стоимость составляет {{policy.main.delivery_cost}}. Срок доставки — {{policy.main.delivery_in_days}} …`

**10. refund request** | openrouter:google/gemini-2.5-flash-lite
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Понимаем ваше желание вернуть деньги. Чтобы мы могли разобраться в ситуации, пожалуйста, уточните номер вашего прошлого заказа. После этого мы сможем передать информацию нашим специалистам для дальнейшей обработки.",
  "reply_language": "ru",
  "attach_groups…`

**12. long rambling message with the price question buried inside** | openrouter:google/gemini-2.5-flash
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Это автоматическая кофемашина, она проста в уходе. В наличии на складе в Алматы есть {{product.coffee-machine.available_pieces}} штук. Если хотите, могу прислать фото и видеообзор.",
  "reply_language…`

**12. long rambling message with the price question buried inside** | openrouter:google/gemini-2.5-flash-lite
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она автоматическая и отлично подходит для дома. Уход за ней достаточно простой. У нас в наличии {{product.coffee-machine.available_pieces}} штук. Хотите оформить заказ?",
  "reply_language": "ru",
  …`

**12. long rambling message with the price question buried inside** | openrouter:deepseek/deepseek-v3.2-exp
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Это автоматическая модель для дома, очень удобная в использовании. В уходе она не сложная — есть система автоматической очистки. У нас в наличии {{product.coffee-machine…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить вам автоматическую кофемашину DeLonghi за {{product.coffee-machine.price}}. У нас сейчас в наличии {{product.coffee-machine.available_pieces}} штук.",
  "reply_language": "ru",
  "attach_groups"…`

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

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, автоматическая кофемашина DeLonghi стоит {{product.coffee-machine.price}}. У нас есть {{product.coffee-machine.available_pieces}} штук в наличии на складе в Алматы. Хотите по…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин. Мы продаем автоматические кофемашины DeLonghi. Возможно, вас заинтересует новая кофемашина? У нас есть {{product.coffee-machine.available_pieces}} штук в наличии на складе в Алматы.",
  "reply_language": "ru",
  "attach_gr…`

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. В наличии на складе в Алматы {{product.coffee-machine.available_pieces}} штук. Хотите оформить заказ?",
  "reply_language": "ru",
  "attach_groups": [
    "product.coffee-machine.images"
  ],
  "esc…`

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

**20. mixed Kazakh/Russian message, rule says answer Russian when mixed** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply looks like Kazakh but a Russian reply was expected
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! DeLonghi кофемашинасының бағасы {{product.coffee-machine.price}}. Қазір қоймамызда {{product.coffee-machine.available_pieces}} дана бар. Қосымша фото немесе бейне қажет пе?",
  "reply_language": "kk",
  "attach_groups": ["product.coffee-machine.images"],
  "…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость доставки составляет {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_in_days}} дня. Хотите оформить заказ?",
  "reply…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Сәлеметсіз бе! Иә, біз Астанаға жеткіземіз. Жеткізу құны {{policy.main.delivery_cost}}, ал {{policy.main.free_delivery_from}} және жоғары тапсырыстарға тегін. Жеткізу {{policy.main.delivery_in_days}} күнде жүзеге асырылады. Тапсырыс беру үшін қандай тауар қызықтыратынын айтыңыз.",…`

**23. topic switch to media: history about the cookware set, then asks for photos** | openrouter:google/gemini-2.5-flash-lite
- message: А как он выглядит? Пришлите фото, пожалуйста.
- reason: unknown token(s), draft would be BLOCKED: {{product.cookware-set.images}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Конечно! Набор посуды состоит из 12 предметов. Вот фотографии, чтобы вы могли его рассмотреть: {{product.cookware-set.images}}",
  "reply_language": "ru",
  "attach_groups": [
    "product.cookware-set.images"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confide…`

**24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up** | openrouter:google/gemini-2.5-flash-lite
- message: А запасные части для неё у вас продаются?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, запасные части для кофемашины DeLonghi мы не продаем. Могу предложить вам саму кофемашину, она стоит {{product.coffee-machine.price}} и у нас есть {{product.coffee-machine.available_pieces}} штук в наличии.",
  "reply_language": "ru",
  "attach_groups": [],
  …`

**29. media request, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашинаның суреттерін жіберіңізші
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Конечно! Вот фото нашей автоматической кофемашины DeLonghi. Она стоит {{product.coffee-machine.price}} и сейчас в наличии на складе в Алматы. У нас осталось {{product.coffee-machine.available_pieces}} штук.",
  "reply_language": "ru",
  "attach_groups": [
    "product.coff…`

**30. follow-up with history, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Ал жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по городу и области стоит {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_in_days}} дня.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "es…`

**h4. context flips the verdict: city named two turns earlier makes the tariff inapplicable** | openrouter:google/gemini-2.5-flash-lite
- message: Сколько будет доставка?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставка по городу и области занимает {{policy.main.delivery_time}}. Стоимость доставки по Алматы — {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точную стоимость…`

**h4. context flips the verdict: city named two turns earlier makes the tariff inapplicable** | openrouter:google/gemini-2.5-flash
- message: Сколько будет доставка?
- reason: reply_text contains forbidden phrase: "1 500"
- raw output: ````json
{
  "reply_text": "Доставка по городу и области зависит от адреса. Стандартная стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а при заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Уточните, пожалуйста, ваш точный адрес, чтобы я могла рассчитать…`

**h4. context flips the verdict: city named two turns earlier makes the tariff inapplicable** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сколько будет доставка?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставка в Астану стоит {{policy.main.delivery_cost}}. Срок доставки — {{policy.main.delivery_time}}. Если сумма заказа превысит {{policy.main.free_delivery_from}}, доставка будет бесплатной. Хотите оформить заказ на кофемашину?",
  "reply_language": "ru",
  "asset_refs": …`

**h5. post-escalation follow-up: must stay escalated, no invented resolution** | openrouter:google/gemini-2.5-flash-lite
- message: Ну что, узнали?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! К сожалению, доставка в Астану пока не осуществляется. Мы доставляем заказы по городу и области. Могу ли я помочь вам с выбором товара или оформлением заказа по Алматы?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason":…`

**h9. language switch RU->KK: Russian history, Kazakh final message — reply in Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Рақмет! Оның бағасы қанша?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Хотите узнать про доставку или оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**h9. language switch RU->KK: Russian history, Kazakh final message — reply in Kazakh** | openrouter:google/gemini-2.5-flash
- message: Рақмет! Оның бағасы қанша?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**h9. language switch RU->KK: Russian history, Kazakh final message — reply in Kazakh** | openrouter:deepseek/deepseek-v3.2-exp
- message: Рақмет! Оның бағасы қанша?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Рақмет! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите посмотреть фото или видео-обзор?",
  "reply_language": "kk",
  "asset_refs": ["coffee-photo-1", "coffee-video"],
  "escalate": false,
  "escalation_reaso…`

**h12. channel switch mid-order: asks for the phone number to call instead** | openrouter:google/gemini-2.5-flash-lite
- message: Знаете, дайте лучше номер телефона — я позвоню и всё обговорю голосом.
- reason: invented digits outside any token: 9, 00, 19
- raw output: ````json
{
  "reply_text": "Конечно! Наш номер телефона: {{contact.support.phone}}. Мы работаем с понедельника по субботу, с 9:00 до 19:00. Если у вас возникнут вопросы по кофемашине, звоните!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidenc…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, посудомоечных машин Bosch в нашем ассортименте нет. Могу предложить вам кофемашину DeLonghi за {{product.coffee-machine-delonghi.price}} или набор посуды (12 предметов) за {{product.cookware-set-12.price}}.",
  "reply_language": "ru",
  "attach_g…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечных машин Bosch нет в нашем каталоге. У нас есть техника для дома и кухни: кофемашины, мелкая бытовая техника, посуда. Если вас интересует что-то из этого, с радостью помогу с выбором!",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": …`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:google/gemini-2.5-flash-lite
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:deepseek/deepseek-v3.2-exp
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук. Доставка по городу и области занимает {{policy.main.delivery_in_days}} дня. Если заказ от {{policy.main.free_delivery_from}}, доставка бесплатная. Х…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечной машины Bosch в нашем ассортименте нет. Могу предложить вам другую технику для кухни, например, кофемашину DeLonghi за {{product.coffee-machine-delonghi.price}} или набор посуды из 12 предметов за {{product.cookware-set-12.price}}.",
  "reply_lang…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечной машины Bosch сейчас нет в наличии. В нашем каталоге представлена техника для дома и кухни: кофемашины, мелкая бытовая техника, посуда. Возможно, вас заинтересует что-то из этого? 😊",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": …`

**2. product we don't sell** | openrouter:deepseek/deepseek-v3.2-exp
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! К сожалению, в нашем текущем каталоге посудомоечных машин Bosch нет. У нас представлена техника для дома и кухни: кофемашины, мелкая бытовая техника, посуда. Полный список и цены можно уточнить у менеджера. Могу ли я помочь вам подобрать что-то другое?",
  "reply_lan…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:google/gemini-2.5-flash-lite
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук.",
  "reply_language": "ru",
  "attach_groups": [
    "product.microwave-samsung.images"
  ],
  "escalate": false,
  "escalation_reason": "",…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечной машины Bosch в нашем ассортименте нет. Могу предложить вам другие товары для кухни, например, кофемашину DeLonghi за {{product.coffee-machine-delonghi.price}} или набор посуды на 12 предметов за {{product.cookware-set-12.price}}.",
  "reply_langu…`

**2. product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, посудомоечных машин Bosch сейчас нет в наличии. В нашем каталоге представлена техника для дома и кухни, например, кофемашины, мелкая бытовая техника и посуда. Могу предложить посмотреть что-то из этого.",
  "reply_language": "ru",
  "attach_groups": [],
  "esc…`

**2. product we don't sell** | openrouter:deepseek/deepseek-v3.2-exp
- message: У вас есть посудомоечная машина Bosch?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! К сожалению, в нашем каталоге сейчас нет посудомоечных машин Bosch. У нас представлена техника для дома и кухни: кофемашины, мелкая бытовая техника и посуда. Полный список и цены можно уточнить у менеджера. Если вас интересует что-то конкретное, напишите — помогу с и…`

**3. ambiguous name matches two products** | openrouter:google/gemini-2.5-flash-lite
- message: Сколько стоит чайник?
- reason: attached 3 media entries — over the frame's cap
- raw output: ````json
{
  "reply_text": "Здравствуйте! У нас есть несколько моделей чайников. Например, электрический чайник Tefal стоит {{product.kettle-tefal.price}} и есть в наличии {{product.kettle-tefal.available_pieces}} штук. Также есть электрический чайник Bosch за {{product.kettle-bosch.price}}, в наличи…`

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:google/gemini-2.5-flash-lite
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Микроволновая печь Samsung стоит {{product.microwave-samsung.price}}. В наличии {{product.microwave-samsung.available_pieces}} штук.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**4. deep-list question is NOT this — Kazakh price question, in-pool product** | openrouter:deepseek/deepseek-v3.2-exp
- message: Микротолқынды пеш Samsung қанша тұрады?
- reason: unknown token(s), draft would be BLOCKED: {{product.micro-wave-samsung.price}}, {{product.micro-wave-samsung.available_pieces}}; leftover brace survived injection
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Samsung микроволновая печінің бағасы {{product.micro-wave-samsung.price}}. Қазір қоймамызда {{product.micro-wave-samsung.available_pieces}} дана бар. Қызығушылық танытқаныңыз үшін рахмет! Тапсырыс беру үшін жеткізу мекенжайын жіберіңіз, біз сізге WhatsApp-та…`

**7. deep-list product (#25 of 30), Kazakh price question** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемолка Bosch қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемолка Bosch стоит {{product.coffee-grinder-bosch.price}}. У нас в наличии {{product.coffee-grinder-bosch.available_pieces}} штук.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**14. payment limit, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Бизнес тарифінің төлем лимиті қандай?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Максимальный лимит платежей по тарифу «Бизнес» в месяц составляет {{tariff.business.payment_limit_monthly}}.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**15. off-KB crypto question** | openrouter:google/gemini-2.5-flash-lite
- message: А вы принимаете оплату криптовалютой?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, мы не принимаем оплату криптовалютой. Мы специализируемся на традиционных платежах. Если у вас есть вопросы по нашим тарифам или подключению, буду рад помочь!",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  …`

**15. off-KB crypto question** | openrouter:google/gemini-2.5-flash
- message: А вы принимаете оплату криптовалютой?
- reason: reply_text contains forbidden phrase: "не принимаем оплату криптовалют"
- raw output: ````json
{
  "reply_text": "К сожалению, мы не принимаем оплату криптовалютой. Уточню у коллеги, какие способы оплаты мы поддерживаем, и вернусь к вам с ответом.",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": true,
  "escalation_reason": "Не могу ответить на вопрос о способах оплаты…`

**16. conversation start, bare greeting** | openrouter:google/gemini-2.5-flash-lite
- message: Здравствуйте!
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Здравствуйте! Рады помочь. Подключение быстрое, занимает {{policy.main.connection_days}} рабочий день. Нужен только БИН вашей компании. Подробная инструкция есть в документе.",
  "reply_language": "ru",
  "attach_groups": [
    "topic.connect.documents"
  ],
  "escalate": …`

**17. conversation close, customer is done** | openrouter:google/gemini-2.5-flash
- message: Спасибо, разобрался, буду подключаться!
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Отлично! Для подключения нам понадобится только БИН вашей компании. Весь процесс займёт всего {{policy.main.connection_days}} рабочий день. Подробную инструкцию по подключению вы найдёте в прикреплённом документе. Если возникнут вопросы, обращайтесь!",
  "reply_language": …`

**17. conversation close, customer is done** | openrouter:google/gemini-2.5-flash-lite
- message: Спасибо, разобрался, буду подключаться!
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Отлично! Подключение займёт всего {{policy.main.connection_days}} рабочий день. Вам понадобится только БИН компании. Подробная инструкция уже в приложенном документе.",
  "reply_language": "ru",
  "attach_groups": [
    "topic.connect.documents"
  ],
  "escalate": false,
 …`

**17. conversation close, customer is done** | openrouter:deepseek/deepseek-v3.2-exp
- message: Спасибо, разобрался, буду подключаться!
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Отлично, рады помочь! Подключение проходит быстро, обычно за {{policy.main.connection_days}} рабочий день. Для этого нужен только БИН вашей компании. Подробная инструкция прилагается — там всё по шагам. Удачи в подключении! Если появятся вопросы — пишите.",
  "reply_langua…`

**18. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: А у вас есть аренда NFC-терминалов для оффлайн-магазина?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы предоставляем платёжные решения для онлайн- и офлайн-бизнеса. Для подключения нам потребуется БИН вашей компании. Процесс подключения занимает {{policy.main.connection_days}} рабочий день. Подробная инструкция доступна в документе.",
  "reply_language": "r…`

**xph2. tariff switch mid-thread, pronoun resolves to the LATER tariff** | openrouter:google/gemini-2.5-flash
- message: Кстати, а какой у него лимит платежей в месяц?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Уточните, пожалуйста, для какого тарифа вас интересует лимит платежей? У нас есть три тарифа: «Старт», «Стандарт» и «Бизнес».",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.8
}
````

**xph2. tariff switch mid-thread, pronoun resolves to the LATER tariff** | openrouter:deepseek/deepseek-v3.2-exp
- message: Кстати, а какой у него лимит платежей в месяц?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Для тарифа «Старт» лимит платежей в месяц составляет {{tariff.start.payment_limit_monthly}}. Это отличный стартовый объём для нового бизнеса. Если понадобится увеличить лимит, можно будет перейти на «Стандарт» или «Бизнес».",
  "reply_language": "ru",
  "attach_groups": []…`

**xph5. unsupported software integration, after a realistic tariff thread** | openrouter:google/gemini-2.5-flash-lite
- message: А у вас есть готовая интеграция с 1С или вебхуки для заказов?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Для подключения тарифа «Бизнес» нам понадобится БИН вашей компании. Сам процесс подключения занимает {{policy.main.connection_days}} рабочий день. Подробная инструкция уже в приложенном документе.",
  "reply_language": "ru",
  "attach_groups": [
    "topic.connect.document…`

**xph8. post-escalation follow-up: crypto question stays unresolved** | openrouter:google/gemini-2.5-flash
- message: Ну как, узнали про крипту?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! Извините за ожидание. К сожалению, мы не принимаем оплату криптовалютой. Если у вас есть другие вопросы, буду рад помочь!",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**xph9. language switch RU->KK: Russian tariff history, Kazakh final question** | openrouter:google/gemini-2.5-flash
- message: Рақмет! Ал оның айлық төлем лимиті қандай?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Тариф «Бизнес» имеет месячный лимит платежей {{tariff.business.payment_limit_monthly}}. Если у вас возникнут другие вопросы, с радостью поможем!",
  "reply_language": "ru",
  "attach_groups": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

