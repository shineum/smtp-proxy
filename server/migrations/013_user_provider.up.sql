-- Add provider_id to users table for direct ESP provider mapping.
-- SMTP service accounts must reference their designated ESP provider.
-- Human accounts have NULL provider_id.
ALTER TABLE users ADD COLUMN provider_id UUID REFERENCES esp_providers(id) ON DELETE SET NULL;

-- Index for provider lookups by user.
CREATE INDEX idx_users_provider_id ON users(provider_id) WHERE provider_id IS NOT NULL;
