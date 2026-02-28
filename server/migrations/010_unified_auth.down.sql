-- Migration 010 DOWN: Reverse Unified Account System Redesign
--
-- WARNING: This rollback is LOSSY.
--   - Multi-group memberships collapse to a single tenant_id (first group wins)
--   - activity_logs are dropped (audit_logs recreated empty)
--   - SMTP accounts extracted back from users into accounts table
--
-- Data loss areas:
--   - group_members rows beyond the first group per user
--   - activity_logs content (audit_logs schema differs)
--   - Any users that belonged to multiple groups lose those memberships

BEGIN;

-- ============================================================
-- PHASE 1: Drop new RLS policies
-- ============================================================

DROP POLICY IF EXISTS delivery_log_group_isolation ON delivery_logs;
DROP POLICY IF EXISTS message_group_isolation ON messages;
DROP POLICY IF EXISTS rule_group_isolation ON routing_rules;
DROP POLICY IF EXISTS provider_group_isolation ON esp_providers;
DROP POLICY IF EXISTS activity_log_group_isolation ON activity_logs;
DROP POLICY IF EXISTS session_group_isolation ON sessions;
DROP POLICY IF EXISTS group_member_isolation ON group_members;
DROP POLICY IF EXISTS user_group_isolation ON users;
DROP POLICY IF EXISTS group_isolation ON groups;

ALTER TABLE activity_logs DISABLE ROW LEVEL SECURITY;
ALTER TABLE group_members DISABLE ROW LEVEL SECURITY;

-- ============================================================
-- PHASE 2: Recreate accounts table
-- ============================================================

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    allowed_domains JSONB NOT NULL DEFAULT '[]'::jsonb,
    api_key VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_api_key ON accounts(api_key);

-- ============================================================
-- PHASE 3: Extract SMTP users back into accounts
-- ============================================================

CREATE TABLE _account_user_map (
    account_id UUID NOT NULL PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE
);

-- Insert SMTP users back as accounts.
-- username -> accounts.name, synthetic email stripped.
INSERT INTO accounts (name, email, password_hash, allowed_domains, api_key, created_at, updated_at)
SELECT
    u.username,
    COALESCE(u.email, u.username || '@smtp.internal'),
    u.password_hash,
    COALESCE(u.allowed_domains, '[]'::jsonb),
    LEFT(COALESCE(u.api_key, encode(gen_random_bytes(32), 'hex')), 64),
    u.created_at,
    u.updated_at
FROM users u
WHERE u.account_type = 'smtp'
  AND u.username IS NOT NULL;

-- Build mapping: accounts.name = users.username for SMTP accounts
INSERT INTO _account_user_map (account_id, user_id)
SELECT a.id, u.id
FROM accounts a
JOIN users u ON u.username = a.name AND u.account_type = 'smtp';

-- ============================================================
-- PHASE 4: Restore tenant_id and role on users
-- ============================================================

ALTER TABLE users ADD COLUMN tenant_id UUID;
ALTER TABLE users ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'member';

-- Populate from group_members (pick first group per user, LOSSY)
UPDATE users u
SET tenant_id = gm.group_id,
    role = gm.role
FROM (
    SELECT DISTINCT ON (user_id) user_id, group_id, role
    FROM group_members
    ORDER BY user_id, created_at ASC
) gm
WHERE u.id = gm.user_id;

-- Add FK constraint (groups will be renamed back to tenants later)
ALTER TABLE users
    ADD CONSTRAINT users_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES groups(id) ON DELETE CASCADE;

ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('owner', 'admin', 'member'));

CREATE INDEX idx_users_tenant_id ON users(tenant_id);

-- Add tenant_id to accounts from group_members mapping
ALTER TABLE accounts ADD COLUMN tenant_id UUID;
UPDATE accounts a
SET tenant_id = gm.group_id
FROM _account_user_map m
JOIN group_members gm ON gm.user_id = m.user_id
WHERE a.id = m.account_id;

-- FK will point to groups (renamed to tenants in a later phase)
ALTER TABLE accounts
    ADD CONSTRAINT accounts_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES groups(id) ON DELETE CASCADE;

CREATE INDEX idx_accounts_tenant_id ON accounts(tenant_id);

-- ============================================================
-- PHASE 5: Restore esp_providers
-- ============================================================
-- Current: group_id UUID NOT NULL FK groups
-- Target:  account_id UUID NOT NULL FK accounts, tenant_id UUID FK groups

ALTER TABLE esp_providers ADD COLUMN account_id UUID;
ALTER TABLE esp_providers ADD COLUMN tenant_id UUID;

-- Set tenant_id = group_id (groups will become tenants)
UPDATE esp_providers SET tenant_id = group_id;

