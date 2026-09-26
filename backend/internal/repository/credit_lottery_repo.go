package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creditLotteryRepository struct{ db *sql.DB }

func NewCreditLotteryRepository(db *sql.DB) service.CreditLotteryRepository {
	return &creditLotteryRepository{db: db}
}

func (r *creditLotteryRepository) CreateSession(ctx context.Context, input service.CreditLotteryCreateInput) (*service.CreditLotterySession, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("credit lottery repository is not configured")
	}
	mode := input.Mode
	if mode == "" {
		mode = service.CreditLotteryModeThreeRound
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if sessionID, ok, err := findCreditLotterySessionByStartKey(ctx, tx, input.UserID, input.SessionDate, input.IdempotencyKey); err != nil {
		return nil, err
	} else if ok {
		return r.getSessionTx(ctx, tx, input.UserID, sessionID)
	}
	if sessionID, ok, err := findActiveCreditLotterySession(ctx, tx, input.UserID); err != nil {
		return nil, err
	} else if ok {
		return r.getSessionTx(ctx, tx, input.UserID, sessionID)
	}

	jackpotPayouts, err := buildJackpotPayouts(ctx, tx, input.Jackpot)
	if err != nil {
		return nil, err
	}
	if err := lockJackpotPool(ctx, tx, "credit"); err != nil {
		return nil, err
	}

	var balance, creditBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&balance, &creditBalance); err != nil {
		return nil, err
	}

	if sessionID, ok, err := findCreditLotterySessionByStartKey(ctx, tx, input.UserID, input.SessionDate, input.IdempotencyKey); err != nil {
		return nil, err
	} else if ok {
		return r.getSessionTx(ctx, tx, input.UserID, sessionID)
	}
	if sessionID, ok, err := findActiveCreditLotterySession(ctx, tx, input.UserID); err != nil {
		return nil, err
	} else if ok {
		return r.getSessionTx(ctx, tx, input.UserID, sessionID)
	}

	var usedCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM credit_lottery_sessions WHERE user_id = $1 AND session_date = $2 AND status <> 'failed'`, input.UserID, input.SessionDate).Scan(&usedCount); err != nil {
		return nil, err
	}
	if usedCount >= 50 {
		return nil, service.ErrCreditLotteryDailyLimit
	}
	if creditBalance < input.CostCredit {
		return nil, service.ErrCheckinInsufficientPoint
	}
	creditAfterCost := creditBalance - input.CostCredit
	if _, err := tx.ExecContext(ctx, `UPDATE users SET credit_balance = $1, updated_at = NOW() WHERE id = $2`, creditAfterCost, input.UserID); err != nil {
		return nil, err
	}

	var sessionID int64
	if err := tx.QueryRowContext(ctx, `
	INSERT INTO credit_lottery_sessions
	(user_id, session_date, mode, status, cost_credit, max_rounds, current_round, balance_before, credit_balance_before, credit_balance_after_cost, balance_after_settlement, credit_balance_after_settlement, expires_at, start_idempotency_key)
	VALUES ($1, $2, $3, 'active', $4, $5, 1, $6, $7, $8, $6, $8, $9, $10)
	RETURNING id`, input.UserID, input.SessionDate, mode, input.CostCredit, input.MaxRounds, balance, creditBalance, creditAfterCost, input.Now.Add(10*time.Minute), input.IdempotencyKey).Scan(&sessionID); err != nil {
		return nil, err
	}
	roundID, err := insertCreditLotteryRound(ctx, tx, sessionID, input.UserID, input.FirstRound, true)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE credit_lottery_sessions SET current_round_id = $1, updated_at = NOW() WHERE id = $2`, roundID, sessionID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'credit_lottery_entry', $2, $3, $4, 'posted', $5, NOW()) ON CONFLICT DO NOTHING`, input.UserID, fmt.Sprintf("session:%d", sessionID), -input.CostCredit, creditAfterCost, "Credit lottery session entry"); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata) VALUES ('credit', 'credit_lottery', $1, 'contribution', $2, $3, '{}'::jsonb) ON CONFLICT DO NOTHING`, fmt.Sprintf("credit_lottery:%d", sessionID), input.CostCredit*0.20, input.UserID); err != nil {
		return nil, err
	}
	if mode == service.CreditLotteryModeSingle {
		if err := settleCreditLotteryWithPayoutsTx(ctx, tx, input.UserID, sessionID, service.CreditLotterySettlementSingleDraw, input.Jackpot, jackpotPayouts); err != nil {
			return nil, err
		}
	}
	session, err := r.getSessionTx(ctx, tx, input.UserID, sessionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return session, nil
}

