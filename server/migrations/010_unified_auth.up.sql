-- Migration 010: Unified Account System Redesign (SPEC-AUTH-001)
--
-- Merges accounts (SMTP service accounts) into users table,
-- renames tenants -> groups, introduces many-to-many group_members,
-- and replaces audit_logs with activity_logs.
--
-- Key changes:
--   tenants       -> groups (with group_type column)
--   accounts      -> users (account_type = 'smtp')
--   users.role    -> group_members.role (many-to-many)
--   audit_logs    -> activity_logs (redesigned schema)
--   tenant_id     -> group_id (all FK references)

BEGIN;

-- ============================================================
-- PHASE 1: Drop all existing RLS policies
-- ============================================================
-- Must happen before renaming tables/columns they reference.

DROP POLICY IF EXISTS delivery_log_tenant_isolation ON delivery_logs;
DROP POLICY IF EXISTS message_tenant_isolation ON messages;
DROP POLICY IF EXISTS rule_tenant_isolation ON routing_rules;
DROP POLICY IF EXISTS provider_tenant_isolation ON esp_providers;
DROP POLICY IF EXISTS account_tenant_isolation ON accounts;
DROP POLICY IF EXISTS audit_tenant_isolation ON audit_logs;
DROP POLICY IF EXISTS session_tenant_isolation ON sessions;
DROP POLICY IF EXISTS user_tenant_isolation ON users;
DROP POLICY IF EXISTS tenant_isolation ON tenants;

-- ============================================================
-- PHASE 2: Rename tenants -> groups and modify
-- ============================================================

ALTER TABLE tenants RENAME TO groups;

-- Add group_type column (existing rows become 'company')
ALTER TABLE groups ADD COLUMN group_type VARCHAR(20) NOT NULL DEFAULT 'company';

-- Change monthly_limit default from 10000 to 0 (0 = unlimited)
ALTER TABLE groups ALTER COLUMN monthly_limit SET DEFAULT 0;

-- ============================================================
-- PHASE 3: Add new columns to users table
-- ============================================================
-- Existing users table has: id, tenant_id, email, password_hash,
-- role, status, failed_attempts, last_login, created_at, updated_at

ALTER TABLE users ADD COLUMN username VARCHAR(255) UNIQUE;
ALTER TABLE users ADD COLUMN account_type VARCHAR(20) NOT NULL DEFAULT 'human';
ALTER TABLE users ADD COLUMN api_key VARCHAR(255) UNIQUE;
ALTER TABLE users ADD COLUMN allowed_domains JSONB;

-- ============================================================
-- PHASE 4: Create group_members table (many-to-many)
-- ============================================================

CREATE TABLE group_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(group_id, user_id)
);

CREATE INDEX idx_group_members_group_id ON group_members(group_id);
CREATE INDEX idx_group_members_user_id ON group_members(user_id);

-- ============================================================
-- PHASE 5: Migrate existing users into group_members
-- ============================================================
-- Each user currently has a single tenant_id and role.
-- Move this relationship to the group_members junction table.

INSERT INTO group_members (group_id, user_id, role)
SELECT tenant_id, id, role
FROM users
WHERE tenant_id IS NOT NULL;

-- ============================================================
-- PHASE 6: Create mapping table and migrate accounts -> users
-- ============================================================
-- accounts (SMTP service accounts) become users with account_type='smtp'.
-- We need a mapping table to update FKs on dependent tables.

CREATE TABLE _account_user_map (
    account_id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL
);

-- Insert accounts as SMTP users.
-- Use accounts.name as username, synthesize email as '{name}@smtp.internal'.
WITH inserted AS (
    INSERT INTO users (email, username, password_hash, account_type, api_key, allowed_domains, status, created_at, updated_at)
    SELECT
        a.name || '@smtp.internal',
        a.name,
        a.password_hash,
        'smtp',
        a.api_key,
        a.allowed_domains,
        'active',
        a.created_at,
        a.updated_at
    FROM accounts a
    RETURNING id, username
)
INSERT INTO _account_user_map (account_id, user_id)
SELECT a.id, i.id
FROM accounts a
JOIN inserted i ON a.name = i.username;

-- Create group_members entries for migrated SMTP accounts that had a tenant.
INSERT INTO group_members (group_id, user_id, role)
SELECT a.tenant_id, m.user_id, 'member'
FROM accounts a
JOIN _account_user_map m ON a.id = m.account_id
WHERE a.tenant_id IS NOT NULL;

-- ============================================================
-- PHASE 7: Create system group for orphaned records
-- ============================================================
-- If any esp_providers or routing_rules have no tenant_id AND their
-- account has no tenant_id, we need a group to assign them to.

