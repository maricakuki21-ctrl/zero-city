-- 223: Durable pre-authorization for shared-pool requests.
--
-- A request must reserve the maximum accepted charge before upstream bytes are
-- sent. Final settlement captures at most that hold and refunds the remainder.
-- Rows are retained for audit and crash recovery; they are never deleted by the
-- runtime recovery worker.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS shared_pool_usage_reservations (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(160) NOT NULL,
    request_fingerprint CHAR(64) NOT NULL,
    access_key_id BIGINT NOT NULL REFERENCES shared_pool_access_keys(id) ON DELETE RESTRICT,
    pool_id BIGINT NOT NULL REFERENCES shared_pools(id) ON DELETE RESTRICT,
    account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    price_version_id BIGINT NULL REFERENCES shared_pool_price_versions(id) ON DELETE RESTRICT,
    endpoint_type VARCHAR(24) NOT NULL,
    model_snapshot VARCHAR(200) NOT NULL DEFAULT '',
    pricing_source_snapshot VARCHAR(24) NOT NULL DEFAULT '',
    price_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    usage_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    hold_amount NUMERIC(24, 12) NOT NULL,
    reported_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    settled_amount NUMERIC(24, 12) NOT NULL DEFAULT 0,
    status VARCHAR(24) NOT NULL DEFAULT 'reserved',
    failure_reason VARCHAR(160) NOT NULL DEFAULT '',
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    forward_started_at TIMESTAMPTZ NULL,
    settlement_staged_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    finalized_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_usage_reservation_amounts_check CHECK (
        hold_amount >= 0
        AND reported_amount >= 0
        AND settled_amount >= 0
        AND settled_amount <= hold_amount + 0.000000000001
    ),
    CONSTRAINT shared_pool_usage_reservation_status_check CHECK (
        status IN ('reserved', 'forwarding', 'settlement_pending', 'settled', 'released', 'review_required')
    ),
    CONSTRAINT shared_pool_usage_reservation_endpoint_check CHECK (
        endpoint_type IN ('chat', 'responses')
    ),
    CONSTRAINT shared_pool_usage_reservation_request_unique UNIQUE (access_key_id, request_id)
);

ALTER TABLE shared_pool_usage_reservations
    DROP CONSTRAINT IF EXISTS shared_pool_usage_reservations_price_version_id_fkey,
    ADD CONSTRAINT shared_pool_usage_reservations_price_version_id_fkey
        FOREIGN KEY (price_version_id)
        REFERENCES shared_pool_price_versions(id)
        ON DELETE RESTRICT
        NOT VALID;

ALTER TABLE shared_pool_usage_reservations
    VALIDATE CONSTRAINT shared_pool_usage_reservations_price_version_id_fkey;

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_reservations_recovery
    ON shared_pool_usage_reservations(expires_at, id)
    WHERE status IN ('reserved', 'forwarding');

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_reservations_settlement_pending
    ON shared_pool_usage_reservations(settlement_staged_at, id)
    WHERE status = 'settlement_pending';

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_reservations_user
    ON shared_pool_usage_reservations(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_shared_pool_usage_reservations_pool
    ON shared_pool_usage_reservations(pool_id, created_at DESC, id DESC);

COMMENT ON TABLE shared_pool_usage_reservations IS
    'Durable shared-pool balance holds and finalization state; retained for audit and recovery.';
