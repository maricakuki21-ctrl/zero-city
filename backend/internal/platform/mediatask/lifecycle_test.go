package mediatask_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	"github.com/stretchr/testify/require"
)

var errMediaTaskMissing = errors.New("media task missing")

type mediaLifecycleStore struct {
	mu    sync.Mutex
	tasks map[string]mediatask.Task
}

func newMediaLifecycleStore() *mediaLifecycleStore {
	return &mediaLifecycleStore{tasks: make(map[string]mediatask.Task)}
}

func mediaLifecycleKey(value mediatask.CreateContext) string {
	return value.BusinessEventID() + ":" + value.IdempotencyKey()
}

func (s *mediaLifecycleStore) ClaimCreate(_ context.Context, value mediatask.CreateContext) (mediatask.Task, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mediaLifecycleKey(value)
	if existing, ok := s.tasks[key]; ok {
		return existing, false, nil
	}
	task := mediatask.Task{Context: value, State: mediatask.StateCreating}
	s.tasks[key] = task
	return task, true, nil
}

func (s *mediaLifecycleStore) Accept(_ context.Context, value mediatask.CreateContext, upstreamTaskID string) (mediatask.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mediaLifecycleKey(value)
	task, ok := s.tasks[key]
	if !ok {
		return mediatask.Task{}, errMediaTaskMissing
	}
	if task.UpstreamTaskID != "" && task.UpstreamTaskID != upstreamTaskID {
		return mediatask.Task{}, mediatask.ErrInvalidTask
	}
	task.UpstreamTaskID = upstreamTaskID
	task.State = mediatask.StateAccepted
	s.tasks[key] = task
	return task, nil
}

func (s *mediaLifecycleStore) Get(_ context.Context, value mediatask.CreateContext) (mediatask.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[mediaLifecycleKey(value)]
	if !ok {
		return mediatask.Task{}, errMediaTaskMissing
	}
	return task, nil
}

func (s *mediaLifecycleStore) Finalize(_ context.Context, value mediatask.CreateContext, observation mediatask.Observation) (mediatask.Task, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mediaLifecycleKey(value)
	task, ok := s.tasks[key]
	if !ok {
		return mediatask.Task{}, false, errMediaTaskMissing
	}
	if task.State.Terminal() {
		return task, false, nil
	}
	task.State = observation.State
	if observation.State.Terminal() {
		task.SettlementEventID = "settle:" + value.BusinessEventID()
	}
	s.tasks[key] = task
	return task, true, nil
}

type mediaLifecycleUpstream struct {
	createCount  int
	statusCount  int
	cancelCount  int
	contentCount int
	status       mediatask.State
}

func (u *mediaLifecycleUpstream) Create(_ context.Context, value mediatask.CreateContext) (string, error) {
	u.createCount++
	return "upstream:" + value.BusinessEventID(), nil
}

func (u *mediaLifecycleUpstream) Status(_ context.Context, _ string) (mediatask.Observation, error) {
	u.statusCount++
	return mediatask.Observation{State: u.status}, nil
}

func (u *mediaLifecycleUpstream) Cancel(_ context.Context, _ string) (mediatask.Observation, error) {
	u.cancelCount++
	return mediatask.Observation{State: mediatask.StateCancelled}, nil
}

func (u *mediaLifecycleUpstream) Content(_ context.Context, _ string) ([]byte, error) {
	u.contentCount++
	return []byte("media-content"), nil
}

type mediaLifecycleSettlement struct{ count int }

func (s *mediaLifecycleSettlement) Apply(_ context.Context, _ mediatask.Task) error {
	s.count++
	return nil
}

func TestMediaTaskCreateReplaysAcceptedTaskWithoutRecreate(t *testing.T) {
	ctx := context.Background()
	createContext, err := mediatask.NewCreateContext("event-1", "client-key-1", []byte(`{"prompt":"cat"}`))
	require.NoError(t, err)
	store := newMediaLifecycleStore()
	upstream := &mediaLifecycleUpstream{}

	first, err := mediatask.NewCoordinator(store, upstream, nil).Create(ctx, createContext)
	require.NoError(t, err)
	replayed, err := mediatask.NewCoordinator(store, upstream, nil).Create(ctx, createContext)
	require.NoError(t, err)

	require.Equal(t, first.UpstreamTaskID, replayed.UpstreamTaskID)
	require.Equal(t, 1, upstream.createCount)
	require.Equal(t, "client-key-1", first.Context.IdempotencyKey())
}

func TestMediaTaskDuplicateTerminalPollSettlesOnce(t *testing.T) {
	ctx := context.Background()
	createContext, err := mediatask.NewCreateContext("event-2", "", []byte(`{"prompt":"video"}`))
	require.NoError(t, err)
	store := newMediaLifecycleStore()
	upstream := &mediaLifecycleUpstream{status: mediatask.StateSucceeded}
	settlement := &mediaLifecycleSettlement{}
	coordinator := mediatask.NewCoordinator(store, upstream, settlement)
	_, err = coordinator.Create(ctx, createContext)
	require.NoError(t, err)

	first, err := coordinator.Status(ctx, createContext)
	require.NoError(t, err)
	duplicate, err := coordinator.Status(ctx, createContext)
	require.NoError(t, err)

	require.Equal(t, mediatask.StateSucceeded, duplicate.State)
	require.Equal(t, first.SettlementEventID, duplicate.SettlementEventID)
	require.Equal(t, 1, upstream.statusCount)
	require.Equal(t, 1, settlement.count)
}

func TestMediaTaskCancelAndContentUseOriginalAcceptedTask(t *testing.T) {
	ctx := context.Background()
	createContext, err := mediatask.NewCreateContext("event-3", "client-key-3", []byte(`{"prompt":"clip"}`))
	require.NoError(t, err)
	store := newMediaLifecycleStore()
	upstream := &mediaLifecycleUpstream{}
	coordinator := mediatask.NewCoordinator(store, upstream, nil)
	created, err := coordinator.Create(ctx, createContext)
	require.NoError(t, err)

	content, err := coordinator.Content(ctx, createContext)
	require.NoError(t, err)
	cancelled, err := coordinator.Cancel(ctx, createContext)
	require.NoError(t, err)

	require.Equal(t, "upstream:event-3", created.UpstreamTaskID)
	require.Equal(t, []byte("media-content"), content)
	require.Equal(t, mediatask.StateCancelled, cancelled.State)
	require.Equal(t, 1, upstream.contentCount)
	require.Equal(t, 1, upstream.cancelCount)
}

func TestMediaHealthEvidenceNeverUsesPaidGeneration(t *testing.T) {
	require.NoError(t, mediatask.ValidateHealthEvidence(mediatask.HealthEvidence{
		Source: mediatask.HealthEvidenceSignedMetadata, Confidence: 80,
	}))
	require.NoError(t, mediatask.ValidateHealthEvidence(mediatask.HealthEvidence{
		Source: mediatask.HealthEvidenceFreeCapability, Confidence: 70,
	}))
	require.NoError(t, mediatask.ValidateHealthEvidence(mediatask.HealthEvidence{
		Source: mediatask.HealthEvidenceRealExecution, Confidence: 90,
	}))
	require.ErrorIs(t, mediatask.ValidateHealthEvidence(mediatask.HealthEvidence{
		Source: "paid_generation", Confidence: 100,
	}), mediatask.ErrPaidGenerationHealthProbe)
	require.Zero(t, mediatask.PaidGenerationProbeCount())
}
