# Extraction eval — 2026-07-10 19:04

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
Мы добавили новые тарифные планы xpayment.kz Выберите подходящий тариф для приема оплат через Kaspi Pay ПРОБНЫЙ Бесплатно 3 бесплатных дня Для первого знакомства СТАРТ 10 000 Т/мес до 250 платежей/мес Для небольшого бизнеса * основной POCT 25 000 Т/мес до 2 000 платежей/мес Оптимальный выбор МАСШТАБ 60 000 Т/мес безлимит платежей Для большого бизнеса до 5 виртуальных касс безлимит вебхуков QR/Deeplink/ Инвойс полный доступ к панели Подробнее на xpayment.kz
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
60 000 т/мес
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
| openai/gpt-4o-mini | PARSE FAILED | $0.00419 | unexpected end of JSON input |
| anthropic/claude-haiku-4.5 | FAIL (6/7 checks) | unknown_pricing | failed: identify_contains_all |
| google/gemini-2.5-flash | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash-lite | FAIL (6/7 checks) | $0.00028 | failed: identify_contains_all |

**anthropic/claude-haiku-4.5 failures:**

- `identify_contains_all`: missing: follis

raw extracted_text:
```
Ilis
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
| openai/gpt-4o-mini | FAIL (2/4 checks) | $0.00409 | failed: text_contains_all, no_invented_numbers |
| anthropic/claude-haiku-4.5 | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash | PARSE FAILED | unknown_pricing | unexpected end of JSON input |
| google/gemini-2.5-flash-lite | PARSE FAILED | $0.00000 | unexpected end of JSON input |

**openai/gpt-4o-mini failures:**

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
Серия автоматизации онлайн-платежей для бизнеса в Казахстане. Не банк, не платежная организация, мы партнер Kaspi.

РЕКВИЗИТЫ
ИП «xGroup»
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

© 2026 ИП «xGroup». Все права защищены.
```

