package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerMarketplaceFundsRoutes(user, admin *gin.RouterGroup, h *handler.Handlers) {
	user.GET("/market/funds/policy", h.Marketplace.GetFundsPolicy)
	user.GET("/market/orders/:id/funds", h.Marketplace.GetOrderFunds)
	user.PUT("/market/orders/:id/funds", h.Marketplace.SetOrderFunds)
	user.POST("/market/orders/:id/pay", h.Marketplace.PayOrder)
	admin.PUT("/market/funds/policy", h.Marketplace.SetFundsPolicy)
}
