//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingServiceSharedPoolDefaultPlatformFeePercent(t *testing.T) {
	t.Run("missing value keeps compatibility default", func(t *testing.T) {
		svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{}}, &config.Config{})

		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, SharedPoolPlatformFeePercentDefault, settings.SharedPoolDefaultPlatformFeePercent)
	})

	t.Run("configured value is parsed", func(t *testing.T) {
		svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{
			SettingKeySharedPoolDefaultPlatformFeePercent: "5",
		}}, &config.Config{})

		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, 5.0, settings.SharedPoolDefaultPlatformFeePercent)
	})

	for _, raw := range []string{"-1", "101", "NaN", "invalid"} {
		t.Run("invalid value "+raw+" uses compatibility default", func(t *testing.T) {
			svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{
				SettingKeySharedPoolDefaultPlatformFeePercent: raw,
			}}, &config.Config{})

			settings, err := svc.GetAllSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, SharedPoolPlatformFeePercentDefault, settings.SharedPoolDefaultPlatformFeePercent)
		})
	}

	t.Run("value is persisted", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			SharedPoolDefaultPlatformFeePercent: 5,
		})
		require.NoError(t, err)
		require.Equal(t, "5.00000000", repo.updates[SettingKeySharedPoolDefaultPlatformFeePercent])
	})
}
