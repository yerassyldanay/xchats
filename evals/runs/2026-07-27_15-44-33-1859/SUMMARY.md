# Run 2026-07-27_15-44-33-1859

Generated 2026-07-27 15:46. One table per scenario; a scenario's own README/PLAYGROUND.md
explains what "model-behavior" vs "contract" pass rate means.
Four result types are reported separately and never averaged together:
final-JSON parse/extraction, operational contract, deterministic code-based
behavior checks (the "model-behavior" column — computed by harness code, not by
an LLM), and the optional LLM-as-judge dimension (its own line per scenario;
"not run" when judge-llm never executed).

**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's hand-maintained prices (source: https://openrouter.ai/models, checked 2026-07-27) × token counts from the API response. promptfoo has no pricing table of its own for generic `openrouter:` provider IDs — this report fills that gap, at the accuracy of whoever last updated models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat run, same question) reports zero prompt/completion tokens from promptfoo, so its cost is estimated by BORROWING the split from a fresh row in the same run for the same (model, test) if one exists — otherwise it's marked unpriceable, not free. Every model/cost cell below says which of these applied.

## shop-kb-v1-10

| model | parse | model-behavior pass (deterministic) | 95% CI (Wilson, pooled) | contract pass (final) | contract pass (first shot) | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|---|---|
| openrouter:google/gemini-2.5-flash | 44/44 (100%) | 43/44 (98%) | [88%, 100%] | 44/44 (100%) | 44/44 (100%) | unpriceable (cached, no split to borrow) | 10ms (cached — not meaningful) | 6282 | n/a |
| openrouter:google/gemini-2.5-flash-lite | 44/44 (100%) | 30/44 (68%) | [53%, 80%] | 42/44 (95%) | 42/44 (95%) | $0.0289 est. (44 measured) | 2088ms | 6291 | 99% |

LLM-as-judge (judge-llm): not run. All pass rates above are deterministic code-based checks.

LLM stock check (judge-llm): not run.

## shop-kb-v1-100

| model | parse | model-behavior pass (deterministic) | 95% CI (Wilson, pooled) | contract pass (final) | contract pass (first shot) | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|---|---|
| openrouter:google/gemini-2.5-flash | 44/44 (100%) | 44/44 (100%) | [92%, 100%] | 44/44 (100%) | 44/44 (100%) | unpriceable (cached, no split to borrow) | 10ms (cached — not meaningful) | 10650 | n/a |
| openrouter:google/gemini-2.5-flash-lite | 44/44 (100%) | 27/44 (61%) | [47%, 74%] | 42/44 (95%) | 42/44 (95%) | $0.0480 est. (44 measured) | 2360ms | 10652 | 99% |

LLM-as-judge (judge-llm): not run. All pass rates above are deterministic code-based checks.

LLM stock check (judge-llm): not run.

## shop-kb-v1-50

| model | parse | model-behavior pass (deterministic) | 95% CI (Wilson, pooled) | contract pass (final) | contract pass (first shot) | est. cost | avg latency | avg tokens | prompt share |
|---|---|---|---|---|---|---|---|---|---|
| openrouter:google/gemini-2.5-flash | 44/44 (100%) | 43/44 (98%) | [88%, 100%] | 44/44 (100%) | 44/44 (100%) | unpriceable (cached, no split to borrow) | 10ms (cached — not meaningful) | 8183 | n/a |
| openrouter:google/gemini-2.5-flash-lite | 44/44 (100%) | 29/44 (66%) | [51%, 78%] | 43/44 (98%) | 43/44 (98%) | $0.0372 est. (44 measured) | 2089ms | 8186 | 99% |

LLM-as-judge (judge-llm): not run. All pass rates above are deterministic code-based checks.

LLM stock check (judge-llm): not run.

## Scale comparison

Model-behavior pass % and avg total tokens per answer at each catalog size — the
direct answer to "does answer quality hold up as the product list grows" and what
that growth costs in tokens (avg tokens here is the raw API count, always
available regardless of whether this model's cost is priced — unlike the est.
cost column above).

| model | shop-kb-v1-10 (behavior / avg tokens) | shop-kb-v1-50 (behavior / avg tokens) | shop-kb-v1-100 (behavior / avg tokens) |
|---|---|---|---|
| openrouter:google/gemini-2.5-flash | 98% / 6282 | 98% / 8183 | 100% / 10650 |
| openrouter:google/gemini-2.5-flash-lite | 68% / 6291 | 66% / 8186 | 61% / 10652 |

## Failures (verbatim)

