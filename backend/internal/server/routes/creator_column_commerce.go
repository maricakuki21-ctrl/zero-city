package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerCreatorColumnCommerceRoutes(public, user, admin *gin.RouterGroup, h *handler.Handlers) {
	public.GET("/community/columns/commerce-policy", h.BizDecipher.GetColumnCommercePolicy)
	user.PATCH("/community/columns/:id/pricing", h.BizDecipher.SetColumnPricing)
	user.POST("/community/columns/:id/purchases", h.BizDecipher.PurchaseColumn)
	user.GET("/community/columns/:id/purchases", h.BizDecipher.ListColumnPurchases)
	admin.PUT("/community/columns/commerce-policy", h.BizDecipher.AdminSetColumnCommercePolicy)
	admin.POST("/community/columns/:id/purchases/:purchaseId/refund", h.BizDecipher.AdminRefundColumnPurchase)
}
