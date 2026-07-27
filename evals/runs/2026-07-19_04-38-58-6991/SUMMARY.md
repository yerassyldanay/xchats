# Run 2026-07-19_04-38-58-6991

Generated 2026-07-19 04:41. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-11) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## combo-canary-v1-kk

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 4/4 (100%) | [51%, 100%] | 4/4 (100%) | $0.0034 est. (4 measured) | 7222ms | 3079 | 94% |
| openrouter:google/gemini-2.5-flash | 4/4 (100%) | [51%, 100%] | 4/4 (100%) | $0.0046 est. (4 measured) | 1305ms | 2633 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 4/4 (100%) | [51%, 100%] | 4/4 (100%) | $0.0013 est. (4 measured) | 1376ms | 2642 | 94% |

## combo-canary-v1-ru

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 6/6 (100%) | [61%, 100%] | 6/6 (100%) | $0.0035 est. (6 measured) | 3630ms | 2128 | 95% |
| openrouter:google/gemini-2.5-flash | 6/6 (100%) | [61%, 100%] | 6/6 (100%) | $0.0048 est. (6 measured) | 925ms | 1961 | 95% |
| openrouter:google/gemini-2.5-flash-lite | 6/6 (100%) | [61%, 100%] | 6/6 (100%) | $0.0014 est. (6 measured) | 1079ms | 1971 | 95% |

## escalation-canary-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 2/6 (33%) | [10%, 70%] | 6/6 (100%) | $0.0031 est. (6 measured) | 4856ms | 1822 | 92% |
| openrouter:google/gemini-2.5-flash | 3/6 (50%) | [19%, 81%] | 6/6 (100%) | $0.0043 est. (6 measured) | 1020ms | 1634 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 1/6 (17%) | [3%, 56%] | 6/6 (100%) | $0.0012 est. (6 measured) | 1154ms | 1636 | 93% |

## escalation-canary-v2

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 6/6 (100%) | [61%, 100%] | 6/6 (100%) | $0.0032 est. (6 measured) | 4170ms | 1896 | 94% |
| openrouter:google/gemini-2.5-flash | 5/6 (83%) | [44%, 97%] | 6/6 (100%) | $0.0045 est. (6 measured) | 874ms | 1789 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 3/6 (50%) | [19%, 81%] | 6/6 (100%) | $0.0013 est. (6 measured) | 744ms | 1799 | 94% |

## lang-canary-v1

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 4/7 (57%) | [25%, 84%] | 7/7 (100%) | $0.0035 est. (7 measured) | 3903ms | 1773 | 93% |
| openrouter:google/gemini-2.5-flash | 2/7 (29%) | [8%, 64%] | 7/7 (100%) | $0.0049 est. (7 measured) | 834ms | 1628 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 2/7 (29%) | [8%, 64%] | 7/7 (100%) | $0.0014 est. (7 measured) | 957ms | 1641 | 93% |

## lang-canary-v2

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 4/7 (57%) | [25%, 84%] | 7/7 (100%) | $0.0040 est. (7 measured) | 5541ms | 2036 | 94% |
| openrouter:google/gemini-2.5-flash | 5/7 (71%) | [36%, 92%] | 7/7 (100%) | $0.0055 est. (7 measured) | 894ms | 1859 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 5/7 (71%) | [36%, 92%] | 7/7 (100%) | $0.0016 est. (7 measured) | 1184ms | 1879 | 93% |

## lang-canary-v3

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 6/7 (86%) | [49%, 97%] | 7/7 (100%) | $0.0051 est. (7 measured) | 4271ms | 2603 | 94% |
| openrouter:google/gemini-2.5-flash | 7/7 (100%) | [65%, 100%] | 7/7 (100%) | $0.0066 est. (7 measured) | 1048ms | 2320 | 95% |
| openrouter:google/gemini-2.5-flash-lite | 3/7 (43%) | [16%, 75%] | 7/7 (100%) | $0.0018 est. (7 measured) | 837ms | 2307 | 96% |

## lang-canary-v4-kk

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 3/4 (75%) | [30%, 95%] | 4/4 (100%) | $0.0027 est. (4 measured) | 6350ms | 2458 | 94% |
| openrouter:google/gemini-2.5-flash | 3/4 (75%) | [30%, 95%] | 4/4 (100%) | $0.0041 est. (4 measured) | 1468ms | 2188 | 92% |
| openrouter:google/gemini-2.5-flash-lite | 2/4 (50%) | [15%, 85%] | 4/4 (100%) | $0.0010 est. (4 measured) | 1437ms | 2162 | 93% |