**3. out-of-stock honesty — no facts or media for an unavailable product** | openrouter:google/gemini-2.5-flash-lite
- message: Блендер Philips есть в наличии?
- reason: escalate did not match expectation
- raw output: ````json
{
  "escalate": true,
  "escalation_reason": "Product Philips blender is not in stock and not in out of stock list. Escalate as per rule 1d.",
  "media_files_to_send": [],
  "reply_language": "ru",
  "reply_text": "Сейчас у меня нет информации о наличии блендера Philips — передаю ваш вопрос …`

**6. photo request — media-less product** | openrouter:google/gemini-2.5-flash-lite
- message: Пришлите фото блендера Bosch, пожалуйста.
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Конечно! Вот фото блендера Bosch: [фото]",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.blender-bosch.featured_image",
    "products.blender-bosch.gallery_images"
  ],
  "escalate": false
}
````

**7. media request for an out-of-stock product — no media at all** | openrouter:google/gemini-2.5-flash-lite
- message: Пришлите видео набора посуды, пожалуйста.
- reason: escalate did not match expectation
- raw output: ````json
{
  "escalate": true,
  "escalation_reason": "Клиент запросил видео для товара, которого нет в наличии (Набор посуды).",
  "media_files_to_send": [],
  "reply_language": "ru",
  "reply_text": "Сейчас у меня нет информации о видео для набора посуды — передаю ваш вопрос менеджеру и вернусь с т…`

**8. partial-media probe — asks for the one missing kind** | openrouter:google/gemini-2.5-flash-lite
- message: А сертификат качества на блендер Tefal есть? Пришлите, если есть.
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Сертификат качества на блендер Tefal есть. Вот ссылка на документ: products.blender-tefal.certificate_documents",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.blender-tefal.certificate_documents"
  ],
  "escalate": false
}
````

**13. missing exact value — return period escalates** | openrouter:google/gemini-2.5-flash
- message: Какой у вас срок возврата товара?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Возврат и обмен товара возможны, если он не был в использовании и сохранена упаковка. Точные условия возврата вы можете уточнить у менеджера при оформлении заказа.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**13. missing exact value — return period escalates** | openrouter:google/gemini-2.5-flash-lite
- message: Какой у вас срок возврата товара?
- reason: escalate did not match expectation
- raw output: ````json
{
  "escalate": false,
  "escalation_reason": null,
  "media_files_to_send": [],
  "reply_language": "ru",
  "reply_text": "Возврат и обмен возможны, если товар не был в использовании и сохранена упаковка. Уточняйте условия у менеджера при оформлении."
}
````

**d4. unlisted country — China refuses via outside_zones_note** | openrouter:google/gemini-2.5-flash-lite
- message: А в Китай доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Сейчас у меня нет информации о доставке в Китай — передаю ваш вопрос менеджеру и вернусь с точным ответом.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,
  "escalation_reason": "Клиент спрашивает про доставку в Китай, а в блоке ЗОНЫ ДОСТАВКИ н…`