INSERT INTO groups (name, group_type, status)
SELECT 'system', 'system', 'active'
WHERE (
    EXISTS (
        SELECT 1 FROM esp_providers e
        JOIN accounts a ON e.account_id = a.id
        WHERE e.tenant_id IS NULL AND a.tenant_id IS NULL
    )
    OR EXISTS (
        SELECT 1 FROM routing_rules r
        JOIN accounts a ON r.account_id = a.id
        WHERE r.tenant_id IS NULL AND a.tenant_id IS NULL
    )
    OR EXISTS (
        SELECT 1 FROM messages m
        JOIN accounts a ON m.account_id = a.id
        WHERE m.tenant_id IS NULL AND a.tenant_id IS NULL
    )
)
AND NOT EXISTS (SELECT 1 FROM groups WHERE name = 'system');

-- ============================================================
-- PHASE 8: Update esp_providers
-- ============================================================
-- Current: account_id UUID NOT NULL FK accounts, tenant_id UUID FK groups (nullable)
-- Target:  group_id UUID NOT NULL FK groups

ALTER TABLE esp_providers ADD COLUMN group_id UUID;

-- Populate from tenant_id where available (tenant_id now references groups)
UPDATE esp_providers SET group_id = tenant_id WHERE tenant_id IS NOT NULL;

-- For rows without tenant_id, resolve via accounts.tenant_id
UPDATE esp_providers e
SET group_id = a.tenant_id
FROM accounts a
WHERE e.account_id = a.id
  AND e.group_id IS NULL
  AND a.tenant_id IS NOT NULL;

-- For any remaining orphans, assign to system group
UPDATE esp_providers
SET group_id = (SELECT id FROM groups WHERE name = 'system' LIMIT 1)
WHERE group_id IS NULL;

ALTER TABLE esp_providers ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE esp_providers
    ADD CONSTRAINT esp_providers_group_id_fkey
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;

-- Drop old indexes and columns
DROP INDEX IF EXISTS idx_esp_providers_tenant_id;
ALTER TABLE esp_providers DROP COLUMN account_id;
ALTER TABLE esp_providers DROP COLUMN tenant_id;

CREATE INDEX idx_esp_providers_group_id ON esp_providers(group_id);

-- ============================================================
-- PHASE 9: Update routing_rules
-- ============================================================
-- Current: account_id UUID NOT NULL FK accounts, tenant_id UUID FK groups (nullable)
-- Target:  group_id UUID NOT NULL FK groups

ALTER TABLE routing_rules ADD COLUMN group_id UUID;

UPDATE routing_rules SET group_id = tenant_id WHERE tenant_id IS NOT NULL;

UPDATE routing_rules r
SET group_id = a.tenant_id
FROM accounts a
WHERE r.account_id = a.id
  AND r.group_id IS NULL
  AND a.tenant_id IS NOT NULL;

UPDATE routing_rules
SET group_id = (SELECT id FROM groups WHERE name = 'system' LIMIT 1)
WHERE group_id IS NULL;

ALTER TABLE routing_rules ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE routing_rules
    ADD CONSTRAINT routing_rules_group_id_fkey
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;

-- Drop old indexes and columns
DROP INDEX IF EXISTS idx_routing_rules_tenant_id;
DROP INDEX IF EXISTS idx_routing_rules_account_priority;
ALTER TABLE routing_rules DROP COLUMN account_id;
ALTER TABLE routing_rules DROP COLUMN tenant_id;

CREATE INDEX idx_routing_rules_group_id ON routing_rules(group_id);
CREATE INDEX idx_routing_rules_group_priority ON routing_rules(group_id, priority);

-- ============================================================
-- PHASE 10: Update messages
-- ============================================================
-- Current: account_id UUID NOT NULL FK accounts, tenant_id UUID FK groups (nullable)
-- Target:  user_id UUID FK users, group_id UUID FK groups

ALTER TABLE messages ADD COLUMN group_id UUID;
ALTER TABLE messages ADD COLUMN user_id UUID;

-- Populate group_id from tenant_id where available
UPDATE messages SET group_id = tenant_id WHERE tenant_id IS NOT NULL;

-- For rows without tenant_id, resolve via accounts.tenant_id
UPDATE messages m
SET group_id = a.tenant_id
FROM accounts a
WHERE m.account_id = a.id
  AND m.group_id IS NULL
  AND a.tenant_id IS NOT NULL;

-- For any remaining orphans, assign to system group
UPDATE messages
SET group_id = (SELECT id FROM groups WHERE name = 'system' LIMIT 1)
WHERE group_id IS NULL;

-- Populate user_id from account mapping (account_id -> new user_id)
UPDATE messages m
SET user_id = aum.user_id
FROM _account_user_map aum
WHERE m.account_id = aum.account_id;

ALTER TABLE messages
    ADD CONSTRAINT messages_group_id_fkey
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;
ALTER TABLE messages
    ADD CONSTRAINT messages_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

-- Drop old indexes and columns
DROP INDEX IF EXISTS idx_messages_tenant_id;
DROP INDEX IF EXISTS idx_messages_account_status;
ALTER TABLE messages DROP COLUMN account_id;
ALTER TABLE messages DROP COLUMN tenant_id;

CREATE INDEX idx_messages_group_id ON messages(group_id);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_user_status ON messages(user_id, status);

