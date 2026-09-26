-- +goose Up
CREATE TABLE IF NOT EXISTS canonical_usage_outbox (
    event_id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    payload JSONB NOT NULL,
    payload_sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    CONSTRAINT canonical_usage_outbox_request_identity_unique UNIQUE (request_id, api_key_id),
    CONSTRAINT canonical_usage_outbox_payload_sha256_length CHECK (length(payload_sha256) = 64),
    CONSTRAINT canonical_usage_outbox_status_check CHECK (status IN ('pending', 'published', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_canonical_usage_outbox_pending
    ON canonical_usage_outbox (created_at, event_id)
    WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS canonical_usage_outbox;
