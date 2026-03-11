BEGIN;

DROP INDEX IF EXISTS idx_users_username_home_group;
ALTER TABLE users DROP COLUMN IF EXISTS description;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
ALTER TABLE users DROP COLUMN IF EXISTS home_group_id;
CREATE UNIQUE INDEX users_username_key ON users(username) WHERE username IS NOT NULL;

ALTER TABLE groups DROP COLUMN IF EXISTS description;
ALTER TABLE groups DROP COLUMN IF EXISTS display_name;
DROP INDEX IF EXISTS idx_groups_group_key;
ALTER TABLE groups DROP COLUMN IF EXISTS group_key;

COMMIT;
