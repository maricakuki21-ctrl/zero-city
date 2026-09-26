package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) RecordCapabilityAssetView(ctx context.Context, assetID, userID int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
		WITH recorded AS (
			INSERT INTO capability_asset_view_events (asset_id, user_id)
			SELECT $1, $2
			WHERE EXISTS (
				SELECT 1 FROM capability_assets
				WHERE id = $1 AND status = 'listed' AND deleted_at IS NULL
			)
			ON CONFLICT (asset_id, user_id, view_day) DO NOTHING
			RETURNING id
		)
		SELECT
			(SELECT COUNT(*)::bigint FROM capability_asset_view_events WHERE asset_id = $1)
			+ (SELECT COUNT(*)::bigint FROM recorded)`, assetID, userID).Scan(&count)
	return count, err
}

func (r *bizDecipherRepository) GetCapabilityAssetViewerState(
	ctx context.Context,
	assetID, userID int64,
) (*service.CapabilityAssetViewerState, error) {
	var state service.CapabilityAssetViewerState
	err := r.db.QueryRowContext(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM capability_asset_likes WHERE asset_id = $1 AND user_id = $2),
			EXISTS (SELECT 1 FROM capability_asset_favorites WHERE asset_id = $1 AND user_id = $2),
			EXISTS (
				SELECT 1
				FROM capability_asset_download_events d
				JOIN capability_asset_versions v ON v.id = d.version_id
				WHERE v.asset_id = $1 AND d.user_id = $2
			),
			EXISTS (
				SELECT 1 FROM capability_asset_view_events
				WHERE asset_id = $1 AND user_id = $2 AND view_day = CURRENT_DATE
			)`, assetID, userID).Scan(
		&state.Liked, &state.Favorited, &state.Downloaded, &state.LatestView,
	)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *bizDecipherRepository) SetCapabilityAssetLike(
	ctx context.Context,
	assetID, userID int64,
	liked bool,
) (int64, error) {
	var count int64
	var err error
	if liked {
		err = r.db.QueryRowContext(ctx, `
			WITH changed AS (
				INSERT INTO capability_asset_likes (asset_id, user_id)
				SELECT $1, $2
				WHERE EXISTS (
					SELECT 1 FROM capability_assets
					WHERE id = $1 AND status = 'listed' AND deleted_at IS NULL
				)
				ON CONFLICT (asset_id, user_id) DO NOTHING
				RETURNING asset_id
			)
			SELECT
				(SELECT COUNT(*)::bigint FROM capability_asset_likes WHERE asset_id = $1)
				+ (SELECT COUNT(*)::bigint FROM changed)`,
			assetID, userID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, `
			WITH removed AS (
				DELETE FROM capability_asset_likes
				WHERE asset_id = $1 AND user_id = $2
				RETURNING asset_id
			)
			SELECT
				(SELECT COUNT(*)::bigint FROM capability_asset_likes WHERE asset_id = $1)
				- (SELECT COUNT(*)::bigint FROM removed)`,
			assetID, userID).Scan(&count)
	}
	return count, err
}

func (r *bizDecipherRepository) SetCapabilityAssetFavorite(
	ctx context.Context,
	assetID, userID int64,
	favorited bool,
) (int64, error) {
	var count int64
	var err error
	if favorited {
		err = r.db.QueryRowContext(ctx, `
			WITH changed AS (
				INSERT INTO capability_asset_favorites (asset_id, user_id)
				SELECT $1, $2
				WHERE EXISTS (
					SELECT 1 FROM capability_assets
					WHERE id = $1 AND status = 'listed' AND deleted_at IS NULL
				)
				ON CONFLICT (asset_id, user_id) DO NOTHING
				RETURNING asset_id
			)
			SELECT
				(SELECT COUNT(*)::bigint FROM capability_asset_favorites WHERE asset_id = $1)
				+ (SELECT COUNT(*)::bigint FROM changed)`,
			assetID, userID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, `
			WITH removed AS (
				DELETE FROM capability_asset_favorites
				WHERE asset_id = $1 AND user_id = $2
				RETURNING asset_id
			)
			SELECT
				(SELECT COUNT(*)::bigint FROM capability_asset_favorites WHERE asset_id = $1)
				- (SELECT COUNT(*)::bigint FROM removed)`,
			assetID, userID).Scan(&count)
	}
	return count, err
}

