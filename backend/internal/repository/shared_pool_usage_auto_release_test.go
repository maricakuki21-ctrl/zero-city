package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAutoReleaseAgedSharedPoolUsageReviewRestoresBalanceAndAuditState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	cutoff := time.Date(2026, 7, 25, 18, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id, user_id, hold_amount, failure_reason.*status = 'review_required'.*FOR UPDATE SKIP LOCKED`).
		WithArgs(cutoff, 0.01, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "hold_amount", "failure_reason"}).
			AddRow(int64(17), int64(44), 0.000761, "upstream_timeout_result_unknown"))
	mock.ExpectExec(`(?s)UPDATE users.*balance = balance \+ \$1.*frozen_balance`).
		WithArgs(0.000761, int64(44)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE shared_pool_usage_reservations.*status = 'released'.*status = 'review_required'`).
		WithArgs(int64(17), "auto_released_low_value_after_grace:upstream_timeout_result_unknown").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE shared_pool_usage_traces.*settlement_outcome = 'released'`).
		WithArgs(int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE shared_pool_media_tasks.*status = 'failed'`).
		WithArgs(int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	summary, err := repo.AutoReleaseAgedSharedPoolUsageReviewsTx(context.Background(), cutoff, 0.01, 100)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Scanned)
	require.Equal(t, 1, summary.Released)
	require.Zero(t, summary.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAutoReleaseAgedSharedPoolUsageReviewLeavesMismatchForManualAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	cutoff := time.Date(2026, 7, 25, 18, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id, user_id, hold_amount, failure_reason.*FOR UPDATE SKIP LOCKED`).
		WithArgs(cutoff, 0.01, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "hold_amount", "failure_reason"}).
			AddRow(int64(18), int64(45), 0.005, "upstream_result_unknown"))
	mock.ExpectExec(`(?s)UPDATE users.*frozen_balance`).
		WithArgs(0.005, int64(45)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`(?s)UPDATE shared_pool_usage_reservations.*failure_reason = \$2.*status = 'review_required'`).
		WithArgs(int64(18), "auto_release_blocked_balance_mismatch:upstream_result_unknown").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	summary, err := repo.AutoReleaseAgedSharedPoolUsageReviewsTx(context.Background(), cutoff, 0.01, 100)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Scanned)
	require.Zero(t, summary.Released)
	require.Equal(t, 1, summary.Failed)
	require.NoError(t, mock.ExpectationsWereMet())
}
