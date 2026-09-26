-- Unpriceable or unmapped history must not block later finalized usage.
ALTER TABLE canonical_usage_outbox
    ADD COLUMN IF NOT EXISTS settlement_retry_at TIMESTAMPTZ NOT NULL DEFAULT '-infinity';

CREATE INDEX IF NOT EXISTS idx_canonical_usage_settlement_retry
    ON canonical_usage_outbox(settlement_retry_at, created_at, event_id)
    WHERE status = 'pending';
