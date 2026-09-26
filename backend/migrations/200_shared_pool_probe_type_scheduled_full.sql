-- Migration: 200_shared_pool_probe_type_scheduled_full
-- Aligns the database CHECK constraint with the scheduler code path that can
-- record full scheduled probe runs.

ALTER TABLE shared_pool_probe_histories
    DROP CONSTRAINT IF EXISTS shared_pool_probe_histories_probe_type_check,
    ADD CONSTRAINT shared_pool_probe_histories_probe_type_check
        CHECK (probe_type IN ('manual', 'publish_gate', 'scheduled', 'scheduled_full'));
