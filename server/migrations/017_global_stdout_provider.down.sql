-- Remove the global stdout provider and recreate per-group ones
DELETE FROM esp_providers WHERE provider_type = 'stdout' AND visibility = 'global';

-- Recreate per-group stdout providers (best effort rollback)
INSERT INTO esp_providers (group_id, name, provider_type, smtp_config, enabled, visibility)
SELECT g.id, 'stdout', 'stdout', '{}', true, 'global'
FROM groups g
WHERE NOT EXISTS (
    SELECT 1 FROM esp_providers ep
    WHERE ep.group_id = g.id AND ep.provider_type = 'stdout'
);
