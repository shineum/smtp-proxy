-- Migration 022: Add indexes for SMTP auth query performance
-- Optimizes GetUserByUsernameAndGroupID JOIN path.

BEGIN;

-- Index on home_group_id for the JOIN users.home_group_id = groups.id
CREATE INDEX idx_users_home_group_id ON users(home_group_id) WHERE home_group_id IS NOT NULL;

COMMIT;
