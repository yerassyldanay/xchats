# Contract verdicts (per answer)

## shop-current

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Хотите узнать о ней подробнее или посмотреть фото?
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она в наличии и готова к отправке. Хотите узнать больше или оформить заказ?
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- unit/currency issues: duplicated day unit
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: false
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает от 1–3 дня дней. Если сумма заказа превышает 20 000 ₸, доставка будет бесплатной. Есть ли что-то конкретное, что вы хотите заказать?
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок — 1–3 дня. Если сумма заказа от 20 000 ₸, доставка бесплатная. Какой товар вас интересует?
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная.
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi в наличии: В наличии.
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi В наличии. Точное количество штук я не вижу, но могу уточнить для вас — напишите, и я проверю на складе. Интересует эта модель?
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина В наличии.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды под заказ, срок доставки составляет 3–5 дней. Если вас это устраивает, дайте знать, и я помогу оформить заказ!
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Цена 24 900 ₸. Если интересует — могу помочь оформить заказ или ответить на вопросы про доставку и условия.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды Под заказ, 3–5 дней.
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Вот несколько фотографий кофемашины DeLonghi:
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — спереди, сбоку и с открытым лотком. Если хотите увидеть её в действии, могу отправить видео-обзор. Интересует эта модель?
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно, вот несколько фото кофемашины DeLonghi. Если у вас есть вопросы по ней, с радостью отвечу!
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, у нас есть сертификат качества на кофемашину DeLonghi. Вы можете скачать его по следующей ссылке: [Сертификат качества на кофемашину DeLonghi (PDF)](coffee-cert). Если у вас есть другие вопросы или хотите оформить заказ, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, конечно! У кофемашины DeLonghi есть сертификат качества. Прикрепляю документ — можете посмотреть все подробности. Если остались вопросы по характеристикам или хотите заказать — пишите!
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, конечно! Вот сертификат качества на кофемашину DeLonghi.
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по городу и области. Вот карта зон доставки по Алматы:
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по Алматы и области! Стоимость доставки 1 500 ₸ по Алматы, срок 1–3 дня. При заказе от 20 000 ₸ доставка бесплатна. Вот карта зон — посмотрите, ваш адрес в зоне доставки?
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от 20 000 ₸ доставка становится бесплатной. Вот карта зон доставки по Алматы:
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует доставка по Алматы или есть вопросы по товарам, дайте знать!
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть знакомые в Алматы, можно отправить туда, или напишите нам в Instagram @demoshop.kz — может быть, найдём решение.
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка становится бесплатной.
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, я не могу помочь с возвратом средств. Для этого вам нужно обратиться в нашу службу поддержки. Вы можете позвонить по номеру +7 727 300 00 00 или написать в Instagram @demoshop.kz. Если у вас есть другие вопросы, с радостью помогу!
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! Я помогу разобраться с возвратом. К сожалению, я не вижу детали вашего прошлого заказа в чате. Напишите, пожалуйста, номер заказа или дату покупки — и я передам вашу просьбу в команду поддержки. Они свяжутся с вами в течение рабочего дня.
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я могу помочь вам с выбором товара и оформлением заказа. По вопросам возврата средств, пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00.
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Чтобы оформить заказ на кофемашину DeLonghi, выполните следующие шаги: 1) Подтвердите, что хотите купить кофемашину; 2) Укажите адрес доставки; 3) Мы пришлём счёт и оформим доставку прямо в WhatsApp. Кофемашина стоит 129 900 ₸ и она в наличии. Жду вашего ответа!
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Отлично! Кофемашина DeLonghi В наличии и стоит 129 900 ₸.

Для оформления заказа нужно:
1) Подтвердить, что берёте кофемашину
2) Указать адрес доставки в Алматы
3) Мы пришлём счёт и оформим доставку

Доставка 1–3 дня, стоимость 1 500 ₸ по Алматы (бесплатно от 20 000 ₸).

Какой адрес указать?
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Отличный выбор! Чтобы оформить заказ на кофемашину DeLonghi, пожалуйста, укажите адрес доставки. Мы пришлём счёт и оформим доставку прямо в WhatsApp.
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Кофемашина DeLonghi стоит 129 900 ₸. Она довольно проста в уходе: достаточно регулярно очищать резервуар для воды и следить за состоянием фильтров. Если хотите, могу помочь с оформлением заказа!
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! 😊 Спасибо, что обратились! Кофемашина DeLonghi стоит 129 900 ₸ и В наличии.

