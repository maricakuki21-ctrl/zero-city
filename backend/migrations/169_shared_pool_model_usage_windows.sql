-- Shared pool per-model usage windows for protection thresholds.
-- Cost is recorded in billing USD after pool/model rate multipliers.

CREATE TABLE IF NOT EXISTS shared_pool_model_usage_windows (
    pool_id      BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    model_name   VARCHAR(128) NOT NULL,
    window_type  VARCHAR(8) NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    cost         DECIMAL(20, 8) NOT NULL DEFAULT 0,
    calls        BIGINT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pool_id, model_name, window_type, window_start),
    CONSTRAINT shared_pool_model_usage_windows_type_check CHECK (window_type IN ('5h', '1d', '7d'))
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_model_usage_windows_lookup
    ON shared_pool_model_usage_windows(pool_id, model_name, window_type, window_start DESC);
