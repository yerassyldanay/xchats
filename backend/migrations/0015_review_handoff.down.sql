SET search_path = xchats, public;

ALTER TABLE xchats.sessions DROP COLUMN IF EXISTS active_organization_id;
