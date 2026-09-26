package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type marketplaceRepoStub struct {
	disputeInput         MarketplaceDisputeResolutionInput
	listing              *MarketplaceListing
	message              *MarketplaceMessage
	order                *MarketplaceOrder
	createInquiryErr     error
	postMessageErr       error
	createOrderErr       error
	transitionOrderErr   error
	createOrderReviewErr error
}

func (s *marketplaceRepoStub) ListMarketplaceDisputes(context.Context, int64, MarketplacePageQuery) (MarketplaceOrderPage, error) {
	return MarketplaceOrderPage{}, nil
}
func (s *marketplaceRepoStub) GetMarketplaceDispute(context.Context, int64, int64) (*MarketplaceOrder, error) {
	return s.order, nil
}
func (s *marketplaceRepoStub) ResolveMarketplaceDispute(_ context.Context, _, _ int64, in MarketplaceDisputeResolutionInput) (*MarketplaceOrder, error) {
	s.disputeInput = in
	return s.order, s.transitionOrderErr
}

func TestMarketplaceDisputeResolutionValidation(t *testing.T) {
	for _, in := range []MarketplaceDisputeResolutionInput{
		{Outcome: "settled", Reason: "reason"}, {Outcome: "confirmed", Reason: " "}, {Outcome: "canceled", Reason: string(make([]rune, 1001))},
	} {
		_, err := NewMarketplaceService(&marketplaceRepoStub{}).ResolveDispute(context.Background(), 1, 2, in)
		require.Error(t, err)
	}
	for _, outcome := range []string{"confirmed", "canceled"} {
		stub := &marketplaceRepoStub{order: &MarketplaceOrder{Status: outcome}}
		_, err := NewMarketplaceService(stub).ResolveDispute(context.Background(), 1, 2, MarketplaceDisputeResolutionInput{Outcome: outcome, Reason: " agreed "})
		require.NoError(t, err)
		require.Equal(t, "agreed", stub.disputeInput.Reason)
	}
	stub := &marketplaceRepoStub{transitionOrderErr: ErrMarketplaceOrderInvalidState}
	_, err := NewMarketplaceService(stub).ResolveDispute(context.Background(), 1, 2, MarketplaceDisputeResolutionInput{Outcome: "confirmed", Reason: "reason"})
	require.ErrorIs(t, err, ErrMarketplaceOrderInvalidState)
}

func (s *marketplaceRepoStub) ListListings(context.Context, MarketplaceListingQuery) (MarketplaceListingPage, error) {
	return MarketplaceListingPage{}, nil
}
func (s *marketplaceRepoStub) GetListing(context.Context, int64, int64) (*MarketplaceListing, error) {
	return s.listing, nil
}
func (s *marketplaceRepoStub) CreateListing(context.Context, int64, MarketplaceListingInput) (*MarketplaceListing, error) {
	return s.listing, nil
}
func (s *marketplaceRepoStub) UpdateListing(context.Context, int64, int64, MarketplaceListingInput) (*MarketplaceListing, error) {
	return s.listing, nil
}
func (s *marketplaceRepoStub) ArchiveListing(context.Context, int64, int64) (*MarketplaceListing, error) {
	return s.listing, nil
}
func (s *marketplaceRepoStub) ListOwnerListings(context.Context, int64, MarketplacePageQuery) (MarketplaceListingPage, error) {
	return MarketplaceListingPage{}, nil
}
func (s *marketplaceRepoStub) CreateInquiry(context.Context, int64, int64) (*MarketplaceInquiry, error) {
	return nil, s.createInquiryErr
}
func (s *marketplaceRepoStub) ListInquiries(context.Context, int64, MarketplacePageQuery) (MarketplaceInquiryPage, error) {
	return MarketplaceInquiryPage{}, nil
}
func (s *marketplaceRepoStub) ListMessages(context.Context, int64, int64, MarketplacePageQuery) (MarketplaceMessagePage, error) {
	return MarketplaceMessagePage{}, nil
}
func (s *marketplaceRepoStub) PostMessage(context.Context, int64, int64, MarketplaceMessageInput) (*MarketplaceMessage, error) {
	return s.message, s.postMessageErr
}
func (s *marketplaceRepoStub) CreateMarketplaceOrder(context.Context, int64, int64, MarketplaceOrderQuoteInput) (*MarketplaceOrder, error) {
	return s.order, s.createOrderErr
}
func (s *marketplaceRepoStub) ListMarketplaceOrders(context.Context, int64, MarketplacePageQuery) (MarketplaceOrderPage, error) {
	if s.order == nil {
		return MarketplaceOrderPage{}, nil
	}
	return MarketplaceOrderPage{Items: []MarketplaceOrder{*s.order}}, nil
}
func (s *marketplaceRepoStub) GetMarketplaceOrder(context.Context, int64, int64) (*MarketplaceOrder, error) {
	return s.order, nil
}
func (s *marketplaceRepoStub) TransitionMarketplaceOrder(context.Context, MarketplaceOrderTransition) (*MarketplaceOrder, error) {
	return s.order, s.transitionOrderErr
}
func (s *marketplaceRepoStub) CreateMarketplaceOrderReview(context.Context, int64, int64, int, string) (*MarketplaceOrderReview, error) {
	return &MarketplaceOrderReview{}, s.createOrderReviewErr
}
func (s *marketplaceRepoStub) AdminSetListingStatus(context.Context, int64, string) (*MarketplaceListing, error) {
	return s.listing, nil
}

