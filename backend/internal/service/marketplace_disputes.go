package service

import (
	"context"
	"strings"
	"unicode/utf8"
)

type MarketplaceDisputeResolutionInput struct {
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
}

func (s *MarketplaceService) ListDisputes(ctx context.Context, adminID int64, q MarketplacePageQuery) (MarketplaceOrderPage, error) {
	if adminID <= 0 {
		return MarketplaceOrderPage{}, ErrMarketplaceOrderRoleDenied
	}
	q.Limit = marketplaceLimit(q.Limit)
	return s.repo.ListMarketplaceDisputes(ctx, adminID, q)
}

func (s *MarketplaceService) GetDispute(ctx context.Context, orderID, adminID int64) (*MarketplaceOrder, error) {
	if orderID <= 0 || adminID <= 0 {
		return nil, badMarketplace("invalid dispute")
	}
	return s.repo.GetMarketplaceDispute(ctx, orderID, adminID)
}

func (s *MarketplaceService) ResolveDispute(ctx context.Context, orderID, adminID int64, in MarketplaceDisputeResolutionInput) (*MarketplaceOrder, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if orderID <= 0 || adminID <= 0 ||
		(in.Outcome != MarketplaceOrderStatusConfirmed && in.Outcome != MarketplaceOrderStatusCanceled) ||
		in.Reason == "" || utf8.RuneCountInString(in.Reason) > 1000 {
		return nil, badMarketplace("dispute resolution requires confirmed/canceled and a reason of 1-1000 characters")
	}
	order, err := s.repo.GetMarketplaceDispute(ctx, orderID, adminID)
	if err != nil {
		return nil, err
	}
	funded, err := s.prepareMarketplaceFunds(ctx, order, adminID)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.ResolveMarketplaceDispute(ctx, orderID, adminID, in)
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
