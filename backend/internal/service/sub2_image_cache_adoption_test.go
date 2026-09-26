package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSub2ImageCacheUsageParsing(t *testing.T) {
	for _, tc := range []struct {
		name, body          string
		cached, imageCached int
	}{
		{"explicit", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":40,"text_tokens":30}}}}`, 70, 40},
		{"details only", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens_details":{"image_tokens":40,"text_tokens":30}}}}`, 70, 40},
		{"unknown split", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70}}}`, 70, 0},
		{"explicit zero", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":0,"cached_tokens_details":{"image_tokens":40}}}}`, 0, 0},
		{"bounded", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":500}}}}`, 70, 70},
		{"negative", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":-1}}}}`, 70, 0},
		{"fractional", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":1.5}}}}`, 70, 0},
		{"overflow", `{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":1e100}}}}`, 70, 0},
		{"chat", `{"usage":{"prompt_tokens":200,"prompt_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":40}}}}`, 70, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usage, ok := extractOpenAIUsageFromJSONBytes([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.cached, usage.CacheReadInputTokens)
			require.Equal(t, tc.imageCached, usage.ImageCacheReadTokens)
			var merged OpenAIUsage
			mergeOpenAIUsage(&merged, []byte(tc.body))
			require.Equal(t, tc.imageCached, merged.ImageCacheReadTokens)
		})
	}
}

func TestSub2ImageCachePricingAndChannelOverride(t *testing.T) {
	pricingSvc := &PricingService{}
	data, err := pricingSvc.parsePricingData([]byte(`{"cache-test":{"input_cost_per_token":0.000002,"output_cost_per_token":0.000004,"cache_read_input_token_cost":0.000001,"cache_read_input_image_token_cost":0.000003}}`))
	require.NoError(t, err)
	pricingSvc.pricingData = data
	svc := NewBillingService(nil, pricingSvc)
	pricing, err := svc.GetModelPricing("cache-test")
	require.NoError(t, err)
	require.Equal(t, 3e-6, pricing.ImageCacheReadPricePerToken)
	tokens := UsageTokens{CacheReadTokens: 70, ImageCacheReadTokens: 40}
	cost := svc.computeTokenBreakdown(pricing, tokens, 1, "", false)
	require.InDelta(t, 30e-6+40*3e-6, cost.CacheReadCost, 1e-12)

	for _, rate := range []float64{0, 7e-6} {
		channel, err := svc.GetModelPricingWithChannel("cache-test", &ChannelModelPricing{CacheReadPrice: &rate})
		require.NoError(t, err)
		cost := svc.computeTokenBreakdown(channel, tokens, 1, "", false)
		require.InDelta(t, 70*rate, cost.CacheReadCost, 1e-12)
		resolver := &ModelPricingResolver{}
		resolved := &ResolvedPricing{BasePricing: pricing}
		resolver.applyTokenOverrides(&ChannelModelPricing{CacheReadPrice: &rate}, resolved)
		resolvedCost := svc.computeTokenBreakdown(resolved.BasePricing, tokens, 1, "", false)
		require.InDelta(t, 70*rate, resolvedCost.CacheReadCost, 1e-12)
	}
	require.Equal(t, 3e-6, pricing.ImageCacheReadPricePerToken)

	pricing.ImageCacheReadPricePerToken = 0
	cost = svc.computeTokenBreakdown(pricing, tokens, 1, "", false)
	require.InDelta(t, 70e-6, cost.CacheReadCost, 1e-12)
	pricing.ImageCacheReadPricePerToken = 3e-6
	tokens.ImageCacheReadTokens = 200
	cost = svc.computeTokenBreakdown(pricing, tokens, 1, "", false)
	require.InDelta(t, 70*3e-6, cost.CacheReadCost, 1e-12)
}

func TestSub2ImageCacheRecordUsage(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.billingService = NewBillingService(svc.cfg, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"cache-test": {
			InputCostPerToken: 2e-6, InputCostPerImageToken: 8e-6,
			OutputCostPerToken: 4e-6, CacheReadInputTokenCost: 1e-6,
			CacheReadInputImageTokenCost: 3e-6,
		},
	}})
	usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{"usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":40,"text_tokens":30}}}}`))
	require.True(t, ok)
	var progressive OpenAIUsage
	svc.parseSSEUsageBytes([]byte(`{"type":"response.in_progress","usage":{"input_tokens":200,"input_tokens_details":{"image_tokens":120,"cached_tokens":70,"cached_tokens_details":{"image_tokens":40}}}}`), &progressive)
	svc.parseSSEUsageBytes([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":0,"output_tokens":0}}}`), &progressive)
	svc.parseSSEUsageBytes([]byte(`{"type":"response.done","response":{}}`), &progressive)
	require.Equal(t, usage, progressive)
	result := &OpenAIForwardResult{
		RequestID: "image-cache-adoption", Model: "cache-test",
		Usage: progressive, Duration: time.Second,
		ImageSizeBreakdown: map[string]int{"1024x1024": 1},
	}
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: result, APIKey: &APIKey{ID: 1001},
		User: &User{ID: 2001}, Account: &Account{ID: 3001},
	})
	require.NoError(t, err)
	log := usageRepo.lastLog
	require.NotNil(t, log)
	require.Equal(t, 130, log.InputTokens)
	require.InDelta(t, 50*2e-6, log.InputCost, 1e-12)
	require.InDelta(t, 80*8e-6, log.ImageInputCost, 1e-12)
	require.InDelta(t, 30e-6+40*3e-6, log.CacheReadCost, 1e-12)
	require.Equal(t, 40, log.ImageSizeBreakdown["image_cache_read_tokens"])
	require.Equal(t, 1, log.ImageSizeBreakdown["1024x1024"])
	require.NotContains(t, result.ImageSizeBreakdown, "image_cache_read_tokens")
	require.InDelta(t, log.TotalCost*1.1, log.ActualCost, 1e-12)
}
