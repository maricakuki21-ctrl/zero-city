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

func TestWorkbenchForkReusesSameInputSnapshot(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		name := "existing"
		if concurrent {
			name = "concurrent_insert"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			source := workbenchTestRun("wbr_source", workbench.StateSucceeded, 3, 3)
			snapshot := workbench.SavedSnapshot{ID: "wbs_original", RunID: source.ID, OwnerID: source.OwnerID,
				WorkspaceID: source.WorkspaceID, Input: source.Input, InputSHA256: source.InputSHA256, CreatedAt: time.Now()}
			payload, err := marshalWorkbenchJSON(snapshot)
			require.NoError(t, err)
			columns := []string{"snapshot_id", "run_id", "owner_user_id", "snapshot", "snapshot_sha256", "created_at"}
			if concurrent {
				mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectForkSnapshotSQL)).
					WithArgs(string(source.ID), string(source.InputSHA256), int64(source.OwnerID)).
					WillReturnRows(sqlmock.NewRows(columns))
				mock.ExpectQuery(regexp.QuoteMeta(workbenchInsertSnapshotSQL)).
					WillReturnRows(sqlmock.NewRows(columns))
			}
			mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectForkSnapshotSQL)).
				WithArgs(string(source.ID), string(source.InputSHA256), int64(source.OwnerID)).
				WillReturnRows(sqlmock.NewRows(columns).AddRow(string(snapshot.ID), string(source.ID),
					int64(source.OwnerID), payload, string(source.InputSHA256), snapshot.CreatedAt))
			store := &workbenchRuntimeStore{repo: NewWorkbenchRuntimeRepository(db)}
			got, err := store.ensureForkSnapshot(context.Background(), workbench.Identity{ActorID: source.OwnerID},
				source, workbenchTestClaim(source.ID, "fork-once", workbench.OperationFork, "a"), time.Now())
			require.NoError(t, err)
			require.Equal(t, snapshot.ID, got.ID)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkbenchCancelRetryReadsOperationBeforeStateTransition(t *testing.T) {
	for _, state := range []workbench.RunState{workbench.StateCancelRequested, workbench.StateCancelled} {
		t.Run(string(state), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			run := workbenchTestRun("wbr_cancel_retry", state, 4, 4)
			claim := workbenchTestClaim(run.ID, "cancel-once", workbench.OperationCancel, "a")
			mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).
				WithArgs(string(run.ID), claim.Key, int64(run.OwnerID)).
				WillReturnRows(workbenchOperationRows(claim, workbench.OperationCommitted, workbench.OperationResult{RunID: run.ID, State: workbench.StateCancelRequested}))
			mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectRunSQL)).
				WithArgs(string(run.ID), int64(run.OwnerID)).WillReturnRows(workbenchRunRows(run))
			mock.ExpectQuery("SELECT artifact_id,run_id,owner_user_id").WithArgs(string(run.ID), int64(run.OwnerID)).
				WillReturnRows(sqlmock.NewRows(workbenchArtifactColumns))
			store := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db))
			got, err := store.RequestCancel(context.Background(), service.WorkbenchCancelRequest{
				Identity: workbench.Identity{ActorID: run.OwnerID}, RunID: run.ID, Claim: claim,
			})
			require.NoError(t, err)
			require.True(t, got.Replayed)
			require.Equal(t, state, got.Run.State)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkbenchCancelRetryRejectsChangedFingerprintBeforeReadingRun(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	claim := workbenchTestClaim("wbr_cancel_retry", "cancel-once", workbench.OperationCancel, "a")
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).
		WillReturnRows(workbenchOperationRows(claim, workbench.OperationCommitted, workbench.OperationResult{RunID: claim.RunID}))
	changed := claim
	changed.Fingerprint = workbenchTestClaim(claim.RunID, claim.Key, claim.Kind, "b").Fingerprint
	_, err = ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db)).RequestCancel(context.Background(), service.WorkbenchCancelRequest{
		Identity: workbench.Identity{ActorID: claim.OwnerID}, RunID: claim.RunID, Claim: changed,
	})
	require.ErrorIs(t, err, workbench.ErrIdempotencyConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchForkRetryReturnsCommittedSnapshotWithoutCreatingAnother(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	source := workbenchTestRun("wbr_source", workbench.StateSucceeded, 3, 3)
	derived := workbenchTestRun("wbr_original_fork", workbench.StateSucceeded, 3, 3)
	claim := workbenchTestClaim(source.ID, "fork-once", workbench.OperationFork, "a")
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectRunSQL)).
		WithArgs(string(source.ID), int64(source.OwnerID)).WillReturnRows(workbenchRunRows(source))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectOperationSQL)).
		WithArgs(string(source.ID), claim.Key, int64(source.OwnerID)).
		WillReturnRows(workbenchOperationRows(claim, workbench.OperationCommitted, workbench.OperationResult{RunID: derived.ID, SnapshotID: "wbs_committed"}))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectRunSQL)).
		WithArgs(string(derived.ID), int64(source.OwnerID)).WillReturnRows(workbenchRunRows(derived))
	mock.ExpectQuery("SELECT artifact_id,run_id,owner_user_id").WithArgs(string(derived.ID), int64(source.OwnerID)).
		WillReturnRows(sqlmock.NewRows(workbenchArtifactColumns))
	got, err := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db)).DeriveRun(context.Background(), service.WorkbenchDeriveRequest{
		Identity: workbench.Identity{ActorID: source.OwnerID}, Claim: claim, SourceRunID: source.ID,
		NewRunID: "wbr_unused", Kind: workbench.DerivationFork, Run: source,
	})
	require.NoError(t, err)
	require.True(t, got.Replayed)
	require.Equal(t, derived.ID, got.Run.ID)
	require.Equal(t, workbench.SnapshotID("wbs_committed"), got.SourceSnapshotID)
	require.NoError(t, mock.ExpectationsWereMet())
}
