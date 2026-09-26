//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type creditLotteryFakeRepo struct {
	createInput          CreditLotteryCreateInput
	continueInput        CreditLotteryContinueInput
	settleInput          CreditLotterySettleInput
	operationInputUserID int64
	operationInputID     string
	operationSession     *CreditLotterySession
	active               *CreditLotterySession
	expired              []CreditLotterySession
}

func (r *creditLotteryFakeRepo) CreateSession(ctx context.Context, input CreditLotteryCreateInput) (*CreditLotterySession, error) {
	r.createInput = input
	status := CreditLotteryStatusActive
	if input.Mode == CreditLotteryModeSingle {
		status = CreditLotteryStatusSettled
	}
	return &CreditLotterySession{
		ID:           10,
		UserID:       input.UserID,
		Mode:         input.Mode,
		Status:       status,
		SessionDate:  input.SessionDate,
		CostCredit:   input.CostCredit,
		MaxRounds:    input.MaxRounds,
		CurrentRound: 1,
		CurrentResult: &CreditLotteryRoundResult{
			RoundNo:      1,
			RewardAsset:  "credit",
			RewardAmount: input.FirstRound.RewardAmount,
		},
		ExpiresAt: input.Now.Add(10 * time.Minute),
	}, nil
}

func (r *creditLotteryFakeRepo) GetActiveSession(ctx context.Context, userID int64, now time.Time) (*CreditLotterySession, error) {
	return r.active, nil
}

func (r *creditLotteryFakeRepo) GetSession(ctx context.Context, userID, sessionID int64) (*CreditLotterySession, error) {
	return &CreditLotterySession{ID: sessionID, UserID: userID, Status: CreditLotteryStatusActive}, nil
}

func (r *creditLotteryFakeRepo) GetSessionByOperationID(ctx context.Context, userID int64, operationID string) (*CreditLotterySession, error) {
	r.operationInputUserID = userID
	r.operationInputID = operationID
	return r.operationSession, nil
}

func (r *creditLotteryFakeRepo) ContinueSession(ctx context.Context, input CreditLotteryContinueInput) (*CreditLotterySession, error) {
	r.continueInput = input
	return &CreditLotterySession{
		ID:           input.SessionID,
		UserID:       input.UserID,
		Status:       CreditLotteryStatusActive,
		CurrentRound: input.ExpectedRound + 1,
		CurrentResult: &CreditLotteryRoundResult{
			RoundNo:      input.ExpectedRound + 1,
			RewardAsset:  "credit",
			RewardAmount: input.NextRound.RewardAmount,
		},
	}, nil
}

func (r *creditLotteryFakeRepo) SettleSession(ctx context.Context, input CreditLotterySettleInput) (*CreditLotterySession, error) {
	r.settleInput = input
	return &CreditLotterySession{
		ID:               input.SessionID,
		UserID:           input.UserID,
		Status:           CreditLotteryStatusSettled,
		SettlementReason: input.Reason,
		FinalResult: &CreditLotteryRoundResult{
			RoundNo:      2,
			RewardAsset:  "credit",
			RewardAmount: 20,
		},
	}, nil
}

func (r *creditLotteryFakeRepo) ListExpiredSessions(ctx context.Context, now time.Time, limit int) ([]CreditLotterySession, error) {
	return r.expired, nil
}

func TestCreditLotteryService_CreateSessionUsesSingleDrawModeByDefault(t *testing.T) {
	repo := &creditLotteryFakeRepo{}
	svc := NewCreditLotteryService(repo)
	fixedNow := time.Date(2026, 7, 11, 9, 30, 0, 0, time.Local)
	svc.now = func() time.Time { return fixedNow }
	svc.rewardPicker = func() (float64, error) { return 50, nil }
	svc.collectiblePicker = func() (*CheckinCollectibleCard, error) { return nil, nil }
	svc.modePicker = func() (string, error) { return CreditLotteryModeSingle, nil }
	svc.jackpotPicker = func() (JackpotPlan, error) { return JackpotPlan{}, nil }

	session, err := svc.CreateSession(context.Background(), 7, "start-idem")

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.createInput.UserID)
	require.Equal(t, "2026-07-11", repo.createInput.SessionDate)
	require.Equal(t, "start-idem", repo.createInput.IdempotencyKey)
	require.Equal(t, CreditLotteryModeSingle, repo.createInput.Mode)
	require.InDelta(t, 20.0, repo.createInput.CostCredit, 1e-9)
	require.Equal(t, 1, repo.createInput.MaxRounds)
	require.InDelta(t, 50.0, repo.createInput.FirstRound.RewardAmount, 1e-9)
	require.Equal(t, 1, session.CurrentRound)
	require.Equal(t, CreditLotteryStatusSettled, session.Status)
}

func TestCreditLotteryService_CreateSessionCanTriggerThreeRoundMode(t *testing.T) {
	repo := &creditLotteryFakeRepo{}
	svc := NewCreditLotteryService(repo)
	svc.rewardPicker = func() (float64, error) { return 50, nil }
	svc.collectiblePicker = func() (*CheckinCollectibleCard, error) { return nil, nil }
	svc.modePicker = func() (string, error) { return CreditLotteryModeThreeRound, nil }

	session, err := svc.CreateSession(context.Background(), 7, "start-three")

	require.NoError(t, err)
	require.Equal(t, CreditLotteryModeThreeRound, repo.createInput.Mode)
	require.Equal(t, 3, repo.createInput.MaxRounds)
	require.Equal(t, CreditLotteryStatusActive, session.Status)
}