-- Pick an account in the same group for account_id
UPDATE esp_providers e
SET account_id = (
    SELECT a.id FROM accounts a
    WHERE a.tenant_id = e.group_id
    LIMIT 1
);

-- If no account found in that group, pick any account
UPDATE esp_providers e
SET account_id = (SELECT id FROM accounts LIMIT 1)
WHERE account_id IS NULL;

-- If still NULL (no accounts exist at all), this will fail.
-- Create a placeholder account if needed.
INSERT INTO accounts (name, email, password_hash, api_key)
SELECT '_placeholder_' || gen_random_uuid()::text,
       'placeholder@internal',
       'no-login',
       encode(gen_random_bytes(32), 'hex')
WHERE EXISTS (SELECT 1 FROM esp_providers WHERE account_id IS NULL)
AND NOT EXISTS (SELECT 1 FROM accounts WHERE name LIKE '_placeholder_%');

UPDATE esp_providers
SET account_id = (SELECT id FROM accounts LIMIT 1)
WHERE account_id IS NULL;

ALTER TABLE esp_providers ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE esp_providers
    ADD CONSTRAINT esp_providers_account_id_fkey
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE esp_providers
    ADD CONSTRAINT esp_providers_tenant_id_fkey_restored
    FOREIGN KEY (tenant_id) REFERENCES groups(id) ON DELETE CASCADE;

-- Drop group_id
DROP INDEX IF EXISTS idx_esp_providers_group_id;
ALTER TABLE esp_providers DROP COLUMN group_id;

CREATE INDEX idx_esp_providers_tenant_id ON esp_providers(tenant_id);

-- ============================================================
-- PHASE 6: Restore routing_rules
-- ============================================================

ALTER TABLE routing_rules ADD COLUMN account_id UUID;
ALTER TABLE routing_rules ADD COLUMN tenant_id UUID;

UPDATE routing_rules SET tenant_id = group_id;

UPDATE routing_rules r
SET account_id = (
    SELECT a.id FROM accounts a
    WHERE a.tenant_id = r.group_id
    LIMIT 1
);

UPDATE routing_rules
SET account_id = (SELECT id FROM accounts LIMIT 1)
WHERE account_id IS NULL;

