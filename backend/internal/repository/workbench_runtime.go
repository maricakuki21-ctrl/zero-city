package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

var ErrWorkbenchVersionConflict = errors.New("workbench run version conflict")

type WorkbenchRuntimeRepository struct{ db *sql.DB }

func NewWorkbenchRuntimeRepository(db *sql.DB) *WorkbenchRuntimeRepository {
	return &WorkbenchRuntimeRepository{db: db}
}

type WorkbenchCanonicalLineage struct {
	UsageEventID  string
	RequestID     string
	PayloadSHA256 string
	JournalID     string
}

type WorkbenchOperationEffects struct {
	Artifact *workbench.Artifact
	Snapshot *workbench.SavedSnapshot
}

type WorkbenchOperationRequest struct {
	Identity        workbench.Identity
	Claim           workbench.OperationClaim
	ExpectedVersion uint64
	Run             workbench.Run
	Event           workbench.Event
	Effects         WorkbenchOperationEffects
}

type WorkbenchDerivationRequest struct {
	Identity   workbench.Identity
	Claim      workbench.OperationClaim
	Run        workbench.Run
	Derivation workbench.Derivation
	Event      workbench.Event
}

type WorkbenchCreateRunRequest struct {
	Identity workbench.Identity
	Claim    workbench.OperationClaim
	Run      workbench.Run
	Event    workbench.Event
}

type WorkbenchOperationOutcome struct {
	Run       workbench.Run
	Event     workbench.Event
	Operation workbench.OperationRecord
	Replayed  bool
}

func (r *WorkbenchRuntimeRepository) CreateRun(ctx context.Context, request WorkbenchCreateRunRequest) (WorkbenchOperationOutcome, error) {
	if err := validateWorkbenchClaim(request.Identity, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if request.Claim.RunID != request.Run.ID || request.Run.OwnerID != request.Identity.ActorID {
		return WorkbenchOperationOutcome{}, workbench.ErrOwnershipMismatch
	}
	if err := validateWorkbenchEvent(request.Event, request.Run.ID); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	tx, err := r.beginWorkbenchTx(ctx)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockWorkbenchOperation(ctx, tx, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if existing, err := selectWorkbenchOperation(ctx, tx, request.Claim); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return WorkbenchOperationOutcome{}, err
	} else if err == nil {
		return replayExistingWorkbenchOperation(ctx, tx, existing, request.Claim)
	}
	run := request.Run.Clone()
	if run.Version == 0 {
		run.Version = 1
	}
	if run.NextEventSeq == 0 {
		run.NextEventSeq = 1
	}
	event := request.Event
	event.Seq = run.NextEventSeq
	run.NextEventSeq++
	stored, err := insertWorkbenchRun(ctx, tx, run)
	if err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("insert workbench run: %w", err)
	}
	if err := insertWorkbenchEvent(ctx, tx, event); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	result := workbench.OperationResult{RunID: stored.ID, State: stored.State}
	operation, err := insertCommittedWorkbenchOperation(ctx, tx, request.Claim, result)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("commit workbench run: %w", err)
	}
	return WorkbenchOperationOutcome{Run: stored, Event: event, Operation: operation}, nil
}

