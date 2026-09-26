package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchRuntimeStore_LoadWorkspace_synthesizesOwnerWorkspace(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	store := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db))
	mock.ExpectQuery("SELECT run_id FROM workbench_runs").WithArgs(int64(41)).
		WillReturnRows(sqlmock.NewRows([]string{"run_id"}))
	mock.ExpectQuery("SELECT snapshot_id,run_id,owner_user_id,snapshot,snapshot_sha256,created_at").
		WithArgs(int64(41)).WillReturnRows(sqlmock.NewRows([]string{"snapshot_id", "run_id", "owner_user_id", "snapshot", "snapshot_sha256", "created_at"}))

	// When
	workspace, err := store.LoadWorkspace(context.Background(), workbench.Identity{ActorID: 41})

	// Then
	require.NoError(t, err)
	require.Equal(t, workbench.WorkspaceID("wbw_41"), workspace.ID)
	require.Equal(t, workbench.ActorID(41), workspace.OwnerID)
	require.Empty(t, workspace.SavedSnapshots)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchRuntimeStore_ClaimOperation_createsRun(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	run := workbenchTestRun("wbr_create", workbench.StateQueued, 1, 1)
	claim := workbenchTestClaim(run.ID, "launch-once", workbench.OperationCreate, "a")
	event := workbench.Event{ID: "wbe_create", RunID: run.ID, Kind: workbench.EventRunQueued, State: run.State, CreatedAt: run.CreatedAt}
	identity := workbench.Identity{ActorID: run.OwnerID}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(workbenchAdvisoryLockSQL)).WithArgs(workbenchOperationLockKey(claim)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).WithArgs(string(claim.RunID), claim.Key, int64(claim.OwnerID)).WillReturnRows(sqlmock.NewRows(workbenchOperationColumns))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchInsertRunSQL)).WillReturnRows(workbenchRunRows(run))
	mock.ExpectExec(regexp.QuoteMeta(workbenchInsertEventSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(workbenchInsertCommittedOperationSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// When
	got, err := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db)).ClaimOperation(context.Background(), service.WorkbenchOperationRequest{
		Identity: identity, Claim: claim, Run: run, Event: event,
	})

	// Then
	require.NoError(t, err)
	require.False(t, got.Replayed)
	require.Equal(t, run.ID, got.Run.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchRuntimeStore_TransitionRun_usesStoredVersionBeforeMutation(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	current := workbenchTestRun("wbr_cas", workbench.StateQueued, 1, 1)
	next, err := workbench.ApplyTransition(current, workbench.Transition{To: workbench.StateRunning, At: current.UpdatedAt.Add(time.Second)})
	require.NoError(t, err)
	require.Equal(t, uint64(2), next.Version)
	event := workbench.Event{ID: "wbe_cas", RunID: next.ID, Kind: workbench.EventRunStarted, State: next.State, CreatedAt: next.UpdatedAt}
	identity := workbench.Identity{ActorID: current.OwnerID}
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).WillReturnRows(sqlmock.NewRows(workbenchOperationColumns))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchLockRunSQL)).WithArgs(string(current.ID), int64(current.OwnerID)).WillReturnRows(workbenchRunRows(current))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchClaimOperationSQL)).WillReturnRows(sqlmock.NewRows([]string{"operation_id"}).AddRow("wbo_cas"))
	updated := workbenchTestRun("wbr_cas", workbench.StateRunning, 2, 2)
	mock.ExpectQuery(regexp.QuoteMeta(workbenchUpdateRunSQL)).WillReturnRows(workbenchRunRows(updated))
	mock.ExpectExec(regexp.QuoteMeta(workbenchInsertEventSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(workbenchCommitOperationSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// When
	got, err := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db)).TransitionRun(context.Background(), service.WorkbenchRunMutation{
		Identity: identity, Run: next, Event: event,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, workbench.StateRunning, got.State)
	require.Equal(t, uint64(2), got.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchRuntimeStore_ReadEvents_returnsRawEventRead(t *testing.T) {
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
	got, err := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db)).ReadEvents(
		context.Background(), workbench.Identity{ActorID: run.OwnerID}, run.ID, workbench.StreamCursor{RunID: run.ID, Seq: 1}, 50,
	)

	// Then
	require.NoError(t, err)
	require.Equal(t, run.ID, got.Run.ID)
	require.Equal(t, uint64(1), got.OldestAvailableSeq)
	require.Len(t, got.Events, 1)
	require.Equal(t, uint64(2), got.Events[0].Seq)
	require.NoError(t, mock.ExpectationsWereMet())
}