func (r *creditLotteryRepository) GetActiveSession(ctx context.Context, userID int64, now time.Time) (*service.CreditLotterySession, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("credit lottery repository is not configured")
	}
	var sessionID int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM credit_lottery_sessions WHERE user_id = $1 AND status = 'active' ORDER BY id DESC LIMIT 1`, userID).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetSession(ctx, userID, sessionID)
}

func (r *creditLotteryRepository) GetSession(ctx context.Context, userID, sessionID int64) (*service.CreditLotterySession, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("credit lottery repository is not configured")
	}
	return r.getSession(ctx, userID, sessionID)
}

func (r *creditLotteryRepository) GetSessionByOperationID(ctx context.Context, userID int64, operationID string) (*service.CreditLotterySession, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("credit lottery repository is not configured")
	}
	var responseSnapshot string
	err := r.db.QueryRowContext(ctx, `
		SELECT response_snapshot::text
		FROM credit_lottery_operation_keys
		WHERE user_id = $1
		  AND idempotency_key = $2
		ORDER BY id DESC
		LIMIT 1`, userID, operationID).Scan(&responseSnapshot)
	if err == nil && strings.TrimSpace(responseSnapshot) != "" && strings.TrimSpace(responseSnapshot) != "{}" {
		var session service.CreditLotterySession
		if unmarshalErr := json.Unmarshal([]byte(responseSnapshot), &session); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		if session.ID > 0 && session.UserID == userID {
			return &session, nil
		}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var sessionID int64
	err = r.db.QueryRowContext(ctx, `
		SELECT id
		FROM credit_lottery_sessions
		WHERE user_id = $1
		  AND start_idempotency_key = $2
		ORDER BY session_date DESC, id DESC
		LIMIT 1`, userID, operationID).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetSession(ctx, userID, sessionID)
}

func (r *creditLotteryRepository) ContinueSession(ctx context.Context, input service.CreditLotteryContinueInput) (*service.CreditLotterySession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	session, err := lockCreditLotterySession(ctx, tx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	operation, err := reserveCreditLotteryOperationKey(ctx, tx, input.UserID, input.SessionID, "continue", input.IdempotencyKey, input.ExpectedRound)
	if err != nil {
		return nil, err
	}
	if operation.replay != nil {
		return operation.replay, nil
	}
	if operation.conflict {
		return r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
	}
	commitWithSnapshot := func(out *service.CreditLotterySession) (*service.CreditLotterySession, error) {
		if err := storeCreditLotteryOperationSnapshot(ctx, tx, operation.id, out); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return out, nil
	}
	if session.Status == service.CreditLotteryStatusSettled {
		out, err := r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
		if err != nil {
			return nil, err
		}
		return commitWithSnapshot(out)
	}
	if session.Status != service.CreditLotteryStatusActive {
		return nil, service.ErrCreditLotteryInvalidSession
	}
	if session.CurrentRound != input.ExpectedRound {
		out, err := r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
		if err != nil {
			return nil, err
		}
		return commitWithSnapshot(out)
	}
	if input.ExpectedRound >= session.MaxRounds {
		return nil, service.ErrCreditLotteryInvalidRound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE credit_lottery_rounds SET is_current = FALSE, superseded_at = NOW() WHERE session_id = $1 AND is_current = TRUE`, input.SessionID); err != nil {
		return nil, err
	}
	roundID, err := insertCreditLotteryRound(ctx, tx, input.SessionID, input.UserID, input.NextRound, true)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE credit_lottery_sessions SET current_round = $1, current_round_id = $2, updated_at = NOW() WHERE id = $3`, input.NextRound.RoundNo, roundID, input.SessionID); err != nil {
		return nil, err
	}
	if input.Finalize {
		if err := settleCreditLotteryTx(ctx, tx, input.UserID, input.SessionID, service.CreditLotterySettlementThirdRound, input.Jackpot); err != nil {
			return nil, err
		}
	}
	out, err := r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	return commitWithSnapshot(out)
}

func (r *creditLotteryRepository) SettleSession(ctx context.Context, input service.CreditLotterySettleInput) (*service.CreditLotterySession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	session, err := lockCreditLotterySession(ctx, tx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	if session.Status == service.CreditLotteryStatusActive && input.ExpectedRound > 0 && session.CurrentRound != input.ExpectedRound {
		out, err := r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return out, nil
	}
	var expectedRound any
	if input.ExpectedRound > 0 {
		expectedRound = input.ExpectedRound
	}
	operation, err := reserveCreditLotteryOperationKey(ctx, tx, input.UserID, input.SessionID, "settle", input.IdempotencyKey, expectedRound)
	if err != nil {
		return nil, err
	}
	if operation.replay != nil {
		return operation.replay, nil
	}
	if operation.conflict {
		return r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
	}
	if err := settleCreditLotteryTx(ctx, tx, input.UserID, input.SessionID, input.Reason, input.Jackpot); err != nil {
		return nil, err
	}
	out, err := r.getSessionTx(ctx, tx, input.UserID, input.SessionID)
	if err != nil {
		return nil, err
	}
	if err := storeCreditLotteryOperationSnapshot(ctx, tx, operation.id, out); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *creditLotteryRepository) ListExpiredSessions(ctx context.Context, now time.Time, limit int) ([]service.CreditLotterySession, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id FROM credit_lottery_sessions WHERE status = 'active' AND expires_at <= $1 ORDER BY expires_at ASC LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	type candidate struct {
		id     int64
		userID int64
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.userID); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]service.CreditLotterySession, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, service.CreditLotterySession{ID: c.id, UserID: c.userID})
	}
	return out, nil
}

func findCreditLotterySessionByStartKey(ctx context.Context, tx *sql.Tx, userID int64, sessionDate string, idempotencyKey string) (int64, bool, error) {
	if idempotencyKey == "" {
		return 0, false, nil
	}
	var sessionID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM credit_lottery_sessions WHERE user_id = $1 AND session_date = $2 AND start_idempotency_key = $3 ORDER BY id DESC LIMIT 1`, userID, sessionDate, idempotencyKey).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return sessionID, true, nil
}

