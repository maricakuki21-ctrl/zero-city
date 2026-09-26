package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrCapabilityAssetActivityNotFound = errors.New("capability asset activity target not found")

var ErrCapabilityAssetActivityForbidden = errors.New("capability asset activity forbidden")

type CapabilityAssetViewerState struct {
	Liked      bool `json:"liked_by_me"`
	Favorited  bool `json:"favorited_by_me"`
	Downloaded bool `json:"downloaded_by_me"`
	LatestView bool `json:"viewed_today"`
}

type CapabilityAssetStats struct {
	AssetID           int64      `json:"asset_id"`
	Title             string     `json:"title"`
	Status            string     `json:"status"`
	ViewCount         int64      `json:"view_count"`
	LikeCount         int64      `json:"like_count"`
	FavoriteCount     int64      `json:"favorite_count"`
	DownloadCount     int64      `json:"download_count"`
	UseCount          int64      `json:"use_count"`
	UniqueViewers     int64      `json:"unique_viewers"`
	UniqueDownloaders int64      `json:"unique_downloaders"`
	UniqueUsers       int64      `json:"unique_users"`
	RevenueConnected  bool       `json:"revenue_connected"`
	LastDownloadAt    *time.Time `json:"last_download_at,omitempty"`
	LastUseAt         *time.Time `json:"last_use_at,omitempty"`
}

func (s *BizDecipherService) GetCapabilityAssetDetail(ctx context.Context, assetID, viewerUserID int64) (*CapabilityAsset, error) {
	asset, err := s.GetCapabilityAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if viewerUserID <= 0 {
		return asset, nil
	}

	if viewerUserID != asset.UserID {
		viewCount, err := s.repo.RecordCapabilityAssetView(ctx, assetID, viewerUserID)
		if err != nil {
			return nil, err
		}
		if viewCount >= 0 {
			asset.ViewCount = viewCount
		}
	}

	state, err := s.repo.GetCapabilityAssetViewerState(ctx, assetID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if state != nil {
		asset.LikedByMe = state.Liked
		asset.FavoritedByMe = state.Favorited
		asset.DownloadedByMe = state.Downloaded
		asset.ViewedToday = state.LatestView
	}
	return asset, nil
}

func (s *BizDecipherService) SetCapabilityAssetLike(ctx context.Context, assetID, userID int64, liked bool) (*CapabilityAsset, error) {
	if userID <= 0 {
		return nil, ErrCapabilityAssetActivityForbidden
	}
	if err := s.requireCapabilityAssetActivityTarget(ctx, assetID); err != nil {
		return nil, err
	}
	count, err := s.repo.SetCapabilityAssetLike(ctx, assetID, userID, liked)
	if err != nil {
		return nil, err
	}
	asset, err := s.GetCapabilityAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	asset.LikeCount = count
	asset.LikedByMe = liked
	return asset, nil
}

func (s *BizDecipherService) SetCapabilityAssetFavorite(ctx context.Context, assetID, userID int64, favorited bool) (*CapabilityAsset, error) {
	if userID <= 0 {
		return nil, ErrCapabilityAssetActivityForbidden
	}
	if err := s.requireCapabilityAssetActivityTarget(ctx, assetID); err != nil {
		return nil, err
	}
	count, err := s.repo.SetCapabilityAssetFavorite(ctx, assetID, userID, favorited)
	if err != nil {
		return nil, err
	}
	asset, err := s.GetCapabilityAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	asset.FavoriteCount = count
	asset.FavoritedByMe = favorited
	return asset, nil
}

func (s *BizDecipherService) RecordCapabilityAssetUse(
	ctx context.Context,
	assetID, userID int64,
	eventType, sourceKind, sourceID string,
) error {
	if userID <= 0 {
		return ErrCapabilityAssetActivityForbidden
	}
	if err := s.requireCapabilityAssetActivityTarget(ctx, assetID); err != nil {
		return err
	}
	asset, err := s.repo.GetCapabilityAsset(ctx, assetID, false)
	if err != nil {
		return err
	}
	if asset == nil {
		return ErrCapabilityAssetActivityNotFound
	}
	if asset.PricingType == "paid" {
		entitled, accessErr := s.assetPackageEntitled(ctx, asset, userID)
		if accessErr != nil {
			return accessErr
		}
		if !entitled {
			return ErrCapabilityAssetActivityForbidden
		}
	}
	eventType = strings.TrimSpace(eventType)
	switch eventType {
	case "run", "install", "derive":
	default:
		return ErrCapabilityAssetActivityForbidden
	}
	sourceKind = truncateCapabilityString(strings.TrimSpace(sourceKind), 40)
	sourceID = truncateCapabilityString(strings.TrimSpace(sourceID), 160)
	return s.repo.RecordCapabilityAssetUse(ctx, assetID, userID, eventType, sourceKind, sourceID)
}

func (s *BizDecipherService) GetMyCapabilityAssetStats(ctx context.Context, ownerUserID int64) ([]CapabilityAssetStats, error) {
	if ownerUserID <= 0 {
		return nil, ErrCapabilityAssetActivityForbidden
	}
	return s.repo.GetCapabilityAssetStats(ctx, ownerUserID)
}

func (s *BizDecipherService) requireCapabilityAssetActivityTarget(ctx context.Context, assetID int64) error {
	if assetID <= 0 || s == nil || s.repo == nil {
		return ErrCapabilityAssetActivityNotFound
	}
	asset, err := s.repo.GetCapabilityAsset(ctx, assetID, false)
	if err != nil {
		return err
	}
	if asset == nil || asset.Status != CapabilityAssetStatusListed {
		return ErrCapabilityAssetActivityNotFound
	}
	return nil
}
