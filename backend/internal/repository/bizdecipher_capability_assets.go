package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const capabilityAssetColumns = `
	a.id, a.user_id,
	COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'User') AS author,
	a.title, a.slug, a.summary, a.description, a.asset_type, a.status,
	a.tags, a.scenario_tags, a.integration_tags, a.cover_url, a.screenshot_urls,
	a.video_url, a.demo_url, a.doc_url, a.source_url, a.template_url,
	a.primary_action_type, a.pricing_type, a.contact_enabled,
	a.is_featured, a.featured_weight,
	(SELECT COUNT(*)::bigint FROM capability_asset_view_events v WHERE v.asset_id = a.id) AS view_count,
	(SELECT COUNT(*)::bigint FROM capability_asset_likes l WHERE l.asset_id = a.id) AS like_count,
	(SELECT COUNT(*)::bigint FROM capability_asset_favorites f WHERE f.asset_id = a.id) AS favorite_count,
	(SELECT COUNT(*)::bigint
		FROM capability_asset_download_events d
		JOIN capability_asset_versions dv ON dv.id = d.version_id
		WHERE dv.asset_id = a.id) AS download_count,
	(SELECT COUNT(*)::bigint FROM capability_asset_use_events ue WHERE ue.asset_id = a.id) AS use_count,
	false AS liked_by_me, false AS favorited_by_me, false AS downloaded_by_me, false AS viewed_today,
	a.comment_count,
	a.rating_avg, a.rating_count, a.review_note, a.reviewed_by, a.reviewed_at,
	a.published_at, a.archived_at, a.created_at, a.updated_at`

func scanCapabilityAsset(s scanner) (*service.CapabilityAsset, error) {
	var asset service.CapabilityAsset
	var tagsRaw, scenarioRaw, integrationRaw, screenshotsRaw []byte
	var reviewedBy sql.NullInt64
	var reviewedAt, publishedAt, archivedAt sql.NullTime
	if err := s.Scan(
		&asset.ID, &asset.UserID, &asset.Author,
		&asset.Title, &asset.Slug, &asset.Summary, &asset.Description, &asset.AssetType, &asset.Status,
		&tagsRaw, &scenarioRaw, &integrationRaw, &asset.CoverURL, &screenshotsRaw,
		&asset.VideoURL, &asset.DemoURL, &asset.DocURL, &asset.SourceURL, &asset.TemplateURL,
		&asset.PrimaryActionType, &asset.PricingType, &asset.ContactEnabled,
		&asset.Featured, &asset.FeaturedWeight,
		&asset.ViewCount, &asset.LikeCount, &asset.FavoriteCount, &asset.DownloadCount, &asset.UseCount,
		&asset.LikedByMe, &asset.FavoritedByMe, &asset.DownloadedByMe, &asset.ViewedToday,
		&asset.CommentCount,
		&asset.RatingAvg, &asset.RatingCount, &asset.ReviewNote, &reviewedBy, &reviewedAt,
		&publishedAt, &archivedAt, &asset.CreatedAt, &asset.UpdatedAt,
	); err != nil {
		return nil, err
	}
	asset.Tags = scanStringArray(tagsRaw)
	asset.ScenarioTags = scanStringArray(scenarioRaw)
	asset.IntegrationTags = scanStringArray(integrationRaw)
	asset.ScreenshotURLs = scanStringArray(screenshotsRaw)
	if reviewedBy.Valid {
		asset.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		asset.ReviewedAt = &reviewedAt.Time
	}
	if publishedAt.Valid {
		asset.PublishedAt = &publishedAt.Time
	}
	if archivedAt.Valid {
		asset.ArchivedAt = &archivedAt.Time
	}
	return &asset, nil
}

func capabilityAssetBaseQuery() string {
	return `SELECT ` + capabilityAssetColumns + `
		FROM capability_assets a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = a.user_id`
}

