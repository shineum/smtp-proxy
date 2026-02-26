-- Drop indexes
DROP INDEX IF EXISTS idx_delivery_logs_provider_status;
DROP INDEX IF EXISTS idx_delivery_logs_account_created;

-- Restore provider_id NOT NULL (backfill any NULLs first)
-- Note: This may fail if there are rows with NULL provider_id.
-- In production, ensure provider_id is populated before rolling back.
UPDATE delivery_logs SET provider_id = (SELECT id FROM esp_providers LIMIT 1) WHERE provider_id IS NULL;
ALTER TABLE delivery_logs ALTER COLUMN provider_id SET NOT NULL;

-- Restore UNIQUE constraint on message_id
-- Note: This may fail if duplicate message_id rows exist from retries.
-- Delete duplicate rows first, keeping only the latest per message_id.
DELETE FROM delivery_logs a
    USING delivery_logs b
    WHERE a.message_id = b.message_id
      AND a.id < b.id;
ALTER TABLE delivery_logs ADD CONSTRAINT delivery_logs_message_id_unique UNIQUE (message_id);

-- Drop added columns
ALTER TABLE delivery_logs
    DROP COLUMN IF EXISTS attempt_number,
    DROP COLUMN IF EXISTS duration_ms,
    DROP COLUMN IF EXISTS account_id;