func findActiveCreditLotterySession(ctx context.Context, tx *sql.Tx, userID int64) (int64, bool, error) {
	var sessionID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM credit_lottery_sessions WHERE user_id = $1 AND status = 'active' ORDER BY id DESC LIMIT 1`, userID).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return sessionID, true, nil
}

func insertCreditLotteryRound(ctx context.Context, tx *sql.Tx, sessionID, userID int64, plan service.CreditLotteryRoundPlan, isCurrent bool) (int64, error) {
	rewardPayload, err := json.Marshal(map[string]any{"asset": plan.RewardAsset, "amount": plan.RewardAmount})
	if err != nil {
		return 0, err
	}
	var candidatePayload any
	if plan.CollectibleCandidate != nil {
		payload, err := json.Marshal(plan.CollectibleCandidate)
		if err != nil {
			return 0, err
		}
		candidatePayload = string(payload)
	}
	var roundID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO credit_lottery_rounds (session_id, user_id, round_no, reward_asset, reward_amount, reward_payload, collectible_candidate_payload, is_current)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, NULLIF($7, '')::jsonb, $8)
RETURNING id`, sessionID, userID, plan.RoundNo, plan.RewardAsset, plan.RewardAmount, string(rewardPayload), creditLotteryJSONString(candidatePayload), isCurrent).Scan(&roundID)
	return roundID, err
}

