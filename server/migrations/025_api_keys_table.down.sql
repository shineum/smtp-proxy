ALTER TABLE users ADD COLUMN api_key VARCHAR(255);
ALTER TABLE users ADD COLUMN api_key_expires_at TIMESTAMPTZ;

-- Restore api_keys (take the most recent key per user)
UPDATE users u SET
    api_key = ak.key_hash,
    api_key_expires_at = ak.expires_at
FROM (
    SELECT DISTINCT ON (user_id) user_id, key_hash, expires_at
    FROM api_keys ORDER BY user_id, created_at DESC
) ak
WHERE u.id = ak.user_id;

DROP TABLE api_keys;
