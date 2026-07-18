# Contract verdicts (per answer)

## shop-current

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 89 out tokens, est. $0.000671)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 136 out tokens, est. $0.002527)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или узнать больше о товаре, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 63 out tokens, est. $0.000251)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1493 in / 496 out tokens, est. $0.006704)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.002824)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1421 in / 43 out tokens, est. $0.000239)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 96 out tokens, est. $0.000688)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 136 out tokens, est. $0.002527)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1494 in / 100 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1493 in / 496 out tokens, est. $0.006704)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 1468 out tokens, est. $0.006125)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1449 in / 1173 out tokens, est. $0.001965)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или узнать о доставке, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 62 out tokens, est. $0.000251)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 133 out tokens, est. $0.002517)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 105 out tokens, est. $0.000713)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она есть в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 102 out tokens, est. $0.000191)
- contract pass: **true** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1449 in / 1641 out tokens, est. $0.009399)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1501 in / 496 out tokens, est. $0.006716)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1449 in / 700 out tokens, est. $0.005235)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1697 in / 500 out tokens, est. $0.002825)
- contract pass: **false** · model-behavior pass: **false**

### 1. price question, Russian — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1449 in / 1508 out tokens, est. $0.002394)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или есть другие вопросы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 61 out tokens, est. $0.000250)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 133 out tokens, est. $0.002517)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 76 out tokens, est. $0.000641)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 102 out tokens, est. $0.000191)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1501 in / 496 out tokens, est. $0.006716)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1456 in / 835 out tokens, est. $0.005842)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1456 in / 786 out tokens, est. $0.005626)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной! Чем могу помочь еще?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 72 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок — 1–3 дня. Если заказ на сумму 20 000 ₸ и больше, доставка бесплатная. Какой адрес доставки и какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1859 in / 152 out tokens, est. $0.002619)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки составляет 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 108 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Стоимость доставки по Алматы — 1 500 ₸ по Алматы, а срок доставки составляет 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ или узнать больше о товарах?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 123 out tokens, est. $0.000200)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1456 in / 2172 out tokens, est. $0.003246)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1507 in / 496 out tokens, est. $0.006725)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1702 in / 500 out tokens, est. $0.002828)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1456 in / 2626 out tokens, est. $0.003827)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы и занимает 1–3 дня. Если сумма заказа превышает 20 000 ₸, доставка будет бесплатной. Есть ли что-то конкретное, что вы хотите заказать?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 79 out tokens, est. $0.000262)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок — 1–3 дня. Если сумма заказа от 20 000 ₸, доставка становится бесплатной.

Какой товар вас интересует? Помогу подобрать и оформить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1859 in / 160 out tokens, est. $0.002659)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Доставка по Алматы стоит 1 500 ₸ по Алматы, а срок доставки составляет 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 108 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Доставка по городу Алматы стоит 1 500 ₸ по Алматы. Обычно мы доставляем заказы в течение 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1508 in / 123 out tokens, est. $0.000200)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1507 in / 496 out tokens, est. $0.006725)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1701 in / 500 out tokens, est. $0.002828)
- contract pass: **false** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1697 in / 3422 out tokens, est. $0.012789)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1463 in / 2194 out tokens, est. $0.011866)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi в наличии. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я помогу оформить заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1425 in / 63 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1463 in / 2817 out tokens, est. $0.004074)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi В наличии. Если вас интересует конкретное количество, пожалуйста, уточните.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 87 out tokens, est. $0.000668)
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi В наличии. Её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 101 out tokens, est. $0.000190)
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi В наличии. Точное количество штук уточню у склада — дайте мне минуту. А пока скажите, интересует ли вас эта модель? Цена 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 176 out tokens, est. $0.002732)
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 496 out tokens, est. $0.006712)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1463 in / 1494 out tokens, est. $0.002380)
- contract pass: **false** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1463 in / 1262 out tokens, est. $0.007742)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi в наличии. Если вас интересует покупка, напишите, пожалуйста, адрес доставки, и я помогу оформить заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1425 in / 63 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi В наличии. Если вас интересует конкретное количество, пожалуйста, уточните.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 87 out tokens, est. $0.000668)
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi В наличии. Точное количество штук я не вижу, но если интересует — могу уточнить у склада. Хотите заказать одну?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 163 out tokens, est. $0.002667)
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi сейчас В наличии. Если хотите узнать цену, то она составляет 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 107 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **true**

### 4. stock quantity question, Russian — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 496 out tokens, est. $0.006712)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1303 out tokens, est. $0.002133)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды под заказ, срок доставки составляет 3–5 дней. Если хотите оформить заказ, напишите, пожалуйста, адрес доставки!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 60 out tokens, est. $0.000249)
- contract pass: **true** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1376 out tokens, est. $0.008232)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Хотите оформить предзаказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 81 out tokens, est. $0.000651)
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Цена 24 900 ₸. Если интересует — могу помочь оформить заказ: просто подтвердите, и мы пришлём счёт с деталями доставки. Какой адрес доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 153 out tokens, est. $0.002612)
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Набор посуды у нас под заказ, срок поставки 3–5 дней. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 86 out tokens, est. $0.000184)
- contract pass: **true** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1495 in / 496 out tokens, est. $0.006707)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.002824)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1800 out tokens, est. $0.010108)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас под заказ, срок доставки составляет 3–5 дней. Если вас это устраивает, дайте знать, и я помогу оформить заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 65 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1448 in / 1093 out tokens, est. $0.001862)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1448 in / 853 out tokens, est. $0.005910)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Цена 24 900 ₸. Если интересует — могу помочь оформить заказ: просто подтвердите, и мы пришлём счёт с деталями доставки. Какой адрес для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1847 in / 161 out tokens, est. $0.002652)
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Хотите оформить предзаказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 81 out tokens, est. $0.000651)
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Набор посуды у нас под заказ, срок поставки 3–5 дней. Хотите оформить заказ или посмотреть другие варианты?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 113 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1495 in / 496 out tokens, est. $0.006707)
- contract pass: **false** · model-behavior pass: **false**

