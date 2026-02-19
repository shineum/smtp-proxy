-- Drop RLS policies
DROP POLICY IF EXISTS delivery_log_tenant_isolation ON delivery_logs;
DROP POLICY IF EXISTS message_tenant_isolation ON messages;
DROP POLICY IF EXISTS rule_tenant_isolation ON routing_rules;
DROP POLICY IF EXISTS provider_tenant_isolation ON esp_providers;
DROP POLICY IF EXISTS account_tenant_isolation ON accounts;
DROP POLICY IF EXISTS audit_tenant_isolation ON audit_logs;
DROP POLICY IF EXISTS session_tenant_isolation ON sessions;
DROP POLICY IF EXISTS user_tenant_isolation ON users;
DROP POLICY IF EXISTS tenant_isolation ON tenants;

-- Disable RLS
ALTER TABLE delivery_logs DISABLE ROW LEVEL SECURITY;
ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE routing_rules DISABLE ROW LEVEL SECURITY;
ALTER TABLE esp_providers DISABLE ROW LEVEL SECURITY;
ALTER TABLE accounts DISABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs DISABLE ROW LEVEL SECURITY;
ALTER TABLE sessions DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE tenants DISABLE ROW LEVEL SECURITY;

-- Drop indexes on tenant_id columns
DROP INDEX IF EXISTS idx_delivery_logs_mt_tenant_id;
DROP INDEX IF EXISTS idx_messages_tenant_id;
DROP INDEX IF EXISTS idx_routing_rules_tenant_id;
DROP INDEX IF EXISTS idx_esp_providers_tenant_id;
DROP INDEX IF EXISTS idx_accounts_tenant_id;
DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_audit_logs_tenant_id;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_tenant_id;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_tenant_id;

-- Remove tenant_id from existing tables
ALTER TABLE delivery_logs DROP COLUMN IF EXISTS mt_tenant_id;
ALTER TABLE messages DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE routing_rules DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE esp_providers DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS tenant_id;

-- Drop new tables
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;
