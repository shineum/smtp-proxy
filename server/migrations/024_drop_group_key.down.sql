ALTER TABLE groups ADD COLUMN group_key UUID NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX idx_groups_group_key ON groups(group_key);
-- Backfill: copy id to group_key for existing rows
UPDATE groups SET group_key = id;
