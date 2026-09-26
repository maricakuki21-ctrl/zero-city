package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestColumnCommercePublicPolicyAndProtectedWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	public := router.Group("/api/v1/biz")
	user := router.Group("/api/v1/biz")
	user.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) })
	admin := router.Group("/api/v1/admin/biz")
	admin.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) })
	registerCreatorColumnCommerceRoutes(public, user, admin, &handler.Handlers{
		BizDecipher: &handler.BizDecipherHandler{},
	})

	// No repository is installed: anonymous policy reads must reach the
	// handler's unavailable response, rather than a JWT rejection.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/biz/community/columns/commerce-policy", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "CREATOR_COLUMN_UNAVAILABLE")

	for _, test := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPatch, "/api/v1/biz/community/columns/1/pricing", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/biz/community/columns/1/purchases", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/biz/community/columns/1/purchases", http.StatusUnauthorized},
		{http.MethodPut, "/api/v1/admin/biz/community/columns/commerce-policy", http.StatusForbidden},
		{http.MethodPost, "/api/v1/admin/biz/community/columns/1/purchases/1/refund", http.StatusForbidden},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			result := httptest.NewRecorder()
			router.ServeHTTP(result, httptest.NewRequest(test.method, test.path, strings.NewReader("{}")))
			require.Equal(t, test.status, result.Code)
		})
	}
}
