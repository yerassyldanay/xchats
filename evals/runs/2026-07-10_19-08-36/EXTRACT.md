# Extraction eval — 2026-07-10 19:08

Eval 1: file -> extracted information. Every check is deterministic (no LLM judge). Raw per-(case,model) outputs are saved alongside this file in `extract_outputs/`.

## infographic

| Model | Result | Cost | Details |
|---|---|---|---|
| google/gemini-2.5-flash | FAIL (4/5 checks) | unknown_pricing | failed: text_contains_all |

**google/gemini-2.5-flash failures:**

- `text_contains_all`: missing: рост

raw extracted_text:
```
Мы добавили новые тарифные планы xpayment.kz Выберите подходящий тариф для приема оплат через Kaspi Pay ПРОБНЫЙ Бесплатно 3 бесплатных дня Для первого знакомства СТАРТ 10 000 Т/мес до 250 платежей/мес Для небольшого бизнеса * основной POCT 25 000 Т/мес до 2 000 платежей/мес Оптимальный выбор МАСШТАБ 60 000 Т/мес безлимит платежей Для большого бизнеса до 5 виртуальных касс безлимит вебхуков QR/Deeplink/ Инвойс полный доступ к панели • Подробнее на хpayment.kz
```