func lockCreditLotterySession(ctx context.Context, tx *sql.Tx, userID, sessionID int64) (*service.CreditLotterySession, error) {
	session := &service.CreditLotterySession{}
	err := tx.QueryRowContext(ctx, `
	SELECT id, user_id, mode, session_date::text, status, cost_credit::double precision, max_rounds, current_round,
	       COALESCE(settlement_reason, ''), balance_after_settlement::double precision, credit_balance_after_settlement::double precision, expires_at, created_at, updated_at
	FROM credit_lottery_sessions
	WHERE id = $1 AND user_id = $2
	FOR UPDATE`, sessionID, userID).Scan(&session.ID, &session.UserID, &session.Mode, &session.SessionDate, &session.Status, &session.CostCredit, &session.MaxRounds, &session.CurrentRound, &session.SettlementReason, &session.BalanceAfter, &session.CreditBalanceAfter, &session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreditLotteryInvalidSession
	}
	return session, err
}

type creditLotteryOperationKeyState struct {
	id       int64
	conflict bool
	replay   *service.CreditLotterySession
}

func reserveCreditLotteryOperationKey(ctx context.Context, tx *sql.Tx, userID, sessionID int64, operation, idempotencyKey string, expectedRound any) (creditLotteryOperationKeyState, error) {
	if idempotencyKey == "" {
		return creditLotteryOperationKeyState{}, nil
	}
	var operationID int64
	err := tx.QueryRowContext(ctx, `
INSERT INTO credit_lottery_operation_keys (session_id, user_id, operation, idempotency_key, expected_round)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING
RETURNING id`, sessionID, userID, operation, idempotencyKey, expectedRound).Scan(&operationID)
	if err == nil {
		return creditLotteryOperationKeyState{id: operationID}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return creditLotteryOperationKeyState{}, err
	}
	replay, ok, err := readCreditLotteryOperationSnapshot(ctx, tx, userID, sessionID, operation, idempotencyKey)
	if err != nil {
		return creditLotteryOperationKeyState{}, err
	}
	if ok {
		return creditLotteryOperationKeyState{conflict: true, replay: replay}, nil
	}
	return creditLotteryOperationKeyState{conflict: true}, nil
}

func readCreditLotteryOperationSnapshot(ctx context.Context, tx *sql.Tx, userID, sessionID int64, operation, idempotencyKey string) (*service.CreditLotterySession, bool, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT response_snapshot::text FROM credit_lottery_operation_keys WHERE session_id = $1 AND user_id = $2 AND operation = $3 AND idempotency_key = $4`, sessionID, userID, operation, idempotencyKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return nil, false, nil
	}
	var session service.CreditLotterySession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, false, err
	}
	if session.ID == 0 {
		return nil, false, nil
	}
	return &session, true, nil
}

func storeCreditLotteryOperationSnapshot(ctx context.Context, tx *sql.Tx, operationID int64, session *service.CreditLotterySession) error {
	if operationID == 0 || session == nil {
		return nil
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE credit_lottery_operation_keys SET response_snapshot = $1::jsonb WHERE id = $2`, string(payload), operationID)
	return err
}

func settleCreditLotteryTx(ctx context.Context, tx *sql.Tx, userID, sessionID int64, reason string, jackpot service.JackpotPlan) error {
	jackpotPayouts, err := buildJackpotPayouts(ctx, tx, jackpot)
	if err != nil {
		return err
	}
	return settleCreditLotteryWithPayoutsTx(ctx, tx, userID, sessionID, reason, jackpot, jackpotPayouts)
}

