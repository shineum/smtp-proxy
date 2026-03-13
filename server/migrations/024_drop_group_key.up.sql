-- Drop redundant group_key column (id is already a UUID PK)
DROP INDEX IF EXISTS idx_groups_group_key;
ALTER TABLE groups DROP COLUMN group_key;
