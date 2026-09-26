package workbench

import (
	"fmt"
	"time"
)

type RunState string

const (
	StateQueued          RunState = "queued"
	StateRunning         RunState = "running"
	StateCancelRequested RunState = "cancel_requested"
	StateSucceeded       RunState = "succeeded"
	StateFailed          RunState = "failed"
	StateCancelled       RunState = "cancelled"
)

func (s RunState) Valid() bool {
	switch s {
	case StateQueued, StateRunning, StateCancelRequested, StateSucceeded, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

func (s RunState) Terminal() bool {
	switch s {
	case StateSucceeded, StateFailed, StateCancelled:
		return true
	case StateQueued, StateRunning, StateCancelRequested:
		return false
	default:
		return false
	}
}

func CanTransition(from, to RunState) bool {
	switch from {
	case StateQueued:
		return to == StateRunning || to == StateCancelRequested || to == StateFailed || to == StateCancelled
	case StateRunning:
		return to == StateCancelRequested || to == StateSucceeded || to == StateFailed || to == StateCancelled
	case StateCancelRequested:
		return to == StateSucceeded || to == StateFailed || to == StateCancelled
	case StateSucceeded, StateFailed, StateCancelled:
		return false
	default:
		return false
	}
}

type Transition struct {
	To      RunState
	At      time.Time
	Failure *RunFailure
}

func ApplyTransition(run Run, transition Transition) (Run, error) {
	if !run.State.Valid() || !transition.To.Valid() || !CanTransition(run.State, transition.To) {
		return Run{}, &TransitionError{From: run.State, To: transition.To}
	}
	if transition.At.IsZero() {
		return Run{}, fmt.Errorf("%w: transition time is required", ErrInvalidCommand)
	}
	if transition.To == StateFailed && transition.Failure == nil {
		return Run{}, fmt.Errorf("%w: failed transition requires failure details", ErrInvalidCommand)
	}
	if transition.To != StateFailed && transition.Failure != nil {
		return Run{}, fmt.Errorf("%w: failure details require failed state", ErrInvalidCommand)
	}

	next := run.Clone()
	next.State = transition.To
	next.UpdatedAt = transition.At
	next.Version++
	next.Failure = cloneFailure(transition.Failure)
	if transition.To.Terminal() {
		at := transition.At
		next.TerminalAt = &at
	} else {
		next.TerminalAt = nil
	}
	return next, nil
}

type RecoveryPhase string

const (
	RecoveryPreCommit    RecoveryPhase = "pre_commit"
	RecoveryPostCommit   RecoveryPhase = "post_commit"
	RecoveryTaskAccepted RecoveryPhase = "task_accepted"
)

type RecoveryAction string

const (
	RecoveryRedispatchSameRequest RecoveryAction = "redispatch_same_request"
	RecoveryObserveCommitted      RecoveryAction = "observe_committed_request"
	RecoveryResumeAcceptedTask    RecoveryAction = "resume_accepted_task"
)

type RecoveryContext struct {
	Phase              RecoveryPhase
	CanonicalRequestID string
	MediaTaskID        string
}

func SelectRecoveryAction(recovery RecoveryContext) (RecoveryAction, error) {
	switch recovery.Phase {
	case RecoveryPreCommit:
		return RecoveryRedispatchSameRequest, nil
	case RecoveryPostCommit:
		if recovery.MediaTaskID != "" {
			return RecoveryResumeAcceptedTask, nil
		}
		if recovery.CanonicalRequestID == "" {
			return "", fmt.Errorf("%w: post-commit recovery requires canonical request identity", ErrInvalidRecovery)
		}
		return RecoveryObserveCommitted, nil
	case RecoveryTaskAccepted:
		if recovery.MediaTaskID == "" {
			return "", fmt.Errorf("%w: accepted-task recovery requires media task identity", ErrInvalidRecovery)
		}
		return RecoveryResumeAcceptedTask, nil
	default:
		return "", fmt.Errorf("%w: unknown phase %q", ErrInvalidRecovery, recovery.Phase)
	}
}
