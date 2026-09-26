package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sharedPoolCompactSelectionRepoStub struct {
	BizDecipherRepository
	sharedPoolPricingRepository
	compactAccessKey *SharedPoolAccessKey
	regularAccessKey *SharedPoolAccessKey
	compactErr       error
	compactCalls     int
	regularCalls     int
	priceCalls       int
}

func (r *sharedPoolCompactSelectionRepoStub) GetSharedPoolCompactAccessKeyByAPIKeyID(_ context.Context, _ int64, _ string) (*SharedPoolAccessKey, error) {
	r.compactCalls++
	return cloneSharedPoolCompactSelectionAccessKey(r.compactAccessKey), r.compactErr
}

func (r *sharedPoolCompactSelectionRepoStub) GetSharedPoolAccessKeyByAPIKeyID(_ context.Context, _ int64, _ string) (*SharedPoolAccessKey, error) {
	r.regularCalls++
	return cloneSharedPoolCompactSelectionAccessKey(r.regularAccessKey), nil
}

func (r *sharedPoolCompactSelectionRepoStub) ResolveSharedPoolPriceQuote(_ context.Context, poolID int64, modelName, endpointType string) (*SharedPoolPriceQuote, error) {
	r.priceCalls++
	price := 0.01
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 91,
		PoolID:         poolID,
		PoolModelID:    92,
		EndpointID:     93,
		ModelName:      modelName,
		EndpointType:   endpointType,
		PricingSource:  SharedPoolPricingSourceOwner,
		PricingStatus:  "ready",
		ConfigVersion:  7,
		Multiplier:     1,
		EffectiveFrom:  time.Now().UTC(),
		BasePrice: SharedPoolPriceComponents{
			BillingMode: "token",
			Currency:    "USD",
			InputPrice:  &price,
			OutputPrice: &price,
		},
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func TestGetSharedPoolOpenAIRequestQuoteWithCompactPrefersCapableOAuthRoute(t *testing.T) {
	repo := &sharedPoolCompactSelectionRepoStub{
		compactAccessKey: sharedPoolCompactSelectionAccessKey(22, AccountTypeOAuth),
		regularAccessKey: sharedPoolCompactSelectionAccessKey(11, AccountTypeAPIKey),
	}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact(
		context.Background(), 41, "published-gpt-5.6", true,
		[]byte(`{"model":"published-gpt-5.6","tools":[{"type":"image_generation"}]}`), true,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, int64(22), accessKey.AccountID)
	require.Equal(t, SharedPoolEndpointResponses, endpointType)
	require.NotNil(t, accessKey.PriceQuote)
	require.Equal(t, 1, repo.compactCalls)
	require.Zero(t, repo.regularCalls)
	require.Equal(t, 1, repo.priceCalls)
}

func TestGetSharedPoolOpenAIRequestQuoteWithCompactFallsBackForStableRejection(t *testing.T) {
	repo := &sharedPoolCompactSelectionRepoStub{
		regularAccessKey: sharedPoolCompactSelectionAccessKey(11, AccountTypeAPIKey),
	}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact(
		context.Background(), 41, "published-gpt-5.6", true, nil, true,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, int64(11), accessKey.AccountID)
	require.Equal(t, AccountTypeAPIKey, accessKey.AuthType)
	require.Equal(t, SharedPoolEndpointResponses, endpointType)
	require.Nil(t, accessKey.PriceQuote, "an unsupported fallback must be rejected before pricing")
	require.Equal(t, 1, repo.compactCalls)
	require.Equal(t, 1, repo.regularCalls)
	require.Zero(t, repo.priceCalls, "missing fallback pricing must not mask compact_not_supported")
	require.ErrorIs(t, gateway.ValidateSharedPoolCompactAccess(context.Background(), accessKey), ErrSharedPoolCompactNotSupported)
}

func TestGetSharedPoolOpenAIRequestQuoteWithCompactDoesNotHideCapabilityStoreError(t *testing.T) {
	repo := &sharedPoolCompactSelectionRepoStub{
		compactErr:       context.DeadlineExceeded,
		regularAccessKey: sharedPoolCompactSelectionAccessKey(11, AccountTypeAPIKey),
	}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	accessKey, _, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact(
		context.Background(), 41, "published-gpt-5.6", true, nil, true,
	)

	require.Nil(t, accessKey)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, repo.compactCalls)
	require.Zero(t, repo.regularCalls)
	require.Zero(t, repo.priceCalls)
}

func TestGetSharedPoolOpenAIRequestQuoteLegacyMethodKeepsOrdinaryScheduling(t *testing.T) {
	repo := &sharedPoolCompactSelectionRepoStub{
		compactAccessKey: sharedPoolCompactSelectionAccessKey(22, AccountTypeOAuth),
		regularAccessKey: sharedPoolCompactSelectionAccessKey(11, AccountTypeAPIKey),
	}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	accessKey, _, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 41, "published-gpt-5.6", true, nil,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, int64(11), accessKey.AccountID)
	require.Zero(t, repo.compactCalls)
	require.Equal(t, 1, repo.regularCalls)
	require.Equal(t, 1, repo.priceCalls)
}

func sharedPoolCompactSelectionAccessKey(accountID int64, authType string) *SharedPoolAccessKey {
	return &SharedPoolAccessKey{
		ID:                 31,
		PoolID:             9,
		UserID:             7,
		APIKeyID:           41,
		AccountID:          accountID,
		AuthType:           authType,
		PublishedModelName: "published-gpt-5.6",
		UpstreamModelName:  "gpt-5.6-codex",
		RateMultiplier:     1,
	}
}

func cloneSharedPoolCompactSelectionAccessKey(input *SharedPoolAccessKey) *SharedPoolAccessKey {
	if input == nil {
		return nil
	}
	copy := *input
	return &copy
}
