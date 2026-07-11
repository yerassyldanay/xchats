# Extraction eval — 2026-07-10 19:52

Eval 1: file -> extracted information. Every check is deterministic (no LLM judge). Raw per-(case,model) outputs are saved alongside this file in `extract_outputs/`.

## how-it-works

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | FAIL (2/5 checks) | $0.00414 | failed: field:content_kind, text_contains_all, no_invented_numbers |
| anthropic/claude-haiku-4.5 | FAIL (3/5 checks) | unknown_pricing | failed: text_contains_all, no_invented_numbers |
| google/gemini-2.5-flash | FAIL (3/5 checks) | unknown_pricing | failed: text_contains_all, no_invented_numbers |
| google/gemini-2.5-flash-lite | FAIL (3/5 checks) | $0.00044 | failed: text_contains_all, no_invented_numbers |

**openai/gpt-4o-mini failures:**

- `field:content_kind`: want "infographic", got "tutorial"
- `text_contains_all`: missing: накоплено 320, green basket
- `no_invented_numbers`: not in the allowed list: 1, 2, 3, 4, 6, 7

raw extracted_text:
```
Как работает xpayment
Флоу интеграции Kaspi Pay для вашего продукта
1
Kaspi Pay кассир
У вас есть Kaspi Pay кассир для приёма платежей.
2
Виртуальный кассир xpayment
Номер кассира 123456789012
Код подтверждения
Kaspi Pay кассир теперь в xpayment как оригинальный кассир
3
xpayment API
API
создание платежей
статусы
уведомления
4
Ваш продукт
интернет-магазин
Telegram бот
CRM
мобильное приложение
5
Клиент видит оплату в вашем продукте
QR
Deeplink
Оформление заказа
Клиенте 24 500 ₸
Заказ #A-2041
К оплате 24 500 ₸
6
Клиент видит оплату в Kaspi Pay
18:43
Kaspi Pay
Сканируйте QR-код.
Заказ #A-2041 
К оплате 24 500 ₸
7 
Успешная оплата 
платеж оплачен 
уведомление в ваш продукт 
заказ оплачен
```

**anthropic/claude-haiku-4.5 failures:**

- `text_contains_all`: missing: статусы, накоплено 320, green basket
- `no_invented_numbers`: not in the allowed list: 1, 2, 3, 4, 6, 11, 250, 7

raw extracted_text:
```
xpayment
Как работает xpayment
Флоу интеграции Kaspi Pay для вашего продукта
1 Kaspi Pay кассир
У вас есть Kaspi Pay кассир для приема платежей.
2 Виртуальный кассир xpayment
Номер кассира
123456789012
Код подтверждения
•••••••
Kaspi Pay кассир теперь в купчете как виртуальный кассир
3 xpayment API
создание платежей
статусум
уведомления
4 Ваш продукт
интернет-магазин
Telegram бот
CRM
мобильное приложение
5 Клиент видит оплату в вашем продукте
QR
Оформление заказа
К оплате
24 500 Т
Оформление QR в приложении Kaspi.kz
Deeplink
Заказ #А-2041
К оплате
24 500 Т
Оплатить через Kaspi
6 Клиент видит оплату в Kaspi Pay
11:43
Kaspi Pay
Сканируйте QR-код
Заказ #А-2041
24 500 Т
Промокод
ТОО ООО ООО
Сумма
24 500 Т
Получить Бонусы
Начисление 250 Т
К оплате
24 500 Т
Скрыть сумму
Способы оплаты
Kaspi Pay
•••• 1234
24 500 Т
Kaspi Gold
•••• 5678
24 500 Т
Оплатить 24 500 Т
7 Успешная оплата
платеж оплачен
уведомление в ваш продукт
заказ оплачен
```

**google/gemini-2.5-flash failures:**

- `text_contains_all`: missing: как работает xpayment
- `no_invented_numbers`: not in the allowed list: 1, 2, 3, 4, 6, 7

