# Extraction eval — 2026-07-10 19:14

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
Мы добавили новые тарифные планы xpayment.kz Выберите подходящий тариф для приема оплат через Kaspi Pay ПРОБНЫЙ Бесплатно 3 бесплатных дня Для первого знакомства * основной POCT 25 000 Т/мес до 2 000 платежей/мес Оптимальный выбор СТАРТ 10 000 Т/мес до 250 платежей/мес Для небольшого бизнеса МАСШТАБ 60 000 Т/мес безлимит платежей Для большого бизнеса до 5 виртуальных касс безлимит вебхуков QR/Deeplink/ Инвойс полный доступ к панели Подробнее на хpayment.kz
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
| openai/gpt-4o-mini | FAIL (3/7 checks) | $0.00396 | failed: field:media_role_hint, identify_contains_all, identify_contains_any, no_invented_numbers |
| anthropic/claude-haiku-4.5 | FAIL (6/7 checks) | unknown_pricing | failed: identify_contains_all |
| google/gemini-2.5-flash | FAIL (6/7 checks) | unknown_pricing | failed: identify_contains_all |
| google/gemini-2.5-flash-lite | FAIL (6/7 checks) | $0.00028 | failed: identify_contains_all |

**openai/gpt-4o-mini failures:**

- `field:media_role_hint`: want "gallery", got "none"
- `identify_contains_all`: missing: follis
- `identify_contains_any`: none of: дрель, сверлильн, станок, magnetic drill, drill
- `no_invented_numbers`: not in the allowed list: 220, 50, 1, 5

raw extracted_text:
```
Ettlis
ON
OFF
ELECTRICAL
MACHINE
220V
50Hz
1.5KW
1.5A


```

**anthropic/claude-haiku-4.5 failures:**

- `identify_contains_all`: missing: follis

raw extracted_text:
```
llis
```

**google/gemini-2.5-flash failures:**

- `identify_contains_all`: missing: follis

raw extracted_text:
```
llis
ON
OFF
ELECTRICAL SWITCH
MAGNET SWITCH
ATTENTION
```

**google/gemini-2.5-flash-lite failures:**

- `identify_contains_all`: missing: follis

raw extracted_text:
```
llis
ON
OFF
ELECTRIC DRILL SWITCH
MAGNET SWITCH
ATTENTION


```

## screenshot

| Model | Result | Cost | Details |
|---|---|---|---|
| openai/gpt-4o-mini | FAIL (3/4 checks) | $0.00409 | failed: field:visibility_suggestion |
| anthropic/claude-haiku-4.5 | FAIL (1/4 checks) | unknown_pricing | failed: field:visibility_suggestion, text_contains_all, no_invented_numbers |
| google/gemini-2.5-flash | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash-lite | PARSE FAILED | $0.00000 | unexpected end of JSON input |

**openai/gpt-4o-mini failures:**

- `field:visibility_suggestion`: want "invisible", got "visible"

raw extracted_text:
```
Сложно разобраться, или не хотите тратить время?

Оставьте номер телефона. Мы перезвоним, уточним ваш сценарий и подскажем, как подключить Kaspi Pay API без лишней ручной работы.

Оставить номер →  WhatsApp

Перезвоним в течение 1 часа

xPayment

Сервис автоматизации онлайн-платежей для бизнеса в Казахстане. Не банк, не платёжная организация, не партнёр Kaspi.

2026 ИП «XGroup». Все права защищены.

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

Оставить номер
WhatsApp
Перевозим в течение 1 часа

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

