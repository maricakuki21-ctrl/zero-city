-- Accelerate the model-bound compact capability lookup used by both account
-- scheduling and the final pre-reservation revalidation. Expression columns
-- match the query's case/whitespace normalization exactly.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_probe_jobs_compact_capability
    ON shared_pool_probe_jobs (
        pool_id,
        account_id,
        config_version,
        (LOWER(BTRIM(model_name))),
        (LOWER(BTRIM(upstream_model_name))),
        finished_at DESC NULLS LAST,
        created_at DESC,
        id DESC
    )
    WHERE status = 'succeeded';
