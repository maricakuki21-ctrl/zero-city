-- Price-version lookups and FK enforcement must not take long write-blocking
-- locks on the append-only accounting tables.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_owner_earnings_price_version
    ON shared_pool_owner_earnings_ledger(price_version_id)
    WHERE price_version_id IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_balance_ledger_price_version
    ON shared_pool_balance_ledger(price_version_id)
    WHERE price_version_id IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_usage_reservations_price_version
    ON shared_pool_usage_reservations(price_version_id)
    WHERE price_version_id IS NOT NULL;
