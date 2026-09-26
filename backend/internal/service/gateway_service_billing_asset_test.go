//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type billingAssetAPIKeyServiceStub struct {
	invalidated int64
}

func (s *billingAssetAPIKeyServiceStub) UpdateQuotaUsed(ctx context.Context, apiKeyID int64, cost float64) error {
	return nil
}

func (s *billingAssetAPIKeyServiceStub) UpdateRateLimitUsage(ctx context.Context, apiKeyID int64, cost float64) error {
	return nil
}

func (s *billingAssetAPIKeyServiceStub) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	atomic.AddInt64(&s.invalidated, 1)
}

func TestFinalizePostUsageBilling_CreditGroupDoesNotDeductBalanceCache(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	billingCache := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	billingCache.Stop()
	apiKeySvc := &billingAssetAPIKeyServiceStub{}
	groupID := int64(7)

	finalizePostUsageBilling(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 2.5, TotalCost: 2.5},
		User: &User{ID: 1},
		APIKey: &APIKey{
			ID:      2,
			Key:     "k-credit",
			GroupID: &groupID,
			Group:   &Group{ID: groupID, BillingAssetType: BillingAssetCredits},
		},
		Account:       &Account{ID: 3},
		APIKeyService: apiKeySvc,
	}, &billingDeps{
		billingCacheService: billingCache,
		deferredService:     &DeferredService{},
	}, &UsageBillingApplyResult{Applied: true})

	require.Zero(t, atomic.LoadInt64(&cache.balanceUpdates))
	require.Equal(t, int64(1), atomic.LoadInt64(&apiKeySvc.invalidated))
}

func TestFinalizePostUsageBilling_BalanceGroupDeductsBalanceCache(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	billingCache := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	billingCache.Stop()
	groupID := int64(7)

	finalizePostUsageBilling(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 2.5, TotalCost: 2.5},
		User: &User{ID: 1},
		APIKey: &APIKey{
			ID:      2,
			Key:     "k-balance",
			GroupID: &groupID,
			Group:   &Group{ID: groupID, BillingAssetType: BillingAssetBalance},
		},
		Account: &Account{ID: 3},
	}, &billingDeps{
		billingCacheService: billingCache,
		deferredService:     &DeferredService{},
	}, &UsageBillingApplyResult{Applied: true})

	require.Equal(t, int64(1), atomic.LoadInt64(&cache.balanceUpdates))
}
