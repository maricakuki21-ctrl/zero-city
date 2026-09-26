package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type marketGuideRepo struct {
	MarketplaceRepository
	userID int64
	intent string
}

func (r *marketGuideRepo) GetMarketplaceOnboarding(_ context.Context, id int64, version int) (*MarketplaceOnboarding, error) {
	r.userID = id
	return &MarketplaceOnboarding{Version: version}, nil
}
func (r *marketGuideRepo) SaveMarketplaceOnboarding(_ context.Context, id int64, version int, intent string) (*MarketplaceOnboarding, error) {
	r.userID, r.intent = id, intent
	return &MarketplaceOnboarding{Version: version, Intent: intent}, nil
}

func TestMarketplaceOnboardingIdentityAndIntent(t *testing.T) {
	repo := &marketGuideRepo{}
	svc := NewMarketplaceService(repo)
	_, err := svc.GetOnboarding(context.Background(), 0)
	require.Error(t, err)
	_, err = svc.CompleteOnboarding(context.Background(), 7, "admin")
	require.Error(t, err)
	require.Zero(t, repo.userID)
	for _, intent := range []string{"hire", "sell", "browse"} {
		result, err := svc.CompleteOnboarding(context.Background(), 7, intent)
		require.NoError(t, err)
		require.EqualValues(t, 7, repo.userID)
		require.Equal(t, intent, result.Intent)
	}
	result, err := svc.GetOnboarding(context.Background(), 8)
	require.NoError(t, err)
	require.EqualValues(t, 8, repo.userID)
	require.Nil(t, result.CompletedAt)
}
