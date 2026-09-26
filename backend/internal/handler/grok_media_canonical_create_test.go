package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type canonicalMediaHandlerProbeRepo struct {
	service.AccountRepository
	service.UsageBillingRepository

	mu            sync.Mutex
	accounts      []service.Account
	claimCalls    int
	resolveCalls  int
	resolvedTask  string
	resolvedOwner int64
}

func (r *canonicalMediaHandlerProbeRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, account := range r.accounts {
		if account.ID == id {
			selected := account
			return &selected, nil
		}
	}
	return nil, service.ErrNoAvailableAccounts
}

func (r *canonicalMediaHandlerProbeRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.accountsForPlatform(platform), nil
}

func (r *canonicalMediaHandlerProbeRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return r.accountsForPlatform(platform), nil
}

func (r *canonicalMediaHandlerProbeRepo) ListSchedulableUngroupedByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return r.accountsForPlatform(platform), nil
}

func (r *canonicalMediaHandlerProbeRepo) accountsForPlatform(platform string) []service.Account {
	r.mu.Lock()
	defer r.mu.Unlock()
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform {
			accounts = append(accounts, account)
		}
	}
	return accounts
}

func (r *canonicalMediaHandlerProbeRepo) ClaimCanonicalMediaTask(_ context.Context, _ mediatask.ClaimBindingInput) (mediatask.Binding, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimCalls++
	return mediatask.Binding{}, true, nil
}

func (r *canonicalMediaHandlerProbeRepo) AcceptCanonicalMediaTask(context.Context, mediatask.CreateContext, string) (mediatask.Binding, error) {
	return mediatask.Binding{}, nil
}

func (r *canonicalMediaHandlerProbeRepo) ResolveCanonicalMediaTask(_ context.Context, _ int64, _ int64, requestID string) (mediatask.Binding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolveCalls++
	r.resolvedTask = requestID
	return mediatask.Binding{
		AccountID:      r.resolvedOwner,
		State:          mediatask.BindingAccepted,
		UpstreamTaskID: requestID,
	}, nil
}

func (r *canonicalMediaHandlerProbeRepo) calls() (int, int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.claimCalls, r.resolveCalls, r.resolvedTask
}

type canonicalMediaHandlerUpstreamCall struct {
	accountID int64
	url       string
}

type canonicalMediaHandlerProbeUpstream struct {
	service.HTTPUpstream

	mu    sync.Mutex
	calls []canonicalMediaHandlerUpstreamCall
}

func (u *canonicalMediaHandlerProbeUpstream) Do(request *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.calls = append(u.calls, canonicalMediaHandlerUpstreamCall{accountID: accountID, url: request.URL.String()})
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"accepted-task-17","status":"processing"}`)),
	}, nil
}

func (u *canonicalMediaHandlerProbeUpstream) recordedCalls() []canonicalMediaHandlerUpstreamCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]canonicalMediaHandlerUpstreamCall(nil), u.calls...)
}

func newCanonicalMediaHandlerProbe(t *testing.T) (*gin.Engine, *canonicalMediaHandlerProbeRepo, *canonicalMediaHandlerProbeUpstream) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	groupID := int64(71)
	repo := &canonicalMediaHandlerProbeRepo{
		accounts: []service.Account{{
			ID:          72,
			Platform:    service.PlatformGrok,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Credentials: map[string]any{"api_key": "test-key"},
			Extra:       map[string]any{service.GrokMediaEligibleExtraKey: true},
		}},
		resolvedOwner: 72,
	}
	upstream := &canonicalMediaHandlerProbeUpstream{}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	gateway := service.NewOpenAIGatewayService(
		repo, nil, repo, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billingCache, upstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	concurrency := service.NewConcurrencyService(&concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	})
	handler := NewOpenAIGatewayHandler(
		gateway,
		concurrency,
		billingCache,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil, nil, nil, nil, cfg,
	)
	apiKey := &service.APIKey{
		ID:      73,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, Platform: service.PlatformGrok, AllowImageGeneration: true},
		User:    &service.User{ID: 74, Status: service.StatusActive},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.POST("/v1/videos/generations", handler.GrokVideoGeneration)
	router.GET("/v1/videos/:request_id", handler.GrokVideoStatus)
	return router, repo, upstream
}

func canonicalMediaHandlerRuntime(t *testing.T) *service.CanonicalGatewayRuntime {
	t.Helper()
	runtime, err := service.NewCanonicalGatewayRuntime(service.CanonicalGatewayRuntimeInput{
		GroupID:       71,
		AccountID:     72,
		BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
	}, func(error) bool { return false })
	require.NoError(t, err)
	return runtime
}

func TestGrokMedia_CanonicalAsyncCreateFailsClosedBeforeSideEffects(t *testing.T) {
	// Given
	router, repo, upstream := newCanonicalMediaHandlerProbe(t)
	request := httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewBufferString(`{"model":"grok-imagine-video","prompt":"waves"}`))
	request = request.WithContext(service.WithCanonicalGatewayRuntime(request.Context(), canonicalMediaHandlerRuntime(t)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "client-create-17")
	recorder := httptest.NewRecorder()

	// When
	router.ServeHTTP(recorder, request)

	// Then
	claimCalls, resolveCalls, _ := repo.calls()
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "canonical_media_create_recovery_unavailable", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
	require.Zero(t, claimCalls)
	require.Zero(t, resolveCalls)
	require.Empty(t, upstream.recordedCalls())
}

func TestGrokMedia_AcceptedCanonicalStatusUsesStoredBindingWithoutCreate(t *testing.T) {
	// Given
	router, repo, upstream := newCanonicalMediaHandlerProbe(t)
	request := httptest.NewRequest(http.MethodGet, "/v1/videos/accepted-task-17", nil)
	request = request.WithContext(service.WithCanonicalGatewayRuntime(request.Context(), canonicalMediaHandlerRuntime(t)))
	recorder := httptest.NewRecorder()

	// When
	router.ServeHTTP(recorder, request)

	// Then
	claimCalls, resolveCalls, resolvedTask := repo.calls()
	calls := upstream.recordedCalls()
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Zero(t, claimCalls)
	require.Equal(t, 1, resolveCalls)
	require.Equal(t, "accepted-task-17", resolvedTask)
	require.Len(t, calls, 1)
	require.Equal(t, int64(72), calls[0].accountID)
	require.True(t, strings.HasSuffix(calls[0].url, "/v1/videos/accepted-task-17"))
}
