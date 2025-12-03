-- 0006_profile_social_links.sql
--
-- Adds a JSONB column to store user-provided social links alongside profile
-- metadata.

BEGIN;

ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS social_links JSONB NOT NULL DEFAULT '[]'::JSONB;

COMMIT;
