-- Create delivery status enum
CREATE TYPE delivery_status AS ENUM (
    'pending',
    'queued',
    'processing',
    'sent',
    'failed',
    'bounced',
    'complained'
);

-- Add new columns to delivery_logs
ALTER TABLE delivery_logs
    ADD COLUMN tenant_id VARCHAR(255),
    ADD COLUMN provider VARCHAR(50),
    ADD COLUMN provider_message_id VARCHAR(255),
    ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_error TEXT,
    ADD COLUMN metadata JSONB,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Add UNIQUE constraint on message_id
ALTER TABLE delivery_logs
    ADD CONSTRAINT delivery_logs_message_id_unique UNIQUE (message_id);

-- Add composite index for tenant + status lookups
CREATE INDEX idx_delivery_logs_tenant_status ON delivery_logs(tenant_id, status);

-- Add index for time-based queries (newest first)
CREATE INDEX idx_delivery_logs_created_at ON delivery_logs(created_at DESC);

-- Add partial index for provider message ID lookups (only non-null values)
CREATE INDEX idx_delivery_logs_provider_message_id ON delivery_logs(provider_message_id)
    WHERE provider_message_id IS NOT NULL;