### 4. stock quantity question, Russian — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 2755 out tokens, est. $0.003991)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.002824)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот несколько фото кофемашины DeLonghi:
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 56 out tokens, est. $0.000247)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi. Что ещё вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 107 out tokens, est. $0.000717)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi. Она сейчас в наличии, а её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 122 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — спереди, сбоку и с открытым лотком. Если хотите увидеть, как она работает, могу отправить видео-обзор. Интересует цена или характеристики?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1853 in / 160 out tokens, est. $0.002653)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 5. out-of-stock product — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1448 in / 1216 out tokens, est. $0.007517)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот несколько фотографий кофемашины DeLonghi. Если вас интересует цена или наличие, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 69 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **true**

### 5. out-of-stock product — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1448 in / 1493 out tokens, est. $0.002374)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото кофемашины DeLonghi. Если вас интересует что-то конкретное, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 115 out tokens, est. $0.000737)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — спереди, сбоку и с открытым лотком. Если хотите увидеть, как она работает, могу отправить видео-обзор. Интересует цена или характеристики?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1853 in / 160 out tokens, est. $0.002653)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi. Она сейчас в наличии, а её цена — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 122 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 635 out tokens, est. $0.004952)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, у нас есть сертификат качества на кофемашину DeLonghi. Вы можете скачать его по следующей ссылке: [coffee-cert]. Если у вас есть еще вопросы или вы хотите оформить заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 80 out tokens, est. $0.000262)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, конечно! У кофемашины DeLonghi есть сертификат качества. Отправляю вам документ. 😊

Если у вас есть ещё вопросы о кофемашине или вы готовы оформить заказ — напишите, помогу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1856 in / 154 out tokens, est. $0.002626)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1493 out tokens, est. $0.002376)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, конечно! Вот сертификат качества на кофемашину DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1503 in / 85 out tokens, est. $0.000663)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{coffee-cert}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (1503 in / 98 out tokens, est. $0.000190)
- contract pass: **false** · model-behavior pass: **true**

### 7. request certificate — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 496 out tokens, est. $0.006717)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 500 out tokens, est. $0.002830)
- contract pass: **false** · model-behavior pass: **false**

### 6. request product photos — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1091 out tokens, est. $0.006969)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, у нас есть сертификат качества на кофемашину DeLonghi. Вы можете скачать его по следующей ссылке: [coffee-cert]. Если у вас есть другие вопросы или хотите оформить заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1429 in / 79 out tokens, est. $0.000262)
- contract pass: **true** · model-behavior pass: **true**

### 6. request product photos — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1394 out tokens, est. $0.002249)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, конечно! Вот сертификат качества на кофемашину DeLonghi.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1503 in / 85 out tokens, est. $0.000663)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{coffee-cert}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (1503 in / 98 out tokens, est. $0.000190)
- contract pass: **false** · model-behavior pass: **true**

### 7. request certificate — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Да, конечно! На кофемашину DeLonghi есть сертификат качества. Отправляю вам документ. 😊

Если у вас есть ещё вопросы о кофемашине или вы готовы оформить заказ — напишите, помогу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1856 in / 154 out tokens, est. $0.002626)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 539 out tokens, est. $0.004534)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 496 out tokens, est. $0.006717)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1704 in / 500 out tokens, est. $0.002830)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Вот карта зон доставки по Алматы:
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1425 in / 50 out tokens, est. $0.000244)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 1149 out tokens, est. $0.001937)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Вот карта зон доставки по Алматы.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 119 out tokens, est. $0.000748)
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{delivery-zones}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (1500 in / 97 out tokens, est. $0.000189)
- contract pass: **false** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставляем по Алматы и области! Стоимость доставки 1 500 ₸ по Алматы, срок 1–3 дня. При заказе от 20 000 ₸ доставка бесплатна.

Вот карта наших зон доставки — посмотрите, охватываем ли мы ваш адрес. Если адрес в зоне, скажите, какой товар интересует, и оформим заказ прямо здесь! 😊
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1854 in / 211 out tokens, est. $0.002909)
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 496 out tokens, est. $0.006712)
- contract pass: **false** · model-behavior pass: **false**

### 7. request certificate — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 673 out tokens, est. $0.005127)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Вот карта зон доставки по Алматы:
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1425 in / 50 out tokens, est. $0.000244)
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 384 out tokens, est. $0.003842)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставляем по Алматы и области! Стоимость доставки 1 500 ₸ по Алматы, срок 1–3 дня. При заказе от 20 000 ₸ доставка становится бесплатной.

Вот карта наших зон доставки — посмотрите, охватываем ли мы ваш адрес. Если адрес в зоне, скажите, какой товар интересует, и оформим заказ прямо здесь! 😊
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1854 in / 213 out tokens, est. $0.002919)
- contract pass: **true** · model-behavior pass: **true**

### 7. request certificate — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 1228 out tokens, est. $0.002038)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{delivery-zones}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (1500 in / 97 out tokens, est. $0.000189)
- contract pass: **false** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе от 20 000 ₸ доставка будет бесплатной. Вот карта зон доставки по Алматы.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 117 out tokens, est. $0.000742)
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 496 out tokens, est. $0.006712)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "не доставляем в астан"
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если вас интересует товар, я могу помочь с выбором и оформлением заказа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 68 out tokens, est. $0.000254)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "за пределами нашей зоны"
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем! Какой товар вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 142 out tokens, est. $0.002556)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите уточнить стоимость доставки до Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 134 out tokens, est. $0.000784)
- contract pass: **true** · model-behavior pass: **true**

### 8. delivery zones question — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1481 out tokens, est. $0.002361)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка по Алматы бесплатная. К сожалению, в Астану доставка пока не осуществляется.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 116 out tokens, est. $0.000196)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 914 out tokens, est. $0.006188)
- contract pass: **false** · model-behavior pass: **false**

