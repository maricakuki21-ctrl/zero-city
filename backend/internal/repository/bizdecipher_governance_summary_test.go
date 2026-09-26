package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetSharedPoolGovernanceSummarySplitsAPIAndSeatLedgerTotals(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 7, 15, 30, 0, 0, time.UTC)
	dayStart := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"api_usage_charges",
		"api_owner_payout",
		"api_platform_fee",
		"seat_fee_charges",
		"seat_owner_payout",
		"seat_platform_fee",
		"usage_count",
		"active_pools",
		"open_complaints",
		"listed_pools",
		"healthy_pools",
		"limited_pools",
		"offline_pools",
		"watch_pools",
		"suppressed_pools",
		"banned_pools",
		"total_calls",
		"successful_calls",
		"failed_calls",
		"average_availability",
	}).AddRow(
		0.6059,
		0.4847,
		0.1212,
		8.7,
		8.7,
		0.0,
		32,
		4,
		3,
		24,
		24,
		1,
		14,
		2,
		1,
		0,
		900,
		870,
		30,
		93.5,
	)

	mock.ExpectQuery("WITH usage_today AS").
		WithArgs(dayStart, dayEnd).
		WillReturnRows(rows)

	summary, err := repo.GetSharedPoolGovernanceSummary(context.Background(), now)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.InDelta(t, 0.6059, summary.TodayAPIUsageCharges, 1e-9)
	require.InDelta(t, 0.4847, summary.TodayAPIOwnerPayout, 1e-9)
	require.InDelta(t, 0.1212, summary.TodayAPIPlatformFee, 1e-9)
	require.InDelta(t, 8.7, summary.TodaySeatFeeCharges, 1e-9)
	require.InDelta(t, 8.7, summary.TodaySeatOwnerPayout, 1e-9)
	require.InDelta(t, 0.0, summary.TodaySeatPlatformFee, 1e-9)
	require.InDelta(t, 9.3059, summary.TodayGrossCharges, 1e-9)
	require.InDelta(t, 9.1847, summary.TodayOwnerPayout, 1e-9)
	require.InDelta(t, 0.1212, summary.TodayPlatformFee, 1e-9)
	require.Equal(t, int64(32), summary.TodayUsageCount)
	require.Equal(t, int64(4), summary.TodayActivePools)
	require.Equal(t, "用户实际扣款 - 池主永久收益总账入账", summary.PlatformFeeBasis)
	require.NoError(t, mock.ExpectationsWereMet())
}
