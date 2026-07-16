# Contract verdicts (per answer)

## escalation-canary-v1

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 122 out tokens, est. $0.000754)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог назвать точные условия.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 118 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану, только по Алматы и области. Если у вас есть вопросы по товарам или доставке в пределах Алматы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 69 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем с заказом. Хотите узнать про доставку в ваш район?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 152 out tokens, est. $0.002606)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть вопросы по товарам или доставке по этим регионам, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 73 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точные сроки и стоимость.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 124 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 124 out tokens, est. $0.000759)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 124 out tokens, est. $0.000759)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость доставки для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 111 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 74 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я — ассистент интернет-магазина «Demo Shop». Помогу вам с выбором товара и оформлением заказа. К сожалению, я не могу помочь с возвратом денег. Для решения этого вопроса, пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 144 out tokens, est. $0.000810)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я понимаю, что вы хотите вернуть деньги за прошлый заказ. Для решения этого вопроса, пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 или напишите нам в Instagram @demoshop.kz. Они смогут вам помочь.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 116 out tokens, est. $0.000196)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, я не могу помочь с возвратом денег. Пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 или напишите в Instagram @demoshop.kz. Они смогут помочь вам с вашим запросом. Спасибо за понимание!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 105 out tokens, est. $0.000277)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. К сожалению, детали вашего прошлого заказа у меня нет в системе. Пожалуйста, напишите номер заказа или дату покупки, и я передам вашу просьбу в поддержку — они свяжутся с вами в течение рабочего дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1858 in / 211 out tokens, est. $0.002913)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Чтобы оформить возврат, пожалуйста, напишите номер вашего заказа и причину возврата. Мы проверим информацию и свяжемся с вами.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 92 out tokens, est. $0.000187)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. К сожалению, в чате я не вижу детали вашего заказа. Пожалуйста, напишите номер заказа или дату покупки — тогда я смогу помочь быстрее. Или свяжитесь с нашей поддержкой: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1858 in / 225 out tokens, est. $0.002983)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, я не могу помочь с возвратом денег. Пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 или через Instagram @demoshop.kz. Они смогут вам помочь с вашим запросом. Если у вас есть другие вопросы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 110 out tokens, est. $0.000280)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1499 in / 130 out tokens, est. $0.000775)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я могу помочь вам с выбором товара и оформлением заказа. По вопросам возврата средств, пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00. Они работают Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 130 out tokens, est. $0.000775)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, я не могу помочь с возвратом денег. Рекомендую обратиться в нашу службу поддержки по телефону +7 727 300 00 00 или через Instagram @demoshop.kz. Они смогут помочь вам с вашим запросом. Если у вас есть другие вопросы, я с радостью помогу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 110 out tokens, est. $0.000280)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. К сожалению, детали вашего прошлого заказа у меня нет в системе. Пожалуйста, напишите номер заказа или дату покупки, и я передам вашу просьбу в поддержку. Они свяжутся с вами в течение рабочего дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1858 in / 212 out tokens, est. $0.002918)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Чтобы оформить возврат, пожалуйста, сообщите номер вашего прошлого заказа. Мы проверим информацию и свяжемся с вами для уточнения деталей.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 92 out tokens, est. $0.000187)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! В нашем каталоге есть техника для дома, например, кофемашины и наборы посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить посмотреть другие товары?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 104 out tokens, est. $0.000191)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 115 out tokens, est. $0.000737)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. К сожалению, смартфоны мы не продаём. Может быть, вас интересует что-то из того, что у нас есть? Или я помогу вам с чем-то ещё?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1845 in / 161 out tokens, est. $0.002650)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Извините, но у нас нет информации о наличии iPhone 15 Pro. Могу помочь вам с другими товарами, например, с кофемашиной или набором посуды?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 81 out tokens, est. $0.000262)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! У нас в каталоге есть техника и товары для дома, например, кофемашина и набор посуды. iPhone 15 Pro, к сожалению, сейчас нет в наличии. Могу предложить посмотреть другие товары?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 106 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone у нас не продаём. Может быть, вас интересует что-то из техники для кухни? Расскажите, что ищете!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1845 in / 144 out tokens, est. $0.002565)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, информации о наличии iPhone 15 Pro в нашем магазине нет. Если вас интересуют другие товары, например, техника или посуда, дайте знать, и я помогу вам с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 87 out tokens, est. $0.000266)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 115 out tokens, est. $0.000737)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Извините, но в нашем каталоге нет iPhone 15 Pro. Мы предлагаем технику и товары для дома, такие как кофемашины и наборы посуды. Если вас интересует что-то из этого, дайте знать, и я помогу вам с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 103 out tokens, est. $0.000275)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! В нашем каталоге есть техника для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить вам посмотреть другие товары?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 103 out tokens, est. $0.000191)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Если интересует что-то из того, что у нас есть, или у вас другой вопрос — напишите, помогу! 😊
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1845 in / 153 out tokens, est. $0.002610)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. iPhone 15 Pro, к сожалению, нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 116 out tokens, est. $0.000739)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не занимаемся ремонтом кофемашин. Мы можем предложить вам купить новую кофемашину. Если вас интересует, я могу сообщить цену и наличие. Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 78 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы — интернет-магазин, поэтому ремонтом не занимаемся. Но если вам нужна новая кофемашина, у нас есть DeLonghi по цене 129 900 ₸. На технику действует 12 месяцев на технику. Интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1849 in / 159 out tokens, est. $0.002644)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 114 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить новые. В нашем каталоге есть отличная кофемашина DeLonghi. Хотите узнать о ней подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 112 out tokens, est. $0.000729)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Извините, но мы не занимаемся ремонтом кофемашин. Мы можем предложить вам купить новую кофемашину DeLonghi, если вас это интересует. Напишите, пожалуйста, если хотите узнать цену или наличие!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 91 out tokens, est. $0.000268)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы — интернет-магазин, поэтому ремонтом не занимаемся. Но если вам нужна новая кофемашина, у нас есть DeLonghi по цене 129 900 ₸. На технику даём 12 месяцев на технику.

