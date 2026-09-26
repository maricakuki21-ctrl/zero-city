SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS shared_pool_cutover_attempts (
    attempt_id UUID PRIMARY KEY,
    phase VARCHAR(40) NOT NULL,
    authority_epoch BIGINT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    first_canonical_effect BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    terminal_at TIMESTAMPTZ NULL,
    CONSTRAINT shared_pool_cutover_attempts_epoch_check CHECK (authority_epoch > 0),
    CONSTRAINT shared_pool_cutover_attempts_version_check CHECK (version > 0),
    CONSTRAINT shared_pool_cutover_attempts_phase_check CHECK (phase IN (
        'legacy_open', 'draining', 'fenced', 'importing', 'verified',
        'canonical_open', 'canonical_recovering', 'complete',
        'aborted_before_activation'
    )),
    CONSTRAINT shared_pool_cutover_attempts_epoch_unique UNIQUE (authority_epoch),
    CONSTRAINT shared_pool_cutover_attempts_terminal_check CHECK (
        (phase IN ('complete', 'aborted_before_activation')) = (terminal_at IS NOT NULL)
    ),
    CONSTRAINT shared_pool_cutover_attempts_effect_check CHECK (
        NOT first_canonical_effect OR phase IN ('canonical_open', 'canonical_recovering', 'complete')
    )
);

CREATE TABLE IF NOT EXISTS shared_pool_cutover_control (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    active_attempt_id UUID NOT NULL REFERENCES shared_pool_cutover_attempts(attempt_id) ON DELETE RESTRICT,
    phase VARCHAR(40) NOT NULL,
    authority_epoch BIGINT NOT NULL,
    version BIGINT NOT NULL,
    first_canonical_effect BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_cutover_control_epoch_check CHECK (authority_epoch > 0),
    CONSTRAINT shared_pool_cutover_control_version_check CHECK (version > 0),
    CONSTRAINT shared_pool_cutover_control_phase_check CHECK (phase IN (
        'legacy_open', 'draining', 'fenced', 'importing', 'verified',
        'canonical_open', 'canonical_recovering', 'complete',
        'aborted_before_activation'
    )),
    CONSTRAINT shared_pool_cutover_control_effect_check CHECK (
        NOT first_canonical_effect OR phase IN ('canonical_open', 'canonical_recovering', 'complete')
    )
);

CREATE TABLE IF NOT EXISTS shared_pool_cutover_transitions (
    id BIGSERIAL PRIMARY KEY,
    attempt_id UUID NOT NULL REFERENCES shared_pool_cutover_attempts(attempt_id) ON DELETE RESTRICT,
    from_authority_epoch BIGINT NOT NULL,
    to_authority_epoch BIGINT NOT NULL,
    from_phase VARCHAR(40) NULL,
    to_phase VARCHAR(40) NOT NULL,
    from_version BIGINT NOT NULL,
    to_version BIGINT NOT NULL,
    transition_kind VARCHAR(32) NOT NULL DEFAULT 'phase',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shared_pool_cutover_transitions_version_check CHECK (to_version = from_version + 1),
    CONSTRAINT shared_pool_cutover_transitions_epoch_check CHECK (
        to_authority_epoch = from_authority_epoch
        OR to_authority_epoch = from_authority_epoch + 1
    ),
    CONSTRAINT shared_pool_cutover_transitions_kind_check CHECK (
        transition_kind IN ('phase', 'first_canonical_effect', 'post_fence_abort')
    ),
    CONSTRAINT shared_pool_cutover_transitions_unique UNIQUE (attempt_id, to_version)
);

CREATE OR REPLACE FUNCTION preserve_terminal_shared_pool_cutover_attempt()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'shared-pool cutover attempts are retained for audit' USING ERRCODE = '55000';
    END IF;
    IF OLD.phase IN ('complete', 'aborted_before_activation') THEN
        RAISE EXCEPTION 'terminal shared-pool cutover attempt is immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_preserve_terminal_shared_pool_cutover_attempt ON shared_pool_cutover_attempts;
CREATE TRIGGER trg_preserve_terminal_shared_pool_cutover_attempt
BEFORE UPDATE OR DELETE ON shared_pool_cutover_attempts
FOR EACH ROW EXECUTE FUNCTION preserve_terminal_shared_pool_cutover_attempt();

