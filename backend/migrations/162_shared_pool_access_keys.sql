-- Migration: 162_shared_pool_access_keys
-- Shared pool access keys + owner-managed upstream settings.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS upstream_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_api_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_share_percent DECIMAL(5, 2) NOT NULL DEFAULT 90,
    ADD COLUMN IF NOT EXISTS quality_score DECIMAL(6, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rank_weight DECIMAL(8, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS complaint_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_calls BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS successful_calls BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS failed_calls BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS shared_pool_access_keys (
    id              BIGSERIAL PRIMARY KEY,
    pool_id         BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id      BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL DEFAULT '',
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    allowed_models  JSONB NOT NULL DEFAULT '[]'::jsonb,
    total_used      DECIMAL(20, 8) NOT NULL DEFAULT 0,
    last_used_at    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_access_keys_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_access_keys_api_key_unique ON shared_pool_access_keys(api_key_id);
CREATE INDEX IF NOT EXISTS idx_shared_pool_access_keys_user ON shared_pool_access_keys(user_id, status);
CREATE INDEX IF NOT EXISTS idx_shared_pool_access_keys_pool ON shared_pool_access_keys(pool_id, status);

CREATE TABLE IF NOT EXISTS shared_pool_complaints (
    id          BIGSERIAL PRIMARY KEY,
    pool_id     BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason      TEXT NOT NULL DEFAULT '',
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_complaints_status_check CHECK (status IN ('open', 'reviewed', 'dismissed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_complaints_pool_user_unique ON shared_pool_complaints(pool_id, user_id);
CREATE INDEX IF NOT EXISTS idx_shared_pool_complaints_pool ON shared_pool_complaints(pool_id, status);

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
              AND pg_get_constraintdef(oid) LIKE '%share_pool_usage%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout'));
        END IF;
    ELSE
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout'));
    END IF;
END
$$;
