package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/google/uuid"
)

var ErrWorkbenchRuntimeUnavailable = errors.New("workbench runtime is unavailable")

type WorkbenchIDSource interface{ NewID(prefix string) string }
type randomWorkbenchIDs struct{}

func (randomWorkbenchIDs) NewID(prefix string) string {
	return prefix + strings.ReplaceAll(uuid.NewString(), "-", "")
}

type WorkbenchRuntimeStore interface {
	LoadWorkspace(context.Context, workbench.Identity) (workbench.Workspace, error)
	LoadRun(context.Context, workbench.Identity, workbench.RunID) (workbench.Run, error)
	ClaimOperation(context.Context, WorkbenchOperationRequest) (WorkbenchOperationResult, error)
	TransitionRun(context.Context, WorkbenchRunMutation) (workbench.Run, error)
	CompleteRun(context.Context, WorkbenchExecutionCommit) (workbench.Run, error)
	RequestCancel(context.Context, WorkbenchCancelRequest) (WorkbenchOperationResult, error)
	SaveSnapshot(context.Context, WorkbenchSaveRequest) (workbench.SaveResult, error)
	DeriveRun(context.Context, WorkbenchDeriveRequest) (WorkbenchDeriveResult, error)
	LoadArtifact(context.Context, workbench.Identity, workbench.RunID, workbench.ArtifactID) (workbench.Artifact, error)
	ReadEvents(context.Context, workbench.Identity, workbench.RunID, workbench.StreamCursor, uint32) (workbench.EventRead, error)
}

type WorkbenchCanonicalExecutor interface {
	Execute(context.Context, WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error)
	Cancel(context.Context, workbench.RunID) error
}

type WorkbenchOperationRequest struct {
	Identity workbench.Identity
	Claim    workbench.OperationClaim
	Run      workbench.Run
	Event    workbench.Event
}
type WorkbenchOperationResult struct {
	Run      workbench.Run
	Replayed bool
}
type WorkbenchRunMutation struct {
	Identity workbench.Identity
	Run      workbench.Run
	Event    workbench.Event
}
type WorkbenchExecutionCommit struct {
	Identity workbench.Identity
	Run      workbench.Run
	Artifact workbench.Artifact
	Event    workbench.Event
}
type WorkbenchCancelRequest struct {
	Identity workbench.Identity
	RunID    workbench.RunID
	Claim    workbench.OperationClaim
}
type WorkbenchSaveRequest struct {
	Identity   workbench.Identity
	RunID      workbench.RunID
	SnapshotID workbench.SnapshotID
	Label      string
	Claim      workbench.OperationClaim
}
type WorkbenchDeriveRequest struct {
	Identity         workbench.Identity
	Claim            workbench.OperationClaim
	SourceRunID      workbench.RunID
	SourceSnapshotID workbench.SnapshotID
	NewRunID         workbench.RunID
	Kind             workbench.DerivationKind
	Run              workbench.Run
}
type WorkbenchDeriveResult struct {
	Run              workbench.Run
	SourceSnapshotID workbench.SnapshotID
	Replayed         bool
}

type WorkbenchRuntimeDependencies struct {
	Store    WorkbenchRuntimeStore
	Catalog  *WorkbenchCatalog
	Executor WorkbenchCanonicalExecutor
	IDs      WorkbenchIDSource
	Clock    WorkbenchClock
}
type WorkbenchRuntime struct {
	store    WorkbenchRuntimeStore
	catalog  *WorkbenchCatalog
	executor WorkbenchCanonicalExecutor
	ids      WorkbenchIDSource
	clock    WorkbenchClock
	active   atomic.Int64
	cancelMu sync.Mutex
	cancels  map[workbench.RunID]context.CancelFunc
}

func NewWorkbenchRuntime(deps WorkbenchRuntimeDependencies) (*WorkbenchRuntime, error) {
	if deps.Store == nil || deps.Catalog == nil || deps.Executor == nil {
		return nil, ErrWorkbenchRuntimeUnavailable
	}
	if deps.IDs == nil {
		deps.IDs = randomWorkbenchIDs{}
	}
	if deps.Clock == nil {
		deps.Clock = systemWorkbenchClock{}
	}
	return &WorkbenchRuntime{store: deps.Store, catalog: deps.Catalog, executor: deps.Executor, ids: deps.IDs, clock: deps.Clock, cancels: make(map[workbench.RunID]context.CancelFunc)}, nil
}

