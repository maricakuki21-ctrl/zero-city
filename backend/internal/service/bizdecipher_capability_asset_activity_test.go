package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type capabilityAssetActivityRepoStub struct {
	BizDecipherRepository
	asset            *CapabilityAsset
	viewCount        int64
	likeCount        int64
	favoriteCount    int64
	viewUserID       int64
	likeUserID       int64
	likeValue        bool
	favoriteUserID   int64
	favoriteValue    bool
	useEventType     string
	useSourceKind    string
	useSourceID      string
	useUserID        int64
	statsOwnerUserID int64
	viewerState      *CapabilityAssetViewerState
}

func (r *capabilityAssetActivityRepoStub) GetCapabilityAsset(_ context.Context, _ int64, _ bool) (*CapabilityAsset, error) {
	asset := *r.asset
	return &asset, nil
}

func (r *capabilityAssetActivityRepoStub) RecordCapabilityAssetView(_ context.Context, _, userID int64) (int64, error) {
	r.viewUserID = userID
	return r.viewCount, nil
}

func (r *capabilityAssetActivityRepoStub) GetCapabilityAssetViewerState(_ context.Context, _, _ int64) (*CapabilityAssetViewerState, error) {
	return r.viewerState, nil
}

func (r *capabilityAssetActivityRepoStub) SetCapabilityAssetLike(_ context.Context, _, userID int64, liked bool) (int64, error) {
	r.likeUserID = userID
	r.likeValue = liked
	return r.likeCount, nil
}

func (r *capabilityAssetActivityRepoStub) SetCapabilityAssetFavorite(_ context.Context, _, userID int64, favorited bool) (int64, error) {
	r.favoriteUserID = userID
	r.favoriteValue = favorited
	return r.favoriteCount, nil
}

func (r *capabilityAssetActivityRepoStub) RecordCapabilityAssetUse(
	_ context.Context,
	_, userID int64,
	eventType, sourceKind, sourceID string,
) error {
	r.useUserID = userID
	r.useEventType = eventType
	r.useSourceKind = sourceKind
	r.useSourceID = sourceID
	return nil
}

func (r *capabilityAssetActivityRepoStub) GetCapabilityAssetStats(_ context.Context, ownerUserID int64) ([]CapabilityAssetStats, error) {
	r.statsOwnerUserID = ownerUserID
	return []CapabilityAssetStats{{AssetID: r.asset.ID, UseCount: 3}}, nil
}

func TestGetCapabilityAssetDetailRecordsOnlyOtherUsersViewAndAppliesState(t *testing.T) {
	repo := &capabilityAssetActivityRepoStub{
		asset:     &CapabilityAsset{ID: 41, UserID: 7, Status: CapabilityAssetStatusListed, ViewCount: 9},
		viewCount: 10,
		viewerState: &CapabilityAssetViewerState{
			Liked: true, Favorited: true, Downloaded: true, LatestView: true,
		},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	asset, err := svc.GetCapabilityAssetDetail(context.Background(), 41, 8)
	require.NoError(t, err)
	require.Equal(t, int64(8), repo.viewUserID)
	require.Equal(t, int64(10), asset.ViewCount)
	require.True(t, asset.LikedByMe)
	require.True(t, asset.FavoritedByMe)
	require.True(t, asset.DownloadedByMe)
	require.True(t, asset.ViewedToday)

	repo.viewUserID = 0
	asset, err = svc.GetCapabilityAssetDetail(context.Background(), 41, 7)
	require.NoError(t, err)
	require.Zero(t, repo.viewUserID, "author detail views must not inflate the public count")
	require.Equal(t, int64(9), asset.ViewCount)
}

func TestCapabilityAssetActivityActionsAreExplicitAndIdempotentAtRepositoryBoundary(t *testing.T) {
	repo := &capabilityAssetActivityRepoStub{
		asset:         &CapabilityAsset{ID: 41, UserID: 7, Status: CapabilityAssetStatusListed},
		likeCount:     6,
		favoriteCount: 4,
	}
	svc := NewBizDecipherService(repo, nil, nil)

	liked, err := svc.SetCapabilityAssetLike(context.Background(), 41, 8, true)
	require.NoError(t, err)
	require.Equal(t, int64(8), repo.likeUserID)
	require.True(t, repo.likeValue)
	require.Equal(t, int64(6), liked.LikeCount)
	require.True(t, liked.LikedByMe)

	favorited, err := svc.SetCapabilityAssetFavorite(context.Background(), 41, 8, true)
	require.NoError(t, err)
	require.Equal(t, int64(8), repo.favoriteUserID)
	require.True(t, repo.favoriteValue)
	require.Equal(t, int64(4), favorited.FavoriteCount)
	require.True(t, favorited.FavoritedByMe)

	err = svc.RecordCapabilityAssetUse(context.Background(), 41, 8, " run ", " harness ", "run-123")
	require.NoError(t, err)
	require.Equal(t, int64(8), repo.useUserID)
	require.Equal(t, "run", repo.useEventType)
	require.Equal(t, "harness", repo.useSourceKind)
	require.Equal(t, "run-123", repo.useSourceID)
}

func TestCapabilityAssetActivityRejectsInvalidUseTypeAndScopesStatsToOwner(t *testing.T) {
	repo := &capabilityAssetActivityRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 7, Status: CapabilityAssetStatusListed},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	err := svc.RecordCapabilityAssetUse(context.Background(), 41, 8, "view", "page", "41")
	require.ErrorIs(t, err, ErrCapabilityAssetActivityForbidden)

	stats, err := svc.GetMyCapabilityAssetStats(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.statsOwnerUserID)
	require.Len(t, stats, 1)
	require.Equal(t, int64(3), stats[0].UseCount)
}
