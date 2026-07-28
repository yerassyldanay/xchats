-- 0008_kb_response_demo_data — temporary demo KB data for the multichannel
-- ResponseService's first production rollout. SQL data only: no Go literals, no
-- seed-kb-dev command, no runtime fallback anywhere in application code. Once
-- real KB administration lands, a later forward migration
-- (0009_remove_kb_response_demo_data) deletes these exact fixed UUIDs — this
-- file itself is never edited after being applied.
--
-- This migration targets an EXISTING organization; it does NOT create one.
-- `serve` runs every pending migration before the identity seed creates the
-- org (cmd/xchats/main.go), so on a brand-new database there is no organization
-- yet at the moment this file runs. Raising an error here would deadlock every
-- fresh install (migrations must all succeed before the org-creating seed ever
-- runs). Instead: no organization -> RAISE NOTICE and no-op.
--
-- Dev workflow to get demo data onto a fresh database: boot the app once so the
-- identity seed creates the organization, then apply this file's body again by
-- hand (or run `migrate down` to 0007 and back `up`) — the schema_migrations
-- tracking table only stops this file from running automatically a SECOND time
-- through the normal migrate step, not from being replayed manually.
--
-- Non-destructive and idempotent: every insert is guarded by a NOT EXISTS check
-- keyed on a fixed UUID or a demo_-prefixed natural ref/slug, so re-running this
-- file's body never truncates or overwrites a real seller's row. A pre-existing
-- ai_policies row (e.g. from the legacy brain seed, which set a non-blank flat
-- delivery_cost) is left completely untouched; demo delivery zones are only
-- added when the org's policies row is already zone-compatible (blank flat
-- delivery fields — see aiprompt.BuildCatalog's fail-closed invariant), so this
-- migration can never leave an org's KB in a state aiprompt itself would reject.
SET search_path = xchats, public;

DO $$
DECLARE
    v_org_id    uuid;
    v_zones_ok  boolean;
