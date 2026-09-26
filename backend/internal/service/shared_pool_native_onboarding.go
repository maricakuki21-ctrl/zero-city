package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	SharedPoolOnboardingDraft                = "draft"
	SharedPoolOnboardingSupplyConfiguring    = "supply_configuring"
	SharedPoolOnboardingSupplyNeedsAttention = "supply_needs_attention"
	SharedPoolOnboardingReadyBillingBlocked  = "supply_ready_billing_blocked"
	SharedPoolOnboardingBillingActive        = "billing_active"
	SharedPoolOnboardingLegacyExisting       = "legacy_existing"
)

var (
	ErrNativeOnboardingUnavailable = infraerrors.ServiceUnavailable("native_onboarding_unavailable", "Native onboarding is temporarily unavailable")
	ErrNativeOperationConflict     = infraerrors.Conflict("operation_conflict", "The operation ID was already used with different input")
	ErrUnsupportedNativeAuth       = infraerrors.New(http.StatusUnprocessableEntity, "unsupported_native_auth", "The native authentication type is not supported")
	ErrUnsupportedNativeProvider   = infraerrors.New(http.StatusUnprocessableEntity, "unsupported_native_provider", "The native provider is not supported")
	ErrUnsafeUpstreamEndpoint      = infraerrors.New(http.StatusUnprocessableEntity, "unsafe_upstream_endpoint", "The upstream endpoint is not allowed")
	ErrOwnerNativeFieldRejected    = infraerrors.New(http.StatusUnprocessableEntity, "owner_native_field_rejected", "The request contains an owner-forbidden native field")
	ErrBillingActivationRequired   = infraerrors.Conflict("billing_activation_required", "Billing activation is required before this pool can be used")
	ErrLegacyMigrationRequired     = infraerrors.Conflict("legacy_migration_required", "This legacy pool must be migrated before native supply can be added")
	ErrNativeIdentityDeleted       = infraerrors.Conflict("native_identity_deleted", "The original native account was deleted and requires administrator repair")
	ErrNativePoolImmutable         = infraerrors.Conflict("native_pool_immutable", "Native R1 supply cannot be converted to the legacy credential mode")
)

type SharedPoolNativeDraftInput struct {
	OwnerID            int64
	Name               string
	Description        string
	OperationID        string
	RequestFingerprint string
}

type SharedPoolNativeAccountRequest struct {
	PoolID          int64
	OwnerID         int64
	OperationID     string
	Name            string
	Description     string
	Provider        string
	AuthType        string
	UpstreamBaseURL string
	APIKey          string
	Credentials     map[string]any
	ProxyID         *int64
	ProxyURL        string
}

type SharedPoolNativeAccountInput struct {
	SharedPoolNativeAccountRequest
	BindingRef         string
	RequestFingerprint string
	CredentialDigest   string
}

type SharedPoolNativeBinding struct {
	Account            SharedPoolAccount
	BindingRef         string
	OperationID        string
	RequestFingerprint string
	State              string
	Step               string
}

type SharedPoolNativeRepairRequest struct {
	PoolID                int64
	OwnerID               int64
	SharedAccountID       int64
	RepairOperationID     string
	ExpectedConfigVersion int64
	Provider              string
	AuthType              string
	UpstreamBaseURL       string
	APIKey                string
	Credentials           map[string]any
}

type SharedPoolNativeRepairInput struct {
	SharedPoolNativeRepairRequest
	RequestFingerprint string
	CredentialDigest   string
}

type SharedPoolNativeRepairClaim struct {
	Binding                 SharedPoolNativeBinding
	ExpectedNativeUpdatedAt time.Time
}

type SharedPoolNativeOnboardingRepository interface {
	CreateNativeDraft(context.Context, SharedPoolNativeDraftInput) (*SharedPool, error)
	ClaimNativeBinding(context.Context, SharedPoolNativeAccountInput) (*SharedPoolNativeBinding, error)
	AttachNativeAccount(context.Context, SharedPoolNativeAccountInput, *SharedPoolNativeBinding, *Account) (*SharedPoolAccount, error)
	MarkNativeBindingAttention(context.Context, int64, int64, int64, string, string) error
	GetNativeBinding(context.Context, int64, int64, int64) (*SharedPoolAccount, error)
	ClaimNativeRepair(context.Context, SharedPoolNativeRepairInput) (*SharedPoolNativeRepairClaim, error)
	CompleteNativeRepair(context.Context, SharedPoolNativeRepairInput, *SharedPoolNativeRepairClaim, *Account) (*SharedPoolAccount, error)
	DetachNativeBinding(context.Context, int64, int64, int64) error
}

type SharedPoolNativeAccountRepository interface {
	CreateWithAccountGroups(context.Context, *Account, []AccountGroup) error
	GetBySharedPoolSource(context.Context, int64) (*Account, bool, error)
	RepairNativeCredentialsCAS(context.Context, *Account, time.Time, string, string) (bool, error)
}

