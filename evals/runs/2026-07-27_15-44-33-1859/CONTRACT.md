# Contract verdicts (per answer)

## shop-kb-v1-10

### 1. exact price — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 1. exact price — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6190 in / 63 out tokens, est. $0.000644)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, кофемашина DeLonghi есть в наличии. Её стоимость составляет 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi есть в наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6192 in / 87 out tokens, est. $0.000654)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, блендера Philips сейчас нет в наличии. Могу предложить вам другие блендеры: Блендер Bosch, Блендер Xiaomi, Блендер Tefal или Блендер Panasonic.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о наличии блендера Philips — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6189 in / 106 out tokens, est. $0.000661)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### 4. photo request — media-ful product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото Кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 4. photo request — media-ful product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото Кофемашины DeLonghi. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6195 in / 101 out tokens, est. $0.000660)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 5. photo and certificate combined (all_of) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото и сертификат качества для Блендера Panasonic. Если у вас возникнут другие вопросы, обращайтесь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 5. photo and certificate combined (all_of) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото блендера Panasonic и сертификат качества, как вы и просили.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6197 in / 95 out tokens, est. $0.000658)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 6. photo request — media-less product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет отдельного фото Блендера Bosch. Могу предложить вам посмотреть фото Блендера Panasonic или Блендера Tefal?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 6. photo request — media-less product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): products.blender-bosch.featured_image, products.blender-bosch.gallery_images
- requires met: true · media met: false · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото блендера Bosch: [фото]
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6192 in / 89 out tokens, est. $0.000655)
- contract pass: **false** · model-behavior pass (deterministic): **false** · llm judge: not run

### 7. media request for an out-of-stock product — no media at all — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, сейчас у нас нет видео Набора посуды, так как этот товар отсутствует в наличии. Могу предложить вам посмотреть видео других товаров, например, Кофемашины DeLonghi или Блендера Tefal?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 7. media request for an out-of-stock product — no media at all — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о видео для набора посуды — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6192 in / 102 out tokens, est. $0.000660)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### 8. partial-media probe — asks for the one missing kind — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет сертификата качества на блендер Tefal. Могу предложить вам посмотреть сертификат на блендер Panasonic, если интересно.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 8. partial-media probe — asks for the one missing kind — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): products.blender-tefal.certificate_documents
- requires met: true · media met: false · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сертификат качества на блендер Tefal есть. Вот ссылка на документ: products.blender-tefal.certificate_documents
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6200 in / 92 out tokens, est. $0.000657)
- contract pass: **false** · model-behavior pass (deterministic): **false** · llm judge: not run

