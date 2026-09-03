<div align="center">

# xchats

**Галлюцинациясыз AI-мен self-hosted омниарналы inbox.**
WhatsApp, Telegram, Instagram және Messenger — бір ортақ inbox-та. Көмекші
әр жауапты сіздің білім қорыңыз бойынша дайындайды, ал жібереді әрдайым адам.

[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](backend)
[![Vue 3](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](frontend)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite&logoColor=white)](docs/release/data-locations-and-privacy.md)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](deploy)
[![CI](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml/badge.svg)](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml)

[English](README.md) · [Русский](README.ru.md) · **Қазақша**

*Бұл файл — аударма. Алшақтық болған жағдайда негізгі құжат ағылшын тіліндегі [`README.md`](README.md) болып саналады.*

</div>

![xchats шолуы](docs/images/overview-bento.png)

**[Толық визуалды тур және скриншоттар →](docs/tour.md)** *(ағылшын тілінде)*

---

## 60 секундтық жылдам бастау

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make up && make seed-demo
```

**http://localhost:8081** ашыңыз — `admin@xchat.kz` / `xchat-admin-change-me`
(жалпыға белгілі әдепкі құпиясөз) арқылы кіріңіз; бірінші кіргеннен кейін
бірден ауыстырыңыз.

## Басты мүмкіндіктер

- **Галлюцинациясыз жауаптар** — модель баға, күн немесе байланыс
  деректерін өзі ешқашан жазбайды. Ол `{{token}}` орынбасарын шығарады, ал
  бэкенд оны сақталған нақты мәнге ауыстырады — немесе жоба тексеруден
  өтпей, адамға беріледі.
- **Адам әрдайым бақылауда** — әр жоба тек ұсыныс. Агент «Жіберу»
  батырмасын баспайынша, клиентке ештеңе жетпейді.
- **Бір бинарник, бір файл** — Go + ендірілген SQLite. Postgres те,
  Redis те, бұлттық қызмет те қажет емес.
- **Кез келген модель** — OpenAI, Claude, Gemini, OpenRouter немесе
  Ollama арқылы жергілікті модель; провайдерді кодта емес, баптауларда
  ауыстырасыз.
- **ChatGPT / Claude арқылы баптау** — MCP-коннекторы LLM-клиентке
  құжаттарыңызды оқып, білім қорына өзгерістерді сіздің қарауыңызға
  дайындауға мүмкіндік береді.

## Бұл қалай жұмыс істейді

1. Клиент сізге WhatsApp, Telegram, Instagram немесе Messenger арқылы жазады.
2. Хабар бүкіл командаға ортақ **бір inbox-қа** түседі.
3. Көмекші тек бекітілген білім қоры бойынша жауап дайындайды.
4. Адам жобаны тексереді, өңдейді немесе алып тастайды — және өзі жібереді.

Толық архитектура сипаттамасы — [`plan/architecture.md`](plan/architecture.md)
файлында; фактілерді қою тетігі
[визуалды турда](docs/tour.md#2-grounded-knowledge-base--strict-token-replacement)
көрсетілген.

## Іске қосудың басқа жолдары

- **Бастапқы кодтан** (әзірлеу үшін): `make dev-backend` + `make dev-frontend`
  — қараңыз [`docs/release/installation.md`](docs/release/installation.md).
- **Десктоп қолданба** (Wails — Windows/macOS/Linux): `make desktop-build`
  — қараңыз [`docs/desktop.md`](docs/desktop.md).
- `make help` барлық қолжетімді командаларды көрсетеді.

> [!WARNING]
> WhatsApp арнасы WhatsApp Web сияқты қосылады (Business API комиссиясыз),
> бірақ бұл бейресми клиент — жоғалтуға дайын нөмірден бастаңыз. Толығырақ —
> [визуалды турда](docs/tour.md#1-team-inbox--omnichannel-sync).

## Құжаттама

- [Визуалды тур](docs/tour.md) — әр экранның түсіндірмесімен *(EN)*
- [`docs/release/installation.md`](docs/release/installation.md) — барлық орнату жолдары
- [`docs/desktop.md`](docs/desktop.md) — десктоп қолданба
- [`plan/`](plan/) — жобалау шешімдерінің журналы, [`plan/README.md`](plan/README.md) файлынан бастаңыз
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — орта баптау, конвенциялар, PR
- [`SECURITY.md`](SECURITY.md) — осалдық туралы қалай хабарлау керек

## Лицензия

[AGPL-3.0-only](LICENSE) — WhatsApp интеграциясы арқылы транзитивті түрде
келетін GPL-3.0 тәуелділігі бэкендке статикалық сілтенгені анықталғаннан
кейін таңдалды; толығырақ — [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)
файлында.
