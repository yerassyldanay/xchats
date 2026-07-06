# Contract verdicts (per answer)

## shop-current

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- contract pass: **true** · model-behavior pass: **true**

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

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если хотите, могу помочь оформить заказ или ответить на другие вопросы!
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
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
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной! Есть ли у вас адрес для расчета доставки?
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок 1–3 дня. Если сумма заказа от 20 000 ₸, доставка становится бесплатной. Какой товар вас интересует?
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки обычно составляет 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной.
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi в наличии. Если хотите узнать цену или оформить заказ, дайте знать!
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi В наличии.
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi В наличии. Точное количество штук уточню у коллег — дайте мне минуту, или напишите свой номер, и я перезвоню с деталями?
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды Под заказ, 3–5 дней. Если хотите оформить заказ, напишите мне.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Цена 24 900 ₸. Интересует? Могу помочь оформить заказ — просто скажите адрес доставки.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды сейчас под заказ, срок доставки составляет 3–5 дней. Если хотите оформить заказ, напишите, пожалуйста, адрес доставки!
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Вот несколько фото кофемашины DeLonghi:
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi. Если вас интересуют другие ракурсы или видеообзор, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — спереди, сбоку и с открытым лотком. Если хотите увидеть, как она работает, могу отправить видео-обзор. Интересует эта модель?
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, у нас есть сертификат качества на кофемашину DeLonghi. Вы можете его скачать по следующей ссылке: [Сертификат качества на кофемашину DeLonghi (PDF)](coffee-cert). Если у вас есть другие вопросы или вы хотите оформить заказ, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, конечно! Сертификат качества на кофемашину DeLonghi у нас есть. Вот документ — можете посмотреть и скачать. Если у вас остались вопросы по кофемашине или вы готовы оформить заказ, я помогу! 😊
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
- injected text: Мы доставляем по городу и области. Вот карта зон доставки по Алматы:
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Вот карта зон доставки по Алматы.
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по Алматы и области. Стоимость доставки 1 500 ₸ по Алматы, срок 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. Вот карта наших зон — посмотрите, ваш адрес в зоне доставки?
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу ли я чем-то ещё помочь?
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем с заказом. Какой товар вас интересует?
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, я не могу помочь с возвратами. Пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 для получения информации о возврате. Они смогут помочь вам с вашим запросом. Спасибо за понимание!
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Понимаю, что вам нужна помощь с возвратом. К сожалению, я не вижу деталей вашего прошлого заказа в чате. Пожалуйста, напишите номер заказа или дату покупки — тогда я смогу помочь быстрее. Или свяжитесь с нашей поддержкой: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
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
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸ и она действительно в наличии. Чтобы оформить заказ, напишите, пожалуйста, адрес доставки, и я помогу вам с подтверждением заказа. Мы пришлём счёт и оформим доставку прямо в WhatsApp!
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Отлично! Кофемашина DeLonghi В наличии и стоит 129 900 ₸.

Для оформления заказа нужно:
1) Подтвердить, что берёте товар
2) Указать адрес доставки в Алматы
3) Мы пришлём счёт и оформим доставку

Доставка 1–3 дня, стоимость 1 500 ₸ по Алматы.

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
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Уход за ней довольно простой: достаточно регулярно очищать резервуар для воды и фильтры. Если у вас есть дополнительные вопросы или хотите оформить заказ, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! 😊 Рады, что вас порекомендовали! Кофемашина DeLonghi стоит 129 900 ₸. Это надёжная модель — в уходе не сложная, регулярно промываете лоток и фильтр.

Хотите посмотреть фото или видео-обзор? Или уже готовы оформить заказ? 😉
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Она очень проста в уходе, достаточно регулярно промывать съемные части и использовать фильтрованную воду. Хотите посмотреть фото или видеообзор?
- contract pass: **true** · model-behavior pass: **true**

