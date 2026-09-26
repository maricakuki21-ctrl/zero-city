package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type nativeOnboardingRepoFake struct {
	binding       *SharedPoolNativeBinding
	claimErr      error
	attachErrors  []error
	attachCalls   int
	repairClaim   *SharedPoolNativeRepairClaim
	repairErr     error
	completeErrs  []error
	completeCalls int
}

func (f *nativeOnboardingRepoFake) CreateNativeDraft(context.Context, SharedPoolNativeDraftInput) (*SharedPool, error) {
	return nil, nil
}

func (f *nativeOnboardingRepoFake) ClaimNativeBinding(context.Context, SharedPoolNativeAccountInput) (*SharedPoolNativeBinding, error) {
	return f.binding, f.claimErr
}

func (f *nativeOnboardingRepoFake) AttachNativeAccount(_ context.Context, _ SharedPoolNativeAccountInput, binding *SharedPoolNativeBinding, _ *Account) (*SharedPoolAccount, error) {
	f.attachCalls++
	if len(f.attachErrors) >= f.attachCalls && f.attachErrors[f.attachCalls-1] != nil {
		return nil, f.attachErrors[f.attachCalls-1]
	}
	account := binding.Account
	account.NativeBindingState = "attached"
	return &account, nil
}

func (f *nativeOnboardingRepoFake) MarkNativeBindingAttention(context.Context, int64, int64, int64, string, string) error {
	return nil
}

func (f *nativeOnboardingRepoFake) GetNativeBinding(context.Context, int64, int64, int64) (*SharedPoolAccount, error) {
	account := f.binding.Account
	return &account, nil
}

func (f *nativeOnboardingRepoFake) ClaimNativeRepair(context.Context, SharedPoolNativeRepairInput) (*SharedPoolNativeRepairClaim, error) {
	return f.repairClaim, f.repairErr
}

func (f *nativeOnboardingRepoFake) CompleteNativeRepair(_ context.Context, _ SharedPoolNativeRepairInput, claim *SharedPoolNativeRepairClaim, account *Account) (*SharedPoolAccount, error) {
	f.completeCalls++
	if len(f.completeErrs) >= f.completeCalls && f.completeErrs[f.completeCalls-1] != nil {
		return nil, f.completeErrs[f.completeCalls-1]
	}
	result := claim.Binding.Account
	result.NativeBindingState = "attached"
	result.NativeBindingStep = "model_discovery"
	result.NativeEvidenceStale = true
	result.NativeAccountObservedUpdatedAt = &account.UpdatedAt
	return &result, nil
}

func (f *nativeOnboardingRepoFake) DetachNativeBinding(context.Context, int64, int64, int64) error {
	return nil
}

type nativeAccountRepoFake struct {
	account       *Account
	deleted       bool
	getErr        error
	getCalls      int
	createCalls   int
	repairCalls   int
	createdGroups []AccountGroup
}

type nativeLegacyCredentialGuardRepo struct {
	BizDecipherRepository
	pool        *SharedPool
	createCalls int
}

func (r *nativeLegacyCredentialGuardRepo) GetOwnedSharedPool(context.Context, int64, int64) (*SharedPool, error) {
	return r.pool, nil
}

func (r *nativeLegacyCredentialGuardRepo) CreateSharedPoolAccount(context.Context, SharedPoolAccountInput) (*SharedPoolAccount, error) {
	r.createCalls++
	return &SharedPoolAccount{ID: 1}, nil
}

func (f *nativeAccountRepoFake) CreateWithAccountGroups(_ context.Context, account *Account, groups []AccountGroup) error {
	f.createCalls++
	account.ID = 91
	account.UpdatedAt = time.Unix(100, 0).UTC()
	f.account = account
	f.createdGroups = groups
	return nil
}

func (f *nativeAccountRepoFake) GetBySharedPoolSource(context.Context, int64) (*Account, bool, error) {
	f.getCalls++
	if f.account == nil {
		return nil, false, f.getErr
	}
	return f.account, f.deleted, nil
}

func (f *nativeAccountRepoFake) RepairNativeCredentialsCAS(_ context.Context, account *Account, expected time.Time, operationID, fingerprint string) (bool, error) {
	f.repairCalls++
	if f.account == nil || !f.account.UpdatedAt.Equal(expected) {
		return false, nil
	}
	account.ID = f.account.ID
	account.UpdatedAt = expected.Add(time.Second)
	account.Extra["shared_pool_repair_operation_id"] = operationID
	account.Extra["shared_pool_repair_request_fingerprint"] = fingerprint
	f.account = account
	return true, nil
}

func TestSharedPoolNativeOnboarding_RejectsPrivateEndpoint(t *testing.T) {
	_, err := NewSharedPoolNativeAccountInput(SharedPoolNativeAccountRequest{
		PoolID: 1, OwnerID: 2, OperationID: "op-1", Name: "private", Provider: PlatformOpenAI,
		AuthType: "api_key", UpstreamBaseURL: "http://127.0.0.1:8080", APIKey: "secret",
	})
	require.ErrorIs(t, err, ErrUnsafeUpstreamEndpoint)
}