func capabilityAssetOrderBy(sort string, admin bool) string {
	switch sort {
	case "latest":
		return "a.published_at DESC NULLS LAST, a.created_at DESC, a.id DESC"
	case "popular":
		return `(SELECT COUNT(*) FROM capability_asset_favorites f WHERE f.asset_id = a.id) DESC,
			(SELECT COUNT(*) FROM capability_asset_view_events v WHERE v.asset_id = a.id) DESC,
			a.published_at DESC NULLS LAST, a.id DESC`
	case "updated":
		return "a.updated_at DESC, a.id DESC"
	case "featured":
		fallthrough
	default:
		if admin {
			return "a.updated_at DESC, a.id DESC"
		}
		return "a.is_featured DESC, a.featured_weight DESC, a.published_at DESC NULLS LAST, a.id DESC"
	}
}

func (r *bizDecipherRepository) ListCapabilityAssets(ctx context.Context, query service.CapabilityAssetQuery) ([]service.CapabilityAsset, error) {
	return r.listCapabilityAssets(ctx, query, false)
}

func (r *bizDecipherRepository) AdminListCapabilityAssets(ctx context.Context, query service.CapabilityAssetQuery) ([]service.CapabilityAsset, error) {
	return r.listCapabilityAssets(ctx, query, true)
}