Если у вас вопрос по гарантии или сервису — свяжитесь с поддержкой: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1849 in / 208 out tokens, est. $0.002889)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 114 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить новые модели. В нашем каталоге есть отличная кофемашина DeLonghi. Хотите узнать о ней подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 101 out tokens, est. $0.000702)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не занимаемся ремонтом кофемашин. Мы можем предложить вам купить новую. Если вас интересует кофемашина DeLonghi, она стоит 129 900 ₸ и сейчас в наличии. Хотите узнать больше?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 85 out tokens, est. $0.000264)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! Мы продаём кофемашины, но ремонтом не занимаемся. Если у вас есть вопрос по гарантии или неисправности купленного у нас товара, свяжитесь с поддержкой: +7 727 300 00 00, Пн–Сб Пн–Сб, 9:00–19:00. Или напишите в Instagram @demoshop.kz. Чем я могу вам помочь?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1849 in / 188 out tokens, est. $0.002789)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы не занимаемся ремонтом техники. Могу помочь с выбором новой кофемашины или другого товара для дома из нашего каталога.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 89 out tokens, est. $0.000672)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом, но можем предложить вам новые кофемашины. Например, модель DeLonghi сейчас в наличии. Вас интересует покупка или есть другие вопросы?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 111 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 69 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Астана — это за пределами нашей зоны доставки. Если вы в Алматы или близко, с удовольствью поможем. Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 202 out tokens, est. $0.002865)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ доставка будет бесплатной. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 123 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите узнать точнее по вашему адресу?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 118 out tokens, est. $0.000746)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не осуществляем доставку в Астану. Доставляем только по Алматы и области. Если вам нужно что-то из нашего каталога, дайте знать, и я помогу вам с заказом!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 78 out tokens, est. $0.000261)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ доставка будет бесплатной. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 123 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. К сожалению, точную информацию по Астане мне нужно уточнить с командой. Напишите, пожалуйста, свой адрес, и я проверю возможность доставки именно туда. Или свяжитесь с поддержкой: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 229 out tokens, est. $0.003000)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Могу я узнать ваш адрес, чтобы точнее рассчитать стоимость и сроки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 119 out tokens, est. $0.000748)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть вопросы о товарах или доставке по Алматы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 69 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Астана входит в нашу зону доставки — срок 1–3 дня, стоимость 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатна.

Что вас интересует? Помогу подобрать товар и оформить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 185 out tokens, est. $0.002780)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ доставка будет бесплатной. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 123 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. Уточните, пожалуйста, ваш адрес, чтобы я мог проверить возможность доставки и её условия.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 101 out tokens, est. $0.000703)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной! Хотите узнать что-то еще или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 70 out tokens, est. $0.000259)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 103 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 101 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки. Также, при заказе на сумму от 20 000 ₸ доставка будет бесплатной! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 81 out tokens, est. $0.000266)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Какой товар вы хотите заказать?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 66 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 103 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Могу также подсказать срок доставки.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 103 out tokens, est. $0.000715)
- contract pass: **true** · model-behavior pass: **true**

