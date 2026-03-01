-- Migration 011: Performance indexes for stats and message queries
--
-- Adds composite indexes to optimize dashboard statistics aggregation
-- and paginated message listing in the admin UI.

-- Composite index for delivery stats aggregation by group and date range
CREATE INDEX IF NOT EXISTS idx_delivery_logs_group_created
    ON delivery_logs(group_id, created_at)
    WHERE group_id IS NOT NULL;

-- Composite index for paginated message listing filtered by group, status, and date
CREATE INDEX IF NOT EXISTS idx_messages_group_status_enqueued
    ON messages(group_id, status, enqueued_at DESC)
    WHERE group_id IS NOT NULL;
