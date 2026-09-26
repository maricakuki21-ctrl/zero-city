package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

type workbenchExecutorStub struct {
	mu      sync.Mutex
	calls   int
	cancels int
	result  WorkbenchCanonicalResult
	started chan struct{}
}

func (s *workbenchExecutorStub) Execute(context.Context, WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
	s.mu.Lock()
	s.calls++
	if s.started != nil {
		select {
		case <-s.started:
		default:
			close(s.started)
		}
	}
	s.mu.Unlock()
	return s.result, nil
}
func (s *workbenchExecutorStub) Cancel(context.Context, workbench.RunID) error {
	s.mu.Lock()
	s.cancels++
	s.mu.Unlock()
	return nil
}

type workbenchIDStub struct{ n int }

func (s *workbenchIDStub) NewID(prefix string) string {
	s.n++
	return prefix + "id" + string(rune('a'+s.n))
}

type workbenchRuntimeStoreStub struct {
	mu        sync.Mutex
	workspace workbench.Workspace
	runs      map[workbench.RunID]workbench.Run
	claims    map[string]workbench.OperationRecord
	events    map[workbench.RunID][]workbench.Event
	snapshots map[workbench.SnapshotID]workbench.SavedSnapshot
	artifacts map[workbench.ArtifactID]workbench.Artifact
}

func newWorkbenchRuntimeStoreStub() *workbenchRuntimeStoreStub {
	return &workbenchRuntimeStoreStub{workspace: workbench.Workspace{ID: "wbw_owner", OwnerID: 41}, runs: map[workbench.RunID]workbench.Run{}, claims: map[string]workbench.OperationRecord{}, events: map[workbench.RunID][]workbench.Event{}, snapshots: map[workbench.SnapshotID]workbench.SavedSnapshot{}, artifacts: map[workbench.ArtifactID]workbench.Artifact{}}
}
func (s *workbenchRuntimeStoreStub) LoadWorkspace(context.Context, workbench.Identity) (workbench.Workspace, error) {
	return s.workspace, nil
}
func (s *workbenchRuntimeStoreStub) LoadRun(_ context.Context, identity workbench.Identity, id workbench.RunID) (workbench.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok || run.OwnerID != workbench.ActorID(identity.ActorID) {
		return workbench.Run{}, workbench.ErrRunNotFound
	}
	return run.Clone(), nil
}
func (s *workbenchRuntimeStoreStub) ClaimOperation(_ context.Context, input WorkbenchOperationRequest) (WorkbenchOperationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := string(input.Claim.RunID) + "|" + input.Claim.Key
	if existing, ok := s.claims[key]; ok {
		decision, err := workbench.ResolveIdempotency(&existing, input.Claim)
		if err != nil {
			return WorkbenchOperationResult{}, err
		}
		run := s.runs[decision.Result.RunID]
		return WorkbenchOperationResult{Run: run, Replayed: true}, nil
	}
	s.claims[key] = workbench.OperationRecord{Claim: input.Claim, State: workbench.OperationClaimed, Result: workbench.OperationResult{RunID: input.Run.ID, State: input.Run.State}}
	s.runs[input.Run.ID] = input.Run
	s.events[input.Run.ID] = append(s.events[input.Run.ID], input.Event)
	return WorkbenchOperationResult{Run: input.Run}, nil
}
func (s *workbenchRuntimeStoreStub) TransitionRun(_ context.Context, input WorkbenchRunMutation) (workbench.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[input.Run.ID] = input.Run
	s.events[input.Run.ID] = append(s.events[input.Run.ID], input.Event)
	return input.Run, nil
}
func (s *workbenchRuntimeStoreStub) CompleteRun(_ context.Context, input WorkbenchExecutionCommit) (workbench.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[input.Run.ID] = input.Run
	if input.Artifact.ArtifactID != "" {
		s.artifacts[input.Artifact.ArtifactID] = input.Artifact
	}
	return input.Run, nil
}
func (s *workbenchRuntimeStoreStub) RequestCancel(_ context.Context, input WorkbenchCancelRequest) (WorkbenchOperationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[input.RunID]
	run.State = workbench.StateCancelRequested
	run.Version++
	s.runs[input.RunID] = run
	return WorkbenchOperationResult{Run: run}, nil
}
func (s *workbenchRuntimeStoreStub) SaveSnapshot(_ context.Context, input WorkbenchSaveRequest) (workbench.SaveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := workbench.SavedSnapshot{ID: input.SnapshotID, RunID: input.RunID, OwnerID: input.Identity.ActorID, WorkspaceID: s.runs[input.RunID].WorkspaceID, Label: input.Label, Input: s.runs[input.RunID].Input, CreatedAt: time.Now()}
	s.snapshots[snapshot.ID] = snapshot
	return workbench.SaveResult{Run: s.runs[input.RunID], Snapshot: snapshot}, nil
}
func (s *workbenchRuntimeStoreStub) DeriveRun(_ context.Context, input WorkbenchDeriveRequest) (WorkbenchDeriveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := input.Run
	if existing, ok := s.claims[string(input.Claim.RunID)+"|"+input.Claim.Key]; ok {
		derived := s.runs[existing.Result.RunID]
		return WorkbenchDeriveResult{Run: derived, SourceSnapshotID: input.SourceSnapshotID, Replayed: true}, nil
	}
	s.claims[string(input.Claim.RunID)+"|"+input.Claim.Key] = workbench.OperationRecord{Claim: input.Claim, Result: workbench.OperationResult{RunID: run.ID, SnapshotID: input.SourceSnapshotID, State: run.State}}
	s.runs[run.ID] = run
	return WorkbenchDeriveResult{Run: run, SourceSnapshotID: input.SourceSnapshotID}, nil
}
func (s *workbenchRuntimeStoreStub) LoadArtifact(_ context.Context, identity workbench.Identity, runID workbench.RunID, artifactID workbench.ArtifactID) (workbench.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	artifact, ok := s.artifacts[artifactID]
	if !ok || artifact.RunID != runID || artifact.OwnerID != identity.ActorID {
		return workbench.Artifact{}, workbench.ErrArtifactNotFound
	}
	return artifact.Clone(), nil
}
func (s *workbenchRuntimeStoreStub) ReadEvents(_ context.Context, identity workbench.Identity, runID workbench.RunID, cursor workbench.StreamCursor, _ uint32) (workbench.EventRead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[runID]
	if run.OwnerID != identity.ActorID {
		return workbench.EventRead{}, workbench.ErrRunNotFound
	}
	return workbench.EventRead{Run: run, Events: append([]workbench.Event(nil), s.events[runID]...), Cursor: cursor, OldestAvailableSeq: 1}, nil
}