-- ============================================================
-- PHASE 11: Update delivery_logs
-- ============================================================
-- Current: account_id UUID FK accounts (009), mt_tenant_id UUID FK groups (007),
--          tenant_id VARCHAR(255) (006)
-- Target:  user_id UUID FK users, group_id UUID FK groups

ALTER TABLE delivery_logs ADD COLUMN user_id UUID;
ALTER TABLE delivery_logs ADD COLUMN group_id UUID;

-- Populate group_id from mt_tenant_id (UUID FK, added in 007)
UPDATE delivery_logs SET group_id = mt_tenant_id WHERE mt_tenant_id IS NOT NULL;

-- Populate user_id from account_id mapping
UPDATE delivery_logs dl
SET user_id = aum.user_id
FROM _account_user_map aum
WHERE dl.account_id = aum.account_id;

ALTER TABLE delivery_logs
    ADD CONSTRAINT delivery_logs_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE delivery_logs
    ADD CONSTRAINT delivery_logs_group_id_fkey
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL;

-- Drop old indexes and columns
DROP INDEX IF EXISTS idx_delivery_logs_mt_tenant_id;
DROP INDEX IF EXISTS idx_delivery_logs_account_created;
DROP INDEX IF EXISTS idx_delivery_logs_tenant_status;
ALTER TABLE delivery_logs DROP COLUMN account_id;
ALTER TABLE delivery_logs DROP COLUMN mt_tenant_id;
ALTER TABLE delivery_logs DROP COLUMN tenant_id;  -- the VARCHAR(255) column from 006

CREATE INDEX idx_delivery_logs_group_id ON delivery_logs(group_id);
CREATE INDEX idx_delivery_logs_user_id ON delivery_logs(user_id);
CREATE INDEX idx_delivery_logs_group_status ON delivery_logs(group_id, status);
CREATE INDEX idx_delivery_logs_user_created ON delivery_logs(user_id, created_at)
    WHERE user_id IS NOT NULL;

-- ============================================================
-- PHASE 12: Update sessions (rename tenant_id -> group_id)
-- ============================================================

DROP INDEX IF EXISTS idx_sessions_tenant_id;
ALTER TABLE sessions RENAME COLUMN tenant_id TO group_id;
CREATE INDEX idx_sessions_group_id ON sessions(group_id);

-- ============================================================
-- PHASE 13: Replace audit_logs with activity_logs
-- ============================================================

DROP TABLE audit_logs;

CREATE TABLE activity_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID,
    changes JSONB,
    comment TEXT,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_activity_logs_group_created ON activity_logs(group_id, created_at DESC);
CREATE INDEX idx_activity_logs_resource ON activity_logs(resource_type, resource_id);
CREATE INDEX idx_activity_logs_actor ON activity_logs(actor_id, created_at DESC);

-- ============================================================
-- PHASE 14: Drop accounts table
-- ============================================================

DROP INDEX IF EXISTS idx_accounts_api_key;
DROP INDEX IF EXISTS idx_accounts_tenant_id;
DROP TABLE accounts;

-- ============================================================
-- PHASE 15: Clean up users table
-- ============================================================
-- Remove tenant_id and role columns (data already in group_members).

DROP INDEX IF EXISTS idx_users_tenant_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users DROP COLUMN tenant_id;
ALTER TABLE users DROP COLUMN role;

-- ============================================================
-- PHASE 16: Recreate RLS policies using app.current_group_id
-- ============================================================

-- groups (was tenants)
CREATE POLICY group_isolation ON groups
    USING (id = current_setting('app.current_group_id', true)::UUID);

-- users: accessible via group_members join (no direct group_id on users)
-- RLS on users uses group_members to check membership.
CREATE POLICY user_group_isolation ON users
    USING (
        EXISTS (
            SELECT 1 FROM group_members gm
            WHERE gm.user_id = users.id
              AND gm.group_id = current_setting('app.current_group_id', true)::UUID
        )
    );

-- group_members
ALTER TABLE group_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY group_member_isolation ON group_members
    USING (group_id = current_setting('app.current_group_id', true)::UUID);

-- sessions
CREATE POLICY session_group_isolation ON sessions
    USING (group_id = current_setting('app.current_group_id', true)::UUID);

-- activity_logs
ALTER TABLE activity_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY activity_log_group_isolation ON activity_logs
    USING (group_id = current_setting('app.current_group_id', true)::UUID);

-- esp_providers
CREATE POLICY provider_group_isolation ON esp_providers
    USING (group_id = current_setting('app.current_group_id', true)::UUID);

-- routing_rules
CREATE POLICY rule_group_isolation ON routing_rules
    USING (group_id = current_setting('app.current_group_id', true)::UUID);

-- messages
CREATE POLICY message_group_isolation ON messages
    USING (group_id = current_setting('app.current_group_id', true)::UUID OR group_id IS NULL);

-- delivery_logs
CREATE POLICY delivery_log_group_isolation ON delivery_logs
    USING (group_id = current_setting('app.current_group_id', true)::UUID OR group_id IS NULL);

-- ============================================================
-- PHASE 17: Cleanup
-- ============================================================

DROP TABLE IF EXISTS _account_user_map;

COMMIT;