func (r *WorkbenchRuntimeRepository) ApplyOperation(ctx context.Context, request WorkbenchOperationRequest) (WorkbenchOperationOutcome, error) {
	if err := validateWorkbenchClaim(request.Identity, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if request.Claim.RunID != request.Run.ID || request.Run.OwnerID != request.Identity.ActorID {
		return WorkbenchOperationOutcome{}, workbench.ErrOwnershipMismatch
	}
	tx, err := r.beginWorkbenchTx(ctx)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockWorkbenchOperation(ctx, tx, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	existing, lookupErr := selectWorkbenchOperation(ctx, tx, request.Claim)
	if lookupErr == nil {
		return replayExistingWorkbenchOperation(ctx, tx, existing, request.Claim)
	}
	if !errors.Is(lookupErr, sql.ErrNoRows) {
		return WorkbenchOperationOutcome{}, lookupErr
	}
	current, err := selectWorkbenchRunForUpdate(ctx, tx, request.Run.ID, request.Identity.ActorID)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkbenchOperationOutcome{}, workbench.ErrRunNotFound
	}
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if current.Version != request.ExpectedVersion {
		return WorkbenchOperationOutcome{}, ErrWorkbenchVersionConflict
	}
	if err := validateWorkbenchRunMutation(current, request.Run); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if err := validateWorkbenchEvent(request.Event, current.ID); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if _, err := claimWorkbenchOperation(ctx, tx, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if request.Effects.Artifact != nil {
		if _, err := insertWorkbenchArtifact(ctx, tx, *request.Effects.Artifact); err != nil {
			return WorkbenchOperationOutcome{}, err
		}
	}
	if request.Effects.Snapshot != nil {
		if _, err := insertWorkbenchSnapshot(ctx, tx, *request.Effects.Snapshot); err != nil {
			return WorkbenchOperationOutcome{}, err
		}
	}
	next := request.Run.Clone()
	next.Version = current.Version + 1
	next.NextEventSeq = current.NextEventSeq + 1
	updated, err := updateWorkbenchRun(ctx, tx, current, next)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	event := request.Event
	event.Seq = current.NextEventSeq
	if err := insertWorkbenchEvent(ctx, tx, event); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	result := workbench.OperationResult{RunID: updated.ID, State: updated.State}
	if request.Effects.Snapshot != nil {
		result.SnapshotID = request.Effects.Snapshot.ID
	}
	operation, err := commitWorkbenchOperation(ctx, tx, request.Claim, result)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("commit workbench operation: %w", err)
	}
	return WorkbenchOperationOutcome{Run: updated, Event: event, Operation: operation}, nil
}

func (r *WorkbenchRuntimeRepository) DeriveRun(ctx context.Context, request WorkbenchDerivationRequest) (WorkbenchOperationOutcome, error) {
	if err := validateWorkbenchClaim(request.Identity, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if request.Claim.RunID != request.Derivation.SourceRunID || request.Run.OwnerID != request.Identity.ActorID || request.Derivation.OwnerID != request.Identity.ActorID || request.Derivation.DerivedRunID != request.Run.ID {
		return WorkbenchOperationOutcome{}, workbench.ErrOwnershipMismatch
	}
	tx, err := r.beginWorkbenchTx(ctx)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockWorkbenchOperation(ctx, tx, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	existing, lookupErr := selectWorkbenchOperation(ctx, tx, request.Claim)
	if lookupErr == nil {
		return replayExistingWorkbenchOperation(ctx, tx, existing, request.Claim)
	}
	if !errors.Is(lookupErr, sql.ErrNoRows) {
		return WorkbenchOperationOutcome{}, lookupErr
	}
	if _, err := selectWorkbenchRunForUpdate(ctx, tx, request.Derivation.SourceRunID, request.Identity.ActorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkbenchOperationOutcome{}, workbench.ErrRunNotFound
		}
		return WorkbenchOperationOutcome{}, err
	}
	if _, err := selectWorkbenchSnapshot(ctx, tx, request.Derivation.SourceSnapshotID, request.Derivation.SourceRunID, request.Identity.ActorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkbenchOperationOutcome{}, workbench.ErrSnapshotNotFound
		}
		return WorkbenchOperationOutcome{}, err
	}
	if _, err := claimWorkbenchOperation(ctx, tx, request.Claim); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	derived := request.Run.Clone()
	if derived.Version == 0 {
		derived.Version = 1
	}
	if derived.NextEventSeq == 0 {
		derived.NextEventSeq = 1
	}
	var event *workbench.Event
	if request.Event.ID != "" {
		if err := validateWorkbenchEvent(request.Event, derived.ID); err != nil {
			return WorkbenchOperationOutcome{}, err
		}
		e := request.Event
		e.Seq = derived.NextEventSeq
		derived.NextEventSeq++
		event = &e
	}
	stored, err := insertWorkbenchRun(ctx, tx, derived)
	if err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("insert derived workbench run: %w", err)
	}
	if event != nil {
		if err := insertWorkbenchEvent(ctx, tx, *event); err != nil {
			return WorkbenchOperationOutcome{}, err
		}
	}
	if _, err := insertWorkbenchDerivation(ctx, tx, request.Derivation); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	result := workbench.OperationResult{RunID: stored.ID, State: stored.State, SnapshotID: request.Derivation.SourceSnapshotID}
	operation, err := commitWorkbenchOperation(ctx, tx, request.Claim, result)
	if err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("commit derived workbench run: %w", err)
	}
	out := WorkbenchOperationOutcome{Run: stored, Operation: operation}
	if event != nil {
		out.Event = *event
	}
	return out, nil
}

