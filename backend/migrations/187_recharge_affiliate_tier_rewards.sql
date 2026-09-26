-- Recharge affiliate rewards are split into three idempotent components:
-- 1) tier-1 withdrawable quota rebate (5% of recharge face amount)
-- 2) tier-1 non-withdrawable credit reward (20% of recharge face amount)
-- 3) tier-2 withdrawable quota rebate (5% of the tier-1 rebate = 0.25% of face amount)

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
              AND pg_get_constraintdef(oid) LIKE '%recharge_credit_reward%'
        ) THEN
            ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_source_type_check;
            ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
                CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share', 'checkin_milestone', 'shared_pool_stability_reward', 'recharge_credit_reward'));
        END IF;
    ELSE
        ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_source_type_check
            CHECK (source_type IN ('starter', 'purchase', 'contribution', 'operator_reward', 'admin_adjustment', 'invite_reward', 'promo_bonus', 'pool_seat_fee', 'pool_owner_payout', 'share_pool_usage', 'share_pool_payout', 'daily_checkin', 'checkin_jackpot_share', 'checkin_milestone', 'shared_pool_stability_reward', 'recharge_credit_reward'));
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS user_affiliate_recharge_tier1_order_once_unique
    ON user_affiliate_ledger(source_order_id, user_id, source_user_id)
    WHERE action = 'accrue'
      AND source_type = 'recharge_tier1'
      AND source_order_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS user_affiliate_recharge_tier2_order_once_unique
    ON user_affiliate_ledger(source_order_id, user_id, source_user_id)
    WHERE action = 'accrue'
      AND source_type = 'recharge_tier2'
      AND source_order_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS credit_ledger_recharge_credit_reward_once_unique
    ON credit_ledger(user_id, source_type, source_id)
    WHERE source_type = 'recharge_credit_reward';

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_recharge_source
    ON user_affiliate_ledger(source_type, source_order_id, user_id, source_user_id)
    WHERE action = 'accrue'
      AND source_type IN ('recharge_tier1', 'recharge_tier2')
      AND source_order_id IS NOT NULL;
