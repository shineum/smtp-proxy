DROP INDEX IF EXISTS idx_users_anonymous_smtp;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_anonymous_cidrs_required;

ALTER TABLE users
    DROP COLUMN IF EXISTS anonymous_allowed_cidrs,
    DROP COLUMN IF EXISTS anonymous_allowed;
