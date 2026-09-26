package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAsyncMediaTaskRead(t *testing.T) {
	require.True(t, isAsyncMediaTaskRead(http.MethodGet, "/v1/images/tasks/imgtask_123"))
	require.True(t, isAsyncMediaTaskRead(http.MethodGet, "/images/tasks/imgtask_123"))
	require.True(t, isAsyncMediaTaskRead(http.MethodGet, "/v1/videos/video_123"))
	require.True(t, isAsyncMediaTaskRead(http.MethodGet, "/videos/video_123"))
	require.False(t, isAsyncMediaTaskRead(http.MethodPost, "/v1/images/tasks/imgtask_123"))
	require.False(t, isAsyncMediaTaskRead(http.MethodPost, "/v1/videos/video_123"))
	require.False(t, isAsyncMediaTaskRead(http.MethodGet, "/v1/videos/video_123/output"))
	require.False(t, isAsyncMediaTaskRead(http.MethodGet, "/v1/images/generations"))
}
