//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type checkinRepoStub struct {
	statusInputUserID    int64
	claimInput           CheckinClaimInput
	claimResult          *CheckinResult
	operationInputUserID int64
	operationInputID     string
	operationResult      *CheckinResult
	purchaseInput        CheckinCardPurchaseInput
}

func (s *checkinRepoStub) GetDailyStatus(_ context.Context, userID int64, _ time.Time) (*CheckinStatus, error) {
	s.statusInputUserID = userID
	return &CheckinStatus{Date: "2026-06-21"}, nil
}

func (s *checkinRepoStub) ClaimDaily(_ context.Context, input CheckinClaimInput) (*CheckinResult, error) {
	s.claimInput = input
	if s.claimResult != nil {
		return s.claimResult, nil
	}
	return &CheckinResult{Type: input.Type, Cost: input.Cost, Reward: input.Reward}, nil
}

func (s *checkinRepoStub) GetOperationResult(_ context.Context, userID int64, operationID string) (*CheckinResult, error) {
	s.operationInputUserID = userID
	s.operationInputID = operationID
	if s.operationResult != nil {
		return s.operationResult, nil
	}
	return &CheckinResult{OperationID: operationID}, nil
}

func (s *checkinRepoStub) ClaimMilestone(_ context.Context, _ int64, milestoneDays int) (*CheckinMilestoneClaimResult, error) {
	return &CheckinMilestoneClaimResult{MilestoneDays: milestoneDays}, nil
}

func (s *checkinRepoStub) BuyCollectibleCard(_ context.Context, input CheckinCardPurchaseInput) (*CheckinCardPurchaseResult, error) {
	s.purchaseInput = input
	return &CheckinCardPurchaseResult{Rarity: input.Rarity, Price: input.Price, CreditBalanceAfter: 1000 - input.Price, CollectibleCard: CheckinCollectibleCard{CardKey: input.CardKey, Rarity: input.Rarity, SourceType: collectibleShopSourceType, SourceLabel: collectibleShopSourceLabel, EditionNo: 1, EditionSupply: input.EditionSupply}}, nil
}

func (s *checkinRepoStub) ListDailyRecords(_ context.Context, _ int64, _ int) ([]CheckinRecord, error) {
	return nil, nil
}

func (s *checkinRepoStub) ListCollectibleCards(_ context.Context, _ int64, _ int) ([]CheckinCollectibleCard, error) {
	return nil, nil
}

func TestCheckinService_ClaimFreeRule(t *testing.T) {
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo)
	svc.now = func() time.Time { return time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC) }

	result, err := svc.Claim(context.Background(), 7, CheckinTypeFree)

	require.NoError(t, err)
	require.Equal(t, CheckinTypeFree, result.Type)
	require.Equal(t, 0.0, repo.claimInput.Cost)
	require.GreaterOrEqual(t, repo.claimInput.Reward, 5.0)
	require.LessOrEqual(t, repo.claimInput.Reward, 15.0)
}

func TestCheckinService_ClaimWithOperationPassesStableOperationID(t *testing.T) {
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo)

	result, err := svc.ClaimWithOperation(context.Background(), 7, CheckinTypeBalance, "checkin-balance-1")

	require.NoError(t, err)
	require.Equal(t, "checkin-balance-1", repo.claimInput.OperationID)
	require.Equal(t, CheckinTypeBalance, result.Type)
}

func TestCheckinService_GetOperationResultScopesByUserAndOperationID(t *testing.T) {
	repo := &checkinRepoStub{operationResult: &CheckinResult{OperationID: "checkin-free-1", Reward: 10}}
	svc := NewCheckinService(repo)

	result, err := svc.GetOperationResult(context.Background(), 7, "checkin-free-1")

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.operationInputUserID)
	require.Equal(t, "checkin-free-1", repo.operationInputID)
	require.Equal(t, 10.0, result.Reward)
}

func TestCheckinService_GetOperationResultRejectsEmptyOperationID(t *testing.T) {
	svc := NewCheckinService(&checkinRepoStub{})

	_, err := svc.GetOperationResult(context.Background(), 7, "")

	require.ErrorIs(t, err, ErrCheckinOperationIDInvalid)
}

