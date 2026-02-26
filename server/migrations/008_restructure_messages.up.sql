-- Add new enum values to message_status
ALTER TYPE message_status ADD VALUE IF NOT EXISTS 'enqueue_failed';
ALTER TYPE message_status ADD VALUE IF NOT EXISTS 'storage_error';

-- Add storage_ref column for external body storage
ALTER TABLE messages ADD COLUMN storage_ref TEXT;

-- Make body nullable (new messages use storage_ref instead)
ALTER TABLE messages ALTER COLUMN body DROP NOT NULL;
