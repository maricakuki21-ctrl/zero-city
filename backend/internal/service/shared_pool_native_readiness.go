package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const NativeConnectionAuthenticatedMetadataReachable = "authenticated_metadata_reachable"

type SharedPoolNativeReadinessInput struct {
	PoolID                int64
	OwnerID               int64
	SharedAccountID       int64
	ExpectedConfigVersion int64
}

type SharedPoolNativeReadinessRepository interface {
	PersistNativeReadiness(context.Context, SharedPoolNativeReadinessInput, NativeReadinessUpdate) error
}

type SharedPoolNativeModelDiscoverer interface {
	DiscoverNativeModels(context.Context, *Account) ([]string, error)
}

type SharedPoolNativeConnectionDiagnostic interface {
	VerifyNativeConnection(context.Context, *Account) (NativeConnectionEvidence, error)
}

type NativeConnectionEvidence struct {
	Status       string
	ErrorCode    string
	ErrorMessage string
	Latency      time.Duration
}

type NativeReadinessUpdate struct {
	Models                  []string
	ModelsVerifiedAt        *time.Time
	ConnectionStatus        string
	ConnectionVerifiedAt    *time.Time
	ErrorCode               string
	ErrorMessage            string
	ObservedNativeUpdatedAt time.Time
	Latency                 time.Duration
}

type sharedPoolNativeModelDiscoverer struct {
	accountTest *AccountTestService
	client      *http.Client
}

func (d sharedPoolNativeModelDiscoverer) DiscoverNativeModels(ctx context.Context, account *Account) ([]string, error) {
	if d.accountTest == nil || account == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	if account.ProxyID != nil || account.Proxy != nil {
		return nil, errors.New("owner native diagnostics do not permit proxy routing")
	}
	request, err := d.accountTest.buildOwnerSafeUpstreamModelsRequest(ctx, account)
	if err != nil {
		return nil, err
	}
	client := d.client
	if client == nil {
		client, err = newOwnerNativeDiagnosticClient()
		if err != nil {
			return nil, err
		}
	}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("native model catalog request rejected")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamModelsBodyLimit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > upstreamModelsBodyLimit {
		return nil, errors.New("native model catalog response is too large")
	}
	extractModels := extractUpstreamModelIDs
	if account.IsGrok() {
		extractModels = extractGrokUpstreamModelIDs
	}
	models, err := extractModels(body)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, errors.New("native model catalog is empty")
	}
	return models, nil
}

type sharedPoolOpenAIMetadataDiagnostic struct{ client *http.Client }

