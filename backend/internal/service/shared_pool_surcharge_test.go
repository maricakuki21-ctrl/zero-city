package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolBuyerSurchargeAllBillingPaths(t *testing.T) {
	for _, mode := range []string{"token", "per_request", "image", "video"} {
		t.Run(mode, func(t *testing.T) {
			price := 5.0
			quote := testSharedPoolQuote(&price, nil, 2)
			quote.BasePrice = SharedPoolPriceComponents{BillingMode: mode, Currency: "USD"}
			switch mode {
			case "token":
				quote.BasePrice.InputPrice = &price
				quote.BasePrice.OutputPrice = &price
			case "per_request":
				quote.BasePrice.PerRequestPrice = &price
			case "image":
				quote.EndpointType = SharedPoolEndpointImageGeneration
				quote.BasePrice.ImageItemPrice = &price
			case "video":
				quote.EndpointType = SharedPoolEndpointVideo
				quote.BasePrice.VideoSecondPrice = &price
			}
			quote.FeeMode = SharedPoolFeeModeBuyerSurcharge
			quote.PlatformFeePercent = 10
			FinalizeSharedPoolPriceQuote(quote)
			require.NotNil(t, quote.OwnerPrice)
			require.Equal(t, 5.0, price, "owner's configured price must not be mutated")

			var charge float64
			var err error
			if mode == "image" || mode == "video" {
				charge, err = calculateSharedPoolMediaCharge(quote, quote.EndpointType, 1)
			} else {
				charge, err = calculateSharedPoolQuoteCost(quote, UsageTokens{InputTokens: 1})
			}
			require.NoError(t, err)
			require.Equal(t, 11.0, charge)

			// Owner bounds apply first; the accepted buyer ceiling includes the fee.
			maximum := 10.0
			quote.BasePrice.MaximumCharge = &maximum
			FinalizeSharedPoolPriceQuote(quote)
			require.Equal(t, 10.0, *quote.OwnerPrice.MaximumCharge)
			require.Equal(t, 11.0, *quote.UserPrice.MaximumCharge)
			if mode == "token" || mode == "per_request" {
				hold, err := calculateSharedPoolMaximumHold(quote, []byte(`{"input":"hello"}`), 100, false)
				require.NoError(t, err)
				require.Equal(t, 11.0, hold)
			} else {
				charge, err = calculateSharedPoolMediaCharge(quote, quote.EndpointType, 100)
				require.NoError(t, err)
				require.Equal(t, 11.0, charge)
			}
		})
	}
}

func TestSharedPoolBuyerSurchargeSnapshotAndClone(t *testing.T) {
	price := 10.0
	quote := testSharedPoolQuote(nil, nil, 1)
	quote.BasePrice = SharedPoolPriceComponents{BillingMode: "per_request", PerRequestPrice: &price}
	quote.FeeMode, quote.PlatformFeePercent = SharedPoolFeeModeBuyerSurcharge, 10
	FinalizeSharedPoolPriceQuote(quote)
	FinalizeSharedPoolPriceQuote(quote)
	require.Equal(t, 11.0, quote.ExampleCost, "finalization must not add the fee twice")
	require.Equal(t, 10.0, *quote.OwnerPrice.PerRequestPrice)

	clone := cloneSharedPoolPriceQuote(quote)
	*clone.OwnerPrice.PerRequestPrice = 999
	require.Equal(t, 10.0, *quote.OwnerPrice.PerRequestPrice)
	var restored SharedPoolPriceQuote
	require.NoError(t, json.Unmarshal(MarshalSharedPoolPriceSnapshot(quote), &restored))
	cost, err := calculateSharedPoolQuoteCost(&restored, UsageTokens{})
	require.NoError(t, err)
	require.Equal(t, 11.0, cost)

	// Historical quotes have no fee mode, even if a newer pool fee is known.
	restored.FeeMode = ""
	cost, err = calculateSharedPoolQuoteCost(&restored, UsageTokens{})
	require.NoError(t, err)
	require.Equal(t, 10.0, cost)
}

func TestSharedPoolBuyerSurchargeRoundingAndZeroFee(t *testing.T) {
	for _, tc := range []struct{ owner, percent, buyer float64 }{
		{10, 10, 11}, {10, 0, 10}, {0, 10, 0}, {10, 100, 20},
		{0.00000001, 10, 0.00000001}, {0.00000005, 10, 0.00000006},
	} {
		require.InDelta(t, tc.buyer, SharedPoolBuyerCharge(tc.owner, tc.percent), 1e-15)
	}
}

type surchargePricingRepo struct {
	BizDecipherRepository
	sharedPoolPricingRepository
	fee float64
	err error
}

func (r *surchargePricingRepo) SharedPoolPlatformFeePercent(context.Context, int64) (float64, error) {
	return r.fee, r.err
}

func TestSharedPoolBuyerSurchargeFeeLookup(t *testing.T) {
	repo := &surchargePricingRepo{fee: 10}
	svc := NewBizDecipherService(repo, nil, nil)
	quote := &SharedPoolPriceQuote{PoolID: 1}
	require.NoError(t, svc.attachSharedPoolPlatformFee(context.Background(), quote))
	require.Equal(t, SharedPoolFeeModeBuyerSurcharge, quote.FeeMode)
	require.Equal(t, 10.0, quote.PlatformFeePercent)
	repo.err = errors.New("fee storage unavailable")
	require.ErrorContains(t, svc.attachSharedPoolPlatformFee(context.Background(), quote), "unavailable")
}
