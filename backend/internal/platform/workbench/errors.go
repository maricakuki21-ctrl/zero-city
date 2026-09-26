package workbench

import (
	"errors"
	"fmt"
)

var (
	ErrCanonicalRuntimeUnavailable = errors.New("canonical workbench runtime is unavailable")
	ErrInvalidCommand              = errors.New("workbench invalid command")
	ErrInvalidDecimal              = errors.New("workbench decimal must be a non-negative NUMERIC(24,12) string")
	ErrInvalidTransition           = errors.New("workbench invalid transition")
	ErrOwnershipMismatch           = errors.New("workbench ownership mismatch")
	ErrRunNotFound                 = errors.New("workbench run not found")
	ErrArtifactNotFound            = errors.New("workbench artifact not found")
	ErrSnapshotNotFound            = errors.New("workbench snapshot not found")
	ErrIdempotencyConflict         = errors.New("workbench idempotency conflict")
	ErrCursorRunMismatch           = errors.New("workbench cursor belongs to another run")
	ErrStaleCursor                 = errors.New("workbench stream cursor is stale")
	ErrInvalidRecovery             = errors.New("workbench invalid recovery state")
)

type TransitionError struct {
	From RunState
	To   RunState
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("%s: %s -> %s", ErrInvalidTransition, e.From, e.To)
}

func (e *TransitionError) Unwrap() error { return ErrInvalidTransition }

type IdempotencyConflictError struct {
	Key       string
	Existing  RequestFingerprint
	Requested RequestFingerprint
}

func (e *IdempotencyConflictError) Error() string {
	return fmt.Sprintf("%s: operation key %q has fingerprint %s, requested %s", ErrIdempotencyConflict, e.Key, e.Existing, e.Requested)
}

func (e *IdempotencyConflictError) Unwrap() error { return ErrIdempotencyConflict }

type StaleCursorError struct {
	Cursor StreamCursor
	Reset  StreamReset
}

func (e *StaleCursorError) Error() string {
	return fmt.Sprintf("%s: run %s sequence %d", ErrStaleCursor, e.Cursor.RunID, e.Cursor.Seq)
}

func (e *StaleCursorError) Unwrap() error { return ErrStaleCursor }
