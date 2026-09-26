package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type sharedPoolCompactLookupErrorRepo struct {
	service.BizDecipherRepository
	err              error
	regularAccessKey *service.SharedPoolAccessKey
}

func (r *sharedPoolCompactLookupErrorRepo) GetSharedPoolCompactAccessKeyByAPIKeyID(context.Context, int64, string) (*service.SharedPoolAccessKey, error) {
	return nil, r.err
}

func (r *sharedPoolCompactLookupErrorRepo) GetSharedPoolAccessKeyByAPIKeyID(context.Context, int64, string) (*service.SharedPoolAccessKey, error) {
	return r.regularAccessKey, nil
}

func TestExactSharedPoolCompactPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/responses/compact", "/responses/compact", "/backend-api/codex/responses/compact"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, path+"?trace=1", nil)
		require.True(t, isExactSharedPoolCompactPath(c), path)
	}
	for _, path := range []string{"/v1/responses", "/v1/responses/compact/", "/v1/responses/compact/detail", "/openai/v1/responses/compact"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, path, nil)
		require.False(t, isExactSharedPoolCompactPath(c), path)
	}
}

func TestSharedPoolBodySignalCompactUsesCapabilityGatedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"input":[{"type":"compaction_trigger"}]}`)

	require.True(t, isSharedPoolCompactRequest(c, body))
	require.False(t, isSharedPoolCompactRequest(c, []byte(`{"input":[{"type":"message"}]}`)))
}

func TestSharedPoolCompactCapabilityLookupFailureReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wantErr := errors.New("capability store unavailable")
	bizService := service.NewBizDecipherService(
		&sharedPoolCompactLookupErrorRepo{err: wantErr},
		nil,
		nil,
	)
	gatewayService := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, bizService, nil,
	)
	h := &OpenAIGatewayHandler{gatewayService: gatewayService}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	h.handleSharedPoolResponses(
		c,
		zap.NewNop(),
		&service.APIKey{ID: 41, SharedPoolManaged: true},
		"published-gpt-5.6",
		[]byte(`{"model":"published-gpt-5.6","input":"hello"}`),
	)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compact_capability_unavailable")
	require.Contains(t, recorder.Body.String(), "No upstream request was sent and no charge was made")
}

func TestSharedPoolRemoteCompactionV2BodySignalUsesCapabilityGatedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wantErr := errors.New("capability store unavailable")
	bizService := service.NewBizDecipherService(
		&sharedPoolCompactLookupErrorRepo{err: wantErr},
		nil,
		nil,
	)
	gatewayService := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, bizService, nil,
	)
	h := &OpenAIGatewayHandler{gatewayService: gatewayService}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"published-gpt-5.6","stream":true,"input":[{"type":"compaction_trigger"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

	normalized, ok := h.normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	require.True(t, ok)
	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.True(t, isSharedPoolCompactRequest(c, normalized))

	h.handleSharedPoolResponses(
		c,
		zap.NewNop(),
		&service.APIKey{ID: 41, SharedPoolManaged: true},
		"published-gpt-5.6",
		normalized,
	)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compact_capability_unavailable")
	require.NotContains(t, recorder.Body.String(), "only on the explicit /responses/compact endpoint")
}

func TestSharedPoolCompactUnsupportedAPIKeyWinsBeforeMissingPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &sharedPoolCompactLookupErrorRepo{regularAccessKey: &service.SharedPoolAccessKey{
		ID:                 31,
		PoolID:             9,
		UserID:             7,
		AccountID:          11,
		AuthType:           service.AccountTypeAPIKey,
		UpstreamBaseURL:    "https://example.invalid/v1",
		UpstreamAPIKey:     "sk-test",
		PublishedModelName: "published-gpt-5.6",
		UpstreamModelName:  "gpt-5.6-codex",
	}}
	bizService := service.NewBizDecipherService(repo, nil, nil)
	gatewayService := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, bizService, nil,
	)
	h := &OpenAIGatewayHandler{gatewayService: gatewayService}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	h.handleSharedPoolResponses(
		c,
		zap.NewNop(),
		&service.APIKey{ID: 41, SharedPoolManaged: true},
		"published-gpt-5.6",
		[]byte(`{"model":"published-gpt-5.6","input":"hello"}`),
	)

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compact_not_supported")
	require.NotContains(t, recorder.Body.String(), "pricing_not_configured")
}

func TestSettleFailedSharedPoolForwardPreservesPartialUsage(t *testing.T) {
	partial := &service.OpenAIForwardResult{}
	partial.Usage.InputTokens = 17
	partial.Usage.OutputTokens = 3

	var recorded *service.OpenAIForwardResult
	attempts, outcome, err := settleFailedSharedPoolForward(
		partial,
		"upstream_partial_response_result_unknown",
		func(result *service.OpenAIForwardResult) (int, error) {
			recorded = result
			return 1, nil
		},
		func(string) (int, error) {
			t.Fatal("verified partial usage must settle without entering review")
			return 0, nil
		},
	)

	require.NoError(t, err)
	require.Same(t, partial, recorded, "partial usage must reach settlement instead of being replaced with nil")
	require.Equal(t, 1, attempts)
	require.Equal(t, service.SharedPoolSettlementSettled, outcome)
}

func TestSettleFailedSharedPoolForwardMovesEmptyResultToReview(t *testing.T) {
	reason := "upstream_timeout_result_unknown"
	var reviewedReason string
	attempts, outcome, err := settleFailedSharedPoolForward(
		nil,
		reason,
		func(*service.OpenAIForwardResult) (int, error) {
			t.Fatal("an unknown forward result must never be recorded as a free failure")
			return 0, nil
		},
		func(value string) (int, error) {
			reviewedReason = value
			return 1, nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, attempts)
	require.Equal(t, service.SharedPoolSettlementUnknown, outcome)
	require.Equal(t, reason, reviewedReason)
}

func TestSettleFailedSharedPoolForwardReportsSettlementFailure(t *testing.T) {
	wantErr := errors.New("ledger unavailable")
	reviewErr := errors.New("review queue unavailable")
	attempts, outcome, err := settleFailedSharedPoolForward(
		&service.OpenAIForwardResult{},
		"upstream_result_unknown",
		func(*service.OpenAIForwardResult) (int, error) { return 3, wantErr },
		func(string) (int, error) { return 2, reviewErr },
	)

	require.ErrorIs(t, err, wantErr)
	require.ErrorIs(t, err, reviewErr)
	require.Equal(t, 5, attempts)
	require.Equal(t, service.SharedPoolSettlementFailed, outcome)
}

func TestSettleFailedSharedPoolForwardMovesUnbillablePartialResultToReview(t *testing.T) {
	partial := &service.OpenAIForwardResult{}
	reviewed := false
	attempts, outcome, err := settleFailedSharedPoolForward(
		partial,
		"upstream_partial_response_result_unknown",
		func(*service.OpenAIForwardResult) (int, error) {
			return 1, service.ErrSharedPoolUsageUnavailable
		},
		func(string) (int, error) {
			reviewed = true
			return 1, nil
		},
	)

	require.NoError(t, err)
	require.True(t, reviewed)
	require.Equal(t, 2, attempts)
	require.Equal(t, service.SharedPoolSettlementUnknown, outcome)
}

func TestSharedPoolUnknownForwardReason(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		partialResponse bool
		want            string
	}{
		{name: "timeout", err: context.DeadlineExceeded, want: "upstream_timeout_result_unknown"},
		{name: "connection reset", err: errors.New("read tcp: connection reset by peer"), want: "upstream_connection_lost_result_unknown"},
		{name: "partial response wins", err: errors.New("unexpected EOF"), partialResponse: true, want: "upstream_partial_response_result_unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, sharedPoolUnknownForwardReason(tt.err, tt.partialResponse))
		})
	}
}

func TestSharedPoolVerifiedHTTPFailureStatusSeparatesTerminalFailureFromUnknownTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.Equal(t, http.StatusBadGateway, sharedPoolVerifiedHTTPFailureStatus(
		c,
		&service.UpstreamFailoverError{StatusCode: http.StatusBadGateway},
		false,
	))
	c.Set(service.OpsUpstreamStatusCodeKey, http.StatusForbidden)
	require.Equal(t, http.StatusForbidden, sharedPoolVerifiedHTTPFailureStatus(c, errors.New("upstream rejected request"), false))
	require.Zero(t, sharedPoolVerifiedHTTPFailureStatus(c, errors.New("partial stream"), true))

	unknownContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Zero(t, sharedPoolVerifiedHTTPFailureStatus(unknownContext, context.DeadlineExceeded, false))
	require.Zero(t, sharedPoolVerifiedHTTPFailureStatus(unknownContext, errors.New("connection reset by peer"), false))
	require.Equal(t, "upstream_http_502_no_success", sharedPoolVerifiedHTTPFailureReason(http.StatusBadGateway))
}

func TestShouldFailoverSharedPoolRouteOnlyBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	makeContext := func() (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		return c, recorder
	}

	c, _ := makeContext()
	retryable := &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway}
	require.True(t, shouldFailoverSharedPoolRoute(c, c.Writer.Size(), retryable, http.StatusBadGateway))

	c, _ = makeContext()
	c.String(http.StatusBadGateway, "already sent")
	require.False(t, shouldFailoverSharedPoolRoute(c, -1, retryable, http.StatusBadGateway))

	c, _ = makeContext()
	require.False(t, shouldFailoverSharedPoolRoute(c, c.Writer.Size(), retryable, http.StatusBadRequest))

	c, _ = makeContext()
	ctx := service.WithSharedPoolRouteExcluded(c.Request.Context(), &service.SharedPoolAccessKey{ID: 1, AccountID: 11})
	ctx = service.WithSharedPoolRouteExcluded(ctx, &service.SharedPoolAccessKey{ID: 2, AccountID: 12})
	c.Request = c.Request.WithContext(ctx)
	require.False(t, shouldFailoverSharedPoolRoute(c, c.Writer.Size(), retryable, http.StatusBadGateway))
}