## shop-decisions-v1

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она автоматическая и есть в наличии на складе в Алматы. Сейчас в наличии 5 штук.
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. У нас в наличии 5 штук. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я пришлю счёт в WhatsApp.
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Это автоматическая машина для дома, сейчас в наличии 5 штук на складе в Алматы. Хотите узнать больше или оформить заказ?
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. В наличии 5 дана. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я отправлю вам счёт в WhatsApp.
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады. Ол Алматыдағы қоймада 5 дана бар.
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. В наличии 5 штук на складе в Алматы. Интересует? Напишите, и мы оформим заказ — доставим за 1–3 дня.
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Жеткізу құны 1 500 ₸. Тапсырысыңыз 1–3 күні жетеді.
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу стоит 1 500 ₸, но она бесплатна при заказе от 20 000 ₸. Привезём за 1–3 дня. Какой товар вас интересует?
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны 1 500 ₸ құрайды. Егер тапсырыс сомасы 20 000 ₸ жоғары болса, жеткізу тегін болады. Тапсырысыңызды 1–3 күнде жеткіземіз.
- contract pass: **true** · model-behavior pass: **true**

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
- injected text: В наличии 5 штук кофемашины DeLonghi. Это автоматическая кофемашина для дома — отличный выбор! Интересует? Напишите, и мы оформим заказ 😊
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi в наличии в количестве 5 штук. Это автоматическая кофемашина для дома.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, набор посуды в данный момент отсутствует на складе. Если вас интересует, можем оформить заказ, и мы привезем его за несколько дней. Напишите, пожалуйста, ваш адрес доставки, и я подготовлю все необходимое для заказа!
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Набор посуды сейчас под заказ, его привозим за несколько дней. Стоимость набора 24 900 ₸.
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, набор посуды сейчас закончился на складе 😔 Но мы можем привезти его под заказ за 1–3 дня. Интересует? Напишите, и я уточню точные сроки у коллеги.
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Вот фотографии кофемашины DeLonghi. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я пришлю счёт в WhatsApp!
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Конечно, вот фотографии кофемашины DeLonghi. Она автоматическая и есть в наличии на складе в Алматы.
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — автоматическая, в наличии 5 штук. Стоит 129 900 ₸. Интересует?
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, у нас есть сертификат качества на кофемашину DeLonghi. Я прикреплю его к этому сообщению.
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, конечно! 📄 Сертификат качества на кофемашину DeLonghi прилагаю. Это подтверждает, что машина прошла все проверки и готова к работе. Интересует кофемашина? Могу рассказать подробнее или помочь с заказом.
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Да, конечно, сертификат качества на кофемашину DeLonghi есть. Прикрепляю его к сообщению.
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Да, у нас есть карта зон доставки. Мы доставляем по городу и области. Прикрепляю карту для вашего удобства! Если у вас есть дополнительные вопросы, дайте знать.
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по Алматы и области! 🗺️ Вот карта наших зон доставки — посмотрите, доходим ли мы до вас. Доставка стоит 1 500 ₸, но бесплатна при заказе от 20 000 ₸. Привезём за 1–3 дня.

Какой адрес доставки? Помогу рассчитать точно.
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. При крупном заказе доставка бесплатна. Карта зон доставки:
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка осуществляется по городу и области, но, к сожалению, в Астану мы не доставляем. Уточню у коллеги, если появится возможность доставки в ваш город.
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Уточню точно по Астане у коллеги — напишу вам в течение часа. А пока: что вас интересует? Есть кофемашина DeLonghi в наличии или набор посуды под заказ.
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. К сожалению, в Астану пока не доставляем. Уточню у коллеги, возможна ли индивидуальная доставка в вашем случае.
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 14
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Для возврата средств, пожалуйста, уточните номер вашего заказа, и мы начнем процесс возврата. Срок возврата составляет 14 дней.
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Понимаю, что случилось. К сожалению, я не вижу детали вашего заказа. Уточню у коллеги — они свяжутся с вами в течение часа с решением. Можете ли вы назвать номер заказа или дату покупки?
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Чтобы оформить заказ на кофемашину DeLonghi, напишите, пожалуйста, адрес доставки, и я пришлю вам счёт в WhatsApp. Кофемашина стоит 129 900 ₸, в наличии 5 штук. Доставка займет 1–3 дня.
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Отлично! 😊 Кофемашина DeLonghi стоит 129 900 ₸, и у нас есть 5 штук в наличии.