func TestCheckinService_ClaimPaidRuleUsesCreditLotteryEndpoint(t *testing.T) {
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo)
	svc.now = func() time.Time { return time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC) }

	_, err := svc.Claim(context.Background(), 7, CheckinTypePaid)

	require.ErrorIs(t, err, ErrCheckinCreditLotteryMoved)
	require.Empty(t, repo.claimInput.Type)
}

func TestCheckinService_ClaimInvalidType(t *testing.T) {
	svc := NewCheckinService(&checkinRepoStub{})

	_, err := svc.Claim(context.Background(), 7, "other")

	require.ErrorIs(t, err, ErrCheckinInvalidType)
}

func TestWeightedCheckinRewardTables(t *testing.T) {
	require.Equal(t, []weightedCheckinReward{{Value: 5, Weight: 40}, {Value: 7, Weight: 30}, {Value: 10, Weight: 15}, {Value: 15, Weight: 15}}, freeCheckinRewards)
	require.Equal(t, []weightedCheckinReward{{Value: 5, Weight: 44}, {Value: 10, Weight: 28}, {Value: 20, Weight: 20}, {Value: 50, Weight: 6}, {Value: 100, Weight: 2}}, creditCheckinRewards)
}

func TestCheckinService_BuyCollectibleCardUsesPointShopTier(t *testing.T) {
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo)

	result, err := svc.BuyCollectibleCard(context.Background(), 7, "rare")

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.purchaseInput.UserID)
	require.Equal(t, "rare", repo.purchaseInput.Rarity)
	require.Equal(t, 2400.0, repo.purchaseInput.Price)
	require.Equal(t, int64(999), repo.purchaseInput.EditionSupply)
	require.NotEmpty(t, repo.purchaseInput.CardKey)
	require.Equal(t, "rare", result.CollectibleCard.Rarity)
	require.Equal(t, int64(999), result.CollectibleCard.EditionSupply)
}

func TestCheckinService_BuyCollectibleCardRejectsUnsoldTier(t *testing.T) {
	svc := NewCheckinService(&checkinRepoStub{})

	_, err := svc.BuyCollectibleCard(context.Background(), 7, "epic")

	require.ErrorIs(t, err, ErrCheckinCardShopInvalidTier)
}

func TestCollectibleEditionSupplies(t *testing.T) {
	require.Equal(t, map[string]int64{
		"common":    9999,
		"good":      3333,
		"rare":      999,
		"epic":      300,
		"legendary": 100,
		"mythic":    30,
	}, collectibleEditionSupplies)
}

func TestCollectibleCardTemplates(t *testing.T) {
	require.Len(t, collectibleCardTemplates, 76)

	seenKeys := make(map[string]struct{}, len(collectibleCardTemplates))
	rarityCounts := map[string]int{}
	allowedRarities := map[string]struct{}{
		"common":    {},
		"good":      {},
		"rare":      {},
		"epic":      {},
		"legendary": {},
		"mythic":    {},
	}

	for _, template := range collectibleCardTemplates {
		require.NotEmpty(t, template.Key)
		require.NotEmpty(t, template.Rarity)
		require.Greater(t, template.Weight, int64(0))
		require.Contains(t, allowedRarities, template.Rarity)
		require.NotContains(t, seenKeys, template.Key)
		seenKeys[template.Key] = struct{}{}
		rarityCounts[template.Rarity]++
	}

	require.Equal(t, 30, rarityCounts["common"])
	require.Equal(t, 20, rarityCounts["good"])
	require.Equal(t, 12, rarityCounts["rare"])
	require.Equal(t, 8, rarityCounts["epic"])
	require.Equal(t, 4, rarityCounts["legendary"])
	require.Equal(t, 2, rarityCounts["mythic"])
	require.Contains(t, seenKeys, "low_battery_saint")
	require.Contains(t, seenKeys, "zero_point_pilot")
}

func TestWeightedRandomRewardRejectsInvalidWeights(t *testing.T) {
	_, err := randomCheckinReward([]weightedCheckinReward{{Value: 20, Weight: 0}})

	require.ErrorIs(t, err, ErrCheckinInvalidType)
}
