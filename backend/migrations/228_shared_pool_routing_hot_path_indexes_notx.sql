-- 228: Online indexes for the shared-pool request-routing hot path.
--
-- This migration is intentionally non-transactional because PostgreSQL does
-- not allow CREATE INDEX CONCURRENTLY inside a transaction. It is additive,
-- does not rewrite user/ledger data, and may be retried safely.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_models_aliases_open_gin
    ON shared_pool_models USING GIN (model_aliases)
    WHERE enabled = TRUE AND model_open = TRUE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_accounts_schedulable_route
    ON shared_pool_accounts (
        pool_id,
        priority,
        full_check_score DESC,
        account_weight DESC,
        total_calls,
        last_used_at,
        id
    )
    WHERE deleted_at IS NULL
      AND schedulable = TRUE
      AND status IN ('active', 'limited', 'testing');
