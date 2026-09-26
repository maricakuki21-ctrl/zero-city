package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MarketplaceListingStatusPublished = "published"
	MarketplaceListingStatusArchived  = "archived"
	MarketplaceListingStatusTakenDown = "taken_down"
)

var (
	ErrMarketplaceListingNotFound            = infraerrors.NotFound("MARKETPLACE_LISTING_NOT_FOUND", "marketplace listing not found")
	ErrMarketplaceListingLocked              = infraerrors.Conflict("MARKETPLACE_LISTING_TAKEN_DOWN", "a taken-down listing can only be restored by an administrator")
	ErrMarketplaceSelfInquiry                = infraerrors.Conflict("MARKETPLACE_SELF_INQUIRY", "cannot create an inquiry on your own listing")
	ErrMarketplaceInquiryUnavailable         = infraerrors.Conflict("MARKETPLACE_INQUIRY_UNAVAILABLE", "listing is unavailable for inquiries")
	ErrMarketplaceInquiryForbidden           = infraerrors.Forbidden("MARKETPLACE_INQUIRY_FORBIDDEN", "marketplace inquiry is private to its participants")
	ErrMarketplaceMessageIdempotencyConflict = infraerrors.Conflict("MARKETPLACE_MESSAGE_IDEMPOTENCY_CONFLICT", "client_message_id was already used with different message content")
)

type MarketplaceListingInput struct {
	Kind             string   `json:"kind"`
	CanonicalAssetID *int64   `json:"canonical_asset_id,omitempty"`
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	Category         string   `json:"category"`
	PriceText        string   `json:"price_text"`
	DeliveryText     string   `json:"delivery_text"`
	Tags             []string `json:"tags"`
}

type MarketplaceListing struct {
	ID               int64     `json:"id"`
	Kind             string    `json:"kind"`
	OwnerUserID      int64     `json:"owner_user_id"`
	OwnerDisplayName string    `json:"owner_display_name"`
	CanonicalAssetID *int64    `json:"canonical_asset_id,omitempty"`
	Title            string    `json:"title"`
	Summary          string    `json:"summary"`
	Category         string    `json:"category"`
	PriceText        string    `json:"price_text"`
	DeliveryText     string    `json:"delivery_text"`
	Tags             []string  `json:"tags"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type MarketplacePageQuery struct {
	Cursor int64
	Limit  int
}
type MarketplaceListingQuery struct {
	MarketplacePageQuery
	Kind, Category, Tag string
}
type MarketplaceListingPage struct {
	Items      []MarketplaceListing `json:"items"`
	NextCursor *int64               `json:"next_cursor,omitempty"`
}

type MarketplaceInquiry struct {
	ID                 int64              `json:"id"`
	ListingID          int64              `json:"listing_id"`
	Listing            MarketplaceListing `json:"listing"`
	InitiatorUserID    int64              `json:"initiator_user_id"`
	ListingOwnerUserID int64              `json:"listing_owner_user_id"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	LastMessageAt      *time.Time         `json:"last_message_at,omitempty"`
}
type MarketplaceInquiryPage struct {
	Items      []MarketplaceInquiry `json:"items"`
	NextCursor *int64               `json:"next_cursor,omitempty"`
}

