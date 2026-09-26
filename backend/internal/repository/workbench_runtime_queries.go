package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

const workbenchRunSelectList = `run_id, owner_user_id, workspace_id, capability, input_snapshot,
	input_sha256, state, version, next_event_seq, replay_of_run_id, forked_from_run_id,
	accepted_quote_id, canonical_request_id, canonical_usage_event_id, ledger_journal_id,
	canonical_media_business_event_id, cached_estimate::text, cached_actual::text, error_code,
	created_at, updated_at, terminal_at`

const (
	workbenchAdvisoryLockSQL = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`
	workbenchSelectRunSQL    = `SELECT ` + workbenchRunSelectList + ` FROM workbench_runs WHERE run_id = $1 AND owner_user_id = $2`
	workbenchLockRunSQL      = workbenchSelectRunSQL + ` FOR UPDATE`
	workbenchInsertRunSQL    = `INSERT INTO workbench_runs (
		run_id, owner_user_id, workspace_id, capability, input_snapshot, input_sha256, state,
		version, next_event_seq, replay_of_run_id, forked_from_run_id, accepted_quote_id,
		canonical_request_id, canonical_usage_event_id, ledger_journal_id,
		canonical_media_business_event_id, cached_estimate, cached_actual, error_code,
		created_at, updated_at, terminal_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),
		NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),NULLIF($16,''),NULLIF($17,'')::numeric,
		NULLIF($18,'')::numeric,NULLIF($19,''),$20,$21,$22) RETURNING ` + workbenchRunSelectList
	workbenchUpdateRunSQL = `UPDATE workbench_runs SET state = $3, version = version + 1,
		next_event_seq = next_event_seq + 1, canonical_request_id = COALESCE(canonical_request_id,NULLIF($4,'')),
		canonical_usage_event_id = COALESCE(canonical_usage_event_id,NULLIF($5,'')),
		ledger_journal_id = COALESCE(ledger_journal_id,NULLIF($6,'')),
		canonical_media_business_event_id = COALESCE(canonical_media_business_event_id,NULLIF($7,'')),
		cached_estimate = NULLIF($8,'')::numeric, cached_actual = NULLIF($9,'')::numeric,
		error_code = NULLIF($10,''), updated_at = $11, terminal_at = $12
		WHERE run_id = $1 AND owner_user_id = $2 AND version = $13 RETURNING ` + workbenchRunSelectList
	workbenchSelectOperationSQL = `SELECT operation_id, run_id, owner_user_id, operation_key,
		operation_kind, request_fingerprint_sha256, state, result_run_id, result_snapshot_id, result_state
		FROM workbench_operations WHERE run_id = $1 AND operation_key = $2 AND owner_user_id = $3`
	workbenchClaimOperationSQL = `INSERT INTO workbench_operations (
		operation_id, run_id, owner_user_id, operation_key, operation_kind, request_fingerprint_sha256, state
	) VALUES ($1,$2,$3,$4,$5,$6,'claimed') ON CONFLICT (run_id, operation_key) DO NOTHING RETURNING operation_id`
	workbenchCommitOperationSQL = `UPDATE workbench_operations SET state = 'committed', result_run_id = $4,
		result_snapshot_id = NULLIF($5,''), result_state = $6, response_snapshot = $7,
		version = version + 1, updated_at = NOW()
		WHERE operation_id = $1 AND run_id = $2 AND owner_user_id = $3 AND state = 'claimed'`
	workbenchInsertCommittedOperationSQL = `INSERT INTO workbench_operations (
		operation_id, run_id, owner_user_id, operation_key, operation_kind, request_fingerprint_sha256,
		state, result_run_id, result_snapshot_id, result_state, response_snapshot
	) VALUES ($1,$2,$3,$4,$5,$6,'committed',$7,NULLIF($8,''),$9,$10)`
	workbenchInsertEventSQL = `INSERT INTO workbench_run_events (
		run_id, seq, event_id, event_kind, event_payload, payload_sha256, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	workbenchSelectEventsSQL = `SELECT event.run_id, event.seq, event.event_id, event.event_kind,
		event.event_payload, event.created_at FROM workbench_run_events event
		JOIN workbench_runs run ON run.run_id = event.run_id
		WHERE event.run_id = $1 AND run.owner_user_id = $2 AND event.seq > $3
		ORDER BY event.seq ASC LIMIT $4`
	workbenchInsertArtifactSQL = `INSERT INTO workbench_artifacts (
		artifact_id, run_id, owner_user_id, artifact_kind, media_type, storage_uri, byte_size,
		digest_sha256, accepted_quote_id, canonical_request_id, canonical_usage_event_id,
		ledger_journal_id, canonical_media_business_event_id, runner_job_id, upstream_task_id, created_at, content_bytes
	) SELECT $1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),
		NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),$16,$17 FROM workbench_runs run
		WHERE run.run_id = $2 AND run.owner_user_id = $3 ON CONFLICT DO NOTHING
		RETURNING artifact_id, run_id, owner_user_id, artifact_kind, media_type, storage_uri, byte_size,
		digest_sha256, accepted_quote_id, canonical_request_id, canonical_usage_event_id,
		ledger_journal_id, canonical_media_business_event_id, runner_job_id, upstream_task_id, created_at, content_bytes`
	workbenchSelectArtifactSQL = `SELECT artifact.artifact_id, artifact.run_id, artifact.owner_user_id,
		artifact.artifact_kind, artifact.media_type, artifact.storage_uri, artifact.byte_size,
		artifact.digest_sha256, artifact.accepted_quote_id, artifact.canonical_request_id,
		artifact.canonical_usage_event_id, artifact.ledger_journal_id,
		artifact.canonical_media_business_event_id, artifact.runner_job_id, artifact.upstream_task_id,
		artifact.created_at, artifact.content_bytes FROM workbench_artifacts artifact JOIN workbench_runs run ON run.run_id = artifact.run_id
		WHERE artifact.run_id = $1 AND run.owner_user_id = $2 AND artifact.artifact_id = $3`
	workbenchInsertSnapshotSQL = `INSERT INTO workbench_saved_snapshots (
		snapshot_id, run_id, owner_user_id, snapshot, snapshot_sha256, created_at
	) SELECT $1,$2,$3,$4,$5,$6 FROM workbench_runs run WHERE run.run_id = $2 AND run.owner_user_id = $3
		ON CONFLICT DO NOTHING RETURNING snapshot_id, run_id, owner_user_id, snapshot, snapshot_sha256, created_at`
	workbenchSelectSnapshotSQL = `SELECT snapshot.snapshot_id, snapshot.run_id, snapshot.owner_user_id,
		snapshot.snapshot, snapshot.snapshot_sha256, snapshot.created_at FROM workbench_saved_snapshots snapshot
		JOIN workbench_runs run ON run.run_id = snapshot.run_id
		WHERE snapshot.snapshot_id = $1 AND snapshot.run_id = $2 AND run.owner_user_id = $3`
	workbenchInsertDerivationSQL = `INSERT INTO workbench_run_derivations (
		derivation_id, owner_user_id, source_run_id, source_snapshot_id, derived_run_id, derivation_kind, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	workbenchSelectCanonicalLineageSQL = `SELECT usage.event_id, usage.request_id, usage.payload_sha256,
		receipt.journal_id FROM workbench_runs run JOIN canonical_usage_outbox usage
		ON usage.event_id = run.canonical_usage_event_id JOIN bizdecipher_ledger_outbox_receipts receipt
		ON receipt.event_id = usage.event_id AND receipt.payload_sha256 = usage.payload_sha256
		WHERE run.run_id = $1 AND run.owner_user_id = $2`
)

var (
	workbenchRunColumns       = []string{"run_id", "owner_user_id", "workspace_id", "capability", "input_snapshot", "input_sha256", "state", "version", "next_event_seq", "replay_of_run_id", "forked_from_run_id", "accepted_quote_id", "canonical_request_id", "canonical_usage_event_id", "ledger_journal_id", "canonical_media_business_event_id", "cached_estimate", "cached_actual", "error_code", "created_at", "updated_at", "terminal_at"}
	workbenchOperationColumns = []string{"operation_id", "run_id", "owner_user_id", "operation_key", "operation_kind", "request_fingerprint_sha256", "state", "result_run_id", "result_snapshot_id", "result_state"}
	workbenchEventColumns     = []string{"run_id", "seq", "event_id", "event_kind", "event_payload", "created_at"}
	workbenchArtifactColumns  = []string{"artifact_id", "run_id", "owner_user_id", "artifact_kind", "media_type", "storage_uri", "byte_size", "digest_sha256", "accepted_quote_id", "canonical_request_id", "canonical_usage_event_id", "ledger_journal_id", "canonical_media_business_event_id", "runner_job_id", "upstream_task_id", "created_at", "content_bytes"}
)

func (r *WorkbenchRuntimeRepository) LoadRun(ctx context.Context, identity workbench.Identity, runID workbench.RunID) (workbench.Run, error) {
	if err := identity.Validate(); err != nil {
		return workbench.Run{}, err
	}
	run, err := scanWorkbenchRun(r.db.QueryRowContext(ctx, workbenchSelectRunSQL, string(runID), int64(identity.ActorID)))
	if errors.Is(err, sql.ErrNoRows) {
		return workbench.Run{}, workbench.ErrRunNotFound
	}
	return run, err
}

func (r *WorkbenchRuntimeRepository) ReadEvents(ctx context.Context, identity workbench.Identity, runID workbench.RunID, cursor workbench.StreamCursor, limit uint32) (workbench.EventStreamResult, error) {
	if cursor.RunID != "" && cursor.RunID != runID {
		return workbench.EventStreamResult{}, workbench.ErrCursorRunMismatch
	}
	run, err := r.LoadRun(ctx, identity, runID)
	if err != nil {
		return workbench.EventStreamResult{}, err
	}
	if limit == 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, workbenchSelectEventsSQL, string(runID), int64(identity.ActorID), cursor.Seq, int(limit))
	if err != nil {
		return workbench.EventStreamResult{}, fmt.Errorf("read workbench events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]workbench.Event, 0)
	for rows.Next() {
		event, scanErr := scanWorkbenchEvent(rows)
		if scanErr != nil {
			return workbench.EventStreamResult{}, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return workbench.EventStreamResult{}, fmt.Errorf("iterate workbench events: %w", err)
	}
	return workbench.ReadEventStream(workbench.EventRead{Run: run, Events: events, Cursor: cursor, OldestAvailableSeq: 1})
}

func (r *WorkbenchRuntimeRepository) StoreArtifact(ctx context.Context, identity workbench.Identity, artifact workbench.Artifact) (workbench.Artifact, error) {
	if err := validateWorkbenchArtifact(identity, artifact); err != nil {
		return workbench.Artifact{}, err
	}
	stored, err := scanWorkbenchArtifact(r.db.QueryRowContext(ctx, workbenchInsertArtifactSQL, workbenchArtifactArgs(artifact)...))
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return workbench.Artifact{}, fmt.Errorf("store workbench artifact: %w", err)
	}
	existing, loadErr := r.LoadArtifact(ctx, identity, artifact.RunID, artifact.ArtifactID)
	if loadErr != nil {
		return workbench.Artifact{}, loadErr
	}
	if existing.ArtifactDigest != artifact.ArtifactDigest || existing.ByteSize != artifact.ByteSize || existing.StorageURI != artifact.StorageURI {
		return workbench.Artifact{}, workbench.ErrIdempotencyConflict
	}
	return existing, nil
}

func (r *WorkbenchRuntimeRepository) LoadArtifact(ctx context.Context, identity workbench.Identity, runID workbench.RunID, artifactID workbench.ArtifactID) (workbench.Artifact, error) {
	if err := identity.Validate(); err != nil {
		return workbench.Artifact{}, err
	}
	artifact, err := scanWorkbenchArtifact(r.db.QueryRowContext(ctx, workbenchSelectArtifactSQL, string(runID), int64(identity.ActorID), string(artifactID)))
	if errors.Is(err, sql.ErrNoRows) {
		return workbench.Artifact{}, workbench.ErrArtifactNotFound
	}
	if err == nil && (artifact.Body == nil || validateWorkbenchArtifact(identity, artifact) != nil) {
		return workbench.Artifact{}, workbench.ErrArtifactNotFound
	}
	return artifact, err
}

func (r *WorkbenchRuntimeRepository) StoreSnapshot(ctx context.Context, identity workbench.Identity, snapshot workbench.SavedSnapshot) (workbench.SavedSnapshot, error) {
	if err := validateWorkbenchSnapshot(identity, snapshot); err != nil {
		return workbench.SavedSnapshot{}, err
	}
	payload, err := marshalWorkbenchJSON(snapshot)
	if err != nil {
		return workbench.SavedSnapshot{}, err
	}
	stored, err := scanWorkbenchSnapshot(r.db.QueryRowContext(ctx, workbenchInsertSnapshotSQL, string(snapshot.ID), string(snapshot.RunID), int64(snapshot.OwnerID), payload, workbenchPayloadDigest(payload), snapshot.CreatedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return workbench.SavedSnapshot{}, workbench.ErrSnapshotNotFound
	}
	return stored, err
}

func (r *WorkbenchRuntimeRepository) LoadSnapshot(ctx context.Context, identity workbench.Identity, runID workbench.RunID, snapshotID workbench.SnapshotID) (workbench.SavedSnapshot, error) {
	if err := identity.Validate(); err != nil {
		return workbench.SavedSnapshot{}, err
	}
	snapshot, err := scanWorkbenchSnapshot(r.db.QueryRowContext(ctx, workbenchSelectSnapshotSQL, string(snapshotID), string(runID), int64(identity.ActorID)))
	if errors.Is(err, sql.ErrNoRows) {
		return workbench.SavedSnapshot{}, workbench.ErrSnapshotNotFound
	}
	return snapshot, err
}

func (r *WorkbenchRuntimeRepository) ResolveCanonicalLineage(ctx context.Context, identity workbench.Identity, runID workbench.RunID) (WorkbenchCanonicalLineage, error) {
	if err := identity.Validate(); err != nil {
		return WorkbenchCanonicalLineage{}, err
	}
	var lineage WorkbenchCanonicalLineage
	err := r.db.QueryRowContext(ctx, workbenchSelectCanonicalLineageSQL, string(runID), int64(identity.ActorID)).Scan(&lineage.UsageEventID, &lineage.RequestID, &lineage.PayloadSHA256, &lineage.JournalID)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkbenchCanonicalLineage{}, workbench.ErrRunNotFound
	}
	if err != nil {
		return WorkbenchCanonicalLineage{}, fmt.Errorf("resolve workbench canonical lineage: %w", err)
	}
	return lineage, nil
}

func scanWorkbenchRun(row scanner) (workbench.Run, error) {
	var run workbench.Run
	var capability string
	var input []byte
	var replay, fork, quote, requestID, usageID, journalID, mediaID, estimate, actual, errorCode sql.NullString
	if err := row.Scan(&run.ID, &run.OwnerID, &run.WorkspaceID, &capability, &input, &run.InputSHA256, &run.State,
		&run.Version, &run.NextEventSeq, &replay, &fork, &quote, &requestID, &usageID, &journalID, &mediaID,
		&estimate, &actual, &errorCode, &run.CreatedAt, &run.UpdatedAt, &run.TerminalAt); err != nil {
		return workbench.Run{}, err
	}
	if err := json.Unmarshal(input, &run.Input); err != nil {
		return workbench.Run{}, fmt.Errorf("decode workbench input snapshot: %w", err)
	}
	if run.Input.CapabilityID == "" {
		run.Input.CapabilityID = workbench.CapabilityID(capability)
	}
	run.ReplayOfRunID, run.ForkedFromRunID = workbench.RunID(replay.String), workbench.RunID(fork.String)
	run.AcceptedQuoteID, run.CanonicalRequestID = workbench.QuoteID(quote.String), requestID.String
	run.CanonicalUsageEventID, run.JournalID, run.CanonicalMediaBusinessEventID = usageID.String, journalID.String, mediaID.String
	run.EstimatedCost, run.ActualCost = parseWorkbenchMoney(estimate), parseWorkbenchMoney(actual)
	if errorCode.Valid {
		run.Failure = &workbench.RunFailure{Code: errorCode.String}
	}
	run.Artifacts = []workbench.Artifact{}
	return run, nil
}

func scanWorkbenchOperation(row scanner) (workbench.OperationRecord, error) {
	var record workbench.OperationRecord
	var resultRun, resultSnapshot, resultState sql.NullString
	err := row.Scan(&record.Claim.ID, &record.Claim.RunID, &record.Claim.OwnerID, &record.Claim.Key,
		&record.Claim.Kind, &record.Claim.Fingerprint, &record.State, &resultRun, &resultSnapshot, &resultState)
	record.Result = workbench.OperationResult{RunID: workbench.RunID(resultRun.String), SnapshotID: workbench.SnapshotID(resultSnapshot.String), State: workbench.RunState(resultState.String)}
	return record, err
}

func scanWorkbenchEvent(row scanner) (workbench.Event, error) {
	var event workbench.Event
	var payload []byte
	if err := row.Scan(&event.RunID, &event.Seq, &event.ID, &event.Kind, &payload, &event.CreatedAt); err != nil {
		return workbench.Event{}, err
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return workbench.Event{}, fmt.Errorf("decode workbench event: %w", err)
	}
	return event, nil
}

func scanWorkbenchArtifact(row scanner) (workbench.Artifact, error) {
	var artifact workbench.Artifact
	var quote, requestID, usageID, journalID, mediaID, runnerID, upstreamID sql.NullString
	err := row.Scan(&artifact.ArtifactID, &artifact.RunID, &artifact.OwnerID, &artifact.Kind, &artifact.ContentType,
		&artifact.StorageURI, &artifact.ByteSize, &artifact.ArtifactDigest, &quote, &requestID, &usageID,
		&journalID, &mediaID, &runnerID, &upstreamID, &artifact.CreatedAt, &artifact.Body)
	artifact.AcceptedQuoteID, artifact.CanonicalRequestID = workbench.QuoteID(quote.String), requestID.String
	artifact.CanonicalUsageEventID, artifact.JournalID = usageID.String, journalID.String
	artifact.CanonicalMediaBusinessEventID, artifact.RunnerJobID, artifact.UpstreamTaskID = mediaID.String, runnerID.String, upstreamID.String
	return artifact, err
}

func scanWorkbenchSnapshot(row scanner) (workbench.SavedSnapshot, error) {
	var snapshot workbench.SavedSnapshot
	var payload []byte
	if err := row.Scan(&snapshot.ID, &snapshot.RunID, &snapshot.OwnerID, &payload, &snapshot.InputSHA256, &snapshot.CreatedAt); err != nil {
		return workbench.SavedSnapshot{}, err
	}
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return workbench.SavedSnapshot{}, fmt.Errorf("decode workbench saved snapshot: %w", err)
	}
	return snapshot, nil
}

func parseWorkbenchMoney(raw sql.NullString) *workbench.Money {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	amount, err := workbench.NewDecimal(raw.String)
	if err != nil {
		return nil
	}
	return &workbench.Money{Amount: amount}
}

func marshalWorkbenchJSON(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode workbench payload: %w", err)
	}
	return payload, nil
}

func workbenchPayloadDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func validateWorkbenchArtifact(identity workbench.Identity, artifact workbench.Artifact) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if artifact.OwnerID != identity.ActorID || artifact.ArtifactID == "" || artifact.RunID == "" || !strings.HasPrefix(artifact.StorageURI, "workbench://") {
		return workbench.ErrOwnershipMismatch
	}
	if artifact.ByteSize != int64(len(artifact.Body)) || len(artifact.Body) > 4<<20 || string(artifact.ArtifactDigest) != workbenchPayloadDigest(artifact.Body) {
		return fmt.Errorf("%w: artifact size or digest mismatch", workbench.ErrInvalidCommand)
	}
	return nil
}

func validateWorkbenchSnapshot(identity workbench.Identity, snapshot workbench.SavedSnapshot) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if snapshot.OwnerID != identity.ActorID {
		return workbench.ErrOwnershipMismatch
	}
	if snapshot.ID == "" || snapshot.RunID == "" || snapshot.WorkspaceID == "" || snapshot.InputSHA256 == "" || snapshot.CreatedAt.IsZero() {
		return fmt.Errorf("%w: incomplete saved snapshot", workbench.ErrInvalidCommand)
	}
	return nil
}

func workbenchArtifactArgs(artifact workbench.Artifact) []any {
	return []any{string(artifact.ArtifactID), string(artifact.RunID), int64(artifact.OwnerID), string(artifact.Kind), artifact.ContentType,
		artifact.StorageURI, artifact.ByteSize, string(artifact.ArtifactDigest), string(artifact.AcceptedQuoteID), artifact.CanonicalRequestID,
		artifact.CanonicalUsageEventID, artifact.JournalID, artifact.CanonicalMediaBusinessEventID, artifact.RunnerJobID, artifact.UpstreamTaskID, artifact.CreatedAt, artifact.Body}
}