raw extracted_text:
```
xpayment
Как работает храyment
Флоу интеграции Kaspi Рау для вашего продукта
1 Kaspi Pay
кассир
2 Виртуальный кассир
xpayment
Номер кассира
123456789012
Код подтверждения
3 xpayment API
4
Ваш продукт
интернет-магазин
→
API
→
<
Telegram бот
У вас есть Kaspi Pay
кассир для приёма
платежей.
↓
Kaspi Рау кассир
теперь в храутент
как виртуальный кассир
• создание платежей
• статусы
• уведомления
CRM
мобильное
приложение
5 Клиент видит оплату в вашем продукте
6 Клиент видит оплату в Kaspi Pay
QR
Оформление заказ
Копла
Deeplink
24 500 T
Заказ #А-2041
Коплате
24 500 〒
18:43
SG
Kaspi Pay
Сканируйте QR-код
X
Заказ №А-2041
24 500 T
Отсканируйте QR
еним Каз
• Оплатить через Каѕрі
Продавец
Сумма
TOO Green Basket
24 500
•
Потратить Бонусы
Накоплено 320
К оплате
7 Успешная оплата
• платеж оплачен
• уведомление в ваш продукт
заказ оплачен
24 500 T
Скрыть суммы
Способы оплаты
O Kaspi Pay
1234
24 500 T
→
Kaspi Gold
5678
24 500 T
Оплатить 24 500 T
```

**google/gemini-2.5-flash-lite failures:**

- `text_contains_all`: missing: как работает xpayment
- `no_invented_numbers`: not in the allowed list: 1, 2, 3, 4, 6, 7

raw extracted_text:
```
xpayment
Как работает храyment
Флоу интеграции Kaspi Рау для вашего продукта
1 Kaspi Pay
кассир
2 Виртуальный кассир
xpayment
Номер кассира
123456789012
Код подтверждения
У вас есть Kaspi Pay
кассир для приёма
платежей.
Kaspi Рау кассир
теперь в храутент
как виртуальный кассир
3 xpayment API
API
• создание платежей
• статусы
• уведомления
4
Ваш продукт
интернет-магазин
Telegram бот
CRM
мобильное
приложение
5 Клиент видит оплату в вашем продукте
QR
Deeplink
Оформление заказ
Коплате
24 500 T
Заказ #А-2041
Коплате
24 500 〒
Отсканируйте QR
в приложении Казрі.kz
• Оплатить через Каѕрі
6 Клиент видит оплату в Kaspi Pay
18:43
Kaspi Pay
Сканируйте QR-код
SG
X
Заказ №А-2041
24 500 T
Продавец
Сумма
TOO Green Basket
24 500
• Потратить Бонусы
Накоплено 320
К оплате
24 500 T
Способы оплаты
Скрыть суммы
O Kaspi Pay
1234
24 500 T
Kaspi Gold
5678
24 500 T
7 Успешная оплата
• платеж оплачен
• уведомление в ваш продукт
заказ оплачен
Оплатить 24 500 T
```

## how-to-cachier

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | FAIL (4/5 checks) | $0.00419 | failed: no_invented_numbers |
| anthropic/claude-haiku-4.5 | FAIL (3/5 checks) | unknown_pricing | failed: text_contains_all, no_invented_numbers |
| google/gemini-2.5-flash | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash-lite | FAIL (3/5 checks) | $0.00055 | failed: field:content_kind, no_invented_numbers |

**openai/gpt-4o-mini failures:**

- `no_invented_numbers`: not in the allowed list: 132 1, 777, 7 2, 15 3, 15 1

raw extracted_text:
```
1 Откройте «Точки продаж и кассиры» На главном экране Kaspi Pay нажмите на блок «Точки продаж и кассиры». 245 600 ₸ 2 Выберите точку продаж Выберите точку продаж из списка, в которой вы хотите добавить кассира. Точки продаж и кассиры Мой магазин ул. Абая, 132 1 кассир Магазин №2 ул. Аль-Фараби, 777/7 2 кассира Магазин №3 ул. Толе би, 15 3 Добавьте кассира • Введите ФИО кассира • Укажите номер телефона • Роль: Кассир (по умолчанию) • Нажмите «Сохранить» Добавьте кассира ФИО кассира Иванов Иван Иванович +7 777 123-45-67 Роль: Кассир Кассир сможет принимать платежи и работать с продажами. Сохранить Отменить Новый кассир появился в списке. Он получит SMS-приглашение и сможет войти в Kaspi Pay. Точки продаж и кассиры Мой магазин ул. Абая, 132 Иванов Иван Иванович +7 777 123-45-67 Петров Петр Петрович +7 777 000-11-22 Магазин №2 ул. Аль-Фараби, 777/7 2 кассира Магазин №3 ул. Толе би, 15 1 кассир Кассир добавлен Кассиру отправлено SMS-приглашение
```