### 9. delivery cost (general, zones present) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы я мог рассчитать стоимость доставки, пожалуйста, уточните город.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 9. delivery cost (general, zones present) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Подскажите, пожалуйста, в какой город или регион вас интересует доставка? Это поможет мне точнее рассчитать стоимость.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6185 in / 71 out tokens, est. $0.000647)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 10. delivery days (general, zones present) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы я мог уточнить сроки доставки, пожалуйста, назовите город, в который нужно доставить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 10. delivery days (general, zones present) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Срок доставки зависит от вашего города. Пожалуйста, уточните, куда вы хотели бы оформить доставку.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6190 in / 68 out tokens, est. $0.000646)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 11. minimum order amount — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Минимальная сумма заказа составляет 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 11. minimum order amount — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Минимальная сумма заказа составляет 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6190 in / 61 out tokens, est. $0.000643)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 12. working hours and phone — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы работаем Пн–Сб, 9:00–19:00. Связаться с нами можно по номеру +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 12. working hours and phone — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Наши часы работы: Пн–Сб, 9:00–19:00. Связаться с нами можно по телефону: +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6192 in / 76 out tokens, est. $0.000650)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 13. missing exact value — return period escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- **reply deflects to a manager while escalate=false:** "уточнить у менеджер"
- requires met: true · media met: true · escalate met: false · escalate/text consistent: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Возврат и обмен товара возможны, если он не был в использовании и сохранена упаковка. Точные условия возврата вы можете уточнить у менеджера при оформлении заказа.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### 13. missing exact value — return period escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- **reply deflects to a manager while escalate=false:** "Уточняйте условия у менеджер"
- requires met: true · media met: true · escalate met: false · escalate/text consistent: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Возврат и обмен возможны, если товар не был в использовании и сохранена упаковка. Уточняйте условия у менеджера при оформлении.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6189 in / 87 out tokens, est. $0.000654)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### 14. warranty duration escalates (prose-only, no fact column) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о точном сроке гарантии на кофемашину — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 14. warranty duration escalates (prose-only, no fact column) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: На технику действует гарантия производителя. Точный срок гарантии уточняйте у менеджера.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6194 in / 103 out tokens, est. $0.000661)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 15. off-KB city resolves via country zone fallback — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем в Караганду. Срок доставки составит 3–4 дня, а стоимость — 10 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 15. off-KB city resolves via country zone fallback — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Караганду осуществляется. Стоимость доставки составит 10 000 ₸, срок доставки — 3–4 дня. Мы доставляем в этот регион.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6190 in / 107 out tokens, est. $0.000662)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h1. pronoun price follow-up — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Могу также прислать фото или видео, если интересно.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h1. pronoun price follow-up — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6222 in / 63 out tokens, est. $0.000647)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h2. purchase intent resolves discussed product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi есть в наличии. Для оформления заказа, пожалуйста, укажите адрес доставки.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h2. purchase intent resolves discussed product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Чтобы оформить заказ на кофемашину DeLonghi, пожалуйста, напишите ваш адрес доставки. После этого я пришлю вам счёт для оплаты.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6250 in / 80 out tokens, est. $0.000657)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h3. repeat prior literal through placeholder — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h3. repeat prior literal through placeholder — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6230 in / 63 out tokens, est. $0.000648)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h4. pronoun photo request — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h4. pronoun photo request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фотографии нашей кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6219 in / 88 out tokens, est. $0.000657)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h5. out-of-stock purchase intent stays honest — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, Набора посуды сейчас нет в наличии, поэтому оформить его не получится. Могу предложить вам другие товары из нашего каталога.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h5. out-of-stock purchase intent stays honest — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мне очень жаль, но набор посуды, к сожалению, сейчас отсутствует в наличии. Сейчас у меня нет информации о том, когда он появится, — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6223 in / 123 out tokens, est. $0.000672)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d1. listed city — Astana — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану доставляем. Стоимость доставки составит 4 000 ₸, срок — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d1. listed city — Astana — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки в Астану составит 4 000 ₸. Ориентировочный срок доставки — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6191 in / 85 out tokens, est. $0.000653)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d2. listed city, different declension — Astane (dative) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Астане стоит 4 000 ₸. Срок доставки — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d2. listed city, different declension — Astane (dative) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Астане составляет 4 000 ₸. Срок доставки — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6194 in / 80 out tokens, est. $0.000651)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d3. explicit deny zone — Baikonur beats its parent's allow — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: В Байконур не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d3. explicit deny zone — Baikonur beats its parent's allow — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Байконур курьером не осуществляется. не доставляем
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6191 in / 70 out tokens, est. $0.000647)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d4. unlisted country — China refuses via outside_zones_note — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по Казахстану. Доставка в Китай не осуществляется. В другие города, регионы и страны за пределами Казахстана мы не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d4. unlisted country — China refuses via outside_zones_note — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Китай — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6188 in / 162 out tokens, est. $0.000684)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### d5. unplaceable location escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточните, пожалуйста, город, чтобы я мог проверить информацию о доставке.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d5. unplaceable location escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у меня нет информации о доставке в Верхние Васюки. Передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6192 in / 124 out tokens, est. $0.000669)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d6. negotiation for an excluded direction escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Китай — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d6. negotiation for an excluded direction escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Китай — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6203 in / 118 out tokens, est. $0.000668)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d7. refund demand escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возврате денег за прошлые заказы — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d7. refund demand escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возврате средств — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6193 in / 114 out tokens, est. $0.000665)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d8. repair service question escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о ремонте — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d8. repair service question escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о ремонте — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6191 in / 69 out tokens, est. $0.000647)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d9. off-catalog product escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о наличии iPhone 15 Pro — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d9. off-catalog product escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о наличии iPhone 15 Pro — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6190 in / 94 out tokens, est. $0.000657)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d10. history follow-up resolves the zone already discussed — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка кофемашины DeLonghi в Астану займёт 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d10. history follow-up resolves the zone already discussed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi доставляется в Астану в течение 1 дня. Стоимость доставки составит 4 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6235 in / 91 out tokens, est. $0.000660)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk1. exact price — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk1. exact price — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6195 in / 63 out tokens, est. $0.000645)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, DeLonghi кофемашинасы қоймада бар. Бағасы: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Да, Кофемашина DeLonghi есть в наличии. Её цена — 129 900 ₸. Хотите узнать подробнее или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6194 in / 98 out tokens, est. $0.000659)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендері қоймада жоқ. Бірақ бізде басқа блендерлер бар: Блендер Bosch, Блендер Xiaomi, Блендер Tefal, Блендер Panasonic, Блендер Samsung. Олардың біріне назар аударғыңыз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта бізде Philips блендері жоқ. Сізге басқа брендтердің блендерлері ұнай ма? Мысалы, DeLonghi кофемашинасы, Bosch, Xiaomi, Tefal немесе Panasonic блендерлері бар.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6189 in / 131 out tokens, est. $0.000671)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk4. listed city — Astana delivery cost and time — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Астанаға жеткізу құны 4 000 ₸ және тауар 1 күн ішінде жетеді.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk4. listed city — Astana delivery cost and time — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Астанаға жеткізу құны 4 000 ₸ және жеткізу мерзімі 1 күн.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6203 in / 84 out tokens, est. $0.000654)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk5. photo request — media-ful product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Әрине, міне, Кофемашина DeLonghi суреттері. Тағы қандай ақпарат қажет?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk5. photo request — media-ful product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: false · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6196 in / 84 out tokens, est. $0.000653)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk6. photo request — media-less product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Bosch блендерінің суреті жоқ. Сізге басқа блендерлердің суреттерін көрсетейін бе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk6. photo request — media-less product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Bosch блендерінің суретін бере алмаймын, себебі бұл тауардың қоймада жоқ екені анықталды. Сізге басқа брендтердің блендерлерін ұсына аламын ба?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6197 in / 109 out tokens, est. $0.000663)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk7. off-catalog product escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk7. off-catalog product escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6193 in / 123 out tokens, est. $0.000669)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk8. repair service question escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk8. repair service question escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде кофемашиналарды жөндеу туралы ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6195 in / 119 out tokens, est. $0.000667)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka1. shared-alphabet Kazakh — out-of-stock honesty — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендері қоймада жоқ. Бірақ бізде басқа блендерлер бар: Bosch, Xiaomi, Tefal және Panasonic. Олардың сипаттамалары мен бағалары туралы айтып бере аламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka1. shared-alphabet Kazakh — out-of-stock honesty — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6201 in / 115 out tokens, est. $0.000666)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka2. shared-alphabet Kazakh — exact price — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka2. shared-alphabet Kazakh — exact price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6193 in / 63 out tokens, est. $0.000644)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астана қаласына жеткізу құны 4 000 ₸ болады, ал жеткізу мерзімі 1 күн.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Доставка в Астану будет стоить 4 000 ₸. Ориентировочный срок доставки — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6194 in / 87 out tokens, est. $0.000654)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka4. bare short follow-up — history carries the language — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, Кофемашина DeLonghi қазір қоймада бар. Бағасы: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka4. bare short follow-up — history carries the language — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi в наличии! Её цена — 129 900 ₸. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6208 in / 89 out tokens, est. $0.000656)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka5. code-switching within one message — exact price — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka5. code-switching within one message — exact price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6202 in / 71 out tokens, est. $0.000649)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka6. mid-conversation language switch — delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астанаға жеткізу құны 4 000 ₸, жеткізу мерзімі 1 күн. жеткіземіз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka6. mid-conversation language switch — delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану стоит 4 000 ₸, занимает 1 дня. жеткіземіз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (6224 in / 88 out tokens, est. $0.000658)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