func TestWorkbenchRuntimeLaunchSameKeyDispatchesOnce(t *testing.T) {
	// Given
	store := newWorkbenchRuntimeStoreStub()
	catalog, err := NewWorkbenchCatalog(workbenchCatalogSourceStub{record: canonicalWorkbenchCatalogRecord(time.Now().Add(time.Hour))}, fixedWorkbenchClock{now: time.Now()})
	require.NoError(t, err)
	executor := &workbenchExecutorStub{result: WorkbenchCanonicalResult{CanonicalRequestID: "req_1", CanonicalUsageEventID: "cue_1", JournalID: "journal_1", Body: []byte("ok")}}
	runtime, err := NewWorkbenchRuntime(WorkbenchRuntimeDependencies{Store: store, Catalog: catalog, Executor: executor, IDs: &workbenchIDStub{}})
	require.NoError(t, err)
	command := canonicalWorkbenchLaunchCommand()

	// When
	first, err := runtime.Launch(context.Background(), command)
	require.NoError(t, err)
	// Retrying an already billed operation must not depend on catalog expiry.
	catalog.clock = fixedWorkbenchClock{now: time.Now().Add(2 * time.Hour)}
	second, err := runtime.Launch(context.Background(), command)

	// Then
	require.NoError(t, err)
	require.False(t, first.Replayed)
	require.True(t, second.Replayed)
	require.Equal(t, 1, executor.calls)
	require.Equal(t, workbench.StateSucceeded, first.Run.State)
	changed := command
	changed.Intent += " changed"
	_, err = runtime.Launch(context.Background(), changed)
	require.ErrorIs(t, err, workbench.ErrIdempotencyConflict)
	require.Equal(t, 1, executor.calls)
}

