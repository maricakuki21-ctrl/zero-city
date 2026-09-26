package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCanonicalSharedKeyRouteCoversEnabledTextAndPaidMedia(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/responses"},
		{http.MethodPost, "/v1/images/generations"},
		{http.MethodPost, "/v1/images/edits"},
		{http.MethodPost, "/v1/videos/generations"},
		{http.MethodGet, "/v1/videos/request-1"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			require.True(t, isCanonicalSharedKeyRoute(req))
		})
	}
}

func TestCanonicalSharedKeyRouteDoesNotBroadenAuthenticationContract(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/messages"},
		{http.MethodPost, "/v1/embeddings"},
		{http.MethodPost, "/v1beta/models/gemini:generateContent"},
		{http.MethodGet, "/v1/responses"},
		{http.MethodGet, "/v1/models"},
		{http.MethodGet, "/v1/images/tasks/task-1"},
		{http.MethodGet, "/v1/videos/request-1/content"},
		{http.MethodPost, "/v1/images/batches"},
		{http.MethodPost, "/other/chat/completions"},
		{http.MethodPost, "/v1/responses/unsupported"},
		{http.MethodPost, "/v1/responses/"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			require.False(t, isCanonicalSharedKeyRoute(httptest.NewRequest(route.method, route.path, nil)))
		})
	}
	require.False(t, isCanonicalSharedKeyRoute(nil))
	require.False(t, isCanonicalSharedKeyRoute(&http.Request{}))
}

func TestCanonicalCommitResponseWriterMarksFirstBodyByte(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	commits := 0
	boundary, err := NewCommitBoundary(func() error {
		commits++
		return nil
	})
	require.NoError(t, err)
	ctx.Writer = &canonicalCommitResponseWriter{ResponseWriter: ctx.Writer, boundary: boundary}

	// When
	_, err = ctx.Writer.Write([]byte("x"))
	require.NoError(t, err)
	_, err = ctx.Writer.Write([]byte("y"))
	require.NoError(t, err)

	// Then
	require.True(t, boundary.Committed())
	require.Equal(t, 1, commits)
}

func TestCanonicalCommitResponseWriterFlushClosesRetryBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	commits := 0
	boundary, err := NewCommitBoundary(func() error { commits++; return nil })
	require.NoError(t, err)
	ctx.Writer = &canonicalCommitResponseWriter{ResponseWriter: ctx.Writer, boundary: boundary}

	ctx.Writer.Flush()
	require.True(t, recorder.Flushed)
	require.True(t, boundary.Committed(), "headers are already committed even without a body")
	ctx.Writer.Flush()
	_, err = ctx.Writer.WriteString("data: done\n\n")
	require.NoError(t, err)
	require.Equal(t, 1, commits)
}
