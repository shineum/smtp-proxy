-- Restore body NOT NULL constraint (backfill empty string for any NULLs first)
UPDATE messages SET body = '' WHERE body IS NULL;
ALTER TABLE messages ALTER COLUMN body SET NOT NULL;

-- Drop storage_ref column
ALTER TABLE messages DROP COLUMN IF EXISTS storage_ref;

-- Note: PostgreSQL does not support removing individual enum values.
-- The 'enqueue_failed' and 'storage_error' values remain in the enum type.
-- To fully remove them, the enum type would need to be recreated,
-- which is destructive and not safe for a down migration.
