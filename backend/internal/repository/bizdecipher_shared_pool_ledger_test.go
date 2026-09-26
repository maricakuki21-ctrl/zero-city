package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListMySharedPoolLedgerSeparatesActivityFromOwnerEarnings(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	userID := int64(42)
	now := time.Now()
	ledgerColumns := []string{"id", "user_id", "pool_id", "source_type", "source_id", "amount", "balance_after", "status", "note", "created_at", "posted_at"}

	mock.ExpectQuery("FROM shared_pool_owner_wallets w").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"owner_id", "available_amount", "pending_amount", "frozen_amount",
			"transferred_amount", "total_earned", "version", "updated_at",
		}).AddRow(userID, 0.16, 0, 0, 0, 0.16, int64(1), now))
	mock.ExpectQuery("FROM shared_pool_owner_earnings_ledger").
		WithArgs(userID, 100).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "owner_id", "pool_id", "account_id", "price_version_id",
			"event_type", "operation_id", "request_id",
			"pool_name_snapshot", "owner_label_snapshot", "model_snapshot", "pricing_source_snapshot",
			"gross_amount", "platform_fee_amount", "net_amount",
			"wallet_delta", "available_after", "status",
			"available_at", "metadata", "created_at", "posted_at",
		}).AddRow(
			int64(4), userID, int64(7), nil, nil,
			"earning", "req-2", "req-2",
			"Pool Seven", "Owner", "gpt-test", "official",
			0.2, 0.04, 0.16,
			0.16, 0.16, "available",
			now, []byte(`{}`), now, now,
		))
	mock.ExpectQuery("source_type IN \\('share_pool_usage', 'pool_seat_fee'\\)").
		WithArgs(userID, 100).
		WillReturnRows(sqlmock.NewRows(ledgerColumns).
			AddRow(int64(1), userID, int64(7), "share_pool_usage", "req-1", -0.2, 9.8, "posted", "Shared pool #7 usage", now, now).
			AddRow(int64(2), userID, int64(7), "pool_seat_fee", "seat:1:hour", -0.1, 9.7, "posted", "Pool seat fee", now, now))
	mock.ExpectQuery("settlement_destination = 'legacy_balance'").
		WithArgs(userID, 100).
		WillReturnRows(sqlmock.NewRows(ledgerColumns).
			AddRow(int64(3), userID, int64(7), "share_pool_payout", "req-2-legacy", 0.08, 9.78, "posted", "Legacy shared pool payout", now, now))
	mock.ExpectQuery("source_type = 'shared_pool_stability_reward'").
		WithArgs(userID, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_type", "source_id", "amount", "balance_after", "status", "note", "created_by", "created_at", "posted_at"}))

	view, err := repo.ListMySharedPoolLedger(context.Background(), userID, 100)
	require.NoError(t, err)
	require.Len(t, view.Activity, 2)
	require.Equal(t, "share_pool_usage", view.Activity[0].SourceType)
	require.Equal(t, "pool_seat_fee", view.Activity[1].SourceType)
	require.InDelta(t, 0.16, view.Wallet.AvailableAmount, 1e-9)
	require.Len(t, view.Earnings, 1)
	require.Len(t, view.Withdrawable, 2)
	require.Equal(t, "owner_wallet_earning", view.Withdrawable[0].SourceType)
	require.Equal(t, "share_pool_payout", view.Withdrawable[1].SourceType)
	require.Len(t, view.LegacyWithdrawable, 1)
	require.NotNil(t, view.Incentives)
	require.Empty(t, view.Incentives)
	require.NoError(t, mock.ExpectationsWereMet())
}
