-- Domain rate limits: configurable per-destination throttling to prevent
-- spam classification by receiving mail servers.
CREATE TABLE IF NOT EXISTS domain_rate_limits (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id       UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    domain         TEXT NOT NULL,
    max_per_minute INT NOT NULL DEFAULT 0,
    max_per_hour   INT NOT NULL DEFAULT 0,
    enabled        BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (group_id, domain)
);

CREATE INDEX idx_domain_rate_limits_group_id ON domain_rate_limits (group_id);
