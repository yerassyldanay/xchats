SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.ai_policies;

ALTER TABLE xchats.ai_products
    DROP COLUMN IF EXISTS availability;

ALTER TABLE xchats.ai_contacts
    DROP COLUMN IF EXISTS working_hours,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS website,
    DROP COLUMN IF EXISTS instagram;
