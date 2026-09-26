package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHarnessProxyUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &BizDecipherHandler{}
	router := gin.New()
	router.GET("/api/v1/biz/harness/catalog", handler.ProxyHarness)
	response := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/biz/harness/catalog", nil).WithContext(ctx))
	// A running loopback runner rejects anonymous requests; without a runner
	// the proxy must give an explicit unavailable response, never HTML success.
	if response.Code != http.StatusUnauthorized && response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable or unauthorized, got %d", response.Code)
	}
}
