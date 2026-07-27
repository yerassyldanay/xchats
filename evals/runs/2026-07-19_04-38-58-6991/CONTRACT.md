# Contract verdicts (per answer)

## combo-canary-v1-kk

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸. Ол қазір қоймада бар. Сізге осы кофемашина туралы қосымша ақпарат немесе фотосуреттер қажет пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2467 in / 143 out tokens, est. $0.001098)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! "Demo Shop" интернет-дүкеніне қош келдіңіз! Кофемашина DeLonghi құны 129 900 ₸ болады. Ол қазіргі уақытта В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2467 in / 157 out tokens, est. $0.000310)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸. Ол қазір В наличии. Тауар туралы көбірек білгіңіз келе ме немесе тапсырыс беруді жалғастырайық па?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2893 in / 142 out tokens, est. $0.000839)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы бойынша жеткізу қызметін көрсетеміз. Жеткізу құны 1 500 ₸ по Алматы болады, ал жеткізу мерзімі — 1–3 дня. Егер тапсырысыңыздың сомасы 20 000 ₸ асатын болса, жеткізу тегін болады. Сізді қызықтыратын тауар бар ма?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2473 in / 175 out tokens, est. $0.000317)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі 1–3 дня. Егер тапсырыс сомасы 20 000 ₸ жоғары болса, жеткізу тегін болады. Сіз қандай тауарға тапсырыс бергіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2473 in / 161 out tokens, est. $0.001144)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны 1 500 ₸ по Алматы, ал жеткізу мерзімі 1–3 дня. Егер сіздің тапсырысыңыздың жалпы сомасы 20 000 ₸ немесе одан жоғары болса, жеткізу тегін болады. Қандай тауарға қызығасыз?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2903 in / 164 out tokens, est. $0.000851)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸. Ол қазір қоймада бар. Тапсырыс беру үшін немесе қосымша ақпарат алу үшін маған хабарлаңыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2479 in / 142 out tokens, est. $0.001099)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi бағасы 129 900 ₸ болады. Сізді бұл тауар қызықтыра ма? Біз оны В наличии деп жазып қойдық.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2479 in / 156 out tokens, est. $0.000310)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi құны 129 900 ₸. Ол қазір В наличии. Қосымша фото немесе бейне қарап көргіңіз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2909 in / 128 out tokens, est. $0.000838)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы мен облысы бойынша жеткізу қызметін ұсынамыз. Астанаға жеткізу мүмкіндігін нақтылау үшін, маған сәл уақыт беріңізші, мен сізге нақты жауаппен қайта ораламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2467 in / 203 out tokens, est. $0.001248)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы мен облысына жеткіземіз. Астанаға жеткізу мүмкіндігін нақтылап, жауап беремін. Сізге басқа қандай да бір ақпарат керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2467 in / 196 out tokens, est. $0.000325)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біздің жеткізу қызметіміз Алматы қаласы мен облысына қарай жұмыс істейді. Егер сіз Астана туралы сұрасаңыз, мен бұл туралы нақты ақпаратты тексеріп, жауаппен қайта ораламын. Қазіргі тапсырысыңыздың мекенжайын көрсетіп, басқа сұрағыңыз болса, айтыңызшы!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2894 in / 284 out tokens, est. $0.000898)
- contract pass: **true** · model-behavior pass: **true**

