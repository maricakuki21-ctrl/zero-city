package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPrepareSharedPoolUsageReviewReleaseIsAtomicAndAudited(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	input := service.ResolveSharedPoolUsageReviewInput{
		ReservationID: 17, AdminUserID: 9, Action: service.SharedPoolUsageReviewActionRelease,
		Note: "确认上游没有接受请求", OperationID: "review-release-17",
	}
	now := time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectNoSharedPoolUsageReviewOperation(mock, input.OperationID)
	expectSharedPoolUsageReviewForUpdate(mock, input.ReservationID, 1.25, now)
	mock.ExpectQuery(`(?s)INSERT INTO shared_pool_usage_review_resolutions.*RETURNING id, created_at`).
		WithArgs(input.ReservationID, input.AdminUserID, input.OperationID, input.Action, 0.0, "forward_result_missing_after_timeout", input.Note).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(71, now))
	mock.ExpectExec(`(?s)UPDATE users.*frozen_balance.*WHERE id = \$2`).
		WithArgs(1.25, int64(44)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE shared_pool_usage_reservations.*status = 'released'.*status = 'review_required'`).
		WithArgs(input.ReservationID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	prepared, err := repo.PrepareSharedPoolUsageReviewResolutionTx(context.Background(), input)
	require.NoError(t, err)
	require.Nil(t, prepared.Usage)
	require.Equal(t, "released", prepared.Review.ReservationStatus)
	require.Equal(t, int64(71), prepared.Review.ResolutionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrepareSharedPoolUsageReviewRejectsRetiredChargeActionsBeforeTransaction(t *testing.T) {
	for _, action := range []string{
		service.SharedPoolUsageReviewActionCaptureHold,
		service.SharedPoolUsageReviewActionSettleAmount,
	} {
		t.Run(action, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			repo := &bizDecipherRepository{db: db}

			_, err = repo.PrepareSharedPoolUsageReviewResolutionTx(context.Background(), service.ResolveSharedPoolUsageReviewInput{
				ReservationID: 17, AdminUserID: 9, Action: action,
				Note: "旧冻结记录不能直接产生消费", OperationID: "review-charge-disabled",
			})

			require.ErrorContains(t, err, "retired")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func expectNoSharedPoolUsageReviewOperation(mock sqlmock.Sqlmock, operationID string) {
	mock.ExpectQuery(`(?s)FROM shared_pool_usage_review_resolutions rr.*WHERE rr.operation_id = \$1`).
		WithArgs(operationID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
}

func expectSharedPoolUsageReviewForUpdate(mock sqlmock.Sqlmock, reservationID int64, hold float64, now time.Time) {
	columns := []string{
		"id", "request_id", "access_key_id", "pool_id", "pool_name", "account_id", "user_id", "user_label",
		"model_snapshot", "endpoint_type", "pricing_source_snapshot", "hold_amount", "reported_amount", "settled_amount", "status",
		"failure_reason", "reserved_at", "forward_started_at", "expires_at", "finalized_at", "price_version_id", "price_snapshot",
	}
	mock.ExpectQuery(`(?s)FROM shared_pool_usage_reservations r.*WHERE r.id = \$1.*FOR UPDATE OF r`).
		WithArgs(reservationID).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			reservationID, "review-request-17", 11, 22, "稳定共享池", 33, 44, "用户 #44",
			"gpt-test", "responses", service.SharedPoolPricingSourceOfficial,
			hold, 0.0, 0.0, "review_required", "forward_result_missing_after_timeout",
			now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour), now,
			55, []byte(`{"price_version_id":55}`),
		))
}
