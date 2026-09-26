CREATE TABLE IF NOT EXISTS bizdecipher_ledger_accounts (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    asset TEXT NOT NULL CHECK (asset IN ('balance', 'credits')),
    purpose TEXT NOT NULL CHECK (purpose IN ('user_balance', 'owner_earnings', 'platform_fee', 'processor_fee', 'rounding_residue', 'clearing')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, asset, purpose)
);

CREATE TABLE IF NOT EXISTS bizdecipher_ledger_journals (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    idempotency_key TEXT NOT NULL UNIQUE,
    payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    journal_type TEXT NOT NULL,
    reversal_of TEXT REFERENCES bizdecipher_ledger_journals(id),
    posted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (reversal_of IS NULL OR reversal_of <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bizdecipher_ledger_one_reversal
    ON bizdecipher_ledger_journals(reversal_of) WHERE reversal_of IS NOT NULL;

CREATE TABLE IF NOT EXISTS bizdecipher_ledger_entries (
    journal_id TEXT NOT NULL REFERENCES bizdecipher_ledger_journals(id),
    line_no INTEGER NOT NULL CHECK (line_no > 0),
    account_id TEXT NOT NULL REFERENCES bizdecipher_ledger_accounts(id),
    amount NUMERIC(24,12) NOT NULL CHECK (amount <> 0),
    PRIMARY KEY (journal_id, line_no)
);

CREATE TABLE IF NOT EXISTS bizdecipher_ledger_outbox_receipts (
    event_id TEXT PRIMARY KEY,
    payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    journal_id TEXT NOT NULL UNIQUE
        REFERENCES bizdecipher_ledger_journals(id) DEFERRABLE INITIALLY DEFERRED,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NOT NULL,
    CHECK (processed_at >= received_at)
);

COMMENT ON TABLE bizdecipher_ledger_outbox_receipts IS
    'Durable consumer inbox receipts for CanonicalUsageFinalized outbox events';

CREATE OR REPLACE FUNCTION bizdecipher_reject_ledger_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'BizDecipher ledger % rows are immutable; post a reversal instead', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bizdecipher_immutable_journals ON bizdecipher_ledger_journals;
CREATE TRIGGER trg_bizdecipher_immutable_journals
    BEFORE UPDATE OR DELETE ON bizdecipher_ledger_journals
    FOR EACH ROW EXECUTE FUNCTION bizdecipher_reject_ledger_mutation();

DROP TRIGGER IF EXISTS trg_bizdecipher_immutable_entries ON bizdecipher_ledger_entries;
CREATE TRIGGER trg_bizdecipher_immutable_entries
    BEFORE UPDATE OR DELETE ON bizdecipher_ledger_entries
    FOR EACH ROW EXECUTE FUNCTION bizdecipher_reject_ledger_mutation();

DROP TRIGGER IF EXISTS trg_bizdecipher_immutable_outbox_receipts ON bizdecipher_ledger_outbox_receipts;
CREATE TRIGGER trg_bizdecipher_immutable_outbox_receipts
    BEFORE UPDATE OR DELETE ON bizdecipher_ledger_outbox_receipts
    FOR EACH ROW EXECUTE FUNCTION bizdecipher_reject_ledger_mutation();

CREATE OR REPLACE FUNCTION bizdecipher_assert_balanced_journal() RETURNS TRIGGER AS $$
DECLARE
    target_journal TEXT := COALESCE(NEW.journal_id, OLD.journal_id);
BEGIN
    IF (SELECT COALESCE(SUM(amount), 0) FROM bizdecipher_ledger_entries WHERE journal_id = target_journal) <> 0 THEN
        RAISE EXCEPTION 'unbalanced BizDecipher journal %', target_journal;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION bizdecipher_assert_complete_journal() RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT COUNT(*) FROM bizdecipher_ledger_entries WHERE journal_id = NEW.id) < 2 OR
       (SELECT COALESCE(SUM(amount), 0) FROM bizdecipher_ledger_entries WHERE journal_id = NEW.id) <> 0 THEN
        RAISE EXCEPTION 'incomplete or unbalanced BizDecipher journal %', NEW.id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bizdecipher_complete_journal ON bizdecipher_ledger_journals;
CREATE CONSTRAINT TRIGGER trg_bizdecipher_complete_journal
    AFTER INSERT ON bizdecipher_ledger_journals
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bizdecipher_assert_complete_journal();

DROP TRIGGER IF EXISTS trg_bizdecipher_balanced_journal ON bizdecipher_ledger_entries;
CREATE CONSTRAINT TRIGGER trg_bizdecipher_balanced_journal
    AFTER INSERT OR UPDATE OR DELETE ON bizdecipher_ledger_entries
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bizdecipher_assert_balanced_journal();

CREATE TABLE IF NOT EXISTS bizdecipher_ledger_projections (
    account_id TEXT PRIMARY KEY REFERENCES bizdecipher_ledger_accounts(id),
    balance NUMERIC(24,12) NOT NULL DEFAULT 0,
    reserved_amount NUMERIC(24,12) NOT NULL DEFAULT 0 CHECK (reserved_amount >= 0),
    version BIGINT NOT NULL DEFAULT 0 CHECK (version >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bizdecipher_ledger_holds (
    reservation_id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    account_id TEXT NOT NULL REFERENCES bizdecipher_ledger_accounts(id),
    amount NUMERIC(24,12) NOT NULL CHECK (amount > 0),
    state TEXT NOT NULL CHECK (state IN ('reserved', 'captured', 'released', 'unknown')),
    authority_epoch BIGINT NOT NULL CHECK (authority_epoch > 0),
    version BIGINT NOT NULL CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bizdecipher_ledger_cutover_pending (
    cutover_attempt_id TEXT NOT NULL,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('reservation', 'business_event', 'media_task')),
    source_id TEXT NOT NULL,
    payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    amount NUMERIC(24,12) NOT NULL,
    authority_epoch BIGINT NOT NULL CHECK (authority_epoch > 0),
    state TEXT NOT NULL CHECK (state IN ('cutover_pending', 'aborted_before_activation', 'canonical_open')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    staged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cutover_attempt_id, source_kind, source_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bizdecipher_ledger_authoritative_cutover_claim
    ON bizdecipher_ledger_cutover_pending(source_kind, source_id)
    WHERE state = 'canonical_open';

CREATE OR REPLACE FUNCTION bizdecipher_rebuild_ledger_projections() RETURNS VOID AS $$
BEGIN
    DELETE FROM bizdecipher_ledger_projections;
    INSERT INTO bizdecipher_ledger_projections (account_id, balance, reserved_amount, version, updated_at)
    SELECT
        a.id,
        COALESCE(e.balance, 0)::NUMERIC(24,12),
        COALESCE(h.reserved_amount, 0)::NUMERIC(24,12),
        COALESCE(e.entry_count, 0) + COALESCE(h.hold_count, 0),
        NOW()
    FROM bizdecipher_ledger_accounts a
    LEFT JOIN (
        SELECT account_id, SUM(amount) AS balance, COUNT(*) AS entry_count
        FROM bizdecipher_ledger_entries
        GROUP BY account_id
    ) e ON e.account_id = a.id
    LEFT JOIN (
        SELECT account_id, SUM(amount) AS reserved_amount, COUNT(*) AS hold_count
        FROM bizdecipher_ledger_holds
        WHERE state = 'reserved'
        GROUP BY account_id
    ) h ON h.account_id = a.id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE VIEW bizdecipher_owner_earnings AS
SELECT a.owner_id, a.asset, p.account_id, p.balance, p.reserved_amount, p.version, p.updated_at
FROM bizdecipher_ledger_projections p
JOIN bizdecipher_ledger_accounts a ON a.id = p.account_id
WHERE a.purpose = 'owner_earnings';
