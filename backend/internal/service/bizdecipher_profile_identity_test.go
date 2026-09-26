package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type zeroCityProfileIdentityRepoStub struct {
	BizDecipherRepository
	ensureUserID int64
	updateUserID int64
	displayName  string
	avatarURL    string
}

func (r *zeroCityProfileIdentityRepoStub) EnsureProfile(_ context.Context, userID int64) (*BizProfile, error) {
	r.ensureUserID = userID
	return &BizProfile{UserID: userID}, nil
}

func (r *zeroCityProfileIdentityRepoStub) UpdateProfileIdentity(_ context.Context, userID int64, displayName, avatarURL string) (*BizProfile, error) {
	r.updateUserID = userID
	r.displayName = displayName
	r.avatarURL = avatarURL
	return &BizProfile{UserID: userID, DisplayName: displayName, AvatarURL: avatarURL}, nil
}

func TestUpdateZeroCityProfileIdentityUpdatesOnlyAuthenticatedUserProfile(t *testing.T) {
	repo := &zeroCityProfileIdentityRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	profile, err := svc.UpdateZeroCityProfileIdentity(context.Background(), 17, ZeroCityProfileIdentityInput{
		DisplayName: "  City Resident  ",
		AvatarURL:   "https://cdn.example.test/resident.png",
	})

	require.NoError(t, err)
	require.Equal(t, int64(17), repo.ensureUserID)
	require.Equal(t, int64(17), repo.updateUserID)
	require.Equal(t, "City Resident", repo.displayName)
	require.Equal(t, "https://cdn.example.test/resident.png", repo.avatarURL)
	require.Equal(t, "City Resident", profile.DisplayName)
}

func TestUpdateZeroCityProfileIdentityRejectsNonHTTPAvatarBeforeWriting(t *testing.T) {
	repo := &zeroCityProfileIdentityRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateZeroCityProfileIdentity(context.Background(), 17, ZeroCityProfileIdentityInput{
		DisplayName: "City Resident",
		AvatarURL:   "file:///C:/secrets/avatar.png",
	})

	require.EqualError(t, err, "avatar url must use http or https")
	require.Zero(t, repo.ensureUserID)
	require.Zero(t, repo.updateUserID)
}
