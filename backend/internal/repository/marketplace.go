package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type marketplaceRepository struct{ db *sql.DB }

func NewMarketplaceRepository(db *sql.DB) service.MarketplaceRepository {
	return &marketplaceRepository{db: db}
}

const marketplaceListingColumns = `
	l.id, l.kind, l.owner_user_id,
	COALESCE(NULLIF(bp.display_name, ''), NULLIF(u.username, ''), split_part(u.email, '@', 1), 'User'),
	l.canonical_asset_id, l.title, l.summary, l.category, l.price_text,
	l.delivery_text, l.tags, l.status, l.created_at, l.updated_at`

func marketplaceListingFrom() string {
	return ` FROM marketplace_listings l
		JOIN users u ON u.id = l.owner_user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = l.owner_user_id`
}

func scanMarketplaceListing(row scanner) (*service.MarketplaceListing, error) {
	var listing service.MarketplaceListing
	var canonicalAssetID sql.NullInt64
	var tagsRaw []byte
	if err := row.Scan(
		&listing.ID, &listing.Kind, &listing.OwnerUserID, &listing.OwnerDisplayName,
		&canonicalAssetID, &listing.Title, &listing.Summary, &listing.Category,
		&listing.PriceText, &listing.DeliveryText, &tagsRaw, &listing.Status,
		&listing.CreatedAt, &listing.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if canonicalAssetID.Valid {
		listing.CanonicalAssetID = &canonicalAssetID.Int64
	}
	listing.Tags = scanStringArray(tagsRaw)
	return &listing, nil
}

func (r *marketplaceRepository) ListListings(ctx context.Context, query service.MarketplaceListingQuery) (service.MarketplaceListingPage, error) {
	clauses := []string{"l.status = 'published'"}
	args := make([]any, 0, 5)
	if query.Cursor > 0 {
		args = append(args, query.Cursor)
		clauses = append(clauses, fmt.Sprintf("l.id < $%d", len(args)))
	}
	if query.Kind != "" {
		args = append(args, query.Kind)
		clauses = append(clauses, fmt.Sprintf("l.kind = $%d", len(args)))
	}
	if query.Category != "" {
		args = append(args, query.Category)
		clauses = append(clauses, fmt.Sprintf("l.category = $%d", len(args)))
	}
	if query.Tag != "" {
		args = append(args, query.Tag)
		clauses = append(clauses, fmt.Sprintf("l.tags ? $%d", len(args)))
	}
	args = append(args, query.Limit+1)
	statement := `SELECT ` + marketplaceListingColumns + marketplaceListingFrom() +
		` WHERE ` + strings.Join(clauses, " AND ") +
		fmt.Sprintf(" ORDER BY l.id DESC LIMIT $%d", len(args))
	return r.queryListingPage(ctx, statement, query.Limit, args...)
}

func (r *marketplaceRepository) GetListing(ctx context.Context, listingID, viewerID int64) (*service.MarketplaceListing, error) {
	return r.getListing(ctx, listingID, viewerID, false)
}

func (r *marketplaceRepository) getListing(ctx context.Context, listingID, viewerID int64, includeAll bool) (*service.MarketplaceListing, error) {
	visibility := "(l.status = 'published' OR l.owner_user_id = NULLIF($2, 0))"
	args := []any{listingID, viewerID}
	if includeAll {
		visibility = "TRUE"
		args = []any{listingID}
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+marketplaceListingColumns+marketplaceListingFrom()+
		` WHERE l.id = $1 AND `+visibility, args...)
	listing, err := scanMarketplaceListing(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceListingNotFound
	}
	return listing, err
}

func (r *marketplaceRepository) CreateListing(ctx context.Context, ownerID int64, input service.MarketplaceListingInput) (*service.MarketplaceListing, error) {
	var listingID int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO marketplace_listings
		(kind, owner_user_id, canonical_asset_id, title, summary, category, price_text, delivery_text, tags, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, 'published') RETURNING id`,
		input.Kind, ownerID, nullableInt64Arg(input.CanonicalAssetID), input.Title, input.Summary,
		input.Category, input.PriceText, input.DeliveryText, marketplaceTagsJSON(input.Tags),
	).Scan(&listingID)
	if err != nil {
		return nil, marketplaceWriteError(err)
	}
	return r.GetListing(ctx, listingID, ownerID)
}

func (r *marketplaceRepository) UpdateListing(ctx context.Context, listingID, ownerID int64, input service.MarketplaceListingInput) (*service.MarketplaceListing, error) {
	var updatedID int64
	var outcome string
	err := r.db.QueryRowContext(ctx, `WITH target AS (
		SELECT id, status FROM marketplace_listings WHERE id = $1 AND owner_user_id = $2
	), updated AS (
		UPDATE marketplace_listings l SET kind = $3, canonical_asset_id = $4, title = $5,
			summary = $6, category = $7, price_text = $8, delivery_text = $9, tags = $10::jsonb, updated_at = NOW()
		FROM target t WHERE l.id = t.id AND t.status <> 'taken_down' RETURNING l.id
	)
	SELECT id, 'updated' FROM updated
	UNION ALL SELECT id, 'locked' FROM target WHERE status = 'taken_down' LIMIT 1`,
		listingID, ownerID, input.Kind, nullableInt64Arg(input.CanonicalAssetID), input.Title,
		input.Summary, input.Category, input.PriceText, input.DeliveryText, marketplaceTagsJSON(input.Tags),
	).Scan(&updatedID, &outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceListingNotFound
	}
	if err != nil {
		return nil, marketplaceWriteError(err)
	}
	if outcome == "locked" {
		return nil, service.ErrMarketplaceListingLocked
	}
	return r.GetListing(ctx, updatedID, ownerID)
}

func (r *marketplaceRepository) ArchiveListing(ctx context.Context, listingID, ownerID int64) (*service.MarketplaceListing, error) {
	var updatedID int64
	var outcome string
	err := r.db.QueryRowContext(ctx, `WITH target AS (
		SELECT id, status FROM marketplace_listings WHERE id = $1 AND owner_user_id = $2
	), updated AS (
		UPDATE marketplace_listings l SET status = 'archived', updated_at = NOW()
		FROM target t WHERE l.id = t.id AND t.status <> 'taken_down' RETURNING l.id
	)
	SELECT id, 'updated' FROM updated
	UNION ALL SELECT id, 'locked' FROM target WHERE status = 'taken_down' LIMIT 1`, listingID, ownerID).Scan(&updatedID, &outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceListingNotFound
	}
	if err != nil {
		return nil, err
	}
	if outcome == "locked" {
		return nil, service.ErrMarketplaceListingLocked
	}
	return r.GetListing(ctx, updatedID, ownerID)
}

func (r *marketplaceRepository) ListOwnerListings(ctx context.Context, ownerID int64, query service.MarketplacePageQuery) (service.MarketplaceListingPage, error) {
	args := []any{ownerID}
	where := "l.owner_user_id = $1"
	if query.Cursor > 0 {
		args = append(args, query.Cursor)
		where += fmt.Sprintf(" AND l.id < $%d", len(args))
	}
	args = append(args, query.Limit+1)
	return r.queryListingPage(ctx, `SELECT `+marketplaceListingColumns+marketplaceListingFrom()+
		` WHERE `+where+fmt.Sprintf(" ORDER BY l.id DESC LIMIT $%d", len(args)), query.Limit, args...)
}

func (r *marketplaceRepository) AdminSetListingStatus(ctx context.Context, listingID int64, status string) (*service.MarketplaceListing, error) {
	var updatedID int64
	if err := r.db.QueryRowContext(ctx, `UPDATE marketplace_listings SET status = $2, updated_at = NOW()
		WHERE id = $1 RETURNING id`, listingID, status).Scan(&updatedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrMarketplaceListingNotFound
		}
		return nil, err
	}
	return r.getListing(ctx, updatedID, 0, true)
}

func (r *marketplaceRepository) queryListingPage(ctx context.Context, statement string, limit int, args ...any) (service.MarketplaceListingPage, error) {
	rows, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return service.MarketplaceListingPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MarketplaceListing, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanMarketplaceListing(rows)
		if scanErr != nil {
			return service.MarketplaceListingPage{}, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return service.MarketplaceListingPage{}, err
	}
	page := service.MarketplaceListingPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		cursor := page.Items[len(page.Items)-1].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

func marketplaceWriteError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && (pqErr.Code == "23503" || pqErr.Code == "23514") {
		return badMarketplaceRepository("canonical asset must exist and belong to the listing owner")
	}
	return err
}

func badMarketplaceRepository(message string) error {
	return infraerrors.BadRequest("MARKETPLACE_INVALID_ARGUMENT", message)
}

func marketplaceTagsJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	return string(jsonBytes(tags))
}