func (r *WorkbenchRuntime) Workspace(ctx context.Context, identity workbench.Identity) (workbench.Workspace, error) {
	if err := identity.Validate(); err != nil {
		return workbench.Workspace{}, err
	}
	workspace, err := r.store.LoadWorkspace(ctx, identity)
	if err != nil {
		return workbench.Workspace{}, err
	}
	capabilities, err := r.catalog.Capabilities(ctx, identity)
	if err != nil {
		return workbench.Workspace{}, err
	}
	workspace.Capabilities = capabilities
	return workspace, nil
}

func (r *WorkbenchRuntime) Launch(ctx context.Context, command workbench.LaunchCommand) (workbench.LaunchResult, error) {
	if err := validateWorkbenchLaunchCommand(command); err != nil {
		return workbench.LaunchResult{}, err
	}
	runID := deterministicWorkbenchRunID(command.Identity, command.IdempotencyKey)
	// A retry reads its already-authorized run, even after the original quote
	// expires. Changed inputs must never reuse that authorization.
	existing, loadErr := r.store.LoadRun(ctx, command.Identity, runID)
	if loadErr == nil {
		expected := workbench.InputSnapshot{
			Intent: command.Intent, CapabilityID: workbench.CapabilityID(command.CapabilityID),
			CapabilityVersion: command.CapabilityVersion, CapabilityDigest: workbench.Digest(command.CapabilityDigest),
			CanonicalModelID: command.CanonicalModelID, CanonicalModelVersion: command.CanonicalModelVersion,
			AcceptedQuoteID: workbench.QuoteID(command.AcceptedQuoteID), AcceptedQuoteSHA: workbench.Digest(command.AcceptedQuoteSHA),
		}
		if existing.Input != expected {
			return workbench.LaunchResult{}, workbench.ErrIdempotencyConflict
		}
		return workbench.LaunchResult{Run: existing, Replayed: true}, nil
	}
	if !errors.Is(loadErr, workbench.ErrRunNotFound) {
		return workbench.LaunchResult{}, loadErr
	}
	resolved, err := r.catalog.ResolveLaunch(ctx, command)
	if err != nil {
		return workbench.LaunchResult{}, err
	}
	workspace, err := r.store.LoadWorkspace(ctx, command.Identity)
	if err != nil {
		return workbench.LaunchResult{}, err
	}
	if workspace.ID == "" {
		return workbench.LaunchResult{}, ErrWorkbenchRuntimeUnavailable
	}
	now := r.clock.Now()
	input := workbench.InputSnapshot{Intent: command.Intent, CapabilityID: resolved.Capability.ID, CapabilityVersion: resolved.Capability.Version, CapabilityDigest: resolved.Capability.Digest, CanonicalModelID: resolved.Capability.CanonicalModelID, CanonicalModelVersion: resolved.Capability.CanonicalModelVersion, AcceptedQuoteID: resolved.Quote.ID, AcceptedQuoteSHA: resolved.Quote.SHA256}
	inputSHA := sha256.Sum256(mustJSON(input))
	run := workbench.Run{ID: runID, OwnerID: command.Identity.ActorID, WorkspaceID: workspace.ID, State: workbench.StateQueued, Input: input, InputSHA256: workbench.Digest(hex.EncodeToString(inputSHA[:])), Version: 1, NextEventSeq: 1, EstimatedCost: cloneWorkbenchMoney(resolved.Quote.Estimate), CreatedAt: now, UpdatedAt: now, Artifacts: []workbench.Artifact{}}
	claim := workbench.OperationClaim{ID: workbench.OperationID(r.ids.NewID("wbo_")), OwnerID: command.Identity.ActorID, RunID: runID, Key: command.IdempotencyKey, Kind: workbench.OperationCreate, Fingerprint: workbench.FingerprintLaunch(command)}
	event := r.newEvent(&run, workbench.EventRunQueued, workbench.StateQueued, now)
	claimed, err := r.store.ClaimOperation(ctx, WorkbenchOperationRequest{Identity: command.Identity, Claim: claim, Run: run, Event: event})
	if err != nil {
		return workbench.LaunchResult{}, err
	}
	if claimed.Replayed {
		return workbench.LaunchResult{Run: claimed.Run, Replayed: true}, nil
	}
	completed, err := r.execute(ctx, command.Identity, claimed.Run, resolved, command.IdempotencyKey)
	return workbench.LaunchResult{Run: completed}, err
}

