package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestReserveSharedPoolUsageTxAtomicallyFreezesBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	expires := now.Add(2 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, request_fingerprint`).
		WithArgs(int64(11), "sp:11:req").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO shared_pool_usage_reservations`).
		WithArgs(
			"sp:11:req", reservationFingerprintForTest, int64(11), int64(22), int64(33),
			int64(44), int64(55), "chat", "gpt-test", service.SharedPoolPricingSourceOwner,
			jsonTextArg(`{"price_version_id":55}`), 1.25, expires,
		).
		WillReturnRows(sharedPoolReservationRows().AddRow(
			int64(1), "sp:11:req", reservationFingerprintForTest, int64(11), int64(22),
			int64(33), int64(44), int64(55), 1.25, 0, 0, "reserved", now, expires,
		))
	mock.ExpectQuery(`UPDATE users(?s).*frozen_balance`).
		WithArgs(1.25, int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(8.75, 1.25))
	mock.ExpectCommit()

	got, err := repo.ReserveSharedPoolUsageTx(context.Background(), service.SharedPoolUsageReservationInput{
		RequestID: "sp:11:req", RequestFingerprint: reservationFingerprintForTest,
		AccessKeyID: 11, PoolID: 22, AccountID: 33, UserID: 44, PriceVersionID: 55,
		EndpointType: "chat", Model: "gpt-test", PricingSource: service.SharedPoolPricingSourceOwner,
		PriceSnapshot: []byte(`{"price_version_id":55}`), HoldAmount: 1.25, ExpiresAt: expires,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), got.ID)
	require.InDelta(t, 1.25, got.HoldAmount, 1e-12)
	require.Equal(t, "reserved", got.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveSharedPoolUsageTxNormalizesEmptySnapshotBeforeReplayValidation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	expires := now.Add(2 * time.Hour)
	input := service.SharedPoolUsageReservationInput{
		RequestID: "sp:11:empty-snapshot-replay", RequestFingerprint: reservationFingerprintForTest,
		AccessKeyID: 11, PoolID: 22, AccountID: 33, UserID: 44, PriceVersionID: 55,
		EndpointType: "chat", Model: "gpt-test", PricingSource: service.SharedPoolPricingSourceOwner,
		HoldAmount: 1.25, ExpiresAt: expires,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, request_fingerprint`).
		WithArgs(input.AccessKeyID, input.RequestID).
		WillReturnRows(lockedSharedPoolReservationRows().AddRow(
			int64(1), input.RequestID, input.RequestFingerprint, input.AccessKeyID, input.PoolID,
			input.AccountID, input.UserID, input.PriceVersionID, input.EndpointType, input.Model,
			input.PricingSource, []byte(`{}`), input.HoldAmount, 0.0, 0.0, "reserved", now, expires,
		))
	mock.ExpectRollback()

	_, err = repo.ReserveSharedPoolUsageTx(context.Background(), input)
	require.ErrorIs(t, err, service.ErrSharedPoolReservationFinalized)
	require.NotErrorIs(t, err, service.ErrSharedPoolReservationConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureSharedPoolUsageReservationRefundsDifferenceAndCapsCharge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	mock.ExpectQuery(`UPDATE users(?s).*balance = balance \+ \(\$1 - \$2\)`).
		WithArgs(2.0, 2.0, int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8.0))
	mock.ExpectExec(`UPDATE shared_pool_usage_reservations`).
		WithArgs(int64(7), 3.5, 2.0, "reported_cost_exceeded_hold").
		WillReturnResult(sqlmock.NewResult(0, 1))

	settled, balance, capped, err := captureSharedPoolUsageReservationTx(
		context.Background(),
		tx,
		&lockedSharedPoolUsageReservation{ID: 7, UserID: 44, HoldAmount: 2, Status: "forwarding"},
		3.5,
	)
	require.NoError(t, err)
	require.InDelta(t, 2, settled, 1e-12)
	require.InDelta(t, 8, balance, 1e-12)
	require.True(t, capped)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseSharedPoolUsageReservationRestoresEntireHold(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	mock.ExpectExec(`UPDATE users(?s).*balance = balance \+ \$1`).
		WithArgs(1.5, int64(44)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_usage_reservations`).
		WithArgs(int64(8), "upstream_request_failed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = releaseSharedPoolUsageReservationTx(
		context.Background(),
		tx,
		&lockedSharedPoolUsageReservation{ID: 8, UserID: 44, HoldAmount: 1.5, Status: "reserved"},
		"upstream_request_failed",
	)
	require.NoError(t, err)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseSharedPoolUsageReservationRejectsForwardingReservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = releaseSharedPoolUsageReservationTx(
		context.Background(),
		tx,
		&lockedSharedPoolUsageReservation{ID: 8, UserID: 44, HoldAmount: 1.5, Status: "forwarding"},
		"upstream_request_failed",
	)
	require.ErrorIs(t, err, service.ErrSharedPoolReservationFinalized)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSharedPoolReservationReplayRejectsFrozenMetadataMismatch(t *testing.T) {
	frozenSnapshot := []byte(`{"price_version_id":55,"multiplier":1}`)
	existing := lockedSharedPoolUsageReservation{
		RequestFingerprint: reservationFingerprintForTest,
		AccessKeyID:        11,
		PoolID:             22,
		AccountID:          sql.NullInt64{Int64: 33, Valid: true},
		UserID:             44,
		PriceVersionID:     sql.NullInt64{Int64: 55, Valid: true},
		EndpointType:       "responses",
		Model:              "gpt-test",
		PricingSource:      service.SharedPoolPricingSourceOwner,
		PriceSnapshot:      frozenSnapshot,
		HoldAmount:         1.25,
	}
	baseInput := service.SharedPoolUsageReservationInput{
		RequestFingerprint: reservationFingerprintForTest,
		AccessKeyID:        11,
		PoolID:             22,
		AccountID:          33,
		UserID:             44,
		PriceVersionID:     55,
		EndpointType:       "responses",
		Model:              "gpt-test",
		PricingSource:      service.SharedPoolPricingSourceOwner,
		PriceSnapshot:      json.RawMessage(`{"multiplier":1.0,"price_version_id":55}`),
		HoldAmount:         1.25,
	}
	require.NoError(t, validateSharedPoolReservationReplay(&existing, baseInput))

	tests := []struct {
		name   string
		mutate func(*service.SharedPoolUsageReservationInput)
	}{
		{name: "account", mutate: func(input *service.SharedPoolUsageReservationInput) { input.AccountID = 34 }},
		{name: "endpoint", mutate: func(input *service.SharedPoolUsageReservationInput) { input.EndpointType = "chat" }},
		{name: "model", mutate: func(input *service.SharedPoolUsageReservationInput) { input.Model = "other-model" }},
		{name: "pricing source", mutate: func(input *service.SharedPoolUsageReservationInput) {
			input.PricingSource = service.SharedPoolPricingSourceOfficial
		}},
		{name: "price snapshot", mutate: func(input *service.SharedPoolUsageReservationInput) {
			input.PriceSnapshot = json.RawMessage(`{"price_version_id":55,"multiplier":2}`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := baseInput
			tt.mutate(&input)
			require.ErrorIs(t, validateSharedPoolReservationReplay(&existing, input), service.ErrSharedPoolReservationConflict)
		})
	}
}

const reservationFingerprintForTest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

var sharedPoolReservationReservedAtForTest = time.Date(2026, 7, 19, 4, 30, 0, 0, time.UTC)

func sharedPoolReservationRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "request_id", "request_fingerprint", "access_key_id", "pool_id",
		"account_id", "user_id", "price_version_id", "hold_amount",
		"reported_amount", "settled_amount", "status", "reserved_at", "expires_at",
	})
}