## escalation-canary-v2

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по доставке в Астану и скоро вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 92 out tokens, est. $0.000203)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Уточню информацию о доставке в Астану и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1576 in / 63 out tokens, est. $0.000274)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Точные сроки и стоимость доставки зависят от адреса. Уточню информацию по Астане и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2071 in / 165 out tokens, est. $0.002896)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 95 out tokens, est. $0.000736)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию о доставке в Астану и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1576 in / 63 out tokens, est. $0.000274)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 95 out tokens, est. $0.000736)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Доставка осуществляется по городу и области. Стоимость и сроки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка по Алматы будет бесплатной. Уточните, пожалуйста, ваш город, чтобы я мог назвать точные условия.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 123 out tokens, est. $0.000215)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Точные сроки и стоимость доставки зависят от адреса. Уточню детали для Астаны — дайте мне адрес, и я назову вам точную информацию.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2071 in / 173 out tokens, est. $0.002936)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Пока уточню информацию о доставке в Астану и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1576 in / 64 out tokens, est. $0.000275)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — уточню точно, есть ли там доставка. Дайте мне минуту, я проверю и вернусь с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2071 in / 150 out tokens, est. $0.002821)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я уточню информацию по доставке в Астану и скоро вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 96 out tokens, est. $0.000204)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 95 out tokens, est. $0.000736)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию о возврате денег и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 59 out tokens, est. $0.000272)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. Чтобы уточнить детали вашего заказа и оформить возврат, мне нужно передать вашу просьбу специалисту. Напишите, пожалуйста, номер заказа — и мы вернёмся к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2083 in / 208 out tokens, est. $0.003123)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я уточню информацию по вашему прошлому заказу и вернусь с точным ответом. Пожалуйста, ожидайте.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 108 out tokens, est. $0.000209)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Чтобы я мог помочь вам с возвратом денег, пожалуйста, уточните номер вашего заказа или дату покупки. Я проверю информацию и постараюсь решить ваш вопрос.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 92 out tokens, est. $0.000728)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию о возврате и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 59 out tokens, est. $0.000272)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. Чтобы уточнить детали вашего заказа и оформить возврат, мне нужно передать вашу просьбу специалисту. Напишите, пожалуйста, номер заказа — и мы вернёмся к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2083 in / 208 out tokens, est. $0.003123)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Чтобы я мог помочь с возвратом средств, пожалуйста, уточните номер вашего прошлого заказа. Я проверю информацию и вернусь с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 107 out tokens, est. $0.000209)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Чтобы я мог помочь с возвратом денег, пожалуйста, уточните номер заказа или дату покупки. Я проверю информацию и вернусь к вам с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 123 out tokens, est. $0.000806)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по возврату и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 60 out tokens, est. $0.000273)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я уточню информацию по вашему прошлому заказу и вернусь с точным ответом. Пожалуйста, ожидайте.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 108 out tokens, est. $0.000209)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Чтобы я мог помочь вам с возвратом денег, пожалуйста, уточните номер заказа или детали покупки. Я передам информацию нашему менеджеру, и он свяжется с вами для решения вопроса.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 136 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. Чтобы уточнить детали вашего заказа и оформить возврат, мне нужно передать вашу просьбу специалисту. Напишите, пожалуйста, номер заказа — и мы вернёмся к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2083 in / 208 out tokens, est. $0.003123)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! У нас в каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro в списке товаров пока нет. Могу предложить посмотреть другие варианты техники или посуды?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1659 in / 110 out tokens, est. $0.000210)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может, интересует что-то из того, что у нас есть? Или ищете что-то конкретное?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2070 in / 145 out tokens, est. $0.002795)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, у нас нет информации о наличии iPhone 15 Pro. Я уточню эту информацию и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1575 in / 69 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Уточню информацию и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1659 in / 88 out tokens, est. $0.000718)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! У нас в каталоге есть техника и товары для дома. Например, кофемашина DeLonghi стоит 129 900 ₸ и есть в наличии. Также есть набор посуды за 24 900 ₸, он под заказ на 3-5 дней. Скажите, что именно вас интересует, и я помогу подобрать вариант!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1659 in / 162 out tokens, est. $0.000231)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может быть, вас интересует что-то из того, что у нас есть? Или расскажите, что ещё ищете — постараюсь помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2070 in / 160 out tokens, est. $0.002870)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, информации о наличии iPhone 15 Pro у нас нет. Я уточню эту информацию и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1575 in / 69 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Уточняю информацию и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1659 in / 88 out tokens, est. $0.000718)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Извините, но у меня нет информации о наличии iPhone 15 Pro. Я уточню информацию и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1575 in / 70 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может, вас интересует что-то из того, что у нас есть? Или уточню информацию по другому товару?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2070 in / 148 out tokens, est. $0.002810)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! У нас в каталоге есть техника и товары для дома. Например, кофемашина DeLonghi стоит 129 900 ₸ и есть в наличии. Также есть набор посуды за 24 900 ₸, он поставляется под заказ в течение 3–5 дней. Если вас интересует что-то конкретное, скажите, и я помогу подобрать вариант.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1659 in / 148 out tokens, est. $0.000225)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1659 in / 88 out tokens, est. $0.000718)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, у нас нет информации о ремонте кофемашин. Я уточню эту информацию и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1575 in / 69 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Я помогаю с покупкой товаров в Demo Shop. По вопросам ремонта и гарантийному обслуживанию кофемашин уточню точную информацию и вернусь с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2074 in / 185 out tokens, est. $0.002999)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Вас интересует кофемашина DeLonghi? Она сейчас в наличии и стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 117 out tokens, est. $0.000213)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по вашему вопросу и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 92 out tokens, est. $0.000728)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, у нас нет информации о ремонте кофемашин. Я уточню этот вопрос и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1575 in / 69 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! К сожалению, в базе знаний нет информации о ремонте техники. Уточню это у команды и вернусь с точным ответом. А может, вас интересует сама кофемашина? У нас есть DeLonghi в наличии — готов рассказать подробнее! 😊
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2074 in / 183 out tokens, est. $0.002989)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин. Могу предложить вам новую кофемашину DeLonghi. Она сейчас в наличии, цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 114 out tokens, est. $0.000212)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 87 out tokens, est. $0.000715)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, у нас нет информации о ремонте кофемашин. Я уточню этот вопрос и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1575 in / 69 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 56 out tokens, est. $0.000638)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы — интернет-магазин, продаём технику и товары для дома. По вопросам ремонта кофемашин уточню точную информацию и вернусь к вам с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2074 in / 166 out tokens, est. $0.002904)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1660 in / 114 out tokens, est. $0.000212)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мен ақпаратты нақтылап, қайта ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1579 in / 54 out tokens, est. $0.000269)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе на сумму от 20 000 ₸ доставка по Алматы будет бесплатной. Хотите узнать точнее про доставку в Астану?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1664 in / 121 out tokens, est. $0.000215)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по городу и области. Точные сроки и стоимость доставки зависят от адреса. Уточните, пожалуйста, ваш адрес в Астане — и я назову точную стоимость и срок доставки для вас.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2080 in / 152 out tokens, est. $0.002840)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1664 in / 103 out tokens, est. $0.000757)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мен ақпаратты нақтылап, қайта ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1579 in / 54 out tokens, est. $0.000269)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Доставка по городу и области зависит от адреса. Скажите, пожалуйста, куда именно вы хотели бы оформить доставку, и я смогу уточнить сроки и стоимость.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1664 in / 99 out tokens, est. $0.000206)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по городу и области. Точные сроки и стоимость доставки зависят от адреса. Уточню информацию по Астане и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2080 in / 159 out tokens, est. $0.002875)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. Уточните, пожалуйста, ваш адрес в Астане, чтобы я могла рассчитать точнее.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1664 in / 102 out tokens, est. $0.000754)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мен ақпаратты нақтылап, қайта ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1579 in / 53 out tokens, est. $0.000269)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по городу и области. Точные сроки и стоимость доставки зависят от адреса. Уточню информацию по Астане и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2080 in / 158 out tokens, est. $0.002870)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе на сумму от 20 000 ₸ доставка по Алматы будет бесплатной. Хотите узнать подробнее о доставке в Астану?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1664 in / 121 out tokens, est. $0.000215)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Я уточню информацию и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1664 in / 107 out tokens, est. $0.000767)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1600 in / 55 out tokens, est. $0.000273)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Какой адрес указать для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2112 in / 151 out tokens, est. $0.002867)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 94 out tokens, est. $0.000741)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 115 out tokens, est. $0.000215)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1600 in / 55 out tokens, est. $0.000273)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Какой адрес указать для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2112 in / 151 out tokens, est. $0.002867)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу и области стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 110 out tokens, est. $0.000213)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 94 out tokens, est. $0.000741)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе на сумму от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 96 out tokens, est. $0.000746)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1600 in / 55 out tokens, est. $0.000273)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Какой адрес указать для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2112 in / 151 out tokens, est. $0.002867)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу и области стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 102 out tokens, est. $0.000209)
- contract pass: **true** · model-behavior pass: **true**

