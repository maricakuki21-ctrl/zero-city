-- 219: Preserve why a shared-pool seat ended and make active/history views
-- explicit. Existing released rows are retained as legacy history.

ALTER TABLE pool_seat_bindings
    ADD COLUMN IF NOT EXISTS release_reason VARCHAR(32) NOT NULL DEFAULT '';

UPDATE pool_seat_bindings
SET released_at = COALESCE(released_at, updated_at, joined_at, NOW()),
    release_reason = CASE
        WHEN release_reason = '' THEN 'legacy_release'
        ELSE release_reason
    END
WHERE status = 'released';

UPDATE pool_seat_bindings
SET released_at = NULL,
    release_reason = ''
WHERE status = 'active'
  AND (released_at IS NOT NULL OR release_reason <> '');

ALTER TABLE pool_seat_bindings
    DROP CONSTRAINT IF EXISTS pool_seat_bindings_release_reason_check,
    ADD CONSTRAINT pool_seat_bindings_release_reason_check CHECK (
        release_reason IN (
            '',
            'user_leave',
            'owner_removed',
            'idle_timeout',
            'insufficient_balance',
            'pool_unavailable',
            'pool_archived',
            'admin_release',
            'legacy_release'
        )
    ),
    DROP CONSTRAINT IF EXISTS pool_seat_bindings_release_state_check,
    ADD CONSTRAINT pool_seat_bindings_release_state_check CHECK (
        (status = 'active' AND released_at IS NULL AND release_reason = '')
        OR (status = 'released' AND released_at IS NOT NULL AND release_reason <> '')
    );

CREATE INDEX IF NOT EXISTS idx_pool_seat_bindings_pool_status_joined
    ON pool_seat_bindings(pool_id, status, joined_at DESC);

-- Repair only the denormalized occupancy cache. Historical seat rows are not
-- removed or changed.
UPDATE shared_pools pool
SET current_users = (
        SELECT COUNT(*)
        FROM pool_seat_bindings seat
        WHERE seat.pool_id = pool.id
          AND seat.status = 'active'
    ),
    updated_at = NOW()
WHERE current_users <> (
    SELECT COUNT(*)
    FROM pool_seat_bindings seat
    WHERE seat.pool_id = pool.id
      AND seat.status = 'active'
);
