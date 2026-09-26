package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type marketplaceRouteRepository struct{}

func (*marketplaceRouteRepository) ListMarketplaceDisputes(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceOrderPage, error) {
	return service.MarketplaceOrderPage{}, nil
}
func (*marketplaceRouteRepository) GetMarketplaceDispute(context.Context, int64, int64) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}
func (*marketplaceRouteRepository) ResolveMarketplaceDispute(context.Context, int64, int64, service.MarketplaceDisputeResolutionInput) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}

func (*marketplaceRouteRepository) ListListings(context.Context, service.MarketplaceListingQuery) (service.MarketplaceListingPage, error) {
	return service.MarketplaceListingPage{Items: []service.MarketplaceListing{{ID: 71, Kind: "service", Status: "published"}}}, nil
}
func (*marketplaceRouteRepository) GetListing(context.Context, int64, int64) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (*marketplaceRouteRepository) CreateListing(context.Context, int64, service.MarketplaceListingInput) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (*marketplaceRouteRepository) UpdateListing(context.Context, int64, int64, service.MarketplaceListingInput) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (*marketplaceRouteRepository) ArchiveListing(context.Context, int64, int64) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}
func (*marketplaceRouteRepository) ListOwnerListings(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceListingPage, error) {
	return service.MarketplaceListingPage{}, nil
}
func (*marketplaceRouteRepository) CreateInquiry(context.Context, int64, int64) (*service.MarketplaceInquiry, error) {
	return &service.MarketplaceInquiry{}, nil
}
func (*marketplaceRouteRepository) ListInquiries(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceInquiryPage, error) {
	return service.MarketplaceInquiryPage{}, nil
}
func (*marketplaceRouteRepository) ListMessages(context.Context, int64, int64, service.MarketplacePageQuery) (service.MarketplaceMessagePage, error) {
	return service.MarketplaceMessagePage{}, nil
}
func (*marketplaceRouteRepository) PostMessage(context.Context, int64, int64, service.MarketplaceMessageInput) (*service.MarketplaceMessage, error) {
	return &service.MarketplaceMessage{}, nil
}
func (*marketplaceRouteRepository) CreateMarketplaceOrder(context.Context, int64, int64, service.MarketplaceOrderQuoteInput) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}
func (*marketplaceRouteRepository) ListMarketplaceOrders(context.Context, int64, service.MarketplacePageQuery) (service.MarketplaceOrderPage, error) {
	return service.MarketplaceOrderPage{}, nil
}
func (*marketplaceRouteRepository) GetMarketplaceOrder(context.Context, int64, int64) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}
func (*marketplaceRouteRepository) TransitionMarketplaceOrder(context.Context, service.MarketplaceOrderTransition) (*service.MarketplaceOrder, error) {
	return &service.MarketplaceOrder{}, nil
}
func (*marketplaceRouteRepository) CreateMarketplaceOrderReview(context.Context, int64, int64, int, string) (*service.MarketplaceOrderReview, error) {
	return &service.MarketplaceOrderReview{}, nil
}
func (*marketplaceRouteRepository) AdminSetListingStatus(context.Context, int64, string) (*service.MarketplaceListing, error) {
	return &service.MarketplaceListing{}, nil
}

func TestMarketplaceRoutesExposePublishedListingFeed(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })
	marketplace := handler.NewMarketplaceHandler(service.NewMarketplaceService(&marketplaceRouteRepository{}))
	RegisterBizDecipherRoutes(v1, &handler.Handlers{
		BizDecipher: handler.NewBizDecipherHandler(nil), Marketplace: marketplace,
	}, jwt, nil, nil)

	// When
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/biz/market/listings", nil))

	// Then
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":71`)
	require.Contains(t, recorder.Body.String(), `"status":"published"`)
}

func TestMarketplaceDisputeRoutesRejectRegularUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	auth := func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4})
		c.Set(string(middleware.ContextKeyUserRole), "user")
		c.Next()
	}
	marketplace := handler.NewMarketplaceHandler(service.NewMarketplaceService(&marketplaceRouteRepository{}))
	RegisterBizDecipherRoutes(v1, &handler.Handlers{BizDecipher: handler.NewBizDecipherHandler(nil), Marketplace: marketplace},
		middleware.JWTAuthMiddleware(auth), middleware.AdminAuthMiddleware(auth), nil)
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/biz/market/disputes"},
		{"GET", "/api/v1/admin/biz/market/disputes/1"},
		{"POST", "/api/v1/admin/biz/market/disputes/1/resolve"},
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, nil))
		require.Equal(t, http.StatusForbidden, recorder.Code, route.path)
	}
}
