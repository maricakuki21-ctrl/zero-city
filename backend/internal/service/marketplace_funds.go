package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrMarketplaceFundsClosed       = infraerrors.Forbidden("MARKETPLACE_FUNDS_CLOSED", "Marketplace balance payments are not enabled")
	ErrMarketplaceFundsInvalid      = infraerrors.Conflict("MARKETPLACE_FUNDS_INVALID", "Payment amount, state or idempotency key changed")
	ErrMarketplaceFundsInsufficient = infraerrors.Conflict("MARKETPLACE_FUNDS_INSUFFICIENT", "Insufficient available balance")
)

type MarketplaceFunds struct {
	OrderID     int64      `json:"order_id"`
	Amount      string     `json:"amount"`
	Currency    string     `json:"currency"`
	Status      string     `json:"status"`
	OperationID *string    `json:"operation_id,omitempty"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
	RefundedAt  *time.Time `json:"refunded_at,omitempty"`
}
type MarketplaceFundsInput struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
type MarketplacePaymentInput struct {
	ExpectedAmount string `json:"expected_amount"`
	Currency       string `json:"currency"`
	OperationID    string `json:"operation_id"`
}
type MarketplaceFundsPolicy struct {
	Enabled  bool   `json:"enabled"`
	Currency string `json:"currency"`
}
type MarketplaceFundsPolicyInput struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}
type MarketplaceFundsRepository interface {
	GetMarketplaceFunds(context.Context, int64, int64) (*MarketplaceFunds, error)
	SetMarketplaceFunds(context.Context, int64, int64, MarketplaceFundsInput) (*MarketplaceFunds, error)
	PayMarketplaceOrder(context.Context, int64, int64, MarketplacePaymentInput) (*MarketplaceFunds, error)
	GetMarketplaceFundsPolicy(context.Context) (*MarketplaceFundsPolicy, error)
	SetMarketplaceFundsPolicy(context.Context, int64, MarketplaceFundsPolicyInput) (*MarketplaceFundsPolicy, error)
}

func NormalizeMarketplacePayment(in MarketplacePaymentInput) (MarketplacePaymentInput, error) {
	amount, err := NormalizeColumnPrice(in.ExpectedAmount, false)
	if err != nil || in.Currency != "USD" || !columnOperationPattern.MatchString(in.OperationID) {
		return in, ErrMarketplaceFundsInvalid
	}
	in.ExpectedAmount = amount
	return in, nil
}
func (s *MarketplaceService) SetFundsAPIKeyService(api *APIKeyService) { s.fundsAPIKeyService = api }
func ProvideMarketplaceService(repo MarketplaceRepository, api *APIKeyService) *MarketplaceService {
	s := NewMarketplaceService(repo)
	s.SetFundsAPIKeyService(api)
	return s
}
func (s *MarketplaceService) invalidateMarketplaceFunds(ctx context.Context, buyer int64) error {
	if s.fundsAPIKeyService == nil {
		return ErrMarketplaceFundsClosed
	}
	cache, ok := s.fundsAPIKeyService.rateLimitCacheInvalid.(columnBalanceInvalidator)
	if !ok {
		return ErrMarketplaceFundsClosed
	}
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := cache.InvalidateUserBalance(c, buyer); err != nil {
		return err
	}
	s.fundsAPIKeyService.InvalidateAuthCacheByUserID(c, buyer)
	return nil
}
func (s *MarketplaceService) FundsRepo() (MarketplaceFundsRepository, error) {
	r, ok := s.repo.(MarketplaceFundsRepository)
	if !ok {
		return nil, ErrMarketplaceFundsClosed
	}
	return r, nil
}
func (s *MarketplaceService) GetOrderFunds(ctx context.Context, id, actor int64) (*MarketplaceFunds, error) {
	r, err := s.FundsRepo()
	if err != nil {
		return nil, err
	}
	return r.GetMarketplaceFunds(ctx, id, actor)
}
func (s *MarketplaceService) SetOrderFunds(ctx context.Context, id, actor int64, in MarketplaceFundsInput) (*MarketplaceFunds, error) {
	r, err := s.FundsRepo()
	if err != nil {
		return nil, err
	}
	return r.SetMarketplaceFunds(ctx, id, actor, in)
}
func (s *MarketplaceService) PayOrder(ctx context.Context, id, actor int64, in MarketplacePaymentInput) (*MarketplaceFunds, error) {
	in, err := NormalizeMarketplacePayment(in)
	if err != nil {
		return nil, err
	}
	r, err := s.FundsRepo()
	if err != nil {
		return nil, err
	}
	if err = s.invalidateMarketplaceFunds(ctx, actor); err != nil {
		return nil, err
	}
	f, err := r.PayMarketplaceOrder(ctx, id, actor, in)
	if err != nil {
		return nil, err
	}
	if err = s.invalidateMarketplaceFunds(ctx, actor); err != nil {
		return nil, err
	}
	return f, nil
}
func (s *MarketplaceService) GetFundsPolicy(ctx context.Context) (*MarketplaceFundsPolicy, error) {
	r, err := s.FundsRepo()
	if err != nil {
		return nil, err
	}
	return r.GetMarketplaceFundsPolicy(ctx)
}
func (s *MarketplaceService) SetFundsPolicy(ctx context.Context, actor int64, in MarketplaceFundsPolicyInput) (*MarketplaceFundsPolicy, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" || len([]rune(in.Reason)) > 1000 {
		return nil, ErrMarketplaceFundsInvalid
	}
	r, err := s.FundsRepo()
	if err != nil {
		return nil, err
	}
	return r.SetMarketplaceFundsPolicy(ctx, actor, in)
}

// Legacy orders have no payment row and never trigger a balance mutation.
func (s *MarketplaceService) prepareMarketplaceFunds(ctx context.Context, order *MarketplaceOrder, actor int64) (bool, error) {
	r, ok := s.repo.(MarketplaceFundsRepository)
	if !ok {
		return false, nil
	}
	f, err := r.GetMarketplaceFunds(ctx, order.ID, actor)
	if err != nil {
		return false, err
	}
	if f == nil || f.Status == "unpaid" {
		return false, nil
	}
	return true, s.invalidateMarketplaceFunds(ctx, order.BuyerUserID)
}
