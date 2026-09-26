package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	CreditLotteryStatusActive  = "active"
	CreditLotteryStatusSettled = "settled"
	CreditLotteryStatusFailed  = "failed"

	CreditLotterySettlementUserStop   = "user_stop"
	CreditLotterySettlementThirdRound = "third_round"
	CreditLotterySettlementTimeout    = "timeout"
	CreditLotterySettlementSingleDraw = "single_draw"

	CreditLotteryModeSingle     = "single"
	CreditLotteryModeThreeRound = "three_round"

	creditLotteryCost                      = 20.0
	creditLotteryMaxRounds                 = 3
	creditLotteryTTL                       = 10 * time.Minute
	creditLotteryStartIdempotencyMaxLength = 255
	creditLotteryThreeRoundTriggerPercent  = int64(10)
)

var (
	ErrCreditLotteryUnavailable       = infraerrors.ServiceUnavailable("CREDIT_LOTTERY_UNAVAILABLE", "credit lottery service is not configured")
	ErrCreditLotteryInvalidSession    = infraerrors.BadRequest("CREDIT_LOTTERY_INVALID_SESSION", "invalid credit lottery session")
	ErrCreditLotteryInvalidRound      = infraerrors.BadRequest("CREDIT_LOTTERY_INVALID_ROUND", "invalid credit lottery round")
	ErrCreditLotteryInvalidIdempotent = infraerrors.BadRequest("CREDIT_LOTTERY_INVALID_IDEMPOTENCY_KEY", "invalid idempotency key")
	ErrCreditLotteryDailyLimit        = infraerrors.Conflict("CREDIT_LOTTERY_DAILY_LIMIT", "daily credit lottery limit reached")
	ErrCreditLotteryAlreadySettled    = infraerrors.Conflict("CREDIT_LOTTERY_ALREADY_SETTLED", "credit lottery session is already settled")
	ErrCreditLotteryOperationNotFound = infraerrors.NotFound("CREDIT_LOTTERY_OPERATION_NOT_FOUND", "credit lottery operation was not found")
)

type CreditLotteryRepository interface {
	CreateSession(ctx context.Context, input CreditLotteryCreateInput) (*CreditLotterySession, error)
	GetActiveSession(ctx context.Context, userID int64, now time.Time) (*CreditLotterySession, error)
	GetSession(ctx context.Context, userID, sessionID int64) (*CreditLotterySession, error)
	GetSessionByOperationID(ctx context.Context, userID int64, operationID string) (*CreditLotterySession, error)
	ContinueSession(ctx context.Context, input CreditLotteryContinueInput) (*CreditLotterySession, error)
	SettleSession(ctx context.Context, input CreditLotterySettleInput) (*CreditLotterySession, error)
	ListExpiredSessions(ctx context.Context, now time.Time, limit int) ([]CreditLotterySession, error)
}

type CreditLotteryService struct {
	repo              CreditLotteryRepository
	now               func() time.Time
	rewardPicker      func() (float64, error)
	collectiblePicker func() (*CheckinCollectibleCard, error)
	jackpotPicker     func() (JackpotPlan, error)
	modePicker        func() (string, error)
}

type CreditLotteryCreateInput struct {
	UserID         int64
	SessionDate    string
	IdempotencyKey string
	Mode           string
	CostCredit     float64
	MaxRounds      int
	FirstRound     CreditLotteryRoundPlan
	Jackpot        JackpotPlan
	Now            time.Time
}

type CreditLotteryContinueInput struct {
	UserID         int64
	SessionID      int64
	ExpectedRound  int
	IdempotencyKey string
	NextRound      CreditLotteryRoundPlan
	Finalize       bool
	Jackpot        JackpotPlan
	Now            time.Time
}

type CreditLotterySettleInput struct {
	UserID         int64
	SessionID      int64
	ExpectedRound  int
	Reason         string
	IdempotencyKey string
	Jackpot        JackpotPlan
	Collectible    *CheckinCollectibleCard
	Now            time.Time
}