## shop-kb-v1-100

### 1. exact price — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 1. exact price — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.

Хотите узнать больше о ней или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10558 in / 93 out tokens, est. $0.001093)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi сейчас в наличии. Её стоимость: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi есть в наличии. Её цена — 129 900 ₸.

Хотите узнать подробнее о доставке или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10560 in / 103 out tokens, est. $0.001097)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Блендера Philips сейчас нет в наличии. Могу предложить вам другие блендеры: Блендер Bosch, Блендер Xiaomi, Блендер Tefal или Блендер Panasonic.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, блендера Philips сейчас нет в наличии. Могу предложить вам рассмотреть другие модели блендеров, например, Bosch, Xiaomi, Tefal или Panasonic. Вас интересуют какие-то конкретные характеристики?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10557 in / 91 out tokens, est. $0.001092)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 4. photo request — media-ful product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 4. photo request — media-ful product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фотографии кофемашины DeLonghi:

[фотографии кофемашины]
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10563 in / 85 out tokens, est. $0.001090)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 5. photo and certificate combined (all_of) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у меня нет фото блендера Panasonic. Могу предложить вам фото блендера Tefal или Panasonic, а также сертификат качества для блендера Panasonic.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 5. photo and certificate combined (all_of) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фотографии блендера Panasonic и сертификат качества, которые вы запрашивали.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10565 in / 96 out tokens, est. $0.001095)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 6. photo request — media-less product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото Блендера Bosch.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 6. photo request — media-less product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): products.blender-bosch.featured_image, products.blender-bosch.gallery_images
- requires met: true · media met: false · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото блендера Bosch. Его цена — 11 200 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10560 in / 97 out tokens, est. $0.001095)
- contract pass: **false** · model-behavior pass (deterministic): **false** · llm judge: not run

