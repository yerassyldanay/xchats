-- Restore the shipped default admin password (self-host ergonomics). This is
-- a deliberate reversal of 0008_bootstrap_admin.up.sql: that migration
-- traded a documented static default for a randomly-generated one-time
-- credential plus a forced first-login password change, and that trade-off
-- turned out to be too much friction for a self-hosted `docker compose up`.
-- The credential this restores is PUBLIC — the password and its hash are
-- already in this repo's git history (0006_init_admin.up.sql, this file) —
-- so it was never secret. Operators are expected to change it before
-- exposing an instance; see README.md and docs/release/installation.md.
--
-- The hash below is the exact literal 0006_init_admin.up.sql originally
-- shipped, for admin@xchat.kz / xchat-admin-change-me — reused verbatim, not
-- regenerated, since it already round-trip verifies against
-- internal/password.Verify.
--
-- The WHERE password_hash = '' guard is not optional: without it this would
-- clobber the real password of every operator who already completed 0008's
-- forced change, silently reopening their instance on a public credential.
-- Only a sentinel admin still sitting in 0008's blanked "no password set"
-- state — a fresh install, or one that never got past the forced-change
-- screen — is restored to the default.
UPDATE users
   SET password_hash = '$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB/ACkGsqq9io',
       must_change_password = 0,
       updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
 WHERE id = '00000000-0000-0000-0000-000000000002'
   AND password_hash = '';
