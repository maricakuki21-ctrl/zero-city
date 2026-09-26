package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// handleSharedPoolChatCompletions is a fail-closed compatibility guard. Routes
// canonicalize managed keys before dispatch, so reaching this function means
// authentication context convergence did not occur.
func (h *OpenAIGatewayHandler) handleSharedPoolChatCompletions(c *gin.Context, _ *zap.Logger, _ *service.APIKey, _ string, _ []byte) {
	h.errorResponse(c, http.StatusServiceUnavailable, "canonical_binding_unavailable", "Shared-market key did not enter the canonical Sub2 gateway")
}

func (h *OpenAIGatewayHandler) handleSharedPoolResponses(c *gin.Context, _ *zap.Logger, _ *service.APIKey, _ string, _ []byte) {
	h.errorResponse(c, http.StatusServiceUnavailable, "canonical_binding_unavailable", "Shared-market key did not enter the canonical Sub2 gateway")
}

func isExactSharedPoolCompactPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	return GetInboundEndpoint(c) == EndpointResponsesCompact
}

func isSharedPoolCompactRequest(c *gin.Context, body []byte) bool {
	return isExactSharedPoolCompactPath(c) ||
		service.HasCompactionTriggerInInput(body)
}

func sharedPoolPublishedModelName(accessKey *service.SharedPoolAccessKey, requested string) string {
	if accessKey != nil && accessKey.PublishedModelName != "" {
		return accessKey.PublishedModelName
	}
	return requested
}

func sharedPoolAccessKeyReady(accessKey *service.SharedPoolAccessKey) bool {
	return accessKey != nil
}