func (r *bizDecipherRepository) listCapabilityAssets(ctx context.Context, query service.CapabilityAssetQuery, admin bool) ([]service.CapabilityAsset, error) {
	clauses := []string{"a.deleted_at IS NULL"}
	args := []any{}
	if query.Status != "" {
		args = append(args, query.Status)
		clauses = append(clauses, fmt.Sprintf("a.status = $%d", len(args)))
	} else if !admin {
		clauses = append(clauses, "a.status = 'listed'")
	}
	if query.AssetType != "" {
		args = append(args, query.AssetType)
		clauses = append(clauses, fmt.Sprintf("a.asset_type = $%d", len(args)))
	}
	if query.Featured {
		clauses = append(clauses, "a.is_featured = true")
	}
	if strings.TrimSpace(query.Keyword) != "" {
		args = append(args, "%"+strings.TrimSpace(query.Keyword)+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf("(a.title ILIKE $%d OR a.summary ILIKE $%d OR a.description ILIKE $%d)", idx, idx, idx))
	}
	args = append(args, clampLimit(query.Limit))
	limitParam := len(args)
	stmt := capabilityAssetBaseQuery() + `
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY ` + capabilityAssetOrderBy(query.Sort, admin) + fmt.Sprintf(" LIMIT $%d", limitParam)
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.CapabilityAsset
	for rows.Next() {
		asset, err := scanCapabilityAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *asset)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) GetCapabilityAsset(ctx context.Context, id int64, includeUnlisted bool) (*service.CapabilityAsset, error) {
	where := "a.deleted_at IS NULL AND a.id = $1"
	if !includeUnlisted {
		where += " AND a.status = 'listed'"
	}
	row := r.db.QueryRowContext(ctx, capabilityAssetBaseQuery()+" WHERE "+where, id)
	asset, err := scanCapabilityAsset(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asset, nil
}

func (r *bizDecipherRepository) ListMyCapabilityAssets(ctx context.Context, userID int64, status string, limit int) ([]service.CapabilityAsset, error) {
	clauses := []string{"a.deleted_at IS NULL", "a.user_id = $1"}
	args := []any{userID}
	if status != "" {
		args = append(args, status)
		clauses = append(clauses, fmt.Sprintf("a.status = $%d", len(args)))
	}
	args = append(args, clampLimit(limit))
	stmt := capabilityAssetBaseQuery() + `
		WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(`
		ORDER BY a.updated_at DESC, a.id DESC LIMIT $%d`, len(args))
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.CapabilityAsset
	for rows.Next() {
		asset, err := scanCapabilityAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *asset)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) CreateCapabilityAsset(ctx context.Context, userID int64, slugBase string, input service.CapabilityAssetInput) (*service.CapabilityAsset, error) {
	slug := fmt.Sprintf("%s-%d", slugBase, time.Now().UnixNano()%1000000)
	var assetID int64
	if err := r.db.QueryRowContext(ctx, `
		INSERT INTO capability_assets (
			user_id, title, slug, summary, description, asset_type, status,
			tags, scenario_tags, integration_tags, cover_url, screenshot_urls,
			video_url, demo_url, doc_url, source_url, template_url,
			primary_action_type, pricing_type, contact_enabled,
			published_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7::text,
			$8::jsonb, $9::jsonb, $10::jsonb, $11, $12::jsonb,
			$13, $14, $15, $16, $17,
			$18, $19, $20,
			CASE WHEN $7::text = 'listed' THEN NOW() ELSE NULL END
		)
		RETURNING id`,
		userID, input.Title, slug, input.Summary, input.Description, input.AssetType, input.Status,
		string(jsonBytes(input.Tags)), string(jsonBytes(input.ScenarioTags)), string(jsonBytes(input.IntegrationTags)), input.CoverURL, string(jsonBytes(input.ScreenshotURLs)),
		input.VideoURL, input.DemoURL, input.DocURL, input.SourceURL, input.TemplateURL,
		input.PrimaryActionType, input.PricingType, input.ContactEnabled,
	).Scan(&assetID); err != nil {
		return nil, err
	}
	return r.GetCapabilityAsset(ctx, assetID, true)
}

func (r *bizDecipherRepository) UpdateCapabilityAsset(ctx context.Context, assetID, userID int64, input service.CapabilityAssetInput) (*service.CapabilityAsset, error) {
	var updatedID int64
	err := r.db.QueryRowContext(ctx, `
		UPDATE capability_assets
		SET title = $3,
			summary = $4,
			description = $5,
			asset_type = $6,
			tags = $7::jsonb,
			scenario_tags = $8::jsonb,
			integration_tags = $9::jsonb,
			cover_url = $10,
			screenshot_urls = $11::jsonb,
			video_url = $12,
			demo_url = $13,
			doc_url = $14,
			source_url = $15,
			template_url = $16,
			primary_action_type = $17,
			pricing_type = $18,
			contact_enabled = $19,
			updated_at = NOW()
		WHERE id = $1
			AND user_id = $2
			AND status = 'draft'
			AND deleted_at IS NULL
		RETURNING id`,
		assetID, userID, input.Title, input.Summary, input.Description, input.AssetType,
		string(jsonBytes(input.Tags)), string(jsonBytes(input.ScenarioTags)), string(jsonBytes(input.IntegrationTags)), input.CoverURL,
		string(jsonBytes(input.ScreenshotURLs)), input.VideoURL, input.DemoURL, input.DocURL, input.SourceURL, input.TemplateURL,
		input.PrimaryActionType, input.PricingType, input.ContactEnabled,
	).Scan(&updatedID)
	if err == sql.ErrNoRows {
		return nil, service.ErrCapabilityAssetDraftNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetCapabilityAsset(ctx, updatedID, true)
}

func (r *bizDecipherRepository) AdminReviewCapabilityAsset(ctx context.Context, assetID, reviewerID int64, input service.CapabilityAssetReviewInput) (*service.CapabilityAsset, error) {
	featuredSQL := "is_featured"
	featuredWeightSQL := "featured_weight"
	args := []any{assetID, reviewerID, input.Status, input.ReviewNote}
	if input.Featured != nil {
		args = append(args, *input.Featured)
		featuredSQL = fmt.Sprintf("$%d", len(args))
	}
	if input.FeaturedWeight != nil {
		args = append(args, *input.FeaturedWeight)
		featuredWeightSQL = fmt.Sprintf("$%d", len(args))
	}
	stmt := fmt.Sprintf(`
		UPDATE capability_assets
		SET status = $3::text,
			review_note = $4,
			reviewed_by = $2,
			reviewed_at = NOW(),
			published_at = CASE WHEN $3::text = 'listed' AND published_at IS NULL THEN NOW() ELSE published_at END,
			archived_at = CASE WHEN $3::text IN ('archived', 'delisted') THEN NOW() ELSE archived_at END,
			is_featured = %s,
			featured_weight = %s,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id`, featuredSQL, featuredWeightSQL)
	var updatedID int64
	if err := r.db.QueryRowContext(ctx, stmt, args...).Scan(&updatedID); err != nil {
		return nil, err
	}
	return r.GetCapabilityAsset(ctx, updatedID, true)
}
