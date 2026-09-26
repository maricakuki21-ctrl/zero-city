CREATE TABLE IF NOT EXISTS bizdecipher_opening_balance_batches (
    batch_id UUID PRIMARY KEY,
    cutover_attempt_id UUID NOT NULL REFERENCES shared_pool_cutover_attempts(attempt_id) ON DELETE RESTRICT,
    authority_epoch BIGINT NOT NULL CHECK (authority_epoch > 0),
    source_snapshot_sha256 CHAR(64) NOT NULL CHECK (source_snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    restore_sha256 CHAR(64) NOT NULL CHECK (restore_sha256 ~ '^[0-9a-f]{64}$'),
    state TEXT NOT NULL DEFAULT 'staged' CHECK (state IN ('staged', 'imported', 'verified', 'aborted_before_activation', 'canonical_open')),
    staged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    imported_at TIMESTAMPTZ NULL,
    verified_at TIMESTAMPTZ NULL,
    UNIQUE (cutover_attempt_id)
);

CREATE TABLE IF NOT EXISTS bizdecipher_opening_balance_items (
    batch_id UUID NOT NULL REFERENCES bizdecipher_opening_balance_batches(batch_id) ON DELETE RESTRICT,
    source_balance_id TEXT NOT NULL,
    source_owner_id TEXT NOT NULL,
    source_account_id TEXT NOT NULL,
    source_asset TEXT NOT NULL CHECK (source_asset IN ('balance', 'credits')),
    canonical_account_id TEXT NOT NULL REFERENCES bizdecipher_ledger_accounts(id) ON DELETE RESTRICT,
    clearing_account_id TEXT NOT NULL REFERENCES bizdecipher_ledger_accounts(id) ON DELETE RESTRICT,
    amount NUMERIC(24,12) NOT NULL CHECK (amount > 0),
    source_version BIGINT NOT NULL CHECK (source_version > 0),
    source_epoch BIGINT NOT NULL CHECK (source_epoch > 0),
    source_payload_sha256 CHAR(64) NOT NULL CHECK (source_payload_sha256 ~ '^[0-9a-f]{64}$'),
    PRIMARY KEY (batch_id, source_balance_id),
    CHECK (canonical_account_id <> clearing_account_id)
);

CREATE TABLE IF NOT EXISTS bizdecipher_cutover_identity_manifest (
    cutover_attempt_id UUID NOT NULL REFERENCES shared_pool_cutover_attempts(attempt_id) ON DELETE RESTRICT,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('hold', 'accepted_media_task', 'terminal_unknown', 'outbox')),
    source_id TEXT NOT NULL,
    business_event_id TEXT NOT NULL,
    accepted_media_task_id TEXT NULL,
    amount NUMERIC(24,12) NULL,
    source_state TEXT NOT NULL,
    source_version BIGINT NOT NULL CHECK (source_version > 0),
    source_epoch BIGINT NOT NULL CHECK (source_epoch > 0),
    payload_sha256 CHAR(64) NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cutover_attempt_id, source_kind, source_id),
    CHECK ((source_kind = 'accepted_media_task') = (accepted_media_task_id IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bizdecipher_cutover_identity_event
    ON bizdecipher_cutover_identity_manifest(cutover_attempt_id, business_event_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bizdecipher_cutover_original_media_task
    ON bizdecipher_cutover_identity_manifest(cutover_attempt_id, accepted_media_task_id)
    WHERE accepted_media_task_id IS NOT NULL;

CREATE OR REPLACE FUNCTION reject_bizdecipher_cutover_snapshot_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'BizDecipher cutover snapshots are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bizdecipher_opening_balance_items_immutable ON bizdecipher_opening_balance_items;
CREATE TRIGGER trg_bizdecipher_opening_balance_items_immutable
    BEFORE UPDATE OR DELETE ON bizdecipher_opening_balance_items
    FOR EACH ROW EXECUTE FUNCTION reject_bizdecipher_cutover_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_bizdecipher_cutover_identity_manifest_immutable ON bizdecipher_cutover_identity_manifest;
CREATE TRIGGER trg_bizdecipher_cutover_identity_manifest_immutable
    BEFORE UPDATE OR DELETE ON bizdecipher_cutover_identity_manifest
    FOR EACH ROW EXECUTE FUNCTION reject_bizdecipher_cutover_snapshot_mutation();

CREATE OR REPLACE FUNCTION bizdecipher_import_opening_balances(
    expected_attempt UUID,
    expected_phase TEXT,
    expected_version BIGINT,
    expected_epoch BIGINT,
    selected_batch UUID
) RETURNS INTEGER AS $$
DECLARE
    control_row shared_pool_cutover_control%ROWTYPE;
    batch_row bizdecipher_opening_balance_batches%ROWTYPE;
    imported_count INTEGER;
BEGIN
    SELECT * INTO control_row
    FROM shared_pool_cutover_control
    WHERE singleton = TRUE
    FOR UPDATE;

    IF control_row.active_attempt_id <> expected_attempt
       OR control_row.phase <> expected_phase
       OR control_row.version <> expected_version
       OR control_row.authority_epoch <> expected_epoch
       OR expected_phase <> 'importing' THEN
        RAISE EXCEPTION 'stale opening-balance import authority' USING ERRCODE = '40001';
    END IF;

    SELECT * INTO batch_row
    FROM bizdecipher_opening_balance_batches
    WHERE batch_id = selected_batch
    FOR UPDATE;

    IF NOT FOUND
       OR batch_row.cutover_attempt_id <> expected_attempt
       OR batch_row.authority_epoch <> expected_epoch
       OR batch_row.state NOT IN ('staged', 'imported') THEN
        RAISE EXCEPTION 'opening-balance batch does not match active authority' USING ERRCODE = '40001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM bizdecipher_opening_balance_items item WHERE item.batch_id = selected_batch
    ) THEN
        RAISE EXCEPTION 'opening-balance batch is empty' USING ERRCODE = '23514';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM bizdecipher_opening_balance_items item
        WHERE item.batch_id = selected_batch
          AND item.source_epoch <> expected_epoch
    ) THEN
        RAISE EXCEPTION 'opening-balance item carries stale authority epoch' USING ERRCODE = '40001';
    END IF;

    INSERT INTO bizdecipher_ledger_journals (
        id, event_id, idempotency_key, payload_sha256, journal_type, posted_at
    )
    SELECT
        'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id,
        'opening-balance:' || item.batch_id::TEXT || ':' || item.source_balance_id,
        'opening-balance:' || item.batch_id::TEXT || ':' || item.source_balance_id,
        item.source_payload_sha256,
        'opening_balance',
        batch_row.staged_at
    FROM bizdecipher_opening_balance_items item
    WHERE item.batch_id = selected_batch
    ON CONFLICT (event_id) DO NOTHING;

    INSERT INTO bizdecipher_ledger_entries (journal_id, line_no, account_id, amount)
    SELECT
        'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id,
        entry.line_no,
        CASE WHEN entry.line_no = 1 THEN item.canonical_account_id ELSE item.clearing_account_id END,
        CASE WHEN entry.line_no = 1 THEN item.amount ELSE -item.amount END
    FROM bizdecipher_opening_balance_items item
    CROSS JOIN (VALUES (1), (2)) AS entry(line_no)
    WHERE item.batch_id = selected_batch
    ON CONFLICT (journal_id, line_no) DO NOTHING;

    IF EXISTS (
        SELECT 1
        FROM bizdecipher_opening_balance_items item
        LEFT JOIN bizdecipher_ledger_journals journal
          ON journal.id = 'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id
        WHERE item.batch_id = selected_batch
          AND (
              journal.id IS NULL
              OR journal.event_id <> 'opening-balance:' || item.batch_id::TEXT || ':' || item.source_balance_id
              OR journal.idempotency_key <> 'opening-balance:' || item.batch_id::TEXT || ':' || item.source_balance_id
              OR journal.payload_sha256 <> item.source_payload_sha256
              OR journal.journal_type <> 'opening_balance'
              OR journal.posted_at <> batch_row.staged_at
          )
    ) OR EXISTS (
        SELECT 1
        FROM bizdecipher_opening_balance_items item
        LEFT JOIN bizdecipher_ledger_entries debit
          ON debit.journal_id = 'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id
         AND debit.line_no = 1
        LEFT JOIN bizdecipher_ledger_entries credit
          ON credit.journal_id = 'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id
         AND credit.line_no = 2
        WHERE item.batch_id = selected_batch
          AND (
              debit.account_id IS DISTINCT FROM item.canonical_account_id
              OR debit.amount IS DISTINCT FROM item.amount
              OR credit.account_id IS DISTINCT FROM item.clearing_account_id
              OR credit.amount IS DISTINCT FROM -item.amount
          )
    ) THEN
        RAISE EXCEPTION 'opening-balance idempotency payload conflict' USING ERRCODE = '23505';
    END IF;

    INSERT INTO bizdecipher_ledger_cutover_pending (
        cutover_attempt_id, source_kind, source_id, payload_sha256,
        amount, authority_epoch, state, version
    )
    SELECT
        expected_attempt::TEXT,
        'business_event',
        'opening-balance:' || item.source_balance_id,
        item.source_payload_sha256,
        item.amount,
        expected_epoch,
        'cutover_pending',
        1
    FROM bizdecipher_opening_balance_items item
    WHERE item.batch_id = selected_batch
    ON CONFLICT (cutover_attempt_id, source_kind, source_id) DO NOTHING;

    IF EXISTS (
        SELECT 1
        FROM bizdecipher_opening_balance_items item
        JOIN bizdecipher_ledger_cutover_pending pending
          ON pending.cutover_attempt_id = expected_attempt::TEXT
         AND pending.source_kind = 'business_event'
         AND pending.source_id = 'opening-balance:' || item.source_balance_id
        WHERE item.batch_id = selected_batch
          AND (
              pending.payload_sha256 <> item.source_payload_sha256
              OR pending.amount IS DISTINCT FROM item.amount
              OR pending.authority_epoch <> expected_epoch
              OR pending.state <> 'cutover_pending'
          )
    ) THEN
        RAISE EXCEPTION 'opening-balance pending claim conflict' USING ERRCODE = '23505';
    END IF;

    SELECT COUNT(*) INTO imported_count
    FROM bizdecipher_ledger_journals journal
    WHERE journal.id LIKE 'opening:' || selected_batch::TEXT || ':%';

    UPDATE bizdecipher_opening_balance_batches
    SET state = 'imported', imported_at = COALESCE(imported_at, NOW())
    WHERE batch_id = selected_batch AND state = 'staged';

    RETURN imported_count;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION bizdecipher_verify_opening_balances(
    expected_attempt UUID,
    expected_version BIGINT,
    expected_epoch BIGINT,
    selected_batch UUID
) RETURNS TABLE (source_count BIGINT, journal_count BIGINT, source_total NUMERIC, entry_total NUMERIC) AS $$
DECLARE
    control_row shared_pool_cutover_control%ROWTYPE;
    batch_row bizdecipher_opening_balance_batches%ROWTYPE;
    reconciled_source_count BIGINT;
    reconciled_journal_count BIGINT;
    reconciled_source_total NUMERIC;
    reconciled_entry_total NUMERIC;
BEGIN
    SELECT * INTO control_row
    FROM shared_pool_cutover_control
    WHERE singleton = TRUE
    FOR UPDATE;

    IF control_row.active_attempt_id <> expected_attempt
       OR control_row.phase <> 'importing'
       OR control_row.version <> expected_version
       OR control_row.authority_epoch <> expected_epoch THEN
        RAISE EXCEPTION 'stale opening-balance verification authority' USING ERRCODE = '40001';
    END IF;

    SELECT * INTO batch_row
    FROM bizdecipher_opening_balance_batches
    WHERE batch_id = selected_batch
    FOR UPDATE;

    IF NOT FOUND
       OR batch_row.cutover_attempt_id <> expected_attempt
       OR batch_row.authority_epoch <> expected_epoch
       OR batch_row.state NOT IN ('imported', 'verified') THEN
        RAISE EXCEPTION 'opening-balance batch is not imported by active authority' USING ERRCODE = '40001';
    END IF;

    SELECT
        (SELECT COUNT(*) FROM bizdecipher_opening_balance_items item WHERE item.batch_id = selected_batch),
        (SELECT COUNT(*) FROM bizdecipher_ledger_journals journal WHERE journal.id LIKE 'opening:' || selected_batch::TEXT || ':%'),
        (SELECT COALESCE(SUM(item.amount), 0) FROM bizdecipher_opening_balance_items item WHERE item.batch_id = selected_batch),
        (SELECT COALESCE(SUM(entry.amount), 0)
         FROM bizdecipher_ledger_entries entry
         JOIN bizdecipher_ledger_journals journal ON journal.id = entry.journal_id
         WHERE journal.id LIKE 'opening:' || selected_batch::TEXT || ':%')
    INTO reconciled_source_count, reconciled_journal_count,
         reconciled_source_total, reconciled_entry_total;

    IF reconciled_source_count = 0
       OR reconciled_source_count <> reconciled_journal_count
       OR reconciled_entry_total <> 0
       OR EXISTS (
           SELECT 1
           FROM bizdecipher_opening_balance_items item
           JOIN bizdecipher_ledger_journals journal
             ON journal.id = 'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id
           LEFT JOIN bizdecipher_ledger_entries entry ON entry.journal_id = journal.id
           WHERE item.batch_id = selected_batch
           GROUP BY item.batch_id, item.source_balance_id, item.amount
           HAVING COUNT(entry.*) <> 2
               OR SUM(entry.amount) <> 0
               OR COUNT(*) FILTER (WHERE entry.amount = item.amount) <> 1
               OR COUNT(*) FILTER (WHERE entry.amount = -item.amount) <> 1
       ) THEN
        RAISE EXCEPTION 'opening-balance exact reconciliation failed' USING ERRCODE = '23514';
    END IF;

    UPDATE bizdecipher_opening_balance_batches
    SET state = 'verified', verified_at = COALESCE(verified_at, NOW())
    WHERE batch_id = selected_batch AND state = 'imported';

    RETURN QUERY SELECT reconciled_source_count, reconciled_journal_count,
        reconciled_source_total, reconciled_entry_total;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION sync_bizdecipher_cutover_import_state()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.phase = 'aborted_before_activation' AND OLD.phase <> NEW.phase THEN
        UPDATE bizdecipher_opening_balance_batches
        SET state = 'aborted_before_activation'
        WHERE cutover_attempt_id = NEW.attempt_id
          AND state IN ('staged', 'imported', 'verified');
        UPDATE bizdecipher_ledger_cutover_pending
        SET state = 'aborted_before_activation', version = version + 1
        WHERE cutover_attempt_id = NEW.attempt_id::TEXT
          AND state = 'cutover_pending';
    ELSIF NEW.phase = 'canonical_open' AND OLD.phase <> NEW.phase THEN
        WITH opened_batches AS (
            UPDATE bizdecipher_opening_balance_batches
            SET state = 'canonical_open'
            WHERE cutover_attempt_id = NEW.attempt_id
              AND state = 'verified'
            RETURNING batch_id
        ), projection_deltas AS (
            SELECT
                entry.account_id,
                SUM(entry.amount) AS balance
            FROM opened_batches batch
            JOIN bizdecipher_opening_balance_items item ON item.batch_id = batch.batch_id
            JOIN bizdecipher_ledger_entries entry
              ON entry.journal_id = 'opening:' || item.batch_id::TEXT || ':' || item.source_balance_id
            GROUP BY entry.account_id
        )
        INSERT INTO bizdecipher_ledger_projections (account_id, balance, reserved_amount, version, updated_at)
        SELECT account_id, balance, 0, 1, NOW()
        FROM projection_deltas
        ON CONFLICT (account_id) DO UPDATE
        SET balance = bizdecipher_ledger_projections.balance + EXCLUDED.balance,
            version = bizdecipher_ledger_projections.version + 1,
            updated_at = NOW();
        UPDATE bizdecipher_ledger_cutover_pending
        SET state = 'canonical_open', version = version + 1
        WHERE cutover_attempt_id = NEW.attempt_id::TEXT
          AND state = 'cutover_pending';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_bizdecipher_cutover_import_state ON shared_pool_cutover_attempts;
CREATE TRIGGER trg_sync_bizdecipher_cutover_import_state
    AFTER UPDATE OF phase ON shared_pool_cutover_attempts
    FOR EACH ROW EXECUTE FUNCTION sync_bizdecipher_cutover_import_state();
