package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPrepareSharedPoolBoundedRequestBodyInjectsDefaultLimit(t *testing.T) {
	body, maxOutput, unknown, err := prepareSharedPoolBoundedRequestBody(
		[]byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`),
		SharedPoolEndpointChat,
	)
	require.NoError(t, err)
	require.Equal(t, sharedPoolDefaultMaxOutputTokens, maxOutput)
	require.False(t, unknown)
	require.Equal(t, sharedPoolDefaultMaxOutputTokens, gjson.GetBytes(body, "max_tokens").Int())
}

func TestPrepareSharedPoolBoundedRequestBodyCompactPreservesWireBody(t *testing.T) {
	original := []byte("{\n  \"model\": \"gpt-test\",\n  \"input\": \"hello\"\n}\n")

	body, maxOutput, unknown, err := prepareSharedPoolBoundedRequestBodyWithCompact(
		original,
		SharedPoolEndpointResponses,
		true,
	)

	require.NoError(t, err)
	require.Equal(t, original, body)
	require.Equal(t, sharedPoolMaximumOutputTokens, maxOutput)
	require.False(t, unknown)
	require.False(t, gjson.GetBytes(body, "max_output_tokens").Exists())
}

func TestPrepareSharedPoolBoundedRequestBodyCompactAllowsPassiveHostedToolCatalog(t *testing.T) {
	original := []byte(`{"model":"gpt-test","input":"hello","tools":[{"type":"image_generation"},{"type":"mcp","server_label":"docs"}],"tool_choice":"image_generation"}`)

	body, maxOutput, unknown, err := prepareSharedPoolBoundedRequestBodyWithCompact(
		original,
		SharedPoolEndpointResponses,
		true,
	)

	require.NoError(t, err)
	require.Equal(t, original, body)
	require.Equal(t, sharedPoolMaximumOutputTokens, maxOutput)
	require.False(t, unknown, "top-level compact tool declarations are passive history, not an invocation")
}

func TestPrepareSharedPoolBoundedRequestBodyCompactRejectsNonResponsesEndpoint(t *testing.T) {
	_, _, _, err := prepareSharedPoolBoundedRequestBodyWithCompact(
		[]byte(`{"model":"gpt-test","messages":[]}`),
		SharedPoolEndpointChat,
		true,
	)

	require.ErrorContains(t, err, "require the responses endpoint")
}

func TestPrepareSharedPoolBoundedRequestBodyRejectsExcessiveLimit(t *testing.T) {
	_, _, _, err := prepareSharedPoolBoundedRequestBody(
		[]byte(`{"model":"gpt-test","max_output_tokens":40000,"input":"hello"}`),
		SharedPoolEndpointResponses,
	)
	require.ErrorIs(t, err, ErrSharedPoolOutputLimitTooHigh)
}

func TestPrepareSharedPoolBoundedRequestBodyRejectsAmbiguousOrFractionalLimits(t *testing.T) {
	_, _, _, err := prepareSharedPoolBoundedRequestBody(
		[]byte(`{"model":"gpt-test","max_tokens":100,"max_completion_tokens":200,"messages":[]}`),
		SharedPoolEndpointChat,
	)
	require.ErrorContains(t, err, "conflicting output limits")

	_, _, _, err = prepareSharedPoolBoundedRequestBody(
		[]byte(`{"model":"gpt-test","max_output_tokens":10.5,"input":"hello"}`),
		SharedPoolEndpointResponses,
	)
	require.ErrorContains(t, err, "positive integer")
}

func TestPrepareSharedPoolBoundedRequestBodyRejectsUnsupportedServiceTier(t *testing.T) {
	_, _, _, err := prepareSharedPoolBoundedRequestBody(
		[]byte(`{"model":"gpt-test","service_tier":"priority","max_output_tokens":100,"input":"hello"}`),
		SharedPoolEndpointResponses,
	)
	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func TestCalculateSharedPoolMaximumHoldBoundsResponsesContinuationContext(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"previous_response_id":"resp_1"}`)
	hold, err := calculateSharedPoolMaximumHold(quote, body, 100, false)
	require.NoError(t, err)

	want := (float64(sharedPoolContinuationTokenCeiling)*2e-6 + 100*8e-6) * quote.Multiplier
	require.InDelta(t, normalizedSharedPoolMoney(want), hold, 1e-12)
}