func (r *bizDecipherRepository) RecordCapabilityAssetUse(
	ctx context.Context,
	assetID, userID int64,
	eventType, sourceKind, sourceID string,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO capability_asset_use_events (
			asset_id, user_id, event_type, source_kind, source_id
		)
		SELECT $1, $2, $3, $4, $5
		WHERE EXISTS (
			SELECT 1 FROM capability_assets
			WHERE id = $1 AND status = 'listed' AND deleted_at IS NULL
		)
		ON CONFLICT DO NOTHING`,
		assetID, userID, eventType, sourceKind, sourceID)
	return err
}

func (r *bizDecipherRepository) GetCapabilityAssetStats(
	ctx context.Context,
	ownerUserID int64,
) ([]service.CapabilityAssetStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			a.id,
			a.title,
			a.status,
			(SELECT COUNT(*)::bigint FROM capability_asset_view_events v WHERE v.asset_id = a.id),
			(SELECT COUNT(*)::bigint FROM capability_asset_likes l WHERE l.asset_id = a.id),
			(SELECT COUNT(*)::bigint FROM capability_asset_favorites f WHERE f.asset_id = a.id),
			(SELECT COUNT(*)::bigint
				FROM capability_asset_download_events d
				JOIN capability_asset_versions dv ON dv.id = d.version_id
				WHERE dv.asset_id = a.id),
			(SELECT COUNT(*)::bigint FROM capability_asset_use_events ue WHERE ue.asset_id = a.id),
			(SELECT COUNT(DISTINCT viewer.user_id)
				FROM capability_asset_view_events viewer
				WHERE viewer.asset_id = a.id),
			(SELECT COUNT(DISTINCT downloader.user_id)
				FROM capability_asset_download_events downloader
				JOIN capability_asset_versions dv ON dv.id = downloader.version_id
				WHERE dv.asset_id = a.id),
			(SELECT COUNT(*)::bigint FROM (
				SELECT user_id FROM capability_asset_view_events WHERE asset_id = a.id
				UNION
				SELECT user_id FROM capability_asset_likes WHERE asset_id = a.id
				UNION
				SELECT user_id FROM capability_asset_favorites WHERE asset_id = a.id
				UNION
				SELECT d.user_id
				FROM capability_asset_download_events d
				JOIN capability_asset_versions dv ON dv.id = d.version_id
				WHERE dv.asset_id = a.id
				UNION
				SELECT user_id FROM capability_asset_use_events WHERE asset_id = a.id
			) participants),
			(EXISTS(SELECT 1 FROM capability_asset_purchases p WHERE p.asset_id=a.id)
			 OR (a.pricing_type='paid' AND a.commerce_price>0
			  AND EXISTS(SELECT 1 FROM capability_asset_commerce_policy WHERE enabled=TRUE)
			  AND EXISTS(SELECT 1 FROM capability_asset_versions v WHERE v.asset_id=a.id
			   AND v.status='published' AND v.file_count>0 AND v.runtime_kind<>'external_api'))),
			(SELECT MAX(d.created_at)
				FROM capability_asset_download_events d
				JOIN capability_asset_versions dv ON dv.id = d.version_id
				WHERE dv.asset_id = a.id),
			(SELECT MAX(ue.created_at) FROM capability_asset_use_events ue WHERE ue.asset_id = a.id)
		FROM capability_assets a
		WHERE a.user_id = $1
		  AND a.deleted_at IS NULL
		ORDER BY a.updated_at DESC, a.id DESC`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.CapabilityAssetStats, 0)
	for rows.Next() {
		var item service.CapabilityAssetStats
		var lastDownloadAt, lastUseAt sql.NullTime
		if err := rows.Scan(
			&item.AssetID,
			&item.Title,
			&item.Status,
			&item.ViewCount,
			&item.LikeCount,
			&item.FavoriteCount,
			&item.DownloadCount,
			&item.UseCount,
			&item.UniqueViewers,
			&item.UniqueDownloaders,
			&item.UniqueUsers,
			&item.RevenueConnected,
			&lastDownloadAt,
			&lastUseAt,
		); err != nil {
			return nil, err
		}
		if lastDownloadAt.Valid {
			item.LastDownloadAt = &lastDownloadAt.Time
		}
		if lastUseAt.Valid {
			item.LastUseAt = &lastUseAt.Time
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