func TestMarkSharedPoolUsageForwardingTxTransitionsReservedReservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectExec(`UPDATE shared_pool_usage_reservations(?s).*status = 'forwarding'.*status = 'reserved'`).
		WithArgs(int64(11), "sp:11:mark-forwarding").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.MarkSharedPoolUsageForwardingTx(context.Background(), 11, "sp:11:mark-forwarding")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkSharedPoolUsageReviewRequiredTxKeepsUnknownForwardFrozen(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectExec(`UPDATE shared_pool_usage_reservations(?s).*status = 'review_required'.*status = 'forwarding'`).
		WithArgs(int64(11), "sp:11:unknown", "upstream_timeout_result_unknown").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.MarkSharedPoolUsageReviewRequiredTx(context.Background(), 11, "sp:11:unknown", "upstream_timeout_result_unknown")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseSharedPoolUsageAfterVerifiedHTTPFailureRestoresHoldAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	input := sharedPoolPendingUsageInput("sp:11:http-failure")
	input.AccountID = 33
	input.Model = "gpt-test"
	reason := "upstream_http_502_no_success"

	mock.ExpectBegin()
	expectSharedPoolReservationLookupWithFrozen(
		mock, input, "forwarding", 1.25,
		input.AccountID, input.PriceVersionID, input.Model, input.PricingSource, input.PriceSnapshot,
	)
	mock.ExpectExec(`UPDATE users(?s).*balance = balance \+ \$1.*frozen_balance`).
		WithArgs(1.25, input.UserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_usage_reservations(?s).*status = 'released'.*status = 'forwarding'`).
		WithArgs(int64(7), reason).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.ReleaseSharedPoolUsageAfterVerifiedFailureTx(context.Background(), input.AccessKeyID, input.RequestID, reason)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStageSharedPoolUsageSettlementTxTransitionsForwardingToPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	input := sharedPoolPendingUsageInput("sp:11:stage-pending")
	expectedPayload, err := json.Marshal(input)
	require.NoError(t, err)
	expectedQuery := `
		UPDATE shared_pool_usage_reservations
		SET status = 'settlement_pending',
		    reported_amount = $3,
		    usage_payload = CASE WHEN status = 'forwarding' THEN $4::jsonb ELSE usage_payload END,
		    settlement_staged_at = COALESCE(settlement_staged_at, NOW()),
		    updated_at = NOW()
		WHERE access_key_id = $1
		  AND request_id = $2
		  AND (
		      status = 'forwarding'
		      OR (status = 'settlement_pending' AND usage_payload = $4::jsonb)
		  )`

	mock.ExpectExec(regexp.QuoteMeta(expectedQuery)).
		WithArgs(input.AccessKeyID, input.RequestID, input.Cost, jsonTextArg(expectedPayload)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.StageSharedPoolUsageSettlementTx(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordSharedPoolUsageTxRejectsSuccessfulReservedReservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	input := sharedPoolPendingUsageInput("sp:11:reserved-success")

	mock.ExpectBegin()
	expectSharedPoolReservationLookup(mock, input, "reserved", 0)
	mock.ExpectRollback()

	err = repo.RecordSharedPoolUsageTx(context.Background(), input)
	require.ErrorContains(t, err, "not marked forwarding")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordSharedPoolUsageTxRejectsBlankSuccessfulRequestIDBeforeTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	input := sharedPoolPendingUsageInput(" \t\r\n ")

	err = repo.RecordSharedPoolUsageTx(context.Background(), input)
	require.ErrorContains(t, err, "requires a request id")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordSharedPoolUsageTxRejectsFrozenReservationMetadataMismatchBeforeWrites(t *testing.T) {
	frozenSnapshot := json.RawMessage(`{"price_version_id":55,"multiplier":1}`)
	tests := []struct {
		name   string
		mutate func(*service.SharedPoolUsageInput)
	}{
		{name: "account", mutate: func(input *service.SharedPoolUsageInput) { input.AccountID = 34 }},
		{name: "price version", mutate: func(input *service.SharedPoolUsageInput) { input.PriceVersionID = 56 }},
		{name: "pricing source", mutate: func(input *service.SharedPoolUsageInput) {
			input.PricingSource = service.SharedPoolPricingSourceOfficial
		}},
		{name: "model", mutate: func(input *service.SharedPoolUsageInput) { input.Model = "other-model" }},
		{name: "price snapshot", mutate: func(input *service.SharedPoolUsageInput) {
			input.PriceSnapshot = json.RawMessage(`{"price_version_id":55,"multiplier":2}`)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &bizDecipherRepository{db: db}
			input := sharedPoolPendingUsageInput("sp:11:frozen-mismatch-" + tt.name)
			input.AccountID = 33
			input.Model = "gpt-test"
			input.PriceSnapshot = append(json.RawMessage(nil), frozenSnapshot...)
			tt.mutate(&input)

			mock.ExpectBegin()
			expectSharedPoolReservationLookupWithFrozen(
				mock, input, "forwarding", 1.25,
				int64(33), int64(55), "gpt-test", service.SharedPoolPricingSourceOwner, frozenSnapshot,
			)
			mock.ExpectRollback()

			err = repo.RecordSharedPoolUsageTx(context.Background(), input)
			require.ErrorIs(t, err, service.ErrSharedPoolReservationConflict)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRecordSharedPoolUsageTxFailsClosedOnUnreservedLedgerIDConflict(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*service.SharedPoolUsageInput)
	}{
		{name: "exact legacy replay", mutate: func(*service.SharedPoolUsageInput) {}},
		{name: "changed amount", mutate: func(input *service.SharedPoolUsageInput) { input.Cost = 1.50 }},
		{name: "changed pool", mutate: func(input *service.SharedPoolUsageInput) { input.PoolID = 23 }},
		{name: "changed account", mutate: func(input *service.SharedPoolUsageInput) { input.AccountID = 34 }},
		{name: "changed price", mutate: func(input *service.SharedPoolUsageInput) {
			input.PriceVersionID = 56
			input.PriceSnapshot = json.RawMessage(`{"price_version_id":56}`)
		}},
		{name: "missing snapshot defaults to empty object", mutate: func(input *service.SharedPoolUsageInput) {
			input.PriceSnapshot = nil
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &bizDecipherRepository{db: db}
			input := sharedPoolPendingUsageInput("sp:11:legacy-conflict")
			input.AccountID = 33
			input.Cost = 1.25
			tt.mutate(&input)
			expectedSnapshot := string(input.PriceSnapshot)
			if expectedSnapshot == "" {
				expectedSnapshot = `{}`
			}

			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT id, request_id, request_fingerprint`).
				WithArgs(input.AccessKeyID, input.RequestID).
				WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(`SELECT owner_id, platform_fee_percent FROM shared_pools`).
				WithArgs(input.PoolID).
				WillReturnRows(sqlmock.NewRows([]string{"owner_id", "platform_fee_percent"}).AddRow(nil, 10.0))
			mock.ExpectQuery(`SELECT hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from`).
				WithArgs(input.PoolID, sqlmock.AnyArg()).
				WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(`INSERT INTO shared_pool_balance_ledger`).
				WithArgs(input.UserID, input.PoolID, input.AccountID, input.RequestID, "共享池 API 调用 · 池 #"+fmt.Sprint(input.PoolID), input.PriceVersionID, input.PricingSource, jsonTextArg(expectedSnapshot)).
				WillReturnError(sql.ErrNoRows)
			mock.ExpectRollback()

			err = repo.RecordSharedPoolUsageTx(context.Background(), input)
			require.ErrorIs(t, err, service.ErrSharedPoolReservationConflict)
			require.ErrorContains(t, err, "idempotency key")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRecordSharedPoolUsageTxNeverRefundsUnknownForwardResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	input := sharedPoolPendingUsageInput("sp:11:unknown-forward")
	input.Success = false
	input.AccountID = 33

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT authority_epoch FROM shared_pool_cutover_control`).
		WillReturnRows(sqlmock.NewRows([]string{"authority_epoch"}).AddRow(1))
	mock.ExpectExec(`SELECT set_config`).
		WithArgs("1").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSharedPoolReservationLookupWithFrozen(
		mock, input, "forwarding", 1.25,
		input.AccountID, input.PriceVersionID, "gpt-test", input.PricingSource, input.PriceSnapshot,
	)
	// Deliberately no UPDATE users expectation: the entire hold remains frozen.
	mock.ExpectExec(`UPDATE shared_pool_usage_reservations(?s).*status = 'review_required'.*status = 'forwarding'`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_access_keys`).
		WithArgs(0.0, input.AccessKeyID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE pool_seat_bindings`).
		WithArgs(input.PoolID, input.UserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_accounts SET total_calls`).
		WithArgs(input.AccountID, input.PoolID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pools SET total_calls`).
		WithArgs(input.PoolID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.RecordSharedPoolUsageTx(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordSharedPoolUsageTxSettlesForwardingAndPendingReservations(t *testing.T) {
	for _, status := range []string{"forwarding", "settlement_pending"} {
		t.Run(status, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &bizDecipherRepository{db: db}
			input := sharedPoolPendingUsageInput("sp:11:settle-" + status)

			mock.ExpectBegin()
			expectSharedPoolReservationLookup(mock, input, status, 0)
			expectZeroCostSharedPoolReservationSettlement(mock, input)
			mock.ExpectCommit()

			err = repo.RecordSharedPoolUsageTx(context.Background(), input)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRecoverExpiredSharedPoolUsageReservationsMovesForwardingToReviewWithoutRefund(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 5, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, user_id, hold_amount(?s).*status = 'reserved'`).
		WithArgs(now, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "hold_amount"}))
	// There is deliberately no UPDATE users expectation: an expired forwarding
	// request has an unknown upstream outcome and its hold must remain frozen.
	mock.ExpectExec(`WITH expired_forwarding AS(?s).*status = 'forwarding'.*UPDATE shared_pool_usage_reservations.*status = 'review_required'`).
		WithArgs(now, 100).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	summary, err := repo.RecoverExpiredSharedPoolUsageReservationsTx(context.Background(), now, 100)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Scanned)
	require.Equal(t, 0, summary.Released)
	require.Equal(t, 1, summary.ReviewRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPendingSharedPoolUsageRecoveryIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	svc := service.NewBizDecipherService(repo, nil, nil)
	now := time.Date(2026, 7, 19, 5, 30, 0, 0, time.UTC)
	input := sharedPoolPendingUsageInput("sp:11:pending-replay")
	payload, err := json.Marshal(input)
	require.NoError(t, err)

	// First worker pass settles the durable payload.
	expectEmptySharedPoolReservationRecoveryScan(mock, now, 100)
	mock.ExpectQuery(`SELECT usage_payload(?s).*status = 'settlement_pending'`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"usage_payload"}).AddRow(payload))
	mock.ExpectBegin()
	expectSharedPoolReservationLookup(mock, input, "settlement_pending", 0)
	expectZeroCostSharedPoolReservationSettlement(mock, input)
	mock.ExpectCommit()
	expectEmptySharedPoolReviewAutoRelease(
		mock,
		now.Add(-time.Duration(service.SharedPoolReviewAutoReleaseMinutesDefault)*time.Minute),
		0.01,
		100,
	)

	first, err := svc.RecoverExpiredSharedPoolUsageReservations(context.Background(), now, 100)
	require.NoError(t, err)
	require.Equal(t, 1, first.PendingScanned)
	require.Equal(t, 1, first.Settled)

	// A stale/repeated worker read is harmless: the settled reservation returns
	// immediately and none of the balance, owner wallet, or counter writes repeat.
	expectEmptySharedPoolReservationRecoveryScan(mock, now, 100)
	mock.ExpectQuery(`SELECT usage_payload(?s).*status = 'settlement_pending'`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"usage_payload"}).AddRow(payload))
	mock.ExpectBegin()
	expectSharedPoolReservationLookup(mock, input, "settled", 0)
	mock.ExpectCommit()
	expectEmptySharedPoolReviewAutoRelease(
		mock,
		now.Add(-time.Duration(service.SharedPoolReviewAutoReleaseMinutesDefault)*time.Minute),
		0.01,
		100,
	)

	second, err := svc.RecoverExpiredSharedPoolUsageReservations(context.Background(), now, 100)
	require.NoError(t, err)
	require.Equal(t, 1, second.PendingScanned)
	require.Equal(t, 1, second.Settled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolMicroUsageOwnerEarningsNeverExceedUserDebit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	svc := service.NewBizDecipherService(repo, nil, nil)
	input := sharedPoolPendingUsageInput("sp:11:micro-usage")
	input.Cost = 4e-9
	input.PriceSnapshot = json.RawMessage(`{"price_version_id":55,"multiplier":1}`)
	frozenSnapshot := json.RawMessage(`{"multiplier":1.0,"price_version_id":55}`)

	// users.balance has eight decimal places. A non-zero 4e-9 actual cost must be
	// quantized to one 1e-8 debit before the owner/platform split is calculated.
	userDebit := 1e-8
	// The theoretical 90% share is 9e-9, below the common 1e-8 money unit,
	// therefore it is rounded down to zero and the exact debit remains platform fee.
	ownerPayout := 0.0
	platformFee := userDebit
	require.LessOrEqual(t, ownerPayout, userDebit)
	require.InDelta(t, userDebit, ownerPayout+platformFee, 1e-20)

	mock.ExpectBegin()
	expectSharedPoolReservationLookupWithFrozen(
		mock, input, "forwarding", userDebit,
		nil, input.PriceVersionID, input.Model, input.PricingSource, frozenSnapshot,
	)
	mock.ExpectQuery(`SELECT owner_id, platform_fee_percent FROM shared_pools`).
		WithArgs(input.PoolID).
		WillReturnRows(sqlmock.NewRows([]string{"owner_id", "platform_fee_percent"}).AddRow(int64(99), 10.0))
	mock.ExpectQuery(`SELECT hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from`).
		WithArgs(input.PoolID, sharedPoolReservationReservedAtForTest).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO shared_pool_balance_ledger`).
		WithArgs(input.UserID, input.PoolID, input.AccountID, input.RequestID, "共享池 API 调用 · 池 #22", input.PriceVersionID, input.PricingSource, jsonTextArg(frozenSnapshot)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(700)))
	mock.ExpectQuery(`UPDATE users(?s).*balance = balance \+ \(\$1 - \$2\)`).
		WithArgs(userDebit, userDebit, input.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.99999999))
	mock.ExpectExec(`UPDATE shared_pool_usage_reservations`).
		WithArgs(int64(7), userDebit, userDebit, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_balance_ledger SET amount`).
		WithArgs(int64(700), -userDebit, 9.99999999, "共享池 API 调用 · 池 #22", platformFee, ownerPayout, input.PriceVersionID, input.PricingSource, jsonTextArg(frozenSnapshot)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_access_keys`).
		WithArgs(userDebit, input.AccessKeyID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE pool_seat_bindings`).
		WithArgs(input.PoolID, input.UserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pools SET total_calls`).
		WithArgs(input.PoolID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = svc.RecordSharedPoolUsage(context.Background(), input)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuantizeSharedPoolLedgerDebitUsesTheBalanceUnit(t *testing.T) {
	require.Equal(t, 1e-8, quantizeSharedPoolLedgerDebit(4e-9))
	require.Equal(t, 2e-8, quantizeSharedPoolLedgerDebit(19e-9))
	require.Zero(t, quantizeSharedPoolLedgerDebit(0))
}

func sharedPoolPendingUsageInput(requestID string) service.SharedPoolUsageInput {
	return service.SharedPoolUsageInput{
		AccessKeyID:    11,
		PoolID:         22,
		UserID:         44,
		RequestID:      requestID,
		Success:        true,
		PriceVersionID: 55,
		PricingSource:  service.SharedPoolPricingSourceOwner,
		PriceSnapshot:  json.RawMessage(`{"price_version_id":55}`),
	}
}

func expectSharedPoolReservationLookup(
	mock sqlmock.Sqlmock,
	input service.SharedPoolUsageInput,
	status string,
	holdAmount float64,
) {
	var accountID any
	if input.AccountID > 0 {
		accountID = input.AccountID
	}
	var priceVersionID any
	if input.PriceVersionID > 0 {
		priceVersionID = input.PriceVersionID
	}
	priceSnapshot := input.PriceSnapshot
	if len(priceSnapshot) == 0 {
		priceSnapshot = json.RawMessage(`{}`)
	}
	expectSharedPoolReservationLookupWithFrozen(
		mock, input, status, holdAmount,
		accountID, priceVersionID, input.Model, input.PricingSource, priceSnapshot,
	)
}

func expectSharedPoolReservationLookupWithFrozen(
	mock sqlmock.Sqlmock,
	input service.SharedPoolUsageInput,
	status string,
	holdAmount float64,
	accountID any,
	priceVersionID any,
	model string,
	pricingSource string,
	priceSnapshot json.RawMessage,
) {
	mock.ExpectQuery(`SELECT id, request_id, request_fingerprint`).
		WithArgs(input.AccessKeyID, input.RequestID).
		WillReturnRows(lockedSharedPoolReservationRows().AddRow(
			int64(7), input.RequestID, reservationFingerprintForTest, input.AccessKeyID, input.PoolID,
			accountID, input.UserID, priceVersionID, "responses", model, pricingSource, []byte(priceSnapshot),
			holdAmount, 0.0, 0.0, status,
			sharedPoolReservationReservedAtForTest,
			time.Date(2026, 7, 19, 7, 0, 0, 0, time.UTC),
		))
}

func lockedSharedPoolReservationRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "request_id", "request_fingerprint", "access_key_id", "pool_id",
		"account_id", "user_id", "price_version_id", "endpoint_type", "model_snapshot",
		"pricing_source_snapshot", "price_snapshot", "hold_amount", "reported_amount",
		"settled_amount", "status", "reserved_at", "expires_at",
	})
}

func expectZeroCostSharedPoolReservationSettlement(mock sqlmock.Sqlmock, input service.SharedPoolUsageInput) {
	mock.ExpectQuery(`SELECT balance FROM users`).
		WithArgs(input.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectExec(`UPDATE shared_pool_usage_reservations(?s).*status = 'settled'.*status IN \('forwarding', 'settlement_pending'\)`).
		WithArgs(int64(7), 0.0, 0.0, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_access_keys`).
		WithArgs(0.0, input.AccessKeyID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE pool_seat_bindings`).
		WithArgs(input.PoolID, input.UserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pools SET total_calls`).
		WithArgs(input.PoolID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectEmptySharedPoolReservationRecoveryScan(mock sqlmock.Sqlmock, now time.Time, limit int) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, user_id, hold_amount(?s).*status = 'reserved'`).
		WithArgs(now, limit).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "hold_amount"}))
	mock.ExpectExec(`WITH expired_forwarding AS(?s).*status = 'forwarding'.*UPDATE shared_pool_usage_reservations.*status = 'review_required'`).
		WithArgs(now, limit).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
}

func expectEmptySharedPoolReviewAutoRelease(mock sqlmock.Sqlmock, cutoff time.Time, maxHold float64, limit int) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, user_id, hold_amount, failure_reason(?s).*status = 'review_required'`).
		WithArgs(cutoff, maxHold, limit).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "hold_amount", "failure_reason"}))
	mock.ExpectCommit()
}