func (r *WorkbenchRuntimeRepository) beginWorkbenchTx(ctx context.Context) (*sql.Tx, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("workbench runtime repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("begin workbench transaction: %w", err)
	}
	return tx, nil
}

func validateWorkbenchClaim(identity workbench.Identity, claim workbench.OperationClaim) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if claim.OwnerID != identity.ActorID || claim.ID == "" || claim.RunID == "" || claim.Key == "" || claim.Kind == "" || len(claim.Fingerprint) != 64 {
		return fmt.Errorf("%w: invalid operation claim", workbench.ErrInvalidCommand)
	}
	return nil
}

func validateWorkbenchEvent(event workbench.Event, runID workbench.RunID) error {
	if event.ID == "" || event.RunID != runID || event.Kind == "" || event.CreatedAt.IsZero() {
		return fmt.Errorf("%w: invalid event", workbench.ErrInvalidCommand)
	}
	return nil
}

func validateWorkbenchRunMutation(current, next workbench.Run) error {
	if current.ID != next.ID || current.OwnerID != next.OwnerID || current.WorkspaceID != next.WorkspaceID || current.InputSHA256 != next.InputSHA256 || current.Input != next.Input {
		return fmt.Errorf("%w: immutable run identity changed", workbench.ErrInvalidCommand)
	}
	if current.ReplayOfRunID != next.ReplayOfRunID || current.ForkedFromRunID != next.ForkedFromRunID || current.AcceptedQuoteID != next.AcceptedQuoteID {
		return fmt.Errorf("%w: immutable run lineage changed", workbench.ErrInvalidCommand)
	}
	if current.CanonicalRequestID != "" && current.CanonicalRequestID != next.CanonicalRequestID || current.CanonicalUsageEventID != "" && current.CanonicalUsageEventID != next.CanonicalUsageEventID || current.JournalID != "" && current.JournalID != next.JournalID || current.CanonicalMediaBusinessEventID != "" && current.CanonicalMediaBusinessEventID != next.CanonicalMediaBusinessEventID {
		return fmt.Errorf("%w: canonical lineage is write-once", workbench.ErrInvalidCommand)
	}
	return nil
}

func workbenchOperationLockKey(claim workbench.OperationClaim) string {
	return strings.Join([]string{fmt.Sprint(claim.OwnerID), string(claim.RunID), string(claim.Kind), claim.Key}, "|")
}

func lockWorkbenchOperation(ctx context.Context, tx *sql.Tx, claim workbench.OperationClaim) error {
	if _, err := tx.ExecContext(ctx, workbenchAdvisoryLockSQL, workbenchOperationLockKey(claim)); err != nil {
		return fmt.Errorf("lock workbench operation: %w", err)
	}
	return nil
}

func selectWorkbenchOperation(ctx context.Context, tx *sql.Tx, claim workbench.OperationClaim) (workbench.OperationRecord, error) {
	return scanWorkbenchOperation(tx.QueryRowContext(ctx, workbenchSelectOperationSQL, string(claim.RunID), claim.Key, int64(claim.OwnerID)))
}

func claimWorkbenchOperation(ctx context.Context, tx *sql.Tx, claim workbench.OperationClaim) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, workbenchClaimOperationSQL, string(claim.ID), string(claim.RunID), int64(claim.OwnerID), claim.Key, string(claim.Kind), string(claim.Fingerprint)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("claim workbench operation: %w", err)
	}
	return id, nil
}