func (d sharedPoolOpenAIMetadataDiagnostic) VerifyNativeConnection(ctx context.Context, account *Account) (NativeConnectionEvidence, error) {
	if account != nil && (account.ProxyID != nil || account.Proxy != nil) {
		return NativeConnectionEvidence{Status: "failed", ErrorCode: "owner_proxy_rejected", ErrorMessage: "owner native diagnostics do not permit proxy routing"}, ErrOwnerNativeFieldRejected
	}
	if account != nil && account.Type == AccountTypeAPIKey {
		if err := validateNativeProviderAuth(account.Platform, account.Type); err == nil {
			// Authenticated catalog metadata proves reachability, not inference or billing.
			start := time.Now()
			_, err := (sharedPoolNativeModelDiscoverer{accountTest: &AccountTestService{}, client: d.client}).DiscoverNativeModels(ctx, account)
			if err != nil {
				return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_request_failed", ErrorMessage: "metadata request failed", Latency: time.Since(start)}, err
			}
			return NativeConnectionEvidence{Status: NativeConnectionAuthenticatedMetadataReachable, Latency: time.Since(start)}, nil
		}
	}
	if account == nil || !account.IsOpenAIOAuth() || account.IsOpenAIAgentIdentity() || account.IsOpenAITokenExpired() {
		return NativeConnectionEvidence{Status: "unverified", ErrorCode: "connection_path_unverified"}, nil
	}
	token := strings.TrimSpace(account.GetOpenAIAccessToken())
	accountID := strings.TrimSpace(account.GetCredential("chatgpt_account_id"))
	if token == "" || accountID == "" {
		return NativeConnectionEvidence{Status: "unverified", ErrorCode: "connection_path_unverified"}, nil
	}
	metadataURL, err := url.Parse(chatGPTUsageURL)
	if err != nil || metadataURL.Scheme != "https" || metadataURL.Hostname() != "chatgpt.com" {
		return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_endpoint_untrusted", ErrorMessage: "metadata endpoint is not trusted"}, errors.New("metadata endpoint is not trusted")
	}
	client := d.client
	if client == nil {
		client, err = newOwnerNativeDiagnosticClient()
		if err != nil {
			return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_client_error", ErrorMessage: "metadata client unavailable"}, err
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	start := time.Now()
	var payload OpenAIQuotaUsage
	request, err := http.NewRequestWithContext(callCtx, http.MethodGet, chatGPTUsageURL, nil)
	if err != nil {
		return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_request_failed", ErrorMessage: "metadata request failed"}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("chatgpt-account-id", accountID)
	request.Header.Set("Accept", "application/json")
	resp, err := client.Do(request)
	latency := time.Since(start)
	if err != nil {
		return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_request_failed", ErrorMessage: "metadata request failed", Latency: latency}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_http_error", ErrorMessage: "metadata request rejected", Latency: latency}, errors.New("metadata request rejected")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamModelsBodyLimit+1))
	if err != nil || int64(len(body)) > upstreamModelsBodyLimit || !json.Valid(body) || json.Unmarshal(body, &payload) != nil || (payload.RateLimit == nil && payload.UserID == "" && payload.AccountID == "" && payload.PlanType == "") {
		return NativeConnectionEvidence{Status: "failed", ErrorCode: "metadata_invalid_response", ErrorMessage: "metadata response was not a recognized quota payload", Latency: latency}, errors.New("metadata response was not a recognized quota payload")
	}
	return NativeConnectionEvidence{Status: NativeConnectionAuthenticatedMetadataReachable, Latency: latency}, nil
}

func (s *BizDecipherService) SetSharedPoolNativeReadiness(repo SharedPoolNativeReadinessRepository, models SharedPoolNativeModelDiscoverer, connection SharedPoolNativeConnectionDiagnostic) {
	if s == nil {
		return
	}
	s.nativeReadinessRepo = repo
	s.nativeModelDiscoverer = models
	s.nativeConnectionDiagnostic = connection
}

func (s *BizDecipherService) VerifySharedPoolNativeReadiness(ctx context.Context, input SharedPoolNativeReadinessInput) (*SharedPoolAccount, error) {
	if s == nil || s.nativeOnboardingRepo == nil || s.nativeAccountRepo == nil || s.nativeReadinessRepo == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	if input.PoolID <= 0 || input.OwnerID <= 0 || input.SharedAccountID <= 0 || input.ExpectedConfigVersion <= 0 {
		return nil, ErrOwnerNativeFieldRejected
	}
	binding, err := s.nativeOnboardingRepo.GetNativeBinding(ctx, input.PoolID, input.OwnerID, input.SharedAccountID)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(binding.NativeBindingState), "detached") {
		return nil, ErrNativeIdentityDeleted
	}
	native, deleted, err := s.nativeAccountRepo.GetBySharedPoolSource(ctx, input.SharedAccountID)
	if err != nil {
		return nil, err
	}
	if deleted || native == nil {
		return nil, ErrNativeIdentityDeleted
	}
	update := NativeReadinessUpdate{ObservedNativeUpdatedAt: native.UpdatedAt}
	if s.nativeModelDiscoverer != nil {
		models, modelErr := s.nativeModelDiscoverer.DiscoverNativeModels(ctx, native)
		if modelErr == nil {
			now := time.Now().UTC()
			update.Models, update.ModelsVerifiedAt = models, &now
		} else {
			update.ErrorCode, update.ErrorMessage = "model_catalog_unavailable", "native model catalog unavailable"
		}
	}
	if s.nativeConnectionDiagnostic != nil {
		evidence, connErr := s.nativeConnectionDiagnostic.VerifyNativeConnection(ctx, native)
		update.ConnectionStatus, update.Latency = evidence.Status, evidence.Latency
		if evidence.ErrorCode != "" {
			update.ErrorCode, update.ErrorMessage = evidence.ErrorCode, evidence.ErrorMessage
		}
		if connErr == nil && evidence.Status == NativeConnectionAuthenticatedMetadataReachable {
			now := time.Now().UTC()
			update.ConnectionVerifiedAt = &now
		}
	}
	if update.ConnectionStatus == "" {
		update.ConnectionStatus = "unverified"
	}
	if err := s.nativeReadinessRepo.PersistNativeReadiness(ctx, input, update); err != nil {
		return nil, err
	}
	// Persistence can revoke runtime eligibility and invalidate the observed
	// account timestamp. Return the committed state, not the pre-write probe.
	return s.nativeOnboardingRepo.GetNativeBinding(ctx, input.PoolID, input.OwnerID, input.SharedAccountID)
}

func NewSharedPoolOpenAIMetadataDiagnostic(_ PrivacyClientFactory) SharedPoolNativeConnectionDiagnostic {
	return sharedPoolOpenAIMetadataDiagnostic{}
}

func NewSharedPoolNativeModelDiscoverer(accountTest *AccountTestService) SharedPoolNativeModelDiscoverer {
	return sharedPoolNativeModelDiscoverer{accountTest: accountTest}
}

func newOwnerNativeDiagnosticClient() (*http.Client, error) {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		ControlContext: func(_ context.Context, _, address string, _ syscall.RawConn) error {
			// net.Dialer passes the resolved IP here, immediately before connect.
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			_, err = urlvalidator.ValidateHTTPSURL("https://"+net.JoinHostPort(ip.WithZone("").String(), port), urlvalidator.ValidationOptions{AllowPrivate: false})
			return err
		},
	}
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}