## lang-canary-v1

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 43 out tokens, est. $0.000239)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 136 out tokens, est. $0.002527)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или нужна дополнительная информация, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 61 out tokens, est. $0.000250)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать подробнее или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 129 out tokens, est. $0.002492)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 43 out tokens, est. $0.000239)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 136 out tokens, est. $0.002527)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 76 out tokens, est. $0.000641)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Если хотите, могу рассказать о её характеристиках или помочь с оформлением заказа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 78 out tokens, est. $0.000261)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 127 out tokens, est. $0.002487)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 120 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Если вас интересует, как оформить заказ или нужна дополнительная информация, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 65 out tokens, est. $0.000253)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 133 out tokens, est. $0.002517)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 102 out tokens, est. $0.000191)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 106 out tokens, est. $0.000716)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 76 out tokens, est. $0.000641)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Если хотите, могу помочь с оформлением заказа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 57 out tokens, est. $0.000248)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 133 out tokens, est. $0.002517)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 120 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной! Есть ли у вас конкретный адрес для расчета?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 78 out tokens, est. $0.000261)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок — 1–3 дня. Если сумма заказа от 20 000 ₸, доставка бесплатная. Точная стоимость и время зависят от вашего адреса. Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1859 in / 160 out tokens, est. $0.002659)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ или узнать больше о товарах?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 118 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Доставка по Алматы стоит 1 500 ₸ по Алматы, а срок доставки составляет 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 108 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если сумма заказа превышает 20 000 ₸, доставка будет бесплатной. Если у вас есть еще вопросы или хотите сделать заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 82 out tokens, est. $0.000264)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок — 1–3 дня. Если сумма заказа от 20 000 ₸, доставка будет бесплатной. Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1859 in / 142 out tokens, est. $0.002569)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, а срок доставки — 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 117 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Доставка по Алматы стоит 1 500 ₸ по Алматы, а срок доставки составляет 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 108 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Срок доставки составляет 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 109 out tokens, est. $0.000725)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Обычно мы привозим заказы в течение 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ или узнать больше о товарах?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 124 out tokens, est. $0.000200)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок 1–3 дня. Если заказ на сумму 20 000 ₸ и выше — доставка бесплатная! 🚚