func TestCreditLotteryService_CreateSessionRejectsLongIdempotencyKey(t *testing.T) {
	svc := NewCreditLotteryService(&creditLotteryFakeRepo{})

	_, err := svc.CreateSession(context.Background(), 7, strings.Repeat("x", 256))

	require.ErrorIs(t, err, ErrCreditLotteryInvalidIdempotent)
}

func TestCreditLotteryService_GetOperationSessionScopesByUserAndOperationID(t *testing.T) {
	repo := &creditLotteryFakeRepo{operationSession: &CreditLotterySession{ID: 10, UserID: 7, Status: CreditLotteryStatusActive}}
	svc := NewCreditLotteryService(repo)

	session, err := svc.GetOperationSession(context.Background(), 7, "continue-operation-1")

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.operationInputUserID)
	require.Equal(t, "continue-operation-1", repo.operationInputID)
	require.Equal(t, int64(10), session.ID)
}

func TestCreditLotteryService_GetOperationSessionReturnsNotFound(t *testing.T) {
	svc := NewCreditLotteryService(&creditLotteryFakeRepo{})

	_, err := svc.GetOperationSession(context.Background(), 7, "missing-operation")

	require.ErrorIs(t, err, ErrCreditLotteryOperationNotFound)
}

func TestCreditLotteryService_ContinueSecondRoundPlansFinalSettlement(t *testing.T) {
	repo := &creditLotteryFakeRepo{}
	svc := NewCreditLotteryService(repo)
	svc.rewardPicker = func() (float64, error) { return 100, nil }
	svc.collectiblePicker = func() (*CheckinCollectibleCard, error) {
		return &CheckinCollectibleCard{CardKey: "tiny_luck_clerk", Rarity: "rare", SourceType: "daily_fortune", SourceLabel: CheckinTypeCredit}, nil
	}
	svc.jackpotPicker = func() (JackpotPlan, error) {
		return JackpotPlan{Hit: true, WinnerRatio: 0.60, CelebrationRatio: 0.10}, nil
	}

	_, err := svc.ContinueSession(context.Background(), 7, 10, 2, "idem-continue")

	require.NoError(t, err)
	require.Equal(t, 2, repo.continueInput.ExpectedRound)
	require.Equal(t, "idem-continue", repo.continueInput.IdempotencyKey)
	require.True(t, repo.continueInput.Finalize)
	require.True(t, repo.continueInput.Jackpot.Hit)
	require.NotNil(t, repo.continueInput.NextRound.CollectibleCandidate)
}

func TestCreditLotteryService_SettleRunsSingleJackpotAndCollectibleDecision(t *testing.T) {
	repo := &creditLotteryFakeRepo{}
	svc := NewCreditLotteryService(repo)
	svc.jackpotPicker = func() (JackpotPlan, error) {
		return JackpotPlan{Hit: true, WinnerRatio: 0.60, CelebrationRatio: 0.10}, nil
	}

	_, err := svc.SettleSession(context.Background(), 7, 10, 2, CreditLotterySettlementUserStop, "idem-settle")

	require.NoError(t, err)
	require.Equal(t, 2, repo.settleInput.ExpectedRound)
	require.Equal(t, CreditLotterySettlementUserStop, repo.settleInput.Reason)
	require.Equal(t, "idem-settle", repo.settleInput.IdempotencyKey)
	require.True(t, repo.settleInput.Jackpot.Hit)
	require.Nil(t, repo.settleInput.Collectible)
}

func TestCreditLotteryService_SettleExpiredSessionsUsesOneJackpotPlanPerSession(t *testing.T) {
	repo := &creditLotteryFakeRepo{
		expired: []CreditLotterySession{{ID: 10, UserID: 7}},
	}
	svc := NewCreditLotteryService(repo)
	svc.jackpotPicker = func() (JackpotPlan, error) {
		return JackpotPlan{Hit: true, WinnerRatio: 0.60, CelebrationRatio: 0.10}, nil
	}

	settled, err := svc.SettleExpiredSessions(context.Background(), 100)

	require.NoError(t, err)
	require.Len(t, settled, 1)
	require.Equal(t, CreditLotterySettlementTimeout, repo.settleInput.Reason)
	require.True(t, repo.settleInput.Jackpot.Hit)
	require.InDelta(t, 0.60, repo.settleInput.Jackpot.WinnerRatio, 1e-9)
}

func TestCreditLotteryExpiryService_RunOnceSettlesExpiredSessions(t *testing.T) {
	repo := &creditLotteryFakeRepo{
		expired: []CreditLotterySession{{ID: 10, UserID: 7}},
	}
	svc := NewCreditLotteryService(repo)
	svc.jackpotPicker = func() (JackpotPlan, error) {
		return JackpotPlan{}, nil
	}
	expiry := NewCreditLotteryExpiryService(svc, time.Minute)

	expiry.runOnce()

	require.Equal(t, CreditLotterySettlementTimeout, repo.settleInput.Reason)
}
