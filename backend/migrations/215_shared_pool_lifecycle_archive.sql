-- 215: Give shared pools a durable lifecycle and replace destructive deletion
-- with auditable archival.
--
-- Safety properties:
--   * no existing pool is auto-archived;
--   * historical rows and all financial/probe/member relations stay intact;
--   * the archive event table uses ON DELETE RESTRICT as a final guard against
--     accidental hard deletion after a pool has lifecycle history.

ALTER TABLE shared_pools
    ADD COLUMN IF NOT EXISTS lifecycle_state VARCHAR(24) NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS archived_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS archive_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_kind VARCHAR(24) NOT NULL DEFAULT 'user';

-- Existing rows were created before lifecycle_state existed. Preserve every row
-- and classify it only as currently operating or suspended; never guess that an
-- offline/unlisted row is safe to archive.
UPDATE shared_pools
SET lifecycle_state = CASE
        WHEN listed = TRUE AND status IN ('healthy', 'limited') THEN 'operating'
        ELSE 'suspended'
    END
WHERE lifecycle_state = 'draft'
  AND archived_at IS NULL;

ALTER TABLE shared_pools
    DROP CONSTRAINT IF EXISTS shared_pools_lifecycle_state_check,
    ADD CONSTRAINT shared_pools_lifecycle_state_check
        CHECK (lifecycle_state IN ('draft', 'operating', 'suspended', 'archived')),
    DROP CONSTRAINT IF EXISTS shared_pools_source_kind_check,
    ADD CONSTRAINT shared_pools_source_kind_check
        CHECK (source_kind IN ('user', 'legacy_seed', 'imported')),
    DROP CONSTRAINT IF EXISTS shared_pools_archived_visibility_check,
    ADD CONSTRAINT shared_pools_archived_visibility_check
        CHECK (
            lifecycle_state <> 'archived'
            OR (listed = FALSE AND archived_at IS NOT NULL)
        );

CREATE OR REPLACE FUNCTION sync_shared_pool_lifecycle()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.lifecycle_state = 'archived' THEN
        NEW.listed = FALSE;
        NEW.archived_at = COALESCE(NEW.archived_at, NOW());
        RETURN NEW;
    END IF;

    NEW.archived_at = NULL;
    NEW.archived_by = NULL;
    IF NEW.listed = TRUE AND NEW.status IN ('healthy', 'limited') THEN
        NEW.lifecycle_state = 'operating';
    ELSIF NEW.lifecycle_state <> 'draft' THEN
        NEW.lifecycle_state = 'suspended';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_shared_pool_lifecycle ON shared_pools;
CREATE TRIGGER trg_sync_shared_pool_lifecycle
BEFORE INSERT OR UPDATE OF lifecycle_state, listed, status, archived_at, archived_by
ON shared_pools
FOR EACH ROW EXECUTE FUNCTION sync_shared_pool_lifecycle();

CREATE INDEX IF NOT EXISTS idx_shared_pools_lifecycle
    ON shared_pools(lifecycle_state, listed, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pools_archived
    ON shared_pools(archived_at DESC, id DESC)
    WHERE lifecycle_state = 'archived';

CREATE TABLE IF NOT EXISTS shared_pool_archive_events (
    id BIGSERIAL PRIMARY KEY,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    actor_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(20) NOT NULL,
    before_state VARCHAR(24) NOT NULL,
    after_state VARCHAR(24) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    operation_id VARCHAR(160) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_archive_events_action_check
        CHECK (action IN ('archive', 'restore')),
    CONSTRAINT shared_pool_archive_events_state_check
        CHECK (
            before_state IN ('draft', 'operating', 'suspended', 'archived')
            AND after_state IN ('draft', 'operating', 'suspended', 'archived')
        )
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_archive_events_pool
    ON shared_pool_archive_events(pool_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS shared_pool_archive_events_operation_unique
    ON shared_pool_archive_events(pool_id, action, operation_id)
    WHERE operation_id <> '';

COMMENT ON COLUMN shared_pools.lifecycle_state IS
    'Business lifecycle independent from technical health and governance status.';
COMMENT ON COLUMN shared_pools.archive_reason IS
    'Human-readable reason retained permanently when a pool is archived.';
COMMENT ON TABLE shared_pool_archive_events IS
    'Append-only archive/restore audit trail. Its restrictive FK prevents hard deletion.';