func settleCreditLotteryWithPayoutsTx(ctx context.Context, tx *sql.Tx, userID, sessionID int64, reason string, jackpot service.JackpotPlan, jackpotPayouts []service.JackpotPayout) error {
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM credit_lottery_sessions WHERE id = $1 AND user_id = $2 FOR UPDATE`, sessionID, userID).Scan(&status); err != nil {
		return err
	}
	if status == service.CreditLotteryStatusSettled {
		return nil
	}
	if status != service.CreditLotteryStatusActive {
		return service.ErrCreditLotteryInvalidSession
	}
	round, err := currentCreditLotteryRound(ctx, tx, userID, sessionID)
	if err != nil {
		return err
	}
	var balance, creditBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&balance, &creditBalance); err != nil {
		return err
	}
	balanceAfter := balance
	roundCreditAfter := creditBalance + round.RewardAmount
	creditAfter := roundCreditAfter
	for _, payout := range jackpotPayouts {
		if payout.WinnerAmount <= 0 {
			continue
		}
		if payout.RewardAsset == "balance" {
			balanceAfter += payout.WinnerAmount
		} else {
			creditAfter += payout.WinnerAmount
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET balance = $1, credit_balance = $2, updated_at = NOW() WHERE id = $3`, balanceAfter, creditAfter, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'credit_lottery_reward', $2, $3, $4, 'posted', $5, NOW()) ON CONFLICT DO NOTHING`, userID, fmt.Sprintf("session:%d:round:%d", sessionID, round.ID), round.RewardAmount, roundCreditAfter, "Credit lottery final reward"); err != nil {
		return err
	}
	if jackpot.Hit {
		for _, payout := range jackpotPayouts {
			if payout.WinnerAmount <= 0 {
				continue
			}
			if payout.RewardAsset == "balance" {
				if err := insertUserBalanceLedger(ctx, tx, userID, "credit_lottery_jackpot_win", fmt.Sprintf("session:%d:jackpot:balance", sessionID), payout.WinnerAmount, balanceAfter, "Credit lottery jackpot winner"); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'credit_lottery_jackpot_win', $2, $3, $4, 'posted', $5, NOW()) ON CONFLICT DO NOTHING`, userID, fmt.Sprintf("session:%d:jackpot:credit", sessionID), payout.WinnerAmount, creditAfter, "Credit lottery jackpot winner"); err != nil {
				return err
			}
		}
	}
	if jackpot.Hit {
		jackpotPayouts, err = grantCreditLotteryJackpotCelebrations(ctx, tx, sessionID, userID, jackpotPayouts)
		if err != nil {
			return err
		}
		if err := recordCreditLotteryJackpotLedger(ctx, tx, sessionID, userID, jackpotPayouts); err != nil {
			return err
		}
	}
	collectibleID, err := issueCreditLotteryCollectible(ctx, tx, sessionID, userID, round)
	if err != nil {
		return err
	}
	legacyJackpot := summarizeJackpotPayouts(jackpotPayouts, "credit")
	jackpotPayoutsJSON, err := marshalJackpotPayouts(jackpotPayouts)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE credit_lottery_sessions
SET status = 'settled',
    final_round_id = $1,
    settlement_reason = $2,
    balance_after_settlement = $3,
    credit_balance_after_settlement = $4,
    jackpot_hit = $5,
    jackpot_pool = NULLIF($6, ''),
    jackpot_winner_amount = $7,
    jackpot_celebration_amount = $8,
    jackpot_celebration_user_count = $9,
    jackpot_payouts = $10::jsonb,
    collectible_card_id = NULLIF($11, 0),
    settled_at = NOW(),
    updated_at = NOW()
WHERE id = $12`, round.ID, reason, balanceAfter, creditAfter, jackpot.Hit, legacyJackpot.Pool, legacyJackpot.WinnerAmount, legacyJackpot.CelebrationAmount, legacyJackpot.CelebrationUserCount, jackpotPayoutsJSON, collectibleID, sessionID); err != nil {
		return err
	}
	return nil
}

func currentCreditLotteryRound(ctx context.Context, tx *sql.Tx, userID, sessionID int64) (*service.CreditLotteryRoundResult, error) {
	row := tx.QueryRowContext(ctx, `
