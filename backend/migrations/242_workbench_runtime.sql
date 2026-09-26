CREATE TABLE IF NOT EXISTS workbench_runs (
    run_id TEXT PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    workspace_id TEXT NOT NULL,
    capability TEXT NOT NULL,
    input_snapshot JSONB NOT NULL,
    input_sha256 CHAR(64) NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued',
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    next_event_seq BIGINT NOT NULL DEFAULT 1 CHECK (next_event_seq > 0),
    lease_owner TEXT,
    lease_epoch BIGINT NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at TIMESTAMPTZ,
    replay_of_run_id TEXT REFERENCES workbench_runs(run_id) ON DELETE RESTRICT,
    forked_from_run_id TEXT REFERENCES workbench_runs(run_id) ON DELETE RESTRICT,
    accepted_quote_id TEXT REFERENCES bizdecipher_accepted_price_quotes(quote_id) ON DELETE RESTRICT,
    canonical_request_id TEXT,
    canonical_usage_event_id TEXT REFERENCES canonical_usage_outbox(event_id) ON DELETE RESTRICT,
    ledger_journal_id TEXT REFERENCES bizdecipher_ledger_journals(id) ON DELETE RESTRICT,
    canonical_media_business_event_id TEXT REFERENCES canonical_media_task_bindings(business_event_id) ON DELETE RESTRICT,
    cached_estimate NUMERIC(24,12) CHECK (cached_estimate >= 0),
    cached_actual NUMERIC(24,12) CHECK (cached_actual >= 0),
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    terminal_at TIMESTAMPTZ,
    CONSTRAINT workbench_runs_id_check CHECK (run_id ~ '^wbr_[0-9a-z]+$'),
    CONSTRAINT workbench_runs_workspace_id_check CHECK (workspace_id ~ '^wbw_[0-9a-z]+$'),
    CONSTRAINT workbench_runs_input_digest_check CHECK (input_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT workbench_runs_state_check CHECK (state IN ('queued', 'running', 'cancel_requested', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT workbench_runs_lease_shape_check CHECK (
        (lease_owner IS NULL AND lease_expires_at IS NULL)
        OR (lease_owner IS NOT NULL AND lease_epoch > 0 AND lease_expires_at IS NOT NULL)
    ),
    CONSTRAINT workbench_runs_terminal_shape_check CHECK (
        (state IN ('succeeded', 'failed', 'cancelled') AND terminal_at IS NOT NULL)
        OR (state NOT IN ('succeeded', 'failed', 'cancelled') AND terminal_at IS NULL)
    ),
    CONSTRAINT workbench_runs_distinct_lineage_check CHECK (
        replay_of_run_id IS NULL OR forked_from_run_id IS NULL
    ),
    CONSTRAINT workbench_runs_owner_identity_unique UNIQUE (run_id, owner_user_id),
    CONSTRAINT workbench_runs_owner_lineage_unique UNIQUE (owner_user_id, run_id),
    CONSTRAINT workbench_runs_replay_owner_fk FOREIGN KEY (owner_user_id, replay_of_run_id)
        REFERENCES workbench_runs(owner_user_id, run_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_runs_fork_owner_fk FOREIGN KEY (owner_user_id, forked_from_run_id)
        REFERENCES workbench_runs(owner_user_id, run_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_workbench_runs_owner_created
    ON workbench_runs(owner_user_id, created_at DESC, run_id);
CREATE INDEX IF NOT EXISTS idx_workbench_runs_recovery
    ON workbench_runs(state, lease_expires_at, run_id)
    WHERE state IN ('queued', 'running', 'cancel_requested');

CREATE TABLE IF NOT EXISTS workbench_operations (
    operation_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation_key TEXT NOT NULL,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('create', 'cancel', 'save', 'replay', 'fork')),
    request_fingerprint_sha256 CHAR(64) NOT NULL,
    state TEXT NOT NULL DEFAULT 'claimed' CHECK (state IN ('claimed', 'committed', 'failed')),
    result_run_id TEXT,
    result_snapshot_id TEXT,
    result_state TEXT CHECK (result_state IN ('queued', 'running', 'cancel_requested', 'succeeded', 'failed', 'cancelled')),
    response_snapshot JSONB,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    lease_owner TEXT,
    lease_epoch BIGINT NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workbench_operations_id_check CHECK (operation_id ~ '^wbo_[0-9a-z]+$'),
    CONSTRAINT workbench_operations_fingerprint_check CHECK (request_fingerprint_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT workbench_operations_key_unique UNIQUE (run_id, operation_key),
    CONSTRAINT workbench_operations_owner_fk FOREIGN KEY (run_id, owner_user_id)
        REFERENCES workbench_runs(run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_operations_result_owner_fk FOREIGN KEY (result_run_id, owner_user_id)
        REFERENCES workbench_runs(run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_operations_identity_unique UNIQUE (operation_id, run_id, owner_user_id),
    CONSTRAINT workbench_operations_result_shape_check CHECK (
        (state = 'committed' AND result_run_id IS NOT NULL AND result_state IS NOT NULL)
        OR (state <> 'committed' AND result_run_id IS NULL AND result_snapshot_id IS NULL AND result_state IS NULL)
    ),
    CONSTRAINT workbench_operations_lease_shape_check CHECK (
        (lease_owner IS NULL AND lease_expires_at IS NULL)
        OR (lease_owner IS NOT NULL AND lease_epoch > 0 AND lease_expires_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_workbench_operations_owner_created
    ON workbench_operations(owner_user_id, created_at DESC, operation_id);

CREATE TABLE IF NOT EXISTS workbench_run_events (
    run_id TEXT NOT NULL REFERENCES workbench_runs(run_id) ON DELETE RESTRICT,
    seq BIGINT NOT NULL,
    event_id TEXT NOT NULL,
    event_kind TEXT NOT NULL,
    event_payload JSONB NOT NULL,
    payload_sha256 CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, seq),
    CONSTRAINT workbench_run_events_seq_check CHECK (seq > 0),
    CONSTRAINT workbench_run_events_id_check CHECK (event_id ~ '^wbe_[0-9a-z]+$'),
    CONSTRAINT workbench_run_events_id_unique UNIQUE (event_id),
    CONSTRAINT workbench_run_events_digest_check CHECK (payload_sha256 ~ '^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS workbench_artifacts (
    artifact_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    artifact_kind TEXT NOT NULL CHECK (artifact_kind IN ('text', 'code', 'image', 'video')),
    media_type TEXT NOT NULL,
    storage_uri TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    digest_sha256 CHAR(64) NOT NULL,
    accepted_quote_id TEXT REFERENCES bizdecipher_accepted_price_quotes(quote_id) ON DELETE RESTRICT,
    canonical_request_id TEXT,
    canonical_usage_event_id TEXT REFERENCES canonical_usage_outbox(event_id) ON DELETE RESTRICT,
    ledger_journal_id TEXT REFERENCES bizdecipher_ledger_journals(id) ON DELETE RESTRICT,
    canonical_media_business_event_id TEXT REFERENCES canonical_media_task_bindings(business_event_id) ON DELETE RESTRICT,
    runner_job_id TEXT,
    upstream_task_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workbench_artifacts_id_check CHECK (artifact_id ~ '^wba_[0-9a-z]+$'),
    CONSTRAINT workbench_artifacts_digest_check CHECK (digest_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT workbench_artifacts_identity_unique UNIQUE (run_id, digest_sha256),
    CONSTRAINT workbench_artifacts_owner_fk FOREIGN KEY (run_id, owner_user_id)
        REFERENCES workbench_runs(run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_artifacts_owner_identity_unique UNIQUE (artifact_id, run_id, owner_user_id)
);

CREATE INDEX IF NOT EXISTS idx_workbench_artifacts_owner_run
    ON workbench_artifacts(owner_user_id, run_id, created_at, artifact_id);

CREATE TABLE IF NOT EXISTS workbench_saved_snapshots (
    snapshot_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    snapshot JSONB NOT NULL,
    snapshot_sha256 CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workbench_saved_snapshots_id_check CHECK (snapshot_id ~ '^wbs_[0-9a-z]+$'),
    CONSTRAINT workbench_saved_snapshots_digest_check CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT workbench_saved_snapshots_identity_unique UNIQUE (run_id, snapshot_sha256),
    CONSTRAINT workbench_saved_snapshots_owner_fk FOREIGN KEY (run_id, owner_user_id)
        REFERENCES workbench_runs(run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_saved_snapshots_owner_identity_unique UNIQUE (snapshot_id, run_id, owner_user_id),
    CONSTRAINT workbench_saved_snapshots_owner_snapshot_unique UNIQUE (owner_user_id, snapshot_id)
);

CREATE INDEX IF NOT EXISTS idx_workbench_saved_snapshots_owner_created
    ON workbench_saved_snapshots(owner_user_id, created_at DESC, snapshot_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'workbench_operations_result_snapshot_fk'
    ) THEN
        ALTER TABLE workbench_operations
            ADD CONSTRAINT workbench_operations_result_snapshot_fk
            FOREIGN KEY (owner_user_id, result_snapshot_id)
            REFERENCES workbench_saved_snapshots(owner_user_id, snapshot_id) ON DELETE RESTRICT;
    END IF;
END;
$$;

CREATE TABLE IF NOT EXISTS workbench_run_derivations (
    derivation_id TEXT PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_run_id TEXT NOT NULL,
    source_snapshot_id TEXT NOT NULL,
    derived_run_id TEXT NOT NULL,
    derivation_kind TEXT NOT NULL CHECK (derivation_kind IN ('replay', 'fork')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workbench_run_derivations_id_check CHECK (derivation_id ~ '^wbd_[0-9a-z]+$'),
    CONSTRAINT workbench_run_derivations_pair_unique UNIQUE (source_run_id, derived_run_id, derivation_kind),
    CONSTRAINT workbench_run_derivations_distinct_run_check CHECK (source_run_id <> derived_run_id),
    CONSTRAINT workbench_run_derivations_source_owner_fk FOREIGN KEY (source_run_id, owner_user_id)
        REFERENCES workbench_runs(run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_run_derivations_derived_owner_fk FOREIGN KEY (derived_run_id, owner_user_id)
        REFERENCES workbench_runs(run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT workbench_run_derivations_snapshot_owner_fk FOREIGN KEY (source_snapshot_id, source_run_id, owner_user_id)
        REFERENCES workbench_saved_snapshots(snapshot_id, run_id, owner_user_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_workbench_run_derivations_derived
    ON workbench_run_derivations(derived_run_id, derivation_id);

CREATE TABLE IF NOT EXISTS canonical_runner_jobs (
    job_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    operation_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_fingerprint_sha256 CHAR(64) NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'leased', 'running', 'succeeded', 'failed', 'cancelled')),
    output_artifact_id TEXT,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    lease_owner TEXT,
    lease_epoch BIGINT NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT canonical_runner_jobs_id_check CHECK (job_id ~ '^wbj_[0-9a-z]+$'),
    CONSTRAINT canonical_runner_jobs_fingerprint_check CHECK (request_fingerprint_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT canonical_runner_jobs_idempotency_unique UNIQUE (run_id, idempotency_key),
    CONSTRAINT canonical_runner_jobs_operation_fk FOREIGN KEY (operation_id, run_id, owner_user_id)
        REFERENCES workbench_operations(operation_id, run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT canonical_runner_jobs_output_fk FOREIGN KEY (output_artifact_id, run_id, owner_user_id)
        REFERENCES workbench_artifacts(artifact_id, run_id, owner_user_id) ON DELETE RESTRICT,
    CONSTRAINT canonical_runner_jobs_lease_shape_check CHECK (
        (lease_owner IS NULL AND lease_expires_at IS NULL)
        OR (lease_owner IS NOT NULL AND lease_epoch > 0 AND lease_expires_at IS NOT NULL)
    ),
    CONSTRAINT canonical_runner_jobs_terminal_output_check CHECK (
        state <> 'succeeded' OR output_artifact_id IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_canonical_runner_jobs_claim
    ON canonical_runner_jobs(state, lease_expires_at, job_id)
    WHERE state IN ('queued', 'leased', 'running');

CREATE OR REPLACE FUNCTION protect_workbench_run_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.run_id IS DISTINCT FROM OLD.run_id
       OR NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id
       OR NEW.workspace_id IS DISTINCT FROM OLD.workspace_id
       OR NEW.capability IS DISTINCT FROM OLD.capability
       OR NEW.input_snapshot IS DISTINCT FROM OLD.input_snapshot
       OR NEW.input_sha256 IS DISTINCT FROM OLD.input_sha256
       OR NEW.replay_of_run_id IS DISTINCT FROM OLD.replay_of_run_id
       OR NEW.forked_from_run_id IS DISTINCT FROM OLD.forked_from_run_id
       OR NEW.accepted_quote_id IS DISTINCT FROM OLD.accepted_quote_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'workbench run immutable fields cannot change';
    END IF;
    IF OLD.state IN ('succeeded', 'failed', 'cancelled') THEN
        RAISE EXCEPTION 'workbench terminal lineage is immutable';
    END IF;
    IF OLD.canonical_request_id IS NOT NULL AND NEW.canonical_request_id IS DISTINCT FROM OLD.canonical_request_id
       OR OLD.canonical_usage_event_id IS NOT NULL AND NEW.canonical_usage_event_id IS DISTINCT FROM OLD.canonical_usage_event_id
       OR OLD.ledger_journal_id IS NOT NULL AND NEW.ledger_journal_id IS DISTINCT FROM OLD.ledger_journal_id
       OR OLD.canonical_media_business_event_id IS NOT NULL AND NEW.canonical_media_business_event_id IS DISTINCT FROM OLD.canonical_media_business_event_id THEN
        RAISE EXCEPTION 'workbench canonical lineage is write-once';
    END IF;
    IF NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'workbench run version must advance exactly once';
    END IF;
    IF NEW.next_event_seq < OLD.next_event_seq OR NEW.lease_epoch < OLD.lease_epoch THEN
        RAISE EXCEPTION 'workbench run fencing values cannot move backward';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_workbench_run_mutation ON workbench_runs;
CREATE TRIGGER trg_workbench_run_mutation
    BEFORE UPDATE ON workbench_runs
    FOR EACH ROW EXECUTE FUNCTION protect_workbench_run_mutation();

CREATE OR REPLACE FUNCTION protect_workbench_operation_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.operation_id IS DISTINCT FROM OLD.operation_id
       OR NEW.run_id IS DISTINCT FROM OLD.run_id
       OR NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id
       OR NEW.operation_key IS DISTINCT FROM OLD.operation_key
       OR NEW.operation_kind IS DISTINCT FROM OLD.operation_kind
       OR NEW.request_fingerprint_sha256 IS DISTINCT FROM OLD.request_fingerprint_sha256
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'workbench operation identity is immutable';
    END IF;
    IF OLD.state IN ('committed', 'failed') THEN
        RAISE EXCEPTION 'workbench terminal operation is immutable';
    END IF;
    IF NEW.version <> OLD.version + 1 OR NEW.lease_epoch < OLD.lease_epoch THEN
        RAISE EXCEPTION 'workbench operation fencing is stale';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_workbench_operation_mutation ON workbench_operations;
CREATE TRIGGER trg_workbench_operation_mutation
    BEFORE UPDATE ON workbench_operations
    FOR EACH ROW EXECUTE FUNCTION protect_workbench_operation_mutation();

CREATE OR REPLACE FUNCTION protect_canonical_runner_job_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.job_id IS DISTINCT FROM OLD.job_id
       OR NEW.run_id IS DISTINCT FROM OLD.run_id
       OR NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id
       OR NEW.operation_id IS DISTINCT FROM OLD.operation_id
       OR NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key
       OR NEW.request_fingerprint_sha256 IS DISTINCT FROM OLD.request_fingerprint_sha256
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'canonical runner job identity is immutable';
    END IF;
    IF OLD.state IN ('succeeded', 'failed', 'cancelled') THEN
        RAISE EXCEPTION 'canonical runner terminal job is immutable';
    END IF;
    IF NEW.version <> OLD.version + 1 OR NEW.lease_epoch < OLD.lease_epoch THEN
        RAISE EXCEPTION 'canonical runner job fencing is stale';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_canonical_runner_job_mutation ON canonical_runner_jobs;
CREATE TRIGGER trg_canonical_runner_job_mutation
    BEFORE UPDATE ON canonical_runner_jobs
    FOR EACH ROW EXECUTE FUNCTION protect_canonical_runner_job_mutation();

CREATE OR REPLACE FUNCTION reject_workbench_immutable_row_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'workbench immutable row cannot change';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_workbench_run_events_immutable ON workbench_run_events;
CREATE TRIGGER trg_workbench_run_events_immutable
    BEFORE UPDATE OR DELETE ON workbench_run_events
    FOR EACH ROW EXECUTE FUNCTION reject_workbench_immutable_row_mutation();

DROP TRIGGER IF EXISTS trg_workbench_artifacts_immutable ON workbench_artifacts;
CREATE TRIGGER trg_workbench_artifacts_immutable
    BEFORE UPDATE OR DELETE ON workbench_artifacts
    FOR EACH ROW EXECUTE FUNCTION reject_workbench_immutable_row_mutation();

DROP TRIGGER IF EXISTS trg_workbench_saved_snapshots_immutable ON workbench_saved_snapshots;
CREATE TRIGGER trg_workbench_saved_snapshots_immutable
    BEFORE UPDATE OR DELETE ON workbench_saved_snapshots
    FOR EACH ROW EXECUTE FUNCTION reject_workbench_immutable_row_mutation();

DROP TRIGGER IF EXISTS trg_workbench_run_derivations_immutable ON workbench_run_derivations;
CREATE TRIGGER trg_workbench_run_derivations_immutable
    BEFORE UPDATE OR DELETE ON workbench_run_derivations
    FOR EACH ROW EXECUTE FUNCTION reject_workbench_immutable_row_mutation();
