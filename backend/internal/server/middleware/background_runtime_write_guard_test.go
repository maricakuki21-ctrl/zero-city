package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBackgroundRuntimeWriteGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "backup schedule", method: http.MethodPut, path: "/api/v1/admin/backups/schedule", want: http.StatusServiceUnavailable},
		{name: "ops advanced settings", method: http.MethodPut, path: "/api/v1/admin/ops/advanced-settings", want: http.StatusServiceUnavailable},
		{name: "monitor create", method: http.MethodPost, path: "/api/v1/admin/channel-monitors", want: http.StatusServiceUnavailable},
		{name: "monitor update", method: http.MethodPut, path: "/api/v1/admin/channel-monitors/42", want: http.StatusServiceUnavailable},
		{name: "monitor delete", method: http.MethodDelete, path: "/api/v1/admin/channel-monitors/42", want: http.StatusServiceUnavailable},
		{name: "manual monitor run remains available", method: http.MethodPost, path: "/api/v1/admin/channel-monitors/42/run", want: http.StatusNoContent},
		{name: "ordinary gateway write remains available", method: http.MethodPost, path: "/v1/chat/completions", want: http.StatusNoContent},
		{name: "backup read remains available", method: http.MethodGet, path: "/api/v1/admin/backups/schedule", want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			standby := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleStandby})
			r.Use(BackgroundRuntimeWriteGuard(standby))
			handlerCalled := false
			r.Handle(tt.method, routePattern(tt.path), func(c *gin.Context) {
				handlerCalled = true
				c.Status(http.StatusNoContent)
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			require.Equal(t, tt.want, w.Code)
			if tt.want == http.StatusServiceUnavailable {
				require.Equal(t, backgroundRuntimeRetryAfterSeconds, w.Header().Get("Retry-After"))
				require.Contains(t, w.Body.String(), "BACKGROUND_RUNTIME_STANDBY")
				require.False(t, handlerCalled, "standby rejection must happen before a scheduling write handler can persist")
			} else {
				require.True(t, handlerCalled)
			}
		})
	}
}

func TestBackgroundRuntimeWriteGuardAllowsActive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	active := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleActive})
	r.Use(BackgroundRuntimeWriteGuard(active))
	r.PUT("/api/v1/admin/backups/schedule", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/v1/admin/backups/schedule", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
}

func routePattern(path string) string {
	if path == "/api/v1/admin/channel-monitors/42" {
		return "/api/v1/admin/channel-monitors/:id"
	}
	return path
}