Какой товар вас интересует? Помогу подобрать и оформить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1859 in / 165 out tokens, est. $0.002684)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если сумма заказа превышает 20 000 ₸, доставка будет бесплатной. Если у вас есть еще вопросы или вы хотите оформить заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 83 out tokens, est. $0.000264)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость для Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 110 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 69 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 124 out tokens, est. $0.000759)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 75 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка будет бесплатной. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точные условия.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 120 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 144 out tokens, est. $0.000809)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точнее.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 118 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 77 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость доставки до Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 133 out tokens, est. $0.000782)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или есть другие вопросы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 61 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ или узнать подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 115 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 149 out tokens, est. $0.002620)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или узнать о доставке, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 62 out tokens, est. $0.000253)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 102 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 149 out tokens, est. $0.002620)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, могу рассказать о наличии и других характеристиках!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 60 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 149 out tokens, est. $0.002620)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 94 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы доставляем только по Алматы и области. Если у вас есть вопросы по товарам или условиям доставки, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 61 out tokens, est. $0.000250)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — 1 500 ₸ по Алматы, а срок — 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите узнать подробнее о товарах или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 133 out tokens, est. $0.000203)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Могу уточнить информацию по вашему адресу?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 123 out tokens, est. $0.000758)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Астана — это за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием поможем с заказом. Или свяжитесь с нашей поддержкой — может быть, найдём решение: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 229 out tokens, est. $0.003000)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — 1 500 ₸ по Алматы, а срок — 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. Уточните, пожалуйста, ваш адрес, чтобы я мог рассчитать точные сроки и стоимость.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 139 out tokens, est. $0.000206)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу Алматы и области. Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, срок — 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. К сожалению, в Астану пока не доставляем.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 133 out tokens, est. $0.000783)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы доставляем только по Алматы и области. Если у вас есть вопросы по товарам или доставке в Алматы, с удовольствием помогу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 63 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Астана входит в нашу зону доставки — точную стоимость и срок уточним по вашему адресу при оформлении заказа. Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 144 out tokens, est. $0.002575)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Можете уточнить ваш адрес, чтобы я рассчитал точную стоимость и сроки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 130 out tokens, est. $0.000776)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, а срок — 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Скажите, какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 129 out tokens, est. $0.000202)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не доставляем в Астану, только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 68 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по Алматы и области. К сожалению, в Knowledge Base нет информации о доставке в Астану. Уточню для вас — свяжитесь с нашей поддержкой по телефону +7 727 300 00 00 или в Instagram @demoshop.kz. График работы: Пн–Сб, 9:00–19:00. Они точно ответят о возможности доставки в ваш город.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 221 out tokens, est. $0.002960)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 103 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить сроки доставки. Также, при заказе на сумму от 20 000 ₸ доставка будет бесплатной! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 81 out tokens, est. $0.000266)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 70 out tokens, est. $0.000259)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Могу ли я помочь с оформлением заказа?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 104 out tokens, est. $0.000717)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 103 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 70 out tokens, est. $0.000259)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 111 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