### 7. media request for an out-of-stock product — no media at all — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, набор посуды сейчас отсутствует в наличии, поэтому видео к нему нет.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 7. media request for an out-of-stock product — no media at all — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, набора посуды сейчас нет в наличии. Могу предложить вам посмотреть другие товары из нашего каталога?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10560 in / 69 out tokens, est. $0.001084)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 8. partial-media probe — asks for the one missing kind — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет сертификата качества на Блендер Tefal. Могу ли я чем-то ещё помочь?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 8. partial-media probe — asks for the one missing kind — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): products.blender-tefal.certificate_documents
- requires met: true · media met: false · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сертификат качества на блендер Tefal есть. Прилагаю его к этому сообщению.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10568 in / 84 out tokens, est. $0.001090)
- contract pass: **false** · model-behavior pass (deterministic): **false** · llm judge: not run

### 9. delivery cost (general, zones present) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы я мог подсказать стоимость и сроки доставки, пожалуйста, уточните город.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 9. delivery cost (general, zones present) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Чтобы рассчитать стоимость доставки, пожалуйста, укажите ваш город.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10553 in / 60 out tokens, est. $0.001079)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 10. delivery days (general, zones present) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Для того чтобы я мог уточнить сроки доставки, пожалуйста, назовите город, в который нужно доставить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 10. delivery days (general, zones present) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы занимает 1 дня. Стоимость доставки — 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10558 in / 80 out tokens, est. $0.001088)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 11. minimum order amount — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Минимальная сумма заказа в нашем магазине составляет 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 11. minimum order amount — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Минимальная сумма заказа составляет 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10558 in / 61 out tokens, est. $0.001080)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 12. working hours and phone — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы работаем Пн–Сб, 9:00–19:00. Связаться с нами можно по номеру +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 12. working hours and phone — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Наш магазин работает с Пн–Сб, 9:00–19:00. Связаться с нами можно по телефону: +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10560 in / 76 out tokens, est. $0.001086)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 13. missing exact value — return period escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о сроках возврата товара — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 13. missing exact value — return period escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- **reply deflects to a manager while escalate=false:** "Уточняйте условия у менеджер"
- requires met: true · media met: true · escalate met: false · escalate/text consistent: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Возврат и обмен возможны, если товар не был в использовании и сохранена упаковка. Уточняйте условия у менеджера при оформлении.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10557 in / 76 out tokens, est. $0.001086)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### 14. warranty duration escalates (prose-only, no fact column) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о сроке гарантии на кофемашину — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 14. warranty duration escalates (prose-only, no fact column) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- **reply deflects to a manager while escalate=false:** "уточнить у менеджер"
- requires met: true · media met: true · escalate met: false · escalate/text consistent: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Гарантия на технику предоставляется производителем. Точный срок гарантии на кофемашину вы можете уточнить у менеджера.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10562 in / 73 out tokens, est. $0.001085)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### 15. off-KB city resolves via country zone fallback — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем в Караганду. Стоимость доставки составит 10 000 ₸, срок доставки — 3–4 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 15. off-KB city resolves via country zone fallback — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Караганду — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10558 in / 73 out tokens, est. $0.001085)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### h1. pronoun price follow-up — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h1. pronoun price follow-up — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10590 in / 63 out tokens, est. $0.001084)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h2. purchase intent resolves discussed product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi есть в наличии. Для оформления заказа, пожалуйста, укажите адрес доставки. После этого мы пришлём счёт и оформим доставку.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h2. purchase intent resolves discussed product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Чтобы оформить заказ на Кофемашину DeLonghi, пожалуйста, укажите ваш адрес доставки. После этого я пришлю вам счёт и мы оформим доставку.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10618 in / 85 out tokens, est. $0.001096)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h3. repeat prior literal through placeholder — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h3. repeat prior literal through placeholder — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10598 in / 63 out tokens, est. $0.001085)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h4. pronoun photo request — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото Кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h4. pronoun photo request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото кофемашины DeLonghi:
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10587 in / 84 out tokens, est. $0.001092)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h5. out-of-stock purchase intent stays honest — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, Набора посуды сейчас нет в наличии. Возможно, вас заинтересуют другие товары из нашего каталога?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h5. out-of-stock purchase intent stays honest — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, набор посуды сейчас отсутствует в наличии. Могу предложить вам другие товары из нашего каталога, например, кофемашину DeLonghi или один из доступных блендеров.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10591 in / 86 out tokens, est. $0.001094)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d1. listed city — Astana — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану доставляем. Стоимость доставки составит 4 000 ₸, срок — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d1. listed city — Astana — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки в Астану составляет 4 000 ₸. Доставка занимает 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10559 in / 80 out tokens, est. $0.001088)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d2. listed city, different declension — Astane (dative) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Астане доставляем. Стоимость доставки составит 4 000 ₸, срок — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d2. listed city, different declension — Astane (dative) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Астане составляет 4 000 ₸. Срок доставки: 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10562 in / 80 out tokens, est. $0.001088)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d3. explicit deny zone — Baikonur beats its parent's allow — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: В Байконур не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d3. explicit deny zone — Baikonur beats its parent's allow — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Байконур курьером не осуществляется. Вы можете ознакомиться с другими доступными направлениями доставки.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10559 in / 70 out tokens, est. $0.001084)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### d4. unlisted country — China refuses via outside_zones_note — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: В другие города, регионы и страны за пределами Казахстана мы не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d4. unlisted country — China refuses via outside_zones_note — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, доставка в Китай не осуществляется. Мы доставляем по Алматы и области.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10556 in / 64 out tokens, est. $0.001081)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### d5. unplaceable location escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточните, пожалуйста, город доставки, чтобы я мог проверить информацию.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d5. unplaceable location escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Верхние Васюки — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10560 in / 75 out tokens, est. $0.001086)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d6. negotiation for an excluded direction escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Китай и возможности самостоятельной организации курьера. Передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d6. negotiation for an excluded direction escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Китай — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10571 in / 174 out tokens, est. $0.001127)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d7. refund demand escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возврате денег — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d7. refund demand escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возврате средств за прошлые заказы — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10561 in / 108 out tokens, est. $0.001099)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d8. repair service question escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о ремонте — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d8. repair service question escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о ремонте — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10559 in / 96 out tokens, est. $0.001094)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d9. off-catalog product escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации об iPhone 15 Pro — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d9. off-catalog product escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о наличии iPhone 15 Pro — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10558 in / 89 out tokens, est. $0.001091)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d10. history follow-up resolves the zone already discussed — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану занимает 1 дня. Стоимость доставки составит 4 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d10. history follow-up resolves the zone already discussed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану займет 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10603 in / 67 out tokens, est. $0.001087)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk1. exact price — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk1. exact price — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10563 in / 63 out tokens, est. $0.001081)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, DeLonghi кофемашинасы қоймада бар. Бағасы: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi есть в наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10562 in / 87 out tokens, est. $0.001091)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендері қоймада жоқ. Бірақ бізде басқа да блендерлер бар, мысалы, Bosch, Xiaomi, Tefal, Panasonic және Samsung блендерлері. Қайсысы сізді қызықтырады?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Да, у нас есть блендер Philips. К сожалению, сейчас он отсутствует в наличии. Могу предложить вам рассмотреть другие модели блендеров, например, Bosch, Xiaomi, Tefal или Panasonic. Они есть в наличии и готовы к отправке.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10557 in / 100 out tokens, est. $0.001096)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk4. listed city — Astana delivery cost and time — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астанаға жеткізу құны 4 000 ₸, ал жеткізу мерзімі 1 күн.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk4. listed city — Astana delivery cost and time — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астанаға жеткізу құны 4 000 ₸ теңге, жеткізу мерзімі 1 күн. Біздің тауарлар жеткізіледі.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10571 in / 108 out tokens, est. $0.001100)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk5. photo request — media-ful product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Әрине, Кофемашина DeLonghi суреттерін жіберемін. Басқа сұрақтарыңыз болса, хабарласыңыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk5. photo request — media-ful product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фотографии кофемашины DeLonghi:

