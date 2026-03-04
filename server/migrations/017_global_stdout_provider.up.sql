-- Consolidate per-group stdout providers into a single global stdout
-- owned by the system group.

-- Delete all per-group stdout providers (they will be replaced by one global)
DELETE FROM esp_providers WHERE provider_type = 'stdout';

-- Create a single global stdout provider owned by the system group
INSERT INTO esp_providers (group_id, name, provider_type, smtp_config, enabled, visibility)
SELECT g.id, 'stdout', 'stdout', '{}', true, 'global'
FROM groups g
WHERE g.group_type = 'system'
LIMIT 1;
