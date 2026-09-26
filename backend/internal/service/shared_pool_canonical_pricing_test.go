package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type canonicalSharedPoolPricingRepoStub struct {
	BizDecipherRepository
	sharedPoolPricingRepository
	catalog   []ModelCatalogEntry
	pools     []SharedPool
	endpoints []SharedPoolModelEndpointPricing
}

func (r *canonicalSharedPoolPricingRepoStub) ListModelCatalog(context.Context) ([]ModelCatalogEntry, error) {
	return append([]ModelCatalogEntry(nil), r.catalog...), nil
}

func (r *canonicalSharedPoolPricingRepoStub) ListSharedPools(context.Context, SharedPoolFilter) ([]SharedPool, error) {
	return append([]SharedPool(nil), r.pools...), nil
}

func (r *canonicalSharedPoolPricingRepoStub) ListSharedPoolModelEndpointPricing(context.Context, int64, int64) ([]SharedPoolModelEndpointPricing, error) {
	return append([]SharedPoolModelEndpointPricing(nil), r.endpoints...), nil
}

func newCanonicalSharedPoolPricingService(repo BizDecipherRepository) *BizDecipherService {
	pricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"official-model": {
			InputCostPerToken:           2e-6,
			OutputCostPerToken:          8e-6,
			CacheReadInputTokenCost:     0.2e-6,
			CacheCreationInputTokenCost: 2.5e-6,
		},
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	svc.SetBillingService(NewBillingService(&config.Config{}, pricing))
	return svc
}

func TestListModelCatalogOverwritesLegacyPricingWithCanonicalBillingService(t *testing.T) {
	t.Parallel()
	legacyPrice := 99.0
	repo := &canonicalSharedPoolPricingRepoStub{catalog: []ModelCatalogEntry{
		{ID: 1, ModelName: "official-model", Pricing: &SharedPoolPriceSnapshot{InputPrice: &legacyPrice, PriceSource: "channel_model_pricing"}},
		{ID: 2, ModelName: "unknown-never-priced", Pricing: &SharedPoolPriceSnapshot{InputPrice: &legacyPrice, PriceSource: "channel_model_pricing"}},
	}}

	catalog, err := newCanonicalSharedPoolPricingService(repo).ListModelCatalog(context.Background())
	require.NoError(t, err)
	require.Len(t, catalog, 2)
	require.NotNil(t, catalog[0].Pricing)
	require.Equal(t, "billing_service", catalog[0].Pricing.PriceSource)
	require.InDelta(t, 2e-6, *catalog[0].Pricing.InputPrice, 1e-12)
	require.InDelta(t, 8e-6, *catalog[0].Pricing.OutputPrice, 1e-12)
	require.Nil(t, catalog[1].Pricing, "canonical lookup failures must hide stale repository prices")
}

func TestListSharedPoolsShowsCanonicalOfficialPriceAndHidesUnresolvedPrices(t *testing.T) {
	t.Parallel()
	legacyPrice := 99.0
	repo := &canonicalSharedPoolPricingRepoStub{pools: []SharedPool{{
		ID: 9,
		ModelConfigs: []SharedPoolModelConfig{
			{Provider: "openai", ModelName: "published-alias", CanonicalModelName: "official-model", Pricing: &SharedPoolPriceSnapshot{InputPrice: &legacyPrice, PriceSource: "channel_model_pricing"}},
			{Provider: "custom", ModelName: "owner-model", Pricing: &SharedPoolPriceSnapshot{InputPrice: &legacyPrice, PriceSource: "channel_model_pricing"}},
		},
	}}}

	view, err := newCanonicalSharedPoolPricingService(repo).ListSharedPools(context.Background(), SharedPoolFilter{})
	require.NoError(t, err)
	require.Len(t, view.Pools, 1)
	require.Equal(t, "billing_service", view.Pools[0].ModelConfigs[0].Pricing.PriceSource)
	require.InDelta(t, 2e-6, *view.Pools[0].ModelConfigs[0].Pricing.InputPrice, 1e-12)
	require.Nil(t, view.Pools[0].ModelConfigs[1].Pricing, "non-catalog models must not inherit channel-table prices")
}