func insertCommittedWorkbenchOperation(ctx context.Context, tx *sql.Tx, claim workbench.OperationClaim, result workbench.OperationResult) (workbench.OperationRecord, error) {
	payload, err := marshalWorkbenchJSON(result)
	if err != nil {
		return workbench.OperationRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, workbenchInsertCommittedOperationSQL, string(claim.ID), string(claim.RunID), int64(claim.OwnerID), claim.Key, string(claim.Kind), string(claim.Fingerprint), string(result.RunID), string(result.SnapshotID), string(result.State), payload); err != nil {
		return workbench.OperationRecord{}, fmt.Errorf("insert committed workbench operation: %w", err)
	}
	return workbench.OperationRecord{Claim: claim, State: workbench.OperationCommitted, Result: result}, nil
}

func commitWorkbenchOperation(ctx context.Context, tx *sql.Tx, claim workbench.OperationClaim, result workbench.OperationResult) (workbench.OperationRecord, error) {
	payload, err := marshalWorkbenchJSON(result)
	if err != nil {
		return workbench.OperationRecord{}, err
	}
	res, err := tx.ExecContext(ctx, workbenchCommitOperationSQL, string(claim.ID), string(claim.RunID), int64(claim.OwnerID), string(result.RunID), string(result.SnapshotID), string(result.State), payload)
	if err != nil {
		return workbench.OperationRecord{}, fmt.Errorf("commit workbench operation: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil || rows != 1 {
		return workbench.OperationRecord{}, fmt.Errorf("commit workbench operation affected %d rows: %w", rows, err)
	}
	return workbench.OperationRecord{Claim: claim, State: workbench.OperationCommitted, Result: result}, nil
}

func replayExistingWorkbenchOperation(ctx context.Context, tx *sql.Tx, existing workbench.OperationRecord, requested workbench.OperationClaim) (WorkbenchOperationOutcome, error) {
	if _, err := workbench.ResolveIdempotency(&existing, requested); err != nil {
		return WorkbenchOperationOutcome{}, err
	}
	return replayWorkbenchOperation(ctx, tx, existing)
}

func replayWorkbenchOperation(ctx context.Context, tx *sql.Tx, existing workbench.OperationRecord) (WorkbenchOperationOutcome, error) {
	if existing.State != workbench.OperationCommitted || existing.Result.RunID == "" {
		return WorkbenchOperationOutcome{}, fmt.Errorf("%w: operation is not committed", workbench.ErrInvalidCommand)
	}
	run, err := scanWorkbenchRun(tx.QueryRowContext(ctx, workbenchSelectRunSQL, string(existing.Result.RunID), int64(existing.Claim.OwnerID)))
	if errors.Is(err, sql.ErrNoRows) {
		return WorkbenchOperationOutcome{}, workbench.ErrRunNotFound
	}
	if err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("load replayed workbench run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return WorkbenchOperationOutcome{}, fmt.Errorf("commit workbench replay: %w", err)
	}
	return WorkbenchOperationOutcome{Run: run, Operation: existing, Replayed: true}, nil
}

func insertWorkbenchRun(ctx context.Context, tx *sql.Tx, run workbench.Run) (workbench.Run, error) {
	input, err := marshalWorkbenchJSON(run.Input)
	if err != nil {
		return workbench.Run{}, err
	}
	return scanWorkbenchRun(tx.QueryRowContext(ctx, workbenchInsertRunSQL, workbenchRunArgs(run, input)...))
}

func selectWorkbenchRunForUpdate(ctx context.Context, tx *sql.Tx, runID workbench.RunID, owner workbench.ActorID) (workbench.Run, error) {
	return scanWorkbenchRun(tx.QueryRowContext(ctx, workbenchLockRunSQL, string(runID), int64(owner)))
}

func selectWorkbenchSnapshot(ctx context.Context, tx *sql.Tx, snapshotID workbench.SnapshotID, runID workbench.RunID, owner workbench.ActorID) (workbench.SavedSnapshot, error) {
	return scanWorkbenchSnapshot(tx.QueryRowContext(ctx, workbenchSelectSnapshotSQL, string(snapshotID), string(runID), int64(owner)))
}

func updateWorkbenchRun(ctx context.Context, tx *sql.Tx, current, next workbench.Run) (workbench.Run, error) {
	return scanWorkbenchRun(tx.QueryRowContext(ctx, workbenchUpdateRunSQL, workbenchUpdateArgs(current, next)...))
}

func insertWorkbenchEvent(ctx context.Context, tx *sql.Tx, event workbench.Event) error {
	payload, err := marshalWorkbenchJSON(event)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, workbenchInsertEventSQL, string(event.RunID), int64(event.Seq), string(event.ID), string(event.Kind), payload, workbenchPayloadDigest(payload), event.CreatedAt); err != nil {
		return fmt.Errorf("append workbench event: %w", err)
	}
	return nil
}

