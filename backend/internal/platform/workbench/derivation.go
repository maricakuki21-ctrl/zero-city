package workbench

import (
	"fmt"
	"time"
)

type SavedSnapshot struct {
	ID          SnapshotID    `json:"id"`
	RunID       RunID         `json:"run_id"`
	OwnerID     ActorID       `json:"owner_id"`
	WorkspaceID WorkspaceID   `json:"workspace_id"`
	Label       string        `json:"label"`
	Input       InputSnapshot `json:"input"`
	InputSHA256 Digest        `json:"input_sha256"`
	CreatedAt   time.Time     `json:"created_at"`
}

type DerivationKind string

const (
	DerivationReplay DerivationKind = "replay"
	DerivationFork   DerivationKind = "fork"
)

type Derivation struct {
	ID               string         `json:"id"`
	OwnerID          ActorID        `json:"owner_id"`
	SourceRunID      RunID          `json:"source_run_id"`
	SourceSnapshotID SnapshotID     `json:"source_snapshot_id"`
	DerivedRunID     RunID          `json:"derived_run_id"`
	Kind             DerivationKind `json:"kind"`
	CreatedAt        time.Time      `json:"created_at"`
}

type DerivationRequest struct {
	Identity Identity
	NewRunID RunID
	At       time.Time
}

func DeriveReplay(source Run, snapshot SavedSnapshot, request DerivationRequest) (ReplayResult, error) {
	run, err := deriveRun(source, snapshot, request, DerivationReplay)
	if err != nil {
		return ReplayResult{}, err
	}
	return ReplayResult{Run: run, SourceSnapshotID: snapshot.ID}, nil
}

func DeriveFork(source Run, snapshot SavedSnapshot, request DerivationRequest) (ForkResult, error) {
	run, err := deriveRun(source, snapshot, request, DerivationFork)
	if err != nil {
		return ForkResult{}, err
	}
	return ForkResult{Run: run, SourceSnapshotID: snapshot.ID}, nil
}

func deriveRun(source Run, snapshot SavedSnapshot, request DerivationRequest, kind DerivationKind) (Run, error) {
	if err := request.Identity.Validate(); err != nil {
		return Run{}, err
	}
	if request.NewRunID == "" || request.NewRunID == source.ID || request.At.IsZero() {
		return Run{}, fmt.Errorf("%w: derivation requires a new run id and timestamp", ErrInvalidCommand)
	}
	if source.State != StateSucceeded {
		return Run{}, &TransitionError{From: source.State, To: StateQueued}
	}
	if source.OwnerID != request.Identity.ActorID || snapshot.OwnerID != request.Identity.ActorID {
		return Run{}, ErrOwnershipMismatch
	}
	if snapshot.ID == "" || snapshot.RunID != source.ID || snapshot.WorkspaceID != source.WorkspaceID {
		return Run{}, fmt.Errorf("%w: snapshot does not belong to source run", ErrInvalidCommand)
	}

	derived := Run{
		ID:               request.NewRunID,
		OwnerID:          request.Identity.ActorID,
		WorkspaceID:      snapshot.WorkspaceID,
		State:            StateQueued,
		Input:            snapshot.Input,
		InputSHA256:      snapshot.InputSHA256,
		Version:          1,
		NextEventSeq:     1,
		AcceptedQuoteID:  snapshot.Input.AcceptedQuoteID,
		AcceptedQuoteSHA: snapshot.Input.AcceptedQuoteSHA,
		CreatedAt:        request.At,
		UpdatedAt:        request.At,
	}
	switch kind {
	case DerivationReplay:
		derived.ReplayOfRunID = source.ID
	case DerivationFork:
		derived.ForkedFromRunID = source.ID
	default:
		return Run{}, fmt.Errorf("%w: unknown derivation kind %q", ErrInvalidCommand, kind)
	}
	return derived, nil
}
