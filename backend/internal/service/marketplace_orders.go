package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MarketplaceOrderStatusQuoted    = "quoted"
	MarketplaceOrderStatusConfirmed = "confirmed"
	MarketplaceOrderStatusDelivered = "delivered"
	MarketplaceOrderStatusAccepted  = "accepted"
	MarketplaceOrderStatusSettled   = "settled"
	MarketplaceOrderStatusCanceled  = "canceled"
	MarketplaceOrderStatusDisputed  = "disputed"

	MarketplaceOrderRoleBuyer  = "buyer"
	MarketplaceOrderRoleSeller = "seller"
)

var (
	ErrMarketplaceOrderNotFound     = infraerrors.NotFound("MARKETPLACE_ORDER_NOT_FOUND", "marketplace order not found")
	ErrMarketplaceOrderForbidden    = infraerrors.Forbidden("MARKETPLACE_ORDER_FORBIDDEN", "marketplace order is private to its participants")
	ErrMarketplaceOrderInvalidState = infraerrors.Conflict("MARKETPLACE_ORDER_INVALID_STATE", "marketplace order is not in a state that allows this action")
	ErrMarketplaceOrderRoleDenied   = infraerrors.Forbidden("MARKETPLACE_ORDER_ROLE_DENIED", "this account cannot perform that order action")
	ErrMarketplaceOrderReviewExists = infraerrors.Conflict("MARKETPLACE_ORDER_REVIEW_EXISTS", "this account already reviewed the order")
	ErrMarketplaceOrderExists       = infraerrors.Conflict("MARKETPLACE_ORDER_EXISTS", "marketplace inquiry already has an active or completed order")
)

type MarketplaceOrderQuoteInput struct {
	ScopeText     string `json:"scope_text"`
	AmountText    string `json:"amount_text"`
	DeliveryText  string `json:"delivery_text"`
	RevisionLimit int    `json:"revision_limit"`
}

type MarketplaceOrderActionInput struct {
	Action string `json:"action"`
	Note   string `json:"note"`
	Rating int    `json:"rating"`
}

type MarketplaceOrderEvent struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"order_id"`
	ActorUserID int64     `json:"actor_user_id"`
	ActorName   string    `json:"actor_name"`
	Event       string    `json:"event"`
	FromStatus  string    `json:"from_status"`
	ToStatus    string    `json:"to_status"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

