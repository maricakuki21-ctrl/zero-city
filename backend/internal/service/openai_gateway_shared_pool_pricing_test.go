package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type canonicalSharedPoolGatewayRepoStub struct {
	BizDecipherRepository
	sharedPoolPricingRepository
	ensuredBase  SharedPoolPriceComponents
	accessKey    *SharedPoolAccessKey
	resolveError error
	ensureCalls  int
}

func (r *canonicalSharedPoolGatewayRepoStub) GetSharedPoolAccessKeyByAPIKeyID(context.Context, int64, string) (*SharedPoolAccessKey, error) {
	if r.accessKey != nil {
		copy := *r.accessKey
		return &copy, nil
	}
	return &SharedPoolAccessKey{
		ID: 1, PoolID: 9, UserID: 7, PublishedModelName: "published-alias",
		CanonicalModelName: "canonical-official-model", RateMultiplier: 1,
	}, nil
}

func (r *canonicalSharedPoolGatewayRepoStub) GetSharedPoolMediaAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, _ string) (*SharedPoolAccessKey, error) {
	return r.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
}

func (r *canonicalSharedPoolGatewayRepoStub) ResolveSharedPoolPriceQuote(context.Context, int64, string, string) (*SharedPoolPriceQuote, error) {
	if r.resolveError != nil {
		return nil, r.resolveError
	}
	return nil, ErrSharedPoolOfficialPriceRequired
}

func (r *canonicalSharedPoolGatewayRepoStub) EnsureSharedPoolOfficialPriceVersion(_ context.Context, poolID int64, modelName, endpointType string, base SharedPoolPriceComponents) (*SharedPoolPriceQuote, error) {
	r.ensureCalls++
	r.ensuredBase = base
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 71, PoolID: poolID, PoolModelID: 72, EndpointID: 73,
		ModelName: modelName, EndpointType: endpointType,
		PricingSource: SharedPoolPricingSourceOfficial, PricingStatus: "ready", ConfigVersion: 1,
		BasePrice: base, Multiplier: 1, EffectiveFrom: time.Now(),
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func TestSharedPoolRuntimeUsesCanonicalCatalogIdentityForOfficialBilling(t *testing.T) {
	t.Parallel()
	repo := &canonicalSharedPoolGatewayRepoStub{}
	biz := NewBizDecipherService(repo, nil, nil)
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"canonical-official-model": {InputCostPerToken: 2e-6, OutputCostPerToken: 8e-6},
	}})
	gateway := &OpenAIGatewayService{bizDecipherService: biz, billingService: billing}

	accessKey, err := gateway.GetSharedPoolAccessKeyQuoteByAPIKeyID(context.Background(), 11, "published-alias", SharedPoolEndpointChat)
	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.NotNil(t, accessKey.PriceQuote)
	require.InDelta(t, 2e-6, *repo.ensuredBase.InputPrice, 1e-12)
	require.InDelta(t, 8e-6, *repo.ensuredBase.OutputPrice, 1e-12)
	require.Equal(t, 1, repo.ensureCalls)
}

func TestSharedPoolOfficialMediaRejectsUnsupportedVariantsBeforePriceVersionWrite(t *testing.T) {
	tests := []struct {
		name            string
		endpointType    string
		imageSize       string
		videoResolution string
	}{
		{name: "image 2K", endpointType: SharedPoolEndpointImageGeneration, imageSize: ImageBillingSize2K},
		{name: "image 4K", endpointType: SharedPoolEndpointImageEdit, imageSize: ImageBillingSize4K},
		{name: "video 720p", endpointType: SharedPoolEndpointVideo, videoResolution: VideoBillingResolution720P},
		{name: "video 1080p", endpointType: SharedPoolEndpointVideo, videoResolution: VideoBillingResolution1080P},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &canonicalSharedPoolGatewayRepoStub{}
			biz := NewBizDecipherService(repo, nil, nil)
			gateway := &OpenAIGatewayService{bizDecipherService: biz}

			accessKey, err := gateway.GetSharedPoolMediaAccessKeyQuoteByAPIKeyID(
				context.Background(), 11, "published-alias", tt.endpointType, tt.imageSize, tt.videoResolution,
			)
			require.Nil(t, accessKey)
			require.ErrorIs(t, err, ErrSharedPoolOfficialMediaVariantUnsupported)
			require.Zero(t, repo.ensureCalls, "unsupported official variants must fail before any price-version write")
		})
	}
}

func TestSharedPoolRuntimeNeverFallsBackToPublishedNameForOwnerModel(t *testing.T) {
	t.Parallel()
	repo := &canonicalSharedPoolGatewayRepoStub{
		accessKey: &SharedPoolAccessKey{
			ID: 1, PoolID: 9, UserID: 7, PublishedModelName: "official-model", RateMultiplier: 1,
		},
		resolveError: ErrSharedPoolPricingUnavailable,
	}
	biz := NewBizDecipherService(repo, nil, nil)
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"official-model": {InputCostPerToken: 2e-6, OutputCostPerToken: 8e-6},
	}})
	gateway := &OpenAIGatewayService{bizDecipherService: biz, billingService: billing}

	accessKey, err := gateway.GetSharedPoolAccessKeyQuoteByAPIKeyID(context.Background(), 11, "official-model", SharedPoolEndpointChat)
	require.Nil(t, accessKey)
	require.ErrorIs(t, err, ErrSharedPoolPricingNotConfigured)
	require.Zero(t, repo.ensureCalls, "owner-defined models must never inherit provider-less official pricing")
}

