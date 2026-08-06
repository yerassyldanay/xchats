-- Retire the shipped default admin password (security hardening ahead of
-- the public release). A FORWARD migration rather than an edit to
-- 0006_init_admin.up.sql: dbx.RunMigrations records applied versions and
-- never re-runs one, so editing 0006 in place would silently leave every
-- already-migrated database (including a fresh clone's first `xchats
-- serve`, if it happened before this file existed) with the old,
-- known-password sentinel — while only brand-new databases got the fix.
--
-- must_change_password gates every route but /me, /auth/logout, and
-- /auth/password (see internal/httpapi's requirePasswordChanged) until an
-- operator sets a real password.

ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0;

-- Blank the sentinel's hash and flag it, but ONLY if it still carries
-- 0006's committed hash — the guard in the WHERE clause. An operator who
-- already changed the password some other way (there is no in-app path
-- yet, but a direct DB edit or a future one is possible) is left alone:
-- this must never blank a hash the operator has already replaced.
--
-- '' is the "no password set" sentinel. internal/password.Verify splits on
-- "$" and requires exactly 6 parts with parts[1]=="argon2id" — an empty
-- string has 1 part, so it always fails closed. The account is unloginnable
-- from this migration until cmd/xchats' first-boot bootstrap mints a real
-- one (or an operator sets XCHATS_BOOTSTRAP_ADMIN_PASSWORD) — there is no
-- window where the account is reachable with a guessable password.
UPDATE users
   SET password_hash = '', must_change_password = 1
 WHERE id = '00000000-0000-0000-0000-000000000002'
   AND password_hash = '$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB/ACkGsqq9io';
