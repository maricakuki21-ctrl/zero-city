package workbench

import (
	"testing"
	"time"
)

func TestReplay_DerivationCreatesNewRunWithoutPriorChargeOrArtifactIdentity(t *testing.T) {
	t.Parallel()

	source, snapshot := derivationFixture(t)
	result, err := DeriveReplay(source, snapshot, DerivationRequest{
		Identity: Identity{ActorID: 41}, NewRunID: "wbr_replay", At: source.UpdatedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("DeriveReplay() error = %v", err)
	}
	assertFreshDerivedRun(t, result.Run, "wbr_replay")
	if result.Run.ReplayOfRunID != source.ID || result.Run.ForkedFromRunID != "" {
		t.Fatalf("replay lineage = replay %q fork %q", result.Run.ReplayOfRunID, result.Run.ForkedFromRunID)
	}
	if result.SourceSnapshotID != snapshot.ID {
		t.Fatalf("SourceSnapshotID = %q, want %q", result.SourceSnapshotID, snapshot.ID)
	}
}

func TestFork_DerivationCreatesNewRunWithoutPriorChargeOrArtifactIdentity(t *testing.T) {
	t.Parallel()

	source, snapshot := derivationFixture(t)
	result, err := DeriveFork(source, snapshot, DerivationRequest{
		Identity: Identity{ActorID: 41}, NewRunID: "wbr_fork", At: source.UpdatedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("DeriveFork() error = %v", err)
	}
	assertFreshDerivedRun(t, result.Run, "wbr_fork")
	if result.Run.ForkedFromRunID != source.ID || result.Run.ReplayOfRunID != "" {
		t.Fatalf("fork lineage = fork %q replay %q", result.Run.ForkedFromRunID, result.Run.ReplayOfRunID)
	}
}

func derivationFixture(t *testing.T) (Run, SavedSnapshot) {
	t.Helper()
	actual, err := NewDecimal("0.0001")
	if err != nil {
		t.Fatalf("NewDecimal() error = %v", err)
	}
	now := time.Date(2026, time.September, 2, 9, 0, 0, 0, time.UTC)
	input := InputSnapshot{Intent: "summarize", CapabilityID: "cap_text", AcceptedQuoteID: "quote_1"}
	source := Run{
		ID: "wbr_source", OwnerID: 41, WorkspaceID: "wbw_owner", State: StateSucceeded, Input: input,
		ActualCost: &Money{Currency: "USD", Amount: actual}, Artifacts: []Artifact{{ArtifactID: "wba_old"}},
		CanonicalRequestID: "req_old", CanonicalUsageEventID: "usage_old", JournalID: "journal_old", MediaTaskID: "task_old",
		CreatedAt: now, UpdatedAt: now, Version: 4,
	}
	return source, SavedSnapshot{
		ID: "wbs_source", RunID: source.ID, OwnerID: source.OwnerID, WorkspaceID: source.WorkspaceID,
		Input: input, InputSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", CreatedAt: now,
	}
}

func assertFreshDerivedRun(t *testing.T, run Run, wantID RunID) {
	t.Helper()
	if run.ID != wantID || run.State != StateQueued || run.Version != 1 || run.NextEventSeq != 1 {
		t.Fatalf("derived run identity/state = %#v", run)
	}
	if run.ActualCost != nil || len(run.Artifacts) != 0 || run.CanonicalRequestID != "" || run.CanonicalUsageEventID != "" || run.JournalID != "" || run.MediaTaskID != "" {
		t.Fatalf("derived run copied charge/artifact identity = %#v", run)
	}
}
