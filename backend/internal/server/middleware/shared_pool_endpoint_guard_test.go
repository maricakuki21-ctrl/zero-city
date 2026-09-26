package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolGatewayEndpointAllowed(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		allowed bool
	}{
		{name: "versioned chat", method: http.MethodPost, path: "/v1/chat/completions", allowed: true},
		{name: "direct chat", method: http.MethodPost, path: "/chat/completions", allowed: true},
		{name: "versioned responses", method: http.MethodPost, path: "/v1/responses", allowed: true},
		{name: "direct responses", method: http.MethodPost, path: "/responses", allowed: true},
		{name: "codex responses", method: http.MethodPost, path: "/backend-api/codex/responses", allowed: true},
		{name: "versioned responses compact", method: http.MethodPost, path: "/v1/responses/compact", allowed: true},
		{name: "direct responses compact", method: http.MethodPost, path: "/responses/compact", allowed: true},
		{name: "codex responses compact", method: http.MethodPost, path: "/backend-api/codex/responses/compact", allowed: true},
		{name: "model list", method: http.MethodGet, path: "/v1/models", allowed: true},
		{name: "billing info", method: http.MethodGet, path: "/v1/sub2api/billing", allowed: true},
		{name: "messages blocked", method: http.MethodPost, path: "/v1/messages", allowed: false},
		{name: "embeddings blocked", method: http.MethodPost, path: "/v1/embeddings", allowed: false},
		{name: "image generation allowed", method: http.MethodPost, path: "/v1/images/generations", allowed: true},
		{name: "image edit allowed", method: http.MethodPost, path: "/v1/images/edits", allowed: true},
		{name: "image variation blocked", method: http.MethodPost, path: "/v1/images/variations", allowed: false},
		{name: "async image read blocked", method: http.MethodGet, path: "/v1/images/tasks/123", allowed: false},
		{name: "versioned video generation allowed", method: http.MethodPost, path: "/v1/videos/generations", allowed: true},
		{name: "direct video generation allowed", method: http.MethodPost, path: "/videos/generations", allowed: true},
		{name: "versioned video status allowed", method: http.MethodGet, path: "/v1/videos/task-123", allowed: true},
		{name: "direct video status allowed", method: http.MethodGet, path: "/videos/task-123", allowed: true},
		{name: "empty video status blocked", method: http.MethodGet, path: "/v1/videos/", allowed: false},
		{name: "nested video status blocked", method: http.MethodGet, path: "/v1/videos/task-123/output", allowed: false},
		{name: "backslash video status blocked", method: http.MethodGet, path: `/v1/videos/task-123\output`, allowed: false},
		{name: "dot video status blocked", method: http.MethodGet, path: "/v1/videos/..", allowed: false},
		{name: "video edit blocked", method: http.MethodPost, path: "/v1/videos/edits", allowed: false},
		{name: "video extension blocked", method: http.MethodPost, path: "/v1/videos/extensions", allowed: false},
		{name: "unknown video submit blocked", method: http.MethodPost, path: "/v1/videos/unknown", allowed: false},
		{name: "responses compact get blocked", method: http.MethodGet, path: "/v1/responses/compact", allowed: false},
		{name: "responses compact trailing slash blocked", method: http.MethodPost, path: "/v1/responses/compact/", allowed: false},
		{name: "responses compact subpath blocked", method: http.MethodPost, path: "/v1/responses/compact/detail", allowed: false},
		{name: "responses compact backslash blocked", method: http.MethodPost, path: `/v1/responses/compact\detail`, allowed: false},
		{name: "direct responses compact subpath blocked", method: http.MethodPost, path: "/responses/compact/detail", allowed: false},
		{name: "codex responses compact subpath blocked", method: http.MethodPost, path: "/backend-api/codex/responses/compact/detail", allowed: false},
		{name: "responses websocket blocked", method: http.MethodGet, path: "/v1/responses", allowed: false},
		{name: "gemini blocked", method: http.MethodPost, path: "/v1beta/models/gemini:generateContent", allowed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.allowed, sharedPoolGatewayEndpointAllowed(tt.method, tt.path))
		})
	}
}

func TestRejectUnsupportedSharedPoolEndpointAbortsBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/tasks/123", nil)

	rejected := rejectUnsupportedSharedPoolEndpoint(c, &service.APIKey{SharedPoolManaged: true})

	require.True(t, rejected)
	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "SHARED_POOL_ENDPOINT_UNAVAILABLE")
}

func TestRejectUnsupportedSharedPoolEndpointLeavesOrdinaryKeyUntouched(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	rejected := rejectUnsupportedSharedPoolEndpoint(c, &service.APIKey{Key: "sk-official"})

	require.False(t, rejected)
	require.False(t, c.IsAborted())
}

func TestRejectUnsupportedSharedPoolEndpointAllowsCompactQuery(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses/compact?trace=1", nil)

	rejected := rejectUnsupportedSharedPoolEndpoint(c, &service.APIKey{SharedPoolManaged: true})

	require.False(t, rejected)
	require.False(t, c.IsAborted())
}

func TestRejectUnsupportedSharedPoolVideoPathStillFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/extensions", nil)

	rejected := rejectUnsupportedSharedPoolEndpoint(c, &service.APIKey{SharedPoolManaged: true})

	require.True(t, rejected)
	require.True(t, c.IsAborted())
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "SHARED_POOL_ENDPOINT_UNAVAILABLE")
}
