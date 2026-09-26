-- 225: Durable, user-visible observability for shared-pool API calls.
--
-- Safety properties:
--   * additive only; no balance, ledger, key, seat, pool, or historical row is changed;
--   * pool/account/key identity is snapshotted without foreign keys so an archived
--     resource cannot erase the audit trail;
--   * account_alias is a generated safe label. Upstream URL, API key, OAuth data,
--     proxy details, and raw errors must never be written to this table.

CREATE TABLE IF NOT EXISTS shared_pool_usage_traces (
    id                       BIGSERIAL PRIMARY KEY,
    access_key_id            BIGINT NOT NULL,
    pool_id                  BIGINT NOT NULL,
    user_id                  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id               VARCHAR(128) NOT NULL,
    pool_name_snapshot       VARCHAR(160) NOT NULL DEFAULT '',
    model_snapshot           VARCHAR(128) NOT NULL DEFAULT '',
    endpoint                 VARCHAR(128) NOT NULL DEFAULT '',
    account_alias            VARCHAR(64) NOT NULL DEFAULT '',
    status                   VARCHAR(20) NOT NULL DEFAULT 'pending',
    failure_stage            VARCHAR(32) NOT NULL DEFAULT '',
    settlement_outcome       VARCHAR(20) NOT NULL DEFAULT 'pending',
    upstream_started         BOOLEAN NOT NULL DEFAULT FALSE,
    usage_observed           BOOLEAN NOT NULL DEFAULT FALSE,
    input_tokens             INT NOT NULL DEFAULT 0,
    output_tokens            INT NOT NULL DEFAULT 0,
    cache_read_tokens        INT NOT NULL DEFAULT 0,
    cache_creation_tokens    INT NOT NULL DEFAULT 0,
    auth_latency_ms          BIGINT NULL,
    seat_latency_ms          BIGINT NULL,
    routing_latency_ms       BIGINT NULL,
    concurrency_latency_ms   BIGINT NULL,
    reservation_latency_ms   BIGINT NULL,
    upstream_latency_ms      BIGINT NULL,
    first_token_ms           BIGINT NULL,
    settlement_latency_ms    BIGINT NULL,
    total_latency_ms         BIGINT NULL,
    retry_count              INT NOT NULL DEFAULT 0,
    http_status              INT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at             TIMESTAMPTZ NULL,
    CONSTRAINT shared_pool_usage_traces_request_unique UNIQUE (access_key_id, request_id),
    CONSTRAINT shared_pool_usage_traces_status_check
        CHECK (status IN ('pending', 'succeeded', 'failed')),
    CONSTRAINT shared_pool_usage_traces_settlement_check
        CHECK (settlement_outcome IN ('pending', 'settled', 'released', 'failed', 'not_required', 'unknown')),
    CONSTRAINT shared_pool_usage_traces_token_check
        CHECK (input_tokens >= 0 AND output_tokens >= 0 AND cache_read_tokens >= 0 AND cache_creation_tokens >= 0),
    CONSTRAINT shared_pool_usage_traces_retry_check CHECK (retry_count >= 0),
    CONSTRAINT shared_pool_usage_traces_http_status_check
        CHECK (http_status IS NULL OR (http_status >= 100 AND http_status <= 599)),
    CONSTRAINT shared_pool_usage_traces_latency_check CHECK (
        (auth_latency_ms IS NULL OR auth_latency_ms >= 0) AND
        (seat_latency_ms IS NULL OR seat_latency_ms >= 0) AND
        (routing_latency_ms IS NULL OR routing_latency_ms >= 0) AND
        (concurrency_latency_ms IS NULL OR concurrency_latency_ms >= 0) AND
        (reservation_latency_ms IS NULL OR reservation_latency_ms >= 0) AND
        (upstream_latency_ms IS NULL OR upstream_latency_ms >= 0) AND
        (first_token_ms IS NULL OR first_token_ms >= 0) AND
        (settlement_latency_ms IS NULL OR settlement_latency_ms >= 0) AND
        (total_latency_ms IS NULL OR total_latency_ms >= 0)
    )
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_traces_user_created
    ON shared_pool_usage_traces(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_traces_pool_created
    ON shared_pool_usage_traces(pool_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_traces_request
    ON shared_pool_usage_traces(request_id);

COMMENT ON TABLE shared_pool_usage_traces IS
    'Safe shared-pool request trace. Never store upstream URL, credentials, proxy data, prompts, responses, or raw errors.';

COMMENT ON COLUMN shared_pool_usage_traces.account_alias IS
    'Deterministic opaque route label safe for pool members, for example Shared route A1B2C3.';
