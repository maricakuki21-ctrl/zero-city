-- Migration: 214_shared_pool_seat_idle_release
-- Shared-pool seats auto-release after 2 hours without real API activity.
-- last_activity_at is the authoritative idle clock:
--   * join / re-activate: set to NOW()
--   * successful/failed shared-pool usage recording: touch to NOW()
--   * background sweep releases active seats older than 2 hours

ALTER TABLE pool_seat_bindings
    ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ NULL;

-- Backfill existing seats: prefer access-key last_used_at, else joined_at.
UPDATE pool_seat_bindings sb
SET last_activity_at = COALESCE(
    (
        SELECT MAX(sak.last_used_at)
        FROM shared_pool_access_keys sak
        WHERE sak.pool_id = sb.pool_id
          AND sak.user_id = sb.user_id
          AND sak.last_used_at IS NOT NULL
    ),
    sb.joined_at,
    sb.created_at,
    NOW()
)
WHERE last_activity_at IS NULL;

ALTER TABLE pool_seat_bindings
    ALTER COLUMN last_activity_at SET DEFAULT NOW();

UPDATE pool_seat_bindings
SET last_activity_at = COALESCE(joined_at, created_at, NOW())
WHERE last_activity_at IS NULL;

ALTER TABLE pool_seat_bindings
    ALTER COLUMN last_activity_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pool_seat_bindings_idle
    ON pool_seat_bindings(status, last_activity_at)
    WHERE status = 'active';

COMMENT ON COLUMN pool_seat_bindings.last_activity_at IS
    'Last real shared-pool API activity (or join time). Used for 2-hour idle auto-release.';