CREATE OR REPLACE FUNCTION preserve_shared_pool_cutover_transition()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'shared-pool cutover transition history is append-only' USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_preserve_shared_pool_cutover_transition ON shared_pool_cutover_transitions;
CREATE TRIGGER trg_preserve_shared_pool_cutover_transition
BEFORE UPDATE OR DELETE ON shared_pool_cutover_transitions
FOR EACH ROW EXECUTE FUNCTION preserve_shared_pool_cutover_transition();

DO $$
DECLARE
    initial_attempt UUID := gen_random_uuid();
BEGIN
    IF NOT EXISTS (SELECT 1 FROM shared_pool_cutover_control WHERE singleton = TRUE) THEN
        INSERT INTO shared_pool_cutover_attempts (attempt_id, phase, authority_epoch, version)
        VALUES (initial_attempt, 'legacy_open', 1, 1);
        INSERT INTO shared_pool_cutover_control (
            singleton, active_attempt_id, phase, authority_epoch, version, first_canonical_effect
        ) VALUES (TRUE, initial_attempt, 'legacy_open', 1, 1, FALSE);
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION shared_pool_cutover_transition(
    expected_attempt UUID,
    expected_phase VARCHAR,
    expected_version BIGINT,
    expected_epoch BIGINT,
    next_phase VARCHAR
) RETURNS TABLE (
    attempt_id UUID, phase VARCHAR, authority_epoch BIGINT,
    version BIGINT, first_canonical_effect BOOLEAN
) AS $$
DECLARE
    control_row shared_pool_cutover_control%ROWTYPE;
    legal BOOLEAN;
    next_epoch BIGINT;
BEGIN
    SELECT * INTO control_row FROM shared_pool_cutover_control
    WHERE singleton = TRUE FOR UPDATE;
    IF control_row.active_attempt_id <> expected_attempt
       OR control_row.phase <> expected_phase
       OR control_row.version <> expected_version
       OR control_row.authority_epoch <> expected_epoch THEN
        RAISE EXCEPTION 'stale shared-pool cutover authority' USING ERRCODE = '40001';
    END IF;
    legal := CASE control_row.phase
        WHEN 'legacy_open' THEN next_phase = 'draining'
        WHEN 'draining' THEN next_phase IN ('fenced', 'legacy_open')
        WHEN 'fenced' THEN next_phase = 'importing'
        WHEN 'importing' THEN next_phase = 'verified'
        WHEN 'verified' THEN next_phase = 'canonical_open'
        WHEN 'canonical_open' THEN next_phase IN ('canonical_recovering', 'complete')
        WHEN 'canonical_recovering' THEN next_phase IN ('canonical_open', 'complete')
        ELSE FALSE
    END;
    IF NOT legal THEN
        RAISE EXCEPTION 'illegal shared-pool cutover transition: % -> %', control_row.phase, next_phase
            USING ERRCODE = '23514';
    END IF;

    next_epoch := control_row.authority_epoch + CASE WHEN next_phase = 'fenced' THEN 1 ELSE 0 END;

    UPDATE shared_pool_cutover_attempts AS attempt
    SET phase = next_phase, authority_epoch = next_epoch, version = attempt.version + 1,
        terminal_at = CASE WHEN next_phase = 'complete' THEN NOW() ELSE NULL END
    WHERE attempt.attempt_id = expected_attempt;
    UPDATE shared_pool_cutover_control AS control
    SET phase = next_phase, authority_epoch = next_epoch, version = control.version + 1, updated_at = NOW()
    WHERE control.singleton = TRUE;
    INSERT INTO shared_pool_cutover_transitions (
        attempt_id, from_authority_epoch, to_authority_epoch,
        from_phase, to_phase, from_version, to_version
    ) VALUES (
        expected_attempt, expected_epoch, next_epoch,
        expected_phase, next_phase, expected_version, expected_version + 1
    );

    RETURN QUERY SELECT c.active_attempt_id, c.phase, c.authority_epoch, c.version, c.first_canonical_effect
    FROM shared_pool_cutover_control c WHERE c.singleton = TRUE;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION shared_pool_cutover_abort_after_fence(
    expected_attempt UUID,
    expected_phase VARCHAR,
    expected_version BIGINT,
    expected_epoch BIGINT,
    replacement_attempt UUID
) RETURNS TABLE (
    failed_attempt_id UUID, failed_version BIGINT,
    active_attempt_id UUID, authority_epoch BIGINT, version BIGINT
) AS $$
DECLARE
    control_row shared_pool_cutover_control%ROWTYPE;
