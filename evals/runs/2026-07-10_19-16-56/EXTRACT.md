# Extraction eval — 2026-07-10 19:17

Eval 1: file -> extracted information. Every check is deterministic (no LLM judge). Raw per-(case,model) outputs are saved alongside this file in `extract_outputs/`.

## screenshot

| Model | Result | Cost | Details |
|---|---|---|---|
| google/gemini-2.5-flash | PASS (4/4 checks) | unknown_pricing | all checks passed |
| google/gemini-2.5-flash-lite | FAIL (2/4 checks) | $0.00036 | failed: field:content_kind, field:visibility_suggestion |

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

