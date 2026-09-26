package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchOperationPostgresEventFailureRollsBack(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewWorkbenchRuntimeRepository(db)
	current := workbenchTestRun("wbr_atomic", workbench.StateQueued, 1, 1)
	next := current.Clone()
	next.State = workbench.StateRunning
	claim := workbenchTestClaim(current.ID, "start-once", workbench.OperationCreate, "a")
	event := workbench.Event{ID: "wbe_atomic", RunID: current.ID, Kind: workbench.EventRunStarted, State: next.State, CreatedAt: current.UpdatedAt.Add(time.Second)}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(workbenchAdvisoryLockSQL)).WithArgs(workbenchOperationLockKey(claim)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).WithArgs(string(claim.RunID), claim.Key, int64(claim.OwnerID)).WillReturnRows(sqlmock.NewRows(workbenchOperationColumns))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchLockRunSQL)).WithArgs(string(current.ID), int64(current.OwnerID)).WillReturnRows(workbenchRunRows(current))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchClaimOperationSQL)).WithArgs(
		string(claim.ID), string(claim.RunID), int64(claim.OwnerID), claim.Key, string(claim.Kind), string(claim.Fingerprint),
	).WillReturnRows(sqlmock.NewRows([]string{"operation_id"}).AddRow(string(claim.ID)))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchUpdateRunSQL)).WillReturnRows(workbenchRunRows(workbenchTestRun("wbr_atomic", workbench.StateRunning, 2, 2)))
	mock.ExpectExec(regexp.QuoteMeta(workbenchInsertEventSQL)).WillReturnError(errors.New("event insert failed"))
	mock.ExpectRollback()

	// When
	_, err = repo.ApplyOperation(context.Background(), WorkbenchOperationRequest{
		Identity: workbench.Identity{ActorID: current.OwnerID}, Claim: claim, ExpectedVersion: 1, Run: next, Event: event,
	})

	// Then
	require.ErrorContains(t, err, "append workbench event")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchOperationPostgresSameFingerprintReplaysAndDifferentFingerprintConflicts(t *testing.T) {
	for _, test := range []struct {
		name         string
		requested    string
		wantConflict bool
	}{
		{name: "same fingerprint", requested: "b"},
		{name: "different fingerprint", requested: "c", wantConflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			run := workbenchTestRun("wbr_replay", workbench.StateRunning, 2, 2)
			existing := workbenchTestClaim(run.ID, "same-key", workbench.OperationCreate, "b")
			requested := workbenchTestClaim(run.ID, "same-key", workbench.OperationCreate, test.requested)
			requested.ID = "wbo_requested"

			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(workbenchAdvisoryLockSQL)).WithArgs(workbenchOperationLockKey(requested)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).WithArgs(string(run.ID), requested.Key, int64(run.OwnerID)).WillReturnRows(workbenchOperationRows(existing, workbench.OperationCommitted, workbench.OperationResult{RunID: run.ID, State: run.State}))
			if test.wantConflict {
				mock.ExpectRollback()
			} else {
				mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectRunSQL)).WithArgs(string(run.ID), int64(run.OwnerID)).WillReturnRows(workbenchRunRows(run))
				mock.ExpectCommit()
			}

			got, err := NewWorkbenchRuntimeRepository(db).ApplyOperation(context.Background(), WorkbenchOperationRequest{
				Identity: workbench.Identity{ActorID: run.OwnerID}, Claim: requested, ExpectedVersion: run.Version, Run: run,
			})
			if test.wantConflict {
				require.ErrorIs(t, err, workbench.ErrIdempotencyConflict)
			} else {
				require.NoError(t, err)
				require.True(t, got.Replayed)
				require.Equal(t, run.ID, got.Run.ID)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkbenchReplayPostgresConcurrentClaimReturnsExistingDerivedRun(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	source := workbenchTestRun("wbr_source", workbench.StateSucceeded, 4, 4)
	derived := workbenchTestRun("wbr_derived", workbench.StateQueued, 1, 2)
	derived.ReplayOfRunID = source.ID
	claim := workbenchTestClaim(source.ID, "replay-once", workbench.OperationReplay, "d")
	result := workbench.OperationResult{RunID: derived.ID, SnapshotID: "wbs_source", State: derived.State}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(workbenchAdvisoryLockSQL)).WithArgs(workbenchOperationLockKey(claim)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).WithArgs(string(source.ID), claim.Key, int64(source.OwnerID)).WillReturnRows(workbenchOperationRows(claim, workbench.OperationCommitted, result))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectRunSQL)).WithArgs(string(derived.ID), int64(source.OwnerID)).WillReturnRows(workbenchRunRows(derived))
	mock.ExpectCommit()

	// When
	got, err := NewWorkbenchRuntimeRepository(db).DeriveRun(context.Background(), WorkbenchDerivationRequest{
		Identity: workbench.Identity{ActorID: source.OwnerID}, Claim: claim, Run: derived,
		Derivation: workbench.Derivation{ID: "wbd_replay", OwnerID: source.OwnerID, SourceRunID: source.ID, SourceSnapshotID: "wbs_source", DerivedRunID: derived.ID, Kind: workbench.DerivationReplay},
	})

	// Then
	require.NoError(t, err)
	require.True(t, got.Replayed)
	require.Equal(t, derived.ID, got.Run.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchEventPostgresReadsOnlyAfterCursor(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	run := workbenchTestRun("wbr_events", workbench.StateRunning, 3, 3)
	payload := []byte(`{"id":"wbe_2","run_id":"wbr_events","seq":2,"kind":"run_started","state":"running","created_at":"2026-09-02T10:00:01Z"}`)
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectRunSQL)).WithArgs(string(run.ID), int64(run.OwnerID)).WillReturnRows(workbenchRunRows(run))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectEventsSQL)).WithArgs(string(run.ID), int64(run.OwnerID), uint64(1), 50).WillReturnRows(
		sqlmock.NewRows(workbenchEventColumns).AddRow(string(run.ID), int64(2), "wbe_2", "run_started", payload, time.Date(2026, 9, 2, 10, 0, 1, 0, time.UTC)),
	)

	// When
	got, err := NewWorkbenchRuntimeRepository(db).ReadEvents(context.Background(), workbench.Identity{ActorID: run.OwnerID}, run.ID, workbench.StreamCursor{RunID: run.ID, Seq: 1}, 50)

	// Then
	require.NoError(t, err)
	require.Len(t, got.Events, 1)
	require.Equal(t, uint64(2), got.Events[0].Seq)
	require.Contains(t, workbenchSelectEventsSQL, "event.seq > $3")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchArtifactOwnershipMismatchIsHidden(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectArtifactSQL)).WithArgs("wbr_private", int64(202), "wba_private").WillReturnRows(sqlmock.NewRows(workbenchArtifactColumns))

	// When
	_, err = NewWorkbenchRuntimeRepository(db).LoadArtifact(context.Background(), workbench.Identity{ActorID: 202}, "wbr_private", "wba_private")

	// Then
	require.ErrorIs(t, err, workbench.ErrArtifactNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchOwnershipRepositoryNeverWritesCanonicalUsageOrLedgerFacts(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, name := range []string{"workbench_runtime.go", "workbench_runtime_queries.go"} {
		body, err := os.ReadFile(filepath.Join(filepath.Dir(file), name))
		require.NoError(t, err)
		lower := strings.ToLower(string(body))
		for _, forbidden := range []string{"insert into canonical_usage_outbox", "insert into bizdecipher_ledger_outbox_receipts", "insert into bizdecipher_ledger_journals", "insert into bizdecipher_ledger_entries"} {
			require.NotContains(t, lower, forbidden)
		}
	}
}

func workbenchTestRun(id workbench.RunID, state workbench.RunState, version, nextSeq uint64) workbench.Run {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	return workbench.Run{ID: id, OwnerID: 101, WorkspaceID: "wbw_owner", State: state,
		Input: workbench.InputSnapshot{Intent: "hello", CapabilityID: "cap_text"}, InputSHA256: workbench.Digest(strings.Repeat("1", 64)),
		Version: version, NextEventSeq: nextSeq, Artifacts: []workbench.Artifact{}, CreatedAt: now, UpdatedAt: now}
}

func workbenchTestClaim(runID workbench.RunID, key string, kind workbench.OperationKind, digit string) workbench.OperationClaim {
	return workbench.OperationClaim{ID: workbench.OperationID("wbo_" + digit), OwnerID: 101, RunID: runID, Key: key, Kind: kind, Fingerprint: workbench.RequestFingerprint(strings.Repeat(digit, 64))}
}

func workbenchRunRows(run workbench.Run) *sqlmock.Rows {
	input, _ := marshalWorkbenchJSON(run.Input)
	return sqlmock.NewRows(workbenchRunColumns).AddRow(string(run.ID), int64(run.OwnerID), string(run.WorkspaceID), string(run.Input.CapabilityID), input,
		string(run.InputSHA256), string(run.State), int64(run.Version), int64(run.NextEventSeq), workbenchNullable(string(run.ReplayOfRunID)), workbenchNullable(string(run.ForkedFromRunID)),
		workbenchNullable(string(run.AcceptedQuoteID)), workbenchNullable(run.CanonicalRequestID), workbenchNullable(run.CanonicalUsageEventID), workbenchNullable(run.JournalID), workbenchNullable(run.CanonicalMediaBusinessEventID),
		nil, nil, nil, run.CreatedAt, run.UpdatedAt, run.TerminalAt)
}

func workbenchOperationRows(claim workbench.OperationClaim, state workbench.OperationState, result workbench.OperationResult) *sqlmock.Rows {
	return sqlmock.NewRows(workbenchOperationColumns).AddRow(string(claim.ID), string(claim.RunID), int64(claim.OwnerID), claim.Key, string(claim.Kind), string(claim.Fingerprint),
		string(state), workbenchNullable(string(result.RunID)), workbenchNullable(string(result.SnapshotID)), workbenchNullable(string(result.State)))
}

func workbenchNullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
