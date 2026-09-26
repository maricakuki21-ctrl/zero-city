-- 170: Balance / credits dual-track billing
-- Paid recharge remains in users.balance. Free signup/activity/community grants use users.credit_balance.
-- Groups can opt into credits-only billing for welfare/community pools.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS credit_balance DECIMAL(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS billing_asset_type VARCHAR(20) NOT NULL DEFAULT 'balance';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'groups_billing_asset_type_check'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_billing_asset_type_check
            CHECK (billing_asset_type IN ('balance', 'credits'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_groups_billing_asset_type ON groups(billing_asset_type) WHERE deleted_at IS NULL;

COMMENT ON COLUMN users.credit_balance IS '赠送/活动/社群福利积分余额，不等同于真实支付余额';
COMMENT ON COLUMN groups.billing_asset_type IS 'standard billing asset: balance=扣真钱余额, credits=扣赠送积分';
