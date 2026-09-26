package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebSocketFirstApplicationFramePreventsRetry(t *testing.T) {
	source, err := os.ReadFile("openai_gateway_shared_pool.go")
	if err != nil {
		t.Fatalf("post-commit retry: read shared gateway service: %v", err)
	}
	if bytes.Contains(source, []byte("forwardSharedPoolWithCapacityRetry(")) {
		t.Fatal("post-commit retry: first WebSocket application frame can still enter shared retry loop")
	}
}

var errRetryableAttempt = errors.New("retryable attempt")

func TestAttemptCoordinatorRunsSchedulerOrderSerially(t *testing.T) {
	coordinator, err := NewAttemptCoordinator(func(err error) bool {
		return errors.Is(err, errRetryableAttempt)
	})
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}

	candidates := []AttemptCandidate{{ID: "account-a"}, {ID: "account-b"}, {ID: "account-c"}}
	var inFlight atomic.Int32
	var peak atomic.Int32
	var called []string
	result := coordinator.Run(context.Background(), candidates, func(_ context.Context, candidate AttemptCandidate, observer *AttemptObserver) error {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		if current > peak.Load() {
			peak.Store(current)
		}
		called = append(called, candidate.ID)
		if candidate.ID != "account-c" {
			return errRetryableAttempt
		}
		return observer.Commit()
	})

	if result.Err != nil {
		t.Fatalf("run coordinator: %v", result.Err)
	}
	if result.State != AttemptStateSucceeded {
		t.Fatalf("state=%s, want %s", result.State, AttemptStateSucceeded)
	}
	if peak.Load() != 1 {
		t.Fatalf("peak in-flight=%d, want 1", peak.Load())
	}
	if len(called) != 3 || called[0] != "account-a" || called[1] != "account-b" || called[2] != "account-c" {
		t.Fatalf("candidate order=%v", called)
	}
	if len(result.Records) != 3 || result.Records[0].State != AttemptStateFailed || result.Records[1].State != AttemptStateFailed || result.Records[2].State != AttemptStateSucceeded {
		t.Fatalf("records=%+v", result.Records)
	}
}

func TestAttemptCoordinatorDoesNotRetryCommittedFailure(t *testing.T) {
	coordinator, err := NewAttemptCoordinator(func(error) bool { return true })
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}

	var calls atomic.Int32
	result := coordinator.Run(
		context.Background(),
		[]AttemptCandidate{{ID: "account-a"}, {ID: "account-b"}},
		func(_ context.Context, _ AttemptCandidate, observer *AttemptObserver) error {
			calls.Add(1)
			if err := observer.Commit(); err != nil {
				return err
			}
			return errRetryableAttempt
		},
	)

	if !errors.Is(result.Err, errRetryableAttempt) {
		t.Fatalf("error=%v, want retryable source error", result.Err)
	}
	if result.State != AttemptStateTerminalUnknown {
		t.Fatalf("state=%s, want %s", result.State, AttemptStateTerminalUnknown)
	}
	if calls.Load() != 1 || len(result.Records) != 1 {
		t.Fatalf("calls=%d records=%d, want one", calls.Load(), len(result.Records))
	}
}

func TestAttemptCoordinatorCancellationStopsActiveAttempt(t *testing.T) {
	coordinator, err := NewAttemptCoordinator(func(error) bool { return true })
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	var calls atomic.Int32
	result := coordinator.Run(ctx, []AttemptCandidate{{ID: "account-a"}, {ID: "account-b"}}, func(ctx context.Context, _ AttemptCandidate, _ *AttemptObserver) error {
		calls.Add(1)
		cancel()
		<-ctx.Done()
		return ctx.Err()
	})

	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("error=%v, want context canceled", result.Err)
	}
	if result.State != AttemptStateCancelled || calls.Load() != 1 || len(result.Records) != 1 {
		t.Fatalf("state=%s calls=%d records=%d", result.State, calls.Load(), len(result.Records))
	}
}

func TestAttemptCoordinatorAcceptedTaskFailureDoesNotRecreate(t *testing.T) {
	coordinator, err := NewAttemptCoordinator(func(error) bool { return true })
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}

	var calls atomic.Int32
	result := coordinator.Run(context.Background(), []AttemptCandidate{{ID: "media-a"}, {ID: "media-b"}}, func(_ context.Context, _ AttemptCandidate, observer *AttemptObserver) error {
		calls.Add(1)
		if err := observer.AcceptTask(); err != nil {
			return err
		}
		return errRetryableAttempt
	})

	if result.State != AttemptStateTerminalUnknown || calls.Load() != 1 {
		t.Fatalf("state=%s calls=%d", result.State, calls.Load())
	}
}

func TestAttemptCoordinatorDeadlineCancelsActiveAttempt(t *testing.T) {
	coordinator, err := NewAttemptCoordinator(func(error) bool { return true })
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	var calls atomic.Int32
	result := coordinator.Run(ctx, []AttemptCandidate{{ID: "account-a"}}, func(context.Context, AttemptCandidate, *AttemptObserver) error {
		calls.Add(1)
		return nil
	})

	if !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want deadline exceeded", result.Err)
	}
	if result.State != AttemptStateCancelled || calls.Load() != 0 || len(result.Records) != 1 {
		t.Fatalf("state=%s calls=%d records=%d", result.State, calls.Load(), len(result.Records))
	}
}

func TestAttemptObserverRejectsTransitionAfterTerminalState(t *testing.T) {
	observer := newAttemptObserver()
	if err := observer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := observer.Succeed(); err != nil {
		t.Fatalf("succeed: %v", err)
	}
	if err := observer.AcceptTask(); !errors.Is(err, ErrInvalidAttemptTransition) {
		t.Fatalf("transition error=%v, want invalid transition", err)
	}
}
