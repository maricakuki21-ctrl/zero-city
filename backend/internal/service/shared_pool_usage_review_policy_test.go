package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type sharedPoolReviewPolicySettingStub struct {
	values  map[string]string
	updates map[string]string
}

func (s *sharedPoolReviewPolicySettingStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *sharedPoolReviewPolicySettingStub) SetMultiple(_ context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for key, value := range settings {
		s.updates[key] = value
		s.values[key] = value
	}
	return nil
}

func TestSharedPoolUsageReviewPolicyUsesConservativeDefaults(t *testing.T) {
	svc := NewBizDecipherService(nil, nil, &sharedPoolReviewPolicySettingStub{values: map[string]string{}})

	policy, err := svc.GetSharedPoolUsageReviewPolicy(context.Background())
	require.NoError(t, err)
	require.True(t, policy.AutoReleaseEnabled)
	require.Equal(t, 15, policy.AutoReleaseMinutes)
	require.InDelta(t, 0.01, policy.AutoReleaseMaxHold, 1e-12)
}

func TestAdminUpdateSharedPoolUsageReviewPolicyPersistsAtomically(t *testing.T) {
	settings := &sharedPoolReviewPolicySettingStub{values: map[string]string{}}
	svc := NewBizDecipherService(nil, nil, settings)

	policy, err := svc.AdminUpdateSharedPoolUsageReviewPolicy(context.Background(), UpdateSharedPoolUsageReviewPolicyInput{
		AutoReleaseEnabled: true,
		AutoReleaseMinutes: 120,
		AutoReleaseMaxHold: 0.025,
	})
	require.NoError(t, err)
	require.Equal(t, 120, policy.AutoReleaseMinutes)
	require.InDelta(t, 0.025, policy.AutoReleaseMaxHold, 1e-12)
	require.Equal(t, "true", settings.updates[SettingKeySharedPoolReviewAutoReleaseEnabled])
	require.Equal(t, "120", settings.updates[SettingKeySharedPoolReviewAutoReleaseMinutes])
	require.Equal(t, "0.02500000", settings.updates[SettingKeySharedPoolReviewAutoReleaseMaxHold])
}

func TestSharedPoolUsageReviewPolicyRejectsInvalidStoredValueInsteadOfAutoReleasing(t *testing.T) {
	svc := NewBizDecipherService(nil, nil, &sharedPoolReviewPolicySettingStub{values: map[string]string{
		SettingKeySharedPoolReviewAutoReleaseMinutes: "0",
	}})

	_, err := svc.GetSharedPoolUsageReviewPolicy(context.Background())
	require.Error(t, err)
}

func TestSharedPoolUsageReviewPolicyRejectsUnsafeAutoReleaseAmount(t *testing.T) {
	settings := &sharedPoolReviewPolicySettingStub{values: map[string]string{}}
	svc := NewBizDecipherService(nil, nil, settings)

	_, err := svc.AdminUpdateSharedPoolUsageReviewPolicy(context.Background(), UpdateSharedPoolUsageReviewPolicyInput{
		AutoReleaseEnabled: true,
		AutoReleaseMinutes: 15,
		AutoReleaseMaxHold: 0.11,
	})
	require.Error(t, err)
	require.Empty(t, settings.updates)
}
