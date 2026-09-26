package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestSub2ThresholdOpenAIWindowDisableAndReset(t *testing.T) {
	now := time.Now()
	account := &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{accountSchedulingThresholdCredentialKey: 60},
		Extra: map[string]any{
			"codex_5h_used_percent": 80.0, "codex_5h_reset_at": now.Add(time.Hour).Format(time.RFC3339),
			"codex_usage_updated_at": now.Format(time.RFC3339), "auto_pause_5h_disabled": true,
		},
	}
	require.True(t, account.IsSchedulable(), "existing per-window disable remains authoritative")
	account.Extra["auto_pause_5h_disabled"] = false
	require.False(t, account.IsSchedulable())
	account.Credentials[accountSchedulingThresholdCredentialKey] = 100
	account.Extra["auto_pause_5h_threshold"] = 50.0
	require.True(t, account.IsSchedulable())
	paused, _ := shouldAutoPauseOpenAIAccountByQuota(context.Background(), account)
	require.False(t, paused, "explicit disabled override must not fall through to legacy threshold")
	account.Credentials[accountSchedulingThresholdCredentialKey] = 60
	account.Extra["codex_5h_reset_at"] = now.Add(-time.Second).Format(time.RFC3339)
	require.True(t, account.IsSchedulable())
	account.Extra["codex_5h_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
	account.Extra["codex_5h_used_percent"] = math.Inf(1)
	require.True(t, account.IsSchedulable(), "invalid utilization cannot pause an account")
}

func TestSub2ThresholdGrokUsesLocalSnapshotAndLatestReset(t *testing.T) {
	now := time.Now()
	limit, remaining := int64(100), int64(20)
	early, late := now.Add(time.Hour).Unix(), now.Add(2*time.Hour).Unix()
	snapshot := &xai.QuotaSnapshot{
		UpdatedAt: now.Format(time.RFC3339), StatusCode: 200,
		Requests: &xai.QuotaWindow{Limit: &limit, Remaining: &remaining, ResetUnix: &early},
		Tokens:   &xai.QuotaWindow{Limit: &limit, Remaining: &remaining, ResetUnix: &late},
	}
	account := &Account{
		Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{accountSchedulingThresholdCredentialKey: 60},
		Extra: map[string]any{grokQuotaSnapshotExtraKey: snapshot,
			"grok_sched_utilization": 99.0, "grok_sched_reset_at": now.Add(time.Hour).Format(time.RFC3339)},
	}
	decision := EvaluateAccountSchedulingThreshold(account, nil, now)
	require.True(t, decision.ShouldPause)
	require.Equal(t, "tokens", decision.Window)
	require.Equal(t, late, decision.Until.Unix())
	remaining = 90
	require.True(t, account.IsSchedulable(), "fresh local snapshot wins over stale projected threshold")
	remaining = 20
	snapshot.UpdatedAt = now.Add(-48 * time.Hour).Format(time.RFC3339)
	require.True(t, account.IsSchedulable(), "expired local snapshot cannot pause")
}
