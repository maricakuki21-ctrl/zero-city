//go:build unit

package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreditLotteryRepository_ContinueSessionReplaysIdempotentSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCreditLotteryRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectCreditLotterySessionLock(mock, now, "active", 1)
	mock.ExpectQuery(`INSERT INTO credit_lottery_operation_keys .*ON CONFLICT DO NOTHING RETURNING id`).
		WithArgs(int64(10), int64(7), "continue", "idem-continue", 1).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT response_snapshot::text FROM credit_lottery_operation_keys WHERE session_id = $1 AND user_id = $2 AND operation = $3 AND idempotency_key = $4`)).
		WithArgs(int64(10), int64(7), "continue", "idem-continue").
		WillReturnRows(sqlmock.NewRows([]string{"response_snapshot"}).AddRow(`{"id":10,"user_id":7,"session_date":"2026-07-12","status":"active","cost_credit":20,"max_rounds":3,"current_round":2,"credit_balance_after":80,"current_result":{"round_no":2,"reward_asset":"credit","reward_amount":50}}`))
	mock.ExpectRollback()

	result, err := repo.ContinueSession(ctx, service.CreditLotteryContinueInput{
		UserID:         7,
		SessionID:      10,
		ExpectedRound:  1,
		IdempotencyKey: "idem-continue",
		NextRound: service.CreditLotteryRoundPlan{
			RoundNo:      2,
			RewardAsset:  "credit",
			RewardAmount: 50,
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(10), result.ID)
	require.Equal(t, 2, result.CurrentRound)
	require.NotNil(t, result.CurrentResult)
	require.InDelta(t, 50.0, result.CurrentResult.RewardAmount, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreditLotteryRepository_SettleSessionReplaysIdempotentSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCreditLotteryRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectCreditLotterySessionLock(mock, now, "active", 2)
	mock.ExpectQuery(`INSERT INTO credit_lottery_operation_keys .*ON CONFLICT DO NOTHING RETURNING id`).
		WithArgs(int64(10), int64(7), "settle", "idem-settle", nil).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT response_snapshot::text FROM credit_lottery_operation_keys WHERE session_id = $1 AND user_id = $2 AND operation = $3 AND idempotency_key = $4`)).
		WithArgs(int64(10), int64(7), "settle", "idem-settle").
		WillReturnRows(sqlmock.NewRows([]string{"response_snapshot"}).AddRow(`{"id":10,"user_id":7,"session_date":"2026-07-12","status":"settled","cost_credit":20,"max_rounds":3,"current_round":2,"settlement_reason":"user_stop","credit_balance_after":130,"final_result":{"round_no":2,"reward_asset":"credit","reward_amount":50}}`))
	mock.ExpectRollback()

	result, err := repo.SettleSession(ctx, service.CreditLotterySettleInput{
		UserID:         7,
		SessionID:      10,
		Reason:         service.CreditLotterySettlementUserStop,
		IdempotencyKey: "idem-settle",
	})

	require.NoError(t, err)
	require.Equal(t, service.CreditLotteryStatusSettled, result.Status)
	require.Equal(t, service.CreditLotterySettlementUserStop, result.SettlementReason)
	require.NotNil(t, result.FinalResult)
	require.InDelta(t, 50.0, result.FinalResult.RewardAmount, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreditLotteryRepository_GetSessionByOperationIDReturnsStoredSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCreditLotteryRepository(db)
	mock.ExpectQuery(`SELECT response_snapshot::text\s+FROM credit_lottery_operation_keys`).
		WithArgs(int64(7), "round-operation-1").
		WillReturnRows(sqlmock.NewRows([]string{"response_snapshot"}).AddRow(`{"id":10,"user_id":7,"session_date":"2026-07-19","status":"settled","current_round":2,"settlement_reason":"user_stop","credit_balance_after":130}`))

	result, err := repo.GetSessionByOperationID(context.Background(), 7, "round-operation-1")

	require.NoError(t, err)
	require.Equal(t, int64(10), result.ID)
	require.Equal(t, service.CreditLotteryStatusSettled, result.Status)
	require.Equal(t, service.CreditLotterySettlementUserStop, result.SettlementReason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreditLotteryRepository_SettleSessionDoesNotSettleStaleExpectedRound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewCreditLotteryRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectCreditLotterySessionLock(mock, now, "active", 2)
	expectCreditLotterySessionSnapshot(mock, now, "active", 2)
	expectCreditLotteryCurrentRound(mock, false)
	expectCreditLotteryLatestRound(mock, true)
	mock.ExpectCommit()

	result, err := repo.SettleSession(ctx, service.CreditLotterySettleInput{
		UserID:         7,
		SessionID:      10,
		ExpectedRound:  1,
		Reason:         service.CreditLotterySettlementUserStop,
		IdempotencyKey: "idem-settle-stale",
	})

	require.NoError(t, err)
	require.Equal(t, service.CreditLotteryStatusActive, result.Status)
	require.Equal(t, 2, result.CurrentRound)
	require.Empty(t, result.SettlementReason)
	require.Nil(t, result.FinalResult)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreditLotterySingleDrawMigrationScopesIdempotencyAndLedgerUniqueness(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "210_credit_lottery_idempotency_and_ledger_uniqueness_notx.sql"))
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "on credit_lottery_sessions(user_id, session_date, start_idempotency_key)")
	require.Contains(t, sql, "drop index concurrently if exists credit_lottery_sessions_start_idempotency_unique")
	require.Contains(t, sql, "create unique index concurrently if not exists credit_lottery_sessions_start_idempotency_unique")
	require.Contains(t, sql, "create unique index concurrently if not exists credit_ledger_credit_lottery_source_unique")
	require.Contains(t, sql, "on credit_ledger(user_id, source_type, source_id)")
	require.Contains(t, sql, "where status <> 'reversed'")
	require.Contains(t, sql, "credit_lottery_entry")
	require.Contains(t, sql, "credit_lottery_reward")
	require.Contains(t, sql, "credit_lottery_jackpot_win")
	require.Contains(t, sql, "credit_lottery_jackpot_share")
}

