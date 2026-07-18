# Contract verdicts (per answer)

## shop-current

### 1. price question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Сейчас она В наличии. Хотите, покажу фото или расскажу подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1579 in / 101 out tokens, est. $0.000468)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 426 out tokens, est. $0.000629)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.001648)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите, покажу фото или расскажу про условия доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1587 in / 95 out tokens, est. $0.000467)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1642 in / 460 out tokens, est. $0.000660)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1697 in / 1626 out tokens, est. $0.003929)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Жеткізу құны 1 500 ₸ по Алматы. Жеткізу мерзімі 1–3 дня. Егер сіздің тапсырысыңыз 20 000 ₸ немесе одан жоғары болса, жеткізу тегін болады. Қандай тауарға тапсырыс бергіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1597 in / 155 out tokens, est. $0.000495)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1652 in / 500 out tokens, est. $0.000698)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1702 in / 2896 out tokens, est. $0.006503)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi сейчас В наличии. Её цена — 129 900 ₸. Хотите посмотреть фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1585 in / 103 out tokens, est. $0.000470)
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1641 in / 447 out tokens, est. $0.000648)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.001650)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды Под заказ, 3–5 дней. Его цена — 24 900 ₸. Если хотите, могу показать фото набора, чтобы вы оценили комплектацию.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1580 in / 113 out tokens, est. $0.000473)
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 500 out tokens, est. $0.000695)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.001648)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот несколько фотографий кофемашины DeLonghi. Она стоит 129 900 ₸ и В наличии. Хотите посмотреть видео-обзор или узнать о доставке?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1764 in / 121 out tokens, est. $0.000526)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1640 in / 479 out tokens, est. $0.000677)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.001650)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, сертификат качества на кофемашину DeLonghi есть. Отправляю его вам. Если хотите узнать цену или наличие, просто спросите — я назову точные цифры.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1771 in / 105 out tokens, est. $0.000521)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1643 in / 459 out tokens, est. $0.000660)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 500 out tokens, est. $0.001651)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставляем по городу и области. Карта зон доставки по Алматы — во вложении. Стоимость доставки — 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ она бесплатна. Срок доставки — 1–3 дня. В каком районе ваш адрес?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1585 in / 142 out tokens, est. $0.000486)
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1639 in / 424 out tokens, est. $0.000627)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.001650)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1634 in / 500 out tokens, est. $0.000695)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если вас интересует конкретный товар, я могу назвать его цену и сроки доставки по Алматы.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1581 in / 112 out tokens, est. $0.000473)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1694 in / 500 out tokens, est. $0.001648)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, в моей базе знаний нет информации о процедуре возврата денег за предыдущие заказы. Чтобы решить ваш вопрос максимально быстро, я передам ваш запрос специалисту отдела поддержки. Они свяжутся с вами в ближайшее время.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1585 in / 136 out tokens, est. $0.000484)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1643 in / 452 out tokens, est. $0.000653)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1705 in / 500 out tokens, est. $0.001652)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично, кофемашина DeLonghi действительно В наличии. Для оформления заказа просто укажите, пожалуйста, адрес доставки в Алматы или области. После этого я отправлю вам счёт для оплаты прямо здесь, в WhatsApp, и оформлю доставку. Доставка по городу занимает 1–3 дня и стоит 1 500 ₸ по Алматы. Куда вам удобнее получить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1615 in / 160 out tokens, est. $0.000502)
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1671 in / 500 out tokens, est. $0.000701)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1737 in / 500 out tokens, est. $0.001664)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Она в В наличии. Уход за ней несложный: достаточно регулярно промывать контейнер для отходов и использовать очищающие таблетки раз в месяц. Хотите, покажу фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1683 in / 150 out tokens, est. $0.000516)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1748 in / 453 out tokens, est. $0.000670)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1824 in / 500 out tokens, est. $0.001697)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Рад приветствовать вас в интернет-магазине «Demo Shop». Помогу подобрать товары для дома или оформить доставку. Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 97 out tokens, est. $0.000466)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1630 in / 500 out tokens, est. $0.000695)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1690 in / 500 out tokens, est. $0.001646)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Если решите, просто напишите, какой товар интересует, и я помогу оформить заказ. У нас есть кофемашина DeLonghi за 129 900 ₸ и набор посуды за 24 900 ₸. Доставка по Алматы — 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ она бесплатная. Жду вашего решения!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1762 in / 164 out tokens, est. $0.000543)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1639 in / 422 out tokens, est. $0.000626)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.001650)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. Если сумма заказа от 20 000 ₸, доставка бесплатная. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1607 in / 118 out tokens, est. $0.000482)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1663 in / 235 out tokens, est. $0.000461)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1724 in / 500 out tokens, est. $0.001659)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1666 in / 459 out tokens, est. $0.000663)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, кофемашина стоит 129 900 ₸. Она сейчас В наличии. Показать вам фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1611 in / 87 out tokens, est. $0.000471)
- contract pass: **true** · model-behavior pass: **true**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1730 in / 500 out tokens, est. $0.001661)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1633 in / 447 out tokens, est. $0.000647)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, в нашем каталоге сейчас нет iPhone. У нас представлена техника для дома, например, кофемашина DeLonghi за 129 900 ₸ и набор посуды за 24 900 ₸. Может, вас заинтересует что-то из этого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1579 in / 128 out tokens, est. $0.000479)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1693 in / 500 out tokens, est. $0.001647)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, в моей базе знаний нет информации о ремонте техники. Чтобы получить точный ответ, я передам ваш вопрос специалисту. А пока могу помочь с выбором новой кофемашины или другого товара из каталога?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1582 in / 125 out tokens, est. $0.000478)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1636 in / 458 out tokens, est. $0.000658)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.001648)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите, я покажу несколько фото или расскажу подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1603 in / 113 out tokens, est. $0.000479)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1661 in / 472 out tokens, est. $0.000674)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1717 in / 500 out tokens, est. $0.001656)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Да, мы доставляем в Астану. Стоимость доставки — 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1763 in / 135 out tokens, est. $0.000531)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1640 in / 500 out tokens, est. $0.000696)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1698 in / 500 out tokens, est. $0.001649)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас В наличии. Хотите посмотреть фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1637 in / 105 out tokens, est. $0.000485)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1691 in / 500 out tokens, est. $0.000704)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1760 in / 500 out tokens, est. $0.001672)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот несколько фотографий набора посуды, чтобы вы могли рассмотреть его детально. Набор стоит 24 900 ₸. Если понравится — просто напишите адрес доставки, и мы оформим заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1618 in / 128 out tokens, est. $0.000489)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1675 in / 360 out tokens, est. $0.000575)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1743 in / 500 out tokens, est. $0.001666)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информации о запасных частях для кофемашин в моей базе нет. Чтобы получить точный ответ, я передам ваш вопрос специалисту. А пока могу показать вам фото самой кофемашины или рассказать о её гарантии?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1611 in / 120 out tokens, est. $0.000484)
- contract pass: **true** · model-behavior pass: **true**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1666 in / 500 out tokens, est. $0.000700)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1729 in / 500 out tokens, est. $0.001661)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Его цена — 24 900 ₸. Если хотите, покажу фото набора.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1615 in / 97 out tokens, est. $0.000476)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1671 in / 266 out tokens, est. $0.000490)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1738 in / 500 out tokens, est. $0.001664)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Чтобы оформить заказ, просто напишите адрес доставки — мы пришлём счёт и всё организуем прямо здесь.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1741 in / 116 out tokens, est. $0.000518)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1803 in / 500 out tokens, est. $0.000720)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1892 in / 500 out tokens, est. $0.001722)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот несколько фотографий кофемашины DeLonghi. Её цена — 129 900 ₸, и она В наличии. Хотите посмотреть видео-обзор или узнать о доставке?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1586 in / 127 out tokens, est. $0.000480)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1642 in / 454 out tokens, est. $0.000655)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1702 in / 500 out tokens, est. $0.001651)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! К сожалению, видео для набора посуды у меня нет. Могу показать вам фотографии набора. Также могу сообщить, что набор посуды стоит 24 900 ₸ и его наличие — Под заказ, 3–5 дней. Присылаю фото?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1584 in / 121 out tokens, est. $0.000477)
- contract pass: **true** · model-behavior pass: **true**

