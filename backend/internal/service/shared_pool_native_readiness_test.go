package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type readinessRepoFake struct {
	update    NativeReadinessUpdate
	onPersist func()
}

func (f *readinessRepoFake) PersistNativeReadiness(_ context.Context, _ SharedPoolNativeReadinessInput, u NativeReadinessUpdate) error {
	f.update = u
	if f.onPersist != nil {
		f.onPersist()
	}
	return nil
}

type readinessModelsFake struct{}

func (readinessModelsFake) DiscoverNativeModels(context.Context, *Account) ([]string, error) {
	return []string{"model-a"}, nil
}

type readinessConnFake struct{}

func (readinessConnFake) VerifyNativeConnection(context.Context, *Account) (NativeConnectionEvidence, error) {
	return NativeConnectionEvidence{Status: NativeConnectionAuthenticatedMetadataReachable}, nil
}

func TestSharedPoolNativeReadiness_DeletedFlagSemantics(t *testing.T) {
	binding := &SharedPoolNativeBinding{Account: SharedPoolAccount{ID: 7, PoolID: 1, OwnerID: 2}}
	base := &BizDecipherService{}
	onboarding := &nativeOnboardingRepoFake{binding: binding}
	accounts := &nativeAccountRepoFake{account: &Account{ID: 7, UpdatedAt: time.Unix(10, 0).UTC()}, deleted: false}
	readiness := &readinessRepoFake{}
	base.SetSharedPoolNativeOnboarding(onboarding, accounts)
	base.SetSharedPoolNativeReadiness(readiness, readinessModelsFake{}, readinessConnFake{})
	got, err := base.VerifySharedPoolNativeReadiness(context.Background(), SharedPoolNativeReadinessInput{PoolID: 1, OwnerID: 2, SharedAccountID: 7, ExpectedConfigVersion: 1})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, NativeConnectionAuthenticatedMetadataReachable, readiness.update.ConnectionStatus)
	require.Equal(t, []string{"model-a"}, readiness.update.Models)
	accounts.deleted = true
	_, err = base.VerifySharedPoolNativeReadiness(context.Background(), SharedPoolNativeReadinessInput{PoolID: 1, OwnerID: 2, SharedAccountID: 7, ExpectedConfigVersion: 1})
	require.ErrorIs(t, err, ErrNativeIdentityDeleted)
}

type readinessWiringRepo struct {
	SharedPoolNativeOnboardingRepository
	readinessRepoFake
}

func TestProvideBizDecipherServiceWiresNativeReadiness(t *testing.T) {
	repo := &readinessWiringRepo{}
	svc := ProvideBizDecipherService(nil, nil, nil, nil, nil, nil, repo, nil)
	require.Same(t, repo, svc.nativeReadinessRepo)
	require.NotNil(t, svc.nativeModelDiscoverer)
	require.NotNil(t, svc.nativeConnectionDiagnostic)
}

func TestSharedPoolNativeReadinessReturnsCommittedState(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "still_active", true: "revoked_stale"}[stale], func(t *testing.T) {
			binding := &SharedPoolNativeBinding{Account: SharedPoolAccount{ID: 7, PoolID: 1, OwnerID: 2}}
			onboarding := &nativeOnboardingRepoFake{binding: binding}
			readiness := &readinessRepoFake{onPersist: func() {
				binding.Account.NativeEvidenceStale = stale
				binding.Account.BillingActivationRequired = stale
				binding.Account.Schedulable = !stale
				binding.Account.NativeBindingState = "ready"
			}}
			svc := &BizDecipherService{}
			svc.SetSharedPoolNativeOnboarding(onboarding, &nativeAccountRepoFake{account: &Account{ID: 91}})
			svc.SetSharedPoolNativeReadiness(readiness, readinessModelsFake{}, readinessConnFake{})
			got, err := svc.VerifySharedPoolNativeReadiness(context.Background(), SharedPoolNativeReadinessInput{
				PoolID: 1, OwnerID: 2, SharedAccountID: 7, ExpectedConfigVersion: 1,
			})
			require.NoError(t, err)
			require.Equal(t, stale, got.NativeEvidenceStale)
			require.Equal(t, stale, got.BillingActivationRequired)
			require.Equal(t, !stale, got.Schedulable)
			require.Equal(t, "ready", got.NativeBindingState)
		})
	}
}

