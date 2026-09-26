//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListSharedPoolOwnerEarningsPageUsesStableCursorAndPoolFilter(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)

	columns := []string{
		"id", "owner_id", "pool_id", "account_id", "price_version_id",
		"event_type", "operation_id", "request_id",
		"pool_name_snapshot", "owner_label_snapshot", "model_snapshot", "pricing_source_snapshot",
		"gross_amount", "platform_fee_amount", "net_amount",
		"wallet_delta", "available_after", "status",
		"available_at", "metadata", "created_at", "posted_at",
	}
	rows := sqlmock.NewRows(columns)
	for _, id := range []int64{99, 98, 97} {
		rows.AddRow(
			id, int64(7), int64(42), nil, nil,
			"earning", "op", "req",
			"归档池", "池主", "gpt-test", "official_catalog",
			1.0, 0.1, 0.9,
			0.9, 3.0, "available",
			now, []byte(`{}`), now, now,
		)
	}
	mock.ExpectQuery(`(?s)FROM shared_pool_owner_earnings_ledger.*owner_id = \$1.*id < \$2.*pool_id = \$3.*ORDER BY id DESC.*LIMIT \$4`).
		WithArgs(int64(7), int64(100), int64(42), 3).
		WillReturnRows(rows)

	page, err := repo.ListSharedPoolOwnerEarningsPage(context.Background(), 7, 42, 100, 2)

	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.Equal(t, int64(98), page.NextBeforeID)
	require.Len(t, page.Items, 2)
	require.Equal(t, []int64{99, 98}, []int64{page.Items[0].ID, page.Items[1].ID})
	require.NoError(t, mock.ExpectationsWereMet())
}