SELECT id, round_no, reward_asset, reward_amount::double precision, collectible_candidate_payload, created_at
FROM credit_lottery_rounds
WHERE session_id = $1 AND user_id = $2 AND is_current = TRUE`, sessionID, userID)
	round := &service.CreditLotteryRoundResult{IsCurrent: true}
	var candidate sql.NullString
	if err := row.Scan(&round.ID, &round.RoundNo, &round.RewardAsset, &round.RewardAmount, &candidate, &round.CreatedAt); err != nil {
		return nil, err
	}
	if candidate.Valid && candidate.String != "" {
		var card service.CheckinCollectibleCard
		if err := json.Unmarshal([]byte(candidate.String), &card); err == nil && card.CardKey != "" {
			round.CollectibleCandidate = &card
		}
	}
	return round, nil
}

func issueCreditLotteryCollectible(ctx context.Context, tx *sql.Tx, sessionID, userID int64, round *service.CreditLotteryRoundResult) (int64, error) {
	if round == nil || round.CollectibleCandidate == nil {
		return 0, nil
	}
	card := *round.CollectibleCandidate
	editionNo, editionSupply, err := issueCollectibleEdition(ctx, tx, card.CardKey, card.Rarity, card.EditionSupply)
	if err != nil {
		if errors.Is(err, service.ErrCheckinCardSoldOut) {
			return 0, nil
		}
		return 0, err
	}
	card.EditionNo = editionNo
	card.EditionSupply = editionSupply
	var cardID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO checkin_collectible_cards
(user_id, checkin_id, card_key, rarity, source_type, source_label, edition_no, edition_supply, credit_lottery_session_id, credit_lottery_round_id)
VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id`, userID, card.CardKey, card.Rarity, card.SourceType, card.SourceLabel, card.EditionNo, card.EditionSupply, sessionID, round.ID).Scan(&cardID)
	if err != nil {
		return 0, err
	}
	round.CollectibleCard = &card
	round.CollectibleCard.ID = cardID
	return cardID, nil
}

func grantCreditLotteryJackpotCelebrations(ctx context.Context, tx *sql.Tx, sessionID, winnerUserID int64, payouts []service.JackpotPayout) ([]service.JackpotPayout, error) {
	out := append([]service.JackpotPayout(nil), payouts...)
	for i := range out {
		if out[i].CelebrationAmount <= 0 {
			continue
		}
		count, actualAmount, err := grantCreditLotteryCelebration(ctx, tx, sessionID, winnerUserID, out[i].PoolType, out[i].CelebrationAmount)
		if err != nil {
			return nil, err
		}
		out[i].CelebrationUserCount = count
		out[i].CelebrationActualAmount = actualAmount
	}
	return out, nil
}