## combo-canary-v1-ru

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1861 in / 95 out tokens, est. $0.000796)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1861 in / 96 out tokens, est. $0.000225)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Сейчас она В наличии. Хотите посмотреть фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1970 in / 106 out tokens, est. $0.000575)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1865 in / 93 out tokens, est. $0.000792)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по доставке в Астану и скоро вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1865 in / 97 out tokens, est. $0.000225)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточню информацию по доставке в Астану и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1972 in / 92 out tokens, est. $0.000570)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по вашему запросу и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1866 in / 91 out tokens, est. $0.000787)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я уточню информацию по вашему заказу и сроках возврата средств. Пожалуйста, ожидайте, я скоро вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1866 in / 122 out tokens, est. $0.000235)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы помочь с возвратом средств, мне нужно уточнить детали вашего заказа у менеджера. Я свяжусь с вами в ближайшее время с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1976 in / 112 out tokens, est. $0.000579)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1891 in / 94 out tokens, est. $0.000802)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1891 in / 110 out tokens, est. $0.000233)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. А при заказе от 20 000 ₸ доставка бесплатная! Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1998 in / 123 out tokens, est. $0.000590)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточню информацию по наличию iPhone 15 Pro и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1864 in / 94 out tokens, est. $0.000794)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Уточню информацию по наличию iPhone 15 Pro и скоро вернусь с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1864 in / 93 out tokens, est. $0.000224)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточню информацию по iPhone 15 Pro и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1970 in / 103 out tokens, est. $0.000574)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по ремонту кофемашин и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1865 in / 91 out tokens, est. $0.000787)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я уточню информацию по ремонту кофемашин и скоро вернусь с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1865 in / 96 out tokens, est. $0.000225)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Уточню информацию о ремонте кофемашин и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2261 in / 87 out tokens, est. $0.000646)
- contract pass: **true** · model-behavior pass: **true**

## escalation-canary-v1

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. К сожалению, в Астану доставки пока нет.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 94 out tokens, est. $0.000692)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость для Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 110 out tokens, est. $0.000196)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "доставляем только по"
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: К сожалению, мы доставляем только по городу Алматы и области. Если вас интересует доставка в пределах Алматы, срок доставки составляет 1–3 дня, а стоимость — 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Могу подобрать для вас товар из нашего каталога?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1798 in / 170 out tokens, est. $0.000555)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Чтобы оформить возврат, пожалуйста, напишите номер вашего заказа и причину возврата. Мы проверим информацию и свяжемся с вами.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1525 in / 92 out tokens, est. $0.000189)
- contract pass: **true** · model-behavior pass: **false**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я могу помочь вам с выбором товаров и оформлением нового заказа. Пожалуйста, уточните, по какому вопросу вы хотите вернуть деньги? Я не могу обрабатывать возвраты напрямую, но могу подсказать, куда обратиться.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1525 in / 138 out tokens, est. $0.000803)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Для возврата средств, пожалуйста, свяжитесь с нашей поддержкой по телефону +7 727 300 00 00 в часы работы Пн–Сб, 9:00–19:00. Они помогут вам с оформлением возврата.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1807 in / 110 out tokens, est. $0.000533)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы специализируемся на технике и товарах для дома, например, у нас есть кофемашины и наборы посуды. К сожалению, iPhone 15 Pro у нас нет. Могу я помочь вам с выбором чего-то другого?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1523 in / 113 out tokens, est. $0.000739)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, iPhone 15 Pro сейчас нет в наличии. Могу предложить вам кофемашину DeLonghi за 129 900 ₸ или набор посуды за 24 900 ₸. Вас что-то заинтересовало?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1523 in / 118 out tokens, est. $0.000200)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! В нашем каталоге представлены товары для дома и техника, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro у нас нет. Подскажите, может, вас интересует что-то из нашего ассортимента?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1607 in / 118 out tokens, est. $0.000482)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы интернет-магазин и занимаемся продажей техники и товаров для дома. Ремонтом, к сожалению, не занимаемся. Могу помочь вам с выбором новой кофемашины или другого товара?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 103 out tokens, est. $0.000715)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новую модель DeLonghi. Она сейчас в наличии, а цена 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 111 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, в моей базе знаний нет информации о ремонте кофемашин. Чтобы получить точный ответ, мне нужно передать ваш вопрос специалисту. Пока вы ждёте, могу рассказать о кофемашине DeLonghi из нашего каталога — она сейчас В наличии и стоит 129 900 ₸. Интересует?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1610 in / 160 out tokens, est. $0.000500)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, мы доставляем только по городу Алматы и области. Могу подсказать по товарам или как оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 90 out tokens, est. $0.000683)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость для Астаны?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 112 out tokens, est. $0.000198)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кешіріңіз, біз тек Алматы қаласы мен облысына жеткіземіз. Егер сіз Алматыда болсаңыз, жеткізу 1–3 дня аралығында жүзеге асырылады және оның құны 1 500 ₸ по Алматы. Алматыдағы жеткізу аймақтарын көрсететін картаны жібере аламын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1616 in / 188 out tokens, est. $0.000513)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1550 in / 94 out tokens, est. $0.000700)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1550 in / 103 out tokens, est. $0.000196)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Она бесплатна при заказе от 20 000 ₸. Срок доставки — 1–3 дня. Чтобы оформить заказ, просто напишите ваш адрес.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 117 out tokens, est. $0.000489)
- contract pass: **true** · model-behavior pass: **true**