[Фотографии кофемашины DeLonghi]
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10564 in / 88 out tokens, est. $0.001092)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk6. photo request — media-less product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Блендер Bosch суретін жіберемін. Басқа сұрақтарыңыз болса, хабарласыңыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk6. photo request — media-less product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Сізге Bosch блендерінің суретін жібере аламын. Сізді қызықтыратын модельді нақтылай аласыз ба? Бізде Bosch брендінің бірнеше блендері бар.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10565 in / 102 out tokens, est. $0.001097)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk7. off-catalog product escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk7. off-catalog product escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта бізде iPhone 15 Pro моделі жоқ. Бізде тек тұрмыстық техника сатылады. Сізге басқа қандай да бір тауар бойынша көмек көрсете аламын ба?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10561 in / 102 out tokens, est. $0.001097)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk8. repair service question escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде кофемашиналарды жөндеу туралы ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk8. repair service question escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10563 in / 111 out tokens, est. $0.001101)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka1. shared-alphabet Kazakh — out-of-stock honesty — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендері қоймада жоқ. Бірақ бізде басқа фирмалардың блендерлері бар: Bosch, Xiaomi, Tefal, Panasonic, Samsung. Қайсысы туралы айтып берейін?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka1. shared-alphabet Kazakh — out-of-stock honesty — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендерінің моделі бізде жоқ. Сізге Bosch, Xiaomi, Tefal немесе Panasonic брендтерінің блендерлері ұнай ма? Олардың барлығы дайын және сатылымда.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10569 in / 105 out tokens, est. $0.001099)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka2. shared-alphabet Kazakh — exact price — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: DeLonghi кофемашинасының бағасы 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka2. shared-alphabet Kazakh — exact price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10561 in / 63 out tokens, est. $0.001081)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астана қаласына жеткізу құны 4 000 ₸ құрайды, ал жеткізу мерзімі 1 күн. Жеткізу жеткіземіз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Доставка в Астану будет стоить 4 000 ₸. Ориентировочный срок доставки: 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10562 in / 87 out tokens, est. $0.001091)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka4. bare short follow-up — history carries the language — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, Кофемашина DeLonghi қазір қоймада бар. Бағасы: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka4. bare short follow-up — history carries the language — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi есть в наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10576 in / 71 out tokens, est. $0.001086)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka5. code-switching within one message — exact price — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ құрайды.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka5. code-switching within one message — exact price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10570 in / 81 out tokens, est. $0.001089)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka6. mid-conversation language switch — delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астанаға жеткізу құны 4 000 ₸ құрайды, жеткізу мерзімі 1 күн. Жеткізу жеткіземіз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka6. mid-conversation language switch — delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану стоит 4 000 ₸. Срок доставки составляет 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (10592 in / 80 out tokens, est. $0.001091)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

