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

type pollRouteRepository struct{ service.BizDecipherRepository }

func (*pollRouteRepository) ListCommunityPolls(context.Context, int64, bool, int) ([]service.CommunityPoll, error) {
	return []service.CommunityPoll{{ID: 9, Title: "真实投票", Status: "open", Options: []service.CommunityPollOption{}}}, nil
}
func (*pollRouteRepository) CreateCommunityPollTx(context.Context, int64, service.CommunityPollInput) (*service.CommunityPoll, error) {
	return &service.CommunityPoll{}, nil
}
func (*pollRouteRepository) VoteCommunityPollTx(context.Context, int64, int64, int64) (*service.CommunityPoll, error) {
	return &service.CommunityPoll{}, nil
}
func (*pollRouteRepository) CloseCommunityPollTx(context.Context, int64, int64, bool) (*service.CommunityPoll, error) {
	return &service.CommunityPoll{}, nil
}

func TestGovernancePollRoutesExposePublicRealResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })
	biz := handler.NewBizDecipherHandler(service.NewBizDecipherService(&pollRouteRepository{}, nil, nil))
	marketplace := handler.NewMarketplaceHandler(service.NewMarketplaceService(&marketplaceRouteRepository{}))
	RegisterBizDecipherRoutes(v1, &handler.Handlers{BizDecipher: biz, Marketplace: marketplace}, jwt, nil, nil)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/biz/community/polls", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":9`)
	require.Contains(t, recorder.Body.String(), `"title":"真实投票"`)
}
