package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sharedPoolResponsesGateRepoStub struct {
	BizDecipherRepository
	sharedPoolPricingRepository
	gateStatus      string
	gateStale       bool
	mediaQuote      *SharedPoolPriceQuote
	textQuote       *SharedPoolPriceQuote
	mediaRouteCalls int
	textRouteCalls  int
	mediaEndpoint   string
}

func (r *sharedPoolResponsesGateRepoStub) GetSharedPoolAccessKeyByAPIKeyID(_ context.Context, _ int64, _ string) (*SharedPoolAccessKey, error) {
	r.textRouteCalls++
	return sharedPoolResponsesGateAccessKey(), nil
}

func (r *sharedPoolResponsesGateRepoStub) GetSharedPoolMediaAccessKeyByAPIKeyID(_ context.Context, _ int64, _ string, endpointType string) (*SharedPoolAccessKey, error) {
	r.mediaRouteCalls++
	r.mediaEndpoint = endpointType
	if r.gateStatus != "passed" || r.gateStale {
		return nil, nil
	}
	return sharedPoolResponsesGateAccessKey(), nil
}

func (r *sharedPoolResponsesGateRepoStub) ResolveSharedPoolPriceQuote(_ context.Context, _ int64, _ string, endpointType string) (*SharedPoolPriceQuote, error) {
	var quote *SharedPoolPriceQuote
	switch endpointType {
	case SharedPoolEndpointResponses, SharedPoolEndpointChat:
		quote = r.textQuote
	case SharedPoolEndpointImageGeneration:
		quote = r.mediaQuote
	}
	if quote == nil {
		return nil, ErrSharedPoolPricingUnavailable
	}
	copy := *quote
	return &copy, nil
}

func TestSharedPoolPlainResponsesDoesNotWaitForMediaGate(t *testing.T) {
	repo, gateway := newSharedPoolResponsesGateGateway("failed", false, "per_request")
	body := []byte(`{"model":"published-model","input":"hello","tools":[{"type":"function","name":"Read"}]}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, SharedPoolEndpointResponses, endpointType)
	require.Equal(t, 1, repo.textRouteCalls)
	require.Zero(t, repo.mediaRouteCalls)
}

func TestSharedPoolPassiveImageNamespaceDoesNotWaitForMediaGate(t *testing.T) {
	repo, gateway := newSharedPoolResponsesGateGateway("failed", false, "per_request")
	body := []byte(`{
		"model":"published-model",
		"input":"hello",
		"tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],
		"tool_choice":"auto"
	}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, SharedPoolEndpointResponses, endpointType)
	require.Equal(t, 1, repo.textRouteCalls)
	require.Zero(t, repo.mediaRouteCalls)
}