BEGIN
    -- The org owning the oldest non-deleted WhatsApp account, else the oldest org.
    SELECT a.organization_id INTO v_org_id
    FROM xchats.wa_accounts a
    WHERE a.deleted_at IS NULL AND a.organization_id IS NOT NULL
    ORDER BY a.created_at ASC
    LIMIT 1;

    IF v_org_id IS NULL THEN
        SELECT o.id INTO v_org_id
        FROM xchats.organizations o
        ORDER BY o.created_at ASC
        LIMIT 1;
    END IF;

    IF v_org_id IS NULL THEN
        RAISE NOTICE '0008_kb_response_demo_data: no organization exists yet — skipping demo KB data (expected on a fresh database; re-apply this file''s body by hand after the identity seed creates the organization)';
        RETURN;
    END IF;

    -- ai_assistants — singleton per org. Persona/mission/guardrails/language_policy
    -- must stay compatible with the shop-kb-v4 frame's own language rule (rule 7:
    -- reply language follows the CUSTOMER's message) — never assert a blanket
    -- "always answer in Russian" here. That exact conflict ("Отвечай только на
    -- русском языке.") was observed in the eval fixture; this persona avoids it.
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_assistants WHERE organization_id = v_org_id) THEN
        INSERT INTO xchats.ai_assistants
            (id, organization_id, persona, mission, guardrails, language_policy, reply_max_words)
        VALUES (
            '00000000-0000-4000-9000-000000000d01', v_org_id,
            'Ты — ассистент по подготовке ответов клиентам интернет-магазина «Demo Shop» в WhatsApp. Ты готовишь ОДИН черновик ответа, который проверит и отправит человек — ты никогда не отправляешь сообщения сам.',
            'Помогай клиентам выбрать товар, узнать актуальную цену и наличие, и оформить заказ.',
            'Никогда не выдумывай цены, наличие, сроки или контактные данные — используй только подтверждённые значения. Никогда не обещай медиафайл, которого нет в каталоге.',
            'Отвечай на языке сообщения клиента — по-русски или по-казахски.',
            120
        );
    END IF;

    -- ai_contacts — singleton per (org, lang='*').
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_contacts WHERE organization_id = v_org_id AND lang = '*') THEN
        INSERT INTO xchats.ai_contacts
            (id, organization_id, lang, whatsapp, email, address, legal, callback_time, working_hours, phone, website, instagram)
        VALUES (
            '00000000-0000-4000-9000-000000000d02', v_org_id, '*',
            '', '', '', '', '', 'Пн–Сб, 9:00–19:00', '+7 700 000 00 00', '', '@demo.shop'
        );
    END IF;

    -- ai_policies — singleton per (org, lang='*'). Zone-compatible from the start:
    -- delivery_cost/delivery_time blank, outside_zones_note set (see the zone
    -- insert guard below).
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_policies WHERE organization_id = v_org_id AND lang = '*') THEN
        INSERT INTO xchats.ai_policies
            (id, organization_id, lang, delivery_cost, delivery_time, free_delivery_from, min_order,
             prepayment, installment, return_period, warranty, outside_zones_note)
        VALUES (
            '00000000-0000-4000-9000-000000000d03', v_org_id, '*',
            '', '', '20 000 ₸', '5 000 ₸', '', '', '', '',
            'В города и страны за пределами списка зон доставки мы не доставляем.'
        );
    END IF;

    -- ai_topics — 2-3 demo topics, pure prose (no fact tokens, no digits).
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_topics WHERE organization_id = v_org_id AND slug = 'demo_catalog') THEN
        INSERT INTO xchats.ai_topics (id, organization_id, slug, lang, title, body_md)
        VALUES (
            '00000000-0000-4000-9000-000000000d11', v_org_id, 'demo_catalog', 'ru', 'Каталог',
            'В каталоге бытовая техника для дома и кухни. Актуальные позиции, цены и наличие — только из блоков товаров, не перечисляй товары по памяти.'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_topics WHERE organization_id = v_org_id AND slug = 'demo_payment') THEN
        INSERT INTO xchats.ai_topics (id, organization_id, slug, lang, title, body_md)
        VALUES (
            '00000000-0000-4000-9000-000000000d12', v_org_id, 'demo_payment', 'ru', 'Оплата',
            'Принимаем оплату картой, через Kaspi и наличными при получении. Оформление — прямо в WhatsApp.'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_topics WHERE organization_id = v_org_id AND slug = 'demo_warranty') THEN
        INSERT INTO xchats.ai_topics (id, organization_id, slug, lang, title, body_md)
        VALUES (
            '00000000-0000-4000-9000-000000000d13', v_org_id, 'demo_warranty', 'ru', 'Гарантия',
            'На технику действует гарантия производителя — она покрывает заводской брак и оформляется чеком или подтверждением заказа.'
        );
    END IF;

    -- ai_products — 5 demo products, mixed in_stock (3 in stock, 2 out of stock).
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_products WHERE organization_id = v_org_id AND ref = 'demo_coffee-machine') THEN
        INSERT INTO xchats.ai_products (id, organization_id, ref, lang, name, price, description, category, status, in_stock)
        VALUES (
            '00000000-0000-4000-9000-000000000d21', v_org_id, 'demo_coffee-machine', 'ru',
            'Кофемашина DeLonghi', '129 900 ₸', 'Автоматическая кофемашина для дома с капучинатором и жерновковой кофемолкой.',
            '', 'active', true
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_products WHERE organization_id = v_org_id AND ref = 'demo_blender') THEN
        INSERT INTO xchats.ai_products (id, organization_id, ref, lang, name, price, description, category, status, in_stock)
        VALUES (
            '00000000-0000-4000-9000-000000000d22', v_org_id, 'demo_blender', 'ru',
            'Блендер Bosch', '11 200 ₸', 'Мощный блендер для смузи, соусов и супов-пюре — несколько скоростей и импульсный режим.',
            '', 'active', true
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_products WHERE organization_id = v_org_id AND ref = 'demo_kettle') THEN
        INSERT INTO xchats.ai_products (id, organization_id, ref, lang, name, price, description, category, status, in_stock)
        VALUES (
            '00000000-0000-4000-9000-000000000d23', v_org_id, 'demo_kettle', 'ru',
            'Чайник Bosch', '40 200 ₸', 'Электрический чайник с быстрым закипанием и автоматическим отключением.',
            '', 'active', false
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_products WHERE organization_id = v_org_id AND ref = 'demo_toaster') THEN
        INSERT INTO xchats.ai_products (id, organization_id, ref, lang, name, price, description, category, status, in_stock)
        VALUES (
            '00000000-0000-4000-9000-000000000d24', v_org_id, 'demo_toaster', 'ru',
            'Тостер Tefal', '81 600 ₸', 'Компактный тостер с регулировкой степени поджаривания и функцией разморозки.',
            '', 'active', true
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_products WHERE organization_id = v_org_id AND ref = 'demo_vacuum') THEN
        INSERT INTO xchats.ai_products (id, organization_id, ref, lang, name, price, description, category, status, in_stock)
        VALUES (
            '00000000-0000-4000-9000-000000000d25', v_org_id, 'demo_vacuum', 'ru',
            'Пылесос Samsung', '83 800 ₸', 'Пылесос с мешком для сбора пыли и насадками для разных типов покрытий.',
            '', 'active', false
        );
    END IF;

    -- ai_tariffs — 2 demo tariffs (schema completeness; the shop-kb-v4 frame does
    -- not render tariff facts today, but the fact type must round-trip cleanly).
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_tariffs WHERE organization_id = v_org_id AND ref = 'demo_basic') THEN
        INSERT INTO xchats.ai_tariffs (id, organization_id, ref, lang, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages, status)
        VALUES (
            '00000000-0000-4000-9000-000000000d31', v_org_id, 'demo_basic', 'ru',
            'Базовая доставка', '5 000 ₸', '', '', 'Доставка курьером в удобное время.', 'fixed', '', '', 'active'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM xchats.ai_tariffs WHERE organization_id = v_org_id AND ref = 'demo_express') THEN
        INSERT INTO xchats.ai_tariffs (id, organization_id, ref, lang, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages, status)
        VALUES (
            '00000000-0000-4000-9000-000000000d32', v_org_id, 'demo_express', 'ru',
            'Экспресс-доставка', '8 000 ₸', '', '', 'Доставка в течение нескольких часов.', 'fixed', '', '', 'active'
        );
    END IF;

    -- ai_delivery_zones — only when the org's ai_policies row is already
    -- zone-compatible (blank flat delivery fields, a real outside_zones_note) —
    -- never forced in alongside a pre-existing, non-zone-shaped policies row.
    SELECT (delivery_cost = '' AND delivery_time = '' AND outside_zones_note <> '')
    INTO v_zones_ok
    FROM xchats.ai_policies
    WHERE organization_id = v_org_id AND lang = '*';

    IF COALESCE(v_zones_ok, false) THEN
        IF NOT EXISTS (SELECT 1 FROM xchats.ai_delivery_zones WHERE organization_id = v_org_id AND ref = 'demo_kazakhstan') THEN
            INSERT INTO xchats.ai_delivery_zones
                (id, organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, notes, status)
            VALUES (
                '00000000-0000-4000-9000-000000000d41', v_org_id, 'demo_kazakhstan', 'Казахстан (остальные города)',
                'country', '', true, '10 000 ₸', '3–4', '', 'active'
            );
        END IF;
        IF NOT EXISTS (SELECT 1 FROM xchats.ai_delivery_zones WHERE organization_id = v_org_id AND ref = 'demo_almaty') THEN
            INSERT INTO xchats.ai_delivery_zones
                (id, organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, notes, status)
            VALUES (
                '00000000-0000-4000-9000-000000000d42', v_org_id, 'demo_almaty', 'Алматы',
                'city', 'demo_kazakhstan', true, '5 000 ₸', '1', '', 'active'
            );
        END IF;
        IF NOT EXISTS (SELECT 1 FROM xchats.ai_delivery_zones WHERE organization_id = v_org_id AND ref = 'demo_baikonur') THEN
            INSERT INTO xchats.ai_delivery_zones
                (id, organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, notes, status)
            VALUES (
                '00000000-0000-4000-9000-000000000d43', v_org_id, 'demo_baikonur', 'Байконур',
                'city', 'demo_kazakhstan', false, '', '', 'Особый административный статус города — курьерская доставка туда не осуществляется.', 'active'
            );
        END IF;
    ELSE
        RAISE NOTICE '0008_kb_response_demo_data: organization % already has a non-zone-shaped ai_policies row — skipping demo delivery zones to avoid an aiprompt.BuildCatalog conflict', v_org_id;
    END IF;
END $$;
