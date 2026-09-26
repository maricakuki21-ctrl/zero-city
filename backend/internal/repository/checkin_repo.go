package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type checkinRepository struct{ db *sql.DB }

func NewCheckinRepository(db *sql.DB) service.CheckinRepository {
	return &checkinRepository{db: db}
}

func (r *checkinRepository) GetDailyStatus(ctx context.Context, userID int64, day time.Time) (*service.CheckinStatus, error) {
	status := &service.CheckinStatus{
		Date:             formatCheckinDate(day),
		CreditLimit:      50,
		BalanceLimit:     10,
		PaidCost:         20,
		CreditCost:       20,
		BalanceCost:      5,
		FreeRewardMin:    5,
		FreeRewardMax:    15,
		PaidRewardMin:    5,
		PaidRewardMax:    100,
		CreditRewardMin:  5,
		CreditRewardMax:  100,
		BalanceRewardMin: 1,
		BalanceRewardMax: 20,
	}
	if r == nil || r.db == nil {
		return status, nil
	}
	if err := r.db.QueryRowContext(ctx, `SELECT balance, credit_balance FROM users WHERE id = $1`, userID).Scan(&status.Balance, &status.CreditBalance); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT GREATEST(0, 500 + COALESCE(SUM(amount), 0))
		FROM jackpot_ledger
		WHERE pool_type = 'credit'
	`).Scan(&status.CreditJackpot); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT GREATEST(0, 50 + COALESCE(SUM(amount), 0))
		FROM jackpot_ledger
		WHERE pool_type = 'balance'
	`).Scan(&status.BalanceJackpot); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT checkin_date) FROM daily_checkins WHERE user_id = $1`, userID).Scan(&status.StreakDays); err != nil {
		return nil, err
	}
	if err := r.populateMilestoneStatus(ctx, userID, status); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT checkin_type, COUNT(*) FROM daily_checkins WHERE user_id = $1 AND checkin_date = $2 GROUP BY checkin_type`, userID, formatCheckinDate(day))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var checkinType string
		var count int
		if err := rows.Scan(&checkinType, &count); err != nil {
			return nil, err
		}
		switch checkinType {
		case service.CheckinTypeFree:
			status.FreeClaimed = count > 0
		case service.CheckinTypeBalance:
			status.BalanceCount += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var creditLotteryCount int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM credit_lottery_sessions WHERE user_id = $1 AND session_date = $2 AND status <> 'failed'`, userID, formatCheckinDate(day)).Scan(&creditLotteryCount); err != nil {
		return nil, err
	}
	status.CreditCount += creditLotteryCount
	status.PaidClaimed = status.CreditCount >= status.CreditLimit
	return status, nil
}

func (r *checkinRepository) ClaimDaily(ctx context.Context, input service.CheckinClaimInput) (*service.CheckinResult, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("checkin repository is not configured")
	}
	if input.OperationID != "" {
		existing, err := getCheckinOperationResult(ctx, r.db, input.UserID, input.OperationID)
		if err == nil {
			if !checkinOperationMatchesInput(existing, input) {
				return nil, service.ErrCheckinOperationConflict
			}
			return existing, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	jackpotPayouts, err := buildJackpotPayouts(ctx, tx, input.Jackpot)
	if err != nil {
		return nil, err
	}
	if pool, contribution := dailyCheckinJackpotContribution(input); contribution > 0 && !input.Jackpot.Hit {
		if err := lockJackpotPool(ctx, tx, pool); err != nil {
			return nil, err
		}
	}

	var balance, creditBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&balance, &creditBalance); err != nil {
		return nil, err
	}
	if input.OperationID != "" {
		existing, err := getCheckinOperationResult(ctx, tx, input.UserID, input.OperationID)
		if err == nil {
			if !checkinOperationMatchesInput(existing, input) {
				return nil, service.ErrCheckinOperationConflict
			}
			return existing, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	var claimedCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_checkins WHERE user_id = $1 AND checkin_date = $2 AND checkin_type = $3`, input.UserID, formatCheckinDate(input.Date), input.Type).Scan(&claimedCount); err != nil {
		return nil, err
	}
	if claimedCount >= input.DailyLimit {
		return nil, service.ErrCheckinAlreadyClaimed
	}
	if input.CostAsset == "credit" && creditBalance < input.Cost {
		return nil, service.ErrCheckinInsufficientPoint
	}
	if input.CostAsset == "balance" && balance < input.Cost {
		return nil, service.ErrCheckinInsufficientBalance
	}

	balanceAfter := balance
	creditAfter := creditBalance
	if input.CostAsset == "credit" {
		creditAfter -= input.Cost
	}
	if input.CostAsset == "balance" {
		balanceAfter -= input.Cost
	}
	if input.RewardAsset == "credit" {
		creditAfter += input.Reward
	}
	if input.RewardAsset == "balance" {
		balanceAfter += input.Reward
	}

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

	if _, err := tx.ExecContext(ctx, `UPDATE users SET balance = $1, credit_balance = $2, updated_at = NOW() WHERE id = $3`, balanceAfter, creditAfter, input.UserID); err != nil {
		return nil, err
	}

	legacyJackpot := summarizeJackpotPayouts(jackpotPayouts, legacyJackpotPoolForCheckin(input.Type))
	jackpotPayoutsJSON, err := marshalJackpotPayouts(jackpotPayouts)
	if err != nil {
		return nil, err
	}
	var checkinID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO daily_checkins (user_id, operation_id, checkin_date, checkin_type, point_cost, credit_reward, balance_after, credit_balance_after, jackpot_hit, jackpot_pool, jackpot_amount, jackpot_winner_amount, jackpot_celebration_amount, jackpot_payouts) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11, $12, $13, $14::jsonb) RETURNING id`, input.UserID, input.OperationID, formatCheckinDate(input.Date), input.Type, input.Cost, input.Reward, balanceAfter, creditAfter, input.Jackpot.Hit, legacyJackpot.Pool, legacyJackpot.TotalAmount, legacyJackpot.WinnerAmount, legacyJackpot.CelebrationAmount, jackpotPayoutsJSON).Scan(&checkinID); err != nil {
		return nil, err
	}
	creditDelta := 0.0
	if input.CostAsset == "credit" {
		creditDelta -= input.Cost
	}
	if input.RewardAsset == "credit" {
		creditDelta += input.Reward
	}
	for _, payout := range jackpotPayouts {
		if payout.RewardAsset == "credit" {
			creditDelta += payout.WinnerAmount
		}
	}
	if creditDelta != 0 {
		note := fmt.Sprintf("Daily %s checkin credit delta", input.Type)
		if input.Jackpot.Hit {
			note = fmt.Sprintf("Daily %s checkin credit delta with jackpot", input.Type)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'daily_checkin', $2, $3, $4, 'posted', $5, NOW())`, input.UserID, fmt.Sprintf("%s:%s:%d", formatCheckinDate(input.Date), input.Type, checkinID), creditDelta, creditAfter, note); err != nil {
			return nil, err
		}
	}
	balanceDelta := 0.0
	if input.CostAsset == "balance" {
		balanceDelta -= input.Cost
	}
	if input.RewardAsset == "balance" {
		balanceDelta += input.Reward
	}
	for _, payout := range jackpotPayouts {
		if payout.RewardAsset == "balance" {
			balanceDelta += payout.WinnerAmount
		}
	}
	if balanceDelta != 0 {
		note := fmt.Sprintf("Daily %s checkin balance delta", input.Type)
		if input.Jackpot.Hit {
			note = fmt.Sprintf("Daily %s checkin balance delta with jackpot", input.Type)
		}
		if err := insertUserBalanceLedger(ctx, tx, input.UserID, "daily_checkin", fmt.Sprintf("%s:%s:%d", formatCheckinDate(input.Date), input.Type, checkinID), balanceDelta, balanceAfter, note); err != nil {
			return nil, err
		}
	}
	var collectible *service.CheckinCollectibleCard
	if input.Collectible != nil {
		card := *input.Collectible
		if card.SourceType == "" {
			card.SourceType = "daily_fortune"
		}
		if card.SourceLabel == "" {
			card.SourceLabel = input.Type
		}
		editionNo, editionSupply, err := issueCollectibleEdition(ctx, tx, card.CardKey, card.Rarity, card.EditionSupply)
		if err != nil {
			if !errors.Is(err, service.ErrCheckinCardSoldOut) {
				return nil, err
			}
		} else {
			card.EditionNo = editionNo
			card.EditionSupply = editionSupply
			if err := tx.QueryRowContext(ctx, `INSERT INTO checkin_collectible_cards (user_id, checkin_id, card_key, rarity, source_type, source_label, edition_no, edition_supply) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, serial_no, created_at`, input.UserID, checkinID, card.CardKey, card.Rarity, card.SourceType, card.SourceLabel, card.EditionNo, card.EditionSupply).Scan(&card.ID, &card.SerialNo, &card.CreatedAt); err != nil {
				return nil, err
			}
			card.CheckinID = checkinID
			collectible = &card
		}
	}

	if input.Jackpot.Hit {
		jackpotPayouts, err = grantDailyCheckinJackpotCelebrations(ctx, tx, checkinID, input.UserID, jackpotPayouts)
		if err != nil {
			return nil, err
		}
		legacyJackpot = summarizeJackpotPayouts(jackpotPayouts, legacyJackpotPoolForCheckin(input.Type))
		jackpotPayoutsJSON, err = marshalJackpotPayouts(jackpotPayouts)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE daily_checkins SET jackpot_amount = $1, jackpot_celebration_amount = $2, jackpot_celebration_user_count = $3, jackpot_payouts = $4::jsonb WHERE id = $5`, legacyJackpot.TotalAmount, legacyJackpot.CelebrationAmount, legacyJackpot.CelebrationUserCount, jackpotPayoutsJSON, checkinID); err != nil {
			return nil, err
		}
	}
	if err := recordDailyCheckinJackpotLedger(ctx, tx, input, checkinID, jackpotPayouts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.CheckinResult{
		OperationID:                 input.OperationID,
		Type:                        input.Type,
		Date:                        formatCheckinDate(input.Date),
		Cost:                        input.Cost,
		Reward:                      input.Reward,
		RewardAsset:                 input.RewardAsset,
		BalanceAfter:                balanceAfter,
		CreditBalanceAfter:          creditAfter,
		AlreadyClaimed:              false,
		JackpotHit:                  input.Jackpot.Hit,
		JackpotPool:                 legacyJackpot.Pool,
		JackpotAmount:               legacyJackpot.TotalAmount,
		JackpotWinnerAmount:         legacyJackpot.WinnerAmount,
		JackpotCelebrationAmount:    legacyJackpot.CelebrationAmount,
		JackpotCelebrationUserCount: legacyJackpot.CelebrationUserCount,
		JackpotPayouts:              jackpotPayouts,
		CollectibleCard:             collectible,
	}, nil
}

func (r *checkinRepository) GetOperationResult(ctx context.Context, userID int64, operationID string) (*service.CheckinResult, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("checkin repository is not configured")
	}
	result, err := getCheckinOperationResult(ctx, r.db, userID, operationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCheckinOperationNotFound
	}
	return result, err
}

func getCheckinOperationResult(ctx context.Context, q checkinQueryer, userID int64, operationID string) (*service.CheckinResult, error) {
	result := &service.CheckinResult{}
	var checkinID int64
	var jackpotPayouts sql.NullString
	var cardID sql.NullInt64
	var cardKey, rarity, sourceType, sourceLabel sql.NullString
	var serialNo, editionNo, editionSupply sql.NullInt64
	var cardCreatedAt sql.NullTime
	err := q.QueryRowContext(ctx, `
		SELECT
			dc.id,
			dc.operation_id,
			dc.checkin_type,
			dc.checkin_date::text,
			dc.point_cost::double precision,
			dc.credit_reward::double precision,
			CASE WHEN dc.checkin_type = 'balance' THEN 'balance' ELSE 'credit' END,
			dc.balance_after::double precision,
			dc.credit_balance_after::double precision,
			dc.jackpot_hit,
			COALESCE(dc.jackpot_pool, ''),
			dc.jackpot_amount::double precision,
			dc.jackpot_winner_amount::double precision,
			dc.jackpot_celebration_amount::double precision,
			dc.jackpot_celebration_user_count,
			dc.jackpot_payouts::text,
			cc.id,
			cc.card_key,
			cc.rarity,
			cc.source_type,
			cc.source_label,
			cc.serial_no,
			cc.edition_no,
			cc.edition_supply,
			cc.created_at
		FROM daily_checkins dc
		LEFT JOIN checkin_collectible_cards cc
		  ON cc.checkin_id = dc.id
		 AND cc.user_id = dc.user_id
		WHERE dc.user_id = $1
		  AND dc.operation_id = $2
		ORDER BY cc.id DESC
		LIMIT 1`, userID, operationID).Scan(
		&checkinID,
		&result.OperationID,
		&result.Type,
		&result.Date,
		&result.Cost,
		&result.Reward,
		&result.RewardAsset,
		&result.BalanceAfter,
		&result.CreditBalanceAfter,
		&result.JackpotHit,
		&result.JackpotPool,
		&result.JackpotAmount,
		&result.JackpotWinnerAmount,
		&result.JackpotCelebrationAmount,
		&result.JackpotCelebrationUserCount,
		&jackpotPayouts,
		&cardID,
		&cardKey,
		&rarity,
		&sourceType,
		&sourceLabel,
		&serialNo,
		&editionNo,
		&editionSupply,
		&cardCreatedAt,
	)
	if err != nil {
		return nil, err
	}
	result.JackpotPayouts, err = scanJackpotPayouts(jackpotPayouts)
	if err != nil {
		return nil, err
	}
	if cardID.Valid {
		result.CollectibleCard = &service.CheckinCollectibleCard{
			ID:            cardID.Int64,
			CheckinID:     checkinID,
			CardKey:       cardKey.String,
			Rarity:        rarity.String,
			SourceType:    sourceType.String,
			SourceLabel:   sourceLabel.String,
			SerialNo:      serialNo.Int64,
			EditionNo:     editionNo.Int64,
			EditionSupply: editionSupply.Int64,
			CreatedAt:     cardCreatedAt.Time,
		}
	}
	return result, nil
}

func checkinOperationMatchesInput(result *service.CheckinResult, input service.CheckinClaimInput) bool {
	return result != nil &&
		result.Type == input.Type &&
		result.Date == formatCheckinDate(input.Date)
}

func (r *checkinRepository) populateMilestoneStatus(ctx context.Context, userID int64, status *service.CheckinStatus) error {
	cycleNo, cycleDay := checkinMilestoneCycle(status.StreakDays)
	status.CycleNo = cycleNo
	status.CycleDay = cycleDay
	claimed := map[int]bool{}
	rows, err := r.db.QueryContext(ctx, `SELECT milestone_days FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2`, userID, cycleNo)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var days int
		if err := rows.Scan(&days); err != nil {
			return err
		}
		claimed[days] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	verified, hasUsage := false, false
	activationLoaded := false
	for _, rule := range checkinMilestoneRules() {
		eligible := cycleDay >= rule.days
		activationRequired := false
		if eligible && (rule.requiresVerified || rule.requiresAPIUsage) {
			if !activationLoaded {
				verified, hasUsage, err = getCheckinMilestoneActivation(ctx, r.db, userID)
				if err != nil {
					return err
				}
				activationLoaded = true
			}
			activationRequired = (rule.requiresVerified && !verified) || (rule.requiresAPIUsage && !hasUsage)
		}
		isClaimed := claimed[rule.days]
		status.Milestones = append(status.Milestones, service.CheckinMilestoneStatus{
			Days:               rule.days,
			CreditReward:       rule.creditReward,
			BalanceReward:      0,
			Eligible:           eligible,
			Claimed:            isClaimed,
			Claimable:          eligible && !isClaimed && !activationRequired,
			RequiresVerified:   rule.requiresVerified,
			RequiresAPIUsage:   rule.requiresAPIUsage,
			ActivationRequired: activationRequired,
		})
	}
	return nil
}

type checkinMilestoneRule struct {
	days             int
	creditReward     float64
	requiresVerified bool
	requiresAPIUsage bool
}

func checkinMilestoneRules() []checkinMilestoneRule {
	return []checkinMilestoneRule{
		{days: 3, creditReward: 30},
		{days: 5, creditReward: 50},
		{days: 7, creditReward: 100},
	}
}

func checkinMilestoneCycle(totalDays int) (cycleNo int, cycleDay int) {
	if totalDays <= 0 {
		return 0, 0
	}
	cycleNo = ((totalDays - 1) / 7) + 1
	cycleDay = totalDays % 7
	if cycleDay == 0 {
		cycleDay = 7
	}
	return cycleNo, cycleDay
}

func getCheckinMilestoneRule(days int) (checkinMilestoneRule, bool) {
	for _, rule := range checkinMilestoneRules() {
		if rule.days == days {
			return rule, true
		}
	}
	return checkinMilestoneRule{}, false
}

type checkinQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getCheckinMilestoneActivation(ctx context.Context, q checkinQueryer, userID int64) (bool, bool, error) {
	var verified bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM auth_identities WHERE user_id = $1 AND verified_at IS NOT NULL)`, userID).Scan(&verified); err != nil {
		return false, false, err
	}
	var hasUsage bool
	if err := q.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM usage_logs
			WHERE user_id = $1
			  AND (
				COALESCE(total_cost, 0) > 0 OR
				COALESCE(actual_cost, 0) > 0 OR
				COALESCE(input_tokens, 0) > 0 OR
				COALESCE(output_tokens, 0) > 0 OR
				COALESCE(image_count, 0) > 0
			  )
			LIMIT 1
		)`, userID).Scan(&hasUsage); err != nil {
		return false, false, err
	}
	return verified, hasUsage, nil
}

func (r *checkinRepository) ClaimMilestone(ctx context.Context, userID int64, milestoneDays int) (*service.CheckinMilestoneClaimResult, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("checkin repository is not configured")
	}
	rule, ok := getCheckinMilestoneRule(milestoneDays)
	if !ok {
		return nil, service.ErrCheckinMilestoneInvalid
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var balance, creditBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&balance, &creditBalance); err != nil {
		return nil, err
	}

	var totalDays int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT checkin_date) FROM daily_checkins WHERE user_id = $1`, userID).Scan(&totalDays); err != nil {
		return nil, err
	}
	cycleNo, cycleDay := checkinMilestoneCycle(totalDays)
	if cycleDay < rule.days {
		return nil, service.ErrCheckinMilestoneNotEligible
	}

	var alreadyClaimed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM checkin_milestone_claims WHERE user_id = $1 AND cycle_no = $2 AND milestone_days = $3)`, userID, cycleNo, rule.days).Scan(&alreadyClaimed); err != nil {
		return nil, err
	}
	if alreadyClaimed {
		return nil, service.ErrCheckinMilestoneAlreadyClaimed
	}

	if rule.requiresVerified || rule.requiresAPIUsage {
		verified, hasUsage, err := getCheckinMilestoneActivation(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		if (rule.requiresVerified && !verified) || (rule.requiresAPIUsage && !hasUsage) {
			return nil, service.ErrCheckinMilestoneActivationRequired
		}
	}

	// Milestones award points only. Keep cash and historical cash ledgers untouched.
	balanceAfter := balance
	creditAfter := creditBalance + rule.creditReward
	var claimID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO checkin_milestone_claims (user_id, cycle_no, milestone_days, credit_reward, balance_reward, balance_after, credit_balance_after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`, userID, cycleNo, rule.days, rule.creditReward, 0.0, balanceAfter, creditAfter).Scan(&claimID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrCheckinMilestoneAlreadyClaimed
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET credit_balance = $1, updated_at = NOW() WHERE id = $2`, creditAfter, userID); err != nil {
		return nil, err
	}

	if rule.creditReward > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'checkin_milestone', $2, $3, $4, 'posted', $5, NOW())`, userID, fmt.Sprintf("%d:%d:%d", cycleNo, rule.days, claimID), rule.creditReward, creditAfter, fmt.Sprintf("Daily fortune cycle %d %d-day milestone gift", cycleNo, rule.days)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.CheckinMilestoneClaimResult{
		MilestoneDays:      rule.days,
		CreditReward:       rule.creditReward,
		BalanceReward:      0,
		BalanceAfter:       balanceAfter,
		CreditBalanceAfter: creditAfter,
	}, nil
}