## lang-canary-v4-ru

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:deepseek/deepseek-v3.2-exp | 2/3 (67%) | [21%, 94%] | 3/3 (100%) | $0.0015 est. (3 measured) | 4962ms | 1734 | 93% |
| openrouter:google/gemini-2.5-flash | 2/3 (67%) | [21%, 94%] | 3/3 (100%) | $0.0021 est. (3 measured) | 953ms | 1623 | 94% |
| openrouter:google/gemini-2.5-flash-lite | 2/3 (67%) | [21%, 94%] | 3/3 (100%) | $0.0006 est. (3 measured) | 1131ms | 1635 | 94% |

## Failures (verbatim)

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

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, мы доставляем только по городу Алматы и области. Если вас интересует доставка в пределах Алматы, срок доставки составляет {{policy.main.delivery_time}}, а стоимость — {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесп…`

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

**10. refund request** | openrouter:deepseek/deepseek-v3.2-exp
- message: Верните мне, пожалуйста, деньги за прошлый заказ
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Для возврата средств, пожалуйста, свяжитесь с нашей поддержкой по телефону {{contact.support.phone}} в часы работы {{contact.support.working_hours}}. Они помогут вам с оформлением возврата.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason":…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro у нас нет. Могу я помочь вам с выбором чего-то другого?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "esca…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить вам кофемашину DeLonghi за {{product.coffee-machine.price}} или набор посуды за {{product.cookware-set.price}}. Вас что-то заинтересовало?",
  "reply_language": "ru",
  "asset_refs": [],
  "esca…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:deepseek/deepseek-v3.2-exp
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Здравствуйте! В нашем каталоге представлены товары для дома и техника, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro у нас нет. Подскажите, может, вас интересует что-то из нашего ассортимента?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
 …`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы интернет-магазин и занимаемся продажей техники и товаров для дома. Ремонтом, к сожалению, не занимаемся. Могу помочь вам с выбором новой кофемашины или другого товара?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reaso…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новую модель DeLonghi. Она сейчас в наличии, а цена {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reaso…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, мы доставляем только по городу Алматы и области. Могу подсказать по товарам или как оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите узнать точную стоимость для Астаны?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "es…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кешіріңіз, біз тек Алматы қаласы мен облысына жеткіземіз. Егер сіз Алматыда болсаңыз, жеткізу {{policy.main.delivery_time}} аралығында жүзеге асырылады және оның құны {{policy.main.delivery_cost}}. Алматыдағы жеткізу аймақтарын көрсететін картаны жібере алам…`

**18. wrong understanding, asks about a product we don't sell** | openrouter:google/gemini-2.5-flash-lite
- message: У вас есть iPhone 15 Pro?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! В нашем каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro в списке нет. Могу я помочь вам с выбором чего-то другого из нашего ассортимента?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": fa…`

**19. wrong understanding, asks about a service we don't offer** | openrouter:google/gemini-2.5-flash-lite
- message: Вы кофемашины ремонтируете?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас стоит {{product.coffee-machine.price}} и есть в наличии. Хотите узнать подробнее?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1…`

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

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Уточняю информацию по доставке в Астану. Скоро вернусь с точным ответом.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "Customer is asking about delivery to Astana, which is not specified in the knowledge base. The…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
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
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}. Хотите, покажу фото или расскажу, как оформить заказ?",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1"],
  "escalate": false,
  "escalation_r…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка по Алматы стоит {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "conf…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а срок доставки — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите оформить заказ или узнать больше о товарах?",
  "reply_l…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу помочь с чем-то ещё?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от {{policy.main.free_delivery_from}} доставка по Алматы бесплатная. Хотите узнать точнее про доставку в ваш город?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate"…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, доставка в Астану пока не осуществляется. Мы доставляем только по городу Алматы и области. Если у вас есть вопросы по другим товарам или доставке в Алматы — с радостью помогу!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_r…`

**20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh)** | openrouter:google/gemini-2.5-flash
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply_language field is "ru", expected "kk"
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она {{product.coffee-machine.availability}}.",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh)** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?
- reason: reply_language field is "ru", expected "kk"
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Она сейчас в наличии. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [
    "coffee-photo-1"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1.0
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, мы доставляем только по городу Алматы и области. Могу ли я чем-то еще помочь?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Хотите узнать точнее про доставку в Астану?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлем! Иә, біз Астанаға жеткіземіз. Жеткізу мерзімі {{policy.main.delivery_time}}, ал құны {{policy.main.delivery_cost}}. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} асса, жеткізу тегін болады. Қандай тауарды тапсырғыңыз келеді?",
  "reply_language": "kk",
…`

**2. price question, Kazakh** | openrouter:deepseek/deepseek-v3.2-exp
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `{
  "reply_text": "Кофемашина DeLonghi {{product.coffee-machine.price}}. Ол {{product.coffee-machine.availability}}. Қосымша фото немесе видео-обзор керек пе?",
  "reply_language": "kk",
  "asset_refs": ["coffee-photo-1", "coffee-photo-2", "coffee-video"],
  "escalate": false,
  "escalation_reason":…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу ли я чем-то ещё помочь?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "К сожалению, доставка осуществляется только по городу и области. Стоимость доставки по Алматы составляет {{policy.main.delivery_cost}}, а при заказе от {{policy.main.free_delivery_from}} доставка будет бесплатной. Срок доставки — {{policy.main.delivery_time}}.",
  "reply_l…`

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: reply_text contains forbidden phrase: "доставку только по"
- raw output: ````json
{
  "reply_text": "К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и его области. Для уточнения точных условий по вашему адресу, пожалуйста, свяжитесь с нашей поддержкой по телефону {{contact.support.phone}} в часы работы {{contact.support.working_hours}}.",
  …`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты, ал үлкен сомаға тапсырыс берген кезде жеткізу тегін болады.",
  "reply_language": "kk",
  "asset_refs": [
    "delivery-zones"
  ],
  "escalate": false,
  "esca…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпаратты нақтылау үшін біздің қолдау қызметіне хабарласыңыз: {{contact.support.phone}}.",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Иә, біз Астанаға жеткіземіз. Доставканың құны {{policy.main.delivery_cost}}, ал жеткізу мерзімі {{policy.main.delivery_time}}. Егер сіздің тапсырысыңыздың құны {{policy.main.free_delivery_from}} жетсе, доставка тегін болады. Қандай тауарды тапсырыс бергіңіз келеді?",
  "reply_lang…`

**2. price question, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "kk",
  "asset_refs": [],
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
  "reply_text": "Доставка по городу Алматы стоит {{policy.main.delivery_cost}}. Обычно доставка занимает {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатная. Хотите оформить заказ?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate":…`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash-lite
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. Хотите узнать подробнее про доставку в конкретный город?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}
````

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Доставка по городу и области осуществляется. Срок и стоимость зависят от адреса. При заказе на сумму от {{policy.main.free_delivery_from}} доставка бесплатная.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Иә, біз Астанаға жеткіземіз. Алматы қаласындағы жеткізу {{policy.main.delivery_time}} аралығында жүреді және құны {{policy.main.delivery_cost}}. Егер сіздің тапсырысыңыз {{policy.main.free_delivery_from}} жетсе, жеткізу тегін болады. Тапсырыс беру үшін қандай тауарды таңдағаныңызд…`

**3. delivery cost + time, Kazakh** | openrouter:google/gemini-2.5-flash-lite
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Жеткізу құны {{policy.main.delivery_cost}} және әдетте 1-3 күн ішінде жетеді. Алматы қаласы бойынша 20 000 ₸-ден жоғары тапсырыстарға жеткізу тегін. Сізді қызықтыратын тауар бар ма?",
  "reply_language": "kk"…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз тек Алматы қаласы мен облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады, ал белгілі бір сомадан асқан тапсырыс үшін жеткізу тегін болады. Алматы бойынша жеткізу құны {{policy.main.delivery_cost}} құрайды, ал {{policy.main.f…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Сәлеметсіз бе! Біз Алматы қаласы мен облысына жеткіземіз. Астанаға жеткізу шарттарын нақтылау үшін, біздің жеткізу аймақтары картасын қарап көруіңізге болады. Сізге қандай тауарлар қызықтырады?",
  "reply_language": "kk",
  "asset_refs": [
    "delivery-zones"
  ],
  "esca…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:deepseek/deepseek-v3.2-exp
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: `{
  "reply_text": "Сәлеметсіз бе! Иә, біз Астанаға да жеткіземіз. Жеткізу құны мен мерзімі мекенжайыңызға байланысты болады. Егер тапсырыс сомасы {{policy.main.free_delivery_from}} жетсе, жеткізу тегін болады. Сізге қандай тауар қызықтырады? Оны айтып берсеңіз, мен бағасы мен қолжетімділігін айтып, …`

**9. off-KB city coverage** | openrouter:google/gemini-2.5-flash
- message: А вы доставляете в Астану?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу помочь с чем-то еще?",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

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

**9. off-KB city coverage** | openrouter:deepseek/deepseek-v3.2-exp
- message: А вы доставляете в Астану?
- reason: reply_text contains forbidden phrase: "доставку только по"
- raw output: `{
  "reply_text": "Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если у вас есть вопросы по товарам или доставке в пределах Алматы, буду рад помочь!",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": true,
  "escalation_reason": "C…`

