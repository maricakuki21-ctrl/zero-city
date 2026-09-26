package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolVideoHandlersRejectMissingOrOrdinaryAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		method    string
		path      string
		apiKey    *service.APIKey
		invoke    func(*OpenAIGatewayHandler, *gin.Context)
		requestID string
	}{
		{
			name:   "generation missing key",
			method: http.MethodPost,
			path:   "/v1/videos/generations",
			invoke: func(h *OpenAIGatewayHandler, c *gin.Context) {
				h.SharedPoolGrokVideoGeneration(c)
			},
		},
		{
			name:   "generation ordinary key",
			method: http.MethodPost,
			path:   "/v1/videos/generations",
			apiKey: &service.APIKey{ID: 17, Key: "sk-ordinary"},
			invoke: func(h *OpenAIGatewayHandler, c *gin.Context) {
				h.SharedPoolGrokVideoGeneration(c)
			},
		},
		{
			name:      "status ordinary key",
			method:    http.MethodGet,
			path:      "/v1/videos/upstream-task-91",
			apiKey:    &service.APIKey{ID: 18, Key: "sk-ordinary"},
			requestID: "upstream-task-91",
			invoke: func(h *OpenAIGatewayHandler, c *gin.Context) {
				h.SharedPoolGrokVideoStatus(c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"model":"grok-imagine-video"}`))
			if tt.apiKey != nil {
				c.Set(string(middleware2.ContextKeyAPIKey), tt.apiKey)
			}
			if tt.requestID != "" {
				c.Params = gin.Params{{Key: "request_id", Value: tt.requestID}}
			}

			tt.invoke(&OpenAIGatewayHandler{}, c)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.Contains(t, recorder.Body.String(), "Invalid shared-pool key")
		})
	}
}
