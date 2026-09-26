package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPoolProbeCompletionPathsUsePersistedAvailability(t *testing.T) {
	for _, job := range []bool{false, true} {
		for _, success := range []bool{false, true} {
			name := "direct/failure"
			if job {
				name = "job/failure"
			}
			if success {
				name += "/success"
			}
			t.Run(name, func(t *testing.T) {
				// Match the real query emitted by each branch, including the shared
				// aggregate and the guard preventing legacy writes to native pools.
				db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
				require.NoError(t, err)
				defer db.Close()
				history := service.SharedPoolProbeHistoryInput{
					PoolID: 7, Success: success, HTTPStatus: 200,
					ProbeType: "scheduled", CheckedAt: time.Now(),
				}
				if !success {
					history.HTTPStatus = 500
				}
				query := `UPDATE shared_pools SET[\s\S]+` +
					regexp.QuoteMeta(sharedPoolProbeAvailabilityAssignments) +
					`[\s\S]+WHERE id = \$1 AND native_onboarding_state = 'legacy_existing' AND deleted_at IS NULL`
				if job {
					mock.ExpectBegin()
					tx, err := db.Begin()
					require.NoError(t, err)
					mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectRollback()
					require.NoError(t, applySharedPoolProbeJobPoolResultTx(context.Background(), tx, history, false))
					require.NoError(t, tx.Rollback())
				} else {
					mock.ExpectQuery("SELECT status, listed").
						WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
							AddRow("healthy", true, "normal", "", 0))
					mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(0, 1))
					require.NoError(t, (&bizDecipherRepository{db: db}).ApplySharedPoolProbeResult(context.Background(), history))
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestPoolProbeGuardedUpdateDoesNotInventGovernanceLog(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "failure"}[success], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery("SELECT status, listed").
				WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
					AddRow("offline", true, "watch", sharedPoolDelistNote, 9))
			mock.ExpectExec("UPDATE shared_pools SET").WillReturnResult(sqlmock.NewResult(0, 0))
			require.NoError(t, (&bizDecipherRepository{db: db}).ApplySharedPoolProbeResult(context.Background(), service.SharedPoolProbeHistoryInput{
				PoolID: 7, Success: success, HTTPStatus: 500,
			}))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPoolProbeRecoveryAuditUsesPersistedPauseState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT status, listed").
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("maintenance", false, "watch", sharedPoolDelistNote, 9))
	mock.ExpectExec("UPDATE shared_pools SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT status, listed").
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("maintenance", false, "normal", "", 0))
	before := string(jsonBytes(map[string]any{
		"status": "maintenance", "listed": false, "governance_status": "watch",
		"status_note": sharedPoolDelistNote, "consecutive_probe_failures": 9,
	}))
	after := string(jsonBytes(map[string]any{
		"status": "maintenance", "listed": false, "governance_status": "normal",
		"status_note": "", "consecutive_probe_failures": 0,
	}))
	mock.ExpectExec("INSERT INTO shared_pool_governance_logs").
		WithArgs(int64(7), before, after, "探测恢复成功，已退出观察期").
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, (&bizDecipherRepository{db: db}).ApplySharedPoolProbeResult(context.Background(), service.SharedPoolProbeHistoryInput{
		PoolID: 7, Success: true, ProbeType: "manual",
		Metadata: service.SharedPoolProbeMetadata{GatePassed: true, FullCheckScore: 100},
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}
