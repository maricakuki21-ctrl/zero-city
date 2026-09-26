-- Daily checkin blind box and reward defaults.
-- Points use users.balance; credits/quota use users.credit_balance.

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
              AND pg_get_constraintdef(oid) LIKE '%daily_checkin%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin'));
        END IF;
    ELSE
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin'));
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS daily_checkins (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date         DATE NOT NULL,
    checkin_type         VARCHAR(20) NOT NULL,
    point_cost           NUMERIC(12, 4) NOT NULL DEFAULT 0,
    credit_reward        NUMERIC(12, 4) NOT NULL,
    balance_after        NUMERIC(12, 4) NOT NULL,
    credit_balance_after NUMERIC(12, 4) NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_checkins_type_check CHECK (checkin_type IN ('free', 'paid')),
    CONSTRAINT daily_checkins_reward_check CHECK (credit_reward > 0),
    CONSTRAINT daily_checkins_cost_check CHECK (point_cost >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS daily_checkins_user_date_type_unique
    ON daily_checkins(user_id, checkin_date, checkin_type);

CREATE INDEX IF NOT EXISTS idx_daily_checkins_user_date
    ON daily_checkins(user_id, checkin_date DESC);

-- Registration points: 30 points on signup, plus 1 credit/quota from migration 171.
-- This intentionally uses a separate key from default_balance because default_balance
-- is currently used by AuthService as starter credit/quota fallback.
UPDATE settings
SET value = '30', updated_at = CURRENT_TIMESTAMP
WHERE key = 'signup_point_balance';

INSERT INTO settings (key, value)
SELECT 'signup_point_balance', '30'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'signup_point_balance');

-- Keep email signup credit/quota at 1 and enabled.
UPDATE settings
SET value = '1', updated_at = CURRENT_TIMESTAMP
WHERE key = 'auth_source_default_email_balance';

INSERT INTO settings (key, value)
SELECT 'auth_source_default_email_balance', '1'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'auth_source_default_email_balance');

UPDATE settings
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'auth_source_default_email_grant_on_signup';

INSERT INTO settings (key, value)
SELECT 'auth_source_default_email_grant_on_signup', 'true'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'auth_source_default_email_grant_on_signup');

-- Invite reward: inviter gets 30 points configured here; invitee gets no extra invite reward.
UPDATE settings
SET value = '30', updated_at = CURRENT_TIMESTAMP
WHERE key = 'invite_reward_amount';

INSERT INTO settings (key, value)
SELECT 'invite_reward_amount', '30'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'invite_reward_amount');

UPDATE settings
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'invite_reward_enabled';

INSERT INTO settings (key, value)
SELECT 'invite_reward_enabled', 'true'
WHERE NOT EXISTS (SELECT 1 FROM settings WHERE key = 'invite_reward_enabled');