**anthropic/claude-haiku-4.5 failures:**

- `text_contains_all`: missing: 77/7
- `no_invented_numbers`: not in the allowed list: 777

raw extracted_text:
```
Откройте «Точки продаж и кассиры»
На главном экране Kaspi Pay нажмите на блок «Точки продаж и кассиры».
9:41
Kaspi Pay
МП «Мой магазине»
Редакция за категории
245 600 ₸
Касса Помощник оператора
Счета на емаил
Точки продаж и кассиры
Магазины на Kaspi kz
Ссылки на емаил
Он-лайн
Монетальные выплаты на Kaspi Gold
Главная
Реквизит
Сообщение
Сервисы
Выберите точку продаж
Выберите точку продаж из списка, в которой вы хотите добавить кассира.
9:41
Точки продаж и кассиры
Мой магазин
ул. Абая, 132
1 кассир
Магазин №2
пр. Ал-Фараби, 777
2 кассира
Магазин №3
ул. Толе би, 15
1 кассир
Добавить точку продаж
Добавьте кассира
Введите ФИО кассира
Укажите номер телефона
Роль. Кассир (по умолчанию)
Нажмите «Сохранить»
9:41
Добавьте кассира
ФИО кассира
Иванов Иван Иванович
Номер телефона
+7 777 123-45-67
Роль
Кассир
Кассир сможет принимать платежи и работать с сервисом
Сохранить
Отменить
Кассир добавлен
Новый кассир появился в списке.
Он получит SMS-приглашение и сможет войти в Kaspi Pay.
9:41
Точки продаж и кассиры
Мой магазин
ул. Абая, 132
Иванов Иван Иванович
+7 777 123-45-67
Петров Петр Петрович
+7 777 000-11-22
Добавить кассира
Магазин №2
пр. Ал-Фараби, 777
2 кассира
Магазин №3
ул. Толе би, 15
1 кассир
Кассир добавлен
Кассиру отправлено SMS-приглашение
```

**google/gemini-2.5-flash-lite failures:**

- `field:content_kind`: want "tutorial", got "infographic"
- `no_invented_numbers`: not in the allowed list: 21, 12, 11, 4

raw extracted_text:
```
Откройте «Точки продаж и кассиры»
На главном экране Kaspi Рау нажмите
на блок «Точки продаж и кассиры».
Выберите точку продаж
Выберите точку продаж из списка,
в которой вы хотите добавить кассира.
9:41
Kaspi Pay
ИП «Мой магазин»>
Продажи за сек
245 600 т
Kaspi QR
Принимать
платени
Счета на
оплату
Точки продаж
и кассиры
Магазин
на Kaspi.kz
Ссылки
на оплату
Отчёты
Моментальные
выплаты
на Казрі Gold
G
Главном Платежи Сообщения Сервисы
3 Добавьте кассира
Введите ФИО кассира
Укажите номер телефона
Роль: Кассир (по умолчанию)
Нажмите «Сохранить»
9:41
< Добавьте кассира
ФИО кассира
Иванов Иван Иванович
Номер телефона
+7 777 123-45-67
Роль
Кассир
• Кассир сможет принимать платежи
и работать с продажами.
Сохранить
Отменить
9:41
< Точки продаж и кассиры
Мой магазин
ул. Абая, 132
21 кассир
Магазин №№2
пр. Аль-Фараби, 77/7
12 кассира
Магазин №3
ул. Толе би, 15
11 кассир
+ Добавить точку продаж
4 Кассир добавлен
Новый кассир появился в списке.
Он получит SMS-приглашение
и сможет войти в Kaspi Pay.
9:41
< Точки продаж и кассиры
Мой магазин
ул. Абая, 132
• Иванов Иван Иванович
+7 777 123-45-67
• Петров Петр Петрович
+7777 000-11-22
+ Добавить кассира
Магазин №№2
пр. Аль-Фараби, 77/7
1 2 кассира
Магазин №3
ул. Толе би, 15
11 кассир
• Кассир добавлен
Кассиру отправлено SMS-приглашение
```

