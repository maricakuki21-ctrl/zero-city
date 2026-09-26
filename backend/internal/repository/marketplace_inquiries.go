package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const marketplaceInquiryColumns = `
	i.id, i.listing_id, i.initiator_user_id, i.listing_owner_user_id,
	i.created_at, i.updated_at, i.last_message_at, `

func scanMarketplaceInquiry(row scanner) (*service.MarketplaceInquiry, error) {
	var inquiry service.MarketplaceInquiry
	var lastMessageAt sql.NullTime
	var canonicalAssetID sql.NullInt64
	var tagsRaw []byte
	if err := row.Scan(
		&inquiry.ID, &inquiry.ListingID, &inquiry.InitiatorUserID,
		&inquiry.ListingOwnerUserID, &inquiry.CreatedAt, &inquiry.UpdatedAt,
		&lastMessageAt, &inquiry.Listing.ID, &inquiry.Listing.Kind,
		&inquiry.Listing.OwnerUserID, &inquiry.Listing.OwnerDisplayName,
		&canonicalAssetID, &inquiry.Listing.Title, &inquiry.Listing.Summary,
		&inquiry.Listing.Category, &inquiry.Listing.PriceText,
		&inquiry.Listing.DeliveryText, &tagsRaw, &inquiry.Listing.Status,
		&inquiry.Listing.CreatedAt, &inquiry.Listing.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if lastMessageAt.Valid {
		inquiry.LastMessageAt = &lastMessageAt.Time
	}
	if canonicalAssetID.Valid {
		inquiry.Listing.CanonicalAssetID = &canonicalAssetID.Int64
	}
	inquiry.Listing.Tags = scanStringArray(tagsRaw)
	return &inquiry, nil
}

func (r *marketplaceRepository) CreateInquiry(ctx context.Context, listingID, initiatorID int64) (*service.MarketplaceInquiry, error) {
	var ownerID int64
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT owner_user_id, status FROM marketplace_listings WHERE id = $1`, listingID).Scan(&ownerID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceInquiryUnavailable
	}
	if err != nil {
		return nil, err
	}
	if ownerID == initiatorID {
		return nil, service.ErrMarketplaceSelfInquiry
	}
	if status != service.MarketplaceListingStatusPublished {
		return nil, service.ErrMarketplaceInquiryUnavailable
	}

	var inquiryID int64
	err = r.db.QueryRowContext(ctx, `INSERT INTO marketplace_inquiries
		(listing_id, initiator_user_id, listing_owner_user_id)
		SELECT id, $2, owner_user_id FROM marketplace_listings
		WHERE id = $1 AND status = 'published' AND owner_user_id <> $2
		ON CONFLICT (listing_id, initiator_user_id)
		DO UPDATE SET updated_at = marketplace_inquiries.updated_at
		RETURNING id`, listingID, initiatorID).Scan(&inquiryID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceInquiryUnavailable
	}
	if err != nil {
		return nil, err
	}
	return r.getInquiry(ctx, inquiryID, initiatorID)
}

func (r *marketplaceRepository) getInquiry(ctx context.Context, inquiryID, participantID int64) (*service.MarketplaceInquiry, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+marketplaceInquiryColumns+marketplaceListingColumns+
		` FROM marketplace_inquiries i JOIN marketplace_listings l ON l.id = i.listing_id
		JOIN users u ON u.id = l.owner_user_id LEFT JOIN biz_profiles bp ON bp.user_id = l.owner_user_id
		WHERE i.id = $1 AND ($2 = i.initiator_user_id OR $2 = i.listing_owner_user_id)`, inquiryID, participantID)
	inquiry, err := scanMarketplaceInquiry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceInquiryForbidden
	}
	return inquiry, err
}

func (r *marketplaceRepository) ListInquiries(ctx context.Context, participantID int64, query service.MarketplacePageQuery) (service.MarketplaceInquiryPage, error) {
	args := []any{participantID}
	where := "($1 = i.initiator_user_id OR $1 = i.listing_owner_user_id)"
	if query.Cursor > 0 {
		args = append(args, query.Cursor)
		where += fmt.Sprintf(" AND i.id < $%d", len(args))
	}
	args = append(args, query.Limit+1)
	rows, err := r.db.QueryContext(ctx, `SELECT `+marketplaceInquiryColumns+marketplaceListingColumns+
		` FROM marketplace_inquiries i JOIN marketplace_listings l ON l.id = i.listing_id
		JOIN users u ON u.id = l.owner_user_id LEFT JOIN biz_profiles bp ON bp.user_id = l.owner_user_id
		WHERE `+where+fmt.Sprintf(" ORDER BY i.id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return service.MarketplaceInquiryPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MarketplaceInquiry, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanMarketplaceInquiry(rows)
		if scanErr != nil {
			return service.MarketplaceInquiryPage{}, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return service.MarketplaceInquiryPage{}, err
	}
	page := service.MarketplaceInquiryPage{Items: items}
	if len(items) > query.Limit {
		page.Items = items[:query.Limit]
		cursor := page.Items[len(page.Items)-1].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

func scanMarketplaceMessage(row scanner) (*service.MarketplaceMessage, error) {
	var message service.MarketplaceMessage
	err := row.Scan(&message.ID, &message.InquiryID, &message.SenderUserID, &message.ClientMessageID, &message.Body, &message.CreatedAt)
	return &message, err
}

func (r *marketplaceRepository) ListMessages(ctx context.Context, inquiryID, participantID int64, query service.MarketplacePageQuery) (service.MarketplaceMessagePage, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT TRUE FROM marketplace_inquiries i
		WHERE i.id = $1 AND ($2 = i.initiator_user_id OR $2 = i.listing_owner_user_id)`, inquiryID, participantID).Scan(&allowed)
	if errors.Is(err, sql.ErrNoRows) {
		return service.MarketplaceMessagePage{}, service.ErrMarketplaceInquiryForbidden
	}
	if err != nil {
		return service.MarketplaceMessagePage{}, err
	}
	args := []any{inquiryID, participantID}
	where := `m.inquiry_id = $1 AND EXISTS (
		SELECT 1 FROM marketplace_inquiries i WHERE i.id = m.inquiry_id
		AND ($2 = i.initiator_user_id OR $2 = i.listing_owner_user_id))`
	if query.Cursor > 0 {
		args = append(args, query.Cursor)
		where += fmt.Sprintf(" AND m.id < $%d", len(args))
	}
	args = append(args, query.Limit+1)
	rows, err := r.db.QueryContext(ctx, `SELECT m.id, m.inquiry_id, m.sender_user_id, m.client_message_id, m.body, m.created_at
		FROM marketplace_inquiry_messages m WHERE `+where+fmt.Sprintf(" ORDER BY m.id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return service.MarketplaceMessagePage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MarketplaceMessage, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanMarketplaceMessage(rows)
		if scanErr != nil {
			return service.MarketplaceMessagePage{}, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return service.MarketplaceMessagePage{}, err
	}
	page := service.MarketplaceMessagePage{Items: items}
	if len(items) > query.Limit {
		page.Items = items[:query.Limit]
		cursor := page.Items[len(page.Items)-1].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

func (r *marketplaceRepository) PostMessage(ctx context.Context, inquiryID, senderID int64, input service.MarketplaceMessageInput) (*service.MarketplaceMessage, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var allowed bool
	err = tx.QueryRowContext(ctx, `SELECT TRUE FROM marketplace_inquiries i
		WHERE i.id = $1 AND ($2 = i.initiator_user_id OR $2 = i.listing_owner_user_id) FOR UPDATE`, inquiryID, senderID).Scan(&allowed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceInquiryForbidden
	}
	if err != nil {
		return nil, err
	}
	existing, err := marketplaceMessageByClientID(ctx, tx, inquiryID, senderID, input.ClientMessageID)
	if err == nil {
		if existing.Body != input.Body {
			return nil, service.ErrMarketplaceMessageIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	message, err := scanMarketplaceMessage(tx.QueryRowContext(ctx, `INSERT INTO marketplace_inquiry_messages
		(inquiry_id, sender_user_id, client_message_id, body) VALUES ($1, $2, $3, $4)
		RETURNING id, inquiry_id, sender_user_id, client_message_id, body, created_at`, inquiryID, senderID, input.ClientMessageID, input.Body))
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE marketplace_inquiries SET updated_at = NOW(), last_message_at = $2 WHERE id = $1`, inquiryID, message.CreatedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return message, nil
}

func marketplaceMessageByClientID(ctx context.Context, tx *sql.Tx, inquiryID, senderID int64, clientMessageID string) (*service.MarketplaceMessage, error) {
	return scanMarketplaceMessage(tx.QueryRowContext(ctx, `SELECT id, inquiry_id, sender_user_id, client_message_id, body, created_at
		FROM marketplace_inquiry_messages WHERE inquiry_id = $1 AND sender_user_id = $2 AND client_message_id = $3`, inquiryID, senderID, clientMessageID))
}
