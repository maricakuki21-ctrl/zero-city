package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

var errRuntimeTestVersion = errors.New("optimistic version conflict")

type runtimeFixExecutor struct {
	workbenchExecutorStub
	execute func(context.Context, WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error)
}

func (e *runtimeFixExecutor) Execute(ctx context.Context, in WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return e.execute(ctx, in)
}

type runtimeFixStore struct {
	*workbenchRuntimeStoreStub
	beforeStart      func()
	beforeComplete   func()
	startOnce        sync.Once
	completeOnce     sync.Once
	completeAttempts int
	terminalDetached bool
	deriveInput      workbench.InputSnapshot
	deriveCalls      int
}

func (s *runtimeFixStore) LoadRun(ctx context.Context, identity workbench.Identity, id workbench.RunID) (workbench.Run, error) {
	if err := ctx.Err(); err != nil {
		return workbench.Run{}, err
	}
	return s.workbenchRuntimeStoreStub.LoadRun(ctx, identity, id)
}
func (s *runtimeFixStore) TransitionRun(ctx context.Context, in WorkbenchRunMutation) (workbench.Run, error) {
	if in.Run.State == workbench.StateRunning {
		s.startOnce.Do(func() {
			if s.beforeStart != nil {
				s.beforeStart()
			}
		})
	}
	if err := ctx.Err(); err != nil {
		return workbench.Run{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs[in.Run.ID].Version+1 != in.Run.Version {
		return workbench.Run{}, errRuntimeTestVersion
	}
	if in.Run.State.Terminal() {
		_, s.terminalDetached = ctx.Deadline()
	}
	s.runs[in.Run.ID] = in.Run
	s.events[in.Run.ID] = append(s.events[in.Run.ID], in.Event)
	return in.Run, nil
}
func (s *runtimeFixStore) CompleteRun(ctx context.Context, in WorkbenchExecutionCommit) (workbench.Run, error) {
	s.completeOnce.Do(func() {
		if s.beforeComplete != nil {
			s.beforeComplete()
		}
	})
	if err := ctx.Err(); err != nil {
		return workbench.Run{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeAttempts++
	if s.runs[in.Run.ID].Version+1 != in.Run.Version {
		return workbench.Run{}, errRuntimeTestVersion
	}
	_, s.terminalDetached = ctx.Deadline()
	s.runs[in.Run.ID] = in.Run
	s.artifacts[in.Artifact.ArtifactID] = in.Artifact
	s.events[in.Run.ID] = append(s.events[in.Run.ID], in.Event)
	return in.Run, nil
}
func (s *runtimeFixStore) RequestCancel(ctx context.Context, in WorkbenchCancelRequest) (WorkbenchOperationResult, error) {
	if err := ctx.Err(); err != nil {
		return WorkbenchOperationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.runs[in.RunID]
	next, err := workbench.ApplyTransition(current, workbench.Transition{To: workbench.StateCancelRequested, At: time.Now()})
	if err != nil {
		return WorkbenchOperationResult{}, err
	}
	next.NextEventSeq++
	s.runs[in.RunID] = next
	return WorkbenchOperationResult{Run: next}, nil
}
func (s *runtimeFixStore) DeriveRun(_ context.Context, in WorkbenchDeriveRequest) (WorkbenchDeriveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deriveCalls++
	input := s.deriveInput
	if in.Kind == workbench.DerivationFork {
		input = in.Run.Input
	}
	run := workbench.Run{ID: in.NewRunID, OwnerID: in.Identity.ActorID, WorkspaceID: "wbw_owner",
		Input: input, State: workbench.StateQueued, Version: 1, NextEventSeq: 2}
	s.runs[run.ID] = run
	return WorkbenchDeriveResult{Run: run, SourceSnapshotID: in.SourceSnapshotID}, nil
}

func newRuntimeFixFixture(t *testing.T) (*WorkbenchRuntime, *runtimeFixStore, *runtimeFixExecutor, workbench.Run) {
	t.Helper()
	base := newWorkbenchRuntimeStoreStub()
	store := &runtimeFixStore{workbenchRuntimeStoreStub: base}
	executor := &runtimeFixExecutor{execute: func(context.Context, WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
		return WorkbenchCanonicalResult{CanonicalRequestID: "native-request", Body: []byte("result"), ContentType: "text/plain"}, nil
	}}
	catalog, err := NewWorkbenchCatalog(workbenchCatalogSourceStub{record: canonicalWorkbenchCatalogRecord(time.Now().Add(time.Hour))}, nil)
	require.NoError(t, err)
	runtime, err := NewWorkbenchRuntime(WorkbenchRuntimeDependencies{Store: store, Catalog: catalog, Executor: executor})
	require.NoError(t, err)
	cmd := canonicalWorkbenchLaunchCommand()
	input := workbench.InputSnapshot{Intent: cmd.Intent, CapabilityID: workbench.CapabilityID(cmd.CapabilityID),
		CapabilityVersion: cmd.CapabilityVersion, CapabilityDigest: workbench.Digest(cmd.CapabilityDigest),
		CanonicalModelID: cmd.CanonicalModelID, CanonicalModelVersion: cmd.CanonicalModelVersion,
		AcceptedQuoteID: workbench.QuoteID(cmd.AcceptedQuoteID), AcceptedQuoteSHA: workbench.Digest(cmd.AcceptedQuoteSHA)}
	run := workbench.Run{ID: "wbr_fix", OwnerID: 41, WorkspaceID: "wbw_owner", State: workbench.StateQueued,
		Version: 1, NextEventSeq: 2, Input: input}
	base.runs[run.ID] = run
	store.deriveInput = input
	return runtime, store, executor, run
}

func TestWorkbenchRuntimeNativeArtifactHasNoCanonicalForeignKeys(t *testing.T) {
	runtime, store, executor, run := newRuntimeFixFixture(t)
	run.Input.CapabilityVersion = "native-metered-v1"
	run.Input.AcceptedQuoteID = "native.signed.authorization"
	store.runs[run.ID] = run
	executor.execute = func(context.Context, WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
		return WorkbenchCanonicalResult{CanonicalRequestID: "actual-native-request", CanonicalUsageEventID: "not-a-real-event",
			JournalID: "not-a-real-journal", Body: []byte("output")}, nil
	}
	got, err := runtime.execute(context.Background(), workbench.Identity{ActorID: 41}, run, ResolvedWorkbenchLaunch{}, "operation")
	require.NoError(t, err)
	require.Equal(t, workbench.StateSucceeded, got.State)
	require.Equal(t, workbench.QuoteID("native.signed.authorization"), got.Input.AcceptedQuoteID)
	require.Empty(t, got.AcceptedQuoteID)
	require.Empty(t, got.CanonicalUsageEventID)
	require.Empty(t, got.JournalID)
	require.Len(t, got.Artifacts, 1)
	a := got.Artifacts[0]
	require.Empty(t, a.AcceptedQuoteID)
	require.Empty(t, a.AcceptedQuoteSHA)
	require.Empty(t, a.CanonicalUsageEventID)
	require.Empty(t, a.JournalID)
	require.Equal(t, "actual-native-request", a.CanonicalRequestID)
	require.True(t, store.terminalDetached)
}

func TestWorkbenchRuntimePersistsAfterRequestCancellation(t *testing.T) {
	for _, success := range []bool{true, false} {
		name := "failure"
		if success {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			runtime, store, executor, run := newRuntimeFixFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			executor.execute = func(context.Context, WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
				cancel()
				if success {
					return WorkbenchCanonicalResult{Body: []byte("committed output")}, nil
				}
				return WorkbenchCanonicalResult{}, context.Canceled
			}
			got, err := runtime.execute(ctx, workbench.Identity{ActorID: 41}, run, ResolvedWorkbenchLaunch{}, "operation")
			if success {
				require.NoError(t, err)
				require.Equal(t, workbench.StateSucceeded, got.State)
			} else {
				require.ErrorIs(t, err, context.Canceled)
				require.Equal(t, workbench.StateCancelled, got.State)
			}
			require.True(t, store.terminalDetached)
			require.True(t, store.runs[run.ID].State.Terminal())
			require.Zero(t, runtime.active.Load())
		})
	}
}

func TestWorkbenchRuntimeCancelActuallyStopsActiveExecution(t *testing.T) {
	runtime, store, executor, run := newRuntimeFixFixture(t)
	started := make(chan struct{})
	done := make(chan error, 1)
	executor.execute = func(ctx context.Context, _ WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
		close(started)
		<-ctx.Done()
		return WorkbenchCanonicalResult{}, ctx.Err()
	}
	go func() {
		_, err := runtime.execute(context.Background(), workbench.Identity{ActorID: 41}, run, ResolvedWorkbenchLaunch{}, "operation")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not start")
	}
	_, err := runtime.Cancel(context.Background(), workbench.CancelCommand{Identity: workbench.Identity{ActorID: 41}, RunID: run.ID, IdempotencyKey: "cancel-operation"})
	require.NoError(t, err)
	select {
	case err = <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("active context was not cancelled")
	}
	final, err := store.LoadRun(context.Background(), workbench.Identity{ActorID: 41}, run.ID)
	require.NoError(t, err)
	require.Equal(t, workbench.StateCancelled, final.State)
	require.Equal(t, uint64(4), final.Version)
	require.Empty(t, runtime.cancels)
}

func TestWorkbenchRuntimeCancellationBeforeExecutorRegistrationIsNotLost(t *testing.T) {
	runtime, store, executor, run := newRuntimeFixFixture(t)
	store.beforeStart = func() {
		_, err := runtime.Cancel(context.Background(), workbench.CancelCommand{Identity: workbench.Identity{ActorID: 41}, RunID: run.ID, IdempotencyKey: "cancel-before-dispatch"})
		require.NoError(t, err)
	}
	got, err := runtime.execute(context.Background(), workbench.Identity{ActorID: 41}, run, ResolvedWorkbenchLaunch{}, "operation")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, workbench.StateCancelled, got.State)
	require.Zero(t, executor.calls)
}

func TestWorkbenchRuntimeCompletionSurvivesCancelVersionRace(t *testing.T) {
	runtime, store, _, run := newRuntimeFixFixture(t)
	store.beforeComplete = func() {
		_, err := runtime.Cancel(context.Background(), workbench.CancelCommand{Identity: workbench.Identity{ActorID: 41}, RunID: run.ID, IdempotencyKey: "late-cancel"})
		require.NoError(t, err)
	}
	got, err := runtime.execute(context.Background(), workbench.Identity{ActorID: 41}, run, ResolvedWorkbenchLaunch{}, "operation")
	require.NoError(t, err)
	require.Equal(t, workbench.StateSucceeded, got.State)
	require.Equal(t, 2, store.completeAttempts)
	require.Len(t, store.artifacts, 1)
	require.Equal(t, uint64(4), got.Version)
}

func TestWorkbenchRuntimeAssetDerivationCannotBypassEntitlement(t *testing.T) {
	runtime, store, executor, run := newRuntimeFixFixture(t)
	run.Input.Intent = AssetExecutionIntentPrefix + " asset=41"
	store.runs[run.ID] = run
	store.deriveInput = run.Input
	_, err := runtime.Fork(context.Background(), workbench.ForkCommand{Identity: workbench.Identity{ActorID: 41}, RunID: run.ID, IdempotencyKey: "fork-asset"})
	require.ErrorIs(t, err, ErrAssetCommerceForbidden)
	require.Zero(t, store.deriveCalls)
	_, err = runtime.Replay(context.Background(), workbench.ReplayCommand{Identity: workbench.Identity{ActorID: 41}, ReplayID: "snapshot-asset", IdempotencyKey: "replay-asset"})
	require.ErrorIs(t, err, ErrAssetCommerceForbidden)
	require.Equal(t, 1, store.deriveCalls)
	require.Zero(t, executor.calls)
	for id, saved := range store.runs {
		if id != run.ID {
			require.Equal(t, workbench.StateFailed, saved.State)
		}
	}
}

func TestWorkbenchRuntimeExpiredDerivationDoesNotStayQueued(t *testing.T) {
	for _, fork := range []bool{false, true} {
		name := "replay"
		if fork {
			name = "fork"
		}
		t.Run(name, func(t *testing.T) {
			runtime, store, executor, run := newRuntimeFixFixture(t)
			runtime.catalog.clock = fixedWorkbenchClock{now: time.Now().Add(2 * time.Hour)}
			var err error
			if fork {
				_, err = runtime.Fork(context.Background(), workbench.ForkCommand{Identity: workbench.Identity{ActorID: 41}, RunID: run.ID, IdempotencyKey: "fork-expired"})
			} else {
				_, err = runtime.Replay(context.Background(), workbench.ReplayCommand{Identity: workbench.Identity{ActorID: 41}, ReplayID: "snapshot-expired", IdempotencyKey: "replay-expired"})
			}
			require.Error(t, err)
			require.Equal(t, 1, store.deriveCalls)
			require.Zero(t, executor.calls)
			for id, saved := range store.runs {
				if id != run.ID {
					require.Equal(t, workbench.StateFailed, saved.State)
				}
			}
		})
	}
}
