-- Anonymous SMTP submission: per-account opt-in with strict CIDR allowlist.
--
-- When a SMTP service account has anonymous_allowed=true, clients connecting
-- from a source IP that falls inside one of anonymous_allowed_cidrs may
-- submit messages without issuing AUTH. The account's username, group, and
-- allowed_domains are resolved server-side from the source address.
--
-- Safety: the CHECK constraint blocks enabling anonymous mode with an empty
-- CIDR list — that combination would be an open relay.
ALTER TABLE users
    ADD COLUMN anonymous_allowed BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN anonymous_allowed_cidrs JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE users
    ADD CONSTRAINT users_anonymous_cidrs_required
    CHECK (
        anonymous_allowed = false
        OR jsonb_typeof(anonymous_allowed_cidrs) = 'array'
        AND jsonb_array_length(anonymous_allowed_cidrs) > 0
    );

-- Partial index accelerates the per-connection IP lookup for anonymous SMTP
-- sessions. Only active SMTP accounts with anonymous mode enabled qualify.
CREATE INDEX IF NOT EXISTS idx_users_anonymous_smtp
    ON users (id)
    WHERE account_type = 'smtp'
      AND status = 'active'
      AND anonymous_allowed = true;