func (r *checkinRepository) BuyCollectibleCard(ctx context.Context, input service.CheckinCardPurchaseInput) (*service.CheckinCardPurchaseResult, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("checkin repository is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var balance, creditBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT balance, credit_balance FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&balance, &creditBalance); err != nil {
		return nil, err
	}
	if creditBalance < input.Price {
		return nil, service.ErrCheckinInsufficientPoint
	}

	card := service.CheckinCollectibleCard{
		CardKey:       input.CardKey,
		Rarity:        input.Rarity,
		SourceType:    input.SourceType,
		SourceLabel:   input.SourceLabel,
		EditionSupply: input.EditionSupply,
	}
	if card.SourceType == "" {
		card.SourceType = "card_shop"
	}
	if card.SourceLabel == "" {
		card.SourceLabel = "credit_shop"
	}
	editionNo, editionSupply, err := issueCollectibleEdition(ctx, tx, card.CardKey, card.Rarity, card.EditionSupply)
	if err != nil {
		return nil, err
	}
	card.EditionNo = editionNo
	card.EditionSupply = editionSupply

	creditAfter := creditBalance - input.Price
	if _, err := tx.ExecContext(ctx, `UPDATE users SET credit_balance = $1, updated_at = NOW() WHERE id = $2`, creditAfter, input.UserID); err != nil {
		return nil, err
	}

	if err := tx.QueryRowContext(ctx, `
		INSERT INTO checkin_collectible_cards
		(user_id, checkin_id, card_key, rarity, source_type, source_label, edition_no, edition_supply)
		VALUES ($1, NULL, $2, $3, $4, $5, $6, $7)
		RETURNING id, serial_no, created_at`,
		input.UserID,
		card.CardKey,
		card.Rarity,
		card.SourceType,
		card.SourceLabel,
		card.EditionNo,
		card.EditionSupply,
	).Scan(&card.ID, &card.SerialNo, &card.CreatedAt); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'checkin_card_shop', $2, $3, $4, 'posted', $5, NOW())`, input.UserID, fmt.Sprintf("card:%d", card.ID), -input.Price, creditAfter, fmt.Sprintf("Zero City %s card shop purchase", input.Rarity)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.CheckinCardPurchaseResult{
		Rarity:             input.Rarity,
		Price:              input.Price,
		BalanceAfter:       balance,
		CreditBalanceAfter: creditAfter,
		CollectibleCard:    card,
	}, nil
}

func issueCollectibleEdition(ctx context.Context, tx *sql.Tx, cardKey, rarity string, maxSupply int64) (int64, int64, error) {
	if cardKey == "" || rarity == "" || maxSupply <= 0 {
		return 0, 0, service.ErrCheckinCardSoldOut
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO checkin_collectible_card_editions (card_key, rarity, max_supply, issued_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (card_key) DO UPDATE
		SET rarity = EXCLUDED.rarity,
			max_supply = EXCLUDED.max_supply,
			updated_at = NOW()
		WHERE checkin_collectible_card_editions.issued_count <= EXCLUDED.max_supply
	`, cardKey, rarity, maxSupply); err != nil {
		return 0, 0, err
	}
	var editionNo, editionSupply int64
	err := tx.QueryRowContext(ctx, `
		UPDATE checkin_collectible_card_editions
		SET issued_count = issued_count + 1,
			updated_at = NOW()
		WHERE card_key = $1
		  AND issued_count < max_supply
		RETURNING issued_count, max_supply
	`, cardKey).Scan(&editionNo, &editionSupply)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, service.ErrCheckinCardSoldOut
	}
	if err != nil {
		return 0, 0, err
	}
	return editionNo, editionSupply, nil
}

