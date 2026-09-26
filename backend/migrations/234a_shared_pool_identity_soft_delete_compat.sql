-- Migration 235 expects the legacy pool soft-delete projection.
-- Preserve all rows and any existing soft-delete timestamps.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Archived pools must not be reintroduced as active canonical supply.
UPDATE shared_pools
SET deleted_at = archived_at
WHERE lifecycle_state = 'archived'
  AND archived_at IS NOT NULL
  AND deleted_at IS NULL;
