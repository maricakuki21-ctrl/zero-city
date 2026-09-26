package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type marketplaceFundsApp interface {
	GetOrderFunds(context.Context, int64, int64) (*service.MarketplaceFunds, error)
	SetOrderFunds(context.Context, int64, int64, service.MarketplaceFundsInput) (*service.MarketplaceFunds, error)
	PayOrder(context.Context, int64, int64, service.MarketplacePaymentInput) (*service.MarketplaceFunds, error)
	GetFundsPolicy(context.Context) (*service.MarketplaceFundsPolicy, error)
	SetFundsPolicy(context.Context, int64, service.MarketplaceFundsPolicyInput) (*service.MarketplaceFundsPolicy, error)
}

func (h *MarketplaceHandler) fundsApp(c *gin.Context) (marketplaceFundsApp, bool) {
	app, ok := h.app.(marketplaceFundsApp)
	if !ok {
		response.ErrorFrom(c, service.ErrMarketplaceFundsClosed)
	}
	return app, ok
}
func (h *MarketplaceHandler) GetOrderFunds(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	app, ok := h.fundsApp(c)
	if !ok {
		return
	}
	f, err := app.GetOrderFunds(c.Request.Context(), id, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, f)
}
func (h *MarketplaceHandler) SetOrderFunds(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	app, ok := h.fundsApp(c)
	if !ok {
		return
	}
	var in service.MarketplaceFundsInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid amount")
		return
	}
	f, err := app.SetOrderFunds(c.Request.Context(), id, subject.UserID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, f)
}
func (h *MarketplaceHandler) PayOrder(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	app, ok := h.fundsApp(c)
	if !ok {
		return
	}
	var in service.MarketplacePaymentInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid payment")
		return
	}
	f, err := app.PayOrder(c.Request.Context(), id, subject.UserID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, f)
}
func (h *MarketplaceHandler) GetFundsPolicy(c *gin.Context) {
	if _, ok := marketplaceSubject(c); !ok {
		return
	}
	app, ok := h.fundsApp(c)
	if !ok {
		return
	}
	p, err := app.GetFundsPolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *MarketplaceHandler) SetFundsPolicy(c *gin.Context) {
	actor, ok := marketplaceAdmin(c)
	if !ok {
		return
	}
	app, ok := h.fundsApp(c)
	if !ok {
		return
	}
	var in service.MarketplaceFundsPolicyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid policy")
		return
	}
	p, err := app.SetFundsPolicy(c.Request.Context(), actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