func TestValidateSharedPoolPricingRequiresFrozenVersion(t *testing.T) {
	t.Parallel()
	input := 1e-6
	output := 4e-6
	quote := testSharedPoolQuote(&input, &output, 1.2)
	err := (&OpenAIGatewayService{}).ValidateSharedPoolPricing(context.Background(), &APIKey{}, &SharedPoolAccessKey{PublishedModelName: "official-model", PriceQuote: quote}, "official-model")
	require.NoError(t, err)

	err = (&OpenAIGatewayService{}).ValidateSharedPoolPricing(context.Background(), &APIKey{}, &SharedPoolAccessKey{PublishedModelName: "official-model"}, "official-model")
	require.ErrorIs(t, err, ErrSharedPoolPricingNotConfigured)
}

func TestSharedPoolAcceptedQuoteDoesNotDriftAfterNewPrice(t *testing.T) {
	t.Parallel()
	oldInput := 1e-6
	oldOutput := 4e-6
	accepted := testSharedPoolQuote(&oldInput, &oldOutput, 1.5)
	first, err := calculateSharedPoolQuoteCost(accepted, UsageTokens{InputTokens: 1000, OutputTokens: 500})
	require.NoError(t, err)
	require.InDelta(t, 0.0045, first, 1e-12)

	newInput := 10e-6
	newOutput := 40e-6
	newVersion := testSharedPoolQuote(&newInput, &newOutput, 2)
	newVersion.PriceVersionID = 12
	newCost, err := calculateSharedPoolQuoteCost(newVersion, UsageTokens{InputTokens: 1000, OutputTokens: 500})
	require.NoError(t, err)
	require.InDelta(t, 0.06, newCost, 1e-12)

	again, err := calculateSharedPoolQuoteCost(accepted, UsageTokens{InputTokens: 1000, OutputTokens: 500})
	require.NoError(t, err)
	require.Equal(t, first, again, "an accepted request must retain its old version components")
}

func TestSharedPoolPerRequestQuoteAppliesFixedBounds(t *testing.T) {
	t.Parallel()
	price := 0.01
	minimum := 0.03
	maximum := 0.04
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 8, PoolID: 1, PoolModelID: 2, EndpointID: 3,
		ModelName: "owner-model", EndpointType: SharedPoolEndpointResponses,
		PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "ready", ConfigVersion: 2,
		BasePrice:  SharedPoolPriceComponents{BillingMode: "per_request", Currency: "USD", PerRequestPrice: &price, MinimumCharge: &minimum, MaximumCharge: &maximum},
		Multiplier: 2, EffectiveFrom: time.Now(),
	}
	FinalizeSharedPoolPriceQuote(quote)
	cost, err := calculateSharedPoolQuoteCost(quote, UsageTokens{})
	require.NoError(t, err)
	require.Equal(t, 0.03, cost)
}

func TestSharedPoolTokenQuoteRejectsSuccessfulResponseWithoutUsage(t *testing.T) {
	t.Parallel()
	input := 1e-6
	output := 4e-6
	quote := testSharedPoolQuote(&input, &output, 1)
	minimum := 0.01
	quote.BasePrice.MinimumCharge = &minimum

	err := validateSharedPoolForwardResultForQuote(quote, &OpenAIForwardResult{})
	require.ErrorIs(t, err, ErrSharedPoolUsageUnavailable)
}

func TestSharedPoolTokenQuoteRejectsUnpricedResultDimensions(t *testing.T) {
	t.Parallel()
	input := 1e-6
	output := 4e-6
	quote := testSharedPoolQuote(&input, &output, 1)

	results := []*OpenAIForwardResult{
		{Usage: OpenAIUsage{InputTokens: 10, ImageOutputTokens: 1}},
		{Usage: OpenAIUsage{InputTokens: 10}, WebSearchCalls: 1},
		{Usage: OpenAIUsage{InputTokens: 10}, VideoCount: 1},
	}
	for _, result := range results {
		require.ErrorIs(t, validateSharedPoolForwardResultForQuote(quote, result), ErrSharedPoolUnsafeCostEstimate)
	}
}

func TestSharedPoolTokenQuoteAllowsVisionInputTokens(t *testing.T) {
	t.Parallel()
	input := 1e-6
	output := 4e-6
	quote := testSharedPoolQuote(&input, &output, 1)
	result := &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 352, ImageInputTokens: 352, OutputTokens: 10}}

	require.NoError(t, validateSharedPoolForwardResultForQuote(quote, result))
	cost, err := calculateSharedPoolQuoteCost(quote, UsageTokens{
		InputTokens:      result.Usage.InputTokens,
		ImageInputTokens: result.Usage.ImageInputTokens,
		OutputTokens:     result.Usage.OutputTokens,
	})
	require.NoError(t, err)
	require.InDelta(t, 352*input+10*output, cost, 1e-12)
}

