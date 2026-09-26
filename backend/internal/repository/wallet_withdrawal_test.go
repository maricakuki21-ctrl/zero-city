package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func withdrawalRows(status, amount, reason, reference string, claimed ...int64) *sqlmock.Rows {
	actor := int64(0)
	if len(claimed) > 0 {
		actor = claimed[0]
	}
	return sqlmock.NewRows([]string{"id", "owner_id", "operation_id", "amount", "channel", "recipient", "status", "reference", "reason", "created_at", "updated_at", "processing_by"}).
		AddRow(9, 7, "withdrawal_token_123", amount, "bank", "recipient", status, reference, reason, time.Now(), time.Now(), actor)
}
func TestWithdrawalExactReplayAndConflict(t *testing.T) {
	for _, amount := range []string{"1", "2"} {
		t.Run(amount, func(t *testing.T) {
			db, m, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT id FROM users WHERE id=.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			m.ExpectQuery(`SELECT available_amount::text .* FOR UPDATE`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"available"}).AddRow("0.000000000000"))
			m.ExpectQuery(`SELECT .* FROM shared_pool_withdrawals WHERE owner_id=`).WillReturnRows(withdrawalRows("pending", "1.000000000000", "", ""))
			if amount == "1" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			result, err := (&bizDecipherRepository{db: db}).CreateWithdrawal(context.Background(), 7, service.WithdrawalInput{OperationID: "withdrawal_token_123", Amount: amount, Channel: "bank", Recipient: "recipient"})
			if amount == "1" {
				require.NoError(t, err)
				require.Equal(t, int64(9), result.ID)
			} else {
				require.ErrorIs(t, err, service.ErrWithdrawalConflict)
			}
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestWithdrawalDebitRollbackAndCommit(t *testing.T) {
	for _, failLedger := range []bool{true, false} {
		t.Run(map[bool]string{true: "rollback", false: "commit"}[failLedger], func(t *testing.T) {
			db, m, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT id FROM users WHERE id=.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			m.ExpectQuery(`SELECT available_amount::text .* FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"available"}).AddRow("3.000000000000"))
			m.ExpectQuery(`SELECT .* FROM shared_pool_withdrawals WHERE owner_id=`).WillReturnError(sql.ErrNoRows)
			m.ExpectQuery(`SELECT enabled,channels .* FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"enabled", "channels"}).AddRow(true, `["bank"]`))
			m.ExpectQuery(`UPDATE shared_pool_owner_wallets SET available_amount=available_amount-`).WithArgs(int64(7), "1.000000000000").WillReturnRows(sqlmock.NewRows([]string{"available"}).AddRow("2.000000000000"))
			m.ExpectQuery(`INSERT INTO shared_pool_withdrawals`).WillReturnRows(withdrawalRows("pending", "1.000000000000", "", ""))
			ledger := m.ExpectExec(`INSERT INTO shared_pool_owner_earnings_ledger`).WithArgs(int64(7), "withdrawal", "withdrawal_9", "-1.000000000000", "2.000000000000", int64(9), int64(7))
			if failLedger {
				ledger.WillReturnError(errors.New("ledger unavailable"))
				m.ExpectRollback()
			} else {
				ledger.WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectExec(`INSERT INTO shared_pool_withdrawal_audit`).WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectCommit()
			}
			_, err = (&bizDecipherRepository{db: db}).CreateWithdrawal(context.Background(), 7, service.WithdrawalInput{OperationID: "withdrawal_token_123", Amount: "1", Channel: "bank", Recipient: "recipient"})
			if failLedger {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestWithdrawalCancelRefundExactlyOnce(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := &bizDecipherRepository{db: db}
	m.ExpectBegin()
	m.ExpectQuery(`SELECT id FROM users WHERE id IN .*ORDER BY id FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	m.ExpectQuery(`SELECT .* WHERE id=.*FOR UPDATE`).WithArgs(int64(9), false, int64(7)).WillReturnRows(withdrawalRows("pending", "1.000000000000", "", ""))
	m.ExpectQuery(`UPDATE shared_pool_owner_wallets SET available_amount=available_amount\+`).WithArgs(int64(7), "1.000000000000").WillReturnRows(sqlmock.NewRows([]string{"available"}).AddRow("3.000000000000"))
	m.ExpectExec(`INSERT INTO shared_pool_owner_earnings_ledger`).WithArgs(int64(7), "withdrawal_return", "withdrawal_9", "1.000000000000", "3.000000000000", int64(9), int64(7)).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectQuery(`UPDATE shared_pool_withdrawals`).WillReturnRows(withdrawalRows("cancelled", "1.000000000000", "", ""))
	m.ExpectExec(`INSERT INTO shared_pool_withdrawal_audit`).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	_, err = r.ActWithdrawal(context.Background(), 7, 9, false, service.WithdrawalAction{Action: "cancelled"})
	require.NoError(t, err)
	m.ExpectBegin()
	m.ExpectQuery(`SELECT id FROM users WHERE id IN .*ORDER BY id FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	m.ExpectQuery(`SELECT .* WHERE id=.*FOR UPDATE`).WillReturnRows(withdrawalRows("cancelled", "1.000000000000", "", ""))
	m.ExpectCommit()
	_, err = r.ActWithdrawal(context.Background(), 7, 9, false, service.WithdrawalAction{Action: "cancelled"})
	require.NoError(t, err)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestWithdrawalRejectsUnsafeTransitions(t *testing.T) {
	for _, action := range []string{"paid", "cancelled", "rejected"} {
		t.Run(action, func(t *testing.T) {
			db, m, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			if action != "paid" {
				m.ExpectBegin()
				m.ExpectQuery(`SELECT id FROM users WHERE id IN .*ORDER BY id FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
				m.ExpectQuery(`SELECT .*FOR UPDATE`).WillReturnRows(withdrawalRows("processing", "1.000000000000", "", ""))
				m.ExpectRollback()
			}
			_, err = (&bizDecipherRepository{db: db}).ActWithdrawal(context.Background(), 7, 9, true, service.WithdrawalAction{Action: action, Reason: "test"})
			require.Error(t, err)
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestWithdrawalPaidRequiresProcessingAndReference(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT id FROM users WHERE id IN .*ORDER BY id FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7).AddRow(8))
	m.ExpectQuery(`SELECT .*FOR UPDATE`).WillReturnRows(withdrawalRows("processing", "1.000000000000", "", ""))
	m.ExpectQuery(`UPDATE shared_pool_withdrawals SET status=\$2::varchar,reason=\$3,reference=\$4,processing_by=CASE WHEN \$2::varchar='processing'::varchar THEN \$5::bigint ELSE processing_by END`).WithArgs(int64(9), "paid", "verified", "bank-ref-9", int64(8)).WillReturnRows(withdrawalRows("paid", "1.000000000000", "verified", "bank-ref-9"))
	m.ExpectExec(`INSERT INTO shared_pool_withdrawal_audit`).WithArgs(int64(9), int64(8), "paid", "verified", "bank-ref-9").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	_, err = (&bizDecipherRepository{db: db}).ActWithdrawal(context.Background(), 8, 9, true, service.WithdrawalAction{Action: "paid", Reason: "verified", Reference: "bank-ref-9"})
	require.NoError(t, err)
	require.NoError(t, m.ExpectationsWereMet())
}

func TestWithdrawalProcessingClaimCannotBeReplayedByAnotherAdmin(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT id FROM users WHERE id IN .*ORDER BY id FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7).AddRow(9))
	m.ExpectQuery(`SELECT .*FOR UPDATE`).WillReturnRows(withdrawalRows("processing", "1.000000000000", "", "", 8))
	m.ExpectRollback()
	_, err = (&bizDecipherRepository{db: db}).ActWithdrawal(context.Background(), 9, 9, true, service.WithdrawalAction{Action: "processing"})
	require.ErrorIs(t, err, service.ErrWithdrawalConflict)
	require.NoError(t, m.ExpectationsWereMet())
}
