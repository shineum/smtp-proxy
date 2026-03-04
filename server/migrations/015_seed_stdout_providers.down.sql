-- Remove stdout providers seeded by this migration.
DELETE FROM esp_providers WHERE provider_type = 'stdout';