ALTER TABLE routing_rules ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE routing_rules
    ADD CONSTRAINT routing_rules_account_id_fkey
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE routing_rules
    ADD CONSTRAINT routing_rules_tenant_id_fkey_restored
    FOREIGN KEY (tenant_id) REFERENCES groups(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_routing_rules_group_id;
DROP INDEX IF EXISTS idx_routing_rules_group_priority;
ALTER TABLE routing_rules DROP COLUMN group_id;

CREATE INDEX idx_routing_rules_tenant_id ON routing_rules(tenant_id);
CREATE INDEX idx_routing_rules_account_priority ON routing_rules(account_id, priority);

-- ============================================================
-- PHASE 7: Restore messages
-- ============================================================
-- Current: user_id UUID FK users, group_id UUID FK groups
-- Target:  account_id UUID NOT NULL FK accounts, tenant_id UUID FK groups

ALTER TABLE messages ADD COLUMN account_id UUID;
ALTER TABLE messages ADD COLUMN tenant_id UUID;

UPDATE messages SET tenant_id = group_id;

-- Map user_id back to account_id via mapping
UPDATE messages m
SET account_id = aum.account_id
FROM _account_user_map aum
WHERE m.user_id = aum.user_id;

-- For messages without a mapped account, pick an account in the same group
UPDATE messages m
SET account_id = (
    SELECT a.id FROM accounts a
    WHERE a.tenant_id = m.group_id
    LIMIT 1
)
WHERE account_id IS NULL;

-- Fallback: any account
UPDATE messages
SET account_id = (SELECT id FROM accounts LIMIT 1)
WHERE account_id IS NULL;

ALTER TABLE messages ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE messages
    ADD CONSTRAINT messages_account_id_fkey
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE messages
    ADD CONSTRAINT messages_tenant_id_fkey_restored
    FOREIGN KEY (tenant_id) REFERENCES groups(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_messages_group_id;
DROP INDEX IF EXISTS idx_messages_user_id;
DROP INDEX IF EXISTS idx_messages_user_status;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_user_id_fkey;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_group_id_fkey;
ALTER TABLE messages DROP COLUMN user_id;
ALTER TABLE messages DROP COLUMN group_id;

CREATE INDEX idx_messages_account_status ON messages(account_id, status);
CREATE INDEX idx_messages_tenant_id ON messages(tenant_id);

-- ============================================================
-- PHASE 8: Restore delivery_logs
-- ============================================================
-- Current: user_id UUID FK users, group_id UUID FK groups
-- Target:  account_id UUID FK accounts, mt_tenant_id UUID FK groups,
--          tenant_id VARCHAR(255)

ALTER TABLE delivery_logs ADD COLUMN account_id UUID;
ALTER TABLE delivery_logs ADD COLUMN mt_tenant_id UUID;
ALTER TABLE delivery_logs ADD COLUMN tenant_id VARCHAR(255);

-- Populate mt_tenant_id from group_id
UPDATE delivery_logs SET mt_tenant_id = group_id;

-- Map user_id back to account_id
UPDATE delivery_logs dl
SET account_id = aum.account_id
FROM _account_user_map aum
WHERE dl.user_id = aum.user_id;

ALTER TABLE delivery_logs
    ADD CONSTRAINT delivery_logs_account_id_fkey_restored
    FOREIGN KEY (account_id) REFERENCES accounts(id);
ALTER TABLE delivery_logs
    ADD CONSTRAINT delivery_logs_mt_tenant_id_fkey_restored
    FOREIGN KEY (mt_tenant_id) REFERENCES groups(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_delivery_logs_group_id;
DROP INDEX IF EXISTS idx_delivery_logs_user_id;
DROP INDEX IF EXISTS idx_delivery_logs_group_status;
DROP INDEX IF EXISTS idx_delivery_logs_user_created;
ALTER TABLE delivery_logs DROP CONSTRAINT IF EXISTS delivery_logs_user_id_fkey;
ALTER TABLE delivery_logs DROP CONSTRAINT IF EXISTS delivery_logs_group_id_fkey;
ALTER TABLE delivery_logs DROP COLUMN user_id;
ALTER TABLE delivery_logs DROP COLUMN group_id;

CREATE INDEX idx_delivery_logs_mt_tenant_id ON delivery_logs(mt_tenant_id);
CREATE INDEX idx_delivery_logs_tenant_status ON delivery_logs(tenant_id, status);
CREATE INDEX idx_delivery_logs_account_created ON delivery_logs(account_id, created_at)
    WHERE account_id IS NOT NULL;

-- ============================================================
-- PHASE 9: Restore sessions (rename group_id -> tenant_id)
-- ============================================================

DROP INDEX IF EXISTS idx_sessions_group_id;
ALTER TABLE sessions RENAME COLUMN group_id TO tenant_id;
CREATE INDEX idx_sessions_tenant_id ON sessions(tenant_id);

-- ============================================================
-- PHASE 10: Replace activity_logs with audit_logs
-- ============================================================

DROP TABLE activity_logs;

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    result VARCHAR(20) NOT NULL CHECK (result IN ('success', 'failure')),
    metadata JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

-- ============================================================
-- PHASE 11: Rename groups -> tenants and restore defaults
-- ============================================================

ALTER TABLE groups DROP COLUMN group_type;
ALTER TABLE groups ALTER COLUMN monthly_limit SET DEFAULT 10000;
ALTER TABLE groups RENAME TO tenants;

-- ============================================================
-- PHASE 12: Delete SMTP users and remove added columns
-- ============================================================

DELETE FROM users WHERE account_type = 'smtp';

ALTER TABLE users DROP COLUMN IF EXISTS allowed_domains;
ALTER TABLE users DROP COLUMN IF EXISTS api_key;
ALTER TABLE users DROP COLUMN IF EXISTS account_type;
ALTER TABLE users DROP COLUMN IF EXISTS username;

-- ============================================================
-- PHASE 13: Drop group_members table
-- ============================================================

DROP TABLE IF EXISTS group_members;

-- ============================================================
-- PHASE 14: Recreate original RLS policies
-- ============================================================

-- tenants
CREATE POLICY tenant_isolation ON tenants
    USING (id = current_setting('app.current_tenant_id', true)::UUID);

-- users
CREATE POLICY user_tenant_isolation ON users
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- sessions
CREATE POLICY session_tenant_isolation ON sessions
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- audit_logs
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_tenant_isolation ON audit_logs
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- accounts
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY account_tenant_isolation ON accounts
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID OR tenant_id IS NULL);

-- esp_providers
CREATE POLICY provider_tenant_isolation ON esp_providers
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID OR tenant_id IS NULL);

-- routing_rules
CREATE POLICY rule_tenant_isolation ON routing_rules
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID OR tenant_id IS NULL);

-- messages
CREATE POLICY message_tenant_isolation ON messages
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID OR tenant_id IS NULL);

-- delivery_logs
CREATE POLICY delivery_log_tenant_isolation ON delivery_logs
    USING (mt_tenant_id = current_setting('app.current_tenant_id', true)::UUID OR mt_tenant_id IS NULL);

-- ============================================================
-- PHASE 15: Cleanup
-- ============================================================

DROP TABLE IF EXISTS _account_user_map;

COMMIT;