func TestCalculateSharedPoolMaximumHoldUsesHighestInputCategory(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	cacheWrite := 10e-6
	quote.BasePrice.CacheWritePrice = &cacheWrite
	body := []byte(`{"input":"hello","max_output_tokens":100}`)

	hold, err := calculateSharedPoolMaximumHold(quote, body, 100, false)
	require.NoError(t, err)

	maxInputTokens := int64(len(body)) + sharedPoolInputTokenOverhead
	want := (float64(maxInputTokens)*cacheWrite + 100*8e-6) * quote.Multiplier
	require.InDelta(t, normalizedSharedPoolMoney(want), hold, 1e-12)
}

func TestCalculateSharedPoolMaximumHoldIncludesLongContextPremium(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	quote.LongContextInputThreshold = 10
	quote.LongContextInputMultiplier = 2
	quote.LongContextOutputMultiplier = 1.5
	body := []byte(`{"input":"hello","max_output_tokens":100}`)

	hold, err := calculateSharedPoolMaximumHold(quote, body, 100, false)
	require.NoError(t, err)
	maxInputTokens := int64(len(body)) + sharedPoolInputTokenOverhead
	want := (float64(maxInputTokens)*2e-6*2 + 100*8e-6*1.5) * quote.Multiplier
	require.InDelta(t, normalizedSharedPoolMoney(want), hold, 1e-12)
}

func TestCalculateSharedPoolMaximumHoldAllowsBoundedVisionInput(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"input_image":"https://example.invalid/a.png"}`)
	features := inspectSharedPoolRequestFeatures(gjson.ParseBytes(body))
	require.Equal(t, 1, features.visionInputCount)
	require.False(t, features.unpricedMedia)

	hold, err := calculateSharedPoolMaximumHold(quote, body, 100, false)
	require.NoError(t, err)
	maxInputTokens := int64(len(body)) + sharedPoolInputTokenOverhead + sharedPoolVisionTokensPerImage
	want := (float64(maxInputTokens)*2e-6 + 100*8e-6) * quote.Multiplier
	require.InDelta(t, normalizedSharedPoolMoney(want), hold, 1e-12)
}

func TestCalculateSharedPoolMaximumHoldAllowsUnicodeEscapedVisionKey(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"input":[{"\u0069mage_url":"https://example.invalid/a.png"}]}`)
	hold, err := calculateSharedPoolMaximumHold(quote, body, 100, false)
	require.NoError(t, err)
	require.Positive(t, hold)
}

func TestCalculateSharedPoolMaximumHoldRejectsTokenPricedMediaModelWithoutMediaFields(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	quote.ModelName = "gpt-image-1"
	_, err := calculateSharedPoolMaximumHold(quote, []byte(`{"input":"draw a fox"}`), 100, false)
	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func TestCalculateSharedPoolMaximumHoldRejectsHostedToolsEvenWithCap(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	maximum := 1.0
	quote.BasePrice.MaximumCharge = &maximum
	for _, toolType := range []string{"web_search_preview", "file_search", "code_interpreter", "computer_use_preview", "mcp"} {
		t.Run(toolType, func(t *testing.T) {
			body := []byte(`{"tools":[{"type":"` + toolType + `"}]}`)
			_, err := calculateSharedPoolMaximumHold(quote, body, 100, true)
			require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
		})
	}
}

func TestCalculateSharedPoolMaximumHoldCompactAllowsPassiveHostedToolCatalog(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"input":"compact this history","tools":[{"type":"image_generation"},{"type":"web_search_preview"}],"tool_choice":"image_generation"}`)

	hold, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.NoError(t, err)
	require.Positive(t, hold)
}

func TestCalculateSharedPoolMaximumHoldCompactAllowsCompletedHostedToolHistory(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"input":[{"type":"web_search_call","status":"completed"},{"type":"file_search_call","status":"completed"},{"type":"code_interpreter_call","status":"completed"},{"type":"computer_call","status":"completed"},{"type":"mcp_call","status":"completed"}],"tools":[{"type":"web_search_preview"}]}`)

	hold, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.NoError(t, err)
	require.Positive(t, hold)
}

