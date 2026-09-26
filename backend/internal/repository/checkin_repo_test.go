//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCheckinMilestoneCycle(t *testing.T) {
	tests := []struct {
		name      string
		totalDays int
		cycleNo   int
		cycleDay  int
	}{
		{name: "no checkins", totalDays: 0, cycleNo: 0, cycleDay: 0},
		{name: "first day", totalDays: 1, cycleNo: 1, cycleDay: 1},
		{name: "first cycle milestone", totalDays: 3, cycleNo: 1, cycleDay: 3},
		{name: "first cycle last day", totalDays: 7, cycleNo: 1, cycleDay: 7},
		{name: "second cycle first day", totalDays: 8, cycleNo: 2, cycleDay: 1},
		{name: "second cycle milestone", totalDays: 10, cycleNo: 2, cycleDay: 3},
		{name: "second cycle last day", totalDays: 14, cycleNo: 2, cycleDay: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycleNo, cycleDay := checkinMilestoneCycle(tt.totalDays)

			require.Equal(t, tt.cycleNo, cycleNo)
			require.Equal(t, tt.cycleDay, cycleDay)
		})
	}
}

func TestCheckinRepository_ClaimMilestone_AllowsSameMilestoneInSecondCycle(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCheckinRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "credit_balance"}).AddRow(0.0, 100.0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(DISTINCT checkin_date) FROM daily_checkins WHERE user_id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(SELECT 1 FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2 AND milestone_days = $3)`)).
		WithArgs(int64(7), 2, 3).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`INSERT INTO checkin_milestone_claims .*cycle_no.*RETURNING id`).
		WithArgs(int64(7), 2, 3, 30.0, 0.0, 0.0, 130.0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET credit_balance = $1, updated_at = NOW() WHERE id = $2`)).
		WithArgs(130.0, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_ledger .*checkin_milestone`).
		WithArgs(int64(7), "2:3:99", 30.0, 130.0, "Daily fortune cycle 2 3-day milestone gift").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.ClaimMilestone(ctx, 7, 3)

	require.NoError(t, err)
	require.Equal(t, 3, result.MilestoneDays)
	require.InDelta(t, 30.0, result.CreditReward, 1e-9)
	require.InDelta(t, 130.0, result.CreditBalanceAfter, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRepository_ClaimMilestone_SevenDaysAwardsPointsWithoutTouchingCash(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCheckinRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "credit_balance"}).AddRow(10.0, 100.0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(DISTINCT checkin_date) FROM daily_checkins WHERE user_id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(14))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(SELECT 1 FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2 AND milestone_days = $3)`)).
		WithArgs(int64(7), 2, 7).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`INSERT INTO checkin_milestone_claims .*cycle_no.*RETURNING id`).
		WithArgs(int64(7), 2, 7, 100.0, 0.0, 10.0, 200.0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET credit_balance = $1, updated_at = NOW() WHERE id = $2`)).
		WithArgs(200.0, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_ledger .*checkin_milestone`).
		WithArgs(int64(7), "2:7:101", 100.0, 200.0, "Daily fortune cycle 2 7-day milestone gift").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.ClaimMilestone(ctx, 7, 7)

	require.NoError(t, err)
	require.Zero(t, result.BalanceReward)
	require.Equal(t, 100.0, result.CreditReward)
	require.InDelta(t, 10.0, result.BalanceAfter, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRepository_ClaimMilestone_PreviousCashClaimAndRetryStayReadOnly(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCheckinRepository(db)
	for range 2 {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`)).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"balance", "credit_balance"}).AddRow(12.0, 200.0))
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(DISTINCT checkin_date) FROM daily_checkins WHERE user_id = $1`)).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(14))
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(SELECT 1 FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2 AND milestone_days = $3)`)).
			WithArgs(int64(7), 2, 7).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectRollback()

		result, err := repo.ClaimMilestone(context.Background(), 7, 7)
		require.Nil(t, result)
		require.ErrorIs(t, err, service.ErrCheckinMilestoneAlreadyClaimed)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRepository_MilestoneStatusPreservesClaimedHistoryWithoutAdvertisingCash(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &checkinRepository{db: db}
	status := &service.CheckinStatus{StreakDays: 14, Balance: 12}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT milestone_days FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2`)).
		WithArgs(int64(7), 2).
		WillReturnRows(sqlmock.NewRows([]string{"milestone_days"}).AddRow(7))

	err = repo.populateMilestoneStatus(context.Background(), 7, status)

	require.NoError(t, err)
	require.Equal(t, 12.0, status.Balance)
	require.Len(t, status.Milestones, 3)
	sevenDays := status.Milestones[2]
	require.Equal(t, 7, sevenDays.Days)
	require.Equal(t, 100.0, sevenDays.CreditReward)
	require.Zero(t, sevenDays.BalanceReward)
	require.True(t, sevenDays.Claimed)
	require.False(t, sevenDays.Claimable)
	require.False(t, sevenDays.ActivationRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRepository_ClaimDaily_ReplaysSettledOperationWithoutSideEffects(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCheckinRepository(db)
	createdAt := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT\s+dc\.id,.*FROM daily_checkins dc.*dc\.operation_id = \$2`).
		WithArgs(int64(7), "draw-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "operation_id", "checkin_type", "checkin_date", "point_cost", "credit_reward", "reward_asset",
			"balance_after", "credit_balance_after", "jackpot_hit", "jackpot_pool", "jackpot_amount",
			"jackpot_winner_amount", "jackpot_celebration_amount", "jackpot_celebration_user_count", "jackpot_payouts",
			"card_id", "card_key", "rarity", "source_type", "source_label", "serial_no", "edition_no", "edition_supply", "card_created_at",
		}).AddRow(
			int64(91), "draw-123", "balance", "2026-07-19", 5.0, 10.0, "balance",
			25.0, 80.0, false, "", 0.0, 0.0, 0.0, 0, "[]",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
		))

	result, err := repo.ClaimDaily(context.Background(), service.CheckinClaimInput{
		UserID:      7,
		OperationID: "draw-123",
		Type:        service.CheckinTypeBalance,
		Date:        createdAt,
		Cost:        5,
		Reward:      1,
		CostAsset:   "balance",
		RewardAsset: "balance",
		DailyLimit:  10,
	})

	require.NoError(t, err)
	require.Equal(t, "draw-123", result.OperationID)
	require.InDelta(t, 10.0, result.Reward, 1e-9, "the stored result must win over a newly planned reward")
	require.InDelta(t, 25.0, result.BalanceAfter, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRepository_GetDailyStatus_ExposesCurrentMilestoneCycle(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCheckinRepository(db)
	ctx := context.Background()
	day := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT balance, credit_balance FROM users WHERE id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "credit_balance"}).AddRow(0.0, 100.0))
	mock.ExpectQuery(`SELECT GREATEST\(0, 500 .*FROM jackpot_ledger`).
		WillReturnRows(sqlmock.NewRows([]string{"credit_jackpot"}).AddRow(500.0))
	mock.ExpectQuery(`SELECT GREATEST\(0, 50 .*FROM jackpot_ledger`).
		WillReturnRows(sqlmock.NewRows([]string{"balance_jackpot"}).AddRow(50.0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(DISTINCT checkin_date) FROM daily_checkins WHERE user_id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT milestone_days FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2`)).
		WithArgs(int64(7), 2).
		WillReturnRows(sqlmock.NewRows([]string{"milestone_days"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT checkin_type, COUNT(*) FROM daily_checkins WHERE user_id = $1 AND checkin_date = $2 GROUP BY checkin_type`)).
		WithArgs(int64(7), "2026-07-10").
		WillReturnRows(sqlmock.NewRows([]string{"checkin_type", "count"}).
			AddRow(service.CheckinTypeCredit, 9).
			AddRow(service.CheckinTypeBalance, 2))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM credit_lottery_sessions WHERE user_id = $1 AND session_date = $2 AND status <> 'failed'`)).
		WithArgs(int64(7), "2026-07-10").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))

	status, err := repo.GetDailyStatus(ctx, 7, day)

	require.NoError(t, err)
	require.Equal(t, 8, status.StreakDays)
	require.Equal(t, 2, status.CycleNo)
	require.Equal(t, 1, status.CycleDay)
	require.Equal(t, 4, status.CreditCount)
	require.Equal(t, 2, status.BalanceCount)
	require.Len(t, status.Milestones, 3)
	for _, milestone := range status.Milestones {
		require.Zero(t, milestone.BalanceReward)
		require.False(t, milestone.Eligible)
		require.False(t, milestone.Claimable)
		require.False(t, milestone.Claimed)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRepository_ClaimDailyRecordsCreditJackpotContribution(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCheckinRepository(db)
	ctx := context.Background()
	day := time.Date(2026, 7, 12, 0, 0, 0, 0, time.Local)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_xact_lock(hashtext('jackpot_pool:' || $1))`)).
		WithArgs("credit").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "credit_balance"}).AddRow(0.0, 100.0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM daily_checkins WHERE user_id = $1 AND checkin_date = $2 AND checkin_type = $3`)).
		WithArgs(int64(7), "2026-07-12", "credit").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET balance = $1, credit_balance = $2, updated_at = NOW() WHERE id = $3`)).
		WithArgs(0.0, 85.0, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO daily_checkins .*RETURNING id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectExec(`INSERT INTO credit_ledger .*daily_checkin`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO jackpot_ledger .*'daily_checkin'.*'contribution'`).
		WithArgs("credit", "daily_checkin:99", 4.0, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err = repo.ClaimDaily(ctx, service.CheckinClaimInput{
		UserID:      7,
		Type:        service.CheckinTypeCredit,
		Date:        day,
		Cost:        20,
		Reward:      5,
		CostAsset:   "credit",
		RewardAsset: "credit",
		DailyLimit:  50,
		Jackpot:     service.JackpotPlan{},
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildJackpotPayoutsReturnsCreditAndBalancePayouts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_xact_lock(hashtext('jackpot_pool:' || $1))`)).
		WithArgs("credit").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`SELECT pg_advisory_xact_lock(hashtext('jackpot_pool:' || $1))`)).
		WithArgs("balance").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(amount), 0)::double precision FROM jackpot_ledger WHERE pool_type = $1`)).
		WithArgs("credit").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(100.0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(amount), 0)::double precision FROM jackpot_ledger WHERE pool_type = $1`)).
		WithArgs("balance").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(10.0))

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	payouts, err := buildJackpotPayouts(ctx, tx, service.JackpotPlan{Hit: true, WinnerRatio: 0.5, CelebrationRatio: 0.1})
	require.NoError(t, err)

	require.Len(t, payouts, 2)
	require.Equal(t, "credit", payouts[0].PoolType)
	require.Equal(t, "credit", payouts[0].RewardAsset)
	require.InDelta(t, 300.0, payouts[0].WinnerAmount, 1e-9)
	require.InDelta(t, 60.0, payouts[0].CelebrationAmount, 1e-9)
	require.Equal(t, "balance", payouts[1].PoolType)
	require.Equal(t, "balance", payouts[1].RewardAsset)
	require.InDelta(t, 30.0, payouts[1].WinnerAmount, 1e-9)
	require.InDelta(t, 6.0, payouts[1].CelebrationAmount, 1e-9)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFloorJackpotShareAmountNeverOverpays(t *testing.T) {
	share := floorJackpotShareAmount(1.0 / 6.0)

	require.InDelta(t, 0.16, share, 1e-9)
	require.LessOrEqual(t, share*6, 1.0)
}