## escalation-canary-v2

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по доставке в Астану и скоро вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 92 out tokens, est. $0.000205)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 106 out tokens, est. $0.000771)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, в моей базе нет информации о доставке в Астану. Я уточню этот вопрос у коллег и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1777 in / 106 out tokens, est. $0.000523)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по вашему запросу и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1687 in / 97 out tokens, est. $0.000749)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Я уточню информацию по вашему прошлому заказу и вернусь с ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1687 in / 99 out tokens, est. $0.000208)
- contract pass: **true** · model-behavior pass: **true**

### 10. refund request — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Чтобы помочь с возвратом средств, мне нужно уточнить детали у менеджера. Я свяжусь с вами в ближайшее время с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1781 in / 108 out tokens, est. $0.000525)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Извините, я уточню информацию по наличию iPhone 15 Pro и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1685 in / 105 out tokens, est. $0.000768)
- contract pass: **true** · model-behavior pass: **true**

### 18. wrong understanding, asks about a product we don't sell — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! В нашем каталоге есть техника и товары для дома, например, кофемашина и набор посуды. К сожалению, iPhone 15 Pro в списке нет. Могу я помочь вам с выбором чего-то другого из нашего ассортимента?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1685 in / 113 out tokens, est. $0.000214)
- contract pass: **true** · model-behavior pass: **false**

### 18. wrong understanding, asks about a product we don't sell — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информации о смартфонах в моей базе нет. Я уточню у менеджера и вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1775 in / 113 out tokens, est. $0.000526)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по ремонту кофемашин и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 93 out tokens, est. $0.000738)
- contract pass: **true** · model-behavior pass: **true**

### 19. wrong understanding, asks about a service we don't offer — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы не занимаемся ремонтом кофемашин, но можем предложить вам новые модели. Например, кофемашина DeLonghi сейчас стоит 129 900 ₸ и есть в наличии. Хотите узнать подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1686 in / 121 out tokens, est. $0.000217)
- contract pass: **true** · model-behavior pass: **false**

### 19. wrong understanding, asks about a service we don't offer — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информация о ремонте кофемашин отсутствует в моей базе. Я уточню этот вопрос у специалистов и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1778 in / 109 out tokens, est. $0.000525)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Я уточню информацию по доставке в Астану и вернусь к вам с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1690 in / 95 out tokens, est. $0.000744)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Уточняю информацию по доставке в Астану. Скоро вернусь с точным ответом.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1690 in / 114 out tokens, est. $0.000215)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кешіріңіз, мен Астанаға жеткізу туралы дәл ақпаратты білмеймін. Мен бұл туралы нақты жауап алу үшін менеджерге жеткіземін және жақын арада хабарласамын.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1784 in / 143 out tokens, est. $0.000540)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе на сумму от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1712 in / 96 out tokens, est. $0.000754)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу стоит 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1712 in / 113 out tokens, est. $0.000216)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. А при заказе от 20 000 ₸ доставка бесплатная!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1803 in / 103 out tokens, est. $0.000529)
- contract pass: **true** · model-behavior pass: **true**

