-- 253_canonical_usage_settlement_snapshot.sql
-- Freeze the request-time accepted quote beside the canonical usage event.
-- The event payload remains schema-version 1; this sidecar is immutable
-- provenance for the shared-pool settlement consumer.

ALTER TABLE canonical_usage_outbox
    ADD COLUMN IF NOT EXISTS settlement_snapshot JSONB,
    ADD COLUMN IF NOT EXISTS settlement_snapshot_sha256 TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'canonical_usage_outbox_settlement_snapshot_check'
    ) THEN
        ALTER TABLE canonical_usage_outbox
            ADD CONSTRAINT canonical_usage_outbox_settlement_snapshot_check
            CHECK (
                (settlement_snapshot IS NULL AND settlement_snapshot_sha256 IS NULL)
                OR (
                    settlement_snapshot IS NOT NULL
                    AND settlement_snapshot_sha256 IS NOT NULL
                    AND settlement_snapshot_sha256 ~ '^[0-9a-f]{64}$'
                )
            );
    END IF;
END
$$;

COMMENT ON COLUMN canonical_usage_outbox.settlement_snapshot IS
    'Immutable request-time settlement provenance, currently the accepted shared-pool quote.';
COMMENT ON COLUMN canonical_usage_outbox.settlement_snapshot_sha256 IS
    'SHA256 of the canonical JSON settlement snapshot.';
