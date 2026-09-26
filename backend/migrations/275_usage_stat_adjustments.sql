CREATE TABLE IF NOT EXISTS usage_stat_adjustments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    actual_cost_delta NUMERIC(20, 10) NOT NULL CHECK (actual_cost_delta <> 0),
    previous_actual_cost NUMERIC(20, 10),
    target_actual_cost NUMERIC(20, 10),
    reason TEXT NOT NULL CHECK (BTRIM(reason) <> ''),
    actor TEXT NOT NULL CHECK (BTRIM(actor) <> ''),
    idempotency_key TEXT NOT NULL UNIQUE CHECK (BTRIM(idempotency_key) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_stat_adjustments_user
    ON usage_stat_adjustments(user_id);
