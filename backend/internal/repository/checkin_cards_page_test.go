//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCheckinRepository_ListCollectibleCardsPageReturnsTotalAndStablePage(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &checkinRepository{db: db}

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT COUNT(*) FROM checkin_collectible_cards WHERE user_id = $1`,
	)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(205)))
	mock.ExpectQuery(`(?s)SELECT id, checkin_id, card_key, rarity, source_type, source_label, serial_no, edition_no, edition_supply, created_at\s+FROM checkin_collectible_cards\s+WHERE user_id = \$1\s+ORDER BY id DESC\s+LIMIT \$2 OFFSET \$3`).
		WithArgs(int64(7), 100, 100).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "checkin_id", "card_key", "rarity", "source_type", "source_label",
			"serial_no", "edition_no", "edition_supply", "created_at",
		}).AddRow(
			int64(104), nil, "low_battery_sprite", "common", "free", "free",
			int64(104), int64(4), int64(9999), time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC),
		))

	page, err := repo.ListCollectibleCardsPage(context.Background(), 7, 2, 100)

	require.NoError(t, err)
	require.Equal(t, int64(205), page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 100, page.PageSize)
	require.True(t, page.HasMore)
	require.Len(t, page.Items, 1)
	require.Equal(t, int64(104), page.Items[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}