### 8. delivery zones question — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1211 out tokens, est. $0.002015)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1694 in / 500 out tokens, est. $0.002823)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "не доставляем в астан"
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставка доступна только по Алматы и области. Если вас интересует что-то из нашего каталога, дайте знать, и я помогу вам с заказом!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 78 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Хотите уточнить стоимость доставки до Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 134 out tokens, est. $0.000784)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Пока мы доставляем только по городу и области. Стоимость доставки по Алматы — 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ она будет бесплатной! Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 124 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "за пределами нашей зоны"
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: Мы доставляем по Алматы и области. Астана — за пределами нашей зоны доставки. Если у вас есть адрес в Алматы, с удовольствием оформим заказ! Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1846 in / 134 out tokens, est. $0.002516)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1694 in / 500 out tokens, est. $0.002823)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1362 out tokens, est. $0.002208)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, я не могу помочь с вопросами возврата денег. Рекомендую обратиться в нашу службу поддержки по телефону +7 727 300 00 00 или через Instagram @demoshop.kz. Они смогут помочь вам с вашим запросом. Если у вас есть другие вопросы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 104 out tokens, est. $0.000276)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1519 out tokens, est. $0.008863)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я — ассистент интернет-магазина «Demo Shop». Помогу вам с выбором товара и оформлением заказа. Пожалуйста, уточните, о каком заказе идёт речь, чтобы я могла помочь с возвратом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 135 out tokens, est. $0.000787)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. К сожалению, в чате я не вижу детали вашего заказа. Пожалуйста, напишите номер заказа или дату покупки — тогда я смогу помочь быстрее. Или свяжитесь с нашей поддержкой: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1858 in / 223 out tokens, est. $0.002973)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я понимаю, что вы хотите вернуть деньги за прошлый заказ. Для этого, пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 или напишите нам в Instagram @demoshop.kz. Они смогут помочь вам с этим вопросом. График работы поддержки: Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 133 out tokens, est. $0.000203)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1222 out tokens, est. $0.002029)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 496 out tokens, est. $0.006711)
- contract pass: **false** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1200 out tokens, est. $0.007452)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, я не могу помочь с возвратом средств. Пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 или через Instagram @demoshop.kz. Они смогут вам помочь с вашим запросом. Если у вас есть другие вопросы, я с радостью помогу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 108 out tokens, est. $0.000278)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я помогу разобраться с возвратом. К сожалению, детали вашего прошлого заказа у меня нет в системе. Пожалуйста, свяжитесь с нашей поддержкой — они быстро найдут заказ и решат вопрос. Звоните +7 727 300 00 00 или напишите в @demoshop.kz. График: Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1858 in / 236 out tokens, est. $0.003038)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1705 in / 500 out tokens, est. $0.002830)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Чтобы оформить возврат, пожалуйста, уточните номер вашего заказа. Мы проверим информацию и свяжемся с вами.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 87 out tokens, est. $0.000185)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я могу помочь вам выбрать товар и оформить заказ. По вопросам возврата средств, пожалуйста, свяжитесь с нашей службой поддержки по телефону +7 727 300 00 00 или напишите нам в Instagram @demoshop.kz. Они работают Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 141 out tokens, est. $0.000802)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 496 out tokens, est. $0.006711)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1705 in / 500 out tokens, est. $0.002830)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1267 out tokens, est. $0.007750)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 2, 3
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Чтобы оформить заказ на кофемашину DeLonghi, выполните следующие шаги: 1) Подтвердите, что хотите купить кофемашину; 2) Укажите адрес доставки; 3) Мы пришлём счёт и оформим доставку прямо в WhatsApp. Кофемашина стоит 129 900 ₸ и она в наличии. Жду вашего ответа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1448 in / 116 out tokens, est. $0.000287)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1480 out tokens, est. $0.002359)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- invented digits: 1, 2
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Отлично, что вы определились с выбором. Оформить заказ очень просто: 1) укажите адрес доставки; 2) подтвердите заказ — мы пришлём счёт и оформим доставку прямо в WhatsApp.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 109 out tokens, est. $0.000729)
- contract pass: **true** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Чтобы оформить заказ на кофемашину DeLonghi, просто укажите ваш адрес доставки. После этого мы выставим счёт и оформим доставку прямо здесь, в WhatsApp. Кофемашина сейчас В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 116 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- invented digits: 1, 2, 3
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо! Кофемашина DeLonghi В наличии — 129 900 ₸.

Оформляем просто:
1️⃣ Подтверждаете товар (уже ясно — кофемашина)
2️⃣ Укажите адрес доставки в Алматы
3️⃣ Мы пришлём счёт и оформим доставку

Доставка 1–3 дня, стоимость 1 500 ₸ по Алматы.

Какой адрес для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1893 in / 232 out tokens, est. $0.003053)
- contract pass: **true** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1521 in / 496 out tokens, est. $0.006745)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1737 in / 500 out tokens, est. $0.002851)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1250 out tokens, est. $0.002065)
- contract pass: **false** · model-behavior pass: **false**

### 10. refund request — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1453 in / 1135 out tokens, est. $0.007166)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 2, 3
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Чтобы оформить заказ на кофемашину DeLonghi, выполните следующие шаги: 1) Подтвердите, что хотите купить кофемашину; 2) Укажите адрес доставки; 3) Мы пришлём счёт и оформим доставку прямо в WhatsApp. Кофемашина стоит 129 900 ₸ и она сейчас В наличии. Жду вашего ответа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1448 in / 123 out tokens, est. $0.000291)
- contract pass: **true** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Отличный выбор! Чтобы оформить заказ на кофемашину DeLonghi, пожалуйста, укажите адрес доставки. Мы пришлём счёт и оформим доставку прямо в WhatsApp.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 100 out tokens, est. $0.000707)
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Чтобы оформить заказ на кофемашину DeLonghi, просто напишите мне адрес доставки. После этого я пришлю вам счёт для оплаты и оформлю доставку прямо в WhatsApp. Кофемашина сейчас В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 117 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- invented digits: 1, 2, 3
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо! Кофемашина DeLonghi В наличии — 129 900 ₸.

Оформляем просто:
1️⃣ Подтверждаете товар (уже ясно — кофемашина)
2️⃣ Укажите адрес доставки в Алматы
3️⃣ Мы пришлём счёт и оформим доставку

Доставка 1–3 дня, стоимость 1 500 ₸ по Алматы.

