package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type sharedPoolCreateRepoStub struct {
	BizDecipherRepository
	createdInput *CreateSharedPoolInput
	catalog      []ModelCatalogEntry
	catalogErr   error
}

func (r *sharedPoolCreateRepoStub) ListModelCatalog(context.Context) ([]ModelCatalogEntry, error) {
	return append([]ModelCatalogEntry(nil), r.catalog...), r.catalogErr
}

func (r *sharedPoolCreateRepoStub) CreateSharedPoolTx(_ context.Context, input CreateSharedPoolInput) (*SharedPool, error) {
	copyInput := input
	r.createdInput = &copyInput
	return &SharedPool{
		ID:                 1001,
		Name:               input.Name,
		Status:             input.Status,
		Listed:             input.Listed,
		UpstreamBaseURL:    input.UpstreamBaseURL,
		HasUpstreamKey:     strings.TrimSpace(input.UpstreamAPIKey) != "",
		AccountModeEnabled: input.AccountModeEnabled,
		PlatformFeePercent: input.PlatformFeePercent,
		Models:             append([]string(nil), input.Models...),
	}, nil
}

type sharedPoolCreateSettingStub struct {
	value string
	err   error
	keys  []string
}

func (s *sharedPoolCreateSettingStub) GetValue(_ context.Context, key string) (string, error) {
	s.keys = append(s.keys, key)
	return s.value, s.err
}

func TestCreateSharedPoolUsesConfiguredDefaultPlatformFeePercent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want float64
	}{
		{name: "configured", raw: "5", want: 5},
		{name: "missing value", raw: "", want: 0},
		{name: "out of range legacy value", raw: "101", want: 0},
		{name: "non finite legacy value", raw: "NaN", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &sharedPoolCreateRepoStub{}
			settings := &sharedPoolCreateSettingStub{value: tt.raw}
			svc := NewBizDecipherService(repo, nil, settings)

			pool, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
				OwnerID:            42,
				Name:               "default fee pool",
				AccountModeEnabled: true,
				Status:             "healthy",
			})
			if err != nil {
				t.Fatalf("CreateSharedPool returned error: %v", err)
			}
			if repo.createdInput == nil {
				t.Fatal("expected repository create input")
			}
			if repo.createdInput.PlatformFeePercent != tt.want {
				t.Fatalf("created platform fee = %v, want %v", repo.createdInput.PlatformFeePercent, tt.want)
			}
			if pool == nil || pool.PlatformFeePercent != tt.want {
				t.Fatalf("returned pool platform fee = %#v, want %v", pool, tt.want)
			}
			if len(settings.keys) != 1 || settings.keys[0] != SettingKeySharedPoolDefaultPlatformFeePercent {
				t.Fatalf("unexpected setting reads: %#v", settings.keys)
			}
		})
	}
}

func TestCreateSharedPoolFailsWhenConfiguredDefaultCannotBeRead(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{}
	settings := &sharedPoolCreateSettingStub{err: errors.New("settings unavailable")}
	svc := NewBizDecipherService(repo, nil, settings)

	_, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "strict default fee pool",
		AccountModeEnabled: true,
		Status:             "healthy",
	})
	if err == nil || !strings.Contains(err.Error(), "load shared pool default platform fee percent") {
		t.Fatalf("expected setting read failure, got %v", err)
	}
	if repo.createdInput != nil {
		t.Fatalf("repository must not create with a stale fallback: %#v", repo.createdInput)
	}
}

func TestCreateSharedPoolUsesCompatibilityDefaultWhenSettingIsMissing(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{}
	settings := &sharedPoolCreateSettingStub{err: ErrSettingNotFound}
	svc := NewBizDecipherService(repo, nil, settings)

	_, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "compatibility default fee pool",
		AccountModeEnabled: true,
		Status:             "healthy",
	})
	if err != nil {
		t.Fatalf("CreateSharedPool returned error: %v", err)
	}
	if repo.createdInput == nil || repo.createdInput.PlatformFeePercent != 0 {
		t.Fatalf("expected compatibility platform fee, got %#v", repo.createdInput)
	}
}

func TestCreateSharedPoolAllowsOAuthOnlyAccountModeWithoutPoolCredentials(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	pool, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "OAuth only control plane",
		UpstreamBaseURL:    "https://should-not-be-saved.example.com/v1",
		UpstreamAPIKey:     "fixture-key-should-not-be-saved",
		AccountModeEnabled: true,
		Listed:             true,
		Status:             "healthy",
	})
	if err != nil {
		t.Fatalf("CreateSharedPool returned error: %v", err)
	}
	if pool == nil || !pool.AccountModeEnabled {
		t.Fatalf("expected account mode pool, got %#v", pool)
	}
	if repo.createdInput == nil {
		t.Fatal("expected repository create input")
	}
	if repo.createdInput.UpstreamBaseURL != "" || repo.createdInput.UpstreamAPIKey != "" {
		t.Fatalf("pool credentials should be cleared for account mode, got base=%q key=%q", repo.createdInput.UpstreamBaseURL, repo.createdInput.UpstreamAPIKey)
	}
	if repo.createdInput.Listed {
		t.Fatal("OAuth-only account-mode control plane must not be listed before account probes pass")
	}
	if len(repo.createdInput.ModelConfigs) != 0 || len(repo.createdInput.Models) != 0 || repo.createdInput.ProbeModel != "" {
		t.Fatalf("expected empty model/probe state before account import, got models=%#v configs=%#v probe=%q", repo.createdInput.Models, repo.createdInput.ModelConfigs, repo.createdInput.ProbeModel)
	}
}

