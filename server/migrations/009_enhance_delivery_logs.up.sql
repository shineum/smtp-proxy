-- Add account_id for direct account association
ALTER TABLE delivery_logs ADD COLUMN account_id UUID REFERENCES accounts(id);

-- Add duration_ms for delivery timing metrics
ALTER TABLE delivery_logs ADD COLUMN duration_ms INTEGER;

-- Add attempt_number for retry tracking
ALTER TABLE delivery_logs ADD COLUMN attempt_number INTEGER NOT NULL DEFAULT 1;

-- Drop UNIQUE constraint on message_id (allow multiple logs per message for retries)
ALTER TABLE delivery_logs DROP CONSTRAINT IF EXISTS delivery_logs_message_id_unique;

-- Make provider_id nullable (we use provider name string now)
ALTER TABLE delivery_logs ALTER COLUMN provider_id DROP NOT NULL;

-- Add index for account-level time-range queries
CREATE INDEX idx_delivery_logs_account_created ON delivery_logs(account_id, created_at)
    WHERE account_id IS NOT NULL;

-- Add index for provider + status aggregation queries
CREATE INDEX idx_delivery_logs_provider_status ON delivery_logs(provider, status);