## shop-kb-v1-50

### 1. exact price — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 1. exact price — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8091 in / 63 out tokens, est. $0.000834)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi есть в наличии. Её цена 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi в наличии. Её цена — 129 900 ₸. Хотите узнать больше о ней или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8093 in / 96 out tokens, est. $0.000848)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, Блендера Philips сейчас нет в наличии. Могу предложить вам другие блендеры: Блендер Bosch, Блендер Xiaomi, Блендер Tefal, Блендер Panasonic или Блендер Samsung.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Блендер Philips сейчас отсутствует в наличии. Можем предложить вам другие модели блендеров, например, Bosch, Xiaomi, Tefal или Panasonic.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8090 in / 76 out tokens, est. $0.000839)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 4. photo request — media-ful product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 4. photo request — media-ful product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото Кофемашины DeLonghi, которая есть в наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8096 in / 103 out tokens, est. $0.000851)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 5. photo and certificate combined (all_of) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото блендера Panasonic и сертификат качества.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 5. photo and certificate combined (all_of) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото блендера Panasonic и сертификат качества, которые вы просили.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8098 in / 97 out tokens, est. $0.000849)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 6. photo request — media-less product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото Блендера Bosch.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 6. photo request — media-less product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): products.blender-bosch.featured_image, products.blender-bosch.gallery_images
- requires met: true · media met: false · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото блендера Bosch. Если хотите узнать цену или оформить заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8093 in / 96 out tokens, est. $0.000848)
- contract pass: **false** · model-behavior pass (deterministic): **false** · llm judge: not run