func grantCreditLotteryCelebration(ctx context.Context, tx *sql.Tx, sessionID, winnerUserID int64, pool string, totalAmount float64) (int, float64, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, balance, credit_balance
FROM users
WHERE deleted_at IS NULL
  AND status = 'active'
  AND id <> $1
  AND last_active_at >= NOW() - INTERVAL '24 hours'
ORDER BY last_active_at DESC, id ASC
LIMIT 500
FOR UPDATE`, winnerUserID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	type activeUser struct {
		id            int64
		balance       float64
		creditBalance float64
	}
	var users []activeUser
	for rows.Next() {
		var u activeUser
		if err := rows.Scan(&u.id, &u.balance, &u.creditBalance); err != nil {
			return 0, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if len(users) == 0 {
		return 0, 0, nil
	}
	share := floorJackpotShareAmount(totalAmount / float64(len(users)))
	if share <= 0 {
		return 0, 0, nil
	}
	for _, u := range users {
		balanceAfter := u.balance
		creditAfter := u.creditBalance
		if pool == "balance" {
			balanceAfter += share
		} else {
			creditAfter += share
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance = $1, credit_balance = $2, updated_at = NOW() WHERE id = $3`, balanceAfter, creditAfter, u.id); err != nil {
			return 0, 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO credit_lottery_jackpot_shares (session_id, user_id, pool, amount, balance_after, credit_balance_after) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`, sessionID, u.id, pool, share, balanceAfter, creditAfter); err != nil {
			return 0, 0, err
		}
		if pool == "balance" {
			if err := insertUserBalanceLedger(ctx, tx, u.id, "credit_lottery_jackpot_share", fmt.Sprintf("session:%d:jackpot:balance:share:%d", sessionID, u.id), share, balanceAfter, "Credit lottery jackpot celebration share"); err != nil {
				return 0, 0, err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'credit_lottery_jackpot_share', $2, $3, $4, 'posted', $5, NOW()) ON CONFLICT DO NOTHING`, u.id, fmt.Sprintf("session:%d:jackpot:credit:share:%d", sessionID, u.id), share, creditAfter, "Credit lottery jackpot celebration share"); err != nil {
			return 0, 0, err
		}
	}
	return len(users), floorJackpotShareAmount(share * float64(len(users))), nil
}

func recordCreditLotteryJackpotLedger(ctx context.Context, tx *sql.Tx, sessionID, userID int64, payouts []service.JackpotPayout) error {
	sourceID := fmt.Sprintf("credit_lottery:%d", sessionID)
	for _, payout := range payouts {
		if payout.WinnerAmount > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata) VALUES ($1, 'credit_lottery', $2, 'payout', $3, $4, '{}'::jsonb) ON CONFLICT DO NOTHING`, payout.PoolType, sourceID, -payout.WinnerAmount, userID); err != nil {
				return err
			}
		}
		if payout.CelebrationActualAmount > 0 {
			metadata := fmt.Sprintf(`{"celebration_user_count":%d,"celebration_amount":%.2f}`, payout.CelebrationUserCount, payout.CelebrationAmount)
			if _, err := tx.ExecContext(ctx, `INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata) VALUES ($1, 'credit_lottery', $2, 'celebration', $3, $4, $5::jsonb) ON CONFLICT DO NOTHING`, payout.PoolType, sourceID, -payout.CelebrationActualAmount, userID, metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

func getJackpotLedgerPoolAmount(ctx context.Context, tx *sql.Tx, pool string) (float64, error) {
	base := 500.0
	if pool == "balance" {
		base = 50.0
	}
	var delta float64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0)::double precision FROM jackpot_ledger WHERE pool_type = $1`, pool).Scan(&delta); err != nil {
		return 0, err
	}
	amount := base + delta
	if amount < 0 {
		return 0, nil
	}
	return amount, nil
}

