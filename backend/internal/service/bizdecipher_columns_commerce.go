package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrColumnCommerceInvalid   = infraerrors.BadRequest("COLUMN_COMMERCE_INVALID", "Invalid column commerce request")
	ErrColumnCommerceClosed    = infraerrors.Forbidden("COLUMN_COMMERCE_CLOSED", "Paid column purchases are not enabled")
	ErrColumnCommerceConflict  = infraerrors.Conflict("COLUMN_COMMERCE_CONFLICT", "Price, purchase state, or idempotency payload changed")
	ErrColumnCommerceForbidden = infraerrors.Forbidden("COLUMN_COMMERCE_FORBIDDEN", "This column commerce operation is not permitted")
	ErrColumnCommerceFunds     = infraerrors.Conflict("COLUMN_COMMERCE_FUNDS", "Insufficient available balance")
)

type ColumnCommercePolicy struct {
	Enabled     bool   `json:"enabled"`
	Currency    string `json:"currency"`
	PlatformFee string `json:"platform_fee"`
}
type ColumnPricingInput struct {
	Mode  string `json:"mode"`
	Price string `json:"price"`
}
type ColumnPurchaseInput struct {
	OperationID   string `json:"operation_id"`
	ExpectedPrice string `json:"expected_price"`
}
type ColumnRefundInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
}
type ColumnPurchase struct {
	ID            int64      `json:"id"`
	ColumnID      int64      `json:"column_id"`
	BuyerUserID   int64      `json:"buyer_user_id"`
	OwnerUserID   int64      `json:"owner_user_id"`
	OperationID   string     `json:"operation_id"`
	ColumnTitle   string     `json:"column_title"`
	Amount        string     `json:"amount"`
	PlatformFee   string     `json:"platform_fee"`
	CreatorAmount string     `json:"creator_amount"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	RefundedAt    *time.Time `json:"refunded_at,omitempty"`
	RefundReason  string     `json:"refund_reason"`
}
type columnCommerceRepository interface {
	GetColumnCommercePolicy(context.Context) (*ColumnCommercePolicy, error)
	SetColumnCommercePolicy(context.Context, int64, bool, string) (*ColumnCommercePolicy, error)
	SetColumnPricing(context.Context, int64, int64, ColumnPricingInput) (*CreatorColumn, error)
	PurchaseColumn(context.Context, int64, int64, ColumnPurchaseInput) (*ColumnPurchase, error)
	RefundColumnPurchase(context.Context, int64, int64, int64, ColumnRefundInput) (*ColumnPurchase, error)
	ListColumnPurchases(context.Context, int64, CreatorColumnQuery) ([]ColumnPurchase, error)
}

func (s *BizDecipherService) ColumnCommerceRepo() (columnCommerceRepository, error) {
	if s != nil && s.repo != nil {
		if r, ok := s.repo.(columnCommerceRepository); ok {
			return r, nil
		}
	}
	return nil, ErrCreatorColumnUnavailable
}

var columnPricePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,8})?$`)
var columnOperationPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,100}$`)

func NormalizeColumnPrice(price string, allowZero bool) (string, error) {
	if !columnPricePattern.MatchString(price) {
		return "", ErrColumnCommerceInvalid
	}
	d, err := decimal.NewFromString(price)
	if err != nil || d.IsNegative() || (!allowZero && d.IsZero()) {
		return "", ErrColumnCommerceInvalid
	}
	return d.StringFixed(8), nil
}
func NormalizeColumnPricing(in ColumnPricingInput) (ColumnPricingInput, error) {
	if in.Mode != "free" && in.Mode != "paid" {
		return in, ErrColumnCommerceInvalid
	}
	price, err := NormalizeColumnPrice(in.Price, in.Mode == "free")
	if err != nil {
		return in, err
	}
	if in.Mode == "free" && price != "0.00000000" {
		return in, ErrColumnCommerceInvalid
	}
	in.Price = price
	return in, nil
}
func NormalizeColumnPurchase(in ColumnPurchaseInput) (ColumnPurchaseInput, error) {
	if !columnOperationPattern.MatchString(in.OperationID) {
		return in, ErrColumnCommerceInvalid
	}
	price, err := NormalizeColumnPrice(in.ExpectedPrice, false)
	in.ExpectedPrice = price
	return in, err
}
func NormalizeColumnRefund(in ColumnRefundInput) (ColumnRefundInput, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if !columnOperationPattern.MatchString(in.OperationID) || in.Reason == "" || len([]rune(in.Reason)) > 1000 {
		return in, ErrColumnCommerceInvalid
	}
	return in, nil
}

type columnBalanceInvalidator interface {
	InvalidateUserBalance(context.Context, int64) error
}

func (s *BizDecipherService) columnCommerceInvalidateBalance(ctx context.Context, userID int64) error {
	// ProvideAPIKeyService already attaches the shared BillingCacheService
	// as rateLimitCacheInvalid. Reuse that instance and its balance hook.
	if s == nil || s.apiKeyService == nil {
		return ErrCreatorColumnUnavailable
	}
	cache, ok := s.apiKeyService.rateLimitCacheInvalid.(columnBalanceInvalidator)
	if !ok {
		return ErrCreatorColumnUnavailable
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := cache.InvalidateUserBalance(cacheCtx, userID); err != nil {
		return ErrCreatorColumnUnavailable
	}
	s.apiKeyService.InvalidateAuthCacheByUserID(cacheCtx, userID)
	return nil
}
func (s *BizDecipherService) PurchaseColumn(ctx context.Context, id, buyer int64, in ColumnPurchaseInput) (*ColumnPurchase, error) {
	r, err := s.ColumnCommerceRepo()
	if err != nil {
		return nil, err
	}
	// Fail closed before charging when cache invalidation is unavailable.
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	p, err := r.PurchaseColumn(ctx, id, buyer, in)
	if err != nil {
		return nil, err
	}
	// An exact retry repairs a failed post-commit invalidation without charging again.
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *BizDecipherService) RefundColumnPurchase(ctx context.Context, columnID, purchaseID, actor int64, in ColumnRefundInput) (*ColumnPurchase, error) {
	r, err := s.ColumnCommerceRepo()
	if err != nil {
		return nil, err
	}
	// The actor check is repeated in the repository transaction. Resolve the
	// buyer from the administrator-only purchase query, never client input.
	items, err := r.ListColumnPurchases(ctx, columnID, CreatorColumnQuery{ViewerID: actor, Admin: true, Cursor: purchaseID + 1, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(items) != 1 || items[0].ID != purchaseID {
		return nil, ErrCreatorColumnNotFound
	}
	buyer := items[0].BuyerUserID
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	p, err := r.RefundColumnPurchase(ctx, columnID, purchaseID, actor, in)
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, buyer); err != nil {
		return nil, err
	}
	return p, nil
}