func NewSharedPoolNativeDraftInput(ownerID int64, name, description, operationID string) (SharedPoolNativeDraftInput, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	operationID = strings.TrimSpace(operationID)
	if ownerID <= 0 || name == "" || operationID == "" || len(operationID) > 160 {
		return SharedPoolNativeDraftInput{}, ErrOwnerNativeFieldRejected
	}
	return SharedPoolNativeDraftInput{
		OwnerID:            ownerID,
		Name:               name,
		Description:        description,
		OperationID:        operationID,
		RequestFingerprint: nativeSHA(strings.ToLower(name) + "\x00" + description + "\x00native"),
	}, nil
}

func NewSharedPoolNativeAccountInput(req SharedPoolNativeAccountRequest) (SharedPoolNativeAccountInput, error) {
	req.OperationID = strings.TrimSpace(req.OperationID)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.AuthType = strings.ToLower(strings.TrimSpace(req.AuthType))
	if req.ProxyID != nil || strings.TrimSpace(req.ProxyURL) != "" {
		return SharedPoolNativeAccountInput{}, ErrOwnerNativeFieldRejected
	}
	if err := rejectNativeCredentialOverrides(req.Credentials); err != nil {
		return SharedPoolNativeAccountInput{}, err
	}
	if req.PoolID <= 0 || req.OwnerID <= 0 || req.OperationID == "" || len(req.OperationID) > 160 || req.Name == "" || req.Provider == "" {
		return SharedPoolNativeAccountInput{}, ErrOwnerNativeFieldRejected
	}
	if req.AuthType == "api_key" {
		req.AuthType = AccountTypeAPIKey
	}
	if err := validateNativeProviderAuth(req.Provider, req.AuthType); err != nil {
		return SharedPoolNativeAccountInput{}, err
	}
	if req.AuthType == AccountTypeAPIKey && strings.TrimSpace(req.APIKey) == "" {
		return SharedPoolNativeAccountInput{}, ErrOwnerNativeFieldRejected
	}
	if req.AuthType == AccountTypeOAuth && len(req.Credentials) == 0 {
		return SharedPoolNativeAccountInput{}, ErrOwnerNativeFieldRejected
	}
	normalizedURL, err := validateNativeOwnerURL(req.UpstreamBaseURL)
	if err != nil {
		return SharedPoolNativeAccountInput{}, err
	}
	req.UpstreamBaseURL = normalizedURL
	credentialDigest := nativeSHA(strings.TrimSpace(req.APIKey) + "\x00" + canonicalNativeCredentials(req.Credentials))
	requestFingerprint := nativeSHA(strings.Join([]string{
		req.Name,
		req.Description,
		req.Provider,
		req.AuthType,
		normalizedURL,
		credentialDigest,
	}, "\x00"))
	return SharedPoolNativeAccountInput{
		SharedPoolNativeAccountRequest: req,
		BindingRef:                     uuid.NewString(),
		RequestFingerprint:             requestFingerprint,
		CredentialDigest:               credentialDigest,
	}, nil
}

func NewSharedPoolNativeRepairInput(req SharedPoolNativeRepairRequest) (SharedPoolNativeRepairInput, error) {
	req.RepairOperationID = strings.TrimSpace(req.RepairOperationID)
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.AuthType = strings.ToLower(strings.TrimSpace(req.AuthType))
	if req.PoolID <= 0 || req.OwnerID <= 0 || req.SharedAccountID <= 0 || req.ExpectedConfigVersion <= 0 || req.RepairOperationID == "" || len(req.RepairOperationID) > 160 {
		return SharedPoolNativeRepairInput{}, ErrOwnerNativeFieldRejected
	}
	if err := rejectNativeCredentialOverrides(req.Credentials); err != nil {
		return SharedPoolNativeRepairInput{}, err
	}
	if req.AuthType == "api_key" {
		req.AuthType = AccountTypeAPIKey
	}
	if err := validateNativeProviderAuth(req.Provider, req.AuthType); err != nil {
		return SharedPoolNativeRepairInput{}, err
	}
	if req.AuthType == AccountTypeAPIKey && strings.TrimSpace(req.APIKey) == "" {
		return SharedPoolNativeRepairInput{}, ErrOwnerNativeFieldRejected
	}
	if req.AuthType == AccountTypeOAuth && len(req.Credentials) == 0 {
		return SharedPoolNativeRepairInput{}, ErrOwnerNativeFieldRejected
	}
	normalizedURL, err := validateNativeOwnerURL(req.UpstreamBaseURL)
	if err != nil {
		return SharedPoolNativeRepairInput{}, err
	}
	req.UpstreamBaseURL = normalizedURL
	credentialDigest := nativeSHA(strings.TrimSpace(req.APIKey) + "\x00" + canonicalNativeCredentials(req.Credentials))
	return SharedPoolNativeRepairInput{
		SharedPoolNativeRepairRequest: req,
		CredentialDigest:              credentialDigest,
		RequestFingerprint: nativeSHA(strings.Join([]string{
			fmt.Sprint(req.SharedAccountID), req.Provider, req.AuthType, normalizedURL, credentialDigest,
		}, "\x00")),
	}, nil
}

