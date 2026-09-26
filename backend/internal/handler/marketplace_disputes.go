package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func marketplaceAdmin(c *gin.Context) (int64, bool) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return 0, false
	}
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || role != "admin" {
		response.ErrorFrom(c, service.ErrMarketplaceOrderRoleDenied)
		return 0, false
	}
	return subject.UserID, true
}

func (h *MarketplaceHandler) ListDisputes(c *gin.Context) {
	adminID, ok := marketplaceAdmin(c)
	if !ok {
		return
	}
	page, err := h.app.ListDisputes(c.Request.Context(), adminID, marketplacePageQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *MarketplaceHandler) GetDispute(c *gin.Context) {
	adminID, ok := marketplaceAdmin(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	order, err := h.app.GetDispute(c.Request.Context(), id, adminID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}

func (h *MarketplaceHandler) ResolveDispute(c *gin.Context) {
	adminID, ok := marketplaceAdmin(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var in service.MarketplaceDisputeResolutionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid dispute resolution")
		return
	}
	order, err := h.app.ResolveDispute(c.Request.Context(), id, adminID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}