func TestCreateSharedPoolSingleCredentialStillRequiresPoolAPIKey(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "single credential pool",
		UpstreamBaseURL:    "https://api.example.com/v1",
		AccountModeEnabled: false,
		Models:             []string{"gpt-fixture"},
		ProbeModel:         "gpt-fixture",
	})
	if err == nil || !strings.Contains(err.Error(), "upstream api key is required") {
		t.Fatalf("expected missing api key error, got %v", err)
	}
	if repo.createdInput != nil {
		t.Fatalf("repository should not be called when single credential key is missing: %#v", repo.createdInput)
	}
}

func TestCreateSharedPoolProfessionalReviewRequiresExemptionReason(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "non-mainstream reviewed pool",
		AccountModeEnabled: false,
		UpstreamBaseURL:    "https://api.example.com/v1",
		UpstreamAPIKey:     "fixture-key",
		Models:             []string{"custom-research-model"},
		ProbeModel:         "custom-research-model",
		ModelConfigs: []SharedPoolModelInput{{
			ModelName: "custom-research-model",
			ModelOpen: true,
			Provider:  "custom",
		}},
		VerificationMode: "professional_review",
	})
	if err == nil || !strings.Contains(err.Error(), "verification exemption reason is required") {
		t.Fatalf("expected professional review reason error, got %v", err)
	}
}

func TestCreateSharedPoolProfessionalReviewRejectsMainstreamUpstreamAlias(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{catalog: []ModelCatalogEntry{{
		Provider: "openai", ModelName: "gpt-5.6", Mainstream: true,
	}}}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "disguised mainstream pool",
		AccountModeEnabled: false,
		UpstreamBaseURL:    "https://api.example.com/v1",
		UpstreamAPIKey:     "fixture-key",
		Models:             []string{"my-custom"},
		ProbeModel:         "my-custom",
		ModelConfigs: []SharedPoolModelInput{{
			Provider: "custom", ModelName: "my-custom", UpstreamModelName: "gpt-5.6", ModelOpen: true,
		}},
		VerificationMode:            "professional_review",
		VerificationExemptionReason: "claimed custom model",
	})
	if err == nil || !strings.Contains(err.Error(), "mainstream models must pass full verification") {
		t.Fatalf("expected mainstream alias rejection, got %v", err)
	}
	if repo.createdInput != nil {
		t.Fatalf("repository must not create a disguised mainstream pool: %#v", repo.createdInput)
	}
}

func TestResolveSharedPoolProfessionalReviewFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		repo    *sharedPoolCreateRepoStub
		configs []SharedPoolModelInput
		want    string
	}{
		{
			name: "no open models",
			repo: &sharedPoolCreateRepoStub{},
			want: "requires at least one open model",
		},
		{
			name:    "catalog unavailable",
			repo:    &sharedPoolCreateRepoStub{catalogErr: errors.New("catalog unavailable")},
			configs: []SharedPoolModelInput{{ModelName: "custom-research", ModelOpen: true}},
			want:    "model catalog is unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewBizDecipherService(tt.repo, nil, nil)
			_, _, err := svc.resolveSharedPoolVerificationPolicy(
				context.Background(), tt.configs, "professional_review", "reviewed",
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCreateSharedPoolFullCheckStartsAsDraftWithoutSynchronousProbe(t *testing.T) {
	repo := &sharedPoolCreateRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	pool, err := svc.CreateSharedPool(context.Background(), CreateSharedPoolInput{
		OwnerID:            42,
		Name:               "draft before probe",
		AccountModeEnabled: false,
		UpstreamBaseURL:    "http://127.0.0.1:1/v1",
		UpstreamAPIKey:     "fixture-key",
		Models:             []string{"custom-research-model"},
		ProbeModel:         "custom-research-model",
		ModelConfigs: []SharedPoolModelInput{{
			ModelName: "custom-research-model", ModelOpen: true,
		}},
		VerificationMode: "full_check",
		Listed:           true,
		Status:           "healthy",
	})
	if err != nil {
		t.Fatalf("CreateSharedPool returned error: %v", err)
	}
	if pool == nil || pool.Listed || repo.createdInput == nil || repo.createdInput.Listed {
		t.Fatalf("full-check creation must remain a draft until the background probe passes: pool=%#v input=%#v", pool, repo.createdInput)
	}
}
