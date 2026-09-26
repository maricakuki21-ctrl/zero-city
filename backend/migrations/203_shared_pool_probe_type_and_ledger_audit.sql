-- 203: Fix probe_type CHECK constraint to include scheduled_full,
--      and add platform_fee_amount / owner_payout_amount to shared_pool_balance_ledger
--      for complete audit trail (user debit = owner credit + platform fee).

-- 1. Drop old probe_type constraint and add new one including scheduled_full
ALTER TABLE shared_pool_probe_histories
    DROP CONSTRAINT IF EXISTS shared_pool_probe_histories_probe_type_check;

ALTER TABLE shared_pool_probe_histories
    ADD CONSTRAINT shared_pool_probe_histories_probe_type_check
        CHECK (probe_type IN ('manual', 'publish_gate', 'scheduled', 'scheduled_full'));

-- 2. Add platform_fee_amount and owner_payout_amount to ledger
--    These are populated only for source_type IN ('share_pool_usage') rows so that
--    a single row carries the full split: amount = user debit,
--    owner_payout_amount = what the pool owner earns,
--    platform_fee_amount  = what the platform takes.
--    For source_type = 'share_pool_payout' these remain NULL (the payout row already
--    duplicates owner_payout_amount as its own amount).
ALTER TABLE shared_pool_balance_ledger
    ADD COLUMN IF NOT EXISTS platform_fee_amount  DECIMAL(20, 8) NULL,
    ADD COLUMN IF NOT EXISTS owner_payout_amount  DECIMAL(20, 8) NULL;

-- 3. Add an index on source_id (request_id) for fast per-request audit lookup
CREATE INDEX IF NOT EXISTS idx_shared_pool_balance_ledger_source_id
    ON shared_pool_balance_ledger(source_id)
    WHERE source_id <> '';

-- 4. Add account_id to shared_pool_probe_histories for account-level health tracking
ALTER TABLE shared_pool_probe_histories
    ADD COLUMN IF NOT EXISTS account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_shared_pool_probe_histories_account
    ON shared_pool_probe_histories(account_id, checked_at DESC)
    WHERE account_id IS NOT NULL;
