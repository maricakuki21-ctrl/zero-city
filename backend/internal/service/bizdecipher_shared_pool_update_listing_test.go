package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type sharedPoolUpdateListingRepoStub struct {
	BizDecipherRepository
	current      *SharedPool
	updatedInput *UpdateSharedPoolInput
	accounts     []SharedPoolAccount
	runtimeReads int
	catalog      []ModelCatalogEntry
	catalogErr   error
}

func (r *sharedPoolUpdateListingRepoStub) GetOwnedSharedPool(context.Context, int64, int64) (*SharedPool, error) {
	if r.current == nil {
		return nil, nil
	}
	current := *r.current
	current.Models = append([]string(nil), r.current.Models...)
	current.ModelConfigs = append([]SharedPoolModelConfig(nil), r.current.ModelConfigs...)
	return &current, nil
}

func (r *sharedPoolUpdateListingRepoStub) ListModelCatalog(context.Context) ([]ModelCatalogEntry, error) {
	return append([]ModelCatalogEntry(nil), r.catalog...), r.catalogErr
}

func (r *sharedPoolUpdateListingRepoStub) ListSharedPoolAccounts(context.Context, int64, int64) ([]SharedPoolAccount, error) {
	return append([]SharedPoolAccount(nil), r.accounts...), nil
}

func (r *sharedPoolUpdateListingRepoStub) GetSharedPoolUpstreamRuntime(context.Context, int64, int64) (*SharedPoolUpstreamRuntime, error) {
	r.runtimeReads++
	return &SharedPoolUpstreamRuntime{
		UpstreamBaseURL: "http://127.0.0.1:1/v1",
		UpstreamAPIKey:  "stored-secret",
		ProbeModel:      "custom-research-model",
	}, nil
}

func (r *sharedPoolUpdateListingRepoStub) UpdateSharedPoolTx(_ context.Context, poolID int64, _ int64, input UpdateSharedPoolInput) (*SharedPool, error) {
	copyInput := input
	r.updatedInput = &copyInput
	return &SharedPool{
		ID:                 poolID,
		Name:               input.Name,
		Status:             input.Status,
		Listed:             input.Listed,
		AccountModeEnabled: input.AccountModeEnabled,
		Models:             append([]string(nil), input.Models...),
	}, nil
}

func sharedPoolUpdateFixture(listed, accountMode bool) *SharedPool {
	return &SharedPool{
		ID:                          2002,
		Name:                        "Existing pool",
		Description:                 "Existing description",
		AvatarURL:                   "https://assets.invalid/pool.png",
		StatusNote:                  "Available",
		Status:                      "healthy",
		Listed:                      listed,
		Models:                      []string{"custom-research-model"},
		ModelConfigs:                []SharedPoolModelConfig{{Provider: "", ModelName: "custom-research-model", RateMultiplier: 1, ModelOpen: true}},
		RateMultiplier:              1,
		MaxUsers:                    20,
		MinBalanceAdmission:         1,
		HourlySeatFee:               0.1,
		HourlyMinUsageWaiver:        1,
		UpstreamBaseURL:             "http://127.0.0.1:1/v1",
		ProxyURL:                    "http://proxy.invalid:8080",
		ProxyRegion:                 "test",
		ProxyStatus:                 "active",
		AccountConcurrency:          1,
		UserConcurrency:             1,
		AccountModeEnabled:          accountMode,
		OAuthProvider:               "",
		VerificationMode:            "",
		VerificationExemptionReason: "",
	}
}