func lockJackpotPool(ctx context.Context, tx *sql.Tx, pool string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('jackpot_pool:' || $1))`, pool)
	return err
}

func (r *creditLotteryRepository) getSession(ctx context.Context, userID, sessionID int64) (*service.CreditLotterySession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	session, err := r.getSessionTx(ctx, tx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return session, nil
}

func (r *creditLotteryRepository) getSessionTx(ctx context.Context, tx *sql.Tx, userID, sessionID int64) (*service.CreditLotterySession, error) {
	session := &service.CreditLotterySession{}
	var settledAt sql.NullTime
	var jackpotPayouts sql.NullString
	err := tx.QueryRowContext(ctx, `
	SELECT id, user_id, mode, session_date::text, status, cost_credit::double precision, max_rounds, current_round,
	       COALESCE(settlement_reason, ''), balance_after_settlement::double precision, credit_balance_after_settlement::double precision,
	       jackpot_hit, COALESCE(jackpot_pool, ''), jackpot_winner_amount::double precision,
	       jackpot_celebration_amount::double precision, jackpot_celebration_user_count, jackpot_payouts::text,
	       expires_at, settled_at, created_at, updated_at
	FROM credit_lottery_sessions
	WHERE id = $1 AND user_id = $2`, sessionID, userID).Scan(&session.ID, &session.UserID, &session.Mode, &session.SessionDate, &session.Status, &session.CostCredit, &session.MaxRounds, &session.CurrentRound, &session.SettlementReason, &session.BalanceAfter, &session.CreditBalanceAfter, &session.JackpotHit, &session.JackpotPool, &session.JackpotWinnerAmount, &session.JackpotCelebrationAmount, &session.JackpotCelebrationUserCount, &jackpotPayouts, &session.ExpiresAt, &settledAt, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreditLotteryInvalidSession
	}
	if err != nil {
		return nil, err
	}
	if settledAt.Valid {
		session.SettledAt = &settledAt.Time
	}
	session.JackpotPayouts, err = scanJackpotPayouts(jackpotPayouts)
	if err != nil {
		return nil, err
	}
	current, _ := scanCreditLotteryRound(ctx, tx, session.ID, session.UserID, true)
	final, _ := scanCreditLotteryRound(ctx, tx, session.ID, session.UserID, false)
	session.CurrentResult = current
	if session.Status == service.CreditLotteryStatusSettled {
		session.FinalResult = final
		if session.FinalResult == nil {
			session.FinalResult = current
		}
	}
	return session, nil
}

func scanCreditLotteryRound(ctx context.Context, tx *sql.Tx, sessionID, userID int64, currentOnly bool) (*service.CreditLotteryRoundResult, error) {
	query := `
SELECT id, round_no, reward_asset, reward_amount::double precision, collectible_candidate_payload, is_current, superseded_at, created_at
FROM credit_lottery_rounds
WHERE session_id = $1 AND user_id = $2`
	if currentOnly {
		query += ` AND is_current = TRUE`
	} else {
		query += ` ORDER BY round_no DESC LIMIT 1`
	}
	var candidate sql.NullString
	var supersededAt sql.NullTime
	round := &service.CreditLotteryRoundResult{}
	err := tx.QueryRowContext(ctx, query, sessionID, userID).Scan(&round.ID, &round.RoundNo, &round.RewardAsset, &round.RewardAmount, &candidate, &round.IsCurrent, &supersededAt, &round.CreatedAt)
	if err != nil {
		return nil, err
	}
	if supersededAt.Valid {
		round.SupersededAt = &supersededAt.Time
	}
	if candidate.Valid && candidate.String != "" {
		var card service.CheckinCollectibleCard
		if err := json.Unmarshal([]byte(candidate.String), &card); err == nil && card.CardKey != "" {
			round.CollectibleCandidate = &card
		}
	}
	var card service.CheckinCollectibleCard
	var checkinID sql.NullInt64
	var editionNo sql.NullInt64
	var editionSupply sql.NullInt64
	err = tx.QueryRowContext(ctx, `
SELECT id, checkin_id, card_key, rarity, source_type, source_label, serial_no, edition_no, edition_supply, created_at
FROM checkin_collectible_cards
WHERE credit_lottery_session_id = $1 AND credit_lottery_round_id = $2
LIMIT 1`, sessionID, round.ID).Scan(&card.ID, &checkinID, &card.CardKey, &card.Rarity, &card.SourceType, &card.SourceLabel, &card.SerialNo, &editionNo, &editionSupply, &card.CreatedAt)
	if err == nil {
		if checkinID.Valid {
			card.CheckinID = checkinID.Int64
		}
		if editionNo.Valid {
			card.EditionNo = editionNo.Int64
		}
		if editionSupply.Valid {
			card.EditionSupply = editionSupply.Int64
		}
		round.CollectibleCard = &card
	}
	return round, nil
}

func creditLotteryJSONString(value any) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprint(value)
}
