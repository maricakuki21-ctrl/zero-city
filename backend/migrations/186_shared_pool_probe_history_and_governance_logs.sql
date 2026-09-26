-- Migration: 186_shared_pool_probe_history_and_governance_logs
-- Adds persistent shared-pool probe histories and admin governance audit logs.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS last_probe_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_probe_success BOOLEAN NULL,
    ADD COLUMN IF NOT EXISTS last_probe_error_type VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_probe_error_message TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS consecutive_probe_failures INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_successful_probe_at TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS shared_pool_probe_histories (
    id                  BIGSERIAL PRIMARY KEY,
    pool_id             BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    owner_id            BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    model_name          VARCHAR(160) NOT NULL DEFAULT '',
    upstream_model_name VARCHAR(160) NOT NULL DEFAULT '',
    probe_type          VARCHAR(32) NOT NULL DEFAULT 'manual',
    success             BOOLEAN NOT NULL DEFAULT FALSE,
    http_status         INT NOT NULL DEFAULT 0,
    error_type          VARCHAR(64) NOT NULL DEFAULT '',
    error_message       TEXT NOT NULL DEFAULT '',
    latency_ms          INT NOT NULL DEFAULT 0,
    checked_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT shared_pool_probe_histories_probe_type_check CHECK (probe_type IN ('manual', 'publish_gate', 'scheduled'))
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_histories_pool_checked
    ON shared_pool_probe_histories(pool_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_histories_pool_success
    ON shared_pool_probe_histories(pool_id, success, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_histories_error_type
    ON shared_pool_probe_histories(error_type, checked_at DESC)
    WHERE success = FALSE;

CREATE TABLE IF NOT EXISTS shared_pool_governance_logs (
    id            BIGSERIAL PRIMARY KEY,
    pool_id       BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    admin_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    action        VARCHAR(64) NOT NULL DEFAULT 'update_governance',
    before_value  JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_value   JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason        TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_governance_logs_pool_created
    ON shared_pool_governance_logs(pool_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pool_governance_logs_admin_created
    ON shared_pool_governance_logs(admin_user_id, created_at DESC);

CREATE OR REPLACE FUNCTION recalculate_shared_pool_market_score()
RETURNS TRIGGER AS $$
BEGIN
    NEW.platform_fee_percent = LEAST(100, GREATEST(0, COALESCE(NEW.platform_fee_percent, 10)));
    NEW.owner_share_percent = LEAST(100, GREATEST(0, 100 - NEW.platform_fee_percent));

    NEW.market_score = ROUND(LEAST(1000, GREATEST(-1000,
        COALESCE(NEW.featured_score, 0) * 1.5 +
        COALESCE(NEW.rank_weight, 0) +
        COALESCE(NEW.quality_score, 0) +
        COALESCE(NEW.today_availability, 0) * 0.60 +
        COALESCE(NEW.seven_day_availability, 0) * 0.30 +
        CASE COALESCE(NEW.status, 'healthy')
            WHEN 'healthy' THEN 20
            WHEN 'limited' THEN -20
            WHEN 'maintenance' THEN -80
            ELSE -150
        END +
        CASE COALESCE(NEW.governance_status, 'normal')
            WHEN 'boosted' THEN 80
            WHEN 'watch' THEN -30
            WHEN 'suppressed' THEN -120
            WHEN 'banned' THEN -500
            ELSE 0
        END +
        CASE COALESCE(NEW.last_probe_success, TRUE)
            WHEN TRUE THEN 10
            ELSE -40
        END -
        LEAST(COALESCE(NEW.consecutive_probe_failures, 0) * 12, 160) -
        LEAST(COALESCE(NEW.avg_latency_ms, 0)::NUMERIC / 20, 80) -
        GREATEST(COALESCE(NEW.rate_multiplier, 1) - 1, 0) * 20 -
        COALESCE(NEW.complaint_count, 0) * 8 -
        GREATEST(0, 100 - COALESCE(NEW.today_availability, 0)) * 0.8 -
        COALESCE(NEW.failed_calls, 0)::NUMERIC * 0.02 +
        COALESCE(NEW.reward_score, 0) -
        COALESCE(NEW.penalty_score, 0)
    )), 2);

    IF NEW.governance_status = 'banned' THEN
        NEW.listed = FALSE;
        IF NEW.disabled_reason = '' THEN
            NEW.disabled_reason = 'Banned by marketplace governance';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_shared_pools_market_score ON shared_pools;
CREATE TRIGGER trg_shared_pools_market_score
BEFORE INSERT OR UPDATE OF featured_score, rank_weight, quality_score, today_availability, seven_day_availability, avg_latency_ms, rate_multiplier, complaint_count, failed_calls, reward_score, penalty_score, governance_status, platform_fee_percent, status, listed, disabled_reason, last_probe_success, consecutive_probe_failures
ON shared_pools
FOR EACH ROW
EXECUTE FUNCTION recalculate_shared_pool_market_score();