Что касается ухода — это хороший вопрос, но детали по обслуживанию лучше уточнить со специалистом. Могу отправить вам видео-обзор машины, там видно, как она устроена. Интересует?
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Она очень проста в уходе, легко чистится и не требует специальных навыков. Могу прислать вам видео-обзор, чтобы вы могли убедиться в этом сами.
- contract pass: **true** · model-behavior pass: **true**

## shop-decisions-v1

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Это автоматическая кофемашина для дома, в наличии на складе в Алматы. Если хотите оформить заказ, напишите нам адрес доставки.
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю вам счёт в WhatsApp!
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Это автоматическая машина для дома, сейчас в наличии 5 штук на складе в Алматы.

Хотите узнать больше или оформить заказ?
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она есть в наличии на складе в Алматы.
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. У нас в наличии 5 штук на складе в Алматы. Интересует? Напишите, и мы оформим заказ! 😊
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. У нас в наличии 5 штук. Если вас интересует, напишите, пожалуйста, адрес доставки, и мы подтвердим заказ. Счёт пришлём в WhatsApp.
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу стоит 1 500 ₸ и занимает 1–3 күнде. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Если у вас есть дополнительные вопросы, дайте знать!
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны 1 500 ₸ құрайды. Егер тапсырыс сомасы 20 000 ₸ жоғары болса, жеткізу тегін болады. Тапсырысыңыз 1–3 күнде жетеді.
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка стоит 1 500 ₸, но она бесплатна при заказе от 20 000 ₸. Привезём за 1–3 дня. Какой товар вас интересует?
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: В наличии есть 5 штук кофемашины DeLonghi. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я подготовлю счёт!
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: В наличии на складе в Алматы 5 штук кофемашины DeLonghi. Хотите узнать подробнее или оформить заказ?
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi есть в наличии в количестве 5 штук. Могу прикрепить фото, если хотите посмотреть.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, набор посуды сейчас отсутствует на складе. Мы можем привезти его под заказ за 1–3 дня. Если вас это устраивает, напишите, пожалуйста, адрес доставки, и я оформлю заказ!
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды сейчас под заказ, привозим за 1–3 дня. Его стоимость — 24 900 ₸.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, набор посуды сейчас закончился на складе — это товар под заказ. Привозим его за несколько дней. Хотите, я зарезервирую для вас? Или интересует кофемашина DeLonghi — она есть в наличии прямо сейчас? 😊
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Вот фотографии кофемашины DeLonghi. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю счёт в WhatsApp!
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi. Это автоматическая кофемашина для дома, в наличии на складе в Алматы. Если будут вопросы, с радостью отвечу!
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi. Это автоматическая машина для дома, сейчас 5 штук в наличии. Стоит 129 900 ₸. Интересует?
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, у нас есть сертификат качества на кофемашину DeLonghi. Я прикреплю его к этому сообщению. Если у вас есть дополнительные вопросы или вы хотите сделать заказ, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Да, конечно, у нас есть сертификат качества на кофемашину DeLonghi. Прикрепляю его к сообщению.
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, конечно! 📋 На кофемашину DeLonghi есть сертификат качества — прикрепляю. Это подтверждает, что товар прошёл все проверки.

Если хотите узнать больше о машине — могу отправить фото и видео-обзор. Интересует?
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по городу и области. У нас есть карта зон доставки, которую я прикреплю. Если у вас есть конкретный адрес, могу уточнить, входит ли он в зону доставки.
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по Алматы и области! 🗺️ Вот карта наших зон доставки. Доставка стоит 1 500 ₸, но она бесплатна при заказе от 20 000 ₸. Привезём за 1–3 дня.

Какой адрес указать для доставки?
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. При крупном заказе доставка бесплатна. Стоимость доставки 1 500 ₸, а при заказе от 20 000 ₸ доставим бесплатно. Срок доставки составляет 1–3 дня.
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по городу и области, но, к сожалению, в Астану доставка не осуществляется. Уточню у коллеги.
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Доставка в Астану, к сожалению, пока недоступна. Уточню у коллеги, возможна ли индивидуальная доставка в вашем случае.
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Уточню, входит ли Астана в нашу зону доставки — свяжусь с коллегой и дам точный ответ. Какой товар вас интересует?
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Чтобы оформить возврат, пожалуйста, свяжитесь с нашим менеджером по телефону +7 727 300 00 00. Он поможет вам с этим вопросом.
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. Уточните, пожалуйста:

• Номер заказа
• Причина возврата
• Когда заказ пришёл

