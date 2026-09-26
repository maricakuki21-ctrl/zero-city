package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
	"time"
)

var (
	ErrTavernCommerceClosed    = infraerrors.Forbidden("TAVERN_COMMERCE_CLOSED", "Tavern balance tickets are not enabled")
	ErrTavernCommerceInvalid   = infraerrors.Conflict("TAVERN_COMMERCE_INVALID", "Ticket price, operation or room state changed")
	ErrTavernCommerceForbidden = infraerrors.Forbidden("TAVERN_COMMERCE_FORBIDDEN", "This ticket operation is not permitted")
	ErrTavernCommerceFunds     = infraerrors.Conflict("TAVERN_COMMERCE_FUNDS", "Insufficient available balance")
)

type TavernCommercePolicy struct {
	Enabled  bool   `json:"enabled"`
	Currency string `json:"currency"`
}
type TavernTicketInput struct {
	ExpectedPrice string `json:"expected_price"`
	Currency      string `json:"currency"`
	OperationID   string `json:"operation_id"`
}
type TavernPricingInput struct {
	Price    string `json:"price"`
	Currency string `json:"currency"`
}
type TavernTicket struct {
	ID           int64      `json:"id"`
	RoomID       int64      `json:"room_id"`
	ScriptID     int64      `json:"script_id"`
	BuyerUserID  int64      `json:"buyer_user_id"`
	AuthorUserID int64      `json:"author_user_id"`
	OperationID  string     `json:"operation_id"`
	Title        string     `json:"title"`
	Amount       string     `json:"amount"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ReleasedAt   *time.Time `json:"released_at,omitempty"`
	RefundedAt   *time.Time `json:"refunded_at,omitempty"`
	RefundReason string     `json:"refund_reason"`
}
type TavernTicketQuote struct {
	OwnerID      int64         `json:"owner_id"`
	Status       string        `json:"status"`
	Title        string        `json:"title"`
	RoomID       int64         `json:"room_id"`
	Price        string        `json:"price"`
	Currency     string        `json:"currency"`
	AuthorUserID *int64        `json:"author_user_id,omitempty"`
	Enabled      bool          `json:"enabled"`
	Ticket       *TavernTicket `json:"ticket,omitempty"`
}
type TavernCommerceRepository interface {
	GetTavernCommercePolicy(context.Context) (*TavernCommercePolicy, error)
	SetTavernCommercePolicy(context.Context, int64, bool, string) (*TavernCommercePolicy, error)
	SetTavernScriptPricing(context.Context, int64, int64, TavernPricingInput) (*TavernScript, error)
	GetTavernTicketQuote(context.Context, int64, int64) (*TavernTicketQuote, error)
	PurchaseTavernTicket(context.Context, int64, int64, TavernTicketInput) (*TavernTicket, error)
	RefundTavernTicket(context.Context, int64, int64, int64) (*TavernTicket, error)
	ListTavernTicketBuyers(context.Context, int64, int64) ([]int64, error)
}

func (s *BizDecipherService) TavernCommerceRepo() (TavernCommerceRepository, error) {
	r, ok := s.repo.(TavernCommerceRepository)
	if !ok {
		return nil, ErrTavernCommerceClosed
	}
	return r, nil
}
func NormalizeTavernTicket(in TavernTicketInput) (TavernTicketInput, error) {
	price, err := NormalizeColumnPrice(in.ExpectedPrice, false)
	if err != nil || in.Currency != "USD" || !columnOperationPattern.MatchString(in.OperationID) {
		return in, ErrTavernCommerceInvalid
	}
	in.ExpectedPrice = price
	return in, nil
}
func (s *BizDecipherService) PurchaseTavernTicket(ctx context.Context, room, actor int64, in TavernTicketInput) (*TavernTicket, error) {
	in, err := NormalizeTavernTicket(in)
	if err != nil {
		return nil, err
	}
	r, err := s.TavernCommerceRepo()
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, actor); err != nil {
		return nil, err
	}
	t, err := r.PurchaseTavernTicket(ctx, room, actor, in)
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, actor); err != nil {
		return nil, err
	}
	return t, nil
}
func (s *BizDecipherService) RefundTavernTicket(ctx context.Context, room, actor, ticketID int64) (*TavernTicket, error) {
	if ticketID <= 0 {
		return nil, ErrTavernCommerceInvalid
	}
	r, err := s.TavernCommerceRepo()
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, actor); err != nil {
		return nil, err
	}
	t, err := r.RefundTavernTicket(ctx, room, actor, ticketID)
	if err != nil {
		return nil, err
	}
	if err = s.columnCommerceInvalidateBalance(ctx, actor); err != nil {
		return nil, err
	}
	return t, nil
}
func (s *BizDecipherService) prepareTavernRefunds(ctx context.Context, room, actor int64) ([]int64, error) {
	r, ok := s.repo.(TavernCommerceRepository)
	if !ok {
		return nil, nil
	}
	ids, err := r.ListTavernTicketBuyers(ctx, room, actor)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err = s.columnCommerceInvalidateBalance(ctx, id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
func NormalizeTavernPricing(in TavernPricingInput) (TavernPricingInput, error) {
	price, err := NormalizeColumnPrice(strings.TrimSpace(in.Price), true)
	if err != nil || in.Currency != "USD" {
		return in, ErrTavernCommerceInvalid
	}
	in.Price = price
	return in, nil
}