func rejectNativeCredentialOverrides(credentials map[string]any) error {
	for key := range credentials {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "base_url", "proxy_id", "proxy_url", "group_id", "group_ids", "schedulable", "status", "extra", "api_key":
			return ErrOwnerNativeFieldRejected
		}
	}
	return nil
}

func validateNativeProviderAuth(provider, authType string) error {
	switch provider {
	case PlatformOpenAI:
		if authType == AccountTypeAPIKey || authType == AccountTypeOAuth {
			return nil
		}
	case PlatformAnthropic, PlatformGemini, PlatformGrok:
		if authType == AccountTypeAPIKey {
			return nil
		}
	default:
		return ErrUnsupportedNativeProvider
	}
	return ErrUnsupportedNativeAuth
}

func (s *BizDecipherService) SetSharedPoolNativeOnboarding(repo SharedPoolNativeOnboardingRepository, accounts SharedPoolNativeAccountRepository) {
	if s == nil {
		return
	}
	s.nativeOnboardingRepo = repo
	s.nativeAccountRepo = accounts
}

func (s *BizDecipherService) CreateSharedPoolNativeDraft(ctx context.Context, input SharedPoolNativeDraftInput) (*SharedPool, error) {
	if s == nil || s.nativeOnboardingRepo == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	return s.nativeOnboardingRepo.CreateNativeDraft(ctx, input)
}

func (s *BizDecipherService) OnboardSharedPoolNativeAccount(ctx context.Context, input SharedPoolNativeAccountInput) (*SharedPoolAccount, error) {
	if s == nil || s.nativeOnboardingRepo == nil || s.nativeAccountRepo == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	binding, err := s.nativeOnboardingRepo.ClaimNativeBinding(ctx, input)
	if err != nil {
		return nil, err
	}

	account, deleted, lookupErr := s.nativeAccountRepo.GetBySharedPoolSource(ctx, binding.Account.ID)
	if lookupErr != nil && !errors.Is(lookupErr, ErrAccountNotFound) {
		return nil, lookupErr
	}
	if errors.Is(lookupErr, ErrAccountNotFound) {
		account = nativeAccountFromOnboarding(input, binding)
		if createErr := s.nativeAccountRepo.CreateWithAccountGroups(ctx, account, []AccountGroup{}); createErr != nil {
			account, deleted, lookupErr = s.nativeAccountRepo.GetBySharedPoolSource(ctx, binding.Account.ID)
			if lookupErr != nil {
				_ = s.nativeOnboardingRepo.MarkNativeBindingAttention(ctx, input.PoolID, input.OwnerID, binding.Account.ID, "native_account_create_failed", "Native account creation failed; retry this item.")
				return nil, fmt.Errorf("create native account: %w", createErr)
			}
		}
	}
	if deleted {
		return nil, ErrNativeIdentityDeleted
	}
	if account == nil || fmt.Sprint(account.Extra["shared_pool_request_fingerprint"]) != input.RequestFingerprint {
		return nil, ErrNativeOperationConflict
	}
	return s.nativeOnboardingRepo.AttachNativeAccount(ctx, input, binding, account)
}

func (s *BizDecipherService) GetSharedPoolNativeAccount(ctx context.Context, poolID, ownerID, sharedAccountID int64) (*SharedPoolAccount, error) {
	if s == nil || s.nativeOnboardingRepo == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	if poolID <= 0 || ownerID <= 0 || sharedAccountID <= 0 {
		return nil, ErrOwnerNativeFieldRejected
	}
	return s.nativeOnboardingRepo.GetNativeBinding(ctx, poolID, ownerID, sharedAccountID)
}

