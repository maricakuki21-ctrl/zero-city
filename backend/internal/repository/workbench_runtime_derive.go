package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *workbenchRuntimeStore) RequestCancel(ctx context.Context, request service.WorkbenchCancelRequest) (service.WorkbenchOperationResult, error) {
	if prior, err := s.replayStoredOperation(ctx, request.Identity, request.Claim); err != nil {
		return service.WorkbenchOperationResult{}, err
	} else if prior != nil {
		return service.WorkbenchOperationResult{Run: prior.Run, Replayed: true}, nil
	}
	current, err := s.repo.LoadRun(ctx, request.Identity, request.RunID)
	if err != nil {
		return service.WorkbenchOperationResult{}, err
	}
	now := time.Now().UTC()
	next, err := workbench.ApplyTransition(current, workbench.Transition{To: workbench.StateCancelRequested, At: now})
	if err != nil {
		// The first read can race the transaction that committed this same key.
		if prior, lookupErr := s.replayStoredOperation(ctx, request.Identity, request.Claim); lookupErr != nil {
			return service.WorkbenchOperationResult{}, lookupErr
		} else if prior != nil {
			return service.WorkbenchOperationResult{Run: prior.Run, Replayed: true}, nil
		}
		return service.WorkbenchOperationResult{}, err
	}
	event := workbench.Event{
		ID: workbench.EventID(workbenchNewID("wbe_")), RunID: next.ID, Kind: workbench.EventCancelRequested,
		State: workbench.StateCancelRequested, CreatedAt: now,
	}
	outcome, err := s.repo.ApplyOperation(ctx, WorkbenchOperationRequest{
		Identity: request.Identity, Claim: request.Claim, ExpectedVersion: current.Version, Run: next, Event: event,
	})
	if err != nil {
		return service.WorkbenchOperationResult{}, err
	}
	workbench.NotifyRun(outcome.Run.ID)
	return service.WorkbenchOperationResult{Run: outcome.Run, Replayed: outcome.Replayed}, nil
}

func (s *workbenchRuntimeStore) SaveSnapshot(ctx context.Context, request service.WorkbenchSaveRequest) (workbench.SaveResult, error) {
	if prior, err := s.replayStoredOperation(ctx, request.Identity, request.Claim); err != nil {
		return workbench.SaveResult{}, err
	} else if prior != nil {
		snapshot, err := s.repo.LoadSnapshot(ctx, request.Identity, prior.Run.ID, prior.Operation.Result.SnapshotID)
		return workbench.SaveResult{Run: prior.Run, Snapshot: snapshot, Replayed: true}, err
	}
	current, err := s.repo.LoadRun(ctx, request.Identity, request.RunID)
	if err != nil {
		return workbench.SaveResult{}, err
	}
	now := time.Now().UTC()
	snapshot := workbench.SavedSnapshot{
		ID: request.SnapshotID, RunID: current.ID, OwnerID: current.OwnerID, WorkspaceID: current.WorkspaceID,
		Label: request.Label, Input: current.Input, InputSHA256: current.InputSHA256, CreatedAt: now,
	}
	event := workbench.Event{
		ID: workbench.EventID(workbenchNewID("wbe_")), RunID: current.ID, Kind: workbench.EventSnapshotSaved,
		State: current.State, SnapshotID: snapshot.ID, CreatedAt: now,
	}
	outcome, err := s.repo.ApplyOperation(ctx, WorkbenchOperationRequest{
		Identity: request.Identity, Claim: request.Claim, ExpectedVersion: current.Version, Run: current, Event: event,
		Effects: WorkbenchOperationEffects{Snapshot: &snapshot},
	})
	if err != nil {
		return workbench.SaveResult{}, err
	}
	if outcome.Replayed {
		snapshot, err = s.repo.LoadSnapshot(ctx, request.Identity, outcome.Run.ID, outcome.Operation.Result.SnapshotID)
		if err != nil {
			return workbench.SaveResult{}, err
		}
	}
	workbench.NotifyRun(outcome.Run.ID)
	return workbench.SaveResult{Run: outcome.Run, Snapshot: snapshot, Replayed: outcome.Replayed}, nil
}

