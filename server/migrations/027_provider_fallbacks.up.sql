-- Provider fallbacks: ordered list of backup ESP providers per user.
-- When the primary provider fails with a transient error, the worker
-- tries fallback providers in priority order.
CREATE TABLE IF NOT EXISTS provider_fallbacks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES esp_providers(id) ON DELETE CASCADE,
    priority   INT NOT NULL DEFAULT 0,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_provider_fallbacks_user_id ON provider_fallbacks (user_id, priority);