func TestSharedPoolResponsesContinuationUsesTextRoute(t *testing.T) {
	repo, gateway := newSharedPoolResponsesGateGateway("failed", false, "per_request")
	body := []byte(`{
		"model":"published-model",
		"previous_response_id":"resp_123",
		"input":"continue the implementation",
		"tools":[{"type":"function","name":"Read"}]
	}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, SharedPoolEndpointResponses, endpointType)
	require.Equal(t, 1, repo.textRouteCalls)
	require.Zero(t, repo.mediaRouteCalls)
}

func TestSharedPoolResponsesImageGenerationRequiresCurrentMediaGate(t *testing.T) {
	tests := []struct {
		name       string
		gateStatus string
		gateStale  bool
	}{
		{name: "failed gate", gateStatus: "failed"},
		{name: "unverified gate", gateStatus: "unverified"},
		{name: "stale gate", gateStatus: "passed", gateStale: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, gateway := newSharedPoolResponsesGateGateway(tt.gateStatus, tt.gateStale, "per_request")
			body := []byte(`{
				"model":"published-model",
				"input":"draw a fox",
				"verification_mode":"professional_review",
				"tools":[{"type":"image_generation"}]
			}`)

			accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
				context.Background(), 17, "published-model", true, body,
			)

			require.Nil(t, accessKey)
			require.Equal(t, SharedPoolEndpointImageGeneration, endpointType)
			require.ErrorIs(t, err, ErrSharedPoolPricingNotConfigured)
			require.Equal(t, 1, repo.mediaRouteCalls)
			require.Zero(t, repo.textRouteCalls, "professional review must not fall back to the text route")
			require.Equal(t, SharedPoolEndpointImageGeneration, repo.mediaEndpoint)
		})
	}
}

func TestSharedPoolResponsesImageGenerationRequiresMediaPrice(t *testing.T) {
	repo, gateway := newSharedPoolResponsesGateGateway("passed", false, "per_request")
	repo.mediaQuote = nil
	body := []byte(`{"model":"published-model","input":"draw","tools":[{"type":"image_generation"}]}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.Nil(t, accessKey)
	require.Equal(t, SharedPoolEndpointImageGeneration, endpointType)
	require.ErrorIs(t, err, ErrSharedPoolPricingNotConfigured)
	require.Equal(t, 1, repo.mediaRouteCalls)
	require.Zero(t, repo.textRouteCalls)
}

func TestSharedPoolResponsesImageGenerationRejectsTokenPrice(t *testing.T) {
	repo, gateway := newSharedPoolResponsesGateGateway("passed", false, "token")
	body := []byte(`{"model":"published-model","input":"draw","tools":[{"type":"image_generation"}]}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.Nil(t, accessKey)
	require.Equal(t, SharedPoolEndpointImageGeneration, endpointType)
	require.ErrorIs(t, err, ErrSharedPoolPricingNotConfigured)
	require.Equal(t, 1, repo.mediaRouteCalls)
	require.Zero(t, repo.textRouteCalls)
}

func TestSharedPoolResponsesImageGenerationAcceptsPassedPerRequestMediaPrice(t *testing.T) {
	repo, gateway := newSharedPoolResponsesGateGateway("passed", false, "per_request")
	body := []byte(`{"model":"published-model","input":"draw","tools":[{"type":"image_generation"}]}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.NoError(t, err)
	require.NotNil(t, accessKey)
	require.Equal(t, SharedPoolEndpointImageGeneration, endpointType)
	require.Equal(t, "per_request", accessKey.PriceQuote.BasePrice.BillingMode)
	require.Equal(t, 1, repo.mediaRouteCalls)
	require.Zero(t, repo.textRouteCalls)
}

func TestSharedPoolResponsesImageGenerationRejectsUnboundedPerImagePrice(t *testing.T) {
	_, gateway := newSharedPoolResponsesGateGateway("passed", false, "image")
	body := []byte(`{"model":"published-model","input":"draw","tools":[{"type":"image_generation"}]}`)

	accessKey, endpointType, err := gateway.GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
		context.Background(), 17, "published-model", true, body,
	)

	require.Nil(t, accessKey)
	require.Equal(t, SharedPoolEndpointImageGeneration, endpointType)
	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func newSharedPoolResponsesGateGateway(gateStatus string, gateStale bool, mediaBillingMode string) (*sharedPoolResponsesGateRepoStub, *OpenAIGatewayService) {
	repo := &sharedPoolResponsesGateRepoStub{
		gateStatus: gateStatus,
		gateStale:  gateStale,
		textQuote:  sharedPoolResponsesGateQuote(SharedPoolEndpointResponses, "token"),
		mediaQuote: sharedPoolResponsesGateQuote(SharedPoolEndpointImageGeneration, mediaBillingMode),
	}
	biz := NewBizDecipherService(repo, nil, nil)
	return repo, &OpenAIGatewayService{bizDecipherService: biz}
}

func sharedPoolResponsesGateAccessKey() *SharedPoolAccessKey {
	return &SharedPoolAccessKey{
		ID: 11, PoolID: 9, UserID: 7, PublishedModelName: "published-model", RateMultiplier: 1,
	}
}

func sharedPoolResponsesGateQuote(endpointType, billingMode string) *SharedPoolPriceQuote {
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 71,
		PoolID:         9,
		PoolModelID:    12,
		EndpointID:     42,
		ModelName:      "published-model",
		EndpointType:   endpointType,
		PricingSource:  SharedPoolPricingSourceOwner,
		PricingStatus:  "ready",
		ConfigVersion:  3,
		Multiplier:     1,
		EffectiveFrom:  time.Now().UTC(),
		BasePrice: SharedPoolPriceComponents{
			BillingMode: billingMode,
			Currency:    "USD",
		},
	}
	price := 0.02
	switch billingMode {
	case "token":
		quote.BasePrice.InputPrice = &price
		quote.BasePrice.OutputPrice = &price
	case "per_request":
		quote.BasePrice.PerRequestPrice = &price
	case "image":
		quote.BasePrice.ImageItemPrice = &price
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote
}