func TestSharedPoolEffectiveMultiplierIsFrozenWithoutMutatingBaseVersion(t *testing.T) {
	t.Parallel()
	input := 1e-6
	output := 4e-6
	base := testSharedPoolQuote(&input, &output, 1.2)

	selected := sharedPoolQuoteWithEffectiveMultiplier(base, 1.75)
	require.NotSame(t, base, selected)
	require.InDelta(t, 1.2, base.Multiplier, 1e-12)
	require.InDelta(t, 1.75, selected.Multiplier, 1e-12)
	require.Equal(t, base.PriceVersionID, selected.PriceVersionID)
	require.Empty(t, selected.PriceHash)

	cost, err := calculateSharedPoolQuoteCost(selected, UsageTokens{InputTokens: 1000, OutputTokens: 500})
	require.NoError(t, err)
	require.InDelta(t, 0.00525, cost, 1e-12)
}

func TestSharedPoolOfficialPolicyChargesLongContextInputCacheAndOutput(t *testing.T) {
	t.Parallel()
	input := 1e-6
	output := 4e-6
	cacheRead := 0.25e-6
	cacheWrite := 1.25e-6
	quote := testSharedPoolQuote(&input, &output, 1)
	quote.BasePrice.CacheReadPrice = &cacheRead
	quote.BasePrice.CacheWritePrice = &cacheWrite
	quote.LongContextInputThreshold = 100
	quote.LongContextInputMultiplier = 2
	quote.LongContextOutputMultiplier = 1.5

	tokens := UsageTokens{InputTokens: 80, CacheReadTokens: 20, CacheCreationTokens: 1, OutputTokens: 10}
	cost, err := calculateSharedPoolQuoteCost(quote, tokens)
	require.NoError(t, err)
	want := (80*input+20*cacheRead+1*cacheWrite)*2 + 10*output*1.5
	require.InDelta(t, want, cost, 1e-12)
}

func TestSharedPoolOfficialBasePricePreservesZeroCachePrices(t *testing.T) {
	t.Parallel()
	base := sharedPoolOfficialBasePrice(&ModelPricing{InputPricePerToken: 2e-6, OutputPricePerToken: 8e-6})
	require.NotNil(t, base.CacheReadPrice)
	require.NotNil(t, base.CacheWritePrice)
	require.Zero(t, *base.CacheReadPrice)
	require.Zero(t, *base.CacheWritePrice)
}

func TestSharedPoolCustomPriceValidationFailsClosed(t *testing.T) {
	t.Parallel()
	input := 1e-6
	negative := -1.0
	maximum := 0.1
	minimum := 0.2

	tests := []SaveSharedPoolCustomPriceInput{
		{PoolID: 1, OwnerID: 2, ModelName: "owner-model", EndpointType: "video", OperationID: "op-1", BillingMode: "token", InputPrice: &input, OutputPrice: &input, Multiplier: 1},
		{PoolID: 1, OwnerID: 2, ModelName: "owner-model", EndpointType: "chat", OperationID: "op-2", BillingMode: "token", InputPrice: &input, Multiplier: 1},
		{PoolID: 1, OwnerID: 2, ModelName: "owner-model", EndpointType: "chat", OperationID: "op-3", BillingMode: "per_request", PerRequestPrice: &negative, Multiplier: 1},
		{PoolID: 1, OwnerID: 2, ModelName: "owner-model", EndpointType: "responses", OperationID: "op-4", BillingMode: "per_request", PerRequestPrice: &input, Multiplier: 1, MinimumCharge: &minimum, MaximumCharge: &maximum},
	}
	for _, input := range tests {
		input := input
		require.Error(t, normalizeAndValidateSharedPoolCustomPrice(&input))
	}
}

func TestSharedPoolUnknownOrIncompleteQuoteIsRejected(t *testing.T) {
	t.Parallel()
	input := 1e-6
	quote := testSharedPoolQuote(&input, nil, 1)
	require.Error(t, ValidateSharedPoolPriceQuote(quote))
	require.True(t, errors.Is((&OpenAIGatewayService{}).ValidateSharedPoolPricing(context.Background(), &APIKey{}, &SharedPoolAccessKey{PriceQuote: quote}, "unknown"), ErrSharedPoolPricingNotConfigured))
}

func testSharedPoolQuote(input, output *float64, multiplier float64) *SharedPoolPriceQuote {
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 11, PoolID: 1, PoolModelID: 2, EndpointID: 3,
		ModelName: "model", EndpointType: SharedPoolEndpointChat,
		PricingSource: SharedPoolPricingSourceOfficial, PricingStatus: "ready", ConfigVersion: 4,
		BasePrice:  SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: input, OutputPrice: output},
		Multiplier: multiplier, EffectiveFrom: time.Now(),
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote
}
