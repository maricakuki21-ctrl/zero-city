package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesEndpointStamp(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetActualOpenAIUpstreamEndpoint(c, "/v1/messages")
	stampOpenAIResponsesUpstreamEndpoint(c, nil)
	require.Equal(t, "/v1/responses", GetActualOpenAIUpstreamEndpoint(c))
	result := &OpenAIForwardResult{}
	stampOpenAIResponsesUpstreamEndpoint(c, result)
	require.Equal(t, "/v1/responses", result.UpstreamEndpoint)
	existing := &OpenAIForwardResult{UpstreamEndpoint: "/actual/adapter"}
	stampOpenAIResponsesUpstreamEndpoint(nil, existing)
	require.Equal(t, "/actual/adapter", existing.UpstreamEndpoint)
	stampOpenAIResponsesUpstreamEndpoint(nil, nil)
	// A fallback attempt must overwrite the previous attempt's endpoint.
	SetActualOpenAIUpstreamEndpoint(c, "/v1/chat/completions")
	require.Equal(t, "/v1/chat/completions", GetActualOpenAIUpstreamEndpoint(c))
}
