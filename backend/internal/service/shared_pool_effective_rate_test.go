package service

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSharedPoolModelInputsPreservesInheritedConcurrencyZero(t *testing.T) {
	models := normalizeSharedPoolModelInputs([]SharedPoolModelInput{{
		ModelName:      "gpt-5.6-luna",
		RateMultiplier: 0,
		MaxConcurrency: 0,
		ModelOpen:      true,
	}}, nil, 0.0001)

	require.Len(t, models, 1)
	require.InDelta(t, 0.0001, models[0].RateMultiplier, 1e-12)
	require.Zero(t, models[0].MaxConcurrency, "zero means inherit the account/pool concurrency limit")
}

func TestSharedPoolEffectiveMultiplierKeepsPreviewHoldAndSettlementConsistent(t *testing.T) {
	inputPrice := 1e-6
	outputPrice := 4e-6
	base := &SharedPoolPriceQuote{
		PriceVersionID: 1,
		PoolID:        2,
		PoolModelID:   3,
		EndpointID:    4,
		ModelName:     "owner-model",
		EndpointType:  SharedPoolEndpointChat,
		PricingSource: SharedPoolPricingSourceOwner,
		PricingStatus: "ready",
		BasePrice: SharedPoolPriceComponents{
			BillingMode: "token",
			Currency:    "USD",
			InputPrice:  &inputPrice,
			OutputPrice: &outputPrice,
		},
		Multiplier:    1,
		EffectiveFrom: time.Now(),
	}
	FinalizeSharedPoolPriceQuote(base)

	quote := sharedPoolQuoteWithEffectiveMultiplier(base, 0.0001)
	require.NotSame(t, base, quote)
	require.Equal(t, 1.0, base.Multiplier)
	require.InDelta(t, 0.0001, quote.Multiplier, 1e-12)
	require.InDelta(t, inputPrice*0.0001, *quote.UserPrice.InputPrice, 1e-18)
	require.InDelta(t, outputPrice*0.0001, *quote.UserPrice.OutputPrice, 1e-18)

	settled, err := calculateSharedPoolQuoteCost(quote, UsageTokens{InputTokens: 1000, OutputTokens: 500})
	require.NoError(t, err)
	require.InDelta(t, quote.ExampleCost, settled, 1e-12)

	body := []byte(`{"model":"owner-model"}`)
	hold, err := calculateSharedPoolMaximumHold(quote, body, 500, false)
	require.NoError(t, err)
	maxInputTokens := int64(len(bytes.TrimSpace(body))) + sharedPoolInputTokenOverhead
	expectedHold := quantizedSharedPoolCharge((float64(maxInputTokens)*inputPrice+500*outputPrice)*quote.Multiplier, nil)
	require.InDelta(t, expectedHold, hold, 1e-12)
}