Какой адрес для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1893 in / 232 out tokens, est. $0.003053)
- contract pass: **true** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1521 in / 496 out tokens, est. $0.006745)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1737 in / 500 out tokens, est. $0.002851)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1474 in / 1690 out tokens, est. $0.002635)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1474 in / 1524 out tokens, est. $0.008918)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Кофемашина DeLonghi стоит 129 900 ₸. Уход за ней несложный: достаточно регулярно чистить и промывать детали. Если вас интересует, могу рассказать подробнее о процессе ухода или помочь с оформлением заказа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1506 in / 101 out tokens, est. $0.000286)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Уход за ней довольно прост, мы можем прислать вам видеообзор, чтобы вы могли оценить процесс. Хотели бы посмотреть?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 121 out tokens, est. $0.000776)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Рада, что вы обратились к нам. Кофемашина DeLonghi стоит 129 900 ₸. Она в наличии. Уход за ней простой, а для подробной информации и фото можете посмотреть видеообзор.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 123 out tokens, est. $0.000207)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! 😊 Рады, что подруга вас к нам направила!

Кофемашина DeLonghi стоит 129 900 ₸. Это надёжная модель — в уходе несложная, регулярно промываете съёмные части, и всё.

Она В наличии. Хотите посмотреть фото или видео-обзор? Помогу оформить заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1976 in / 213 out tokens, est. $0.003041)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1576 in / 496 out tokens, est. $0.006828)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1824 in / 500 out tokens, est. $0.002909)
- contract pass: **false** · model-behavior pass: **false**

### 11. mid-to-end conversation, closing the order — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1474 in / 1260 out tokens, est. $0.007750)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Кофемашина DeLonghi стоит 129 900 ₸. Уход за ней несложный: достаточно регулярно чистить и промывать детали. Если хотите, могу помочь оформить заказ или ответить на другие вопросы!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1506 in / 96 out tokens, est. $0.000284)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! 😊 Рады, что вас порекомендовали! Кофемашина DeLonghi стоит 129 900 ₸. Это надёжная модель — в уходе несложная, есть автоматическая промывка. Хотите посмотреть фото или видео-обзор? Или у вас есть ещё вопросы перед заказом?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1976 in / 189 out tokens, est. $0.002921)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Рады, что вы к нам обратились. Кофемашина DeLonghi стоит 129 900 ₸. Она очень проста в уходе, достаточно регулярно очищать контейнер для воды и поддон для капель. Могу прислать вам фото или видео-обзор, чтобы вы могли рассмотреть её поближе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 146 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Добрый день! Рада, что вы обратились к нам. Кофемашина DeLonghi стоит 129 900 ₸. Она в наличии, и уход за ней несложный. Хотите узнать больше о ней или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1577 in / 135 out tokens, est. $0.000212)
- contract pass: **true** · model-behavior pass: **true**

### 11. mid-to-end conversation, closing the order — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1474 in / 1991 out tokens, est. $0.003020)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1576 in / 496 out tokens, est. $0.006828)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1824 in / 500 out tokens, est. $0.002909)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1534 in / 2023 out tokens, est. $0.003080)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Как я могу помочь вам сегодня? Если вас интересует какой-то товар, просто напишите, и я назову цену и подберу вариант.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1416 in / 61 out tokens, est. $0.000249)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! 👋 Добро пожаловать в Demo Shop. Мы продаём технику и товары для дома — кофемашины, наборы посуды и другое. Что вас интересует? Помогу подобрать и оформить заказ прямо здесь.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1842 in / 148 out tokens, est. $0.002582)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я — ваш помощник из Demo Shop. Чем могу помочь? У нас есть техника и товары для дома. 😊
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1490 in / 84 out tokens, est. $0.000657)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я ассистент интернет-магазина «Demo Shop». Рад помочь вам с выбором товаров. У нас есть техника и посуда. Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1490 in / 97 out tokens, est. $0.000188)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1489 in / 496 out tokens, est. $0.006698)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1534 in / 2045 out tokens, est. $0.003108)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1690 in / 500 out tokens, est. $0.002820)
- contract pass: **false** · model-behavior pass: **false**

### 12. long rambling message with the price question buried inside — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1534 in / 2939 out tokens, est. $0.015268)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Как я могу помочь вам сегодня? Если вас интересует какой-то товар, например, кофемашина или набор посуды, дайте знать, и я назову цену и подберу вариант!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1416 in / 73 out tokens, est. $0.000256)
- contract pass: **true** · model-behavior pass: **true**

### 12. long rambling message with the price question buried inside — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1534 in / 2239 out tokens, est. $0.012170)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я — ваш помощник из Demo Shop. Чем могу помочь? Ищете что-то конкретное или хотите ознакомиться с нашим ассортиментом?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1490 in / 90 out tokens, est. $0.000672)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! 👋 Добро пожаловать в Demo Shop. Мы продаём технику и товары для дома — кофемашины, наборы посуды и многое другое. Что вас интересует? Помогу подобрать и оформить заказ прямо здесь.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1842 in / 150 out tokens, est. $0.002592)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я ассистент интернет-магазина «Demo Shop». Рад помочь вам с выбором товаров. У нас есть техника и товары для дома, например, кофемашина и набор посуды. Что вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1490 in / 108 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1489 in / 496 out tokens, est. $0.006698)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1690 in / 500 out tokens, est. $0.002820)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1444 in / 1410 out tokens, est. $0.002267)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Пожалуйста! Если у вас возникнут вопросы или вы решите сделать заказ, дайте знать. Мы всегда готовы помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 56 out tokens, est. $0.000247)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо! 😊 Если у вас появятся вопросы или вы будете готовы заказать — напишите мне. Я помогу подобрать товар и оформить доставку. Ждём вас!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 130 out tokens, est. $0.002502)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1444 in / 1564 out tokens, est. $0.009051)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Всегда пожалуйста! Если возникнут вопросы или решите оформить заказ, обращайтесь. Будем рады помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 81 out tokens, est. $0.000652)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Хорошо! Если появятся вопросы или решите оформить заказ, обращайтесь. Будем рады помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 64 out tokens, est. $0.000175)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 496 out tokens, est. $0.006708)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 14. conversation start, bare greeting — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1444 in / 1703 out tokens, est. $0.002642)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Пожалуйста! Если у вас возникнут вопросы или вы решите сделать заказ, не стесняйтесь обращаться. Мы всегда готовы помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1423 in / 59 out tokens, est. $0.000249)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо! 😊 Если у вас появятся вопросы или вы будете готовы заказать — напишите мне. Я помогу подобрать товар и оформить всё прямо здесь. Буду рад помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1852 in / 137 out tokens, est. $0.002537)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Всегда пожалуйста! Если возникнут вопросы или решите оформить заказ, обращайтесь. Будем рады помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 81 out tokens, est. $0.000652)
- contract pass: **true** · model-behavior pass: **true**

