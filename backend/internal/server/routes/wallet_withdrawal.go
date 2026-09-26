package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerWalletWithdrawalRoutes(user, admin *gin.RouterGroup, h *handler.Handlers) {
	user.GET("/withdrawals/policy", h.BizDecipher.GetWithdrawalPolicy)
	user.GET("/withdrawals", h.BizDecipher.ListWithdrawals)
	user.POST("/withdrawals", h.BizDecipher.CreateWithdrawal)
	user.POST("/withdrawals/:id/cancel", h.BizDecipher.CancelWithdrawal)
	admin.GET("/withdrawals/policy", h.BizDecipher.GetWithdrawalPolicy)
	admin.PUT("/withdrawals/policy", h.BizDecipher.AdminSaveWithdrawalPolicy)
	admin.GET("/withdrawals", h.BizDecipher.AdminListWithdrawals)
	admin.POST("/withdrawals/:id/actions", h.BizDecipher.AdminActWithdrawal)
}
