package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdminListSharedPoolOwnerEarningsPageKeepsPermanentSnapshotsAndPreciseTotals(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	filter := service.AdminSharedPoolOwnerEarningsFilter{
		OwnerID: 7, PoolID: 42, BeforeID: 100, Kind: "api", Status: "available", Limit: 2,
	}
	mock.ExpectQuery(`(?s)COUNT\(\*\).*FROM shared_pool_owner_earnings_ledger earning`).
		WithArgs(int64(7), int64(42), "available", "api").
		WillReturnRows(sqlmock.NewRows([]string{
			"matching_entries", "matching_owners", "gross", "platform_fee", "net",
		}).AddRow(3, 1, 0.000009, 0.000001, 0.000008))

	columns := []string{
		"id", "owner_id", "pool_id", "account_id", "price_version_id",
		"event_type", "operation_id", "request_id",
		"pool_name_snapshot", "owner_label_snapshot", "model_snapshot", "pricing_source_snapshot",
		"gross_amount", "platform_fee_amount", "net_amount",
		"wallet_delta", "available_after", "status",
		"available_at", "metadata", "created_at", "posted_at",
		"owner_email", "owner_username",
	}
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows(columns).
		AddRow(99, 7, nil, nil, 11, "earning", "op-99", "req-99", "已删除池仍保留快照", "池主", "gpt-test", "official_catalog", 0.000003, 0.000001, 0.000002, 0.000002, 0.000002, "available", now, []byte(`{"source_type":"share_pool_payout"}`), now, now, "owner@example.com", "owner").
		AddRow(98, 7, 42, 5, 11, "earning", "op-98", "req-98", "在线池", "池主", "gpt-test", "official_catalog", 0.000003, 0, 0.000003, 0.000003, 0.000005, "available", now, []byte(`{"source_type":"share_pool_payout"}`), now, now, "owner@example.com", "owner").
		AddRow(97, 7, 42, 6, 11, "earning", "op-97", "req-97", "在线池", "池主", "gpt-test", "official_catalog", 0.000003, 0, 0.000003, 0.000003, 0.000008, "available", now, []byte(`{"source_type":"share_pool_payout"}`), now, now, "owner@example.com", "owner")
	mock.ExpectQuery(`(?s)COALESCE\(owner.email.*FROM shared_pool_owner_earnings_ledger earning`).
		WithArgs(int64(7), int64(42), "available", "api", int64(100), 3).
		WillReturnRows(rows)

	page, err := repo.AdminListSharedPoolOwnerEarningsPage(context.Background(), filter)
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.Equal(t, int64(98), page.NextBeforeID)
	require.Equal(t, int64(3), page.MatchingEntries)
	require.Equal(t, int64(1), page.MatchingOwners)
	require.InDelta(t, 0.000009, page.MatchingGrossAmount, 1e-12)
	require.InDelta(t, 0.000001, page.MatchingPlatformFee, 1e-12)
	require.InDelta(t, 0.000008, page.MatchingNetAmount, 1e-12)
	require.Len(t, page.Items, 2)
	require.Nil(t, page.Items[0].PoolID)
	require.Equal(t, "已删除池仍保留快照", page.Items[0].PoolNameSnapshot)
	require.Equal(t, "owner@example.com", page.Items[0].OwnerEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}