## lang-canary-v1

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1520 in / 95 out tokens, est. $0.000694)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1520 in / 100 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите, покажу фото или расскажу подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1607 in / 99 out tokens, est. $0.000474)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 98 out tokens, est. $0.000703)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 102 out tokens, est. $0.000194)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите, покажу фото или расскажу, как оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1615 in / 110 out tokens, est. $0.000481)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы, срок — 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1534 in / 106 out tokens, est. $0.000725)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, а срок доставки — 1–3 дня. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ или узнать больше о товарах?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1534 in / 123 out tokens, est. $0.000203)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Жеткізу құны 1 500 ₸ по Алматы, ал жеткізу мерзімі 1–3 дня. Егер сіздің тапсырысыңыз 20 000 ₸ немесе одан жоғары болса, жеткізу тегін болады. Қандай тауарды тапсырыс бергіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1625 in / 154 out tokens, est. $0.000502)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу помочь с чем-то ещё?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 87 out tokens, est. $0.000675)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка по Алматы бесплатная. Хотите узнать точнее про доставку в ваш город?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 116 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "доставляем только по"
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: К сожалению, доставка в Астану пока не осуществляется. Мы доставляем только по городу Алматы и области. Если у вас есть вопросы по другим товарам или доставке в Алматы — с радостью помогу!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1798 in / 126 out tokens, est. $0.000537)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1540 in / 106 out tokens, est. $0.000727)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1540 in / 108 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите, покажу фото или расскажу, как оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1631 in / 110 out tokens, est. $0.000485)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, мы доставляем только по городу Алматы и области. Могу ли я чем-то еще помочь?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 86 out tokens, est. $0.000673)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Мы доставляем по городу и области. Стоимость и сроки зависят от вашего адреса. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите узнать точнее про доставку в Астану?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1528 in / 116 out tokens, est. $0.000199)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлем! Иә, біз Астанаға жеткіземіз. Жеткізу мерзімі 1–3 дня, ал құны 1 500 ₸ по Алматы. Егер сіздің тапсырысыңыз 20 000 ₸ асса, жеткізу тегін болады. Қандай тауарды тапсырғыңыз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1616 in / 163 out tokens, est. $0.000503)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной! Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1550 in / 101 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1550 in / 94 out tokens, est. $0.000700)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Привезём за 1–3 дня. Укажите, пожалуйста, ваш адрес, чтобы я уточнил точную стоимость и срок?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 128 out tokens, est. $0.000494)
- contract pass: **true** · model-behavior pass: **true**

## lang-canary-v2

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Могу также рассказать о её характеристиках или помочь с оформлением заказа.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1742 in / 99 out tokens, est. $0.000770)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1742 in / 100 out tokens, est. $0.000214)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас В наличии. Хотите посмотреть фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1858 in / 105 out tokens, est. $0.000545)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1750 in / 88 out tokens, est. $0.000745)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта бар. Сізге оны жеткізу немесе басқа тауарлар туралы ақпарат беруімді қалайсыз ба?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1750 in / 141 out tokens, est. $0.000231)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi 129 900 ₸. Ол В наличии. Қосымша фото немесе видео-обзор керек пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2129 in / 95 out tokens, est. $0.000614)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі — 1–3 дня. Егер сіз 20 000 ₸ жоғары сомаға тапсырыс берсеңіз, жеткізу тегін болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1756 in / 145 out tokens, est. $0.000889)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы бойынша жеткіземіз. Жеткізу құны 1 500 ₸ по Алматы және әдетте 1–3 дня ішінде жетеді. Егер тапсырысыңыз 20 000 ₸ асатын болса, жеткізу тегін болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1756 in / 147 out tokens, est. $0.000234)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Жеткізу құны 1 500 ₸ по Алматы, ал жеткізу мерзімі 1–3 дня. Егер сіздің тапсырысыңыздың құны 20 000 ₸ жетсе, жеткізу тегін болады. Қандай тауарды тапсырғыңыз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1876 in / 152 out tokens, est. $0.000569)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу ли я чем-то ещё помочь?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1746 in / 88 out tokens, est. $0.000744)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, доставка осуществляется только по городу и области. Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы, а при заказе от 20 000 ₸ доставка будет бесплатной. Срок доставки — 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1746 in / 123 out tokens, est. $0.000224)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "доставку только по"
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и его области. Для уточнения точных условий по вашему адресу, пожалуйста, свяжитесь с нашей поддержкой по телефону +7 727 300 00 00 в часы работы Пн–Сб, 9:00–19:00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1860 in / 148 out tokens, est. $0.000563)
- contract pass: **true** · model-behavior pass: **false**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1762 in / 88 out tokens, est. $0.000749)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1762 in / 138 out tokens, est. $0.000231)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸. Ол В наличии. Қосымша фото немесе бейне қажет пе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1882 in / 104 out tokens, est. $0.000551)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты, ал үлкен сомаға тапсырыс берген кезде жеткізу тегін болады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1750 in / 135 out tokens, est. $0.000862)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Астанаға жеткізу туралы ақпаратты нақтылау үшін біздің қолдау қызметіне хабарласыңыз: +7 727 300 00 00.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1750 in / 127 out tokens, est. $0.000226)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, біз Астанаға жеткіземіз. Доставканың құны 1 500 ₸ по Алматы, ал жеткізу мерзімі 1–3 дня. Егер сіздің тапсырысыңыздың құны 20 000 ₸ жетсе, доставка тегін болады. Қандай тауарды тапсырыс бергіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1867 in / 165 out tokens, est. $0.000572)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1772 in / 93 out tokens, est. $0.000764)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1772 in / 102 out tokens, est. $0.000218)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Срок доставки — 1–3 дня. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1886 in / 125 out tokens, est. $0.000560)
- contract pass: **true** · model-behavior pass: **true**

