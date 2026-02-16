CREATE TYPE provider_type AS ENUM ('sendgrid', 'mailgun', 'ses', 'smtp', 'msgraph');

CREATE TABLE esp_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider_type provider_type NOT NULL,
    api_key VARCHAR(512),
    smtp_config JSONB,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
