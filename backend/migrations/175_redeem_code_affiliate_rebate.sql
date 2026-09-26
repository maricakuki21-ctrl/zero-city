-- Redeem-code affiliate rebates need their own idempotency source.
-- source_order_id references payment_orders and must not be reused for redeem_code IDs.

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(64) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_id VARCHAR(128) NULL;

COMMENT ON COLUMN user_affiliate_ledger.source_type IS 'Optional idempotency/source namespace for non-payment-order affiliate accruals, e.g. redeem_code';
COMMENT ON COLUMN user_affiliate_ledger.source_id IS 'Optional idempotency/source identifier within source_type, e.g. redeem:<redeem_code_id>';

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source
    ON user_affiliate_ledger(source_type, source_id)
    WHERE source_type IS NOT NULL AND source_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS user_affiliate_redeem_code_once_unique
    ON user_affiliate_ledger(source_type, source_id)
    WHERE action = 'accrue'
      AND source_type = 'redeem_code'
      AND source_id IS NOT NULL;