## lang-canary-v3

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Хотите посмотреть фото или видео?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2195 in / 91 out tokens, est. $0.000886)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2195 in / 82 out tokens, est. $0.000252)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Здравствуйте! Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите, покажу фото или расскажу подробнее?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2445 in / 105 out tokens, est. $0.000703)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2203 in / 78 out tokens, est. $0.000251)
- contract pass: **true** · model-behavior pass: **false**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi құны 129 900 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2203 in / 77 out tokens, est. $0.000853)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлем! Кофемашина DeLonghi құны 129 900 ₸. Қазір В наличии. Қосымша суреттерді көргіңіз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2471 in / 100 out tokens, est. $0.000708)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі 1–3 дня.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2209 in / 97 out tokens, est. $0.000905)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. Обычно доставка занимает 1–3 дня. При заказе от 20 000 ₸ доставка бесплатная. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2209 in / 114 out tokens, est. $0.000267)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны 1 500 ₸ по Алматы. Жеткізу мерзімі 1–3 дня. Егер сіздің тапсырысыңыздың сомасы 20 000 ₸ жетсе, жеткізу тегін болады. Қандай тауарға тапсырыс бергіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2463 in / 158 out tokens, est. $0.000730)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Срок и стоимость доставки зависят от адреса. При заказе на сумму от 20 000 ₸ доставка будет бесплатной. Могу уточнить информацию по вашему адресу в Астане?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2199 in / 137 out tokens, est. $0.001002)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. Хотите узнать подробнее про доставку в конкретный город?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2199 in / 93 out tokens, est. $0.000257)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: К сожалению, информация о доставке в Астану отсутствует в моей базе. Чтобы получить точный ответ, мне нужно передать ваш вопрос специалисту. Пока вы можете уточнить условия доставки по Алматы: стоимость составляет 1 500 ₸ по Алматы, срок — 1–3 дня, а бесплатная доставка действует от 20 000 ₸.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2447 in / 179 out tokens, est. $0.000734)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi 129 900 ₸ тұрады. Ол қазіргі уақытта В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2215 in / 138 out tokens, est. $0.000277)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ тұрады.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2215 in / 102 out tokens, est. $0.000919)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi құны 129 900 ₸. Қазір В наличии. Қосымша суреттерді көргіңіз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2469 in / 135 out tokens, est. $0.000722)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Біз қала бойынша және облысқа жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады. Астанаға жеткізу туралы ақпарат білім базасында жоқ, сондықтан бұл сұрақты нақтылау үшін эскалация қажет.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2203 in / 194 out tokens, est. $0.001146)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: false · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу и области осуществляется. Срок и стоимость зависят от адреса. При заказе на сумму от 20 000 ₸ доставка бесплатная.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2203 in / 98 out tokens, est. $0.000260)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Иә, біз Астанаға жеткіземіз. Алматы қаласындағы жеткізу 1–3 дня аралығында жүреді және құны 1 500 ₸ по Алматы. Егер сіздің тапсырысыңыз 20 000 ₸ жетсе, жеткізу тегін болады. Тапсырыс беру үшін қандай тауарды таңдағаныңызды және жеткізу мекенжайыңызды айтыңызшы.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2454 in / 201 out tokens, est. $0.000745)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2225 in / 93 out tokens, est. $0.000900)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по городу Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2225 in / 97 out tokens, est. $0.000261)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Если сумма вашего заказа будет от 20 000 ₸, доставка станет бесплатной. Срок доставки — 1–3 дня. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2473 in / 127 out tokens, est. $0.000720)
- contract pass: **true** · model-behavior pass: **true**

