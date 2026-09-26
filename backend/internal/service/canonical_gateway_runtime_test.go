package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/stretchr/testify/require"
)

func TestCanonicalGatewayRuntimeCommitsBillingAndRetryBoundary(t *testing.T) {
	// Given
	retryable := errors.New("retryable")
	runtime, err := NewCanonicalGatewayRuntime(CanonicalGatewayRuntimeInput{
		GroupID:       17,
		AccountID:     29,
		BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
	}, func(err error) bool { return errors.Is(err, retryable) })
	require.NoError(t, err)
	ctx := WithCanonicalGatewayRuntime(context.Background(), runtime)

	bound, ok := CanonicalGatewayRuntimeFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, int64(17), bound.GroupID())
	accountID, ok := bound.AccountID()
	require.True(t, ok)
	require.Equal(t, int64(29), accountID)
	policy, err := ResolveRequestBillingPolicy(ctx, "")
	require.NoError(t, err)
	require.Equal(t, corecontracts.BillingPolicyBizDecipherLedger, policy)
	require.True(t, bound.CanRetry(retryable))

	require.NoError(t, bound.Commit())
	require.False(t, bound.CanRetry(retryable))
}

func TestCanonicalGatewayRuntimeAcceptTaskPermanentlyClosesCreateRetry(t *testing.T) {
	// Given
	runtime, err := NewCanonicalGatewayRuntime(CanonicalGatewayRuntimeInput{
		GroupID:       17,
		BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
	}, func(error) bool { return true })
	require.NoError(t, err)

	// When
	require.NoError(t, runtime.AcceptTask())

	// Then
	require.False(t, runtime.CanRetry(errors.New("retryable")))
}

func TestCanonicalGatewayRuntimePreservesAcceptedQuoteSnapshot(t *testing.T) {
	inputPrice := 0.0001
	outputPrice := 0.0002
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 61, PoolID: 7, EndpointID: 8,
		ModelName: "published-model", EndpointType: SharedPoolEndpointResponses,
		PricingSource: SharedPoolPricingSourceOwner, BasePrice: SharedPoolPriceComponents{
			BillingMode: "token", Currency: "USD",
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		},
		Multiplier: 1,
	}
	FinalizeSharedPoolPriceQuote(quote)

	runtime, err := NewCanonicalGatewayRuntime(CanonicalGatewayRuntimeInput{
		GroupID: 1, AccountID: 2, BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
		AcceptedQuote: quote,
	}, func(error) bool { return false })
	require.NoError(t, err)

	got := runtime.AcceptedQuote()
	require.NotNil(t, got)
	require.Equal(t, quote.PriceVersionID, got.PriceVersionID)
	got.PriceVersionID = 999
	require.Equal(t, int64(61), runtime.AcceptedQuote().PriceVersionID, "callers must not mutate the runtime snapshot")
}