func (r *WorkbenchRuntime) Run(ctx context.Context, command workbench.RunCommand) (workbench.RunResult, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.RunResult{}, err
	}
	run, err := r.store.LoadRun(ctx, command.Identity, command.RunID)
	return workbench.RunResult{Run: run}, err
}

func (r *WorkbenchRuntime) Cancel(ctx context.Context, command workbench.CancelCommand) (workbench.CancelResult, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.CancelResult{}, err
	}
	claim := workbench.OperationClaim{ID: workbench.OperationID(r.ids.NewID("wbo_")), OwnerID: command.Identity.ActorID, RunID: command.RunID, Key: command.IdempotencyKey, Kind: workbench.OperationCancel, Fingerprint: workbench.FingerprintCancel(command)}
	var result WorkbenchOperationResult
	for attempt := 0; attempt < 4; attempt++ {
		before, err := r.store.LoadRun(ctx, command.Identity, command.RunID)
		if err != nil {
			return workbench.CancelResult{}, err
		}
		var cancelErr error
		result, cancelErr = r.store.RequestCancel(ctx, WorkbenchCancelRequest{Identity: command.Identity, RunID: command.RunID, Claim: claim})
		if cancelErr == nil {
			break
		}
		latest, loadErr := r.store.LoadRun(ctx, command.Identity, command.RunID)
		if loadErr != nil {
			return workbench.CancelResult{}, errors.Join(cancelErr, loadErr)
		}
		if latest.Version == before.Version {
			return workbench.CancelResult{}, cancelErr
		}
		if latest.State.Terminal() {
			return workbench.CancelResult{Run: latest}, nil
		}
		if latest.State == workbench.StateCancelRequested {
			result = WorkbenchOperationResult{Run: latest, Replayed: true}
			break
		}
		if attempt == 3 {
			return workbench.CancelResult{}, cancelErr
		}
	}
	r.cancelMu.Lock()
	if cancel := r.cancels[command.RunID]; cancel != nil {
		cancel()
	}
	r.cancelMu.Unlock()
	if err := r.executor.Cancel(ctx, command.RunID); err != nil {
		return workbench.CancelResult{}, err
	}
	return workbench.CancelResult{Run: result.Run, Replayed: result.Replayed}, nil
}

func (r *WorkbenchRuntime) Save(ctx context.Context, command workbench.SaveCommand) (workbench.SaveResult, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.SaveResult{}, err
	}
	claim := workbench.OperationClaim{ID: workbench.OperationID(r.ids.NewID("wbo_")), OwnerID: command.Identity.ActorID, RunID: command.RunID, Key: command.IdempotencyKey, Kind: workbench.OperationSave, Fingerprint: workbench.FingerprintSave(command)}
	return r.store.SaveSnapshot(ctx, WorkbenchSaveRequest{Identity: command.Identity, RunID: command.RunID, SnapshotID: workbench.SnapshotID(r.ids.NewID("wbs_")), Label: command.ReplayLabel, Claim: claim})
}