### 14. conversation start, bare greeting — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1444 in / 1580 out tokens, est. $0.009121)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Хорошо, буду ждать вашего решения! Если появятся вопросы, обращайтесь.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 58 out tokens, est. $0.000173)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 512 out tokens, est. $0.004406)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 496 out tokens, est. $0.006708)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1700 in / 500 out tokens, est. $0.002827)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 1148 out tokens, est. $0.001934)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸. Если ваш заказ превысит 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 70 out tokens, est. $0.000259)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 101 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **true**

### 15. conversation close, customer is done — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 496 out tokens, est. $0.004335)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1523 in / 496 out tokens, est. $0.006749)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1724 in / 500 out tokens, est. $0.002843)
- contract pass: **false** · model-behavior pass: **false**

### 15. conversation close, customer is done — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 1046 out tokens, est. $0.001803)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 1, 500, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки составляет 1 500 ₸ по Алматы. Если ваш заказ превышает 20 000 ₸, доставка будет бесплатной. Какой товар вы хотите заказать?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 66 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки 1–3 дня. Если сумма заказа будет 20 000 ₸ и выше, доставка станет бесплатной. Хотите оформить заказ кофемашины?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1887 in / 154 out tokens, est. $0.002657)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 102 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1523 in / 496 out tokens, est. $0.006749)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1724 in / 500 out tokens, est. $0.002843)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1479 in / 1711 out tokens, est. $0.002663)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 129, 900
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 45 out tokens, est. $0.000245)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1479 in / 1907 out tokens, est. $0.010620)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 76 out tokens, est. $0.000648)
- contract pass: **true** · model-behavior pass: **true**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше о характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1886 in / 133 out tokens, est. $0.002551)
- contract pass: **true** · model-behavior pass: **true**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 81 out tokens, est. $0.000185)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1479 in / 1289 out tokens, est. $0.002123)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1527 in / 496 out tokens, est. $0.006755)
- contract pass: **false** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1479 in / 1362 out tokens, est. $0.008208)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1730 in / 500 out tokens, est. $0.002847)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 129, 900
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 45 out tokens, est. $0.000245)
- contract pass: **true** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 76 out tokens, est. $0.000648)
- contract pass: **true** · model-behavior pass: **true**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1886 in / 127 out tokens, est. $0.002521)
- contract pass: **true** · model-behavior pass: **true**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 78 out tokens, est. $0.000184)
- contract pass: **true** · model-behavior pass: **true**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1527 in / 496 out tokens, est. $0.006755)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1730 in / 500 out tokens, est. $0.002847)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 1280 out tokens, est. $0.007851)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 1420 out tokens, est. $0.002292)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информации о наличии iPhone 15 Pro у нас нет. Могу помочь вам с выбором других товаров, например, техники или посуды. Напишите, что вас интересует!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 82 out tokens, est. $0.000262)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может, вас интересует что-то из техники для кухни? Или помогу найти другой товар?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1845 in / 144 out tokens, est. $0.002565)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу я помочь вам с выбором чего-то другого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 116 out tokens, est. $0.000739)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! В нашем каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro в наличии нет. Могу предложить вам посмотреть другие товары из нашего ассортимента?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 108 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 496 out tokens, est. $0.006708)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 655 out tokens, est. $0.005086)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1693 in / 500 out tokens, est. $0.002822)
- contract pass: **false** · model-behavior pass: **false**

### 17. history contradiction trap, must re-use token not copy history's literal number — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 1299 out tokens, est. $0.002137)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информации о наличии iPhone 15 Pro у нас нет. Могу помочь вам с выбором других товаров или ответить на другие вопросы?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 75 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro нет в нашем ассортименте. Могу я помочь вам с выбором чего-то другого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 115 out tokens, est. $0.000737)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! В нашем каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить посмотреть другие товары?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 104 out tokens, est. $0.000191)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Привет! 👋 В нашем каталоге есть техника и товары для дома — кофемашина и набор посуды. iPhone в ассортименте нет. Может быть, вас интересует что-то из того, что у нас есть? Или напишите, что ещё ищете — помогу подобрать.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1845 in / 157 out tokens, est. $0.002630)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1496 in / 496 out tokens, est. $0.006708)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 682 out tokens, est. $0.005158)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1693 in / 500 out tokens, est. $0.002822)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, мы не занимаемся ремонтом кофемашин. Мы можем предложить вам купить новую кофемашину DeLonghi, если вас это интересует. Она стоит 129 900 ₸ и сейчас в наличии. Хотите узнать больше?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 86 out tokens, est. $0.000265)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо за вопрос! Мы — интернет-магазин, поэтому ремонтом не занимаемся. Но у нас есть кофемашина DeLonghi 129 900 ₸, В наличии. На технику даём 12 месяцев на технику.