### 7. media request for an out-of-stock product — no media at all — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, видео набора посуды сейчас нет в наличии. Могу предложить вам другие товары из нашего каталога.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 7. media request for an out-of-stock product — no media at all — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, набора посуды сейчас нет в наличии. Могу предложить вам другие товары из нашего каталога, например, кофемашину DeLonghi или один из доступных блендеров.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8093 in / 87 out tokens, est. $0.000844)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 8. partial-media probe — asks for the one missing kind — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет сертификата качества на Блендер Tefal. Могу предложить вам посмотреть сертификат на Блендер Panasonic, если вас интересует.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 8. partial-media probe — asks for the one missing kind — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сертификат качества на блендер Tefal отсутствует в наличии. Могу предложить вам сертификат на блендер Panasonic, если это интересно.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8101 in / 75 out tokens, est. $0.000840)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 9. delivery cost (general, zones present) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы я мог рассчитать стоимость и сроки доставки, пожалуйста, уточните город, в который нужно доставить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 9. delivery cost (general, zones present) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Чтобы рассчитать стоимость доставки, пожалуйста, укажите ваш город.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8086 in / 60 out tokens, est. $0.000833)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 10. delivery days (general, zones present) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы я мог уточнить сроки доставки, пожалуйста, назовите город, в который нужно доставить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 10. delivery days (general, zones present) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы я мог рассчитать срок доставки, пожалуйста, уточните ваш город.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8091 in / 60 out tokens, est. $0.000833)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 11. minimum order amount — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Минимальная сумма заказа составляет 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 11. minimum order amount — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Минимальная сумма заказа составляет 5 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8091 in / 61 out tokens, est. $0.000834)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 12. working hours and phone — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы работаем Пн–Сб, 9:00–19:00. Связаться с нами можно по номеру +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 12. working hours and phone — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Наш магазин работает ежедневно с Пн–Сб, 9:00–19:00. Связаться с нами можно по телефону: +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8093 in / 77 out tokens, est. $0.000840)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 13. missing exact value — return period escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о сроках возврата товара — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 13. missing exact value — return period escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о сроках возврата товара — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8090 in / 101 out tokens, est. $0.000849)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 14. warranty duration escalates (prose-only, no fact column) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: На технику действует гарантия производителя. Точный срок гарантии на Кофемашину DeLonghi уточняйте у менеджера. Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 14. warranty duration escalates (prose-only, no fact column) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: На технику действует гарантия производителя. Точный срок гарантии на кофемашину уточняйте у менеджера.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8095 in / 114 out tokens, est. $0.000855)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 15. off-KB city resolves via country zone fallback — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Караганду доставляем. Стоимость доставки составит 10 000 ₸, срок — 3–4 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### 15. off-KB city resolves via country zone fallback — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Караганду — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8091 in / 73 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### h1. pronoun price follow-up — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h1. pronoun price follow-up — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8123 in / 63 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h2. purchase intent resolves discussed product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi в наличии. Для оформления заказа мне понадобится ваш адрес доставки. После этого я пришлю вам счёт.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h2. purchase intent resolves discussed product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Чтобы оформить заказ на Кофемашину DeLonghi, просто подтвердите ваш адрес доставки. После этого я пришлю вам счёт и мы оформим доставку.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8151 in / 84 out tokens, est. $0.000849)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h3. repeat prior literal through placeholder — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h3. repeat prior literal through placeholder — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8131 in / 63 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h4. pronoun photo request — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото Кофемашины DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h4. pronoun photo request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото Кофемашины DeLonghi:
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8120 in / 85 out tokens, est. $0.000846)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h5. out-of-stock purchase intent stays honest — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, Набора посуды сейчас нет в наличии, поэтому оформить заказ на него невозможно. Могу предложить вам что-то из товаров, которые есть в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### h5. out-of-stock purchase intent stays honest — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, набор посуды сейчас отсутствует в наличии. Мы можем предложить вам другие товары из нашего каталога, например, кофемашину DeLonghi или один из доступных блендеров. Вас интересуют какие-либо из них?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8124 in / 95 out tokens, est. $0.000850)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d1. listed city — Astana — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану доставляем. Стоимость доставки составит 4 000 ₸, срок — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d1. listed city — Astana — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки в Астану составит 4 000 ₸. Ориентировочный срок доставки — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8092 in / 85 out tokens, est. $0.000843)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d2. listed city, different declension — Astane (dative) — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Астане доставляем. Стоимость доставки составит 4 000 ₸, срок — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d2. listed city, different declension — Astane (dative) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Астане составляет 4 000 ₸. Срок доставки — 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8095 in / 80 out tokens, est. $0.000842)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d3. explicit deny zone — Baikonur beats its parent's allow — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: В Байконур не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d3. explicit deny zone — Baikonur beats its parent's allow — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Байконур не осуществляется.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8092 in / 55 out tokens, est. $0.000831)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### d4. unlisted country — China refuses via outside_zones_note — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по Казахстану. В другие города, регионы и страны за пределами Казахстана мы не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d4. unlisted country — China refuses via outside_zones_note — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: false · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у меня нет информации о доставке в Китай. Передаю ваш вопрос менеджеру, и он вернется с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8089 in / 76 out tokens, est. $0.000839)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### d5. unplaceable location escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточните, пожалуйста, город, чтобы я мог проверить информацию о доставке.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d5. unplaceable location escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Верхние Васюки — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8093 in / 75 out tokens, est. $0.000839)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d6. negotiation for an excluded direction escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возможности доставки товара в Китай, даже если вы наймёте курьера. Передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d6. negotiation for an excluded direction escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о доставке в Китай — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8104 in / 123 out tokens, est. $0.000860)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d7. refund demand escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возврате денег за прошлый заказ — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d7. refund demand escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о возврате средств за прошлые заказы — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8094 in / 108 out tokens, est. $0.000853)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d8. repair service question escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о ремонте — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d8. repair service question escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о ремонте кофемашин — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8092 in / 100 out tokens, est. $0.000849)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d9. off-catalog product escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о наличии iPhone 15 Pro — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d9. off-catalog product escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сейчас у меня нет информации о наличии iPhone 15 Pro — передаю ваш вопрос менеджеру и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8091 in / 89 out tokens, est. $0.000845)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d10. history follow-up resolves the zone already discussed — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка кофемашины DeLonghi в Астану займет 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### d10. history follow-up resolves the zone already discussed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi будет доставлена в Астану в течение 1 дня. Стоимость доставки составит 4 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8136 in / 102 out tokens, est. $0.000854)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk1. exact price — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi бағасы 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk1. exact price — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Хотите узнать больше о ней или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8096 in / 89 out tokens, est. $0.000845)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, Кофемашина DeLonghi қоймада бар. Бағасы: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk2. in-stock availability — coffee machine — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, Кофемашина DeLonghi есть в наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8095 in / 87 out tokens, est. $0.000844)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендері қоймада жоқ. Басқа блендерлерді қарастырғыңыз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk3. out-of-stock honesty — no facts or media for an unavailable product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, блендера Philips сейчас нет в наличии. Могу предложить вам рассмотреть другие модели блендеров, например, Bosch, Xiaomi, Tefal или Panasonic. Вас интересуют какие-то конкретные характеристики?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8090 in / 93 out tokens, est. $0.000846)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk4. listed city — Astana delivery cost and time — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Астана қаласына жеткізу құны 4 000 ₸ құрайды және 1 күн ішінде жетеді. жеткіземіз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk4. listed city — Astana delivery cost and time — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану стоит 4 000 ₸ и занимает 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8104 in / 78 out tokens, est. $0.000842)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk5. photo request — media-ful product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Әрине, Кофемашина DeLonghi кофемашинасының суреттерін жіберемін.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk5. photo request — media-ful product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: false · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8097 in / 84 out tokens, est. $0.000843)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk6. photo request — media-less product — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Блендер Bosch суретін жіберемін. Тағы не көмектесе аламын?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk6. photo request — media-less product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Сізге Bosch блендерінің суретін жібере аламын. Сізді қызықтыратын модельді нақтылай аласыз ба? Бізде Bosch брендінен бірнеше блендер бар.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8098 in / 101 out tokens, est. $0.000850)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk7. off-catalog product escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk7. off-catalog product escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта бізде iPhone 15 Pro моделі жоқ. Бізде бар тауарлар тізімін мына жерден көре аласыз: [ссылка на каталог]. Сізге басқа қандай да бір тауарлар бойынша ақпарат керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8094 in / 110 out tokens, est. $0.000853)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kk8. repair service question escalates — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kk8. repair service question escalates — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жолдаймын және нақты жауаппен ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8096 in / 111 out tokens, est. $0.000854)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka1. shared-alphabet Kazakh — out-of-stock honesty — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендері қоймада жоқ. Бірақ бізде басқа фирмалардың блендерлері бар: Bosch, Xiaomi, Tefal, Panasonic, Samsung. Олардың бағасы 11 200 ₸, 19 500 ₸, 23 600 ₸, 31 900 ₸, 36 000 ₸ тұрады. Қайсысы сізді қызықтырады?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka1. shared-alphabet Kazakh — out-of-stock honesty — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Қазіргі уақытта Philips блендерінің қоры жоқ. Сізге Bosch, Xiaomi, Tefal немесе Panasonic брендтерінің блендерлері ұнай ма? Олардың бағалары мен сипаттамаларын айтып бере аламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8102 in / 108 out tokens, est. $0.000853)
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka2. shared-alphabet Kazakh — exact price — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka2. shared-alphabet Kazakh — exact price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8094 in / 63 out tokens, est. $0.000835)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астанаға жеткізу құны 4 000 ₸ құрайды, ал жеткізу мерзімі 1 күн.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka3. mixed RU greeting + shared-alphabet KK question clause — delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Доставка в Астану стоит 4 000 ₸ и занимает 1 дня. Мы доставляем в Астану.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8095 in / 89 out tokens, est. $0.000845)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka4. bare short follow-up — history carries the language — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, Кофемашина DeLonghi қазір қоймада бар. Бағасы: 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka4. bare short follow-up — history carries the language — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi в наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8109 in / 68 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka5. code-switching within one message — exact price — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ құрайды.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka5. code-switching within one message — exact price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Хотите узнать подробнее или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8103 in / 90 out tokens, est. $0.000846)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

### kka6. mid-conversation language switch — delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Астанаға жеткізу құны 4 000 ₸ құрайды, ал жеткізу мерзімі 1 күн. жеткіземіз
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_unpriceable
- contract pass: **true** · model-behavior pass (deterministic): **true** · llm judge: not run

### kka6. mid-conversation language switch — delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- final answer extraction: outer_fence
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · escalate/text consistent: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка в Астану стоит 4 000 ₸. Срок доставки составит 1 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (8125 in / 81 out tokens, est. $0.000845)
- contract pass: **true** · model-behavior pass (deterministic): **false** · llm judge: not run