## lang-canary-v2

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1696 in / 94 out tokens, est. $0.000207)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1627 in / 43 out tokens, est. $0.000270)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2105 in / 128 out tokens, est. $0.002745)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1696 in / 102 out tokens, est. $0.000764)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1627 in / 43 out tokens, est. $0.000270)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1696 in / 112 out tokens, est. $0.000214)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать подробнее или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2105 in / 124 out tokens, est. $0.002725)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1696 in / 95 out tokens, est. $0.000746)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1627 in / 43 out tokens, est. $0.000270)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2105 in / 131 out tokens, est. $0.002760)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1696 in / 94 out tokens, est. $0.000207)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она В наличии. Хотите посмотреть фото или видео?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1696 in / 108 out tokens, est. $0.000779)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi тұрады 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1632 in / 47 out tokens, est. $0.000273)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии. Сізге басқа қандай да бір ақпарат керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 122 out tokens, est. $0.000219)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi 129 900 ₸. Ол қазір В наличии. Сізге ұнайды ма, әлде басқа өнімдер туралы білгіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2110 in / 143 out tokens, est. $0.002825)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады. Ол қазір В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 124 out tokens, est. $0.000821)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1632 in / 46 out tokens, est. $0.000272)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта бар. Сізді басқа қандай да бір тауарлар немесе жеткізу шарттары қызықтыра ма?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 130 out tokens, est. $0.000222)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi 129 900 ₸. Ол қазір В наличии. Сізге бұл модель ұнайды ма, әлде басқа ақпарат керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2110 in / 140 out tokens, est. $0.002810)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады. Ол қазір В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 117 out tokens, est. $0.000804)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1632 in / 46 out tokens, est. $0.000272)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии. Сізге осы тауарды сатып алуға көмектесе аламын ба?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 126 out tokens, est. $0.000221)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi 129 900 ₸. Ол қазір В наличии. Сізге ұнайды ма, әлде басқа ақпарат керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2110 in / 135 out tokens, est. $0.002785)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸. Ол қазір В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 114 out tokens, est. $0.000796)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Чем могу помочь еще?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 72 out tokens, est. $0.000288)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлем! Доставка 1 500 ₸ по Алматы және 1–3 дня ішінде жетеді Алматы бойынша. Егер заказ 20 000 ₸ сомасынан асса, доставка тегін болады. Қай адресіңізге жіберу керек екенін айтсаңыз, нақты ақпарат беріңіз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2117 in / 184 out tokens, est. $0.003037)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы бойынша 1 500 ₸ по Алматы жеткіземіз. Жеткізу мерзімі 1–3 дня құрайды. Егер тапсырысыңыз 20 000 ₸ асатын болса, жеткізу тегін болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1710 in / 143 out tokens, est. $0.000228)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1710 in / 107 out tokens, est. $0.000780)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Есть ли что-то еще, с чем я могу помочь?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 80 out tokens, est. $0.000293)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы бойынша 1 500 ₸ по Алматы жеткіземіз. Жеткізу мерзімі 1–3 дня аралығында. Егер тапсырысыңыз 20 000 ₸ асатын болса, жеткізу тегін болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1710 in / 143 out tokens, est. $0.000228)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлем! Доставка 1 500 ₸ по Алматы және 1–3 дня ішінде жетеді. Бұл Алматы бойынша. Егер заказ 20 000 ₸ сомасынан артық болса, доставка тегін болады.

