CREATE TABLE routing_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0,
    conditions JSONB NOT NULL DEFAULT '{}'::jsonb,
    provider_id UUID NOT NULL REFERENCES esp_providers(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_rules_account_priority ON routing_rules(account_id, priority);