type MarketplaceMessageInput struct {
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}
type MarketplaceMessage struct {
	ID              int64     `json:"id"`
	InquiryID       int64     `json:"inquiry_id"`
	SenderUserID    int64     `json:"sender_user_id"`
	ClientMessageID string    `json:"client_message_id"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}
type MarketplaceMessagePage struct {
	Items      []MarketplaceMessage `json:"items"`
	NextCursor *int64               `json:"next_cursor,omitempty"`
}

type MarketplaceRepository interface {
	ListListings(context.Context, MarketplaceListingQuery) (MarketplaceListingPage, error)
	GetListing(context.Context, int64, int64) (*MarketplaceListing, error)
	CreateListing(context.Context, int64, MarketplaceListingInput) (*MarketplaceListing, error)
	UpdateListing(context.Context, int64, int64, MarketplaceListingInput) (*MarketplaceListing, error)
	ArchiveListing(context.Context, int64, int64) (*MarketplaceListing, error)
	ListOwnerListings(context.Context, int64, MarketplacePageQuery) (MarketplaceListingPage, error)
	CreateInquiry(context.Context, int64, int64) (*MarketplaceInquiry, error)
	ListInquiries(context.Context, int64, MarketplacePageQuery) (MarketplaceInquiryPage, error)
	ListMessages(context.Context, int64, int64, MarketplacePageQuery) (MarketplaceMessagePage, error)
	PostMessage(context.Context, int64, int64, MarketplaceMessageInput) (*MarketplaceMessage, error)
	CreateMarketplaceOrder(context.Context, int64, int64, MarketplaceOrderQuoteInput) (*MarketplaceOrder, error)
	ListMarketplaceOrders(context.Context, int64, MarketplacePageQuery) (MarketplaceOrderPage, error)
	GetMarketplaceOrder(context.Context, int64, int64) (*MarketplaceOrder, error)
	TransitionMarketplaceOrder(context.Context, MarketplaceOrderTransition) (*MarketplaceOrder, error)
	CreateMarketplaceOrderReview(context.Context, int64, int64, int, string) (*MarketplaceOrderReview, error)
	AdminSetListingStatus(context.Context, int64, string) (*MarketplaceListing, error)
	ListMarketplaceDisputes(context.Context, int64, MarketplacePageQuery) (MarketplaceOrderPage, error)
	GetMarketplaceDispute(context.Context, int64, int64) (*MarketplaceOrder, error)
	ResolveMarketplaceDispute(context.Context, int64, int64, MarketplaceDisputeResolutionInput) (*MarketplaceOrder, error)
}

type MarketplaceService struct {
	repo               MarketplaceRepository
	fundsAPIKeyService *APIKeyService
}

func NewMarketplaceService(repo MarketplaceRepository) *MarketplaceService {
	return &MarketplaceService{repo: repo}
}

func (s *MarketplaceService) ListListings(ctx context.Context, q MarketplaceListingQuery) (MarketplaceListingPage, error) {
	q = normalizeListingQuery(q)
	return s.repo.ListListings(ctx, q)
}
func (s *MarketplaceService) GetListing(ctx context.Context, id, viewerID int64) (*MarketplaceListing, error) {
	if id <= 0 {
		return nil, badMarketplace("invalid listing id")
	}
	return s.repo.GetListing(ctx, id, viewerID)
}
func (s *MarketplaceService) ListOwnerListings(ctx context.Context, ownerID int64, q MarketplacePageQuery) (MarketplaceListingPage, error) {
	if ownerID <= 0 {
		return MarketplaceListingPage{}, badMarketplace("invalid owner")
	}
	q.Limit = marketplaceLimit(q.Limit)
	return s.repo.ListOwnerListings(ctx, ownerID, q)
}
func (s *MarketplaceService) ListInquiries(ctx context.Context, userID int64, q MarketplacePageQuery) (MarketplaceInquiryPage, error) {
	if userID <= 0 {
		return MarketplaceInquiryPage{}, badMarketplace("invalid user")
	}
	q.Limit = marketplaceLimit(q.Limit)
	return s.repo.ListInquiries(ctx, userID, q)
}
func (s *MarketplaceService) ListMessages(ctx context.Context, inquiryID, userID int64, q MarketplacePageQuery) (MarketplaceMessagePage, error) {
	if inquiryID <= 0 || userID <= 0 {
		return MarketplaceMessagePage{}, badMarketplace("invalid inquiry")
	}
	q.Limit = marketplaceLimit(q.Limit)
	return s.repo.ListMessages(ctx, inquiryID, userID, q)
}

func (s *MarketplaceService) CreateListing(ctx context.Context, ownerID int64, in MarketplaceListingInput) (*MarketplaceListing, error) {
	if ownerID <= 0 {
		return nil, badMarketplace("invalid owner")
	}
	normalized, err := normalizeListingInput(in)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateListing(ctx, ownerID, normalized)
}
func (s *MarketplaceService) UpdateListing(ctx context.Context, id, ownerID int64, in MarketplaceListingInput) (*MarketplaceListing, error) {
	if id <= 0 || ownerID <= 0 {
		return nil, badMarketplace("invalid listing")
	}
	normalized, err := normalizeListingInput(in)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateListing(ctx, id, ownerID, normalized)
}
func (s *MarketplaceService) ArchiveListing(ctx context.Context, id, ownerID int64) (*MarketplaceListing, error) {
	if id <= 0 || ownerID <= 0 {
		return nil, badMarketplace("invalid listing")
	}
	return s.repo.ArchiveListing(ctx, id, ownerID)
}
func (s *MarketplaceService) CreateInquiry(ctx context.Context, listingID, initiatorID int64) (*MarketplaceInquiry, error) {
	if listingID <= 0 || initiatorID <= 0 {
		return nil, badMarketplace("invalid inquiry")
	}
	return s.repo.CreateInquiry(ctx, listingID, initiatorID)
}
func (s *MarketplaceService) PostMessage(ctx context.Context, inquiryID, senderID int64, in MarketplaceMessageInput) (*MarketplaceMessage, error) {
	in.ClientMessageID = strings.TrimSpace(in.ClientMessageID)
	in.Body = strings.TrimSpace(in.Body)
	if inquiryID <= 0 || senderID <= 0 || in.ClientMessageID == "" || utf8.RuneCountInString(in.ClientMessageID) > 100 || in.Body == "" || utf8.RuneCountInString(in.Body) > 4000 {
		return nil, badMarketplace("invalid message")
	}
	return s.repo.PostMessage(ctx, inquiryID, senderID, in)
}
func (s *MarketplaceService) AdminSetListingStatus(ctx context.Context, id int64, status string) (*MarketplaceListing, error) {
	status = strings.TrimSpace(status)
	if id <= 0 || !validListingStatus(status) {
		return nil, badMarketplace("invalid listing status")
	}
	return s.repo.AdminSetListingStatus(ctx, id, status)
}

func normalizeListingInput(in MarketplaceListingInput) (MarketplaceListingInput, error) {
	in.Kind, in.Title, in.Summary = strings.TrimSpace(in.Kind), strings.TrimSpace(in.Title), strings.TrimSpace(in.Summary)
	in.Category, in.PriceText, in.DeliveryText = strings.TrimSpace(in.Category), strings.TrimSpace(in.PriceText), strings.TrimSpace(in.DeliveryText)
	if !validListingKind(in.Kind) || in.Title == "" || in.Summary == "" || in.Category == "" || utf8.RuneCountInString(in.Title) > 160 || utf8.RuneCountInString(in.Summary) > 500 || utf8.RuneCountInString(in.Category) > 80 || utf8.RuneCountInString(in.PriceText) > 120 || utf8.RuneCountInString(in.DeliveryText) > 240 {
		return in, badMarketplace("invalid marketplace listing")
	}
	if in.CanonicalAssetID != nil && *in.CanonicalAssetID <= 0 {
		return in, badMarketplace("invalid canonical asset")
	}
	in.Tags = normalizeMarketplaceTags(in.Tags)
	return in, nil
}
func normalizeMarketplaceTags(tags []string) []string {
	out := make([]string, 0, min(len(tags), 12))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || utf8.RuneCountInString(tag) > 40 {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
		if len(out) == 12 {
			break
		}
	}
	return out
}
func normalizeListingQuery(q MarketplaceListingQuery) MarketplaceListingQuery {
	q.Kind, q.Category, q.Tag = strings.TrimSpace(q.Kind), strings.TrimSpace(q.Category), strings.TrimSpace(q.Tag)
	if !validListingKind(q.Kind) {
		q.Kind = ""
	}
	q.Limit = marketplaceLimit(q.Limit)
	return q
}
func validListingKind(v string) bool { return v == "service" || v == "demand" || v == "talent" }
func validListingStatus(v string) bool {
	return v == MarketplaceListingStatusPublished || v == MarketplaceListingStatusArchived || v == MarketplaceListingStatusTakenDown
}
func marketplaceLimit(v int) int {
	if v <= 0 {
		return 20
	}
	if v > 100 {
		return 100
	}
	return v
}
func badMarketplace(message string) error {
	return infraerrors.BadRequest("MARKETPLACE_INVALID_ARGUMENT", message)
}