## lang-canary-v4-kk

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! «Demo Shop» интернет-дүкеніне қош келдіңіз! Кофемашина DeLonghi құны 129 900 ₸. Ол қазіргі уақытта В наличии. Сізді қызықтыратын басқа тауарлар бар ма?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2012 in / 141 out tokens, est. $0.000258)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы 129 900 ₸ құрайды. Ол қазір В наличии бар. Тапсырыс беру үшін немесе басқа сұрақтарыңыз болса, хабарласыңыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2012 in / 160 out tokens, est. $0.001004)
- contract pass: **true** · model-behavior pass: **true**

### 2. price question, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi құны 129 900 ₸. Ол қазір В наличии. Сізге оның бірнеше суреттерін көрсетейін бе?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2342 in / 126 out tokens, est. $0.000684)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны Алматы бойынша 1 500 ₸ по Алматы құрайды, ал жеткізу мерзімі 1–3 дня аралығында. Егер тапсырыс сомасы 20 000 ₸ жоғары болса, жеткізу тегін болады. Тапсырыс беру үшін қандай тауарға қызығушылық танытып отырсыз?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2018 in / 167 out tokens, est. $0.001023)
- contract pass: **true** · model-behavior pass: **true**

### 3. delivery cost + time, Kazakh — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- invented digits: 1, 3, 20, 000
- requires met: false · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы және облысы бойынша жеткіземіз. Жеткізу құны 1 500 ₸ по Алматы және әдетте 1-3 күн ішінде жетеді. Алматы қаласы бойынша 20 000 ₸-ден жоғары тапсырыстарға жеткізу тегін. Сізді қызықтыратын тауар бар ма?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2018 in / 165 out tokens, est. $0.000268)
- contract pass: **true** · model-behavior pass: **false**

### 3. delivery cost + time, Kazakh — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Жеткізу құны 1 500 ₸ по Алматы, ал жеткізу мерзімі 1–3 дня. Егер тапсырыс сомасы 20 000 ₸ жетсе, жеткізу тегін болады. Қандай тауарға тапсырыс бергіңіз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2191 in / 124 out tokens, est. $0.000642)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! DeLonghi кофемашинасының бағасы — 129 900 ₸. Ол қазір В наличии бар. Тапсырыс беру үшін немесе қосымша ақпарат алу үшін хабарласыңыз.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2024 in / 149 out tokens, est. $0.000980)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi құны 129 900 ₸. Ол қазіргі уақытта қоймада бар. Сізді басқа қандай да бір тауарлар немесе қызметтер қызықтыра ма?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2024 in / 129 out tokens, est. $0.000254)
- contract pass: **true** · model-behavior pass: **true**

