package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestSub2GlobalThresholdPersistenceAndAdmission(t *testing.T) {
	ctx := context.Background()
	repo := newRuntimeSettingRepoStub()
	settings := NewSettingService(repo, &config.Config{})
	ops := &OpsService{settingRepo: repo}
	ops.SetOpenAIQuotaAutoPauseSettingsSink(settings.SetOpenAIQuotaAutoPauseSettings)
	cfg := defaultOpsAdvancedSettings()
	cfg.OpenAIAccountQuotaAutoPause.AnthropicThreshold = 60
	cfg.OpenAIAccountQuotaAutoPause.GrokThreshold = 70
	saved, err := ops.UpdateOpsAdvancedSettings(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, 60, saved.OpenAIAccountQuotaAutoPause.AnthropicThreshold)
	restarted := NewSettingService(repo, &config.Config{})
	require.Equal(t, 70, restarted.WarmOpenAIQuotaAutoPauseSettings(ctx).GrokThreshold)
	gateway := &GatewayService{settingService: settings}
	now := time.Now()
	end := now.Add(time.Hour)
	account := &Account{
		Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		SessionWindowEnd: &end, Extra: map[string]any{"session_window_utilization": .75},
	}
	require.False(t, gateway.isAccountSchedulableForModelSelection(ctx, account, "claude-sonnet-4"))
	account.Credentials = map[string]any{accountSchedulingThresholdCredentialKey: 100}
	require.True(t, gateway.isAccountSchedulableForModelSelection(ctx, account, "claude-sonnet-4"))
	delete(account.Credentials, accountSchedulingThresholdCredentialKey)
	account.Extra["session_window_utilization"] = .1
	account.Extra["passive_usage_7d_oi_utilization"] = .8
	account.Extra["passive_usage_7d_oi_reset"] = end.Unix()
	require.True(t, gateway.isAccountSchedulableForModelSelection(ctx, account, "claude-sonnet-4"))
	require.False(t, gateway.isAccountSchedulableForModelSelection(ctx, account, "claude-fable-5"))

	limit, remaining, reset := int64(100), int64(20), end.Unix()
	grok := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{grokQuotaSnapshotExtraKey: &xai.QuotaSnapshot{
			UpdatedAt: now.Format(time.RFC3339), StatusCode: 200,
			Requests: &xai.QuotaWindow{Limit: &limit, Remaining: &remaining, ResetUnix: &reset},
		}}}
	openai := &OpenAIGatewayService{settingService: settings}
	require.False(t, grok.IsSchedulableForModelWithContext(openai.withOpenAIQuotaAutoPauseContext(ctx), "grok"))
	cfg.OpenAIAccountQuotaAutoPause.AnthropicThreshold = 100
	cfg.OpenAIAccountQuotaAutoPause.GrokThreshold = 100
	_, err = ops.UpdateOpsAdvancedSettings(ctx, cfg)
	require.NoError(t, err)
	require.True(t, gateway.isAccountSchedulableForModelSelection(ctx, account, "claude-fable-5"))
	require.True(t, grok.IsSchedulableForModelWithContext(openai.withOpenAIQuotaAutoPauseContext(ctx), "grok"))
	require.Nil(t, account.TempUnschedulableUntil)
}

func TestSub2GlobalThresholdLegacyAndInvalidSettings(t *testing.T) {
	cfg := defaultOpsAdvancedSettings()
	normalizeOpsAdvancedSettings(cfg)
	require.Equal(t, 100, cfg.OpenAIAccountQuotaAutoPause.AnthropicThreshold)
	require.Equal(t, 100, cfg.OpenAIAccountQuotaAutoPause.GrokThreshold)
	cfg.OpenAIAccountQuotaAutoPause.GrokThreshold = 101
	require.Error(t, validateOpsAdvancedSettings(cfg))
	cfg.OpenAIAccountQuotaAutoPause.GrokThreshold = -1
	require.Error(t, validateOpsAdvancedSettings(cfg))
	repo := newRuntimeSettingRepoStub()
	repo.values[SettingKeyOpsAdvancedSettings] = `{"openai_account_quota_auto_pause":{"default_threshold_5h":0.9}}`
	legacy := NewSettingService(repo, &config.Config{}).WarmOpenAIQuotaAutoPauseSettings(context.Background())
	require.Equal(t, 100, legacy.AnthropicThreshold)
	require.Equal(t, 100, legacy.GrokThreshold)
	require.Equal(t, .9, legacy.DefaultThreshold5h)
}

type sub2PausedSettingsRead struct {
	SettingRepository
	entered chan struct{}
	resume  chan struct{}
}

func (r *sub2PausedSettingsRead) GetValue(ctx context.Context, _ string) (string, error) {
	close(r.entered)
	select {
	case <-r.resume:
		return `{"openai_account_quota_auto_pause":{"anthropic_threshold":50}}`, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestSub2GlobalThresholdOldRefreshCannotOverwriteSave(t *testing.T) {
	repo := &sub2PausedSettingsRead{entered: make(chan struct{}), resume: make(chan struct{})}
	settings := NewSettingService(repo, &config.Config{})
	done := make(chan struct{})
	go func() { defer close(done); settings.WarmOpenAIQuotaAutoPauseSettings(context.Background()) }()
	select {
	case <-repo.entered:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}
	settings.SetOpenAIQuotaAutoPauseSettings(OpsOpenAIAccountQuotaAutoPauseSettings{AnthropicThreshold: 90})
	close(repo.resume)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresh did not finish")
	}
	require.Equal(t, 90, settings.GetOpenAIQuotaAutoPauseSettings(context.Background()).AnthropicThreshold)
}

type sub2ThresholdAccountRepo struct {
	AccountRepository
	account *Account
}

func (r *sub2ThresholdAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}
func (r *sub2ThresholdAccountRepo) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	return []Account{*r.account}, nil
}

type sub2ThresholdStickyCache struct{ GatewayCache }

func (*sub2ThresholdStickyCache) GetSessionAccountID(context.Context, int64, string) (int64, error) {
	return 19, nil
}
func (*sub2ThresholdStickyCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}
func (*sub2ThresholdStickyCache) SetSessionAccountID(context.Context, int64, string, int64, time.Duration) error {
	return nil
}

func TestSub2GlobalThresholdGeminiCompatibilitySelection(t *testing.T) {
	settings := NewSettingService(nil, &config.Config{})
	settings.SetOpenAIQuotaAutoPauseSettings(OpsOpenAIAccountQuotaAutoPauseSettings{AnthropicThreshold: 60})
	end := time.Now().Add(time.Hour)
	account := &Account{ID: 19, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, SessionWindowEnd: &end,
		Extra: map[string]any{"session_window_utilization": .8}}
	svc := NewGeminiMessagesCompatService(&sub2ThresholdAccountRepo{account: account}, nil,
		&sub2ThresholdStickyCache{}, nil, nil, nil, nil, nil, &config.Config{}, settings)
	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	for _, session := range []string{"", "sticky"} {
		selected, err := svc.SelectAccountForModel(ctx, nil, session, "")
		require.Error(t, err)
		require.Nil(t, selected)
	}
	account.Credentials = map[string]any{accountSchedulingThresholdCredentialKey: 100}
	for _, session := range []string{"", "sticky"} {
		selected, err := svc.SelectAccountForModel(ctx, nil, session, "")
		require.NoError(t, err)
		require.Equal(t, account.ID, selected.ID)
	}
}