func (r *WorkbenchRuntime) Replay(ctx context.Context, command workbench.ReplayCommand) (workbench.ReplayResult, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.ReplayResult{}, err
	}
	newID := workbench.RunID(r.ids.NewID("wbr_"))
	claim := workbench.OperationClaim{ID: workbench.OperationID(r.ids.NewID("wbo_")), OwnerID: command.Identity.ActorID, RunID: workbench.RunID(command.ReplayID), Key: command.IdempotencyKey, Kind: workbench.OperationReplay, Fingerprint: workbench.FingerprintReplay(command)}
	derived, err := r.store.DeriveRun(ctx, WorkbenchDeriveRequest{Identity: command.Identity, Claim: claim, SourceSnapshotID: command.ReplayID, NewRunID: newID, Kind: workbench.DerivationReplay})
	if err != nil {
		return workbench.ReplayResult{}, err
	}
	if strings.HasPrefix(derived.Run.Input.Intent, AssetExecutionIntentPrefix) {
		if !derived.Run.State.Terminal() {
			_, persistErr := r.finishExecution(ctx, command.Identity, derived.Run.ID, nil, ErrAssetCommerceForbidden)
			return workbench.ReplayResult{}, errors.Join(ErrAssetCommerceForbidden, persistErr)
		}
		return workbench.ReplayResult{}, ErrAssetCommerceForbidden
	}
	if derived.Replayed {
		return workbench.ReplayResult{Run: derived.Run, SourceSnapshotID: derived.SourceSnapshotID, Replayed: true}, nil
	}
	resolved, err := r.resolveInput(ctx, command.Identity, derived.Run.Input)
	if err != nil {
		failed, persistErr := r.finishExecution(ctx, command.Identity, derived.Run.ID, nil, err)
		return workbench.ReplayResult{Run: failed, SourceSnapshotID: derived.SourceSnapshotID}, errors.Join(err, persistErr)
	}
	run, err := r.execute(ctx, command.Identity, derived.Run, resolved, command.IdempotencyKey)
	return workbench.ReplayResult{Run: run, SourceSnapshotID: derived.SourceSnapshotID}, err
}

func (r *WorkbenchRuntime) Fork(ctx context.Context, command workbench.ForkCommand) (workbench.ForkResult, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.ForkResult{}, err
	}
	source, err := r.store.LoadRun(ctx, command.Identity, command.RunID)
	if err != nil {
		return workbench.ForkResult{}, err
	}
	if strings.HasPrefix(source.Input.Intent, AssetExecutionIntentPrefix) {
		return workbench.ForkResult{}, ErrAssetCommerceForbidden
	}
	newID := workbench.RunID(r.ids.NewID("wbr_"))
	claim := workbench.OperationClaim{ID: workbench.OperationID(r.ids.NewID("wbo_")), OwnerID: command.Identity.ActorID, RunID: command.RunID, Key: command.IdempotencyKey, Kind: workbench.OperationFork, Fingerprint: workbench.FingerprintFork(command)}
	derived, err := r.store.DeriveRun(ctx, WorkbenchDeriveRequest{Identity: command.Identity, Claim: claim, SourceRunID: command.RunID, NewRunID: newID, Kind: workbench.DerivationFork, Run: source})
	if err != nil {
		return workbench.ForkResult{}, err
	}
	if derived.Replayed {
		return workbench.ForkResult{Run: derived.Run, SourceSnapshotID: derived.SourceSnapshotID, Replayed: true}, nil
	}
	resolved, err := r.resolveInput(ctx, command.Identity, derived.Run.Input)
	if err != nil {
		failed, persistErr := r.finishExecution(ctx, command.Identity, derived.Run.ID, nil, err)
		return workbench.ForkResult{Run: failed, SourceSnapshotID: derived.SourceSnapshotID}, errors.Join(err, persistErr)
	}
	run, err := r.execute(ctx, command.Identity, derived.Run, resolved, command.IdempotencyKey)
	return workbench.ForkResult{Run: run, SourceSnapshotID: derived.SourceSnapshotID}, err
}

func (r *WorkbenchRuntime) Artifact(ctx context.Context, command workbench.ArtifactCommand) (workbench.Artifact, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.Artifact{}, err
	}
	return r.store.LoadArtifact(ctx, command.Identity, command.RunID, command.ArtifactID)
}

func (r *WorkbenchRuntime) Events(ctx context.Context, command workbench.EventsCommand) (workbench.EventStreamResult, error) {
	if err := command.Identity.Validate(); err != nil {
		return workbench.EventStreamResult{}, err
	}
	read, err := r.store.ReadEvents(ctx, command.Identity, command.RunID, command.Cursor, command.Limit)
	if err != nil {
		return workbench.EventStreamResult{}, err
	}
	return workbench.ReadEventStream(read)
}