У нас есть 14 дней на возврат. Свяжусь с коллегой, чтобы быстро решить вопрос.
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 14
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Для возврата средств, пожалуйста, уточните номер вашего заказа, чтобы мы могли помочь вам быстрее. Напоминаем, что срок возврата составляет 14 дней. Ждем вашего ответа!
- contract pass: **true** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Чтобы оформить заказ на кофемашину DeLonghi, напишите, пожалуйста, адрес доставки, и я пришлю вам счёт в WhatsApp. Кофемашина стоит 129 900 ₸ и в наличии 5 штук. Доставка займет от 1–3 дня.
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Отличный выбор! Кофемашина DeLonghi в наличии. Чтобы оформить заказ, пожалуйста, напишите ваш адрес доставки. После этого мы пришлем вам счет в WhatsApp.
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо! Кофемашина DeLonghi в наличии — 5 штук на складе в Алматы.

Для оформления:
1. Подтвердите адрес доставки
2. Выберите способ оплаты (карта, Kaspi, наличные)
3. Мы пришлём счёт в WhatsApp

Доставка за 1–3 дня, стоимость 1 500 ₸.

Какой адрес указать?
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Кофемашина DeLonghi стоит 129 900 ₸. Она достаточно проста в уходе: достаточно регулярно очищать резервуар и менять фильтры. Если у вас есть дополнительные вопросы или хотите оформить заказ, напишите, пожалуйста, адрес доставки, и я отправлю вам счёт в WhatsApp!
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Она автоматическая и очень проста в уходе, идеально подходит для дома. Если хотите, могу прислать фото и видеообзор.
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! 😊 Рады, что подруга вас к нам направила! Кофемашина DeLonghi стоит 129 900 ₸ — отличный выбор для дома. Это автоматическая машина, в уходе довольно простая: регулярно промываем группу и чистим молочную трубку. У нас в наличии 5 штук. Хотите посмотреть фото и видео-обзор, или сразу оформим заказ?
- contract pass: **true** · model-behavior pass: **true**

## xpayment-decisions-v1

### 13. tariff price + commission — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Стандарт» стоит 19 900 ₸, а комиссия составляет 2.5%. Если у вас есть дополнительные вопросы, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 13. tariff price + commission — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Стандарт» стоит 19 900 ₸ в месяц, а комиссия составляет 2.5%. Этот тариф оптимален для растущего бизнеса. Если у вас есть другие вопросы, с радостью помогу!
- contract pass: **true** · model-behavior pass: **true**

### 13. tariff price + commission — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Стандарт» стоит 19 900 ₸ в месяц с комиссией 2.5 процентов. Это оптимальный вариант для растущего бизнеса — лимит платежей 5 000 000 ₸ в месяц. Хотите подробнее о других тарифах или готовы подключиться?
- contract pass: **true** · model-behavior pass: **true**

### 14. payment limit, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: «Бизнес» тарифінің төлем лимиті айына 10 000 000 ₸. Егер қосымша сұрақтарыңыз болса, менімен хабарласыңыз!
- contract pass: **true** · model-behavior pass: **true**

### 14. payment limit, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Лимит по платежам для тарифа «Бизнес» составляет до 10 000 000 ₸ в месяц. Этот тариф оптимален для крупных компаний, которым важен максимальный объём операций и приоритетная поддержка.
- contract pass: **true** · model-behavior pass: **false**

### 14. payment limit, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Бизнес тарифінде ай сайынғы төлем лимиті 10 000 000 ₸. Бұл тариф ірі көлемді операциялар үшін ең қолайлы. Қосымша сұрақтарыңыз болса, 9:00–19:00 ішінде +7 727 300 00 01 номеріне хабарласыңыз.
- contract pass: **true** · model-behavior pass: **true**

### 15. off-KB crypto question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не принимаем оплату криптовалют")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не принимаем оплату криптовалютой. Мы работаем с традиционными методами оплаты. Если у вас есть другие вопросы или нужна помощь, дайте знать!
- contract pass: **true** · model-behavior pass: **false**

### 15. off-KB crypto question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! В базе знаний о криптовалютных платежах информации нет. Уточню у коллеги и дам вам точный ответ. Пока что могу рассказать о наших основных тарифах и комиссиях — они работают с традиционными платежами. Что ещё интересует?
- contract pass: **true** · model-behavior pass: **true**

### 15. off-KB crypto question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, в нашей базе знаний нет информации о приёме оплаты криптовалютой. Уточню этот вопрос у коллеги и вернусь к вам с ответом.
- contract pass: **true** · model-behavior pass: **true**