func TestListSharedPoolEndpointPricingUsesCanonicalPriceAndFailsClosed(t *testing.T) {
	t.Parallel()
	inputCustom, outputCustom := 1e-6, 4e-6
	repo := &canonicalSharedPoolPricingRepoStub{endpoints: []SharedPoolModelEndpointPricing{
		{
			PoolModelID: 1, ModelName: "published-alias", EndpointID: 11, EndpointType: SharedPoolEndpointChat,
			CanonicalModelName: "official-model", RateMultiplier: 1.5,
			PricingSource: SharedPoolPricingSourceOfficial, PricingStatus: "ready", EndpointPricingStatus: "ready",
			CurrentPrice: &SharedPoolPriceQuote{PriceVersionID: 91, PricingSource: SharedPoolPricingSourceOfficial, PriceHash: "stale-channel-derived-hash"},
		},
		{
			PoolModelID: 2, ModelName: "unknown", EndpointID: 12, EndpointType: SharedPoolEndpointChat,
			CanonicalModelName: "unknown-never-priced", RateMultiplier: 1,
			PricingSource: SharedPoolPricingSourceOfficial, PricingStatus: "ready", EndpointPricingStatus: "ready",
		},
		{
			PoolModelID: 3, ModelName: "owner-model", EndpointID: 13, EndpointType: SharedPoolEndpointResponses,
			RateMultiplier: 0.0001,
			PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "ready", EndpointPricingStatus: "ready",
			CurrentPrice: &SharedPoolPriceQuote{
				PriceVersionID: 93, PricingSource: SharedPoolPricingSourceOwner,
				BasePrice: SharedPoolPriceComponents{BillingMode: "token", InputPrice: &inputCustom, OutputPrice: &outputCustom},
				Multiplier: 1,
			},
		},
	}}

	items, err := newCanonicalSharedPoolPricingService(repo).ListSharedPoolModelEndpointPricing(context.Background(), 9, 7)
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, SharedPoolPricingSourceOfficial, items[0].PricingSource)
	require.InDelta(t, 2e-6, *items[0].CurrentPrice.BasePrice.InputPrice, 1e-12)
	require.InDelta(t, 12e-6, *items[0].CurrentPrice.UserPrice.OutputPrice, 1e-12)
	require.Zero(t, items[0].CurrentPrice.PriceVersionID, "a stale persisted version must not be attached to a new canonical preview")
	require.Equal(t, "invalid", items[1].PricingStatus)
	require.Equal(t, "invalid", items[1].EndpointPricingStatus)
	require.Nil(t, items[1].CurrentPrice)
	require.Equal(t, int64(93), items[2].CurrentPrice.PriceVersionID)
	require.InDelta(t, 0.0001, items[2].CurrentPrice.Multiplier, 1e-12)
	require.InDelta(t, outputCustom*0.0001, *items[2].CurrentPrice.UserPrice.OutputPrice, 1e-18)
}

func TestListSharedPoolEndpointPricingDoesNotAdvertiseOfficialPriceWithoutCatalogIdentity(t *testing.T) {
	t.Parallel()
	repo := &canonicalSharedPoolPricingRepoStub{endpoints: []SharedPoolModelEndpointPricing{
		{
			PoolModelID: 4, Provider: "openai_compatible", ModelName: "gpt-5.6-terra", EndpointID: 14,
			EndpointType: SharedPoolEndpointResponses, RateMultiplier: 1.2,
			PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "pending", EndpointPricingStatus: "pending",
		},
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	svc.SetBillingService(NewBillingService(&config.Config{}, nil))

	items, err := svc.ListSharedPoolModelEndpointPricing(context.Background(), 9, 7)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, SharedPoolPricingSourceOwner, items[0].PricingSource)
	require.Equal(t, "pending", items[0].PricingStatus)
	require.Equal(t, "pending", items[0].EndpointPricingStatus)
	require.Nil(t, items[0].CurrentPrice)
}

func TestListSharedPoolEndpointPricingDoesNotTrustLocalizedProviderWithoutCatalogIdentity(t *testing.T) {
	t.Parallel()
	repo := &canonicalSharedPoolPricingRepoStub{endpoints: []SharedPoolModelEndpointPricing{
		{
			PoolModelID: 5, Provider: "OpenAI 兼容中转", ModelName: "gpt-5.6-sol", EndpointID: 15,
			EndpointType: SharedPoolEndpointResponses, RateMultiplier: 1,
			PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "pending", EndpointPricingStatus: "pending",
		},
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	svc.SetBillingService(NewBillingService(&config.Config{}, nil))

	items, err := svc.ListSharedPoolModelEndpointPricing(context.Background(), 9, 7)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, SharedPoolPricingSourceOwner, items[0].PricingSource)
	require.Equal(t, "pending", items[0].PricingStatus)
	require.Nil(t, items[0].CurrentPrice)
}