Если нужен ремонт, рекомендую связаться с сервис-центром. Могу помочь с покупкой новой машины — интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1849 in / 203 out tokens, est. $0.002864)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить новые модели. В нашем каталоге есть кофемашина DeLonghi. Хотите узнать о ней подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 140 out tokens, est. $0.000799)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии по цене 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 114 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 1601 out tokens, est. $0.002514)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.002824)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 1598 out tokens, est. $0.002510)
- contract pass: **false** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 1395 out tokens, est. $0.008313)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, мы не занимаемся ремонтом кофемашин. Мы предлагаем только продажу товаров. Если вас интересует покупка кофемашины, дайте знать, и я помогу вам с выбором!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1422 in / 75 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин. Наш магазин специализируется на продаже техники и товаров для дома. Могу я помочь вам с выбором новой кофемашины или другого товара?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 103 out tokens, est. $0.000707)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы продаём кофемашины, но ремонтом не занимаемся. Если у вас есть вопрос по гарантии или неисправности купленной у нас техники — свяжитесь с поддержкой: +7 727 300 00 00, Пн–Сб, 9:00–19:00. Или напишите в @demoshop.kz. Чем я ещё могу помочь?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1849 in / 177 out tokens, est. $0.002734)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас в наличии и стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 114 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 596 out tokens, est. $0.004779)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1303 out tokens, est. $0.002132)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1695 in / 500 out tokens, est. $0.002824)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если хотите, могу рассказать о её характеристиках или помочь с оформлением заказа!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 62 out tokens, est. $0.000253)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать подробнее или оформить заказ? Могу показать фото или видео машины.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 159 out tokens, est. $0.002670)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 100 out tokens, est. $0.000704)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 102 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1513 in / 496 out tokens, est. $0.006733)
- contract pass: **false** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 898 out tokens, est. $0.006115)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1717 in / 500 out tokens, est. $0.002838)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Если вас интересует, как оформить заказ или есть другие вопросы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1437 in / 61 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! 👋 Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Хотите узнать больше о характеристиках или сразу оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1875 in / 149 out tokens, est. $0.002620)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она есть В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 107 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1514 in / 102 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 1498 out tokens, est. $0.002382)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1513 in / 496 out tokens, est. $0.006733)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1717 in / 500 out tokens, est. $0.002838)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1469 in / 1021 out tokens, est. $0.006685)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, мы не доставляем в Астану. Доставляем только по Алматы и области. Если у вас есть другие вопросы или вы хотите узнать о товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 69 out tokens, est. $0.000255)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Астана — это за пределами нашей зоны доставки. Если у вас есть адрес в Алматы или близлежащих районах, с удовольствием поможем с заказом. Или свяжитесь с нашей поддержкой — может быть, найдём решение: +7 727 300 00 00, Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 192 out tokens, est. $0.002815)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Могу я уточнить ваш адрес, чтобы рассчитать точнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 127 out tokens, est. $0.000768)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость доставки по Алматы — 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ доставка бесплатная. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 122 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1469 in / 1691 out tokens, est. $0.002635)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1501 in / 496 out tokens, est. $0.006716)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1469 in / 799 out tokens, est. $0.005702)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1698 in / 500 out tokens, est. $0.002826)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, мы доставляем только по Алматы и области. Если у вас есть вопросы по товарам или доставке в Алматы, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 62 out tokens, est. $0.000251)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо за вопрос! Мы доставляем по Алматы и области. Астана входит в нашу зону доставки — точную стоимость и сроки уточним по вашему адресу. Какой товар вас интересует? Помогу подобрать и оформить заказ.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 152 out tokens, est. $0.002615)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Можете уточнить ваш адрес, чтобы я рассчитал точную стоимость и сроки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 130 out tokens, est. $0.000776)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе от 20 000 ₸ доставка по Алматы будет бесплатной. Хотите узнать точнее для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1502 in / 116 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1501 in / 496 out tokens, est. $0.006716)
- contract pass: **false** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message, rule says answer Russian when mixed — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1469 in / 2113 out tokens, est. $0.003175)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1698 in / 500 out tokens, est. $0.002826)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 1480 out tokens, est. $0.008698)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она сейчас в наличии. Хотите узнать больше о её характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1464 in / 62 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 1831 out tokens, est. $0.002810)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть в наличии. Хотели бы оформить заказ или узнать подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1538 in / 100 out tokens, est. $0.000711)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1923 in / 122 out tokens, est. $0.002533)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ или узнать больше?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1538 in / 103 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1537 in / 496 out tokens, est. $0.006769)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1760 in / 500 out tokens, est. $0.002867)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 1106 out tokens, est. $0.007043)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она сейчас в наличии. Хотите узнать больше о её характеристиках или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1464 in / 62 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите узнать больше или оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1923 in / 122 out tokens, est. $0.002533)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она есть в наличии. Хотите посмотреть фото или видео?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1538 in / 103 out tokens, est. $0.000719)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1538 in / 100 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1537 in / 496 out tokens, est. $0.006769)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1760 in / 500 out tokens, est. $0.002867)
- contract pass: **false** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1457 in / 2396 out tokens, est. $0.003533)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1489 in / 1407 out tokens, est. $0.008422)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото набора посуды, который вы можете заказать. Если вас всё устраивает, напишите адрес доставки, и я помогу оформить заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 72 out tokens, est. $0.000261)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1489 in / 1710 out tokens, est. $0.002665)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото набора посуды. Он сейчас Под заказ, 3–5 дней. Если хотите оформить заказ, напишите мне.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 113 out tokens, est. $0.000741)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото нашего набора посуды. Он доступен под заказ, доставка займёт 3–5 дней. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 116 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото набора посуды — общий вид и отдельные предметы. Набор стоит 24 900 ₸, статус Под заказ, 3–5 дней. Интересует — готовы оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1901 in / 153 out tokens, est. $0.002666)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1527 in / 496 out tokens, est. $0.006755)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1743 in / 500 out tokens, est. $0.002855)
- contract pass: **false** · model-behavior pass: **false**

### 22. history gives distractor context, question switches to another product's price — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1489 in / 1594 out tokens, est. $0.002517)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1477 in / 858 out tokens, est. $0.005975)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот фото набора посуды. Если вас устраивает, напишите адрес доставки, и я помогу оформить заказ!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1452 in / 66 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **true**