func TestUpdateSharedPoolKeepsListedForMetadataAndCommercialChanges(t *testing.T) {
	proxyID := int64(77)
	current := sharedPoolUpdateFixture(true, false)
	current.ProxyID = &proxyID
	current.ConfigVersion = 17
	repo := &sharedPoolUpdateListingRepoStub{current: current}
	svc := NewBizDecipherService(repo, nil, nil)

	pool, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
		Name:             "Renamed pool",
		NameSet:          true,
		HourlySeatFee:    0.25,
		HourlySeatFeeSet: true,
	})
	if err != nil {
		t.Fatalf("UpdateSharedPool returned error: %v", err)
	}
	if pool == nil || !pool.Listed {
		t.Fatalf("metadata-only save must keep the pool listed, got %#v", pool)
	}
	if repo.runtimeReads != 0 {
		t.Fatalf("metadata-only save must not read probe credentials, got %d reads", repo.runtimeReads)
	}
	if repo.updatedInput == nil {
		t.Fatal("expected repository update")
	}
	if repo.updatedInput.ExpectedConfigVersion != current.ConfigVersion {
		t.Fatalf("expected config version fence %d, got %d", current.ConfigVersion, repo.updatedInput.ExpectedConfigVersion)
	}
	if repo.updatedInput.ModelsSet || repo.updatedInput.ModelConfigsSet {
		t.Fatalf("metadata-only save must not request model rewrites: %#v", repo.updatedInput)
	}
	if repo.updatedInput.AvatarURL != current.AvatarURL || repo.updatedInput.StatusNote != current.StatusNote {
		t.Fatalf("omitted branding fields were not preserved: %#v", repo.updatedInput)
	}
	if !sameOptionalInt64(repo.updatedInput.ProxyID, current.ProxyID) || repo.updatedInput.ProxyURL != current.ProxyURL {
		t.Fatalf("omitted proxy fields were not preserved: %#v", repo.updatedInput)
	}
	if repo.updatedInput.OAuthProvider != "openai" || repo.updatedInput.VerificationMode != "full_check" {
		t.Fatalf("expected effective defaults to persist, got oauth=%q verification=%q", repo.updatedInput.OAuthProvider, repo.updatedInput.VerificationMode)
	}
}

func TestUpdateSharedPoolForwardsClientConfigVersion(t *testing.T) {
	current := sharedPoolUpdateFixture(true, false)
	current.ConfigVersion = 18
	repo := &sharedPoolUpdateListingRepoStub{current: current}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
		ExpectedConfigVersion: 17,
		Name:                  "stale page save",
		NameSet:               true,
	})
	if err != nil {
		t.Fatalf("UpdateSharedPool returned error: %v", err)
	}
	if repo.updatedInput == nil || repo.updatedInput.ExpectedConfigVersion != 17 {
		t.Fatalf("client config version must reach the repository unchanged: %#v", repo.updatedInput)
	}
}

func TestUpdateSharedPoolUnlistsWhenGateConfigurationChanges(t *testing.T) {
	newProxyID := int64(88)
	tests := []struct {
		name  string
		patch UpdateSharedPoolInput
	}{
		{name: "upstream URL", patch: UpdateSharedPoolInput{UpstreamBaseURL: "https://changed.invalid/v1", UpstreamBaseURLSet: true}},
		{name: "upstream key", patch: UpdateSharedPoolInput{UpstreamAPIKey: "new-secret", UpstreamAPIKeySet: true}},
		{name: "proxy", patch: UpdateSharedPoolInput{ProxyID: &newProxyID, ProxyIDSet: true}},
		{name: "model routing", patch: UpdateSharedPoolInput{
			Models:          []string{"custom-next-model"},
			ModelsSet:       true,
			ModelConfigs:    []SharedPoolModelInput{{Provider: "custom", ModelName: "custom-next-model", ModelOpen: true}},
			ModelConfigsSet: true,
		}},
		{name: "verification policy", patch: UpdateSharedPoolInput{
			VerificationMode:            "professional_review",
			VerificationModeSet:         true,
			VerificationExemptionReason: "Custom model reviewed by the operator.",
			VerificationReasonSet:       true,
		}},
		{name: "account mode", patch: UpdateSharedPoolInput{AccountModeEnabled: true, AccountModeSet: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := sharedPoolUpdateFixture(true, false)
			repo := &sharedPoolUpdateListingRepoStub{current: current}
			svc := NewBizDecipherService(repo, nil, nil)

			pool, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, test.patch)
			if err != nil {
				t.Fatalf("UpdateSharedPool returned error: %v", err)
			}
			if pool == nil || pool.Listed {
				t.Fatalf("gate change must automatically unlist the pool, got %#v", pool)
			}
			if repo.updatedInput == nil || repo.updatedInput.Listed {
				t.Fatalf("expected listed=false persisted, got %#v", repo.updatedInput)
			}
			if repo.runtimeReads != 0 {
				t.Fatalf("automatic unlisting must not synchronously probe, got %d reads", repo.runtimeReads)
			}
		})
	}
}