func TestSharedPoolNativeReadiness_ModelDiscoveryRequiresNativeClient(t *testing.T) {
	_, err := (sharedPoolNativeModelDiscoverer{}).DiscoverNativeModels(context.Background(), &Account{})
	require.ErrorIs(t, err, ErrNativeOnboardingUnavailable)
}

func TestSharedPoolNativeReadiness_ModelDiscoveryRejectsPrivateOwnerEndpoint(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowlist_disabled", true: "admin_private_allowed"}[enabled], func(t *testing.T) {
			calls := 0
			d := sharedPoolNativeModelDiscoverer{
				accountTest: &AccountTestService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
					Enabled: enabled, AllowPrivateHosts: true, AllowInsecureHTTP: true, UpstreamHosts: []string{"127.0.0.1"},
				}}}},
				client: &http.Client{Transport: ownerDiagnosticRoundTripper(func(*http.Request) (*http.Response, error) {
					calls++
					return nil, errors.New("unexpected request")
				})},
			}
			_, err := d.DiscoverNativeModels(context.Background(), &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://127.0.0.1/v1", "api_key": "test-key"}})
			require.Error(t, err)
			require.Zero(t, calls)
		})
	}
}

func TestSharedPoolNativeReadiness_ModelDiscoveryRejectsOwnerProxy(t *testing.T) {
	proxyID := int64(9)
	d := sharedPoolNativeModelDiscoverer{accountTest: &AccountTestService{}}
	_, err := d.DiscoverNativeModels(context.Background(), &Account{ProxyID: &proxyID, Credentials: map[string]any{"base_url": "https://api.example.com"}})
	require.Error(t, err)
}

type ownerDiagnosticRoundTripper func(*http.Request) (*http.Response, error)

