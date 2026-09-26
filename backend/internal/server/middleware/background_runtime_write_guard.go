package middleware

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const backgroundRuntimeRetryAfterSeconds = "5"

// BackgroundRuntimeWriteGuard prevents standby HTTP instances from accepting
// writes whose successful completion requires an active process-local scheduler.
// The guard runs before handlers, so rejected requests cannot partially persist.
func BackgroundRuntimeWriteGuard(runtime config.BackgroundRuntime) gin.HandlerFunc {
	return func(c *gin.Context) {
		if runtime.Enabled() || !requiresActiveBackgroundRuntime(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}

		c.Header("Retry-After", backgroundRuntimeRetryAfterSeconds)
		response.ErrorWithDetails(
			c,
			http.StatusServiceUnavailable,
			"This operation is temporarily available only on the active server. Please retry.",
			"BACKGROUND_RUNTIME_STANDBY",
			map[string]string{"runtime_role": config.BackgroundRuntimeRoleStandby},
		)
		c.Abort()
	}
}

func requiresActiveBackgroundRuntime(method, routePath string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	routePath = strings.TrimSpace(routePath)

	switch routePath {
	case "/api/v1/admin/backups/schedule", "/api/v1/admin/ops/advanced-settings":
		return method == http.MethodPut
	case "/api/v1/admin/channel-monitors":
		return method == http.MethodPost
	case "/api/v1/admin/channel-monitors/:id":
		return method == http.MethodPut || method == http.MethodDelete
	default:
		return false
	}
}