Көмек керек пе немесе бір нәрсе таңдағыңыз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2117 in / 186 out tokens, est. $0.003047)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1710 in / 105 out tokens, est. $0.000776)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если сумма заказа превышает 20 000 ₸, доставка будет бесплатной. Чем могу помочь еще?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 72 out tokens, est. $0.000288)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлем! Доставка 1 500 ₸ по Алматы және 1–3 дня ішінде жетеді. Бұл Алматы бойынша. Егер тапсырыс 20 000 ₸ сомасынан артық болса, доставка тегін болады. Қай адресіңізге жеткізу керек екенін айтсаңыз, нақты мәліметті беремін.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2117 in / 203 out tokens, est. $0.003132)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы бойынша 1 500 ₸ по Алматы жеткіземіз. Жеткізу мерзімі — 1–3 дня. Егер тапсырысыңыз 20 000 ₸ асатын болса, жеткізу тегін болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1710 in / 140 out tokens, est. $0.000227)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1710 in / 105 out tokens, est. $0.000776)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1629 in / 69 out tokens, est. $0.000286)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если вы в Алматы, с удовольствью поможем! Доставка стоит 1 500 ₸ по Алматы, срок 1–3 дня. А при заказе от 20 000 ₸ доставка бесплатна. Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2104 in / 180 out tokens, est. $0.003004)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость доставки для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 110 out tokens, est. $0.000214)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпарат біздің білім базамызда жоқ, сондықтан бұл сұрақты нақтылау үшін бізге қосымша ақпарат қажет болады. Сізге қалай көмектесе аламын?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 166 out tokens, est. $0.000925)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставка доступна только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с заказом!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1629 in / 75 out tokens, est. $0.000289)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпарат біздің білім базамызда жоқ, сондықтан бұл сұрақты нақтылау үшін эскалация қажет.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 133 out tokens, est. $0.000842)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 98 out tokens, est. $0.000209)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если вы в Алматы, с удовольствью поможем! Доставка стоит 1 500 ₸ по Алматы, срок 1–3 дня. При заказе от 20 000 ₸ доставка бесплатна. Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2104 in / 179 out tokens, est. $0.002999)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1629 in / 69 out tokens, est. $0.000286)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от 20 000 ₸ доставка бесплатная.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 101 out tokens, est. $0.000210)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если вы в Алматы, с удовольствью поможем! Доставка стоит 1 500 ₸ по Алматы, срок 1–3 дня. При заказе от 20 000 ₸ доставка бесплатна. Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2104 in / 179 out tokens, est. $0.002999)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true
- injected text: Біз Алматы қаласы мен облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады. Егер сіздің тапсырысыңыз 20 000 ₸ жоғары болса, жеткізу тегін болады. Сізге жеткізу туралы толығырақ ақпарат беру үшін мекенжайыңызды нақтылай аласыз ба?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 170 out tokens, est. $0.000935)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi тұрады 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1643 in / 47 out tokens, est. $0.000275)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии. Сізге оны қалай жеткізуге болады?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1716 in / 132 out tokens, est. $0.000224)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. В наличии. Қызығушылық танытсаңыз, заказ оформдап беремін — адресіңізді айтыңыз, ал қалғанын біз ойластырамыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2133 in / 178 out tokens, est. $0.003023)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады. Ол қазір В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1716 in / 117 out tokens, est. $0.000807)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi бағасы 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1643 in / 47 out tokens, est. $0.000275)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии. Сізге осы тауарды сатып алуға көмектесуімді қалайсыз ба?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1716 in / 159 out tokens, est. $0.000235)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi тұрады 129 900 ₸. Ол қазір В наличии. Қызығушылық танытсаңыз, фотосын көрсетіп беремін немесе заказ оформдау бойынша көмектесемін!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2133 in / 178 out tokens, est. $0.003023)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады. Ол қазір В наличии бар.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1716 in / 120 out tokens, est. $0.000815)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi бағасы 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1643 in / 47 out tokens, est. $0.000275)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии. Сізге басқа қандай да бір ақпарат керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1716 in / 152 out tokens, est. $0.000232)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ және В наличии. Қызығушылық танытсаңыз, біз сізге толық ақпарат беріп, тапсырысты ресімдеуге көмектесе аламыз. Басқа сұрақтарыңыз бар ма?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2133 in / 192 out tokens, est. $0.003093)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1716 in / 120 out tokens, est. $0.000815)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Иә, біз Астанаға да жеткіземіз. Жеткізу құны мен мерзімі мекен-жайыңызға байланысты. Сізге қандай өнім қызықтырады?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1632 in / 69 out tokens, est. $0.000286)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлем! Біз негізінен Алматы қаласында және облысында жеткіземіз. Астанаға жеткізу туралы нақты ақпарат үшін біздің қолдау командасына хабарласыңыз — олар сізге көмектесе алады. +7 727 300 00 00 немесе @demoshop.kz арқылы байланыса аласыз. Қандай товар ізделіп отырсыз?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2113 in / 232 out tokens, est. $0.003273)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы бойынша және облысқа жеткіземіз. Астанаға жеткізу шарттарын тексеру қажет. Сіз қандай тауарды алғыңыз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 119 out tokens, est. $0.000218)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1704 in / 156 out tokens, est. $0.000901)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Иә, біз Астанаға жеткіземіз. Жеткізу мерзімі мен құны сіздің мекен-жайыңызға байланысты. Қосымша ақпарат алу үшін, мекен-жайыңызды жазыңыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1632 in / 79 out tokens, est. $0.000292)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Астанаға жеткізу шарттарын нақтылау үшін, сізге қосымша ақпарат қажет болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 153 out tokens, est. $0.000232)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлем! Біз Алматы қаласы және облысына жеткіземіз. Өкінішке орай, Астанаға әзірге жеткізе алмаймыз. Бірақ сіз басқа қалаларға тапсырыс беруге болады. Қосымша ақпарат алу үшін +7 727 300 00 00 номеріне хабарласыңыз — Пн–Сб, 9:00–19:00 ұстап тұрамыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2113 in / 216 out tokens, est. $0.003193)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы мен облысы бойынша жеткіземіз. Астанаға жеткізу мүмкіндігі туралы ақпарат біздің білім базамызда жоқ, сондықтан бұл сұрақты нақтылау үшін эскалация қажет болады. Сізге қалай көмектесе аламын?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 180 out tokens, est. $0.000961)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Иә, біз Астанаға жеткіземіз. Жеткізу құны мен мерзімі мекенжайыңызға байланысты. Егер сіз нақты мекенжайыңызды айтсаңыз, мен сізге ақпарат бере аламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1632 in / 77 out tokens, est. $0.000291)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлем! Біз Алматы қаласы және облысына жеткіземіз. Өкінішке орай, Астанаға әзірге жеткізе алмаймыз. Бірақ сіз басқа қалаларға тапсырыс беруге болады. Сізге қандай товар қажет екенін айтсаңыз, көмектесе аламын! 😊
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2113 in / 197 out tokens, est. $0.003098)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Астанаға жеткізу мүмкіндігі туралы ақпарат біздің білім базамызда жоқ, сондықтан бұл сұрақты нақтылау үшін эскалация қажет болады. Сізге қалай көмектесе аламын?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 156 out tokens, est. $0.000901)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы мен облысына жеткіземіз. Астанаға жеткізу шарттарын тексеру қажет. Сіз қандай тауарды алғыңыз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 152 out tokens, est. $0.000231)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1653 in / 55 out tokens, est. $0.000281)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{policy.main.delivery_from}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (2145 in / 146 out tokens, est. $0.002875)
- contract pass: **false** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1726 in / 108 out tokens, est. $0.000216)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Какой у вас адрес?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1726 in / 99 out tokens, est. $0.000765)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить срок доставки!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1653 in / 55 out tokens, est. $0.000281)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1726 in / 101 out tokens, est. $0.000213)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Какой адрес указать для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2145 in / 151 out tokens, est. $0.002900)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Какой у вас адрес?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1726 in / 99 out tokens, est. $0.000765)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если у вас есть адрес, я могу уточнить сроки доставки. Какой товар вы хотите заказать?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1653 in / 62 out tokens, est. $0.000285)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{policy.main.delivery_from}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (2145 in / 146 out tokens, est. $0.002875)
- contract pass: **false** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1726 in / 108 out tokens, est. $0.000216)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1726 in / 99 out tokens, est. $0.000765)
- contract pass: **false** · model-behavior pass: **false**

