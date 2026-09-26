package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrNoAttemptCandidates      = errors.New("attempt coordinator: no candidates")
	ErrInvalidAttemptCandidate  = errors.New("attempt coordinator: invalid candidate")
	ErrInvalidAttemptTransition = errors.New("attempt coordinator: invalid state transition")
	ErrMissingAttemptExecutor   = errors.New("attempt coordinator: missing executor")
	ErrMissingRetryClassifier   = errors.New("attempt coordinator: missing retry classifier")
)

type AttemptState string

const (
	AttemptStateNotStarted      AttemptState = "not_started"
	AttemptStateAttempted       AttemptState = "attempted"
	AttemptStateCommitted       AttemptState = "committed"
	AttemptStateAcceptedTask    AttemptState = "accepted_task"
	AttemptStateTerminalUnknown AttemptState = "terminal_unknown"
	AttemptStateSucceeded       AttemptState = "succeeded"
	AttemptStateFailed          AttemptState = "failed"
	AttemptStateCancelled       AttemptState = "cancelled"
)

type AttemptCandidate struct {
	ID string
}

type AttemptRecord struct {
	Candidate AttemptCandidate
	State     AttemptState
	Err       error
}

type AttemptResult struct {
	State   AttemptState
	Records []AttemptRecord
	Err     error
}

type AttemptExecutor func(ctx context.Context, candidate AttemptCandidate, observer *AttemptObserver) error

type RetryClassifier func(err error) bool

type AttemptCoordinator struct {
	isRetryable RetryClassifier
}

func NewAttemptCoordinator(isRetryable RetryClassifier) (*AttemptCoordinator, error) {
	if isRetryable == nil {
		return nil, ErrMissingRetryClassifier
	}
	return &AttemptCoordinator{isRetryable: isRetryable}, nil
}

func (c *AttemptCoordinator) Run(
	ctx context.Context,
	candidates []AttemptCandidate,
	execute AttemptExecutor,
) AttemptResult {
	if len(candidates) == 0 {
		return AttemptResult{State: AttemptStateNotStarted, Err: ErrNoAttemptCandidates}
	}
	if execute == nil {
		return AttemptResult{State: AttemptStateNotStarted, Err: ErrMissingAttemptExecutor}
	}

	records := make([]AttemptRecord, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == "" {
			err := ErrInvalidAttemptCandidate
			return AttemptResult{State: AttemptStateFailed, Records: records, Err: err}
		}
		if err := ctx.Err(); err != nil {
			records = append(records, AttemptRecord{Candidate: candidate, State: AttemptStateCancelled, Err: err})
			return AttemptResult{State: AttemptStateCancelled, Records: records, Err: err}
		}

		observer := newAttemptObserver()
		attemptCtx, cancelAttempt := context.WithCancel(ctx)
		err := execute(attemptCtx, candidate, observer)
		cancelAttempt()
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		state := observer.State()

		if err == nil {
			if state == AttemptStateAttempted || state == AttemptStateCommitted {
				if transitionErr := observer.transition(AttemptStateSucceeded); transitionErr != nil {
					return AttemptResult{State: state, Records: records, Err: transitionErr}
				}
				state = observer.State()
			}
			records = append(records, AttemptRecord{Candidate: candidate, State: state})
			return AttemptResult{State: state, Records: records}
		}

		if state == AttemptStateCommitted || state == AttemptStateAcceptedTask {
			if transitionErr := observer.transition(AttemptStateTerminalUnknown); transitionErr != nil {
				return AttemptResult{State: state, Records: records, Err: transitionErr}
			}
			state = AttemptStateTerminalUnknown
		} else if ctx.Err() != nil {
			if transitionErr := observer.transition(AttemptStateCancelled); transitionErr != nil {
				return AttemptResult{State: state, Records: records, Err: transitionErr}
			}
			state = AttemptStateCancelled
		} else {
			if transitionErr := observer.transition(AttemptStateFailed); transitionErr != nil {
				return AttemptResult{State: state, Records: records, Err: transitionErr}
			}
			state = AttemptStateFailed
		}

		records = append(records, AttemptRecord{Candidate: candidate, State: state, Err: err})
		if state != AttemptStateFailed || !c.isRetryable(err) {
			return AttemptResult{State: state, Records: records, Err: err}
		}
	}

	last := records[len(records)-1]
	return AttemptResult{State: last.State, Records: records, Err: last.Err}
}

func (c *AttemptCoordinator) canRetry(state AttemptState, err error) bool {
	return c != nil && c.isRetryable != nil && state == AttemptStateAttempted && c.isRetryable(err)
}

type AttemptObserver struct {
	mu    sync.Mutex
	state AttemptState
}

func newAttemptObserver() *AttemptObserver {
	return &AttemptObserver{state: AttemptStateAttempted}
}

func (o *AttemptObserver) State() AttemptState {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state
}

func (o *AttemptObserver) Commit() error {
	return o.transition(AttemptStateCommitted)
}

func (o *AttemptObserver) AcceptTask() error {
	return o.transition(AttemptStateAcceptedTask)
}

func (o *AttemptObserver) Succeed() error {
	return o.transition(AttemptStateSucceeded)
}

func (o *AttemptObserver) transition(next AttemptState) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.state == next {
		return nil
	}
	if !validAttemptTransition(o.state, next) {
		return fmt.Errorf("%s -> %s: %w", o.state, next, ErrInvalidAttemptTransition)
	}
	o.state = next
	return nil
}

func validAttemptTransition(current, next AttemptState) bool {
	switch current {
	case AttemptStateAttempted:
		return next == AttemptStateCommitted ||
			next == AttemptStateAcceptedTask ||
			next == AttemptStateSucceeded ||
			next == AttemptStateFailed ||
			next == AttemptStateCancelled
	case AttemptStateCommitted:
		return next == AttemptStateSucceeded || next == AttemptStateTerminalUnknown
	case AttemptStateAcceptedTask:
		return next == AttemptStateSucceeded || next == AttemptStateTerminalUnknown
	case AttemptStateNotStarted,
		AttemptStateTerminalUnknown,
		AttemptStateSucceeded,
		AttemptStateFailed,
		AttemptStateCancelled:
		return false
	default:
		return false
	}
}