**kk1. exact price — coffee machine** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kk2. in-stock availability — coffee machine** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қоймада бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Да, Кофемашина DeLonghi есть в наличии. Её цена — {{product.coffee-machine.price}}. Хотите узнать подробнее или оформить заказ?",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**kk3. out-of-stock honesty — no facts or media for an unavailable product** | openrouter:google/gemini-2.5-flash-lite
- message: Philips блендері бар ма?
- reason: escalate did not match expectation
- raw output: ````json
{
  "escalate": true,
  "escalation_reason": "Product not found in stock or out of stock lists.",
  "media_files_to_send": [],
  "reply_language": "kk",
  "reply_text": "Қазіргі уақытта бізде Philips блендері жоқ. Сізге басқа брендтердің блендерлері ұнай ма? Мысалы, DeLonghi кофемашинасы, Bo…`

**kk5. photo request — media-ful product** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашинаның суреттерін жіберіңізші
- reason: did not attach the expected media
- raw output: ````json
{
  "reply_text": "Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": true
}
````

**kka1. shared-alphabet Kazakh — out-of-stock honesty** | openrouter:google/gemini-2.5-flash-lite
- message: Сиздерде Филипс блендери бар ма? Билейин деген едим.
- reason: escalate did not match expectation
- raw output: ````json
{
  "escalate": true,
  "escalation_reason": "Клиент спрашивает о товаре, которого нет в каталоге (PRODUCTS OUT OF STOCK).",
  "media_files_to_send": [],
  "reply_language": "kk",
  "reply_text": "Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын."
}…`

**kka2. shared-alphabet Kazakh — exact price** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi канша турады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost** | openrouter:google/gemini-2.5-flash-lite
- message: Здравствуйте! Астанага жеткизу канша турады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка в Астану будет стоить {{delivery.astana.delivery_cost}}. Ориентировочный срок доставки — {{delivery.astana.delivery_in_days}} дня.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka4. bare short follow-up — history carries the language** | openrouter:google/gemini-2.5-flash-lite
- message: Бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi в наличии! Её цена — {{product.coffee-machine.price}}. Хотите оформить заказ?",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**6. photo request — media-less product** | openrouter:google/gemini-2.5-flash-lite
- message: Пришлите фото блендера Bosch, пожалуйста.
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Вот фото блендера Bosch. Его цена — {{product.blender-bosch.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.blender-bosch.featured_image",
    "products.blender-bosch.gallery_images"
  ],
  "escalate": false
}
````

**8. partial-media probe — asks for the one missing kind** | openrouter:google/gemini-2.5-flash-lite
- message: А сертификат качества на блендер Tefal есть? Пришлите, если есть.
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Сертификат качества на блендер Tefal есть. Прилагаю его к этому сообщению.",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.blender-tefal.certificate_documents"
  ],
  "escalate": false
}
````

**13. missing exact value — return period escalates** | openrouter:google/gemini-2.5-flash-lite
- message: Какой у вас срок возврата товара?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Возврат и обмен возможны, если товар не был в использовании и сохранена упаковка. Уточняйте условия у менеджера при оформлении.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**14. warranty duration escalates (prose-only, no fact column)** | openrouter:google/gemini-2.5-flash-lite
- message: На сколько месяцев у вас гарантия на кофемашину?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Гарантия на технику предоставляется производителем. Точный срок гарантии на кофемашину вы можете уточнить у менеджера.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**15. off-KB city resolves via country zone fallback** | openrouter:google/gemini-2.5-flash-lite
- message: А в Караганду доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Сейчас у меня нет информации о доставке в Караганду — передаю ваш вопрос менеджеру и вернусь с точным ответом.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true
}
````

**d3. explicit deny zone — Baikonur beats its parent's allow** | openrouter:google/gemini-2.5-flash-lite
- message: А в Байконур доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Доставка в Байконур курьером не осуществляется. Вы можете ознакомиться с другими доступными направлениями доставки.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**d4. unlisted country — China refuses via outside_zones_note** | openrouter:google/gemini-2.5-flash-lite
- message: А в Китай доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "К сожалению, доставка в Китай не осуществляется. Мы доставляем по Алматы и области.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kk1. exact price — coffee machine** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kk2. in-stock availability — coffee machine** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қоймада бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Да, Кофемашина DeLonghi есть в наличии. Её цена — {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**kk3. out-of-stock honesty — no facts or media for an unavailable product** | openrouter:google/gemini-2.5-flash-lite
- message: Philips блендері бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Да, у нас есть блендер Philips. К сожалению, сейчас он отсутствует в наличии. Могу предложить вам рассмотреть другие модели блендеров, например, Bosch, Xiaomi, Tefal или Panasonic. Они есть в наличии и готовы к отправке.",
  "reply_language": "ru",
  "media_f…`

**kk5. photo request — media-ful product** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашинаның суреттерін жіберіңізші
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Конечно! Вот фотографии кофемашины DeLonghi:\n\n[Фотографии кофемашины DeLonghi]",
  "reply_language": "kk",
  "media_files_to_send": [
    "products.coffee-machine.gallery_images"
  ],
  "escalate": false
}
````

**kk7. off-catalog product escalates** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздерде iPhone 15 Pro бар ма?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Қазіргі уақытта бізде iPhone 15 Pro моделі жоқ. Бізде тек тұрмыстық техника сатылады. Сізге басқа қандай да бір тауар бойынша көмек көрсете аламын ба?",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka2. shared-alphabet Kazakh — exact price** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi канша турады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost** | openrouter:google/gemini-2.5-flash-lite
- message: Здравствуйте! Астанага жеткизу канша турады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка в Астану будет стоить {{delivery.astana.delivery_cost}}. Ориентировочный срок доставки: {{delivery.astana.delivery_in_days}} дня.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka4. bare short follow-up — history carries the language** | openrouter:google/gemini-2.5-flash-lite
- message: Бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Да, Кофемашина DeLonghi есть в наличии. Её цена — {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka5. code-switching within one message — exact price** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Подскажите бағасын кофемашины DeLonghi
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**kka6. mid-conversation language switch — delivery cost** | openrouter:google/gemini-2.5-flash-lite
- message: Астанаға жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка в Астану стоит {{delivery.astana.delivery_cost}}. Срок доставки составляет {{delivery.astana.delivery_in_days}} дня.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**6. photo request — media-less product** | openrouter:google/gemini-2.5-flash-lite
- message: Пришлите фото блендера Bosch, пожалуйста.
- reason: attached media, but this test forbids any attachment
- raw output: ````json
{
  "reply_text": "Вот фото блендера Bosch. Если хотите узнать цену или оформить заказ, дайте знать!",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.blender-bosch.featured_image",
    "products.blender-bosch.gallery_images"
  ],
  "escalate": false
}
````

