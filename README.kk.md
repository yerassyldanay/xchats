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

![xchats шолуы](docs/images/social-preview.png)

---

## 60 секундтық жылдам бастау

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make up
make seed
```

**http://localhost:8081** ашыңыз — `admin@xchat.kz` / `xchat-admin-change-me`
(жалпыға белгілі әдепкі құпиясөз) арқылы кіріңіз; бірінші кіргеннен кейін
бірден ауыстырыңыз.
`make seed` жергілікті инстансты Qazan Home асүй техникасы дүкенінің демо
деректерімен толтырады; пәрменді қайта іске қосу қауіпсіз.

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

## Визуалды тур

Барлық скриншот деректермен толтырылған Qazan Home инстансынан алынған, макеттер емес.

### 1. Ортақ inbox

![Клиент диалогы және дайындалған AI жауабы](docs/images/inbox.png)

Барлық арна бір кезекке жиналады; AI жауабы адам тексергенше жоба күйінде қалады.

### 2. Арналар және автоматтандыру

![Қосылған арналар және олардың күйі](docs/images/channels.png)
![WhatsApp, Telegram, WhatsApp Cloud, Instagram немесе Messenger таңдау](docs/images/channel-connect.png)

Әр тіркелгіде күй тексеруі және жеке автоматтандыру режимі бар.

### 3. Білім қоры

![Білім қорындағы нақты тауар суреттері, атаулары мен бағалары бар каталог](docs/images/knowledge-base.png)

Тауарлар, бағалар, жеткізу және ережелер — көмекші қолданатын жалғыз фактілер. Сандар `{{token}}` арқылы қордан қойылады; толығырақ [сызбада](docs/images/grounding.svg).

### 4. Жобалар және ChatGPT / Claude

![Тауар бағасының өзгерісін дейін/кейін диффі ретінде көрсететін Жоба беті](docs/images/draft-staging.png)
![ChatGPT және Claude MCP коннекторын баптау](docs/images/mcp-connect.png)

Файлдар, сілтемелер және 13 MCP құралы бір диффі бар жобаға жазады. Жариялауды адам орындайды.

### 5. Білім қорының көмекшісі

![Каталог туралы сұрақтар тарихы бар жеке көмекші](docs/images/assistant.png)

Оператор жұмыс деректері мен жарияланбаған өзгерістерді сұрай алады; бұл хабарлар клиентке жіберілмейді.

### 6. CRM және тапсырмалар

![Мәртебесі, тегтері және арналық сәйкестендіргіштері бар клиенттер торшасы](docs/images/customers.png)
![Мерзімі өткен, Бүгін, Ертең және Кейін топтарына бөлінген тапсырмалар тақтасы](docs/images/followups.png)

Профиль арналарды, мәртебені, тегтер мен жазбаларды біріктіреді; тапсырмалар мерзімі бойынша бөлінеді.

### 7. Науқандар

![Жеткізу барысын тікелей көрсететін науқандар тізімі](docs/images/campaigns.png)

Алушыларды импорттап, жіберу қарқынын орнатыңыз және әр жеткізу мен жауапты бақылаңыз.

### 8. Симулятор және тесттер

![Көмекшіні тексеруге арналған мысал сұрақтары бар симулятор беті](docs/images/simulator.png)

Нақты арнасыз жұмыс қорын немесе жобаны тексеріп, модельдерді [`evals/`](evals/) арқылы салыстырыңыз.

### 9. Баптаулар

![Команда және қолданбаның басқа баптаулары](docs/images/settings.png)

Модельдер, парсерлер, мониторинг, қашықтан қол жеткізу, арналар, команда және сақтық көшірмелер UI арқылы бапталады.

## Бұл қалай жұмыс істейді

1. Клиент сізге WhatsApp, Telegram, Instagram немесе Messenger арқылы жазады.
2. Хабар бүкіл командаға ортақ **бір inbox-қа** түседі.
3. Көмекші тек бекітілген білім қоры бойынша жауап дайындайды.
4. Адам жобаны тексереді, өңдейді немесе алып тастайды — және өзі жібереді.

Толық архитектура сипаттамасы — [`docs/overview.md`](docs/overview.md)
файлында; фактілерді қою тетігі
[визуалды турда](#2-сенімді-білім-қоры-және-токендерді-қатаң-ауыстыру)
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
> [визуалды турда](#1-ортақ-inbox-және-омниарналы-синхрондау).

## Құжаттама

- [`docs/overview.md`](docs/overview.md) — өнім және архитектуралық шолу
- [`docs/release/installation.md`](docs/release/installation.md) — барлық орнату жолдары
- [`docs/desktop.md`](docs/desktop.md) — десктоп қолданба
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — орта баптау, конвенциялар, PR
- [`SECURITY.md`](SECURITY.md) — осалдық туралы қалай хабарлау керек

## Лицензия

[AGPL-3.0-only](LICENSE) — WhatsApp интеграциясы арқылы транзитивті түрде
келетін GPL-3.0 тәуелділігі бэкендке статикалық сілтенгені анықталғаннан
кейін таңдалды; толығырақ — [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)
файлында.