func (r *WorkbenchRuntime) execute(ctx context.Context, identity workbench.Identity, run workbench.Run, resolved ResolvedWorkbenchLaunch, idempotencyKey string) (workbench.Run, error) {
	executionCtx, cancel := context.WithCancel(ctx)
	r.cancelMu.Lock()
	if r.cancels == nil {
		r.cancels = make(map[workbench.RunID]context.CancelFunc)
	}
	if _, exists := r.cancels[run.ID]; exists {
		r.cancelMu.Unlock()
		cancel()
		return run, workbench.ErrInvalidTransition
	}
	r.cancels[run.ID] = cancel
	r.cancelMu.Unlock()
	r.active.Add(1)
	defer func() {
		cancel()
		r.cancelMu.Lock()
		delete(r.cancels, run.ID)
		r.cancelMu.Unlock()
		r.active.Add(-1)
	}()
	// Registration precedes the queued -> running write. A cancellation that
	// arrived before registration is observed by the authoritative state read.
	running, err := r.mutateRuntimeRun(executionCtx, identity, run.ID, func(current workbench.Run) (workbench.Run, *workbench.Artifact, error) {
		if current.State == workbench.StateCancelRequested {
			return current, nil, context.Canceled
		}
		next, e := workbench.ApplyTransition(current, workbench.Transition{To: workbench.StateRunning, At: r.clock.Now()})
		return next, nil, e
	})
	if err != nil {
		terminal, persistErr := r.finishExecution(ctx, identity, run.ID, nil, err)
		return terminal, errors.Join(err, persistErr)
	}
	if running.State.Terminal() {
		return running, nil
	}
	if err := executionCtx.Err(); err != nil {
		terminal, persistErr := r.finishExecution(ctx, identity, run.ID, nil, err)
		return terminal, errors.Join(err, persistErr)
	}
	request := WorkbenchCanonicalRequest{RunID: running.ID, IdempotencyKey: idempotencyKey, Intent: running.Input.Intent, Resolved: resolved}
	result, executionErr := r.executor.Execute(executionCtx, request)
	var outcome *WorkbenchCanonicalResult
	if executionErr == nil {
		outcome = &result
	}
	terminal, persistErr := r.finishExecution(ctx, identity, running.ID, outcome, executionErr)
	return terminal, errors.Join(executionErr, persistErr)
}

func (r *WorkbenchRuntime) finishExecution(ctx context.Context, identity workbench.Identity, runID workbench.RunID, result *WorkbenchCanonicalResult, cause error) (workbench.Run, error) {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return r.mutateRuntimeRun(persistCtx, identity, runID, func(current workbench.Run) (workbench.Run, *workbench.Artifact, error) {
		target := workbench.StateSucceeded
		var failure *workbench.RunFailure
		if result == nil {
			target = workbench.StateFailed
			message := "execution unavailable"
			if cause != nil {
				message = cause.Error()
			}
			failure = &workbench.RunFailure{Code: "execution_failed", Message: message}
			if errors.Is(cause, context.Canceled) || current.State == workbench.StateCancelRequested {
				target, failure = workbench.StateCancelled, nil
			}
		}
		next, err := workbench.ApplyTransition(current, workbench.Transition{To: target, At: r.clock.Now(), Failure: failure})
		if err != nil || result == nil {
			return next, nil, err
		}
		next.CanonicalRequestID = result.CanonicalRequestID
		next.CanonicalUsageEventID = result.CanonicalUsageEventID
		next.JournalID = result.JournalID
		next.ActualCost = cloneWorkbenchMoney(result.ActualCost)
		artifact := &workbench.Artifact{ArtifactID: workbench.ArtifactID(r.ids.NewID("wba_")), RunID: next.ID,
			OwnerID: next.OwnerID, Kind: workbench.ArtifactText, ContentType: result.ContentType,
			Body: append([]byte(nil), result.Body...), StorageURI: "workbench://" + string(next.ID), ByteSize: int64(len(result.Body)),
			ArtifactDigest: workbench.Digest(sha256Hex(result.Body)), CanonicalRequestID: result.CanonicalRequestID,
			CanonicalUsageEventID: result.CanonicalUsageEventID, AcceptedQuoteID: next.Input.AcceptedQuoteID,
			AcceptedQuoteSHA: next.Input.AcceptedQuoteSHA, JournalID: result.JournalID, CreatedAt: r.clock.Now()}
		if next.Input.CapabilityVersion == "native-metered-v1" {
			// Native input carries a signed resource authorization, not an
			// accepted_quotes row or a canonical settlement journal.
			next.AcceptedQuoteID, next.AcceptedQuoteSHA, next.CanonicalUsageEventID, next.JournalID = "", "", "", ""
			artifact.AcceptedQuoteID, artifact.AcceptedQuoteSHA, artifact.CanonicalUsageEventID, artifact.JournalID = "", "", "", ""
		}
		next.Artifacts = append(next.Artifacts, *artifact)
		return next, artifact, nil
	})
}

