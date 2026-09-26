-- 222: Keep the exact accepted shared-pool price beside every billed call.
-- This is append-only accounting metadata; no balance or historical row is rewritten.

ALTER TABLE shared_pool_balance_ledger
    ADD COLUMN IF NOT EXISTS price_version_id BIGINT NULL REFERENCES shared_pool_price_versions(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS pricing_source_snapshot VARCHAR(24) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS price_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE shared_pool_balance_ledger
    DROP CONSTRAINT IF EXISTS shared_pool_balance_ledger_price_version_id_fkey,
    ADD CONSTRAINT shared_pool_balance_ledger_price_version_id_fkey
        FOREIGN KEY (price_version_id)
        REFERENCES shared_pool_price_versions(id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_balance_ledger
    VALIDATE CONSTRAINT shared_pool_balance_ledger_price_version_id_fkey;

COMMENT ON COLUMN shared_pool_balance_ledger.price_snapshot IS
    'Immutable accepted quote (base components, multiplier, bounds, endpoint and effective time).';
