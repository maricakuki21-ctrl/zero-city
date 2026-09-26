CREATE TABLE IF NOT EXISTS bizdecipher_accepted_price_quotes (
    quote_id TEXT PRIMARY KEY,
    price_version_id TEXT NOT NULL,
    model TEXT NOT NULL,
    protocol TEXT NOT NULL CHECK (protocol IN ('chat', 'responses', 'websocket', 'image', 'video')),
    asset TEXT NOT NULL CHECK (asset IN ('balance', 'credits')),
    canonical_snapshot JSONB NOT NULL,
    snapshot_sha256 CHAR(64) NOT NULL CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    maximum_authorized_cost NUMERIC(24,12) NOT NULL CHECK (maximum_authorized_cost >= 0),
    accepted_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > accepted_at),
    source_epoch BIGINT NOT NULL CHECK (source_epoch > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (snapshot_sha256)
);

CREATE TABLE IF NOT EXISTS bizdecipher_decimal_holds (
    reservation_id TEXT PRIMARY KEY,
    business_event_id TEXT NOT NULL UNIQUE,
    request_id TEXT NOT NULL,
    quote_sha256 CHAR(64) NOT NULL REFERENCES bizdecipher_accepted_price_quotes(snapshot_sha256),
    amount NUMERIC(24,12) NOT NULL CHECK (amount >= 0),
    asset TEXT NOT NULL CHECK (asset IN ('balance', 'credits')),
    original_expires_at TIMESTAMPTZ NOT NULL,
    source_epoch BIGINT NOT NULL CHECK (source_epoch > 0),
    accepted_media_task_id TEXT NULL,
    state TEXT NOT NULL CHECK (state IN ('reserved', 'dispatching', 'settlement_pending', 'unknown', 'captured', 'released')),
    lease_owner TEXT NOT NULL,
    lease_epoch BIGINT NOT NULL CHECK (lease_epoch > 0),
    version BIGINT NOT NULL CHECK (version > 0),
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (request_id, quote_sha256)
);

CREATE INDEX IF NOT EXISTS idx_bizdecipher_decimal_holds_recovery
    ON bizdecipher_decimal_holds(state, original_expires_at, reservation_id)
    WHERE state IN ('dispatching', 'settlement_pending', 'unknown');

CREATE OR REPLACE FUNCTION reject_bizdecipher_accepted_quote_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'accepted price quotes are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bizdecipher_accepted_quote_immutable ON bizdecipher_accepted_price_quotes;
CREATE TRIGGER trg_bizdecipher_accepted_quote_immutable
    BEFORE UPDATE OR DELETE ON bizdecipher_accepted_price_quotes
    FOR EACH ROW EXECUTE FUNCTION reject_bizdecipher_accepted_quote_mutation();

CREATE OR REPLACE FUNCTION protect_bizdecipher_hold_immutable_fields()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.reservation_id IS DISTINCT FROM OLD.reservation_id
       OR NEW.business_event_id IS DISTINCT FROM OLD.business_event_id
       OR NEW.request_id IS DISTINCT FROM OLD.request_id
       OR NEW.quote_sha256 IS DISTINCT FROM OLD.quote_sha256
       OR NEW.amount IS DISTINCT FROM OLD.amount
       OR NEW.asset IS DISTINCT FROM OLD.asset
       OR NEW.original_expires_at IS DISTINCT FROM OLD.original_expires_at
       OR NEW.source_epoch IS DISTINCT FROM OLD.source_epoch
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'decimal hold immutable fields cannot change';
    END IF;
    IF OLD.accepted_media_task_id IS NOT NULL
       AND NEW.accepted_media_task_id IS DISTINCT FROM OLD.accepted_media_task_id THEN
        RAISE EXCEPTION 'accepted media task id is write-once';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bizdecipher_decimal_hold_immutable ON bizdecipher_decimal_holds;
CREATE TRIGGER trg_bizdecipher_decimal_hold_immutable
    BEFORE UPDATE ON bizdecipher_decimal_holds
    FOR EACH ROW EXECUTE FUNCTION protect_bizdecipher_hold_immutable_fields();