func (f ownerDiagnosticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSharedPoolNativeReadiness_ModelDiscoveryRejectsRedirect(t *testing.T) {
	client, err := newOwnerNativeDiagnosticClient()
	require.NoError(t, err)
	calls := 0
	client.Transport = ownerDiagnosticRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://127.0.0.1/v1/models"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	d := sharedPoolNativeModelDiscoverer{accountTest: &AccountTestService{}, client: client}
	_, err = d.DiscoverNativeModels(context.Background(), &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.example.com", "api_key": "test-key"}})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestSharedPoolNativeReadiness_NativeProviderModelsAndMetadata(t *testing.T) {
	cases := []struct {
		provider, auth, endpoint, authHeader, authValue, body, model string
	}{
		{PlatformOpenAI, AccountTypeAPIKey, "https://api.openai.com/v1/models", "Authorization", "Bearer test-key", `{"data":[{"id":"gpt-test"}]}`, "gpt-test"},
		{PlatformOpenAI, AccountTypeOAuth, "https://chatgpt.com/backend-api/codex/models?client_version=" + openAICodexProbeVersion, "Authorization", "Bearer test-token", `{"models":[{"slug":"gpt-test"}]}`, "gpt-test"},
		{PlatformAnthropic, AccountTypeAPIKey, "https://api.anthropic.com/v1/models", "X-Api-Key", "test-key", `{"data":[{"id":"claude-test"}]}`, "claude-test"},
		{PlatformGemini, AccountTypeAPIKey, "https://generativelanguage.googleapis.com/v1beta/models", "X-Goog-Api-Key", "test-key", `{"models":[{"name":"models/gemini-test"}]}`, "gemini-test"},
		{PlatformGrok, AccountTypeAPIKey, "https://api.x.ai/v1/models", "Authorization", "Bearer test-key", `{"models":[{"modelId":"grok-test","name":"Display label"}]}`, "grok-test"},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"_"+tc.auth, func(t *testing.T) {
			account := &Account{ID: 91, Platform: tc.provider, Type: tc.auth, Credentials: map[string]any{
				"api_key": "test-key", "access_token": "test-token", "chatgpt_account_id": "test-account",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			}, UpdatedAt: time.Unix(10, 0).UTC()}
			calls := 0
			client := &http.Client{Transport: ownerDiagnosticRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, http.MethodGet, req.Method)
				require.Nil(t, req.Body)
				require.Equal(t, tc.authValue, req.Header.Get(tc.authHeader))
				body := tc.body
				if req.URL.String() == chatGPTUsageURL {
					require.Equal(t, "test-account", req.Header.Get("ChatGPT-Account-Id"))
					body = `{"account_id":"test-account","plan_type":"plus"}`
				} else {
					require.Equal(t, tc.endpoint, req.URL.String())
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			readiness := &readinessRepoFake{}
			svc := &BizDecipherService{}
			svc.SetSharedPoolNativeOnboarding(&nativeOnboardingRepoFake{binding: &SharedPoolNativeBinding{
				Account: SharedPoolAccount{ID: 7, PoolID: 1, OwnerID: 2, NativeBindingState: "attached"},
			}}, &nativeAccountRepoFake{account: account})
			svc.SetSharedPoolNativeReadiness(readiness, sharedPoolNativeModelDiscoverer{accountTest: &AccountTestService{}, client: client}, sharedPoolOpenAIMetadataDiagnostic{client: client})
			got, err := svc.VerifySharedPoolNativeReadiness(context.Background(), SharedPoolNativeReadinessInput{
				PoolID: 1, OwnerID: 2, SharedAccountID: 7, ExpectedConfigVersion: 1,
			})
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, []string{tc.model}, readiness.update.Models)
			require.NotNil(t, readiness.update.ModelsVerifiedAt)
			require.Equal(t, NativeConnectionAuthenticatedMetadataReachable, readiness.update.ConnectionStatus)
			require.NotNil(t, readiness.update.ConnectionVerifiedAt)
			require.Empty(t, readiness.update.ErrorCode)
			require.Equal(t, 2, calls)
		})
	}
}

func TestSharedPoolNativeReadiness_OwnerTransportBlocksResolvedPrivateIP(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	for _, host := range []string{"127.0.0.1", "localhost"} {
		t.Run(host, func(t *testing.T) {
			client, err := newOwnerNativeDiagnosticClient()
			require.NoError(t, err)
			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)
			require.Nil(t, transport.Proxy)
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+net.JoinHostPort(host, port), nil)
			require.NoError(t, err)
			response, err := client.Do(request)
			require.ErrorContains(t, err, "host is not allowed")
			require.Nil(t, response)
		})
	}
	require.Zero(t, connections.Load())
}

func TestSharedPoolNativeReadiness_ModelDiscoveryCannotProveConnection(t *testing.T) {
	base := &BizDecipherService{}
	base.SetSharedPoolNativeOnboarding(&nativeOnboardingRepoFake{binding: &SharedPoolNativeBinding{
		Account: SharedPoolAccount{ID: 7, PoolID: 1, OwnerID: 2},
	}}, &nativeAccountRepoFake{account: &Account{ID: 91}})
	readiness := &readinessRepoFake{}
	base.SetSharedPoolNativeReadiness(readiness, readinessModelsFake{}, nil)
	got, err := base.VerifySharedPoolNativeReadiness(context.Background(), SharedPoolNativeReadinessInput{
		PoolID: 1, OwnerID: 2, SharedAccountID: 7, ExpectedConfigVersion: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, []string{"model-a"}, readiness.update.Models)
	require.Equal(t, "unverified", readiness.update.ConnectionStatus)
	require.Nil(t, readiness.update.ConnectionVerifiedAt)
}

func TestSharedPoolNativeReadiness_MetadataErrorsAreSanitized(t *testing.T) {
	client := &http.Client{Transport: ownerDiagnosticRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("sensitive upstream token and response")
	})}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
	evidence, err := (sharedPoolOpenAIMetadataDiagnostic{client: client}).VerifyNativeConnection(context.Background(), account)
	require.Error(t, err)
	require.Equal(t, "failed", evidence.Status)
	require.Equal(t, "metadata_request_failed", evidence.ErrorCode)
	require.NotContains(t, evidence.ErrorMessage, "sensitive")
}