func TestUpdateSharedPoolAllowsAccountModeListingAfterGatePassed(t *testing.T) {
	current := sharedPoolUpdateFixture(false, true)
	repo := &sharedPoolUpdateListingRepoStub{
		current: current,
		accounts: []SharedPoolAccount{{
			ID:              1,
			AuthType:        AccountTypeAPIKey,
			UpstreamBaseURL: "https://routing.example.invalid/v1",
			HasUpstreamKey:  true,
			GateRequired:    true,
			GatePassed:      true,
			Schedulable:     true,
			Status:          "active",
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	pool, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{Listed: true, ListedSet: true})
	if err != nil {
		t.Fatalf("UpdateSharedPool returned error: %v", err)
	}
	if pool == nil || !pool.Listed {
		t.Fatalf("expected listed account-mode pool, got %#v", pool)
	}
	if repo.updatedInput == nil || !repo.updatedInput.Listed {
		t.Fatalf("expected listed=true persisted for account mode after gate pass, got %#v", repo.updatedInput)
	}
	if repo.updatedInput.UpstreamBaseURL != "" || repo.updatedInput.UpstreamAPIKey != "" {
		t.Fatalf("pool credentials should stay empty in account mode, got base=%q key=%q", repo.updatedInput.UpstreamBaseURL, repo.updatedInput.UpstreamAPIKey)
	}
}

func TestUpdateSharedPoolBlocksAccountModeListingWithoutGatePass(t *testing.T) {
	current := sharedPoolUpdateFixture(false, true)
	repo := &sharedPoolUpdateListingRepoStub{
		current: current,
		accounts: []SharedPoolAccount{{
			ID:           1,
			GateRequired: true,
			GatePassed:   false,
			Schedulable:  true,
			Status:       "testing",
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{Listed: true, ListedSet: true})
	if err == nil || !strings.Contains(err.Error(), "full capability check") {
		t.Fatalf("expected listing blocked before gate pass, got %v", err)
	}
	if repo.updatedInput != nil {
		t.Fatalf("repository should not update when listing is blocked: %#v", repo.updatedInput)
	}
}

func TestUpdateSharedPoolAllowsProfessionalReviewListingWithSchedulableAccount(t *testing.T) {
	current := sharedPoolUpdateFixture(false, true)
	repo := &sharedPoolUpdateListingRepoStub{
		current: current,
		accounts: []SharedPoolAccount{{
			ID:              1,
			AuthType:        AccountTypeAPIKey,
			UpstreamBaseURL: "https://routing.example.invalid/v1",
			HasUpstreamKey:  true,
			GateRequired:    false,
			GatePassed:      false,
			Schedulable:     true,
			Status:          "active",
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	pool, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
		Listed:                      true,
		ListedSet:                   true,
		VerificationMode:            "professional_review",
		VerificationModeSet:         true,
		VerificationExemptionReason: "Custom model reviewed by the operator.",
		VerificationReasonSet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateSharedPool returned error: %v", err)
	}
	if pool == nil || !pool.Listed {
		t.Fatalf("expected listed professional review pool, got %#v", pool)
	}
	if repo.updatedInput == nil || repo.updatedInput.VerificationMode != "professional_review" {
		t.Fatalf("expected professional review mode persisted, got %#v", repo.updatedInput)
	}
}

func TestUpdateSharedPoolBlocksListingWhenOnlyGateSnapshotIsStale(t *testing.T) {
	current := sharedPoolUpdateFixture(false, true)
	// This account used to pass the gate, but no longer has a usable route.
	// Listing must not rely on the historical gate_passed bit alone.
	repo := &sharedPoolUpdateListingRepoStub{
		current: current,
		accounts: []SharedPoolAccount{{
			ID:           1,
			AuthType:     AccountTypeAPIKey,
			GateRequired: true,
			GatePassed:   true,
			Schedulable:  true,
			Status:       "active",
			// Missing upstream URL/key is the current routing failure.
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{Listed: true, ListedSet: true})
	if err == nil || !strings.Contains(err.Error(), "full capability check") {
		t.Fatalf("expected stale gate snapshot to be rejected, got %v", err)
	}
	if repo.updatedInput != nil {
		t.Fatalf("repository should not update when no account is currently routable: %#v", repo.updatedInput)
	}
}

func TestUpdateSharedPoolBlocksListingForExpiredOAuthAccount(t *testing.T) {
	current := sharedPoolUpdateFixture(false, true)
	expired := time.Now().Add(-time.Minute)
	repo := &sharedPoolUpdateListingRepoStub{
		current: current,
		accounts: []SharedPoolAccount{{
			ID:                  1,
			AuthType:            AccountTypeOAuth,
			HasOAuthCredentials: true,
			ExpiresAt:           &expired,
			AutoPauseOnExpired:  true,
			GateRequired:        true,
			GatePassed:          true,
			Schedulable:         true,
			Status:              "active",
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{Listed: true, ListedSet: true})
	if err == nil || !strings.Contains(err.Error(), "full capability check") {
		t.Fatalf("expected expired account to be rejected, got %v", err)
	}
	if repo.updatedInput != nil {
		t.Fatalf("repository should not update for expired account: %#v", repo.updatedInput)
	}
}

func TestUpdateSharedPoolProfessionalReviewDoesNotRunSynchronousProbe(t *testing.T) {
	current := sharedPoolUpdateFixture(false, false)
	repo := &sharedPoolUpdateListingRepoStub{current: current}
	svc := NewBizDecipherService(repo, nil, nil)

	pool, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
		Listed:                      true,
		ListedSet:                   true,
		VerificationMode:            "professional_review",
		VerificationModeSet:         true,
		VerificationExemptionReason: "Custom model reviewed by the operator.",
		VerificationReasonSet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateSharedPool returned error: %v", err)
	}
	if pool == nil || !pool.Listed {
		t.Fatalf("expected listed professional review pool, got %#v", pool)
	}
	if repo.runtimeReads != 0 {
		t.Fatalf("professional review must not read credentials for a synchronous probe, got %d reads", repo.runtimeReads)
	}
}

func TestUpdateSharedPoolFullCheckListingRequiresBackgroundProbe(t *testing.T) {
	current := sharedPoolUpdateFixture(false, false)
	repo := &sharedPoolUpdateListingRepoStub{current: current}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
		Listed: true, ListedSet: true,
	})
	if !errors.Is(err, ErrSharedPoolProbeRequired) {
		t.Fatalf("expected background probe requirement, got %v", err)
	}
	if repo.updatedInput != nil || repo.runtimeReads != 0 {
		t.Fatalf("listing request must not update or synchronously read credentials: input=%#v reads=%d", repo.updatedInput, repo.runtimeReads)
	}
}

func TestUpdateSharedPoolBlocksOwnerListingUnderGovernance(t *testing.T) {
	for _, governanceStatus := range []string{"watch", "suppressed", "banned"} {
		t.Run(governanceStatus, func(t *testing.T) {
			current := sharedPoolUpdateFixture(false, false)
			current.GovernanceStatus = governanceStatus
			repo := &sharedPoolUpdateListingRepoStub{current: current}
			svc := NewBizDecipherService(repo, nil, nil)

			_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
				Listed:                      true,
				ListedSet:                   true,
				VerificationMode:            "professional_review",
				VerificationModeSet:         true,
				VerificationExemptionReason: "custom model reviewed",
				VerificationReasonSet:       true,
			})
			if !errors.Is(err, ErrSharedPoolGovernanceBlocked) {
				t.Fatalf("expected governance rejection, got %v", err)
			}
			if repo.updatedInput != nil {
				t.Fatalf("governance-blocked listing must not update: %#v", repo.updatedInput)
			}
		})
	}
}

func TestUpdateSharedPoolProfessionalReviewRejectsMainstreamUpstreamAlias(t *testing.T) {
	current := sharedPoolUpdateFixture(false, false)
	current.ModelConfigs[0].UpstreamModelName = "gpt-5.6"
	repo := &sharedPoolUpdateListingRepoStub{
		current: current,
		catalog: []ModelCatalogEntry{{Provider: "openai", ModelName: "gpt-5.6", Mainstream: true}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateSharedPool(context.Background(), current.ID, 42, UpdateSharedPoolInput{
		Listed:                      true,
		ListedSet:                   true,
		VerificationMode:            "professional_review",
		VerificationModeSet:         true,
		VerificationExemptionReason: "claimed custom model",
		VerificationReasonSet:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "mainstream models must pass full verification") {
		t.Fatalf("expected mainstream alias rejection, got %v", err)
	}
	if repo.updatedInput != nil {
		t.Fatalf("repository must not update disguised mainstream listing: %#v", repo.updatedInput)
	}
}
