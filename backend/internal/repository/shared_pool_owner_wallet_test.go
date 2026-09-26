package repository

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func expectSharedPoolOwnerEarningReplay(
	t *testing.T,
	db *sql.DB,
	mock sqlmock.Sqlmock,
	input sharedPoolOwnerEarningCredit,
	existing sharedPoolOwnerEarningCredit,
) *sql.Tx {
	t.Helper()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT name, owner_label FROM shared_pools WHERE id = $1`)).
		WithArgs(input.PoolID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "owner_label"}).AddRow("测试池", "池主"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO shared_pool_owner_wallets (owner_id)
		 VALUES ($1)
		 ON CONFLICT (owner_id) DO NOTHING`)).
		WithArgs(input.OwnerID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)INSERT INTO shared_pool_owner_earnings_ledger .*jsonb_build_object\('source_type', \$14::text\).*ON CONFLICT .*DO NOTHING.*RETURNING id, available_after`).
		WithArgs(
			input.OwnerID,
			input.PoolID,
			input.AccountID,
			input.PriceVersionID,
			input.OperationID,
			input.RequestID,
			"测试池",
			"池主",
			input.Model,
			input.PricingSource,
			input.GrossAmount,
			input.PlatformFee,
			input.NetAmount,
			strings.TrimSpace(input.SourceType),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "available_after"}))
	mock.ExpectQuery(`(?s)SELECT\s+id,\s+available_after,\s+COALESCE\(pool_id, 0\).*metadata->>'source_type'.*owner_earnings_ledger_id = earning\.id.*FROM shared_pool_owner_earnings_ledger earning`).
		WithArgs(input.OwnerID, input.OperationID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"available_after",
			"pool_id",
			"account_id",
			"price_version_id",
			"request_id",
			"model_snapshot",
			"pricing_source_snapshot",
			"gross_amount",
			"platform_fee_amount",
			"net_amount",
			"source_type",
		}).AddRow(
			int64(91),
			12.75,
			existing.PoolID,
			existing.AccountID,
			existing.PriceVersionID,
			existing.RequestID,
			existing.Model,
			existing.PricingSource,
			existing.GrossAmount,
			existing.PlatformFee,
			existing.NetAmount,
			existing.SourceType,
		))
	return tx
}

func TestCreditSharedPoolOwnerWalletAllowsExactOperationReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	input := sharedPoolOwnerEarningCredit{
		OwnerID:        7,
		PoolID:         11,
		AccountID:      13,
		PriceVersionID: 17,
		OperationID:    "usage:req-123",
		RequestID:      "req-123",
		Model:          "gpt-5.6",
		PricingSource:  "official",
		GrossAmount:    2.50,
		PlatformFee:    0.25,
		NetAmount:      2.25,
		SourceType:     "  share_pool_payout  ",
	}
	existing := input
	existing.SourceType = "share_pool_payout"
	tx := expectSharedPoolOwnerEarningReplay(t, db, mock, input, existing)
	mock.ExpectRollback()

	result, err := creditSharedPoolOwnerWalletTx(context.Background(), tx, input)
	require.NoError(t, err)
	require.Equal(t, &sharedPoolOwnerEarningCreditResult{
		LedgerID:      91,
		WalletAfter:   12.75,
		AlreadyPosted: true,
	}, result)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreditSharedPoolOwnerWalletRejectsOperationIDPayloadMismatch(t *testing.T) {
	base := sharedPoolOwnerEarningCredit{
		OwnerID:        7,
		PoolID:         11,
		AccountID:      13,
		PriceVersionID: 17,
		OperationID:    "usage:req-123",
		RequestID:      "req-123",
		Model:          "gpt-5.6",
		PricingSource:  "official",
		GrossAmount:    2.50,
		PlatformFee:    0.25,
		NetAmount:      2.25,
		SourceType:     "share_pool_payout",
	}

	tests := []struct {
		name   string
		mutate func(*sharedPoolOwnerEarningCredit)
	}{
		{name: "pool", mutate: func(v *sharedPoolOwnerEarningCredit) { v.PoolID++ }},
		{name: "account", mutate: func(v *sharedPoolOwnerEarningCredit) { v.AccountID++ }},
		{name: "price version", mutate: func(v *sharedPoolOwnerEarningCredit) { v.PriceVersionID++ }},
		{name: "request", mutate: func(v *sharedPoolOwnerEarningCredit) { v.RequestID += "-other" }},
		{name: "model", mutate: func(v *sharedPoolOwnerEarningCredit) { v.Model = "gpt-5.6-mini" }},
		{name: "pricing source", mutate: func(v *sharedPoolOwnerEarningCredit) { v.PricingSource = "owner_custom" }},
		{name: "source type", mutate: func(v *sharedPoolOwnerEarningCredit) { v.SourceType = "pool_owner_payout" }},
		{name: "gross amount", mutate: func(v *sharedPoolOwnerEarningCredit) { v.GrossAmount += 0.01 }},
		{name: "platform fee", mutate: func(v *sharedPoolOwnerEarningCredit) { v.PlatformFee += 0.01 }},
		{name: "net amount", mutate: func(v *sharedPoolOwnerEarningCredit) { v.NetAmount += 0.01 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			existing := base
			tt.mutate(&existing)
			tx := expectSharedPoolOwnerEarningReplay(t, db, mock, base, existing)
			mock.ExpectRollback()

			result, err := creditSharedPoolOwnerWalletTx(context.Background(), tx, base)
			require.Nil(t, result)
			require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreditSharedPoolOwnerWalletRejectsConflictingBalanceLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	input := sharedPoolOwnerEarningCredit{
		OwnerID:        7,
		PoolID:         11,
		AccountID:      13,
		PriceVersionID: 17,
		OperationID:    "usage:req-legacy-payout",
		RequestID:      "req-legacy-payout",
		Model:          "gpt-5.6",
		PricingSource:  "official",
		GrossAmount:    2.50,
		PlatformFee:    0.25,
		NetAmount:      2.25,
		SourceType:     "share_pool_payout",
		Note:           "共享池 API 分润",
	}

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT name, owner_label FROM shared_pools WHERE id = $1`)).
		WithArgs(input.PoolID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "owner_label"}).AddRow("测试池", "池主"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO shared_pool_owner_wallets (owner_id)
		 VALUES ($1)
		 ON CONFLICT (owner_id) DO NOTHING`)).
		WithArgs(input.OwnerID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)INSERT INTO shared_pool_owner_earnings_ledger .*jsonb_build_object\('source_type', \$14::text\).*RETURNING id, available_after`).
		WithArgs(
			input.OwnerID, input.PoolID, input.AccountID, input.PriceVersionID,
			input.OperationID, input.RequestID, "测试池", "池主", input.Model, input.PricingSource,
			input.GrossAmount, input.PlatformFee, input.NetAmount, input.SourceType,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "available_after"}).AddRow(int64(91), 10.0))
	mock.ExpectQuery(`(?s)UPDATE shared_pool_owner_wallets.*RETURNING available_amount`).
		WithArgs(input.OwnerID, input.NetAmount).
		WillReturnRows(sqlmock.NewRows([]string{"available_amount"}).AddRow(12.25))
	mock.ExpectExec(`(?s)UPDATE shared_pool_owner_earnings_ledger.*status = 'available'.*WHERE id = \$1`).
		WithArgs(int64(91), input.NetAmount, 12.25).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)INSERT INTO shared_pool_balance_ledger .*ON CONFLICT .*DO NOTHING.*RETURNING id`).
		WithArgs(
			input.OwnerID, input.PoolID, input.AccountID, input.SourceType, input.OperationID,
			input.NetAmount, 12.25, input.Note, "测试池", "池主", input.Model, input.PriceVersionID, int64(91),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	result, err := creditSharedPoolOwnerWalletTx(context.Background(), tx, input)
	require.Nil(t, result)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTransferSharedPoolOwnerEarningsRejectsOperationIDPayloadMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &bizDecipherRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO shared_pool_owner_wallets (owner_id)
		 VALUES ($1)
		 ON CONFLICT (owner_id) DO NOTHING`)).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT available_amount FROM shared_pool_owner_wallets WHERE owner_id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"available_amount"}).AddRow(9.0))
	mock.ExpectQuery(`(?s)SELECT wallet_delta, available_after, metadata\s+FROM shared_pool_owner_earnings_ledger`).
		WithArgs(int64(7), "transfer:fixed-operation").
		WillReturnRows(sqlmock.NewRows([]string{"wallet_delta", "available_after", "metadata"}).
			AddRow(-1.25, 7.75, []byte(`{"balance_after":21.25}`)))
	mock.ExpectRollback()

	result, err := repo.TransferSharedPoolOwnerEarningsTx(
		context.Background(),
		7,
		2.50,
		"transfer:fixed-operation",
	)
	require.Nil(t, result)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
