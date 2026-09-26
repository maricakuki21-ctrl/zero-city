package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func sharedPoolProbeJobTestRows(now time.Time, status, errorType, errorMessage string, accountID any) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "operation_id", "pool_id", "account_id", "owner_id",
		"model_name", "upstream_model_name", "probe_type", "check_level", "config_version", "status",
		"attempt", "max_attempts", "lease_owner", "lease_expires_at", "heartbeat_at", "started_at",
		"finished_at", "result_summary", "error_type", "error_message", "created_at", "updated_at",
	}).AddRow(
		"00000000-0000-4000-8000-000000000001", "operation-1", int64(7), accountID, int64(19),
		"custom-model", "vendor/custom-model", "manual", "full", int64(3), status,
		1, 3, "worker-1", nil, nil, now, nil, []byte(`{}`), errorType, errorMessage, now, now,
	)
}

func TestCompleteSharedPoolProbeJobMarksOldConfigStaleWithoutApplyingReadiness(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	jobID := "00000000-0000-4000-8000-000000000001"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id::text, operation_id, pool_id, account_id, owner_id,[\s\S]+FROM shared_pool_probe_jobs WHERE id = \$1::uuid FOR UPDATE`).
		WithArgs(jobID).
		WillReturnRows(sharedPoolProbeJobTestRows(now, service.SharedPoolProbeJobRunning, "", "", nil))
	mock.ExpectQuery(`SELECT config_version, COALESCE\(governance_status, 'normal'\),`).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{"config_version", "governance_status", "governance_note"}).
			AddRow(int64(4), "normal", ""))
	mock.ExpectQuery(`UPDATE shared_pool_probe_jobs SET`).
		WillReturnRows(sharedPoolProbeJobTestRows(now, service.SharedPoolProbeJobStale, "stale_config", "pool configuration changed", nil))
	mock.ExpectCommit()

	completed, err := repo.CompleteSharedPoolProbeJob(context.Background(), service.SharedPoolProbeJobCompletion{
		JobID:          jobID,
		LeaseOwner:     "worker-1",
		TerminalStatus: service.SharedPoolProbeJobSucceeded,
		History: service.SharedPoolProbeHistoryInput{
			PoolID:            7,
			OwnerID:           19,
			ModelName:         "custom-model",
			UpstreamModelName: "vendor/custom-model",
			ProbeType:         "manual",
			Success:           true,
			CheckedAt:         now,
			Metadata: service.SharedPoolProbeMetadata{
				CheckLevel:      "full",
				GateRequired:    true,
				GatePassed:      true,
				FullCheckPassed: 15,
				FullCheckTotal:  15,
				FullCheckScore:  100,
			},
		},
		Result: &service.SharedPoolUpstreamProbeResult{
			OK: true, Model: "custom-model", CheckLevel: "full",
			GateRequired: true, GatePassed: true, FullCheckPassed: 15, FullCheckTotal: 15, FullCheckScore: 100,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, completed)
	require.Equal(t, service.SharedPoolProbeJobStale, completed.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCompleteSharedPoolProbeJobProjectsAccountReadinessAndAutoLists(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 8, 30, 0, 0, time.UTC)
	jobID := "00000000-0000-4000-8000-000000000001"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id::text, operation_id, pool_id, account_id, owner_id,[\s\S]+FROM shared_pool_probe_jobs WHERE id = \$1::uuid FOR UPDATE`).
		WithArgs(jobID).
		WillReturnRows(sharedPoolProbeJobTestRows(now, service.SharedPoolProbeJobRunning, "", "", int64(31)))
	mock.ExpectQuery(`SELECT config_version, COALESCE\(governance_status, 'normal'\),`).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{"config_version", "governance_status", "governance_note"}).
			AddRow(int64(3), "normal", ""))
	mock.ExpectExec(`INSERT INTO shared_pool_probe_histories`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE shared_pool_accounts SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT status, listed, COALESCE\(governance_status, 'normal'\)`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("testing", false, "normal", "", 0))
	mock.ExpectExec(`WITH eligible_accounts AS`).
		WithArgs(int64(7), sharedPoolObservationFailureThreshold, sharedPoolDelistFailureThreshold,
			sharedPoolObservationNote, sharedPoolDelistNote, sharedPoolFullCheckPassingScore).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT status, listed, COALESCE\(governance_status, 'normal'\)`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("healthy", true, "normal", "", 0))
	mock.ExpectExec(`UPDATE shared_pool_probe_histories history[\s\S]+SET config_version = pool.config_version`).
		WithArgs(jobID, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE shared_pool_probe_jobs SET`).
		WillReturnRows(sharedPoolProbeJobTestRows(now, service.SharedPoolProbeJobSucceeded, "", "", int64(31)))
	mock.ExpectCommit()

	completed, err := repo.CompleteSharedPoolProbeJob(context.Background(), service.SharedPoolProbeJobCompletion{
		JobID:          jobID,
		LeaseOwner:     "worker-1",
		TerminalStatus: service.SharedPoolProbeJobSucceeded,
		History: service.SharedPoolProbeHistoryInput{
			PoolID: 7, AccountID: 31, OwnerID: 19,
			ModelName: "custom-model", UpstreamModelName: "vendor/custom-model",
			ProbeType: "manual", Success: true, LatencyMs: 42, CheckedAt: now,
			Metadata: service.SharedPoolProbeMetadata{
				CheckLevel: "full", GateRequired: true, GatePassed: true,
				FullCheckPassed: 15, FullCheckTotal: 15, FullCheckScore: 100,
			},
		},
		Result: &service.SharedPoolUpstreamProbeResult{
			OK: true, Model: "custom-model", CheckLevel: "full",
			GateRequired: true, GatePassed: true, FullCheckPassed: 15, FullCheckTotal: 15, FullCheckScore: 100,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, completed)
	require.Equal(t, service.SharedPoolProbeJobSucceeded, completed.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimSharedPoolProbeJobsReclaimsExpiredLease(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE shared_pool_probe_jobs SET[\s\S]+status = 'timed_out'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`WITH candidates AS \(`).
		WithArgs(1, "worker-recovery", 240).
		WillReturnRows(sharedPoolProbeJobTestRows(now, service.SharedPoolProbeJobRunning, "", "", nil))
	mock.ExpectCommit()

	jobs, err := repo.ClaimSharedPoolProbeJobs(context.Background(), "worker-recovery", 1, 4*time.Minute)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, service.SharedPoolProbeJobRunning, jobs[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolProbeJobRequestMatchIncludesTargetAndModel(t *testing.T) {
	accountID := int64(31)
	job := &service.SharedPoolProbeJob{
		PoolID: 7, AccountID: &accountID, ModelName: "custom-model", ProbeType: "manual",
	}
	input := service.SharedPoolProbeJobEnqueueInput{
		PoolID: 7, AccountID: 31, ModelName: "custom-model", ProbeType: "manual",
	}
	require.True(t, sharedPoolProbeJobRequestMatches(job, input))
	input.ModelName = "different-model"
	require.False(t, sharedPoolProbeJobRequestMatches(job, input))
}