## infographic

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | PASS (5/5 checks) | $0.00408 | all checks passed |
| anthropic/claude-haiku-4.5 | FAIL (4/5 checks) | unknown_pricing | failed: text_contains_all |
| google/gemini-2.5-flash | FAIL (4/5 checks) | unknown_pricing | failed: text_contains_all |
| google/gemini-2.5-flash-lite | FAIL (4/5 checks) | $0.00033 | failed: text_contains_all |

**anthropic/claude-haiku-4.5 failures:**

- `text_contains_all`: missing: масштаб, xpayment.kz

raw extracted_text:
```
Мы добавили новые тарифные планы xraument.kz

Выберите подходящий тариф для приема оплат через Kaspi Pay

ПРОБНЫЙ
Бесплатно
3 бесплатных дня
Для первого знакомства

СТАРТ
10 000 ₸/мес
до 250 платежей/мес
Для небольшого бизнеса

★ ОСНОВНОЙ
РОСТ
25 000 ₸/мес
до 2 000 платежей/мес
Оптимальный выбор

МАШТАБ
60 000 ₸/мес
безлимит платежей
Для большого бизнеса

до 5 виртуальных касс
безлимит вебхуков
QR / Deeplink / Инвойс
полный доступ к панели

Подробнее на xraument.kz
```

**google/gemini-2.5-flash failures:**

- `text_contains_all`: missing: рост

raw extracted_text:
```
Мы добавили новые тарифные планы xpayment.kz Выберите подходящий тариф для приема оплат через Kaspi Pay ПРОБНЫЙ Бесплатно 3 бесплатных дня Для первого знакомства * основной POCT 25 000 Т/мес до 2 000 платежей/мес Оптимальный выбор СТАРТ 10 000 Т/мес до 250 платежей/мес Для небольшого бизнеса МАСШТАБ 60 000 Т/мес безлимит платежей Для большого бизнеса до 5 виртуальных касс безлимит вебхуков QR/Deeplink/ Инвойс полный доступ к панели • Подробнее на хpayment.kz
```

**google/gemini-2.5-flash-lite failures:**

- `text_contains_all`: missing: рост

raw extracted_text:
```
Мы добавили
новые тарифные планы
xpayment.kz
Выберите подходящий тариф
для приема оплат через Kaspi Pay
ПРОБНЫЙ
Бесплатно
3 бесплатных дня
Для первого знакомства
СТАРТ
10 000 Т/мес
до 250 платежей/мес
Для небольшого бизнеса
* основной
POCT
25 000 Т/мес
до 2 000 платежей/мес
Оптимальный выбор
МАСШТАБ
60 000 Т/мес
безлимит платежей
Для большого бизнеса
до 5
виртуальных касс
безлимит
вебхуков
QR/Deeplink/
Инвойс
полный доступ
к панели
• Подробнее на хpayment.kz
```

## product-photo

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | FAIL (5/7 checks) | $0.00399 | failed: field:media_role_hint, identify_contains_any |
| anthropic/claude-haiku-4.5 | FAIL (6/7 checks) | unknown_pricing | failed: text_contains_all |
| google/gemini-2.5-flash | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash-lite | FAIL (5/7 checks) | $0.00031 | failed: field:media_role_hint, no_invented_numbers |

**openai/gpt-4o-mini failures:**

- `field:media_role_hint`: want "gallery", got "none"
- `identify_contains_any`: none of: дрель, сверлильн, станок, magnetic drill, drill

raw extracted_text:
```
Pellis
ON
OFF
ELECTRICAL SWITCH
MANUAL SWITCH

```

**anthropic/claude-haiku-4.5 failures:**

- `text_contains_all`: missing: llis

raw extracted_text:
```
ilis
```

**google/gemini-2.5-flash-lite failures:**

- `field:media_role_hint`: want "gallery", got "none"
- `no_invented_numbers`: not in the allowed list: 00000

raw extracted_text:
```
llis
ON
OFF
ELECTRIC DRILL SWITCH
MAGNET SWITCH
ATTENTION
00000
```

