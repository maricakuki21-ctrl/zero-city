package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) GetTavernCommercePolicy(c *gin.Context) {
	if _, ok := marketplaceSubject(c); !ok {
		return
	}
	r, err := h.bizService.TavernCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.GetTavernCommercePolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) SetTavernCommercePolicy(c *gin.Context) {
	actor, ok := marketplaceAdmin(c)
	if !ok {
		return
	}
	var in struct {
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid policy")
		return
	}
	r, err := h.bizService.TavernCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.SetTavernCommercePolicy(c.Request.Context(), actor, in.Enabled, in.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) SetTavernScriptPricing(c *gin.Context) {
	actor, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var in service.TavernPricingInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid price")
		return
	}
	r, err := h.bizService.TavernCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.SetTavernScriptPricing(c.Request.Context(), id, actor.UserID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) GetTavernTicketQuote(c *gin.Context) {
	actor, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	r, err := h.bizService.TavernCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.GetTavernTicketQuote(c.Request.Context(), id, actor.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) PurchaseTavernTicket(c *gin.Context) {
	actor, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var in service.TavernTicketInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid ticket")
		return
	}
	p, err := h.bizService.PurchaseTavernTicket(c.Request.Context(), id, actor.UserID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) RefundTavernTicket(c *gin.Context) {
	actor, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var in struct {
		TicketID int64 `json:"ticket_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "ticket_id is required")
		return
	}
	p, err := h.bizService.RefundTavernTicket(c.Request.Context(), id, actor.UserID, in.TicketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
