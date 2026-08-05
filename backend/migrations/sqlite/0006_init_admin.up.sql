-- xchats SQLite schema, part 6: the default organization + first admin
-- user. This is the ONLY source of required initial state — per the
-- sqlite-cutover plan's "Seeding & bootstrap" decision there is no `xchats
-- seed` command and no boot-time seed() call; a fresh DB_PATH is ready to
-- log into the moment migrations finish.
--
-- Fixed, sentinel-shaped UUIDs (not real uuidv4 output — deliberately
-- so, to be unmistakable in logs/debugging) make both inserts idempotent
-- by primary key; INSERT OR IGNORE additionally no-ops a stray re-run
-- outside the tracked migration runner.
--
-- Default admin login: admin@xchat.kz / xchat-admin-change-me
-- The password hash below is a PRECOMPUTED argon2id hash of that literal
-- password — SQLite cannot hash at migration time, so this was generated
-- once with the exact parameters internal/httpapi/auth.go's HashPassword
-- uses (argon2id, m=65536 KiB, t=1, p=4, keyLen=32) and round-trip
-- verified against its VerifyPassword. Change the password (and, if
-- desired, the email) in the app immediately after first login.

INSERT OR IGNORE INTO organizations (id, name, respond_mode, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'xchats',
    'NEVER',
    strftime('%Y-%m-%d %H:%M:%f','now'),
    strftime('%Y-%m-%d %H:%M:%f','now')
);

INSERT OR IGNORE INTO users (id, email, password_hash, display_name, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'admin@xchat.kz',
    '$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB/ACkGsqq9io',
    'Admin',
    strftime('%Y-%m-%d %H:%M:%f','now'),
    strftime('%Y-%m-%d %H:%M:%f','now')
);

INSERT OR IGNORE INTO organization_users (organization_id, user_id, joined_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000002',
    strftime('%Y-%m-%d %H:%M:%f','now')
);
