-- seed.sql seeds the fixture rows internal/pgimport's PGIMPORT_TEST_DSN-
-- gated tests (pgimport_test.go) expect to find in the source Postgres
-- database — one row (or a small connected chain, for the wa_* tables)
-- per distinct pg_type this schema uses, so a single import run exercises
-- every type conversion convert.go implements.
--
-- Usage: apply migrations/postgres's *.up.sql files (in order) against an
-- empty database first, THEN this file:
--
--   for f in backend/migrations/postgres/*.up.sql; do
--     psql "$DSN" -v ON_ERROR_STOP=1 -f "$f"
--   done
--   psql "$DSN" -v ON_ERROR_STOP=1 -f backend/internal/pgimport/testdata/seed.sql
--
-- Safe to re-run against a database that already has this exact fixture
-- data (every INSERT below targets a fixed id) only after clearing it
-- first — these are plain INSERTs, not upserts, matching
-- internal/pgimport's own non-idempotent-by-design import semantics (see
-- pgimport_test.go's TestImport_SecondRunFailsWithoutDuplicatingOrPartiallyImporting).

INSERT INTO xchats.organizations (id, name, respond_mode, respond_window_start, respond_window_end)
VALUES ('00000000-0000-0000-0000-0000000000aa', 'Test Org', 'auto', '09:00:00', '18:30:00');

INSERT INTO xchats.users (id, email, password_hash, display_name)
VALUES ('00000000-0000-0000-0000-0000000000bb', 'Mixed@Example.com', 'hash123', 'Tester');

INSERT INTO xchats.organization_users (organization_id, user_id)
VALUES ('00000000-0000-0000-0000-0000000000aa', '00000000-0000-0000-0000-0000000000bb');

INSERT INTO xchats.sessions (id, user_id, expires_at, active_organization_id)
VALUES ('sess_test_1', '00000000-0000-0000-0000-0000000000bb', now() + interval '1 day', '00000000-0000-0000-0000-0000000000aa');

INSERT INTO xchats.kbd_materials (id, organization_id, source_type, source_ref, extracted_text, media_kind, status, extraction)
VALUES ('00000000-0000-0000-0000-0000000000cc', '00000000-0000-0000-0000-0000000000aa', 'text', 'ref', 'extracted', 'image', 'ready', '{"key": "value", "n": null}');

INSERT INTO xchats.ai_topics (id, organization_id, slug, title, body_md, featured_image, illustration_images, explainer_videos, reference_documents)
VALUES ('00000000-0000-0000-0000-0000000000dd', '00000000-0000-0000-0000-0000000000aa', 'slug1', 'Title', 'Body', '00000000-0000-0000-0000-0000000000cc',
  ARRAY['00000000-0000-0000-0000-0000000000cc'::uuid], ARRAY[]::uuid[],
  ARRAY['00000000-0000-0000-0000-0000000000cc'::uuid, '00000000-0000-0000-0000-0000000000cc'::uuid]);

INSERT INTO xchats.mcp_oauth_clients (client_id, client_name, redirect_uris, registration_source, token_endpoint_auth_method)
VALUES ('client1', 'Client One', ARRAY['https://a.example/cb', 'https://b.example/cb'], 'dcr', 'none');

-- ai_drafts carries no foreign keys (migration 0013 dropped them — see
-- migrations/postgres/0013_telegram_channel.up.sql), so chat_id/
-- trigger_message_id below reference no real row on purpose.
INSERT INTO xchats.ai_drafts (id, chat_id, trigger_message_id, option_ordinal, draft_text, context_state, confidence, escalate, escalation_reason, draft_state, reply_language, channel)
VALUES ('00000000-0000-0000-0000-0000000000ee', gen_random_uuid(), gen_random_uuid(), 1, 'draft text', 'state', 0.87654321, false, '', 'suggested', 'ru', 'whatsapp');

INSERT INTO xchats.tg_accounts (id, organization_id, bot_id, bot_username, connection_state)
VALUES ('00000000-0000-0000-0000-0000000000ff', '00000000-0000-0000-0000-0000000000aa', 12345, 'testbot', 'connected');

INSERT INTO xchats.tg_credentials (account_id, bot_token_enc, encryption_key_version)
VALUES ('00000000-0000-0000-0000-0000000000ff', '\x0102030405deadbeef'::bytea, 1);

-- A connected wa_* chain (account -> contact -> chat -> message -> media)
-- proves multi-table foreign-key-preserving import, not just single-row
-- type conversion.
INSERT INTO xchats.wa_accounts (id, organization_id, display_name, owner_jid, phone_number, connection_state)
VALUES ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-0000000000aa', 'WA Test', '77012345678@s.whatsapp.net', '77012345678', 'connected');

INSERT INTO xchats.wa_contacts (id, account_id, phone_jid, phone_number, push_name)
VALUES ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000101', '77099998888@s.whatsapp.net', '77099998888', 'Customer');

INSERT INTO xchats.wa_chats (id, account_id, contact_id, remote_jid, chat_state, last_message_preview)
VALUES ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000102', '77099998888@s.whatsapp.net', 'open', 'Привет!');

INSERT INTO xchats.wa_messages (id, account_id, chat_id, direction, sender_kind, evolution_message_id, body, message_ts)
VALUES ('00000000-0000-0000-0000-000000000104', '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000103', 'in', 'contact', 'WAMSG1', 'Привет! Сколько стоит?', now());

INSERT INTO xchats.message_media (id, message_id, media_type, mimetype, file_name, file_size, storage_url, download_status)
VALUES ('00000000-0000-0000-0000-000000000105', '00000000-0000-0000-0000-000000000104', 'image', 'image/jpeg', 'photo.jpg', 12345, 'https://example/photo.jpg', 'ready');