func TestSharedPoolNativeOnboarding_RejectsCredentialNetworkOverride(t *testing.T) {
	_, err := NewSharedPoolNativeAccountInput(SharedPoolNativeAccountRequest{
		PoolID: 1, OwnerID: 2, OperationID: "op-1", Name: "oauth", Provider: PlatformOpenAI,
		AuthType: AccountTypeOAuth, UpstreamBaseURL: "https://api.openai.com",
		Credentials: map[string]any{"access_token": "secret", "base_url": "https://127.0.0.1"},
	})
	require.ErrorIs(t, err, ErrOwnerNativeFieldRejected)
}

func TestSharedPoolNativeOnboarding_RejectsUnsupportedOAuthProvider(t *testing.T) {
	_, err := NewSharedPoolNativeAccountInput(SharedPoolNativeAccountRequest{
		PoolID: 1, OwnerID: 2, OperationID: "op-1", Name: "oauth", Provider: PlatformAnthropic,
		AuthType: AccountTypeOAuth, UpstreamBaseURL: "https://api.anthropic.com",
		Credentials: map[string]any{"access_token": "secret"},
	})
	require.ErrorIs(t, err, ErrUnsupportedNativeAuth)
}

func TestSharedPoolNativeOnboarding_CreatesUnschedulableNativeAccount(t *testing.T) {
	input := mustNativeAccountInput(t, "create-1", "secret-1")
	binding := &SharedPoolNativeBinding{
		Account:    SharedPoolAccount{ID: 41, PoolID: 1, OwnerID: 2},
		BindingRef: "34aa25c0-18f2-46df-855a-bccce936607e", OperationID: input.OperationID,
		RequestFingerprint: input.RequestFingerprint,
	}
	onboarding := &nativeOnboardingRepoFake{binding: binding}
	accounts := &nativeAccountRepoFake{getErr: ErrAccountNotFound}
	service := &BizDecipherService{}
	service.SetSharedPoolNativeOnboarding(onboarding, accounts)

	result, err := service.OnboardSharedPoolNativeAccount(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, int64(41), result.ID)
	require.Equal(t, 1, accounts.createCalls)
	require.Empty(t, accounts.createdGroups)
	require.False(t, accounts.account.Schedulable)
	require.Empty(t, accounts.account.GroupIDs)
	require.Equal(t, "https://api.openai.com", accounts.account.Credentials["base_url"])
	require.Equal(t, "secret-1", accounts.account.Credentials["api_key"])
	require.NotContains(t, accounts.account.Extra, "base_url")
	require.Equal(t, "pool_account", accounts.account.Extra["bizdecipher_supply_source"])
	require.Equal(t, int64(41), accounts.account.Extra["bizdecipher_supply_id"])
}

func TestSharedPoolNativeOnboarding_ResumesAfterAttachFailure(t *testing.T) {
	input := mustNativeAccountInput(t, "create-2", "secret-2")
	binding := &SharedPoolNativeBinding{Account: SharedPoolAccount{ID: 42, PoolID: 1, OwnerID: 2}, BindingRef: "c42ab441-3afb-4fd7-a500-ecb24af53e10", RequestFingerprint: input.RequestFingerprint}
	onboarding := &nativeOnboardingRepoFake{binding: binding, attachErrors: []error{errors.New("attach interrupted"), nil}}
	accounts := &nativeAccountRepoFake{getErr: ErrAccountNotFound}
	service := &BizDecipherService{}
	service.SetSharedPoolNativeOnboarding(onboarding, accounts)

	_, firstErr := service.OnboardSharedPoolNativeAccount(context.Background(), input)
	result, retryErr := service.OnboardSharedPoolNativeAccount(context.Background(), input)

	require.Error(t, firstErr)
	require.NoError(t, retryErr)
	require.Equal(t, int64(42), result.ID)
	require.Equal(t, 1, accounts.createCalls)
	require.Equal(t, 2, onboarding.attachCalls)
}

func TestSharedPoolNativeOnboarding_RejectsForeignOwnerBeforeNativeLookup(t *testing.T) {
	input := mustNativeAccountInput(t, "create-3", "secret-3")
	onboarding := &nativeOnboardingRepoFake{claimErr: ErrPoolForbidden}
	accounts := &nativeAccountRepoFake{}
	service := &BizDecipherService{}
	service.SetSharedPoolNativeOnboarding(onboarding, accounts)

	_, err := service.OnboardSharedPoolNativeAccount(context.Background(), input)

	require.ErrorIs(t, err, ErrPoolForbidden)
	require.Zero(t, accounts.getCalls)
	require.Zero(t, accounts.createCalls)
}

