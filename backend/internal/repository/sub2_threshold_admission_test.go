//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSub2ThresholdSnapshotAdmissionAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		shared, fable           float64
		threshold               int
		blockShared, blockFable bool
	}{
		{"shared", .7, .8, 60, true, true},
		{"fable", .3, .8, 60, false, true},
		{"below", .3, .4, 60, false, false},
		{"disabled", .9, .9, 100, false, false},
		{"unset", .9, .9, 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			end := time.Now().Add(time.Hour).Truncate(time.Second)
			account := service.Account{
				ID: 913, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
				Status: service.StatusActive, Schedulable: true, SessionWindowEnd: &end,
				Credentials: map[string]any{"account_scheduling_threshold": tc.threshold, "access_token": "do-not-project"},
				Extra: map[string]any{
					"session_window_utilization":      tc.shared,
					"passive_usage_7d_oi_utilization": tc.fable,
					"passive_usage_7d_oi_reset":       end.Unix(),
					"irrelevant":                      "do-not-project",
				},
			}
			cache := newSchedulerCacheUnit(t)
			bucket := service.SchedulerBucket{GroupID: 8, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
			token, err := cache.CaptureBucketWriteToken(ctx, bucket)
			require.NoError(t, err)
			require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
			candidates, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, candidates, 1)
			require.NotContains(t, candidates[0].Credentials, "access_token")
			require.NotContains(t, candidates[0].Extra, "irrelevant")
			full, err := cache.GetAccount(ctx, account.ID)
			require.NoError(t, err)
			for _, candidate := range []*service.Account{full, candidates[0]} {
				require.NotNil(t, candidate)
				require.Equal(t, !tc.blockShared, candidate.IsSchedulable())
				require.Equal(t, !tc.blockShared, candidate.IsSchedulableForModel("claude-sonnet-4"))
				require.Equal(t, !tc.blockFable, candidate.IsSchedulableForModel("claude-fable-5"))
				require.Nil(t, candidate.TempUnschedulableUntil, "threshold must not persist a freeze")
			}
			account.Extra["session_window_utilization"] = .1
			account.Extra["passive_usage_7d_oi_utilization"] = .1
			require.NoError(t, cache.SetAccount(ctx, &account))
			recovered, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.True(t, recovered[0].IsSchedulableForModel("claude-fable-5"))
		})
	}
}

func TestSub2ThresholdSnapshotPreservesOpenAIIdentity(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	account := service.Account{
		ID: 914, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"account_scheduling_threshold": 60, "chatgpt_account_id": "current"},
		Extra: map[string]any{
			"chatgpt_account_id": "old-account", "codex_5h_used_percent": 80.0,
			"codex_5h_reset_at":      now.Add(time.Hour).Format(time.RFC3339),
			"codex_usage_updated_at": now.Format(time.RFC3339),
		},
	}
	cache := newSchedulerCacheUnit(t)
	bucket := service.SchedulerBucket{GroupID: 8, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
	candidates, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.True(t, candidates[0].IsSchedulable(), "stale identity must not pause the replacement account")
	account.Extra["chatgpt_account_id"] = "current"
	require.NoError(t, cache.SetAccount(ctx, &account))
	candidates, _, err = cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.False(t, candidates[0].IsSchedulable())
}
