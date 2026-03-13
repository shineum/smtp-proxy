-- Migration 025: Create api_keys table for multi-key support
-- Allows service accounts to have multiple API keys with independent expiration

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_prefix VARCHAR(12) NOT NULL,  -- first 12 chars for identification
    key_hash VARCHAR(255) NOT NULL,   -- bcrypt hash of the full key
    label VARCHAR(100) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE UNIQUE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

-- Migrate existing api_keys from users table
-- Note: existing keys are plain text, store them as-is temporarily
-- They will be hashed by the application on first use
INSERT INTO api_keys (user_id, key_prefix, key_hash, label, expires_at, created_at)
SELECT id, LEFT(api_key, 12), api_key, 'default', api_key_expires_at, created_at
FROM users
WHERE api_key IS NOT NULL AND api_key != '';

-- Remove api_key columns from users table
ALTER TABLE users DROP COLUMN api_key;
ALTER TABLE users DROP COLUMN api_key_expires_at;
