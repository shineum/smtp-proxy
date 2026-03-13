-- Migration 023: Replace UUID primary keys with SERIAL/BIGSERIAL
-- Data reset: all tables are dropped and recreated.
-- Enum types are preserved (they survive table drops).

BEGIN;

-- ============================================================
-- PHASE 1: Drop all tables in dependency order
-- ============================================================
DROP TABLE IF EXISTS delivery_logs CASCADE;
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS routing_rules CASCADE;
DROP TABLE IF EXISTS provider_group_access CASCADE;
DROP TABLE IF EXISTS activity_logs CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS group_members CASCADE;
DROP TABLE IF EXISTS esp_providers CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS groups CASCADE;

-- ============================================================
-- PHASE 2: Recreate tables with SERIAL/BIGSERIAL PKs
-- ============================================================

-- groups (formerly tenants)
CREATE TABLE groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    group_type VARCHAR(20) NOT NULL DEFAULT 'company',
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'deleted')),
    monthly_limit INTEGER NOT NULL DEFAULT 0,
    monthly_sent INTEGER NOT NULL DEFAULT 0,
    allowed_ips CIDR[],
    display_name VARCHAR(255),
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- users
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL DEFAULT '',
    username VARCHAR(255),
    account_type VARCHAR(20) NOT NULL DEFAULT 'human',
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'deleted')),
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    allowed_domains JSONB DEFAULT '[]',
    password_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    home_group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL,
    display_name VARCHAR(255),
    description TEXT,
    api_key VARCHAR(255) UNIQUE,
    api_key_expires_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- esp_providers
CREATE TABLE esp_providers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    provider_type provider_type NOT NULL,
    api_key VARCHAR(255),
    smtp_config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    visibility provider_visibility NOT NULL DEFAULT 'private',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add provider_id FK to users (after esp_providers is created)
ALTER TABLE users ADD COLUMN provider_id INTEGER REFERENCES esp_providers(id) ON DELETE SET NULL;

-- group_members (composite PK, no synthetic id)
CREATE TABLE group_members (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

-- sessions
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- routing_rules
CREATE TABLE routing_rules (
    id SERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0,
    conditions JSONB NOT NULL DEFAULT '{}',
    provider_id INTEGER NOT NULL REFERENCES esp_providers(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- messages (high volume: BIGSERIAL)
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    sender VARCHAR(255) NOT NULL,
    recipients JSONB NOT NULL,
    subject VARCHAR(998),
    headers JSONB DEFAULT '{}',
    body TEXT,
    storage_ref TEXT,
    status message_status NOT NULL DEFAULT 'queued',
    provider_id INTEGER REFERENCES esp_providers(id) ON DELETE SET NULL,
    group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    enqueued_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ
);

-- delivery_logs (high volume: BIGSERIAL)
CREATE TABLE delivery_logs (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    provider_id INTEGER REFERENCES esp_providers(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    response_code INTEGER,
    response_body TEXT,
    delivered_at TIMESTAMPTZ,
    provider VARCHAR(100),
    provider_message_id VARCHAR(255),
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    metadata JSONB,
    duration_ms INTEGER,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- activity_logs (high volume: BIGSERIAL)
CREATE TABLE activity_logs (
    id BIGSERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    actor_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id INTEGER,
    changes JSONB,
    comment TEXT,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- provider_group_access (composite PK)
CREATE TABLE provider_group_access (
    provider_id INTEGER NOT NULL REFERENCES esp_providers(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (provider_id, group_id)
);

-- ============================================================
-- PHASE 3: Indexes
-- ============================================================

-- users
CREATE UNIQUE INDEX idx_users_email_human ON users(email) WHERE account_type != 'smtp';
CREATE UNIQUE INDEX idx_users_username ON users(username) WHERE username IS NOT NULL;
CREATE INDEX idx_users_home_group_id ON users(home_group_id) WHERE home_group_id IS NOT NULL;
CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_smtp_auth_username_group ON users(username, home_group_id)
    WHERE account_type = 'smtp' AND username IS NOT NULL;

-- group_members
CREATE INDEX idx_group_members_user_id ON group_members(user_id);

-- sessions
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_group_id ON sessions(group_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- esp_providers
CREATE INDEX idx_esp_providers_group_id ON esp_providers(group_id);

-- routing_rules
CREATE INDEX idx_routing_rules_group_id ON routing_rules(group_id);
CREATE INDEX idx_routing_rules_group_priority ON routing_rules(group_id, priority);

-- messages
CREATE INDEX idx_messages_group_id ON messages(group_id);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_user_status ON messages(user_id, status);
CREATE INDEX idx_messages_status ON messages(status);

-- delivery_logs
CREATE INDEX idx_delivery_logs_message_id ON delivery_logs(message_id);
CREATE INDEX idx_delivery_logs_group_id ON delivery_logs(group_id);
CREATE INDEX idx_delivery_logs_group_status ON delivery_logs(group_id, status);
CREATE INDEX idx_delivery_logs_group_created ON delivery_logs(group_id, created_at);
CREATE INDEX idx_delivery_logs_provider_msg_id ON delivery_logs(provider_message_id)
    WHERE provider_message_id IS NOT NULL;
CREATE INDEX idx_delivery_logs_status_created ON delivery_logs(status, created_at);

-- activity_logs
CREATE INDEX idx_activity_logs_group_created ON activity_logs(group_id, created_at DESC);
CREATE INDEX idx_activity_logs_resource ON activity_logs(resource_type, resource_id);
CREATE INDEX idx_activity_logs_actor ON activity_logs(actor_id, created_at DESC);

-- provider_group_access
CREATE INDEX idx_provider_group_access_group ON provider_group_access(group_id);

-- ============================================================
-- PHASE 4: RLS policies (app user bypasses, but kept for safety)
-- ============================================================

ALTER TABLE groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE esp_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE routing_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE delivery_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE activity_logs ENABLE ROW LEVEL SECURITY;

CREATE POLICY group_isolation ON groups
    USING (id = current_setting('app.current_group_id', true)::INTEGER);
CREATE POLICY user_group_isolation ON users
    USING (EXISTS (
        SELECT 1 FROM group_members gm
        WHERE gm.user_id = users.id
          AND gm.group_id = current_setting('app.current_group_id', true)::INTEGER
    ));
CREATE POLICY group_member_isolation ON group_members
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER);
CREATE POLICY session_group_isolation ON sessions
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER);
CREATE POLICY provider_group_isolation ON esp_providers
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER);
CREATE POLICY rule_group_isolation ON routing_rules
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER);
CREATE POLICY message_group_isolation ON messages
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER OR group_id IS NULL);
CREATE POLICY delivery_log_group_isolation ON delivery_logs
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER OR group_id IS NULL);
CREATE POLICY activity_log_group_isolation ON activity_logs
    USING (group_id = current_setting('app.current_group_id', true)::INTEGER);

COMMIT;
