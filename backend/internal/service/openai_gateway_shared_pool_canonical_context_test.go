package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/stretchr/testify/require"
)

type canonicalGatewayContextRepo struct {
	BizDecipherRepository
	quote    *SharedPoolPriceQuote
	identity *SharedPoolCanonicalIdentity
	query    SharedPoolIdentityQuery
}

func (r *canonicalGatewayContextRepo) GetCanonicalSharedPoolAccessKeyByAPIKeyID(context.Context, int64, string) (*SharedPoolAccessKey, error) {
	ownerID := int64(17)
	return &SharedPoolAccessKey{
		ID: 31, APIKeyID: 41, PoolID: 11, UserID: 19, OwnerID: &ownerID,
		PublishedModelName: "gpt-canonical", RateMultiplier: 1,
	}, nil
}

func (r *canonicalGatewayContextRepo) ResolveSharedPoolPriceQuote(context.Context, int64, string, string) (*SharedPoolPriceQuote, error) {
	return r.quote, nil
}

func (r *canonicalGatewayContextRepo) ListSharedPoolModelEndpointPricing(context.Context, int64, int64) ([]SharedPoolModelEndpointPricing, error) {
	return nil, nil
}

func (r *canonicalGatewayContextRepo) SaveSharedPoolCustomPriceVersion(context.Context, SaveSharedPoolCustomPriceInput) (*SharedPoolPriceQuote, error) {
	return nil, nil
}

func (r *canonicalGatewayContextRepo) EnsureSharedPoolOfficialPriceVersion(context.Context, int64, string, string, SharedPoolPriceComponents) (*SharedPoolPriceQuote, error) {
	return nil, nil
}

func (r *canonicalGatewayContextRepo) ResolveSharedPoolCanonicalIdentity(_ context.Context, query SharedPoolIdentityQuery) (*SharedPoolCanonicalIdentity, error) {
	r.query = query
	return r.identity, nil
}

func TestResolveSharedPoolCanonicalGatewayContextBindsLedgerPolicyAndQuote(t *testing.T) {
	inputPrice := 0.0001
	outputPrice := 0.0002
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 51, PoolID: 11, PoolModelID: 52, EndpointID: 53,
		ModelName: "gpt-canonical", EndpointType: SharedPoolEndpointChat,
		PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "ready",
		ConfigVersion: 1, BasePrice: SharedPoolPriceComponents{
			BillingMode: "token", Currency: "USD", InputPrice: &inputPrice, OutputPrice: &outputPrice,
		},
		Multiplier: 1, EffectiveFrom: time.Unix(1_700_000_000, 0).UTC(),
	}
	FinalizeSharedPoolPriceQuote(quote)
	repo := &canonicalGatewayContextRepo{
		quote: quote,
		identity: &SharedPoolCanonicalIdentity{
			PoolID: 11, OwnerID: 17, GroupID: CanonicalGroupID(23), Lifecycle: "active",
		},
	}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	resolved, err := gateway.ResolveSharedPoolCanonicalGatewayContext(
		context.Background(), 41, "gpt-canonical", false,
		[]byte(`{"model":"gpt-canonical"}`), false,
	)

	require.NoError(t, err)
	require.Equal(t, CanonicalGroupID(23), resolved.Identity.GroupID)
	require.Nil(t, repo.query.Supply, "native text routing must leave account selection to Sub2")
	require.Equal(t, corecontracts.BillingPolicyBizDecipherLedger, resolved.BillingPolicy)
	require.Equal(t, quote.PriceHash, resolved.AcceptedQuote.PriceHash)
}
