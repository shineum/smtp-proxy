-- Add provider visibility model for access control.
-- visibility determines which groups can use a provider:
--   private: only the owning group
--   shared:  owning group + groups in provider_group_access
--   global:  all groups

CREATE TYPE provider_visibility AS ENUM ('private', 'shared', 'global');

ALTER TABLE esp_providers ADD COLUMN visibility provider_visibility NOT NULL DEFAULT 'private';

-- Existing stdout providers should be globally accessible.
UPDATE esp_providers SET visibility = 'global' WHERE provider_type = 'stdout';

-- Junction table for shared provider access.
CREATE TABLE provider_group_access (
    provider_id UUID NOT NULL REFERENCES esp_providers(id) ON DELETE CASCADE,
    group_id    UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (provider_id, group_id)
);

CREATE INDEX idx_provider_group_access_group ON provider_group_access(group_id);