func expectCreditLotterySessionLock(mock sqlmock.Sqlmock, now time.Time, status string, currentRound int) {
	mock.ExpectQuery(regexp.QuoteMeta(`
	SELECT id, user_id, mode, session_date::text, status, cost_credit::double precision, max_rounds, current_round,
	       COALESCE(settlement_reason, ''), balance_after_settlement::double precision, credit_balance_after_settlement::double precision, expires_at, created_at, updated_at
	FROM credit_lottery_sessions
	WHERE id = $1 AND user_id = $2
FOR UPDATE`)).
		WithArgs(int64(10), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"mode",
			"session_date",
			"status",
			"cost_credit",
			"max_rounds",
			"current_round",
			"settlement_reason",
			"balance_after_settlement",
			"credit_balance_after_settlement",
			"expires_at",
			"created_at",
			"updated_at",
		}).AddRow(int64(10), int64(7), "three_round", "2026-07-12", status, 20.0, 3, currentRound, "", 0.0, 80.0, now.Add(10*time.Minute), now, now))
}

func expectCreditLotterySessionSnapshot(mock sqlmock.Sqlmock, now time.Time, status string, currentRound int) {
	mock.ExpectQuery(regexp.QuoteMeta(`
	SELECT id, user_id, mode, session_date::text, status, cost_credit::double precision, max_rounds, current_round,
	       COALESCE(settlement_reason, ''), balance_after_settlement::double precision, credit_balance_after_settlement::double precision,
	       jackpot_hit, COALESCE(jackpot_pool, ''), jackpot_winner_amount::double precision,
	       jackpot_celebration_amount::double precision, jackpot_celebration_user_count, jackpot_payouts::text,
	       expires_at, settled_at, created_at, updated_at
	FROM credit_lottery_sessions
	WHERE id = $1 AND user_id = $2`)).
		WithArgs(int64(10), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"mode",
			"session_date",
			"status",
			"cost_credit",
			"max_rounds",
			"current_round",
			"settlement_reason",
			"balance_after_settlement",
			"credit_balance_after_settlement",
			"jackpot_hit",
			"jackpot_pool",
			"jackpot_winner_amount",
			"jackpot_celebration_amount",
			"jackpot_celebration_user_count",
			"jackpot_payouts",
			"expires_at",
			"settled_at",
			"created_at",
			"updated_at",
		}).AddRow(int64(10), int64(7), "three_round", "2026-07-12", status, 20.0, 3, currentRound, "", 0.0, 80.0, false, "", 0.0, 0.0, 0, "[]", now.Add(10*time.Minute), nil, now, now))
}

func expectCreditLotteryCurrentRound(mock sqlmock.Sqlmock, withCard bool) {
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, round_no, reward_asset, reward_amount::double precision, collectible_candidate_payload, is_current, superseded_at, created_at
FROM credit_lottery_rounds
WHERE session_id = $1 AND user_id = $2 AND is_current = TRUE`)).
		WithArgs(int64(10), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "round_no", "reward_asset", "reward_amount", "collectible_candidate_payload", "is_current", "superseded_at", "created_at"}).
			AddRow(int64(20), 2, "credit", 50.0, nil, true, nil, time.Date(2026, 7, 12, 12, 1, 0, 0, time.UTC)))
	if withCard {
		mock.ExpectQuery(`SELECT id, checkin_id, card_key, rarity, source_type, source_label, serial_no, edition_no, edition_supply, created_at\s+FROM checkin_collectible_cards`).
			WithArgs(int64(10), int64(20)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "checkin_id", "card_key", "rarity", "source_type", "source_label", "serial_no", "edition_no", "edition_supply", "created_at"}))
		return
	}
	mock.ExpectQuery(`SELECT id, checkin_id, card_key, rarity, source_type, source_label, serial_no, edition_no, edition_supply, created_at\s+FROM checkin_collectible_cards`).
		WithArgs(int64(10), int64(20)).
		WillReturnError(sql.ErrNoRows)
}

func expectCreditLotteryLatestRound(mock sqlmock.Sqlmock, noRows bool) {
	query := regexp.QuoteMeta(`
SELECT id, round_no, reward_asset, reward_amount::double precision, collectible_candidate_payload, is_current, superseded_at, created_at
FROM credit_lottery_rounds
WHERE session_id = $1 AND user_id = $2`) + ` ORDER BY round_no DESC LIMIT 1`
	if noRows {
		mock.ExpectQuery(query).
			WithArgs(int64(10), int64(7)).
			WillReturnError(sql.ErrNoRows)
		return
	}
	mock.ExpectQuery(query).
		WithArgs(int64(10), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "round_no", "reward_asset", "reward_amount", "collectible_candidate_payload", "is_current", "superseded_at", "created_at"}).
			AddRow(int64(20), 2, "credit", 50.0, nil, true, nil, time.Date(2026, 7, 12, 12, 1, 0, 0, time.UTC)))
}