type CreditLotteryRoundPlan struct {
	RoundNo              int                     `json:"round_no"`
	RewardAsset          string                  `json:"reward_asset"`
	RewardAmount         float64                 `json:"reward_amount"`
	CollectibleCandidate *CheckinCollectibleCard `json:"collectible_candidate,omitempty"`
}

type CreditLotterySession struct {
	ID                          int64                     `json:"id"`
	UserID                      int64                     `json:"user_id"`
	Mode                        string                    `json:"mode"`
	SessionDate                 string                    `json:"session_date"`
	Status                      string                    `json:"status"`
	CostCredit                  float64                   `json:"cost_credit"`
	MaxRounds                   int                       `json:"max_rounds"`
	CurrentRound                int                       `json:"current_round"`
	SettlementReason            string                    `json:"settlement_reason,omitempty"`
	BalanceAfter                float64                   `json:"balance_after,omitempty"`
	CreditBalanceAfter          float64                   `json:"credit_balance_after,omitempty"`
	JackpotHit                  bool                      `json:"jackpot_hit"`
	JackpotPool                 string                    `json:"jackpot_pool,omitempty"`
	JackpotWinnerAmount         float64                   `json:"jackpot_winner_amount,omitempty"`
	JackpotCelebrationAmount    float64                   `json:"jackpot_celebration_amount,omitempty"`
	JackpotCelebrationUserCount int                       `json:"jackpot_celebration_user_count,omitempty"`
	JackpotPayouts              []JackpotPayout           `json:"jackpot_payouts,omitempty"`
	CurrentResult               *CreditLotteryRoundResult `json:"current_result,omitempty"`
	FinalResult                 *CreditLotteryRoundResult `json:"final_result,omitempty"`
	ExpiresAt                   time.Time                 `json:"expires_at"`
	SettledAt                   *time.Time                `json:"settled_at,omitempty"`
	CreatedAt                   time.Time                 `json:"created_at"`
	UpdatedAt                   time.Time                 `json:"updated_at"`
}

type CreditLotteryRoundResult struct {
	ID                   int64                   `json:"id,omitempty"`
	RoundNo              int                     `json:"round_no"`
	RewardAsset          string                  `json:"reward_asset"`
	RewardAmount         float64                 `json:"reward_amount"`
	CollectibleCandidate *CheckinCollectibleCard `json:"collectible_candidate,omitempty"`
	CollectibleCard      *CheckinCollectibleCard `json:"collectible_card,omitempty"`
	IsCurrent            bool                    `json:"is_current"`
	SupersededAt         *time.Time              `json:"superseded_at,omitempty"`
	CreatedAt            time.Time               `json:"created_at"`
}

func NewCreditLotteryService(repo CreditLotteryRepository) *CreditLotteryService {
	return &CreditLotteryService{
		repo:              repo,
		now:               time.Now,
		rewardPicker:      func() (float64, error) { return randomCheckinReward(creditCheckinRewards) },
		collectiblePicker: func() (*CheckinCollectibleCard, error) { return randomCollectibleCard(CheckinTypeCredit) },
		jackpotPicker:     func() (JackpotPlan, error) { return randomJackpotPlan(CheckinTypeCredit) },
		modePicker:        randomCreditLotteryMode,
	}
}