### 22. history gives distractor context, question switches to another product's price — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1489 in / 1670 out tokens, est. $0.009586)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото набора посуды. Он сейчас Под заказ, 3–5 дней. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 109 out tokens, est. $0.000731)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото набора посуды — общий вид и отдельные предметы. Набор стоит 24 900 ₸, доступен под заказ на Под заказ, 3–5 дней. Интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1901 in / 149 out tokens, est. $0.002646)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото нашего набора посуды. Он сейчас под заказ, срок доставки 3–5 дней. Стоит 24 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 120 out tokens, est. $0.000201)
- contract pass: **true** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1477 in / 1270 out tokens, est. $0.002098)
- contract pass: **false** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1527 in / 496 out tokens, est. $0.006755)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет информации о продаже запасных частей для кофемашины DeLonghi. Могу помочь с чем-то другим?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 75 out tokens, est. $0.000263)
- contract pass: **true** · model-behavior pass: **true**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Хороший вопрос! К сожалению, в моей базе нет информации о запасных частях для кофемашины. Дайте мне уточнить это у команды — напишу вам в течение дня. А пока скажите, интересует ли вас сама кофемашина? Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1889 in / 185 out tokens, est. $0.002814)
- contract pass: **true** · model-behavior pass: **true**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, в нашем каталоге нет запасных частей для кофемашины DeLonghi. Мы специализируемся на продаже готовой техники и товаров для дома.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 110 out tokens, est. $0.000733)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1743 in / 500 out tokens, est. $0.002855)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, запасные части для кофемашины DeLonghi мы не продаем. Могу предложить вам саму кофемашину, она сейчас в наличии. Хотите узнать подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 113 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1477 in / 602 out tokens, est. $0.004842)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1527 in / 496 out tokens, est. $0.006755)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1729 in / 500 out tokens, est. $0.002846)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информация о продаже запасных частей для кофемашины DeLonghi отсутствует в нашей базе. Я передам ваш запрос для уточнения. Если у вас есть другие вопросы или вы хотите узнать о других товарах, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1451 in / 99 out tokens, est. $0.000277)
- contract pass: **true** · model-behavior pass: **true**

### 23. topic switch to media: history about the cookware set, then asks for photos — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1477 in / 1087 out tokens, est. $0.001864)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информации о продаже запасных частей для кофемашины DeLonghi в нашей базе знаний нет. Могу ли я помочь вам с чем-то ещё?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 107 out tokens, est. $0.000726)
- contract pass: **true** · model-behavior pass: **true**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Спасибо за вопрос! К сожалению, в моей базе нет информации о запасных частях для кофемашины. Уточню это у команды — напишите, пожалуйста, какая именно деталь вам нужна, и я найду ответ или свяжу вас с поддержкой.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1889 in / 184 out tokens, est. $0.002809)
- contract pass: **true** · model-behavior pass: **true**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, запасные части для кофемашины DeLonghi не продаются отдельно. Могу предложить вам саму кофемашину, она сейчас в наличии!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 107 out tokens, est. $0.000196)
- contract pass: **true** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1527 in / 496 out tokens, est. $0.006755)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 768 out tokens, est. $0.005586)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1729 in / 500 out tokens, est. $0.002846)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас под заказ, срок ожидания составляет 3–5 дней.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 48 out tokens, est. $0.000246)
- contract pass: **true** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Цена 24 900 ₸. Если оформите заказ, доставим за 1–3 дня. Хотите узнать больше или оформить?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1898 in / 143 out tokens, est. $0.002613)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Если хотите оформить заказ, напишите, пожалуйста, ваш адрес доставки.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 88 out tokens, est. $0.000677)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас под заказ, ожидаем поступление в течение 3–5 дней. Хотите оформить предзаказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 88 out tokens, est. $0.000187)
- contract pass: **true** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 1225 out tokens, est. $0.002043)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1521 in / 496 out tokens, est. $0.006745)
- contract pass: **false** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 567 out tokens, est. $0.004696)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1738 in / 500 out tokens, est. $0.002852)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас под заказ, срок доставки составляет 3–5 дней. Если вы хотите оформить заказ, напишите, пожалуйста, адрес доставки, и я помогу вам с оформлением!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1447 in / 71 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **false**

### 24. topic switch to escalation: coffee machine discussed, off-KB spare-parts follow-up — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1483 in / 1152 out tokens, est. $0.001949)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Если хотите оформить заказ, напишите, пожалуйста, ваш адрес доставки.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 88 out tokens, est. $0.000677)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас под заказ, срок доставки 3–5 дней. Хотите оформить?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1522 in / 81 out tokens, est. $0.000185)
- contract pass: **true** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Набор посуды сейчас Под заказ, 3–5 дней. Цена 24 900 ₸. Если оформите заказ, мы подготовим его за 1–3 дня и доставим по адресу. Интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1898 in / 146 out tokens, est. $0.002628)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1521 in / 496 out tokens, est. $0.006745)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1738 in / 500 out tokens, est. $0.002852)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1475 in / 1016 out tokens, est. $0.006671)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Чтобы оформить заказ, напишите, пожалуйста, адрес доставки, и подтвердите, что хотите купить. Мы отправим вам счёт и организуем доставку прямо в WhatsApp!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1560 in / 83 out tokens, est. $0.000284)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Для оформления заказа нужно:
1) подтвердить, что берёте этот товар;
2) указать адрес доставки в Алматы;
3) мы пришлём счёт и оформим доставку.

Какой адрес для доставки?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2072 in / 200 out tokens, est. $0.003072)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отличный выбор! Кофемашина DeLonghi стоит 129 900 ₸. Чтобы оформить заказ, просто напишите адрес доставки, и мы пришлём счёт.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1636 in / 99 out tokens, est. $0.000738)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Чтобы оформить заказ, просто напишите мне, какой товар интересует, укажите адрес доставки, и я пришлю вам счёт для оплаты прямо в WhatsApp. Доставка занимает 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1636 in / 135 out tokens, est. $0.000218)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1475 in / 1640 out tokens, est. $0.002571)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 496 out tokens, est. $0.006916)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1892 in / 500 out tokens, est. $0.002954)
- contract pass: **false** · model-behavior pass: **false**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1475 in / 1422 out tokens, est. $0.002292)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Чтобы оформить заказ, напишите, пожалуйста, адрес доставки, и подтвердите заказ — мы пришлём счёт и оформим доставку прямо в WhatsApp!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1560 in / 80 out tokens, est. $0.000282)
- contract pass: **true** · model-behavior pass: **true**

### 25. pronoun resolution across history: availability of the cookware set — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1475 in / 1576 out tokens, est. $0.009149)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.

Для оформления заказа нужно:
1) Подтвердить, что вам подходит этот товар
2) Указать адрес доставки в Алматы
3) Мы пришлём счёт прямо в WhatsApp

