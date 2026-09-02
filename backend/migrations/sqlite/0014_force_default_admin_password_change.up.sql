-- Force the mandatory password-change screen back on for the shipped
-- default admin credential (docs/ux/flows/01-onboarding.md, friction point
-- 1). 0011_restore_default_admin_password restored the known default
-- password for self-host ergonomics but left must_change_password = 0, so
-- the publicly-documented password (it is printed in README.md and sits in
-- this repo's git history) worked indefinitely on any deployment an
-- operator forgot to rotate — an immediate account-takeover risk on a
-- publicly reachable, unconfigured instance.
--
-- This keeps the same easy first login (no CLI bootstrap step, no random
-- one-time password to copy out of a log) but routes the very first
-- session through /change-password before anything else works — mirrored
-- by the backend's requirePasswordChanged gate (internal/httpapi/auth.go),
-- which is the actual enforcement boundary; this migration just flips the
-- flag that gate reads.
--
-- The WHERE guard mirrors 0011's own: only a row that still carries EXACTLY
-- the hash 0011 restored, with must_change_password still 0, is touched —
-- an operator who already changed the password (through the API, the CLI,
-- or a future in-app flow) is left alone.
UPDATE users
   SET must_change_password = 1,
       updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
 WHERE id = '00000000-0000-0000-0000-000000000002'
   AND password_hash = '$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB/ACkGsqq9io'
   AND must_change_password = 0;