func (s *workbenchRuntimeStore) DeriveRun(ctx context.Context, request service.WorkbenchDeriveRequest) (service.WorkbenchDeriveResult, error) {
	if err := request.Identity.Validate(); err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	now := time.Now().UTC()
	switch request.Kind {
	case workbench.DerivationReplay:
		return s.deriveReplay(ctx, request, now)
	case workbench.DerivationFork:
		return s.deriveFork(ctx, request, now)
	default:
		return service.WorkbenchDeriveResult{}, fmt.Errorf("%w: unknown derivation kind %q", workbench.ErrInvalidCommand, request.Kind)
	}
}

func (s *workbenchRuntimeStore) deriveReplay(ctx context.Context, request service.WorkbenchDeriveRequest, now time.Time) (service.WorkbenchDeriveResult, error) {
	snapshot, err := s.repo.LoadSnapshotByID(ctx, request.Identity, request.SourceSnapshotID)
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	source, err := s.repo.LoadRun(ctx, request.Identity, snapshot.RunID)
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	request.Claim.RunID = source.ID
	if prior, err := s.replayStoredOperation(ctx, request.Identity, request.Claim); err != nil {
		return service.WorkbenchDeriveResult{}, err
	} else if prior != nil {
		return service.WorkbenchDeriveResult{Run: prior.Run, SourceSnapshotID: prior.Operation.Result.SnapshotID, Replayed: true}, nil
	}
	derived, err := workbench.DeriveReplay(source, snapshot, workbench.DerivationRequest{Identity: request.Identity, NewRunID: request.NewRunID, At: now})
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	return s.insertDerivedRun(ctx, request, source, snapshot.ID, derived.Run, workbench.DerivationReplay, workbench.EventRunReplayed, now)
}

func (s *workbenchRuntimeStore) deriveFork(ctx context.Context, request service.WorkbenchDeriveRequest, now time.Time) (service.WorkbenchDeriveResult, error) {
	source, err := s.repo.LoadRun(ctx, request.Identity, request.SourceRunID)
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	request.Claim.RunID = source.ID
	if prior, err := s.replayStoredOperation(ctx, request.Identity, request.Claim); err != nil {
		return service.WorkbenchDeriveResult{}, err
	} else if prior != nil {
		return service.WorkbenchDeriveResult{Run: prior.Run, SourceSnapshotID: prior.Operation.Result.SnapshotID, Replayed: true}, nil
	}
	if source.State != workbench.StateSucceeded {
		return service.WorkbenchDeriveResult{}, workbench.ErrInvalidTransition
	}
	snapshot, err := s.ensureForkSnapshot(ctx, request.Identity, source, request.Claim, now)
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	derived, err := workbench.DeriveFork(source, snapshot, workbench.DerivationRequest{Identity: request.Identity, NewRunID: request.NewRunID, At: now})
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	return s.insertDerivedRun(ctx, request, source, snapshot.ID, derived.Run, workbench.DerivationFork, workbench.EventRunForked, now)
}

func (s *workbenchRuntimeStore) ensureForkSnapshot(ctx context.Context, identity workbench.Identity, source workbench.Run, claim workbench.OperationClaim, now time.Time) (workbench.SavedSnapshot, error) {
	if existing, err := s.loadForkSnapshot(ctx, identity, source); !errors.Is(err, workbench.ErrSnapshotNotFound) {
		return existing, err
	}
	// Concurrent first requests share one snapshot even before either operation
	// commits. Sequential retries have already returned from the operation lookup.
	key, err := marshalWorkbenchJSON([]any{identity.ActorID, source.ID, claim.Key, claim.Kind})
	if err != nil {
		return workbench.SavedSnapshot{}, err
	}
	snapshot := workbench.SavedSnapshot{
		ID: workbench.SnapshotID("wbs_" + workbenchPayloadDigest(key)[:32]), RunID: source.ID, OwnerID: source.OwnerID, WorkspaceID: source.WorkspaceID,
		Label: "fork", Input: source.Input, InputSHA256: source.InputSHA256, CreatedAt: now,
	}
	stored, err := s.repo.StoreSnapshot(ctx, identity, snapshot)
	if !errors.Is(err, workbench.ErrSnapshotNotFound) {
		return stored, err
	}
	// A concurrent request may already have persisted the deterministic ID.
	// Return a committed matching-input snapshot instead of the proposed ID.
	return s.loadForkSnapshot(ctx, identity, source)
}