func TestSharedPoolNativeOnboarding_RejectsSoftDeletedIdentity(t *testing.T) {
	input := mustNativeAccountInput(t, "create-4", "secret-4")
	binding := &SharedPoolNativeBinding{Account: SharedPoolAccount{ID: 44, PoolID: 1, OwnerID: 2}, RequestFingerprint: input.RequestFingerprint}
	onboarding := &nativeOnboardingRepoFake{binding: binding}
	accounts := &nativeAccountRepoFake{account: &Account{ID: 92, Extra: map[string]any{"shared_pool_request_fingerprint": input.RequestFingerprint}}, deleted: true}
	service := &BizDecipherService{}
	service.SetSharedPoolNativeOnboarding(onboarding, accounts)

	_, err := service.OnboardSharedPoolNativeAccount(context.Background(), input)

	require.ErrorIs(t, err, ErrNativeIdentityDeleted)
	require.Zero(t, accounts.createCalls)
}

func TestSharedPoolNativeOnboarding_RepairResumesAfterNativeCommit(t *testing.T) {
	initialFingerprint := "initial-create-fingerprint"
	observed := time.Unix(200, 0).UTC()
	binding := SharedPoolNativeBinding{
		Account:            SharedPoolAccount{ID: 45, PoolID: 1, OwnerID: 2, NativeOperationID: "initial-create-op"},
		RequestFingerprint: initialFingerprint,
	}
	input, err := NewSharedPoolNativeRepairInput(SharedPoolNativeRepairRequest{
		PoolID: 1, OwnerID: 2, SharedAccountID: 45, RepairOperationID: "repair-1", ExpectedConfigVersion: 7,
		Provider: PlatformOpenAI, AuthType: AccountTypeAPIKey, UpstreamBaseURL: "https://api.openai.com", APIKey: "new-secret",
	})
	require.NoError(t, err)
	claim := &SharedPoolNativeRepairClaim{Binding: binding, ExpectedNativeUpdatedAt: observed}
	onboarding := &nativeOnboardingRepoFake{binding: &binding, repairClaim: claim, completeErrs: []error{errors.New("progress commit interrupted"), nil}}
	accounts := &nativeAccountRepoFake{account: &Account{ID: 93, UpdatedAt: observed, Extra: map[string]any{"shared_pool_request_fingerprint": initialFingerprint}}}
	service := &BizDecipherService{}
	service.SetSharedPoolNativeOnboarding(onboarding, accounts)

	_, firstErr := service.RepairSharedPoolNativeAccount(context.Background(), input)
	result, retryErr := service.RepairSharedPoolNativeAccount(context.Background(), input)

	require.Error(t, firstErr)
	require.NoError(t, retryErr)
	require.Equal(t, 1, accounts.repairCalls)
	require.Equal(t, 2, onboarding.completeCalls)
	require.Equal(t, "initial-create-op", result.NativeOperationID)
	require.Equal(t, initialFingerprint, binding.RequestFingerprint)
	require.True(t, result.NativeEvidenceStale)
}

func TestSharedPoolNativeOnboarding_FingerprintRedactsSecret(t *testing.T) {
	input := mustNativeAccountInput(t, "create-5", "R1_SECRET_MUST_NOT_RENDER")
	require.NotContains(t, input.RequestFingerprint, "R1_SECRET_MUST_NOT_RENDER")
	require.Len(t, input.RequestFingerprint, 64)
}

func TestSharedPoolNativeOnboarding_LegacyCreateAndImportRejectNativePool(t *testing.T) {
	ownerID := int64(2)
	repo := &nativeLegacyCredentialGuardRepo{pool: &SharedPool{
		ID: 1, OwnerID: &ownerID, NativeOnboardingState: SharedPoolOnboardingSupplyConfiguring,
	}}
	service := NewBizDecipherService(repo, nil, nil)
	input := SharedPoolAccountInput{
		PoolID: 1, OwnerID: ownerID, Name: "legacy-secret", Provider: PlatformOpenAI,
		AuthType: AccountTypeAPIKey, UpstreamBaseURL: "https://api.openai.com", UpstreamAPIKey: "secret",
		ModelConfigs: []SharedPoolModelInput{{ModelName: "gpt-5.4", RateMultiplier: 1, ModelOpen: true}},
	}

	_, createErr := service.CreateSharedPoolAccount(context.Background(), input)
	result, importErr := service.ImportSharedPoolAccounts(context.Background(), ImportSharedPoolAccountsInput{
		PoolID: 1, OwnerID: ownerID, Items: []SharedPoolAccountInput{input},
	})

	require.ErrorIs(t, createErr, ErrNativePoolImmutable)
	require.NoError(t, importErr)
	require.Equal(t, 1, result.Failed)
	require.ErrorContains(t, errors.New(result.Items[0].Error), ErrNativePoolImmutable.Error())
	require.Zero(t, repo.createCalls)
}

func mustNativeAccountInput(t *testing.T, operationID, secret string) SharedPoolNativeAccountInput {
	t.Helper()
	input, err := NewSharedPoolNativeAccountInput(SharedPoolNativeAccountRequest{
		PoolID: 1, OwnerID: 2, OperationID: operationID, Name: "primary", Provider: PlatformOpenAI,
		AuthType: AccountTypeAPIKey, UpstreamBaseURL: "https://api.openai.com", APIKey: secret,
	})
	require.NoError(t, err)
	return input
}