### 20. mixed Kazakh/Russian message — reply in the dominant language (the question itself is Kazakh) — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Кофемашина DeLonghi құны 129 900 ₸. Ол қазір В наличии. Қосымша суреттерді немесе сипаттаманы көргіңіз келе ме?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2358 in / 138 out tokens, est. $0.000693)
- contract pass: **true** · model-behavior pass: **true**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз тек Алматы қаласы мен облысы бойынша жеткіземіз. Жеткізу мерзімі мен құны мекенжайға байланысты болады, ал белгілі бір сомадан асқан тапсырыс үшін жеткізу тегін болады. Алматы бойынша жеткізу құны 1 500 ₸ по Алматы құрайды, ал 20 000 ₸ бастап тегін. Жеткізу мерзімі әдетте 1–3 дня алады. Тағы не сұрағыңыз келеді?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2012 in / 213 out tokens, est. $0.001136)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Біз Алматы қаласы мен облысына жеткіземіз. Астанаға жеткізу шарттарын нақтылау үшін, біздің жеткізу аймақтары картасын қарап көруіңізге болады. Сізге қандай тауарлар қызықтырады?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2012 in / 149 out tokens, est. $0.000261)
- contract pass: **true** · model-behavior pass: **false**

### 21. off-KB city coverage, Kazakh — escalation must stay Kazakh with no invented claim — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Сәлеметсіз бе! Иә, біз Астанаға да жеткіземіз. Жеткізу құны мен мерзімі мекенжайыңызға байланысты болады. Егер тапсырыс сомасы 20 000 ₸ жетсе, жеткізу тегін болады. Сізге қандай тауар қызықтырады? Оны айтып берсеңіз, мен бағасы мен қолжетімділігін айтып, тапсырысты рәсімдеуге көмектесемін.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (2343 in / 210 out tokens, est. $0.000719)
- contract pass: **true** · model-behavior pass: **false**

## lang-canary-v4-ru

### 1. price question, Russian — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она сейчас в наличии. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1520 in / 100 out tokens, est. $0.000192)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина стоит 129 900 ₸. Она В наличии.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1520 in / 95 out tokens, est. $0.000694)
- contract pass: **true** · model-behavior pass: **true**

### 1. price question, Russian — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Кофемашина DeLonghi стоит 129 900 ₸. Она В наличии. Хотите посмотреть фото или видео-обзор?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1607 in / 100 out tokens, est. $0.000475)
- contract pass: **true** · model-behavior pass: **true**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. К сожалению, в Астану доставки нет. Могу помочь с чем-то еще?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 87 out tokens, est. $0.000675)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: false · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Мы доставляем по городу и области. Стоимость и сроки зависят от адреса. При заказе от 20 000 ₸ доставка бесплатная. Хотите узнать точную стоимость доставки для вашего адреса?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1524 in / 111 out tokens, est. $0.000197)
- contract pass: **true** · model-behavior pass: **false**

### 9. off-KB city coverage — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- **reply_text contains a forbidden phrase:** "доставку только по"
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: false · units ok: true · media count ok: true
- injected text: Здравствуйте! К сожалению, на данный момент мы осуществляем доставку только по городу Алматы и области. Если у вас есть вопросы по товарам или доставке в пределах Алматы, буду рад помочь!
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1609 in / 129 out tokens, est. $0.000487)
- contract pass: **true** · model-behavior pass: **false**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Стоимость доставки по Алматы составляет 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной.
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1550 in / 94 out tokens, est. $0.000700)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:google/gemini-2.5-flash-lite

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. При заказе от 20 000 ₸ доставка будет бесплатной. Хотите оформить заказ?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1550 in / 101 out tokens, est. $0.000195)
- contract pass: **true** · model-behavior pass: **true**

### 16. follow-up with history, needs delivery cost — openrouter:deepseek/deepseek-v3.2-exp

- parse ok: true
- contract fields ok: true
- requires met: true · media met: true · escalate met: true · language met: true · no-invented-answer met: true · units ok: true · media count ok: true
- injected text: Доставка по Алматы стоит 1 500 ₸ по Алматы. Если сумма заказа превысит 20 000 ₸, доставка будет бесплатной. Срок доставки — 1–3 дня. Хотите оформить заказ на кофемашину?
- injection clean (no brace survived, whether blocked or not): true
- cost basis: measured_split (1635 in / 124 out tokens, est. $0.000492)
- contract pass: **true** · model-behavior pass: **true**