**15. off-KB city resolves via country zone fallback** | openrouter:google/gemini-2.5-flash-lite
- message: А в Караганду доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Сейчас у меня нет информации о доставке в Караганду — передаю ваш вопрос менеджеру и вернусь с точным ответом.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true
}
````

**d3. explicit deny zone — Baikonur beats its parent's allow** | openrouter:google/gemini-2.5-flash-lite
- message: А в Байконур доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "Доставка в Байконур не осуществляется.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**d4. unlisted country — China refuses via outside_zones_note** | openrouter:google/gemini-2.5-flash-lite
- message: А в Китай доставляете?
- reason: did not use the required fact token(s)
- raw output: ````json
{
  "reply_text": "К сожалению, у меня нет информации о доставке в Китай. Передаю ваш вопрос менеджеру, и он вернется с точным ответом.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true
}
````

**kk1. exact price — coffee machine** | openrouter:google/gemini-2.5-flash
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi бағасы {{product.coffee-machine.price}}.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kk1. exact price — coffee machine** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Хотите узнать больше о ней или оформить заказ?",
  "reply_language": "kk",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**kk2. in-stock availability — coffee machine** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi қоймада бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Да, Кофемашина DeLonghi есть в наличии. Её цена — {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**kk3. out-of-stock honesty — no facts or media for an unavailable product** | openrouter:google/gemini-2.5-flash-lite
- message: Philips блендері бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! К сожалению, блендера Philips сейчас нет в наличии. Могу предложить вам рассмотреть другие модели блендеров, например, Bosch, Xiaomi, Tefal или Panasonic. Вас интересуют какие-то конкретные характеристики?",
  "reply_language": "ru",
  "media_files_to_send": …`

**kk4. listed city — Astana delivery cost and time** | openrouter:google/gemini-2.5-flash-lite
- message: Астанаға жеткізу қанша тұрады және қанша күнде жетеді?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка в Астану стоит {{delivery.astana.delivery_cost}} и занимает {{delivery.astana.delivery_in_days}} дня.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kk5. photo request — media-ful product** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашинаның суреттерін жіберіңізші
- reason: did not attach the expected media
- raw output: ````json
{
  "reply_text": "Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": true
}
````

**kk7. off-catalog product escalates** | openrouter:google/gemini-2.5-flash-lite
- message: Сіздерде iPhone 15 Pro бар ма?
- reason: escalate did not match expectation
- raw output: ````json
{
  "reply_text": "Қазіргі уақытта бізде iPhone 15 Pro моделі жоқ. Бізде бар тауарлар тізімін мына жерден көре аласыз: [ссылка на каталог]. Сізге басқа қандай да бір тауарлар бойынша ақпарат керек пе?",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka2. shared-alphabet Kazakh — exact price** | openrouter:google/gemini-2.5-flash-lite
- message: Кофемашина DeLonghi канша турады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost** | openrouter:google/gemini-2.5-flash-lite
- message: Здравствуйте! Астанага жеткизу канша турады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Доставка в Астану стоит {{delivery.astana.delivery_cost}} и занимает {{delivery.astana.delivery_in_days}} дня. Мы доставляем в Астану.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka4. bare short follow-up — history carries the language** | openrouter:google/gemini-2.5-flash-lite
- message: Бар ма?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Кофемашина DeLonghi в наличии. Её цена — {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}
````

**kka5. code-switching within one message — exact price** | openrouter:google/gemini-2.5-flash-lite
- message: Сәлеметсіз бе! Подскажите бағасын кофемашины DeLonghi
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Здравствуйте! Кофемашина DeLonghi стоит {{product.coffee-machine.price}}. Хотите узнать подробнее или оформить заказ?",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false
}
````

**kka6. mid-conversation language switch — delivery cost** | openrouter:google/gemini-2.5-flash-lite
- message: Астанаға жеткізу қанша тұрады?
- reason: reply does not look like Kazakh (too few Kazakh-specific letters)
- raw output: ````json
{
  "reply_text": "Доставка в Астану стоит {{delivery.astana.delivery_cost}}. Срок доставки составит {{delivery.astana.delivery_in_days}} дня.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false
}
````

