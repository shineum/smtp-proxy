CREATE TABLE delivery_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES esp_providers(id),
    status VARCHAR(50) NOT NULL,
    response_code INTEGER,
    response_body TEXT,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delivery_logs_message ON delivery_logs(message_id);
