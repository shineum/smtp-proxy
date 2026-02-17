CREATE TYPE message_status AS ENUM ('queued', 'processing', 'delivered', 'failed');

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    sender VARCHAR(255) NOT NULL,
    recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
    subject VARCHAR(998),
    headers JSONB,
    body TEXT NOT NULL,
    status message_status NOT NULL DEFAULT 'queued',
    provider_id UUID REFERENCES esp_providers(id),
    enqueued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_messages_account_status ON messages(account_id, status);