Доставка 1–3 дня, стоимость 1 500 ₸ по Алматы. Готовы?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2072 in / 234 out tokens, est. $0.003242)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Отлично! Кофемашина DeLonghi стоит 129 900 ₸. Чтобы оформить заказ, просто напишите адрес доставки, и я пришлю счёт.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1636 in / 96 out tokens, est. $0.000731)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Оформить заказ очень просто: напишите мне, какой товар интересует, укажите адрес доставки, и мы пришлём счёт прямо в WhatsApp для подтверждения.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1636 in / 124 out tokens, est. $0.000213)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 496 out tokens, est. $0.006916)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1892 in / 500 out tokens, est. $0.002954)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1584 in / 1614 out tokens, est. $0.002573)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1584 in / 1285 out tokens, est. $0.008023)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот несколько фотографий кофемашины DeLonghi. Если у вас есть вопросы или вы хотите оформить заказ, дайте знать!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 72 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот несколько фото нашей кофемашины DeLonghi. Если у вас возникнут вопросы, обращайтесь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 113 out tokens, est. $0.000732)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{coffee-photo-1}}, {{coffee-photo-2}}, {{coffee-photo-3}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (1500 in / 141 out tokens, est. $0.000206)
- contract pass: **false** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — спереди, сбоку и с открытым лотком. Если хотите увидеть её в действии, могу отправить видео-обзор. Интересует цена или характеристики?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 159 out tokens, est. $0.002650)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 496 out tokens, est. $0.006712)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1702 in / 500 out tokens, est. $0.002828)
- contract pass: **false** · model-behavior pass: **false**

### 26. long history, the deciding product is named only at the very top — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1584 in / 1057 out tokens, est. $0.007014)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Вот несколько фото кофемашины DeLonghi:
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1426 in / 56 out tokens, est. $0.000247)
- contract pass: **true** · model-behavior pass: **true**

### 26. long history, the deciding product is named only at the very top — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1584 in / 1545 out tokens, est. $0.002484)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно! Вот фото кофемашины DeLonghi — спереди, сбоку и с открытым лотком. Если хотите увидеть её в действии, могу отправить видео-обзор. Интересует цена или характеристики?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1855 in / 159 out tokens, est. $0.002650)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Конечно, вот фото кофемашины. Если у вас возникнут другие вопросы, с радостью на них отвечу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1500 in / 114 out tokens, est. $0.000735)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- **BLOCKED — unknown token(s):** {{coffee-photo-1}}, {{coffee-photo-2}}, {{coffee-photo-3}}
- **leftover `{{` after injection**
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injection clean (no brace survived, whether blocked or not): false
- cost basis: measured_split (1500 in / 140 out tokens, est. $0.000206)
- contract pass: **false** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1499 in / 496 out tokens, est. $0.006712)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1454 in / 697 out tokens, est. $0.005229)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1702 in / 500 out tokens, est. $0.002828)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет видео набора посуды. Могу прислать фото или рассказать о характеристиках. Какой вариант вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 77 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1454 in / 1256 out tokens, est. $0.002073)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, видео набора посуды пока нет, но могу прислать фото. Что бы вы хотели посмотреть?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 105 out tokens, est. $0.000712)
- contract pass: **true** · model-behavior pass: **true**

### 28. asks for a media type that does not exist (cookware video) — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, видео есть только на кофемашину DeLonghi. А вот набор посуды я могу показать фотографиями — общий вид и отдельные предметы. Интересует? Цена набора 24 900 ₸, статус Под заказ, 3–5 дней.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1853 in / 166 out tokens, est. $0.002683)
- contract pass: **true** · model-behavior pass: **true**

### 28. asks for a media type that does not exist (cookware video) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): cookware-video
- invented digits: 3, 5
- **reply_text contains a forbidden phrase:** "вот видео"
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: Здравствуйте! Конечно, вот видео нашего набора посуды. Он доступен под заказ, доставка займет 3–5 дней. Вас интересует, как оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 107 out tokens, est. $0.000193)
- contract pass: **true** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1701 in / 500 out tokens, est. $0.002828)
- contract pass: **false** · model-behavior pass: **false**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:qwen/qwen3.7-plus

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1454 in / 1167 out tokens, est. $0.001959)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:openai/gpt-4o-mini

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет видео набора посуды. Могу прислать фото набора или рассказать о его характеристиках. Какой вариант вас интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1424 in / 80 out tokens, est. $0.000262)
- contract pass: **true** · model-behavior pass: **true**

### 27. asks for ALL photos — exercises the frame's own attachment cap — openrouter:qwen/qwen3.7-max

- parse ok: false
- contract fields ok: false
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1454 in / 1167 out tokens, est. $0.007309)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, у нас нет видео набора посуды. Могу прислать фотографии, если хотите.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 105 out tokens, est. $0.000712)
- contract pass: **true** · model-behavior pass: **true**

### 28. asks for a media type that does not exist (cookware video) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Набор посуды у нас под заказ, доставка займет 3–5 дней. Стоит он 24 900 ₸. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1498 in / 99 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:anthropic/claude-haiku-4.5

- parse ok: true
- contract fields ok: true
- invented digits: 3, 5
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, видео есть только для кофемашины DeLonghi. Для набора посуды у нас есть фото — общий вид и отдельные предметы. Хотите посмотреть? Набор стоит 24 900 ₸, доступен под заказ на 3–5 дней.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1853 in / 171 out tokens, est. $0.002708)
- contract pass: **true** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:google/gemini-3.5-flash

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1497 in / 496 out tokens, est. $0.006710)
- contract pass: **false** · model-behavior pass: **false**

### 28. asks for a media type that does not exist (cookware video) — openrouter:moonshotai/kimi-k2.6

- parse ok: false
- contract fields ok: false
- **TRUNCATED — finish_reason=length** (response cut off before the model finished; contract fails regardless of what parsed)
- **reply_text contains a forbidden phrase:** ""
- requires met: false · media met: false · escalate met: false · language met: false · no-invented-answer met: false · units ok: true · media count ok: n/a (verdict predates this check)
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1701 in / 500 out tokens, est. $0.002828)
- contract pass: **false** · model-behavior pass: **false**

