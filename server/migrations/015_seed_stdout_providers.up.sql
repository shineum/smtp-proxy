-- Create a stdout provider for every existing group that doesn't already have one.
INSERT INTO esp_providers (group_id, name, provider_type, smtp_config, enabled)
SELECT g.id, 'stdout', 'stdout', '{}', true
FROM groups g
WHERE NOT EXISTS (
    SELECT 1 FROM esp_providers ep
    WHERE ep.group_id = g.id AND ep.provider_type = 'stdout'
);
