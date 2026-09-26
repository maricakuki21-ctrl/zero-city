package workbench

import (
	"errors"
	"testing"
	"time"
)

func TestState_ApplyTransitionAdvancesVersionAndTerminalTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 2, 10, 0, 0, 0, time.UTC)
	run := Run{ID: "wbr_state", State: StateRunning, Version: 7}

	got, err := ApplyTransition(run, Transition{To: StateSucceeded, At: now})
	if err != nil {
		t.Fatalf("ApplyTransition() error = %v", err)
	}
	if got.State != StateSucceeded || got.Version != 8 || got.TerminalAt == nil || !got.TerminalAt.Equal(now) {
		t.Fatalf("ApplyTransition() = %#v", got)
	}
	if run.State != StateRunning || run.Version != 7 || run.TerminalAt != nil {
		t.Fatalf("input run mutated = %#v", run)
	}
}

func TestState_TerminalRunRejectsFurtherTransition(t *testing.T) {
	t.Parallel()

	_, err := ApplyTransition(Run{ID: "wbr_terminal", State: StateSucceeded, Version: 3}, Transition{To: StateFailed, At: time.Now()})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("ApplyTransition() error = %v, want ErrInvalidTransition", err)
	}
}

func TestState_CancelRequestedMayResolveWithoutReopeningDispatch(t *testing.T) {
	t.Parallel()

	for _, target := range []RunState{StateSucceeded, StateFailed, StateCancelled} {
		if !CanTransition(StateCancelRequested, target) {
			t.Fatalf("CanTransition(%q, %q) = false", StateCancelRequested, target)
		}
	}
	if CanTransition(StateCancelRequested, StateRunning) {
		t.Fatal("cancel-requested run must not return to running")
	}
}
