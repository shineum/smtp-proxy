DROP TABLE IF EXISTS provider_group_access;
ALTER TABLE esp_providers DROP COLUMN IF EXISTS visibility;
DROP TYPE IF EXISTS provider_visibility;