BEGIN
    SELECT * INTO control_row FROM shared_pool_cutover_control
    WHERE singleton = TRUE FOR UPDATE;
    IF control_row.active_attempt_id <> expected_attempt
       OR control_row.phase <> expected_phase
       OR control_row.version <> expected_version
       OR control_row.authority_epoch <> expected_epoch THEN
        RAISE EXCEPTION 'stale shared-pool cutover authority' USING ERRCODE = '40001';
    END IF;
    IF control_row.phase NOT IN ('fenced', 'importing', 'verified') THEN
        RAISE EXCEPTION 'post-fence abort is not legal from %', control_row.phase USING ERRCODE = '23514';
    END IF;
    IF replacement_attempt = expected_attempt THEN
        RAISE EXCEPTION 'replacement cutover attempt must be new' USING ERRCODE = '23514';
    END IF;

    UPDATE shared_pool_cutover_attempts AS attempt
    SET phase = 'aborted_before_activation', version = attempt.version + 1, terminal_at = NOW()
    WHERE attempt.attempt_id = expected_attempt;
    INSERT INTO shared_pool_cutover_transitions (
        attempt_id, from_authority_epoch, to_authority_epoch,
        from_phase, to_phase, from_version, to_version, transition_kind
    ) VALUES (
        expected_attempt, expected_epoch, expected_epoch,
        expected_phase, 'aborted_before_activation',
        expected_version, expected_version + 1, 'post_fence_abort'
    );
    INSERT INTO shared_pool_cutover_attempts (attempt_id, phase, authority_epoch, version)
    VALUES (replacement_attempt, 'legacy_open', expected_epoch + 1, expected_version + 1);
    UPDATE shared_pool_cutover_control
    SET active_attempt_id = replacement_attempt, phase = 'legacy_open',
        authority_epoch = expected_epoch + 1, version = expected_version + 1,
        first_canonical_effect = FALSE, updated_at = NOW()
    WHERE singleton = TRUE;

    RETURN QUERY SELECT expected_attempt, expected_version + 1,
        replacement_attempt, expected_epoch + 1, expected_version + 1;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION shared_pool_cutover_mark_first_canonical_effect(
    expected_attempt UUID,
    expected_version BIGINT,
    expected_epoch BIGINT
) RETURNS TABLE (
    attempt_id UUID, phase VARCHAR, authority_epoch BIGINT,
    version BIGINT, first_canonical_effect BOOLEAN
) AS $$
DECLARE
    control_row shared_pool_cutover_control%ROWTYPE;
BEGIN
    SELECT * INTO control_row FROM shared_pool_cutover_control
    WHERE singleton = TRUE FOR UPDATE;
    IF control_row.active_attempt_id <> expected_attempt
       OR control_row.phase <> 'canonical_open'
       OR control_row.version <> expected_version
       OR control_row.authority_epoch <> expected_epoch THEN
        RAISE EXCEPTION 'canonical effect authority is not open' USING ERRCODE = '40001';
    END IF;
    IF NOT control_row.first_canonical_effect THEN
        UPDATE shared_pool_cutover_attempts AS attempt
        SET first_canonical_effect = TRUE, version = attempt.version + 1
        WHERE attempt.attempt_id = expected_attempt;
        UPDATE shared_pool_cutover_control AS control
        SET first_canonical_effect = TRUE, version = control.version + 1, updated_at = NOW()
        WHERE control.singleton = TRUE;
        INSERT INTO shared_pool_cutover_transitions (
            attempt_id, from_authority_epoch, to_authority_epoch, from_phase, to_phase,
            from_version, to_version, transition_kind
        ) VALUES (
            expected_attempt, expected_epoch, expected_epoch, 'canonical_open', 'canonical_open',
            expected_version, expected_version + 1, 'first_canonical_effect'
        );
    END IF;
    RETURN QUERY SELECT c.active_attempt_id, c.phase, c.authority_epoch, c.version, c.first_canonical_effect
    FROM shared_pool_cutover_control c WHERE c.singleton = TRUE;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION assert_shared_pool_legacy_runtime_epoch(writer_epoch BIGINT)
