-- Migration: 184_shared_pool_market_governance
-- Adds admin-controlled marketplace governance fields for shared pools.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS platform_fee_percent NUMERIC(5, 2) NOT NULL DEFAULT 10,
    ADD COLUMN IF NOT EXISTS reward_score NUMERIC(8, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS penalty_score NUMERIC(8, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS market_score NUMERIC(10, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS governance_status VARCHAR(24) NOT NULL DEFAULT 'normal',
    ADD COLUMN IF NOT EXISTS governance_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS admin_note TEXT NOT NULL DEFAULT '';

ALTER TABLE shared_pools
    DROP CONSTRAINT IF EXISTS shared_pools_platform_fee_percent_check,
    ADD CONSTRAINT shared_pools_platform_fee_percent_check CHECK (platform_fee_percent >= 0 AND platform_fee_percent <= 100);

ALTER TABLE shared_pools
    DROP CONSTRAINT IF EXISTS shared_pools_governance_status_check,
    ADD CONSTRAINT shared_pools_governance_status_check CHECK (governance_status IN ('normal', 'boosted', 'watch', 'suppressed', 'banned'));

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
        END -
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
BEFORE INSERT OR UPDATE OF featured_score, rank_weight, quality_score, today_availability, seven_day_availability, avg_latency_ms, rate_multiplier, complaint_count, failed_calls, reward_score, penalty_score, governance_status, platform_fee_percent, status, listed, disabled_reason
ON shared_pools
FOR EACH ROW
EXECUTE FUNCTION recalculate_shared_pool_market_score();

UPDATE shared_pools
SET platform_fee_percent = LEAST(100, GREATEST(0, 100 - owner_share_percent)),
    updated_at = updated_at;

CREATE INDEX IF NOT EXISTS idx_shared_pools_market_score
    ON shared_pools(listed, governance_status, market_score DESC, featured_score DESC, rank_weight DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pools_governance_status
    ON shared_pools(governance_status, listed);
