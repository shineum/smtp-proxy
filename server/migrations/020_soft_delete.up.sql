-- Migration 020: Add soft delete support for users
-- Enables 30-day retention before permanent deletion.

BEGIN;

ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NOT NULL;

COMMIT;
