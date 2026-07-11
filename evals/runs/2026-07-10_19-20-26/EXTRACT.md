# Extraction eval — 2026-07-10 19:22

Eval 1: file -> extracted information. Every check is deterministic (no LLM judge). Raw per-(case,model) outputs are saved alongside this file in `extract_outputs/`.

## infographic

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | PASS (5/5 checks) | $0.00405 | all checks passed |
| anthropic/claude-haiku-4.5 | FAIL (4/5 checks) | unknown_pricing | failed: text_contains_all |
| google/gemini-2.5-flash | FAIL (4/5 checks) | unknown_pricing | failed: text_contains_all |
| google/gemini-2.5-flash-lite | FAIL (4/5 checks) | $0.00030 | failed: text_contains_all |

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
| openai/gpt-4o-mini | PARSE FAILED | $0.00509 | unexpected end of JSON input |
| anthropic/claude-haiku-4.5 | PASS (7/7 checks) | unknown_pricing | all checks passed |
| google/gemini-2.5-flash | PASS (7/7 checks) | unknown_pricing | all checks passed |
| google/gemini-2.5-flash-lite | PASS (7/7 checks) | $0.00028 | all checks passed |

## screenshot

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | FAIL (1/4 checks) | $0.00409 | failed: field:visibility_suggestion, text_contains_all, no_invented_numbers |
| anthropic/claude-haiku-4.5 | FAIL (1/4 checks) | unknown_pricing | failed: field:visibility_suggestion, text_contains_all, no_invented_numbers |
| google/gemini-2.5-flash | PASS (4/4 checks) | unknown_pricing | all checks passed |
| google/gemini-2.5-flash-lite | FAIL (2/4 checks) | $0.00036 | failed: field:content_kind, field:visibility_suggestion |

**openai/gpt-4o-mini failures:**

- `field:visibility_suggestion`: want "invisible", got "visible"
- `text_contains_all`: missing: 702 976-65-09
- `no_invented_numbers`: not in the allowed list: 7702, 976

raw extracted_text:
```
Сложно разобраться, или не хотите тратить время?
Оставьте номер телефона. Мы перезвоним, уточним ваш сценарий и подскажем, как подключить Kaspi Pay API без лишней ручной работы.
Оставить номер  
WhatsApp
Перезвоним в течение 1 часа

xPayment
Сервис автоматизации онлайн-платежей для бизнеса в Казахстане. Не банк, не платежная организация, а партнер Kaspi.

2026 ИП «XGroup». Все права защищены.

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

- `field:visibility_suggestion`: want "invisible", got "visible"
- `text_contains_all`: missing: 702 976-65-09
- `no_invented_numbers`: not in the allowed list: 45, 2024

raw extracted_text:
```
Сложно разобраться, или не хотите тратить время?

Оставьте номер телефона. Мы перезвоним, уточним ваш сценарий и подскажем, как подключить Kaspi Pay API без лишней ручной работы.

Оставить номер →

WhatsApp

Перевозим в течение 1 часа

xPayment

Серия: автоматизация онлайн-платежей для бизнеса в Казахстане. Не банк, не платежная организация, не партнер Kaspi.

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
- `field:visibility_suggestion`: want "invisible", got "visible"

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

