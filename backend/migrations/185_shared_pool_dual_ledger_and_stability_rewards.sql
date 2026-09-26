-- Migration: 185_shared_pool_dual_ledger_and_stability_rewards
-- Shared pool accounting hardening:
--   * withdrawable owner earnings are balance movements (users.balance)
--   * stability listing rewards are non-withdrawable credits (users.credit_balance)
--   * each hourly stability reward is idempotent per (pool, owner, reward_hour)
-- Safety net for production drift: these columns are normally created by
-- migrations 180 and 184. Keep them here as IF NOT EXISTS guards so this
-- accounting migration does not fail on a server that missed those files.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS upstream_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_api_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quality_score DECIMAL(6, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rank_weight DECIMAL(8, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS disabled_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS featured_score DECIMAL(8, 4) NOT NULL DEFAULT 0,
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

CREATE INDEX IF NOT EXISTS idx_shared_pools_featured_score
    ON shared_pools(featured_score DESC, rank_weight DESC, quality_score DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pools_governance_status
    ON shared_pools(governance_status, listed);

ALTER TABLE shared_pool_models
    ADD COLUMN IF NOT EXISTS provider VARCHAR(64) NOT NULL DEFAULT 'openai',
    ADD COLUMN IF NOT EXISTS model_aliases JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS rate_multiplier NUMERIC(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rank_weight NUMERIC(12, 4) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS five_hour_protection_percent NUMERIC(5, 2) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS seven_day_protection_percent NUMERIC(5, 2) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS min_balance_admission NUMERIC(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS hourly_seat_fee NUMERIC(12, 4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_concurrency INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS daily_protection_percent NUMERIC(5, 2) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS model_open BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS upstream_model_name VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_shared_pool_models_model_enabled
    ON shared_pool_models(model_name, enabled);
CREATE INDEX IF NOT EXISTS idx_shared_pool_models_provider_enabled
    ON shared_pool_models(provider, enabled);
CREATE INDEX IF NOT EXISTS idx_shared_pool_models_open_model
    ON shared_pool_models(model_open, model_name);
CREATE INDEX IF NOT EXISTS idx_shared_pool_models_upstream_model_name
    ON shared_pool_models(upstream_model_name)
    WHERE upstream_model_name <> '';

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

CREATE TABLE IF NOT EXISTS shared_pool_balance_ledger (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pool_id       BIGINT NULL REFERENCES shared_pools(id) ON DELETE SET NULL,
    source_type   VARCHAR(50) NOT NULL,
    source_id     VARCHAR(160) NOT NULL DEFAULT '',
    amount        DECIMAL(20, 8) NOT NULL,
    balance_after DECIMAL(20, 8) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'posted',
    note          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    posted_at     TIMESTAMPTZ NULL,
    CONSTRAINT shared_pool_balance_ledger_source_type_check CHECK (source_type IN ('pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout')),
    CONSTRAINT shared_pool_balance_ledger_status_check CHECK (status IN ('pending', 'posted', 'reversed'))
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_user_created
    ON shared_pool_balance_ledger(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_pool_created
    ON shared_pool_balance_ledger(pool_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_source
    ON shared_pool_balance_ledger(source_type, source_id);
CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_balance_ledger_source_unique
    ON shared_pool_balance_ledger(user_id, source_type, source_id)
    WHERE source_id <> '';

CREATE TABLE IF NOT EXISTS shared_pool_stability_rewards (
    id               BIGSERIAL PRIMARY KEY,
    pool_id          BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    owner_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_hour      TIMESTAMPTZ NOT NULL,
    credit_amount    DECIMAL(20, 8) NOT NULL DEFAULT 30,
    credit_ledger_id BIGINT NULL REFERENCES credit_ledger(id) ON DELETE SET NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'posted',
    note             TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_stability_rewards_amount_check CHECK (credit_amount > 0),
    CONSTRAINT shared_pool_stability_rewards_status_check CHECK (status IN ('posted', 'skipped', 'reversed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_stability_rewards_pool_hour_unique
    ON shared_pool_stability_rewards(pool_id, owner_id, reward_hour);
CREATE INDEX IF NOT EXISTS idx_shared_pool_stability_rewards_owner_created
    ON shared_pool_stability_rewards(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shared_pool_stability_rewards_pool_hour
    ON shared_pool_stability_rewards(pool_id, reward_hour DESC);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'credit_ledger_source_type_check'
          AND conrelid = 'credit_ledger'::regclass
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'credit_ledger_source_type_check'
              AND conrelid = 'credit_ledger'::regclass
              AND pg_get_constraintdef(oid) LIKE '%shared_pool_stability_reward%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share', 'checkin_milestone', 'shared_pool_stability_reward'));
        END IF;
    ELSE
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share', 'checkin_milestone', 'shared_pool_stability_reward'));
    END IF;
END
$$;