RETURNS VOID AS $$
DECLARE
    control_row shared_pool_cutover_control%ROWTYPE;
BEGIN
    SELECT * INTO control_row FROM shared_pool_cutover_control WHERE singleton = TRUE;
    IF writer_epoch IS NULL OR writer_epoch <> control_row.authority_epoch THEN
        RAISE EXCEPTION 'stale shared-pool legacy runtime epoch' USING ERRCODE = '40001';
    END IF;
    IF control_row.phase NOT IN ('legacy_open', 'draining') THEN
        RAISE EXCEPTION 'shared-pool legacy runtime is fenced' USING ERRCODE = '55000';
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

CREATE OR REPLACE FUNCTION guard_shared_pool_legacy_runtime_write()
RETURNS TRIGGER AS $$
DECLARE
    writer_epoch_text TEXT;
BEGIN
    writer_epoch_text := current_setting('bizdecipher.shared_pool_authority_epoch', TRUE);
    IF writer_epoch_text IS NULL OR BTRIM(writer_epoch_text) = '' THEN
        RAISE EXCEPTION 'shared-pool legacy runtime write requires a fenced authority epoch' USING ERRCODE = '40001';
    END IF;
    PERFORM assert_shared_pool_legacy_runtime_epoch(writer_epoch_text::BIGINT);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_guard_shared_pool_usage_windows ON shared_pool_model_usage_windows;
CREATE TRIGGER trg_guard_shared_pool_usage_windows
BEFORE INSERT OR UPDATE OR DELETE ON shared_pool_model_usage_windows
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_probe_histories ON shared_pool_probe_histories;
CREATE TRIGGER trg_guard_shared_pool_probe_histories
BEFORE INSERT OR UPDATE OR DELETE ON shared_pool_probe_histories
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_usage_traces ON shared_pool_usage_traces;
CREATE TRIGGER trg_guard_shared_pool_usage_traces
BEFORE INSERT OR UPDATE OR DELETE ON shared_pool_usage_traces
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_probe_jobs ON shared_pool_probe_jobs;
CREATE TRIGGER trg_guard_shared_pool_probe_jobs
BEFORE INSERT OR UPDATE OR DELETE ON shared_pool_probe_jobs
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_probe_job_items ON shared_pool_probe_job_items;
CREATE TRIGGER trg_guard_shared_pool_probe_job_items
BEFORE INSERT OR UPDATE OR DELETE ON shared_pool_probe_job_items
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_media_probes ON shared_pool_media_endpoint_probes;
CREATE TRIGGER trg_guard_shared_pool_media_probes
BEFORE INSERT OR UPDATE OR DELETE ON shared_pool_media_endpoint_probes
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_runtime_counters ON shared_pools;
CREATE TRIGGER trg_guard_shared_pool_runtime_counters
BEFORE UPDATE OF total_calls, successful_calls, failed_calls, avg_latency_ms,
    today_availability, seven_day_availability, last_probe_at, last_probe_success,
    last_probe_error_type, last_probe_error_message, consecutive_probe_failures,
    last_successful_probe_at ON shared_pools
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

DROP TRIGGER IF EXISTS trg_guard_shared_pool_account_runtime ON shared_pool_accounts;
CREATE TRIGGER trg_guard_shared_pool_account_runtime
BEFORE UPDATE OF total_calls, successful_calls, failed_calls, last_used_at,
    last_probe_at, last_probe_success, last_probe_error_type,
    last_probe_error_message, last_successful_probe_at, full_check_score,
    full_check_passed, full_check_total ON shared_pool_accounts
FOR EACH ROW EXECUTE FUNCTION guard_shared_pool_legacy_runtime_write();

COMMENT ON TABLE shared_pool_cutover_attempts IS
    'Cutover attempt summaries retained permanently; terminal failed attempts are never reused.';
COMMENT ON TABLE shared_pool_cutover_transitions IS
    'Append-only fenced transition history for the shared-pool runtime authority.';
