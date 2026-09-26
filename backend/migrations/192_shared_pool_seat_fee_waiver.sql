-- Migration: 192_shared_pool_seat_fee_waiver
-- Adds an explicit hourly minimum usage waiver for shared-pool seat fees.
--
-- min_balance_admission is only an admission/safety balance threshold.
-- hourly_min_usage_waiver is the usage-spend threshold inside each billing hour.
-- Positive posted usage reduces the snapshotted hourly fee proportionally;
-- reaching the threshold waives the full fee for that hour.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS hourly_min_usage_waiver DECIMAL(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE pool_seat_bindings
    ADD COLUMN IF NOT EXISTS hourly_min_usage_waiver DECIMAL(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE pool_seat_charges
    ADD COLUMN IF NOT EXISTS usage_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS waived_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS waiver_threshold DECIMAL(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS waiver_applied BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_pool_seat_bindings_waiver
    ON pool_seat_bindings(pool_id, user_id, status, hourly_min_usage_waiver);

CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_pool_user_source_created
    ON shared_pool_balance_ledger(pool_id, user_id, source_type, created_at DESC);