func (r *checkinRepository) ListDailyRecords(ctx context.Context, userID int64, limit int) ([]service.CheckinRecord, error) {
	if r == nil || r.db == nil {
		return []service.CheckinRecord{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			dc.id,
			dc.checkin_date::text,
			dc.checkin_type,
			dc.point_cost,
			dc.credit_reward,
			CASE WHEN dc.checkin_type = 'balance' THEN 'balance' ELSE 'credit' END AS reward_asset,
			dc.jackpot_hit,
			dc.jackpot_pool,
			dc.jackpot_winner_amount,
			dc.jackpot_payouts::text,
			dc.created_at,
			cc.id,
			cc.card_key,
			cc.rarity,
			cc.source_type,
			cc.source_label,
			cc.serial_no,
			cc.edition_no,
			cc.edition_supply,
			cc.created_at
		FROM daily_checkins dc
		LEFT JOIN checkin_collectible_cards cc ON cc.checkin_id = dc.id AND cc.user_id = dc.user_id
		WHERE dc.user_id = $1
		ORDER BY dc.created_at DESC, dc.id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]service.CheckinRecord, 0, limit)
	for rows.Next() {
		var record service.CheckinRecord
		var jackpotPool sql.NullString
		var jackpotWinner sql.NullFloat64
		var jackpotPayouts sql.NullString
		var cardID sql.NullInt64
		var cardKey, rarity, sourceType, sourceLabel sql.NullString
		var serialNo, editionNo, editionSupply sql.NullInt64
		var cardCreatedAt sql.NullTime
		if err := rows.Scan(&record.ID, &record.Date, &record.Type, &record.Cost, &record.Reward, &record.RewardAsset, &record.JackpotHit, &jackpotPool, &jackpotWinner, &jackpotPayouts, &record.CreatedAt, &cardID, &cardKey, &rarity, &sourceType, &sourceLabel, &serialNo, &editionNo, &editionSupply, &cardCreatedAt); err != nil {
			return nil, err
		}
		if jackpotPool.Valid {
			record.JackpotPool = jackpotPool.String
		}
		if jackpotWinner.Valid {
			record.JackpotWinnerAmount = jackpotWinner.Float64
		}
		record.JackpotPayouts, err = scanJackpotPayouts(jackpotPayouts)
		if err != nil {
			return nil, err
		}
		if cardID.Valid {
			record.CollectibleCard = &service.CheckinCollectibleCard{
				ID:            cardID.Int64,
				CheckinID:     record.ID,
				CardKey:       cardKey.String,
				Rarity:        rarity.String,
				SourceType:    sourceType.String,
				SourceLabel:   sourceLabel.String,
				SerialNo:      serialNo.Int64,
				EditionNo:     editionNo.Int64,
				EditionSupply: editionSupply.Int64,
				CreatedAt:     cardCreatedAt.Time,
			}
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	lotteryRecords, err := r.listCreditLotteryRecords(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	records = append(records, lotteryRecords...)
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (r *checkinRepository) listCreditLotteryRecords(ctx context.Context, userID int64, limit int) ([]service.CheckinRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			-cls.id,
			cls.session_date::text,
			'credit',
			cls.cost_credit,
			clr.reward_amount,
			clr.reward_asset,
			cls.jackpot_hit,
			cls.jackpot_pool,
			cls.jackpot_winner_amount,
			cls.jackpot_payouts::text,
			COALESCE(cls.settled_at, cls.updated_at),
			cc.id,
			cc.card_key,
			cc.rarity,
			cc.source_type,
			cc.source_label,
			cc.serial_no,
			cc.edition_no,
			cc.edition_supply,
			cc.created_at
		FROM credit_lottery_sessions cls
		JOIN credit_lottery_rounds clr ON clr.id = cls.final_round_id
		LEFT JOIN checkin_collectible_cards cc ON cc.credit_lottery_session_id = cls.id AND cc.credit_lottery_round_id = clr.id AND cc.user_id = cls.user_id
		WHERE cls.user_id = $1 AND cls.status = 'settled'
		ORDER BY COALESCE(cls.settled_at, cls.updated_at) DESC, cls.id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	records := make([]service.CheckinRecord, 0, limit)
	for rows.Next() {
		var record service.CheckinRecord
		var jackpotPool sql.NullString
		var jackpotWinner sql.NullFloat64
		var jackpotPayouts sql.NullString
		var cardID sql.NullInt64
		var cardKey, rarity, sourceType, sourceLabel sql.NullString
		var serialNo, editionNo, editionSupply sql.NullInt64
		var cardCreatedAt sql.NullTime
		if err := rows.Scan(&record.ID, &record.Date, &record.Type, &record.Cost, &record.Reward, &record.RewardAsset, &record.JackpotHit, &jackpotPool, &jackpotWinner, &jackpotPayouts, &record.CreatedAt, &cardID, &cardKey, &rarity, &sourceType, &sourceLabel, &serialNo, &editionNo, &editionSupply, &cardCreatedAt); err != nil {
			return nil, err
		}
		if jackpotPool.Valid {
			record.JackpotPool = jackpotPool.String
		}
		if jackpotWinner.Valid {
			record.JackpotWinnerAmount = jackpotWinner.Float64
		}
		record.JackpotPayouts, err = scanJackpotPayouts(jackpotPayouts)
		if err != nil {
			return nil, err
		}
		if cardID.Valid {
			record.CollectibleCard = &service.CheckinCollectibleCard{
				ID:            cardID.Int64,
				CardKey:       cardKey.String,
				Rarity:        rarity.String,
				SourceType:    sourceType.String,
				SourceLabel:   sourceLabel.String,
				SerialNo:      serialNo.Int64,
				EditionNo:     editionNo.Int64,
				EditionSupply: editionSupply.Int64,
				CreatedAt:     cardCreatedAt.Time,
			}
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *checkinRepository) ListCollectibleCards(ctx context.Context, userID int64, limit int) ([]service.CheckinCollectibleCard, error) {
	if r == nil || r.db == nil {
		return []service.CheckinCollectibleCard{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 60
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, checkin_id, card_key, rarity, source_type, source_label, serial_no, edition_no, edition_supply, created_at
		FROM checkin_collectible_cards
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	cards := make([]service.CheckinCollectibleCard, 0, limit)
	for rows.Next() {
		var card service.CheckinCollectibleCard
		var checkinID sql.NullInt64
		var editionNo, editionSupply sql.NullInt64
		if err := rows.Scan(&card.ID, &checkinID, &card.CardKey, &card.Rarity, &card.SourceType, &card.SourceLabel, &card.SerialNo, &editionNo, &editionSupply, &card.CreatedAt); err != nil {
			return nil, err
		}
		if checkinID.Valid {
			card.CheckinID = checkinID.Int64
		}
		if editionNo.Valid {
			card.EditionNo = editionNo.Int64
		}
		if editionSupply.Valid {
			card.EditionSupply = editionSupply.Int64
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, nil
}

func (r *checkinRepository) ListCollectibleCardsPage(ctx context.Context, userID int64, page, pageSize int) (*service.CheckinCollectibleCardsPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}
	result := &service.CheckinCollectibleCardsPage{
		Items:    []service.CheckinCollectibleCard{},
		Page:     page,
		PageSize: pageSize,
	}
	if r == nil || r.db == nil {
		return result, nil
	}
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM checkin_collectible_cards WHERE user_id = $1`,
		userID,
	).Scan(&result.Total); err != nil {
		return nil, err
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, checkin_id, card_key, rarity, source_type, source_label, serial_no, edition_no, edition_supply, created_at
		FROM checkin_collectible_cards
		WHERE user_id = $1
		ORDER BY id DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result.Items = make([]service.CheckinCollectibleCard, 0, pageSize)
	for rows.Next() {
		var card service.CheckinCollectibleCard
		var checkinID sql.NullInt64
		var editionNo, editionSupply sql.NullInt64
		if err := rows.Scan(&card.ID, &checkinID, &card.CardKey, &card.Rarity, &card.SourceType, &card.SourceLabel, &card.SerialNo, &editionNo, &editionSupply, &card.CreatedAt); err != nil {
			return nil, err
		}
		if checkinID.Valid {
			card.CheckinID = checkinID.Int64
		}
		if editionNo.Valid {
			card.EditionNo = editionNo.Int64
		}
		if editionSupply.Valid {
			card.EditionSupply = editionSupply.Int64
		}
		result.Items = append(result.Items, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.HasMore = int64(offset+len(result.Items)) < result.Total
	return result, nil
}

func formatCheckinDate(day time.Time) string {
	return day.Format("2006-01-02")
}

func getJackpotPoolAmount(ctx context.Context, tx *sql.Tx, pool string) (float64, error) {
	return getJackpotLedgerPoolAmount(ctx, tx, pool)
}

type jackpotLegacySummary struct {
	Pool                 string
	WinnerAmount         float64
	CelebrationAmount    float64
	CelebrationUserCount int
	TotalAmount          float64
}

func buildJackpotPayouts(ctx context.Context, tx *sql.Tx, jackpot service.JackpotPlan) ([]service.JackpotPayout, error) {
	if !jackpot.Hit {
		return nil, nil
	}
	for _, pool := range []string{"credit", "balance"} {
		if err := lockJackpotPool(ctx, tx, pool); err != nil {
			return nil, err
		}
	}
	payouts := make([]service.JackpotPayout, 0, 2)
	for _, pool := range []string{"credit", "balance"} {
		currentPool, err := getJackpotPoolAmount(ctx, tx, pool)
		if err != nil {
			return nil, err
		}
		payouts = append(payouts, service.JackpotPayout{
			PoolType:          pool,
			RewardAsset:       pool,
			WinnerAmount:      roundJackpotAmount(currentPool * jackpot.WinnerRatio),
			CelebrationAmount: roundJackpotAmount(currentPool * jackpot.CelebrationRatio),
		})
	}
	return payouts, nil
}

func summarizeJackpotPayouts(payouts []service.JackpotPayout, preferredPool string) jackpotLegacySummary {
	if len(payouts) == 0 {
		return jackpotLegacySummary{}
	}
	selected := payouts[0]
	for _, payout := range payouts {
		if payout.PoolType == preferredPool {
			selected = payout
			break
		}
	}
	return jackpotLegacySummary{
		Pool:                 selected.PoolType,
		WinnerAmount:         selected.WinnerAmount,
		CelebrationAmount:    selected.CelebrationActualAmount,
		CelebrationUserCount: selected.CelebrationUserCount,
		TotalAmount:          selected.WinnerAmount + selected.CelebrationActualAmount,
	}
}

func legacyJackpotPoolForCheckin(checkinType string) string {
	if checkinType == service.CheckinTypeBalance {
		return "balance"
	}
	return "credit"
}

func marshalJackpotPayouts(payouts []service.JackpotPayout) (string, error) {
	if payouts == nil {
		payouts = []service.JackpotPayout{}
	}
	payload, err := json.Marshal(payouts)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func scanJackpotPayouts(raw sql.NullString) ([]service.JackpotPayout, error) {
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var payouts []service.JackpotPayout
	if err := json.Unmarshal([]byte(raw.String), &payouts); err != nil {
		return nil, err
	}
	return payouts, nil
}

func insertUserBalanceLedger(ctx context.Context, tx *sql.Tx, userID int64, sourceType, sourceID string, amount, balanceAfter float64, note string) error {
	if amount == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO user_balance_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, $2, $3, $4, $5, 'posted', $6, NOW()) ON CONFLICT DO NOTHING`, userID, sourceType, sourceID, amount, balanceAfter, note)
	return err
}

func recordDailyCheckinJackpotLedger(ctx context.Context, tx *sql.Tx, input service.CheckinClaimInput, checkinID int64, payouts []service.JackpotPayout) error {
	pool, contribution := dailyCheckinJackpotContribution(input)
	sourceID := fmt.Sprintf("daily_checkin:%d", checkinID)
	if contribution > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata) VALUES ($1, 'daily_checkin', $2, 'contribution', $3, $4, '{}'::jsonb) ON CONFLICT DO NOTHING`, pool, sourceID, contribution, input.UserID); err != nil {
			return err
		}
	}
	if !input.Jackpot.Hit {
		return nil
	}
	for _, payout := range payouts {
		if payout.WinnerAmount > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata) VALUES ($1, 'daily_checkin', $2, 'payout', $3, $4, '{}'::jsonb) ON CONFLICT DO NOTHING`, payout.PoolType, sourceID, -payout.WinnerAmount, input.UserID); err != nil {
				return err
			}
		}
		if payout.CelebrationActualAmount > 0 {
			metadata := fmt.Sprintf(`{"celebration_user_count":%d,"celebration_amount":%.2f}`, payout.CelebrationUserCount, payout.CelebrationAmount)
			if _, err := tx.ExecContext(ctx, `INSERT INTO jackpot_ledger (pool_type, source_type, source_id, entry_type, amount, user_id, metadata) VALUES ($1, 'daily_checkin', $2, 'celebration', $3, $4, $5::jsonb) ON CONFLICT DO NOTHING`, payout.PoolType, sourceID, -payout.CelebrationActualAmount, input.UserID, metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

func dailyCheckinJackpotContribution(input service.CheckinClaimInput) (string, float64) {
	switch input.Type {
	case service.CheckinTypeBalance:
		return "balance", input.Cost * 0.10
	case service.CheckinTypePaid, service.CheckinTypeCredit:
		return "credit", input.Cost * 0.20
	default:
		return "", 0
	}
}

func roundJackpotAmount(value float64) float64 {
	if value < 0 {
		return 0
	}
	return float64(int(value*100+0.5)) / 100
}

func grantDailyCheckinJackpotCelebrations(ctx context.Context, tx *sql.Tx, checkinID, winnerUserID int64, payouts []service.JackpotPayout) ([]service.JackpotPayout, error) {
	out := append([]service.JackpotPayout(nil), payouts...)
	for i := range out {
		if out[i].CelebrationAmount <= 0 {
			continue
		}
		count, actualAmount, err := grantJackpotCelebration(ctx, tx, checkinID, winnerUserID, out[i].PoolType, out[i].CelebrationAmount)
		if err != nil {
			return nil, err
		}
		out[i].CelebrationUserCount = count
		out[i].CelebrationActualAmount = actualAmount
	}
	return out, nil
}

func grantJackpotCelebration(ctx context.Context, tx *sql.Tx, checkinID, winnerUserID int64, pool string, totalAmount float64) (int, float64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, balance, credit_balance
		FROM users
		WHERE deleted_at IS NULL
		  AND status = 'active'
		  AND id <> $1
		  AND last_active_at >= NOW() - INTERVAL '24 hours'
		ORDER BY last_active_at DESC, id ASC
		LIMIT 500
		FOR UPDATE
	`, winnerUserID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()

	type activeUser struct {
		id            int64
		balance       float64
		creditBalance float64
	}
	users := make([]activeUser, 0, 128)
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO checkin_jackpot_shares (checkin_id, user_id, pool, amount, balance_after, credit_balance_after) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`, checkinID, u.id, pool, share, balanceAfter, creditAfter); err != nil {
			return 0, 0, err
		}
		if pool != "balance" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'checkin_jackpot_share', $2, $3, $4, 'posted', $5, NOW())`, u.id, fmt.Sprintf("checkin:%d:jackpot:credit:share:%d", checkinID, u.id), share, creditAfter, "Daily fortune jackpot celebration share"); err != nil {
				return 0, 0, err
			}
			continue
		}
		if err := insertUserBalanceLedger(ctx, tx, u.id, "checkin_jackpot_share", fmt.Sprintf("checkin:%d:jackpot:balance:share:%d", checkinID, u.id), share, balanceAfter, "Daily fortune jackpot celebration share"); err != nil {
			return 0, 0, err
		}
	}
	return len(users), floorJackpotShareAmount(share * float64(len(users))), nil
}

func floorJackpotShareAmount(value float64) float64 {
	if value < 0 {
		return 0
	}
	return math.Floor(value*100) / 100
}