// Only retry a failed mutation when a fresh read proves its optimistic version
// changed. Transport and constraint failures are returned without blind retries.
func (r *WorkbenchRuntime) mutateRuntimeRun(ctx context.Context, identity workbench.Identity, runID workbench.RunID,
	apply func(workbench.Run) (workbench.Run, *workbench.Artifact, error)) (workbench.Run, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		current, err := r.store.LoadRun(ctx, identity, runID)
		if err != nil {
			return workbench.Run{}, err
		}
		if current.State.Terminal() {
			return current, nil
		}
		next, artifact, err := apply(current)
		if err != nil {
			return current, err
		}
		kind := eventForState(next.State)
		if next.State == workbench.StateRunning {
			kind = workbench.EventRunStarted
		}
		if next.State == workbench.StateSucceeded {
			kind = workbench.EventRunSucceeded
		}
		event := r.newEvent(&next, kind, next.State, next.UpdatedAt)
		var stored workbench.Run
		if artifact != nil {
			stored, err = r.store.CompleteRun(ctx, WorkbenchExecutionCommit{Identity: identity, Run: next, Artifact: *artifact, Event: event})
		} else {
			stored, err = r.store.TransitionRun(ctx, WorkbenchRunMutation{Identity: identity, Run: next, Event: event})
		}
		if err == nil {
			return stored, nil
		}
		lastErr = err
		latest, loadErr := r.store.LoadRun(ctx, identity, runID)
		if loadErr != nil {
			return current, errors.Join(err, loadErr)
		}
		if latest.State.Terminal() {
			return latest, nil
		}
		if latest.Version == current.Version {
			return latest, err
		}
	}
	return workbench.Run{}, lastErr
}

func deterministicWorkbenchRunID(identity workbench.Identity, key string) workbench.RunID {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s", identity.ActorID, key)))
	return workbench.RunID("wbr_" + hex.EncodeToString(sum[:16]))
}
func (r *WorkbenchRuntime) resolveInput(ctx context.Context, identity workbench.Identity, input workbench.InputSnapshot) (ResolvedWorkbenchLaunch, error) {
	return r.catalog.ResolveLaunch(ctx, workbench.LaunchCommand{Identity: identity, CapabilityID: string(input.CapabilityID), CapabilityVersion: input.CapabilityVersion, CapabilityDigest: string(input.CapabilityDigest), CanonicalModelID: input.CanonicalModelID, CanonicalModelVersion: input.CanonicalModelVersion, AcceptedQuoteID: string(input.AcceptedQuoteID), AcceptedQuoteSHA: string(input.AcceptedQuoteSHA), Intent: input.Intent, IdempotencyKey: string(input.AcceptedQuoteID)})
}
func (r *WorkbenchRuntime) newEvent(run *workbench.Run, kind workbench.EventKind, state workbench.RunState, at time.Time) workbench.Event {
	seq := run.NextEventSeq
	if seq == 0 {
		seq = 1
	}
	run.NextEventSeq = seq + 1
	return workbench.Event{ID: workbench.EventID(r.ids.NewID("wbe_")), RunID: run.ID, Seq: seq, Kind: kind, State: state, CreatedAt: at}
}
func eventForState(state workbench.RunState) workbench.EventKind {
	switch state {
	case workbench.StateCancelled:
		return workbench.EventRunCancelled
	default:
		return workbench.EventRunFailed
	}
}
func mustJSON(value any) []byte    { body, _ := json.Marshal(value); return body }
func sha256Hex(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func cloneWorkbenchMoney(value *workbench.Money) *workbench.Money {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