func TestWorkbenchRuntimeInspectSaveEventsAndArtifactDoNotDispatch(t *testing.T) {
	// Given
	runtime, store, executor := newWorkbenchRuntimeFixture(t)
	launched, err := runtime.Launch(context.Background(), canonicalWorkbenchLaunchCommand())
	require.NoError(t, err)
	before := executor.calls

	// When
	_, err = runtime.Run(context.Background(), workbench.RunCommand{Identity: workbench.Identity{ActorID: 41}, RunID: launched.Run.ID})
	require.NoError(t, err)
	_, err = runtime.Save(context.Background(), workbench.SaveCommand{Identity: workbench.Identity{ActorID: 41}, RunID: launched.Run.ID, ReplayLabel: "saved", IdempotencyKey: "save-1"})
	require.NoError(t, err)
	_, err = runtime.Events(context.Background(), workbench.EventsCommand{Identity: workbench.Identity{ActorID: 41}, RunID: launched.Run.ID, Limit: 50})
	require.NoError(t, err)
	_, err = runtime.Artifact(context.Background(), workbench.ArtifactCommand{Identity: workbench.Identity{ActorID: 41}, RunID: launched.Run.ID, ArtifactID: launched.Run.Artifacts[0].ArtifactID})

	// Then
	require.NoError(t, err)
	require.Equal(t, before, executor.calls)
	require.NotNil(t, store)
}

func TestWorkbenchRuntimeCancelCallsExecutorCancel(t *testing.T) {
	// Given
	runtime, _, executor := newWorkbenchRuntimeFixture(t)
	launched, err := runtime.Launch(context.Background(), canonicalWorkbenchLaunchCommand())
	require.NoError(t, err)

	// When
	_, err = runtime.Cancel(context.Background(), workbench.CancelCommand{Identity: workbench.Identity{ActorID: 41}, RunID: launched.Run.ID, IdempotencyKey: "cancel-1"})

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, executor.cancels)
}

func TestWorkbenchRuntimeAdapterUsesInjectedCanonicalSeams(t *testing.T) {
	// Given
	store := newWorkbenchRuntimeStoreStub()
	record := canonicalWorkbenchCatalogRecord(time.Now().Add(time.Hour))
	executor := &workbenchExecutorStub{result: WorkbenchCanonicalResult{CanonicalRequestID: "req_wired", CanonicalUsageEventID: "cue_wired", JournalID: "journal_wired", Body: []byte("ok")}}

	// When
	runtime, err := ProvideWorkbenchRuntimeAdapter(store, workbenchCatalogSourceStub{record: record}, executor)
	require.NoError(t, err)
	got, err := runtime.Launch(context.Background(), canonicalWorkbenchLaunchCommand())

	// Then
	require.NoError(t, err)
	require.Equal(t, "req_wired", got.Run.CanonicalRequestID)
	require.Equal(t, 1, executor.calls)
	require.NotEqual(t, ErrWorkbenchRuntimeUnavailable, err)
}

func newWorkbenchRuntimeFixture(t *testing.T) (*WorkbenchRuntime, *workbenchRuntimeStoreStub, *workbenchExecutorStub) {
	t.Helper()
	now := time.Now().Add(time.Hour)
	record := canonicalWorkbenchCatalogRecord(now)
	catalog, err := NewWorkbenchCatalog(workbenchCatalogSourceStub{record: record}, fixedWorkbenchClock{now: time.Now()})
	require.NoError(t, err)
	store := newWorkbenchRuntimeStoreStub()
	executor := &workbenchExecutorStub{result: WorkbenchCanonicalResult{CanonicalRequestID: "req_fixture", CanonicalUsageEventID: "cue_fixture", JournalID: "journal_fixture", Body: []byte("artifact")}}
	runtime, err := NewWorkbenchRuntime(WorkbenchRuntimeDependencies{Store: store, Catalog: catalog, Executor: executor, IDs: &workbenchIDStub{}})
	require.NoError(t, err)
	return runtime, store, executor
}
