-- 234: Efficient bounded scan for aged low-value shared-pool review holds.
-- Non-transactional migration: keep production writes available while the
-- partial index is built.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_usage_reservations_review_age
    ON shared_pool_usage_reservations(finalized_at, id)
    WHERE status = 'review_required';