func (s *BizDecipherService) RepairSharedPoolNativeAccount(ctx context.Context, input SharedPoolNativeRepairInput) (*SharedPoolAccount, error) {
	if s == nil || s.nativeOnboardingRepo == nil || s.nativeAccountRepo == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	claim, err := s.nativeOnboardingRepo.ClaimNativeRepair(ctx, input)
	if err != nil {
		return nil, err
	}
	account, deleted, err := s.nativeAccountRepo.GetBySharedPoolSource(ctx, input.SharedAccountID)
	if err != nil {
		return nil, err
	}
	if deleted {
		return nil, ErrNativeIdentityDeleted
	}
	if account == nil || fmt.Sprint(account.Extra["shared_pool_request_fingerprint"]) != claim.Binding.RequestFingerprint {
		return nil, ErrNativeOperationConflict
	}
	if repairOperation := fmt.Sprint(account.Extra["shared_pool_repair_operation_id"]); repairOperation == input.RepairOperationID {
		if fmt.Sprint(account.Extra["shared_pool_repair_request_fingerprint"]) != input.RequestFingerprint {
			return nil, ErrNativeOperationConflict
		}
		return s.nativeOnboardingRepo.CompleteNativeRepair(ctx, input, claim, account)
	}
	if !account.UpdatedAt.Equal(claim.ExpectedNativeUpdatedAt) {
		return nil, ErrSharedPoolConcurrentUpdate
	}
	repaired := repairedNativeAccount(account, input)
	updated, err := s.nativeAccountRepo.RepairNativeCredentialsCAS(ctx, repaired, claim.ExpectedNativeUpdatedAt, input.RepairOperationID, input.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if !updated {
		account, deleted, err = s.nativeAccountRepo.GetBySharedPoolSource(ctx, input.SharedAccountID)
		if err != nil {
			return nil, err
		}
		if deleted {
			return nil, ErrNativeIdentityDeleted
		}
		if fmt.Sprint(account.Extra["shared_pool_repair_operation_id"]) != input.RepairOperationID || fmt.Sprint(account.Extra["shared_pool_repair_request_fingerprint"]) != input.RequestFingerprint {
			return nil, ErrSharedPoolConcurrentUpdate
		}
		repaired = account
	}
	return s.nativeOnboardingRepo.CompleteNativeRepair(ctx, input, claim, repaired)
}

func isNativeR1Pool(pool *SharedPool) bool {
	return pool != nil && pool.NativeOnboardingState != "" && pool.NativeOnboardingState != SharedPoolOnboardingLegacyExisting
}

func guardNativeR1PoolUpdate(pool *SharedPool, input UpdateSharedPoolInput) error {
	if !isNativeR1Pool(pool) {
		return nil
	}
	if !input.AccountModeEnabled || strings.TrimSpace(input.UpstreamBaseURL) != "" || strings.TrimSpace(input.UpstreamAPIKey) != "" || input.ProxyID != nil || strings.TrimSpace(input.ProxyURL) != "" {
		return ErrNativePoolImmutable
	}
	if input.Listed && !isNativeBillingActive(pool) {
		return ErrBillingActivationRequired
	}
	return nil
}

func nativeAccountFromOnboarding(input SharedPoolNativeAccountInput, binding *SharedPoolNativeBinding) *Account {
	credentials := make(map[string]any, len(input.Credentials)+2)
	for key, value := range input.Credentials {
		credentials[key] = value
	}
	if input.AuthType == AccountTypeAPIKey {
		credentials["api_key"] = strings.TrimSpace(input.APIKey)
	}
	credentials["base_url"] = input.UpstreamBaseURL
	return &Account{
		Name:        input.Name,
		Platform:    input.Provider,
		Type:        input.AuthType,
		Credentials: credentials,
		Extra: map[string]any{
			"bizdecipher_supply_source":       "pool_account",
			"bizdecipher_supply_id":           binding.Account.ID,
			"shared_pool_binding_ref":         binding.BindingRef,
			"shared_pool_request_fingerprint": input.RequestFingerprint,
		},
		Concurrency: 1,
		Priority:    100,
		Status:      StatusActive,
		Schedulable: false,
		GroupIDs:    []int64{},
	}
}

func repairedNativeAccount(current *Account, input SharedPoolNativeRepairInput) *Account {
	repaired := *current
	repaired.Platform = input.Provider
	repaired.Type = input.AuthType
	repaired.Credentials = make(map[string]any, len(input.Credentials)+2)
	for key, value := range input.Credentials {
		repaired.Credentials[key] = value
	}
	if input.AuthType == AccountTypeAPIKey {
		repaired.Credentials["api_key"] = strings.TrimSpace(input.APIKey)
	}
	repaired.Credentials["base_url"] = input.UpstreamBaseURL
	repaired.Extra = make(map[string]any, len(current.Extra)+2)
	for key, value := range current.Extra {
		repaired.Extra[key] = value
	}
	return &repaired
}

func validateNativeOwnerURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return "", ErrUnsafeUpstreamEndpoint
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" {
		return "", ErrUnsafeUpstreamEndpoint
	}
	if ip := net.ParseIP(host); ip != nil && forbiddenNativeOwnerIP(ip) {
		return "", ErrUnsafeUpstreamEndpoint
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func forbiddenNativeOwnerIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func canonicalNativeCredentials(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fmt.Sprint(values[key]))
	}
	return strings.Join(parts, "&")
}

func nativeSHA(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