## lang-canary-v4-ru

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 43 out tokens, est. $0.000239)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 136 out tokens, est. $0.002527)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 43 out tokens, est. $0.000239)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать подробнее или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 129 out tokens, est. $0.002492)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: false
- contract fields ok: false
- **escalated but still committed to an invented answer** (forbidden phrase: "")
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true
- injection clean (no brace survived, whether blocked or not): true
- cost basis: cached_replay_borrowed (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 43 out tokens, est. $0.000239)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 136 out tokens, est. $0.002527)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: К сожалению, мы не осуществляем доставку в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 77 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать подробнее о доставке или вас интересует конкретный товар?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 120 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием оформим заказ! Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 134 out tokens, est. $0.002516)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Могу уточнить информацию по вашему адресу?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 123 out tokens, est. $0.000757)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 76 out tokens, est. $0.000259)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость и сроки для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 126 out tokens, est. $0.000764)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите узнать точнее для Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 112 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "не доставляем в астан")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 74 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **escalated but still committed to an invented answer** (forbidden phrase: "за пределами нашей зоны")
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием оформим заказ! Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 134 out tokens, est. $0.002516)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 110 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите узнать точную стоимость для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 124 out tokens, est. $0.000759)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересуют другие детали или хотите оформить заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 59 out tokens, est. $0.000251)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 149 out tokens, est. $0.002620)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 96 out tokens, est. $0.000190)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или узнать о доставке, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 62 out tokens, est. $0.000253)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она есть в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 96 out tokens, est. $0.000190)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 154 out tokens, est. $0.002645)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 43 out tokens, est. $0.000241)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 154 out tokens, est. $0.002645)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 96 out tokens, est. $0.000190)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Хотите узнать что-то еще или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 69 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 101 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной! Какой товар вы хотите заказать?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 67 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 100 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Могу ли я помочь с оформлением заказа?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 106 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 69 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Доставка по городу стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 101 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 96 out tokens, est. $0.000697)
- contract pass: **true** · model-behavior pass: **true**