func (s *CreditLotteryService) CreateSession(ctx context.Context, userID int64, idempotencyKey string) (*CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !isValidCreditLotteryIdempotencyKey(idempotencyKey) {
		return nil, ErrCreditLotteryInvalidIdempotent
	}
	now := s.currentTime()
	active, err := s.repo.GetActiveSession(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if active != nil {
		if isCreditLotteryExpired(active, now) {
			return s.repo.SettleSession(ctx, CreditLotterySettleInput{
				UserID:    userID,
				SessionID: active.ID,
				Reason:    CreditLotterySettlementTimeout,
				Jackpot:   s.mustJackpotPlan(),
				Now:       now,
			})
		}
		return active, nil
	}
	round, err := s.planRound(1)
	if err != nil {
		return nil, err
	}
	mode, err := s.pickMode()
	if err != nil {
		return nil, err
	}
	maxRounds := creditLotteryMaxRounds
	var jackpot JackpotPlan
	if mode == CreditLotteryModeSingle {
		maxRounds = 1
		jackpot, err = s.pickJackpot()
		if err != nil {
			return nil, err
		}
	}
	return s.repo.CreateSession(ctx, CreditLotteryCreateInput{
		UserID:         userID,
		SessionDate:    formatCreditLotteryDate(now),
		IdempotencyKey: idempotencyKey,
		Mode:           mode,
		CostCredit:     creditLotteryCost,
		MaxRounds:      maxRounds,
		FirstRound:     round,
		Jackpot:        jackpot,
		Now:            now,
	})
}

func (s *CreditLotteryService) GetActiveSession(ctx context.Context, userID int64) (*CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	now := s.currentTime()
	active, err := s.repo.GetActiveSession(ctx, userID, now)
	if err != nil || active == nil {
		return active, err
	}
	if isCreditLotteryExpired(active, now) {
		return s.repo.SettleSession(ctx, CreditLotterySettleInput{
			UserID:    userID,
			SessionID: active.ID,
			Reason:    CreditLotterySettlementTimeout,
			Jackpot:   s.mustJackpotPlan(),
			Now:       now,
		})
	}
	return active, nil
}

func (s *CreditLotteryService) GetSession(ctx context.Context, userID, sessionID int64) (*CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	if sessionID <= 0 {
		return nil, ErrCreditLotteryInvalidSession
	}
	return s.repo.GetSession(ctx, userID, sessionID)
}

func (s *CreditLotteryService) GetOperationSession(ctx context.Context, userID int64, operationID string) (*CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	operationID = strings.TrimSpace(operationID)
	if !isValidCreditLotteryIdempotencyKey(operationID) {
		return nil, ErrCreditLotteryInvalidIdempotent
	}
	session, err := s.repo.GetSessionByOperationID(ctx, userID, operationID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrCreditLotteryOperationNotFound
	}
	return session, nil
}

func (s *CreditLotteryService) ContinueSession(ctx context.Context, userID, sessionID int64, expectedRound int, idempotencyKey string) (*CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if sessionID <= 0 {
		return nil, ErrCreditLotteryInvalidSession
	}
	if expectedRound < 1 || expectedRound >= creditLotteryMaxRounds {
		return nil, ErrCreditLotteryInvalidRound
	}
	if idempotencyKey == "" {
		return nil, ErrCreditLotteryInvalidIdempotent
	}
	if !isValidCreditLotteryIdempotencyKey(idempotencyKey) {
		return nil, ErrCreditLotteryInvalidIdempotent
	}
	nextRound, err := s.planRound(expectedRound + 1)
	if err != nil {
		return nil, err
	}
	input := CreditLotteryContinueInput{
		UserID:         userID,
		SessionID:      sessionID,
		ExpectedRound:  expectedRound,
		IdempotencyKey: idempotencyKey,
		NextRound:      nextRound,
		Finalize:       expectedRound+1 == creditLotteryMaxRounds,
		Now:            s.currentTime(),
	}
	if input.Finalize {
		jackpot, err := s.pickJackpot()
		if err != nil {
			return nil, err
		}
		input.Jackpot = jackpot
	}
	return s.repo.ContinueSession(ctx, input)
}

func (s *CreditLotteryService) SettleSession(ctx context.Context, userID, sessionID int64, expectedRound int, reason, idempotencyKey string) (*CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if sessionID <= 0 {
		return nil, ErrCreditLotteryInvalidSession
	}
	if idempotencyKey == "" {
		return nil, ErrCreditLotteryInvalidIdempotent
	}
	if !isValidCreditLotteryIdempotencyKey(idempotencyKey) {
		return nil, ErrCreditLotteryInvalidIdempotent
	}
	if expectedRound < 1 || expectedRound > creditLotteryMaxRounds {
		return nil, ErrCreditLotteryInvalidRound
	}
	if reason == "" {
		reason = CreditLotterySettlementUserStop
	}
	jackpot, err := s.pickJackpot()
	if err != nil {
		return nil, err
	}
	return s.repo.SettleSession(ctx, CreditLotterySettleInput{
		UserID:         userID,
		SessionID:      sessionID,
		ExpectedRound:  expectedRound,
		Reason:         reason,
		IdempotencyKey: idempotencyKey,
		Jackpot:        jackpot,
		Now:            s.currentTime(),
	})
}

func (s *CreditLotteryService) SettleExpiredSessions(ctx context.Context, limit int) ([]CreditLotterySession, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCreditLotteryUnavailable
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	expired, err := s.repo.ListExpiredSessions(ctx, s.currentTime(), limit)
	if err != nil {
		return nil, err
	}
	settled := make([]CreditLotterySession, 0, len(expired))
	for _, session := range expired {
		jackpot, err := s.pickJackpot()
		if err != nil {
			return settled, err
		}
		result, err := s.repo.SettleSession(ctx, CreditLotterySettleInput{
			UserID:    session.UserID,
			SessionID: session.ID,
			Reason:    CreditLotterySettlementTimeout,
			Jackpot:   jackpot,
			Now:       s.currentTime(),
		})
		if err != nil {
			return settled, err
		}
		settled = append(settled, *result)
	}
	return settled, nil
}

func (s *CreditLotteryService) planRound(roundNo int) (CreditLotteryRoundPlan, error) {
	reward, err := s.pickReward()
	if err != nil {
		return CreditLotteryRoundPlan{}, err
	}
	collectible, err := s.pickCollectible()
	if err != nil {
		return CreditLotteryRoundPlan{}, err
	}
	return CreditLotteryRoundPlan{
		RoundNo:              roundNo,
		RewardAsset:          "credit",
		RewardAmount:         reward,
		CollectibleCandidate: collectible,
	}, nil
}

func (s *CreditLotteryService) pickReward() (float64, error) {
	if s != nil && s.rewardPicker != nil {
		return s.rewardPicker()
	}
	return randomCheckinReward(creditCheckinRewards)
}

func (s *CreditLotteryService) pickCollectible() (*CheckinCollectibleCard, error) {
	if s != nil && s.collectiblePicker != nil {
		return s.collectiblePicker()
	}
	return randomCollectibleCard(CheckinTypeCredit)
}

func (s *CreditLotteryService) pickJackpot() (JackpotPlan, error) {
	if s != nil && s.jackpotPicker != nil {
		return s.jackpotPicker()
	}
	return randomJackpotPlan(CheckinTypeCredit)
}

func (s *CreditLotteryService) pickMode() (string, error) {
	if s != nil && s.modePicker != nil {
		return s.modePicker()
	}
	return randomCreditLotteryMode()
}

func (s *CreditLotteryService) mustJackpotPlan() JackpotPlan {
	plan, err := s.pickJackpot()
	if err != nil && !errors.Is(err, ErrCheckinInvalidType) {
		return JackpotPlan{}
	}
	return plan
}

func (s *CreditLotteryService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func formatCreditLotteryDate(day time.Time) string {
	return day.In(time.Local).Format("2006-01-02")
}

func isCreditLotteryExpired(session *CreditLotterySession, now time.Time) bool {
	return session != nil && session.Status == CreditLotteryStatusActive && !session.ExpiresAt.IsZero() && !now.Before(session.ExpiresAt)
}

func isValidCreditLotteryIdempotencyKey(key string) bool {
	return key != "" && len(key) <= creditLotteryStartIdempotencyMaxLength
}

func randomCreditLotteryMode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return "", err
	}
	if n.Int64() < creditLotteryThreeRoundTriggerPercent {
		return CreditLotteryModeThreeRound, nil
	}
	return CreditLotteryModeSingle, nil
}