func TestSharedPoolNativeReadiness_InvalidMetadataCannotVerifyConnection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", http.StatusUnauthorized, `{"data":[{"id":"model-a"}]}`},
		{"empty_catalog", http.StatusOK, `{"data":[]}`},
		{"invalid_json", http.StatusOK, `not-json`},
		{"oversized_response", http.StatusOK, `{"data":[{"id":"model-a"}],"padding":"` + strings.Repeat("x", int(upstreamModelsBodyLimit)) + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: ownerDiagnosticRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
			evidence, err := (sharedPoolOpenAIMetadataDiagnostic{client: client}).VerifyNativeConnection(context.Background(), account)
			require.Error(t, err)
			require.Equal(t, "failed", evidence.Status)
		})
	}
}

func TestSharedPoolNativeReadiness_OpenAIModelsRejectsExpiredOrAgentAccountsBeforeTransport(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: ownerDiagnosticRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected request")
	})}
	for _, credentials := range []map[string]any{
		{"access_token": "test-token", "chatgpt_account_id": "test-account", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)},
		{"access_token": "test-token", "chatgpt_account_id": "test-account", "auth_mode": OpenAIAuthModeAgentIdentity},
	} {
		_, err := (sharedPoolNativeModelDiscoverer{accountTest: &AccountTestService{}, client: client}).DiscoverNativeModels(context.Background(), &Account{
			Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: credentials,
		})
		require.Error(t, err)
	}
	require.Zero(t, calls)
}

func TestSharedPoolNativeReadiness_DetachedBindingSkipsDiagnostics(t *testing.T) {
	binding := &SharedPoolNativeBinding{Account: SharedPoolAccount{ID: 7, PoolID: 1, OwnerID: 2, NativeBindingState: "detached"}}
	accounts := &nativeAccountRepoFake{account: &Account{ID: 7}}
	base := &BizDecipherService{}
	base.SetSharedPoolNativeOnboarding(&nativeOnboardingRepoFake{binding: binding}, accounts)
	base.SetSharedPoolNativeReadiness(&readinessRepoFake{}, readinessModelsFake{}, readinessConnFake{})
	_, err := base.VerifySharedPoolNativeReadiness(context.Background(), SharedPoolNativeReadinessInput{PoolID: 1, OwnerID: 2, SharedAccountID: 7, ExpectedConfigVersion: 1})
	require.ErrorIs(t, err, ErrNativeIdentityDeleted)
	require.Zero(t, accounts.getCalls)
}

func TestSharedPoolNativeReadiness_OpenAIMetadataRejectsExpiredOrAgentAccounts(t *testing.T) {
	d := sharedPoolOpenAIMetadataDiagnostic{}
	for _, account := range []*Account{
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "acct", "expires_at": time.Now().Add(-time.Minute).Format(time.RFC3339)}},
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "acct", "auth_mode": OpenAIAuthModeAgentIdentity}},
	} {
		evidence, err := d.VerifyNativeConnection(context.Background(), account)
		require.NoError(t, err)
		require.Equal(t, "unverified", evidence.Status)
	}
}
