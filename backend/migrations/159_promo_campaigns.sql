-- Migration: 159_promo_campaigns
-- Promo campaign system: admin-configurable time-boxed reward campaigns
-- (e.g. "register bonus: get 30 credits, limited 3 days").
-- Replaces the hardcoded frontend PromoBanner mock with a real backend.

-- 1. Campaign definitions (admin-configurable, toggleable on/off)
CREATE TABLE IF NOT EXISTS promo_campaigns (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(120) NOT NULL,
    type          VARCHAR(50)  NOT NULL DEFAULT 'register_bonus',
    title         VARCHAR(200) NOT NULL DEFAULT '',
    description   TEXT         NOT NULL DEFAULT '',
    enabled       BOOLEAN      NOT NULL DEFAULT FALSE,
    credit_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    start_at      TIMESTAMPTZ  NOT NULL,
    end_at        TIMESTAMPTZ  NOT NULL,
    target        VARCHAR(50)  NOT NULL DEFAULT 'new_users',
    auto_hide     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT promo_campaigns_type_check CHECK (type IN ('register_bonus', 'topup_bonus', 'custom')),
    CONSTRAINT promo_campaigns_target_check CHECK (target IN ('new_users', 'all_users'))
);

CREATE INDEX IF NOT EXISTS idx_promo_campaigns_enabled_window
    ON promo_campaigns(enabled, start_at, end_at);

-- 2. Per-user claim records (idempotency: one claim per user per campaign)
CREATE TABLE IF NOT EXISTS promo_claims (
    id          BIGSERIAL PRIMARY KEY,
    promo_id    BIGINT NOT NULL REFERENCES promo_campaigns(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      DECIMAL(20, 8) NOT NULL,
    ledger_id   BIGINT NULL,
    claimed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS promo_claims_promo_user_unique
    ON promo_claims(promo_id, user_id);
CREATE INDEX IF NOT EXISTS idx_promo_claims_user ON promo_claims(user_id);

-- Seed: the "3-day register bonus" campaign requested by the product owner.
-- Window is relative to migration time (yesterday -> tomorrow) so the banner is
-- live immediately. It is a normal DB row: admins can edit/disable it from the
-- console (/admin/biz/promos). Only seeded when no campaign exists yet, so this
-- never duplicates on re-run or clobbers admin changes.
INSERT INTO promo_campaigns (name, type, title, description, enabled, credit_amount, start_at, end_at, target, auto_hide)
SELECT
    '三天注册赠送活动',
    'register_bonus',
    '新用户注册赠送活动',
    '注册即领 30 额度，限时三天，先到先得。',
    TRUE,
    30,
    NOW() - INTERVAL '1 day',
    NOW() + INTERVAL '1 day',
    'new_users',
    TRUE
WHERE NOT EXISTS (SELECT 1 FROM promo_campaigns);

-- 3. Extend credit_ledger source_type CHECK to include 'promo_bonus'
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
              AND pg_get_constraintdef(oid) LIKE '%promo_bonus%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus'));
        END IF;
    ELSE
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus'));
    END IF;
END
$$;
