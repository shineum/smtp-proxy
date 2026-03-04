DROP INDEX IF EXISTS idx_users_provider_id;
ALTER TABLE users DROP COLUMN IF EXISTS provider_id;
