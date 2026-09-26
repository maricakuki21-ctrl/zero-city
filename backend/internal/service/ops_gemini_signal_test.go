package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsGeminiSignalScope_FirstMarkerWins(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	MarkOpsStreamErrorValue(c, OpsStreamError{
		ErrType: " invalid_request_error ", Code: " SAFETY ", Message: " policy ",
		IntendedStatus: 400, RequestScoped: true, NonStream: true, CountTowardsSLA: true,
	})
	MarkOpsStreamFailure(c, "upstream_error", "SECOND", "second", 502)
	marker, ok := GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, "SAFETY", marker.Code)
	require.Equal(t, "invalid_request_error", marker.ErrType)
	require.Equal(t, "policy", marker.Message)
	require.True(t, marker.RequestScoped)
	require.True(t, marker.NonStream)
	require.False(t, marker.CountTowardsSLA)
}