func TestCalculateSharedPoolMaximumHoldCompactAllowsVisionInput(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"tools":[{"type":"image_generation"}],"input":[{"type":"input_image","image_url":"https://example.invalid/a.png"}]}`)
	features := inspectSharedPoolRequestFeaturesWithCompact(gjson.ParseBytes(body), true)
	require.False(t, features.unpricedMedia)
	require.Equal(t, 1, features.visionInputCount)

	hold, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.NoError(t, err)
	require.Positive(t, hold)
}

func TestCalculateSharedPoolMaximumHoldCompactRejectsHiddenItemReference(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"tools":[{"type":"web_search_preview"}],"input":[{"type":"item_reference","id":"item_hidden"}]}`)
	features := inspectSharedPoolRequestFeaturesWithCompact(gjson.ParseBytes(body), true)
	require.True(t, features.serverManagedContext)

	_, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func TestCalculateSharedPoolMaximumHoldCompactRejectsStoredPromptReference(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"prompt":{"id":"pmpt_hidden","version":"3"},"input":"hello"}`)
	features := inspectSharedPoolRequestFeaturesWithCompact(gjson.ParseBytes(body), true)
	require.True(t, features.serverManagedContext)

	_, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func TestCalculateSharedPoolMaximumHoldCompactRejectsImageGenerationCallResult(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"input":[{"type":"image_generation_call","result":"data:image/png;base64,AAAA"}]}`)
	features := inspectSharedPoolRequestFeaturesWithCompact(gjson.ParseBytes(body), true)
	require.True(t, features.unpricedMedia)

	_, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func TestCalculateSharedPoolMaximumHoldCompactRejectsIncompleteHostedToolHistory(t *testing.T) {
	quote := sharedPoolReservationTestQuote("token")
	body := []byte(`{"input":[{"type":"web_search_call","status":"in_progress"}]}`)
	features := inspectSharedPoolRequestFeaturesWithCompact(gjson.ParseBytes(body), true)
	require.True(t, features.hostedTool)

	_, err := calculateSharedPoolMaximumHoldWithCompact(
		quote,
		body,
		sharedPoolMaximumOutputTokens,
		false,
		true,
	)

	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
}

func TestCalculateSharedPoolMaximumHoldAllowsFixedPerRequestMedia(t *testing.T) {
	quote := sharedPoolReservationTestQuote("per_request")
	hold, err := calculateSharedPoolMaximumHold(quote, []byte(`{"input_image":"https://example.invalid/a.png"}`), 100, true)
	require.NoError(t, err)
	require.InDelta(t, 0.25*quote.Multiplier, hold, 1e-12)
}

func TestSharedPoolReservationRequestIDIsStableAndScoped(t *testing.T) {
	first := sharedPoolReservationRequestID("local:req-1", 7)
	require.Equal(t, first, sharedPoolReservationRequestID("local:req-1", 7))
	require.NotEqual(t, first, sharedPoolReservationRequestID("local:req-1", 8))
	require.LessOrEqual(t, len(first), 160)
	require.False(t, errors.Is(ErrSharedPoolReservationConflict, ErrSharedPoolReservationFinalized))
}

type sharedPoolUsageRecordRepoStub struct {
	BizDecipherRepository
	recordCalls int
}

func (r *sharedPoolUsageRecordRepoStub) RecordSharedPoolUsageTx(_ context.Context, _ SharedPoolUsageInput) error {
	r.recordCalls++
	return nil
}

func TestRecordSharedPoolUsageRejectsBlankRequestIDBeforeRepository(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
	}{
		{name: "empty", requestID: ""},
		{name: "whitespace", requestID: " \t\r\n "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &sharedPoolUsageRecordRepoStub{}
			svc := NewBizDecipherService(repo, nil, nil)

			err := svc.RecordSharedPoolUsage(context.Background(), SharedPoolUsageInput{
				AccessKeyID: 11, PoolID: 22, UserID: 33, Cost: 1,
				RequestID: tt.requestID, Success: true, PriceVersionID: 44,
				PricingSource: SharedPoolPricingSourceOfficial,
			})

			require.ErrorContains(t, err, "requires a request id")
			require.Zero(t, repo.recordCalls)
		})
	}
}

func TestWithdrawableSharedPoolMoneyNeverRoundsUp(t *testing.T) {
	require.Zero(t, withdrawableSharedPoolMoney(4e-9))
	require.Equal(t, 1e-8, withdrawableSharedPoolMoney(19e-9))
	require.LessOrEqual(t, withdrawableSharedPoolMoney(1.234567899), 1.234567899)
}

func sharedPoolReservationTestQuote(mode string) *SharedPoolPriceQuote {
	input := 2e-6
	output := 8e-6
	perRequest := 0.25
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 1,
		PoolID:         2,
		PoolModelID:    3,
		EndpointID:     4,
		ModelName:      "gpt-test",
		EndpointType:   SharedPoolEndpointResponses,
		PricingSource:  SharedPoolPricingSourceOwner,
		PricingStatus:  "ready",
		Multiplier:     1.2,
		BasePrice: SharedPoolPriceComponents{
			BillingMode: mode,
			Currency:    "USD",
		},
	}
	if mode == "per_request" {
		quote.BasePrice.PerRequestPrice = &perRequest
	} else {
		quote.BasePrice.InputPrice = &input
		quote.BasePrice.OutputPrice = &output
	}
	return quote
}
