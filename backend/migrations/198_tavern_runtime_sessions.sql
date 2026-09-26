-- Runtime sessions give the tavern stage a short-lived launch credential.
-- The clear token is returned once to the browser; only its SHA-256 hash is stored.

CREATE TABLE IF NOT EXISTS tavern_runtime_sessions (
    id BIGSERIAL PRIMARY KEY,
    token_hash CHAR(64) NOT NULL UNIQUE,
    room_id BIGINT NOT NULL REFERENCES tavern_rooms(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    runtime_id VARCHAR(160) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tavern_runtime_sessions_status_check CHECK (status IN ('active', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_tavern_runtime_sessions_user_active
    ON tavern_runtime_sessions(user_id, status, expires_at DESC);

CREATE INDEX IF NOT EXISTS idx_tavern_runtime_sessions_room_active
    ON tavern_runtime_sessions(room_id, status, expires_at DESC);

COMMENT ON TABLE tavern_runtime_sessions IS 'Short-lived Zero City Tavern stage sessions. Stores token hashes only; runtime source code remains isolated outside BizDecipher.';
COMMENT ON COLUMN tavern_runtime_sessions.runtime_id IS 'Stable runtime identity derived from room and user, not a secret.';
