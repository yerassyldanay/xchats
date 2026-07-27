-- evals/schemas/current.sql
--
-- DOCUMENTATION ONLY. This describes the structure the real product uses TODAY
-- (see backend/migrations/0005_kb_typed_facts.up.sql and 0006_kb_policies_contacts.up.sql).
-- Nothing in evals/ executes this file — it exists so a reader can see exactly what
-- the "shop-current.txt" prompt is built from and how it differs from decisions-v1.sql.
--
-- Two traits under test in this exam:
--   1. Fact values may contain WORDS, not just digits — «1–3 дня», «В наличии»,
--      «1 500 ₸ по Алматы», «12 месяцев на технику». A Kazakh-language reply that quotes
--      one of these verbatim ends up with Russian words inside a Kazakh sentence.
--   2. Media files live in a SEPARATE table, one row per file, selected by the model
--      reading a text description — not grouped by role.

CREATE TABLE ai_products (
    ref          text PRIMARY KEY,   -- 'coffee-machine'
    name         text NOT NULL,
    price        text NOT NULL,      -- '129 900 ₸'
    availability text NOT NULL,      -- 'В наличии' / 'Под заказ, 3–5 дней' — WORDS, not a number
    category     text NOT NULL,
    description  text NOT NULL
);

CREATE TABLE ai_policies (
    slug               text NOT NULL DEFAULT 'main',
    lang               text NOT NULL DEFAULT '*',
    delivery_cost      text NOT NULL,   -- '1 500 ₸ по Алматы'      — location word baked into the value
    delivery_time      text NOT NULL,   -- '1–3 дня'                — unit word baked into the value
    free_delivery_from text NOT NULL,   -- '20 000 ₸'
    min_order          text NOT NULL,
    prepayment         text NOT NULL,   -- 'не требуется'
    installment        text NOT NULL,   -- 'рассрочка 0-0-3 при заказе от 50 000 ₸'
    return_period      text NOT NULL,   -- '14 дней'
    warranty           text NOT NULL,   -- '12 месяцев на технику'
    PRIMARY KEY (slug, lang)
);

CREATE TABLE ai_contacts (
    slug          text NOT NULL DEFAULT 'support',
    lang          text NOT NULL DEFAULT '*',
    phone         text NOT NULL,
    whatsapp      text NOT NULL,
    email         text NOT NULL,
    address       text NOT NULL,
    working_hours text NOT NULL,   -- 'Пн–Сб, 9:00–19:00'
    website       text NOT NULL,
    instagram     text NOT NULL,
    PRIMARY KEY (slug, lang)
);

CREATE TABLE ai_topics (
    slug     text PRIMARY KEY,   -- 'delivery', 'catalog', 'how_to_order'
    lang     text NOT NULL DEFAULT 'ru',
    title    text NOT NULL,
    keywords text NOT NULL,
    body_md  text NOT NULL       -- pure prose, no digits, no placeholders
);

-- Media: ONE ROW PER FILE, in its own table, matched by a free-text description.
-- A product with 3 photos + 1 video + 1 certificate is 5 separate rows here.
CREATE TABLE ai_assets (
    ref         text PRIMARY KEY,   -- 'coffee-photo-1'
    asset_kind  text NOT NULL,      -- 'image' | 'video' | 'document' | 'audio'
    owner_kind  text NOT NULL,      -- 'topic' | 'product' | 'tariff'
    owner_ref   text NOT NULL,      -- 'coffee-machine'
    title       text NOT NULL,
    description text NOT NULL       -- the ONLY thing the model uses to pick this file
);
