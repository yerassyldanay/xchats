-- evals/schemas/decisions-v1.sql
--
-- DOCUMENTATION ONLY. This is the target structure from DECISIONS.md (repo root),
-- NOT YET implemented in the real product. Nothing in evals/ executes this file — it
-- exists so a reader can see exactly what the "shop-new.txt" / "xpayment-new.txt"
-- prompts are built from, and how it differs from current.sql.
--
-- Two changes under test in this exam:
--   1. Fact columns hold ONLY language-neutral values (digits, money, %, time, links).
--      The unit lives in the COLUMN NAME, not the value: delivery_in_days holds '1–3'
--      (not '1–3 дня'); available_pieces holds a NUMBER (not 'В наличии'). Word-bearing
--      meaning (availability wording, prepayment terms) moves into a plain `description`
--      field that the model reads and rephrases itself, in the customer's language.
--   2. Media files become COLUMNS on the table that owns them — arrays of refs, grouped
--      by role (images / videos / certificates / documents). No separate `ai_assets`
--      table, no per-file description; a group is always attached whole.
--
-- A REVIEW CAUGHT THIS FILE DOCUMENTING A RESTRICTION WITHOUT SHOWING HOW IT WOULD BE
-- ENFORCED — that only being "language-neutral" or "a real count" is a rule someone has
-- to remember, not something the database rejects a violation of. The CHECK constraints
-- below are what a real migration should carry to make these rules actually enforced,
-- not just documented. Two things are NOT solved here (flagged, not silently ignored):
--   - price / price_monthly / commission-adjacent MONEY fields are still free-form text
--     ('129 900 ₸') — a regex CHECK could require digits+space+currency-symbol shape, but
--     a cleaner fix is splitting amount (numeric) from currency (enum/lookup) into two
--     columns; not done here since that's a real design decision, not a documentation fix.
--   - MEDIA REF INTEGRITY CANNOT BE ENFORCED WITH ARRAY COLUMNS. Postgres has no way to
--     put a FOREIGN KEY on individual elements of a `text[]` — so nothing stops `images`
--     from containing a ref to a file that doesn't exist. If that guarantee ever matters
--     (e.g. before this is used for real sends), media needs a child table with a real
--     FK (ref, owner_table, owner_ref) instead of array columns — the exact shape
--     `ai_assets` already had, which this design intentionally traded away for the
--     simplicity of "a group is always attached whole." That trade-off is real and open.

CREATE TABLE ai_products (
    ref              text PRIMARY KEY,      -- 'coffee-machine'
    name             text NOT NULL,
    price            text NOT NULL,         -- '129 900 ₸'         — language-neutral
    available_pieces integer NOT NULL       -- 5                    — a NUMBER
                     CHECK (available_pieces >= 0),
    description      text NOT NULL,         -- prose: 'В наличии на складе...' lives HERE now
    category         text NOT NULL,
    images           text[] NOT NULL DEFAULT '{}',   -- ['cm-1.jpg','cm-2.jpg','cm-3.jpg']
    videos           text[] NOT NULL DEFAULT '{}',   -- ['cm-demo.mp4']
    certificates     text[] NOT NULL DEFAULT '{}'    -- ['cm-cert.pdf']
    -- no FK on images/videos/certificates elements — see the file header note.
);

CREATE TABLE ai_tariffs (
    ref                   text PRIMARY KEY,   -- 'start', 'standard', 'business'
    name                  text NOT NULL,
    price_monthly         text NOT NULL,      -- '19 900 ₸'
    commission_percent    text NOT NULL       -- '2.5'               — number only, no '%'
                          CHECK (commission_percent ~ '^[0-9]+(\.[0-9]+)?$'),
    payment_limit_monthly text NOT NULL,      -- '10 000 000 ₸'
    description           text NOT NULL,      -- prose: who the tariff is for
    images                text[] NOT NULL DEFAULT '{}',
    documents             text[] NOT NULL DEFAULT '{}'
    -- no FK on images/documents elements — see the file header note.
);

CREATE TABLE ai_policies (
    slug                text NOT NULL DEFAULT 'main',
    delivery_cost       text NOT NULL,   -- '1 500 ₸'            — location moved to topic prose
    delivery_in_days    text NOT NULL    -- '1–3'                — unit word left to the model
                        CHECK (delivery_in_days ~ '^[0-9]+(–[0-9]+)?$'),
    free_delivery_from  text NOT NULL,   -- '20 000 ₸'
    min_order           text NOT NULL,   -- '5 000 ₸'
    return_period_days  text NOT NULL    -- '14'
                        CHECK (return_period_days ~ '^[0-9]+$'),
    warranty_months     text NOT NULL    -- '12'
                        CHECK (warranty_months ~ '^[0-9]+$'),
    installment_scheme  text NOT NULL,   -- '0-0-3'
    installment_from    text NOT NULL,   -- '50 000 ₸'
    connection_days     text NOT NULL DEFAULT ''  -- '1' — added for the xpayment prompt
                        CHECK (connection_days = '' OR connection_days ~ '^[0-9]+$'),
    PRIMARY KEY (slug)
    -- prepayment ('не требуется') is no longer a fact column — it's word-bearing text,
    -- it moves into the topic/product description prose instead.
);

CREATE TABLE ai_contacts (
    slug       text NOT NULL DEFAULT 'support',
    phone      text NOT NULL,
    whatsapp   text NOT NULL,
    email      text NOT NULL,
    website    text NOT NULL,
    instagram  text NOT NULL,
    work_hours text NOT NULL,   -- '9:00–19:00'      — the days ('Пн–Сб') move to topic prose
    PRIMARY KEY (slug)
);

CREATE TABLE ai_topics (
    slug     text PRIMARY KEY,   -- 'delivery', 'payment', 'how_to_order'
    title    text NOT NULL,
    keywords text NOT NULL,
    body     text NOT NULL,      -- prose only: no digits, no placeholders
    images   text[] NOT NULL DEFAULT '{}',    -- e.g. delivery-zones map, grouped as topic.images
    documents text[] NOT NULL DEFAULT '{}'    -- e.g. price-list PDF
    -- no FK on images/documents elements — see the file header note.
);

-- No ai_assets table in this schema — media lives directly on its owner.