### 28. asks for a media type that does not exist (cookware video) — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1640 in / 500 out tokens, est. $0.000696)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1701 in / 500 out tokens, est. $0.001650)
- contract pass: **false** · model-behavior pass: **false**

### 29. media request, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот несколько фотографий кофемашины DeLonghi. Она сейчас В наличии и стоит 129 900 ₸. Хотите посмотреть видео-обзор или узнать подробнее о доставке?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1590 in / 131 out tokens, est. $0.000483)
- contract pass: **true** · model-behavior pass: **false**

### 29. media request, Kazakh — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1642 in / 500 out tokens, est. $0.000696)
- contract pass: **false** · model-behavior pass: **false**

### 29. media request, Kazakh — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.001650)
- contract pass: **false** · model-behavior pass: **false**

### 30. follow-up with history, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Если сумма заказа превышает 20 000 ₸, доставка бесплатная. Срок доставки — 1–3 дня. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1783 in / 120 out tokens, est. $0.000531)
- contract pass: **true** · model-behavior pass: **false**

### 30. follow-up with history, Kazakh — openrouter:minimax/minimax-m2.5

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1675 in / 497 out tokens, est. $0.000699)
- contract pass: **false** · model-behavior pass: **false**

### 30. follow-up with history, Kazakh — openrouter:moonshotai/kimi-k2.5

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1722 in / 500 out tokens, est. $0.001658)
- contract pass: **false** · model-behavior pass: **false**