func TestMarketplaceServiceCreateListing_whenKindInvalid(t *testing.T) {
	// Given
	svc := NewMarketplaceService(&marketplaceRepoStub{})

	// When
	_, err := svc.CreateListing(context.Background(), 7, MarketplaceListingInput{
		Kind: "other", Title: "Title", Summary: "Summary", Category: "general",
	})

	// Then
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestMarketplaceServiceCreateInquiry_whenRepositoryRejectsSelfInquiry(t *testing.T) {
	// Given
	svc := NewMarketplaceService(&marketplaceRepoStub{createInquiryErr: ErrMarketplaceSelfInquiry})

	// When
	_, err := svc.CreateInquiry(context.Background(), 15, 8)

	// Then
	require.ErrorIs(t, err, ErrMarketplaceSelfInquiry)
	require.True(t, infraerrors.IsConflict(err))
}

func TestMarketplaceServicePostMessage_whenClientMessageIDReusedWithDifferentBody(t *testing.T) {
	// Given
	svc := NewMarketplaceService(&marketplaceRepoStub{postMessageErr: ErrMarketplaceMessageIdempotencyConflict})

	// When
	_, err := svc.PostMessage(context.Background(), 9, 17, MarketplaceMessageInput{
		ClientMessageID: "retry-1", Body: "different body",
	})

	// Then
	require.ErrorIs(t, err, ErrMarketplaceMessageIdempotencyConflict)
	require.True(t, infraerrors.IsConflict(err))
}

func TestMarketplaceServiceQuoteOrder_whenScopeMissing(t *testing.T) {
	svc := NewMarketplaceService(&marketplaceRepoStub{})

	_, err := svc.QuoteOrder(context.Background(), 9, 17, MarketplaceOrderQuoteInput{})

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestMarketplaceServiceQuoteOrder_whenInquiryAlreadyHasOrder(t *testing.T) {
	svc := NewMarketplaceService(&marketplaceRepoStub{createOrderErr: ErrMarketplaceOrderExists})

	_, err := svc.QuoteOrder(context.Background(), 9, 17, MarketplaceOrderQuoteInput{ScopeText: "交付范围"})

	require.ErrorIs(t, err, ErrMarketplaceOrderExists)
	require.True(t, infraerrors.IsConflict(err))
}

func TestMarketplaceServiceApplyOrderAction_rejectsWrongRole(t *testing.T) {
	svc := NewMarketplaceService(&marketplaceRepoStub{order: &MarketplaceOrder{
		ID: 51, BuyerUserID: 17, SellerUserID: 23, Status: MarketplaceOrderStatusQuoted,
	}})

	_, err := svc.ApplyOrderAction(context.Background(), 51, 23, MarketplaceOrderActionInput{Action: "confirm"})

	require.ErrorIs(t, err, ErrMarketplaceOrderRoleDenied)
	require.True(t, infraerrors.IsForbidden(err))
}

func TestMarketplaceServiceApplyOrderAction_rejectsInvalidState(t *testing.T) {
	svc := NewMarketplaceService(&marketplaceRepoStub{order: &MarketplaceOrder{
		ID: 51, BuyerUserID: 17, SellerUserID: 23, Status: MarketplaceOrderStatusDelivered,
	}})

	_, err := svc.ApplyOrderAction(context.Background(), 51, 17, MarketplaceOrderActionInput{Action: "confirm"})

	require.ErrorIs(t, err, ErrMarketplaceOrderInvalidState)
	require.True(t, infraerrors.IsConflict(err))
}

func TestMarketplaceServiceApplyOrderAction_reviewRequiresCompletedOrder(t *testing.T) {
	svc := NewMarketplaceService(&marketplaceRepoStub{order: &MarketplaceOrder{
		ID: 51, BuyerUserID: 17, SellerUserID: 23, Status: MarketplaceOrderStatusDelivered,
	}})

	_, err := svc.ApplyOrderAction(context.Background(), 51, 17, MarketplaceOrderActionInput{Action: "review", Rating: 5})

	require.ErrorIs(t, err, ErrMarketplaceOrderInvalidState)
}