const workbenchSelectForkSnapshotSQL = `SELECT snapshot.snapshot_id,snapshot.run_id,snapshot.owner_user_id,
	snapshot.snapshot,snapshot.snapshot_sha256,snapshot.created_at
	FROM workbench_saved_snapshots snapshot JOIN workbench_runs run ON run.run_id=snapshot.run_id
	WHERE snapshot.run_id=$1 AND snapshot.snapshot->>'input_sha256'=$2
	AND snapshot.owner_user_id=$3 AND run.owner_user_id=$3
	ORDER BY snapshot.created_at,snapshot.snapshot_id LIMIT 1`

func (s *workbenchRuntimeStore) loadForkSnapshot(ctx context.Context, identity workbench.Identity, source workbench.Run) (workbench.SavedSnapshot, error) {
	snapshot, err := scanWorkbenchSnapshot(s.repo.db.QueryRowContext(ctx, workbenchSelectForkSnapshotSQL,
		string(source.ID), string(source.InputSHA256), int64(identity.ActorID)))
	if errors.Is(err, sql.ErrNoRows) {
		return workbench.SavedSnapshot{}, workbench.ErrSnapshotNotFound
	}
	if err == nil && (snapshot.InputSHA256 != source.InputSHA256 || snapshot.Input != source.Input) {
		return workbench.SavedSnapshot{}, workbench.ErrIdempotencyConflict
	}
	return snapshot, err
}

func (s *workbenchRuntimeStore) insertDerivedRun(
	ctx context.Context,
	request service.WorkbenchDeriveRequest,
	source workbench.Run,
	snapshotID workbench.SnapshotID,
	derived workbench.Run,
	kind workbench.DerivationKind,
	eventKind workbench.EventKind,
	now time.Time,
) (service.WorkbenchDeriveResult, error) {
	claim := request.Claim
	claim.RunID = source.ID
	if derived.Input.CapabilityVersion == service.WorkbenchNativeVersion {
		derived.AcceptedQuoteID, derived.AcceptedQuoteSHA = "", ""
		derived.CanonicalUsageEventID, derived.JournalID = "", ""
	}
	outcome, err := s.repo.DeriveRun(ctx, WorkbenchDerivationRequest{
		Identity: request.Identity,
		Claim:    claim,
		Run:      derived,
		Derivation: workbench.Derivation{
			ID: workbenchNewID("wbd_"), OwnerID: request.Identity.ActorID, SourceRunID: source.ID,
			SourceSnapshotID: snapshotID, DerivedRunID: derived.ID, Kind: kind, CreatedAt: now,
		},
		Event: workbench.Event{
			ID: workbench.EventID(workbenchNewID("wbe_")), RunID: derived.ID, Kind: eventKind,
			State: workbench.StateQueued, SourceRunID: source.ID, CreatedAt: now,
		},
	})
	if err != nil {
		return service.WorkbenchDeriveResult{}, err
	}
	workbench.NotifyRun(outcome.Run.ID)
	return service.WorkbenchDeriveResult{Run: outcome.Run, SourceSnapshotID: outcome.Operation.Result.SnapshotID, Replayed: outcome.Replayed}, nil
}

func (s *workbenchRuntimeStore) replayStoredOperation(ctx context.Context, identity workbench.Identity, claim workbench.OperationClaim) (*WorkbenchOperationOutcome, error) {
	if err := validateWorkbenchClaim(identity, claim); err != nil {
		return nil, err
	}
	existing, err := scanWorkbenchOperation(s.repo.db.QueryRowContext(ctx, workbenchSelectOperationSQL,
		string(claim.RunID), claim.Key, int64(identity.ActorID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := workbench.ResolveIdempotency(&existing, claim); err != nil {
		return nil, err
	}
	if existing.State != workbench.OperationCommitted || existing.Result.RunID == "" {
		return nil, workbench.ErrInvalidCommand
	}
	run, err := s.LoadRun(ctx, identity, existing.Result.RunID)
	if err != nil {
		return nil, err
	}
	return &WorkbenchOperationOutcome{Run: run, Operation: existing, Replayed: true}, nil
}
