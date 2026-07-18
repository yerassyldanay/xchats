# Run 2026-07-19_02-44-27-436a

Generated 2026-07-19 02:44. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-11) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## shop-current

| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|
| openrouter:minimax/minimax-m2.5 | 23/29 (79%) | [62%, 90%] | 28/29 (97%) | $0.0211 est. (29 measured) | 9812ms | 2197 | 76% |
| openrouter:moonshotai/kimi-k2.5 | 25/29 (86%) | [69%, 95%] | 29/29 (100%) | $0.1239 est. (29 measured) | 53701ms | 3607 | 51% |

Retries (retry.go — see each row's `attempts` in .judged.json for the full history):

- openrouter:moonshotai/kimi-k2.5: retried 2, recovered 2 — first-attempt JSON parse success 27/29 (93%)

## Failures (verbatim)

**2. price question, Kazakh** | openrouter:moonshotai/kimi-k2.5
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi — {{product.coffee-machine.price}}. Хотите оформить заказ? Просто напишите адрес доставки, и мы всё рассчитаем.",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1", "coffee-video"],
  "escalate": false,
  "escalation_reason": "",
  "confidence…`

**3. delivery cost + time, Kazakh** | openrouter:minimax/minimax-m2.5
- message: Жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: `

```json
{
  "reply_text": "Стоимость доставки — {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_time}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатно.",
  "reply_language": "kk",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": …`

**4. stock quantity question, Russian** | openrouter:moonshotai/kimi-k2.5
- message: А сколько штук кофемашины есть в наличии?
- reason: did not use the required fact token(s)
- raw output: ` ```json
{
"reply_text": "Уточняю точное количество кофемашин на складе у менеджера. Отвечу вам в течение нескольких минут.",
"reply_language": "ru",
"asset_refs": [],
"escalate": true,
"escalation_reason": "В списке FACTS отсутствует точное количество единиц товара «Кофемашина DeLonghi», указан тол…`

**5. out-of-stock product** | openrouter:minimax/minimax-m2.5
- message: Набор посуды есть в наличии?
- reason: invented digits outside any token: 3, 5
- raw output: ````json
{
  "reply_text": "Набор посуды сейчас под заказ, срок поставки 3–5 дней. Если хотите, могу оформить заказ — просто напишите, какой товар и адрес доставки.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**12. long rambling message with the price question buried inside** | openrouter:minimax/minimax-m2.5
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: reply_text contains control character U+0008
- raw output: ````json
{
  "reply_text": "Добрый день! Рады, что написали. 😊 Кофемашина DeLonghi стоит {{product.coffee-machine.price}}, и она в наличии — можем оформить прямо сейчас.\br\nУход за ней несложный: достаточно раз в день промыть съёмные части водой и раз в несколько недель прогнать чистящее средство — …`

**12. long rambling message with the price question buried inside** | openrouter:moonshotai/kimi-k2.5
- message: Добрый день! Простите, что долго не писала, были дела, и вот наконец решила написать вам, потому что подруга посоветовала ваш магазин, а я как раз давно хотела купить нормальную кофемашину домой, чтобы не бегать в кофейню каждое утро, и вот увидела у вас DeLonghi — кстати, сколько она стоит? — и ещё интересно, насколько она сложная в уходе.

- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Добрый день! Рады приветствовать вас 😊 Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. А вот про уход — сейчас уточню детали у специалиста и отвечу.",
  "reply_language": "ru",
  "asset_refs": ["coffee-photo-1"],
  "escalate": true,
  "escalation_reason": "Отсу…`

**21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim** | openrouter:minimax/minimax-m2.5
- message: Сіздер Астанаға жеткізесіздер ме?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Извините, уточните, пожалуйста, какой город и район доставки вас интересует? Мы посмотрим возможность и назовём точную стоимость.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5
}
````

**25. pronoun resolution across history: availability of the cookware set** | openrouter:minimax/minimax-m2.5
- message: А он сейчас в наличии?
- reason: invented digits outside any token: 3, 5
- raw output: ````json
{
  "reply_text": "Набор посуды сейчас {{product.cookware-set.availability}} — доставим в течение 3–5 дней после оформления. Если хотите, могу забронировать его для вас. Для заказа просто укажите адрес доставки, и мы вышлем счёт.",
  "reply_language": "ru",
  "asset_refs": [],
  "escalate": …`

**29. media request, Kazakh** | openrouter:minimax/minimax-m2.5
- message: Кофемашинаның суреттерін жіберіңізші
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Конечно! Вот фотографии кофемашины DeLonghi:",
  "reply_language": "kk",
  "asset_refs": ["coffee-photo-1", "coffee-photo-2", "coffee-photo-3"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**30. follow-up with history, Kazakh** | openrouter:minimax/minimax-m2.5
- message: Ал жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Стоимость доставки по городу — {{policy.main.delivery_cost}}. При заказе от {{policy.main.free_delivery_from}} доставка бесплатна.",
  "reply_language": "ru",
  "asset_refs": ["delivery-zones"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 1
}
````

**30. follow-up with history, Kazakh** | openrouter:moonshotai/kimi-k2.5
- message: Ал жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Жеткізу {{policy.main.delivery_cost}}. Заказ бойынша {{policy.main.free_delivery_from}} жеткізу тегін. Мерзімі — {{policy.main.delivery_time}}.\n\nКофемашинаны тапсырыс бересіз бе?",
  "reply_language": "kk",
  "asset_refs": ["delivery-zones"],
  "escalate": false,
  "esca…`

