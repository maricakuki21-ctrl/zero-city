package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchRoutesExposeAuthenticatedForkAndEvents(t *testing.T) {
	// Given
	source, err := os.ReadFile("bizdecipher.go")
	require.NoError(t, err)
	text := string(source)
	require.Contains(t, text, `authenticated.GET("/workbench/runs/:id/events"`)
	require.Contains(t, text, `authenticated.POST("/workbench/runs/:id/fork"`)
	require.NotContains(t, text, `c.Query("token")`)
	require.NotContains(t, text, `c.Query("jwt")`)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	stub := &workbenchRouteStub{fork: workbench.ForkResult{Run: workbench.Run{ID: "wbr_forked", OwnerID: 41}}}
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 41})
		c.Next()
	})
	RegisterBizDecipherRoutes(v1, &handler.Handlers{BizDecipher: handler.NewBizDecipherHandler(nil, stub)}, jwt, nil, nil)

	// When
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/biz/workbench/runs/wbr_source/fork", strings.NewReader("{}"))
	req.Header.Set("Idempotency-Key", "fork-route")
	engine.ServeHTTP(rec, req)

	// Then
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "wbr_forked")
}

type workbenchRouteStub struct {
	workbench.UnavailableAdapter
	fork workbench.ForkResult
}

func (s *workbenchRouteStub) Fork(context.Context, workbench.ForkCommand) (workbench.ForkResult, error) {
	return s.fork, nil
}
