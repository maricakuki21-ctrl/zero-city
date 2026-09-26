-- 226: Audited operator resolution for ambiguous shared-pool forwarding.
--
-- A forwarding request whose upstream result is unknown keeps the user's hold
-- in review_required. This append-only record makes every manual decision
-- attributable and idempotent. No reservation, ledger, key, pool, or user row
-- is deleted by this migration.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS shared_pool_usage_review_resolutions (
    id BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES shared_pool_usage_reservations(id) ON DELETE RESTRICT,
    admin_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation_id VARCHAR(160) NOT NULL,
    action VARCHAR(24) NOT NULL,
    resolution_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    trigger_reason VARCHAR(160) NOT NULL DEFAULT '',
    note VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_usage_review_resolution_action_check CHECK (
        action IN ('release', 'capture_hold', 'settle_amount')
    ),
    CONSTRAINT shared_pool_usage_review_resolution_amount_check CHECK (
        resolution_amount >= 0
    ),
    CONSTRAINT shared_pool_usage_review_resolution_reservation_unique UNIQUE (reservation_id),
    CONSTRAINT shared_pool_usage_review_resolution_operation_unique UNIQUE (operation_id)
);

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_reservations_review_required
    ON shared_pool_usage_reservations(id DESC)
    WHERE status = 'review_required';

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_review_resolutions_created
    ON shared_pool_usage_review_resolutions(created_at DESC, id DESC);

-- Keep the member-facing trace aligned even when a queued manual settlement is
-- completed later by the crash-recovery worker instead of the original HTTP
-- request. Only reservations with an administrator resolution are affected.
CREATE OR REPLACE FUNCTION sync_shared_pool_usage_review_trace()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM shared_pool_usage_review_resolutions resolution
        WHERE resolution.reservation_id = NEW.id
    ) THEN
        RETURN NEW;
    END IF;

    IF NEW.status = 'settlement_pending' THEN
        UPDATE shared_pool_usage_traces
        SET settlement_outcome = 'pending'
        WHERE access_key_id = NEW.access_key_id
          AND request_id = NEW.request_id;
    ELSIF NEW.status = 'settled' THEN
        UPDATE shared_pool_usage_traces
        SET settlement_outcome = 'settled'
        WHERE access_key_id = NEW.access_key_id
          AND request_id = NEW.request_id;
    ELSIF NEW.status = 'released' THEN
        UPDATE shared_pool_usage_traces
        SET settlement_outcome = 'released'
        WHERE access_key_id = NEW.access_key_id
          AND request_id = NEW.request_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_shared_pool_usage_review_trace
    ON shared_pool_usage_reservations;
CREATE TRIGGER trg_sync_shared_pool_usage_review_trace
    AFTER UPDATE OF status ON shared_pool_usage_reservations
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION sync_shared_pool_usage_review_trace();

COMMENT ON TABLE shared_pool_usage_review_resolutions IS
    'Append-only administrator decisions for ambiguous shared-pool usage holds.';
