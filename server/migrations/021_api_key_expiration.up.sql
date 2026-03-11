-- Migration 021: Add API key expiration support
-- Default NULL = no expiration.

ALTER TABLE users ADD COLUMN api_key_expires_at TIMESTAMPTZ;