## screenshot

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | FAIL (3/4 checks) | $0.00412 | failed: text_contains_all |
| anthropic/claude-haiku-4.5 | FAIL (2/4 checks) | unknown_pricing | failed: text_contains_all, no_invented_numbers |
| google/gemini-2.5-flash | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash-lite | FAIL (3/4 checks) | $0.00039 | failed: field:content_kind |

**openai/gpt-4o-mini failures:**

- `text_contains_all`: missing: 702 976-65-09

raw extracted_text:
```
Сложно разобраться, или не хотите тратить время?

Оставьте номер телефона. Мы перезвоним, уточним ваш сценарий и подскажем, как подключить Kaspi Pay API без лишней ручной работы.

Оставить номер → WhatsApp

Перезвоним в течение 1 часа

xPayment
Сервис автоматизации онлайн-платежей для бизнеса в Казахстане. Не банк, не платежная организация, а партнер Kaspi.

© 2026 ИП «XGroup». Все права защищены.

РЕКВИЗИТЫ
ИП «XGroup»
Республика Казахстан
г. Шымкент, ул. Аргынбеков, 29/4

ПОДДЕРЖКА
WhatsApp: +7702-976-65-09
support@xpayment.kz

ДОКУМЕНТЫ
Публичная оферта
Пользовательское соглашение
Политика конфиденциальности
Согласие на обработку ПДн
Политика cookie
```

**anthropic/claude-haiku-4.5 failures:**

- `text_contains_all`: missing: 702 976-65-09
- `no_invented_numbers`: not in the allowed list: 7 702 976-45-09, 2024

raw extracted_text:
```
Сложно разобраться, или не хотите тратить время?

Оставьте номер телефона. Мы перезвоним, уточним ваш сценарий и подскажем, как подключить Kaspi Pay API без лишней ручной работы.

Оставить номер
WhatsApp
Перевозоним в течение 1 часа

xPayment
Сервис автоматизации онлайн-платежей для бизнеса в Казахстане. Не банк, не платежная организация, не партнер Kaspi.

РЕКВИЗИТЫ
ИП «XGroup»
Республика Казахстан
г. Шымкент, ул. АргынБеков, 29/4

ПОДДЕРЖКА
WhatsApp: +7 702 976-45-09
support@xpayment.kz

ДОКУМЕНТЫ
Публичная оферта
Пользовательское соглашение
Политика конфиденциальности
Согласие на обработку ПДн
Политика cookie

© 2024 ИП «XGroup». Все права защищены.

xPayment — это веб-сервис автоматизации онлайн-платежей для предпринимателей. Мы не являемся банком, платежной организацией или официальным партнером АО «Касpi Банк». Денежные средства покупателей никогда не поступают на счета или карты xPayment — оплата уходит напрямую по торговую точку Пользователя в инфраструктуре Kaspi.
```

**google/gemini-2.5-flash-lite failures:**

- `field:content_kind`: want "screenshot", got "infographic"

raw extracted_text:
```
Сложно разобраться,
или не хотите тратить время?
Оставьте номер телефона. Мы перезвоним, уточним ваш сценарий и подскажем, как
подключить Kaspi Pay API без лишней ручной работы.
Оставить номер →
В Перезвоним в течение 1 часа
WhatsApp
xPayment
Сервис автоматизации онлайн-платежей для
бизнеса в Казахстане. Не банк, не платёжная
организация, не партнёр Кaspi.
РЕКВИЗИТЫ
ИП «XGroup»
Республика Казахстан
г. Шымкент, ул. Аргынбеков, 29/4
ПОДДЕРЖКА
WhatsApp: +7702 976-65-09
support@xpayment.kz
ДОКУМЕНТЫ
Публичная оферта
Пользовательское соглашение
Политика конфиденциальности
Согласие на обработку ПДн
Политика сookie
© 2026 ИП «XGroup». Все права защищены.
xPayment - это веб-сервис автоматизации онлайн-платежей для предпринимателей. Мы не
являемся банком, платёжной организацией или официальным партнёром АО «Каспи Банк».
Денежные средства покупателей никогда не поступают на счета или карты хPayment - оплата
уходит напрямую на торговую точку Пользователя в инфраструктуре Kaspi.
```