func insertWorkbenchArtifact(ctx context.Context, tx *sql.Tx, artifact workbench.Artifact) (workbench.Artifact, error) {
	if err := validateWorkbenchArtifact(workbench.Identity{ActorID: artifact.OwnerID}, artifact); err != nil {
		return workbench.Artifact{}, err
	}
	return scanWorkbenchArtifact(tx.QueryRowContext(ctx, workbenchInsertArtifactSQL, workbenchArtifactArgs(artifact)...))
}

func insertWorkbenchSnapshot(ctx context.Context, tx *sql.Tx, snapshot workbench.SavedSnapshot) (workbench.SavedSnapshot, error) {
	payload, err := marshalWorkbenchJSON(snapshot)
	if err != nil {
		return workbench.SavedSnapshot{}, err
	}
	return scanWorkbenchSnapshot(tx.QueryRowContext(ctx, workbenchInsertSnapshotSQL, string(snapshot.ID), string(snapshot.RunID), int64(snapshot.OwnerID), payload, workbenchPayloadDigest(payload), snapshot.CreatedAt))
}

func insertWorkbenchDerivation(ctx context.Context, tx *sql.Tx, derivation workbench.Derivation) (workbench.Derivation, error) {
	if _, err := tx.ExecContext(ctx, workbenchInsertDerivationSQL, derivation.ID, int64(derivation.OwnerID), string(derivation.SourceRunID), string(derivation.SourceSnapshotID), string(derivation.DerivedRunID), string(derivation.Kind), derivation.CreatedAt); err != nil {
		return workbench.Derivation{}, fmt.Errorf("insert workbench derivation: %w", err)
	}
	return derivation, nil
}

func workbenchRunArgs(run workbench.Run, input []byte) []any {
	return []any{string(run.ID), int64(run.OwnerID), string(run.WorkspaceID), string(run.Input.CapabilityID), input, string(run.InputSHA256), string(run.State), run.Version, run.NextEventSeq, string(run.ReplayOfRunID), string(run.ForkedFromRunID), string(run.AcceptedQuoteID), run.CanonicalRequestID, run.CanonicalUsageEventID, run.JournalID, run.CanonicalMediaBusinessEventID, decimalArg(run.EstimatedCost), decimalArg(run.ActualCost), failureCode(run.Failure), run.CreatedAt, run.UpdatedAt, run.TerminalAt}
}

func workbenchUpdateArgs(current, next workbench.Run) []any {
	return []any{string(current.ID), int64(current.OwnerID), string(next.State), next.CanonicalRequestID, next.CanonicalUsageEventID, next.JournalID, next.CanonicalMediaBusinessEventID, decimalArg(next.EstimatedCost), decimalArg(next.ActualCost), failureCode(next.Failure), next.UpdatedAt, next.TerminalAt, current.Version}
}

func decimalArg(money *workbench.Money) any {
	if money == nil {
		return nil
	}
	return string(money.Amount)
}

func failureCode(failure *workbench.RunFailure) any {
	if failure == nil {
		return nil
	}
	return failure.Code
}

var _ = json.Valid
