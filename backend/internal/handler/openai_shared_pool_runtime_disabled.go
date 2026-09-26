package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) acquireSharedPoolSlots(
	c *gin.Context,
	_ *zap.Logger,
	_ *service.SharedPoolAccessKey,
	_ string,
) (func(), bool) {
	h.errorResponse(c, http.StatusServiceUnavailable, "canonical_gateway_required", "Shared-market requests require the canonical Sub2 gateway")
	c.Abort()
	return nil, false
}
