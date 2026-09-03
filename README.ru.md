<div align="center">

# xchats

**Self-hosted омниканальный инбокс с AI без галлюцинаций.**
WhatsApp, Telegram, Instagram и Messenger — в одном общем инбоксе. Ассистент
готовит каждый ответ по вашей базе знаний, а отправляет его всегда человек.

[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](backend)
[![Vue 3](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](frontend)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)](docs/release/data-locations-and-privacy.md)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](deploy)
[![CI](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml/badge.svg)](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml)

[English](README.md) · **Русский** · [Қазақша](README.kk.md)

*Этот файл — перевод. При расхождениях [`README.md`](README.md) на английском считается основным.*

</div>

![Обзор xchats](docs/images/overview-bento.png)

**[Полный визуальный тур и скриншоты →](docs/tour.md)** *(на английском)*

---

## Быстрый старт за 60 секунд

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make up && make seed-demo
```

Откройте **http://localhost:8081** — войдите как `admin@xchat.kz` /
`xchat-admin-change-me` (публичный пароль по умолчанию; смените его сразу
после первого входа).

## Главные преимущества

- **Ответы без галлюцинаций** — модель никогда сама не пишет цену, дату
  или контакт. Она подставляет плейсхолдер `{{token}}`, а бэкенд заменяет его
  на точное сохранённое значение — или черновик не проходит проверку и
  эскалируется человеку.
- **Человек в контуре — всегда** — каждый черновик лишь предложение.
  Ничего не уходит клиенту, пока агент не нажмёт «Отправить».
- **Один бинарник, один файл** — Go + встроенный SQLite. Не нужен ни
  Postgres, ни Redis, ни облачный сервис.
- **Любая модель** — OpenAI, Claude, Gemini, OpenRouter или локальная
  модель через Ollama; провайдер меняется в настройках, а не в коде.
- **Настройка прямо из ChatGPT / Claude** — MCP-коннектор позволяет
  LLM-клиенту прочитать ваши документы и подготовить изменения базы знаний
  на ваше рассмотрение.

## Как это работает

1. Клиент пишет вам в WhatsApp, Telegram, Instagram или Messenger.
2. Сообщение попадает в **один инбокс**, общий для всей команды.
3. Ассистент готовит ответ строго по одобренной базе знаний.
4. Человек проверяет, редактирует или отклоняет черновик — и отправляет сам.

Полное описание архитектуры — в [`plan/architecture.md`](plan/architecture.md);
механизм подстановки фактов показан в
[визуальном туре](docs/tour.md#2-grounded-knowledge-base--strict-token-replacement).

## Другие способы запуска

- **Из исходников** (для разработки): `make dev-backend` + `make dev-frontend`
  — см. [`docs/release/installation.md`](docs/release/installation.md).
- **Десктопное приложение** (Wails — Windows/macOS/Linux): `make desktop-build`
  — см. [`docs/desktop.md`](docs/desktop.md).
- `make help` покажет все доступные команды.

> [!WARNING]
> Канал WhatsApp подключается как WhatsApp Web (без комиссии Business API),
> но это неофициальный клиент — начните с номера, потерю которого вы можете
> себе позволить. Подробности — в [визуальном туре](docs/tour.md#1-team-inbox--omnichannel-sync).

## Документация

- [Визуальный тур](docs/tour.md) — каждый экран с пояснением *(EN)*
- [`docs/release/installation.md`](docs/release/installation.md) — все способы установки
- [`docs/desktop.md`](docs/desktop.md) — десктопное приложение
- [`plan/`](plan/) — журнал проектных решений, начиная с [`plan/README.md`](plan/README.md)
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — настройка окружения, соглашения, PR
- [`SECURITY.md`](SECURITY.md) — как сообщить об уязвимости

## Лицензия

[AGPL-3.0-only](LICENSE) — выбрана после того, как в зависимостях нашлась
GPL-3.0 библиотека (приходит транзитивно через интеграцию с WhatsApp),
статически слинкованная в бэкенд; подробности —
в [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