Для оформления:
1. Подтвердите адрес доставки в городе
2. Выберите способ оплаты (карта, Kaspi, наличные)
3. Пришлём счёт в WhatsApp

Доставим за 1–3 дня. Какой адрес?
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Отлично, что вы выбрали кофемашину DeLonghi. Чтобы оформить заказ, напишите, пожалуйста, адрес доставки. После этого мы пришлём счёт в WhatsApp.
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Кофемашина DeLonghi стоит 129 900 ₸. Она довольно проста в уходе: достаточно периодически очищать резервуар и менять фильтры. Если у вас есть дополнительные вопросы или вы хотите оформить заказ, напишите, пожалуйста, адрес доставки, и я помогу вам с оформлением!
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Это автоматическая кофемашина, она проста в уходе. В наличии на складе в Алматы есть 5 штук. Если готовы оформить заказ, напишите нам адрес доставки.
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Добрый день! 😊 Рады, что вас порекомендовали! Кофемашина DeLonghi стоит 129 900 ₸ — отличный выбор для дома. Она автоматическая и довольно удобна в уходе: основные детали легко снимаются и моются.

Чтобы рассказать подробнее об уходе и характеристиках, покажу вам фото и видео-обзор. Интересует?
- contract pass: **true** · model-behavior pass: **true**

## xpayment-decisions-v1

### 13. tariff price + commission — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Стандарт» стоит 19 900 ₸ в месяц, а комиссия составляет 2.5%. Этот тариф оптимален для растущего бизнеса. Могу прислать подробную информацию по всем тарифам.
- contract pass: **true** · model-behavior pass: **true**

### 13. tariff price + commission — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Стандарт» стоит 19 900 ₸ в месяц с комиссией 2.5%. Это оптимальный выбор для растущего бизнеса — лимит платежей 5 000 000 ₸ в месяц. Хотите узнать про другие тарифы или начать подключение?
- contract pass: **true** · model-behavior pass: **true**

### 13. tariff price + commission — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Стандарт» стоит 19 900 ₸ в месяц, а комиссия составляет 2.5%. Если у вас есть дополнительные вопросы, дайте знать!
- contract pass: **true** · model-behavior pass: **true**

### 14. payment limit, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: «Бизнес» тарифінің төлем лимиті айына 10 000 000 ₸. Қосымша ақпарат қажет болса, хабарласыңыз!
- contract pass: **true** · model-behavior pass: **true**

### 14. payment limit, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Тариф «Бизнес» үшін ай сайынғы төлем лимиті 10 000 000 ₸. Бұл ең жоғары лимит — ірі көлемді операциялар үшін идеалды. Сізге бұл тариф ұсынылса, біз сізге толық ақпарат беремін!
- contract pass: **true** · model-behavior pass: **true**

### 14. payment limit, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! «Бизнес» тарифінің айлық төлем лимиті 10 000 000 ₸ құрайды. Бұл тариф максималды көлем және басым қолдау үшін оңтайлы.
- contract pass: **true** · model-behavior pass: **true**

### 15. off-KB crypto question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не принимаем оплату криптовалют")
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не принимаем оплату криптовалютой. Мы работаем с традиционными методами оплаты. Если у вас есть другие вопросы или нужна помощь, пожалуйста, дайте знать!
- contract pass: **true** · model-behavior pass: **false**

### 15. off-KB crypto question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, в нашей базе знаний нет информации о приеме оплаты криптовалютой. Я уточню этот вопрос у коллеги и вернусь с ответом. Спасибо за понимание!
- contract pass: **true** · model-behavior pass: **true**

### 15. off-KB crypto question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! В базе знаний у меня нет информации о приёме криптовалют. Уточню у коллеги — напишите, пожалуйста, свои контакты, и мы вам ответим в рабочие часы 9:00–19:00.
- contract pass: **true** · model-behavior pass: **true**