type MarketplaceOrderReview struct {
	ID           int64     `json:"id"`
	OrderID      int64     `json:"order_id"`
	AuthorUserID int64     `json:"author_user_id"`
	AuthorName   string    `json:"author_name"`
	Rating       int       `json:"rating"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

type MarketplaceOrder struct {
	ID                int64      `json:"id"`
	InquiryID         int64      `json:"inquiry_id"`
	ListingID         int64      `json:"listing_id"`
	ListingTitle      string     `json:"listing_title"`
	BuyerUserID       int64      `json:"buyer_user_id"`
	BuyerDisplayName  string     `json:"buyer_display_name"`
	SellerUserID      int64      `json:"seller_user_id"`
	SellerDisplayName string     `json:"seller_display_name"`
	Status            string     `json:"status"`
	LegacyCooperation bool       `json:"legacy_cooperation"`
	ScopeText         string     `json:"scope_text"`
	AmountText        string     `json:"amount_text"`
	DeliveryText      string     `json:"delivery_text"`
	RevisionLimit     int        `json:"revision_limit"`
	DeliveryNote      string     `json:"delivery_note"`
	AcceptNote        string     `json:"accept_note"`
	DisputeNote       string     `json:"dispute_note"`
	CancelReason      string     `json:"cancel_reason"`
	QuotedAt          time.Time  `json:"quoted_at"`
	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty"`
	DeliveredAt       *time.Time `json:"delivered_at,omitempty"`
	AcceptedAt        *time.Time `json:"accepted_at,omitempty"`
	SettledAt         *time.Time `json:"settled_at,omitempty"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	// ViewerRole and AvailableActions are resolved per requesting account so the
	// client never has to guess who may do what.
	ViewerRole       string                   `json:"viewer_role"`
	AvailableActions []string                 `json:"available_actions"`
	Events           []MarketplaceOrderEvent  `json:"events,omitempty"`
	Reviews          []MarketplaceOrderReview `json:"reviews,omitempty"`
}

type MarketplaceOrderPage struct {
	Items      []MarketplaceOrder `json:"items"`
	NextCursor *int64             `json:"next_cursor,omitempty"`
}

type MarketplaceOrderTransition struct {
	OrderID         int64
	ActorID         int64
	Action          string
	FromStatuses    []string
	ToStatus        string
	TimestampColumn string
	NoteColumn      string
	Note            string
}

// orderActionRule encodes the whole state machine in one place: which role may
// move an order, from which statuses, to which status, and what gets recorded.
type orderActionRule struct {
	Action          string
	From            []string
	To              string
	Roles           []string
	TimestampColumn string
	NoteColumn      string
	RequiresNote    bool
}

func marketplaceOrderActionRules() map[string]orderActionRule {
	return map[string]orderActionRule{
		"confirm": {
			Action: "confirm", From: []string{MarketplaceOrderStatusQuoted}, To: MarketplaceOrderStatusConfirmed,
			Roles: []string{MarketplaceOrderRoleBuyer}, TimestampColumn: "confirmed_at",
		},
		"deliver": {
			Action: "deliver", From: []string{MarketplaceOrderStatusConfirmed}, To: MarketplaceOrderStatusDelivered,
			Roles: []string{MarketplaceOrderRoleSeller}, TimestampColumn: "delivered_at",
			NoteColumn: "delivery_note", RequiresNote: true,
		},
		"accept": {
			Action: "accept", From: []string{MarketplaceOrderStatusDelivered}, To: MarketplaceOrderStatusAccepted,
			Roles: []string{MarketplaceOrderRoleBuyer}, TimestampColumn: "accepted_at", NoteColumn: "accept_note",
		},
		"settle": {
			Action: "settle", From: []string{MarketplaceOrderStatusAccepted}, To: MarketplaceOrderStatusSettled,
			Roles: []string{MarketplaceOrderRoleBuyer, MarketplaceOrderRoleSeller}, TimestampColumn: "settled_at",
		},
		"cancel": {
			Action: "cancel", From: []string{MarketplaceOrderStatusQuoted, MarketplaceOrderStatusConfirmed}, To: MarketplaceOrderStatusCanceled,
			Roles: []string{MarketplaceOrderRoleBuyer, MarketplaceOrderRoleSeller}, TimestampColumn: "canceled_at",
			NoteColumn: "cancel_reason", RequiresNote: true,
		},
		"dispute": {
			Action: "dispute", From: []string{MarketplaceOrderStatusConfirmed, MarketplaceOrderStatusDelivered, MarketplaceOrderStatusAccepted}, To: MarketplaceOrderStatusDisputed,
			Roles: []string{MarketplaceOrderRoleBuyer, MarketplaceOrderRoleSeller}, NoteColumn: "dispute_note",
			RequiresNote: true,
		},
	}
}

func MarketplaceOrderViewerRole(order *MarketplaceOrder, userID int64) string {
	if order == nil {
		return ""
	}
	switch userID {
	case order.BuyerUserID:
		return MarketplaceOrderRoleBuyer
	case order.SellerUserID:
		return MarketplaceOrderRoleSeller
	default:
		return ""
	}
}

// MarketplaceOrderActions lists the actions an account may take right now. It is
// used both to drive the UI and to reject requests the server would refuse.
func MarketplaceOrderActions(order *MarketplaceOrder, userID int64) []string {
	role := MarketplaceOrderViewerRole(order, userID)
	if role == "" {
		return nil
	}
	actions := make([]string, 0, 4)
	rules := marketplaceOrderActionRules()
	for _, key := range []string{"confirm", "deliver", "accept", "settle", "cancel", "dispute"} {
		if key == "confirm" && !order.LegacyCooperation {
			continue
		}
		rule := rules[key]
		if !containsString(rule.From, order.Status) || !containsString(rule.Roles, role) {
			continue
		}
		actions = append(actions, rule.Action)
	}
	if (order.Status == MarketplaceOrderStatusAccepted || order.Status == MarketplaceOrderStatusSettled) && !orderHasReview(order, userID) {
		actions = append(actions, "review")
	}
	return actions
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func orderHasReview(order *MarketplaceOrder, userID int64) bool {
	for _, review := range order.Reviews {
		if review.AuthorUserID == userID {
			return true
		}
	}
	return false
}

func (s *MarketplaceService) QuoteOrder(ctx context.Context, inquiryID, sellerID int64, in MarketplaceOrderQuoteInput) (*MarketplaceOrder, error) {
	in.ScopeText = strings.TrimSpace(in.ScopeText)
	in.AmountText = strings.TrimSpace(in.AmountText)
	in.DeliveryText = strings.TrimSpace(in.DeliveryText)
	if inquiryID <= 0 || sellerID <= 0 || in.ScopeText == "" || utf8.RuneCountInString(in.ScopeText) > 2000 ||
		utf8.RuneCountInString(in.AmountText) > 120 || utf8.RuneCountInString(in.DeliveryText) > 240 ||
		in.RevisionLimit < 0 || in.RevisionLimit > 10 {
		return nil, badMarketplace("invalid marketplace order quote")
	}
	return s.repo.CreateMarketplaceOrder(ctx, inquiryID, sellerID, in)
}

func (s *MarketplaceService) ListOrders(ctx context.Context, userID int64, q MarketplacePageQuery) (MarketplaceOrderPage, error) {
	if userID <= 0 {
		return MarketplaceOrderPage{}, badMarketplace("invalid user")
	}
	q.Limit = marketplaceLimit(q.Limit)
	return s.repo.ListMarketplaceOrders(ctx, userID, q)
}

func (s *MarketplaceService) GetOrder(ctx context.Context, orderID, userID int64) (*MarketplaceOrder, error) {
	if orderID <= 0 || userID <= 0 {
		return nil, badMarketplace("invalid order")
	}
	return s.repo.GetMarketplaceOrder(ctx, orderID, userID)
}

func (s *MarketplaceService) ApplyOrderAction(ctx context.Context, orderID, actorID int64, in MarketplaceOrderActionInput) (*MarketplaceOrder, error) {
	in.Action = strings.TrimSpace(in.Action)
	in.Note = strings.TrimSpace(in.Note)
	if orderID <= 0 || actorID <= 0 || in.Action == "" {
		return nil, badMarketplace("invalid order action")
	}
	order, err := s.repo.GetMarketplaceOrder(ctx, orderID, actorID)
	if err != nil {
		return nil, err
	}
	role := MarketplaceOrderViewerRole(order, actorID)
	if role == "" {
		return nil, ErrMarketplaceOrderForbidden
	}
	// Retry also repairs a prior post-commit cache failure on a terminal order.
	funded, err := s.prepareMarketplaceFunds(ctx, order, actorID)
	if err != nil {
		return nil, err
	}
	if in.Action == "review" {
		if in.Rating < 1 || in.Rating > 5 || utf8.RuneCountInString(in.Note) > 2000 {
			return nil, badMarketplace("invalid order review")
		}
		if order.Status != MarketplaceOrderStatusAccepted && order.Status != MarketplaceOrderStatusSettled {
			return nil, ErrMarketplaceOrderInvalidState
		}
		if _, err := s.repo.CreateMarketplaceOrderReview(ctx, orderID, actorID, in.Rating, in.Note); err != nil {
			return nil, err
		}
		return s.repo.GetMarketplaceOrder(ctx, orderID, actorID)
	}
	rule, ok := marketplaceOrderActionRules()[in.Action]
	if !ok {
		return nil, badMarketplace("unknown order action")
	}
	if !containsString(rule.Roles, role) {
		return nil, ErrMarketplaceOrderRoleDenied
	}
	if !containsString(rule.From, order.Status) {
		return nil, ErrMarketplaceOrderInvalidState
	}
	if in.Action == "confirm" && !order.LegacyCooperation {
		return nil, ErrMarketplaceFundsInvalid
	}
	if rule.RequiresNote && in.Note == "" {
		return nil, badMarketplace("this action needs a note")
	}
	if utf8.RuneCountInString(in.Note) > 2000 {
		return nil, badMarketplace("order note is too long")
	}
	result, err := s.repo.TransitionMarketplaceOrder(ctx, MarketplaceOrderTransition{
		OrderID: orderID, ActorID: actorID, Action: rule.Action,
		FromStatuses: rule.From, ToStatus: rule.To,
		TimestampColumn: rule.TimestampColumn, NoteColumn: rule.NoteColumn, Note: in.Note,
	})
	if err != nil {
		return nil, err
	}
	if funded {
		if err = s.invalidateMarketplaceFunds(ctx, order.BuyerUserID); err != nil {
			return nil, err
		}
	}
	return result, nil
}
