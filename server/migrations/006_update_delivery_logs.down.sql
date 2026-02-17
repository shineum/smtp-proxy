-- Drop indexes
DROP INDEX IF EXISTS idx_delivery_logs_provider_message_id;
DROP INDEX IF EXISTS idx_delivery_logs_created_at;
DROP INDEX IF EXISTS idx_delivery_logs_tenant_status;

-- Drop unique constraint
ALTER TABLE delivery_logs
    DROP CONSTRAINT IF EXISTS delivery_logs_message_id_unique;

-- Drop added columns
ALTER TABLE delivery_logs
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS provider_message_id,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS tenant_id;

-- Drop delivery status enum
DROP TYPE IF EXISTS delivery_status;
