-- Migration 019: Add group_key, display_name, description to groups
--                Add home_group_id, display_name, description to users
-- Enables username@group_key SMTP authentication format.

BEGIN;

-- ============================================================
-- PHASE 1: Groups - add group_key, display_name, description
-- ============================================================

ALTER TABLE groups ADD COLUMN group_key UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE groups ADD COLUMN display_name VARCHAR(255);
ALTER TABLE groups ADD COLUMN description TEXT;

CREATE UNIQUE INDEX idx_groups_group_key ON groups(group_key);

-- Backfill display_name from name for existing groups
UPDATE groups SET display_name = name;

-- ============================================================
-- PHASE 2: Users - add home_group_id, display_name, description
-- ============================================================

ALTER TABLE users ADD COLUMN home_group_id UUID REFERENCES groups(id);
ALTER TABLE users ADD COLUMN display_name VARCHAR(255);
ALTER TABLE users ADD COLUMN description TEXT;

-- Backfill home_group_id for existing SMTP accounts from group_members
UPDATE users u
SET home_group_id = gm.group_id
FROM group_members gm
WHERE gm.user_id = u.id
  AND u.account_type = 'smtp';

-- Drop the old global unique constraint on username
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;

-- Create composite unique: (username, home_group_id) for SMTP accounts only
CREATE UNIQUE INDEX idx_users_username_home_group
ON users(username, home_group_id)
WHERE account_type = 'smtp' AND username IS NOT NULL;

COMMIT;
