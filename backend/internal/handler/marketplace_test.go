package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type marketplaceHandlerStub struct {
	senderID    int64
	orderActor  int64
	orderAction service.MarketplaceOrderActionInput
	input       service.MarketplaceMessageInput
}

func (s *marketplaceHandlerStub) ListDisputes(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceOrderPage, error) {
	return service.MarketplaceOrderPage{}, nil
}
func (s *marketplaceHandlerStub) GetDispute(context.Context, int64, int64) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}
func (s *marketplaceHandlerStub) ResolveDispute(_ context.Context, id, actor int64, in service.MarketplaceDisputeResolutionInput) (*service.MarketplaceOrder, error) {
	s.orderActor = actor
	return &service.MarketplaceOrder{ID: id, Status: in.Outcome}, nil
}

func TestMarketplaceDisputeHandlersRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &MarketplaceHandler{app: &marketplaceHandlerStub{}}
	for _, method := range []gin.HandlerFunc{h.ListDisputes, h.GetDispute, h.ResolveDispute} {
		for _, role := range []string{"", "user", "admin"} {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"outcome":"confirmed","reason":"agreed","admin_id":999}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "id", Value: "1"}}
			if role != "" {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 31})
				c.Set(string(middleware.ContextKeyUserRole), role)
			}
			method(c)
			switch role {
			case "":
				require.Equal(t, 401, recorder.Code)
			case "user":
				require.Equal(t, 403, recorder.Code)
			case "admin":
				require.Equal(t, 200, recorder.Code)
			}
		}
	}
	require.Equal(t, int64(31), h.app.(*marketplaceHandlerStub).orderActor)
}

func (s *marketplaceHandlerStub) ListListings(context.Context, service.MarketplaceListingQuery) (service.MarketplaceListingPage, error) {
	return service.MarketplaceListingPage{}, nil
}
func (s *marketplaceHandlerStub) GetListing(context.Context, int64, int64) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (s *marketplaceHandlerStub) CreateListing(context.Context, int64, service.MarketplaceListingInput) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (s *marketplaceHandlerStub) UpdateListing(context.Context, int64, int64, service.MarketplaceListingInput) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (s *marketplaceHandlerStub) ArchiveListing(context.Context, int64, int64) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (s *marketplaceHandlerStub) ListOwnerListings(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceListingPage, error) {
	return service.MarketplaceListingPage{}, nil
}
func (s *marketplaceHandlerStub) CreateInquiry(context.Context, int64, int64) (*service.MarketplaceInquiry, error) {
	return &service.MarketplaceInquiry{}, nil
}
func (s *marketplaceHandlerStub) ListInquiries(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceInquiryPage, error) {
	return service.MarketplaceInquiryPage{}, nil
}
func (s *marketplaceHandlerStub) ListMessages(context.Context, int64, int64, service.MarketplacePageQuery) (service.MarketplaceMessagePage, error) {
	return service.MarketplaceMessagePage{}, nil
}
func (s *marketplaceHandlerStub) PostMessage(_ context.Context, inquiryID, senderID int64, input service.MarketplaceMessageInput) (*service.MarketplaceMessage, error) {
	s.senderID = senderID
	s.input = input
	return &service.MarketplaceMessage{InquiryID: inquiryID, SenderUserID: senderID}, nil
}
func (s *marketplaceHandlerStub) QuoteOrder(_ context.Context, inquiryID, sellerID int64, input service.MarketplaceOrderQuoteInput) (*service.MarketplaceOrder, error) {
	s.orderActor = sellerID
	return &service.MarketplaceOrder{InquiryID: inquiryID, SellerUserID: sellerID, ScopeText: input.ScopeText}, nil
}
func (s *marketplaceHandlerStub) ListOrders(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceOrderPage, error) {
	return service.MarketplaceOrderPage{}, nil
}
func (s *marketplaceHandlerStub) GetOrder(context.Context, int64, int64) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}
func (s *marketplaceHandlerStub) ApplyOrderAction(_ context.Context, orderID, actorID int64, input service.MarketplaceOrderActionInput) (*service.MarketplaceOrder, error) {
	s.orderActor = actorID
	s.orderAction = input
	return &service.MarketplaceOrder{ID: orderID, Status: service.MarketplaceOrderStatusConfirmed}, nil
}
func (s *marketplaceHandlerStub) AdminSetListingStatus(context.Context, int64, string) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}

func TestMarketplaceHandlerPostMessage_derivesSenderFromAuthContext(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	stub := &marketplaceHandlerStub{}
	h := &MarketplaceHandler{app: stub}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/biz/market/inquiries/44/messages", strings.NewReader(`{"sender_user_id":999,"client_message_id":"client-1","body":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "44"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 31})

	// When
	h.PostMessage(c)

	// Then
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, int64(31), stub.senderID)
	require.Equal(t, "client-1", stub.input.ClientMessageID)
}

func TestMarketplaceHandlerQuoteOrder_derivesSellerFromAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &marketplaceHandlerStub{}
	h := &MarketplaceHandler{app: stub}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/biz/market/inquiries/44/orders", strings.NewReader(`{"seller_user_id":999,"scope_text":"交付范围"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "44"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 31})

	h.QuoteOrder(c)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, int64(31), stub.orderActor)
	require.Contains(t, recorder.Body.String(), `"scope_text":"交付范围"`)
}

func TestMarketplaceHandlerApplyOrderAction_derivesActorFromAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &marketplaceHandlerStub{}
	h := &MarketplaceHandler{app: stub}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/biz/market/orders/51/actions", strings.NewReader(`{"actor_user_id":999,"action":"confirm"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "51"}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 31})

	h.ApplyOrderAction(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(31), stub.orderActor)
	require.Equal(t, "confirm", stub.orderAction.Action)
}
