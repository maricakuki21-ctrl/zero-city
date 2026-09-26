package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) GetColumnCommercePolicy(c *gin.Context) {
	r, err := h.bizService.ColumnCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.GetColumnCommercePolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func columnCommerceAdminActor(c *gin.Context) (int64, bool) {
	actor, ok := creatorActor(c)
	if !ok {
		return 0, false
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	if role != service.RoleAdmin {
		response.ErrorFrom(c, service.ErrColumnCommerceForbidden)
		return 0, false
	}
	return actor, true
}
func (h *BizDecipherHandler) AdminSetColumnCommercePolicy(c *gin.Context) {
	actor, ok := columnCommerceAdminActor(c)
	if !ok {
		return
	}
	var in struct {
		Enabled *bool  `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Enabled == nil {
		response.ErrorFrom(c, service.ErrColumnCommerceInvalid)
		return
	}
	r, err := h.bizService.ColumnCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.SetColumnCommercePolicy(c.Request.Context(), actor, *in.Enabled, in.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) SetColumnPricing(c *gin.Context) {
	actor, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	var in service.ColumnPricingInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrColumnCommerceInvalid)
		return
	}
	r, err := h.bizService.ColumnCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.SetColumnPricing(c.Request.Context(), id, actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) PurchaseColumn(c *gin.Context) {
	actor, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	var in service.ColumnPurchaseInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrColumnCommerceInvalid)
		return
	}
	p, err := h.bizService.PurchaseColumn(c.Request.Context(), id, actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) ListColumnPurchases(c *gin.Context) {
	if _, ok := creatorActor(c); !ok {
		return
	}
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	q, ok := creatorColumnQuery(c, false)
	if !ok {
		return
	}
	r, err := h.bizService.ColumnCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, err := r.ListColumnPurchases(c.Request.Context(), id, q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var next int64
	if len(items) == q.Limit {
		next = items[len(items)-1].ID
	}
	response.Success(c, gin.H{"items": items, "next_cursor": next})
}
func (h *BizDecipherHandler) AdminRefundColumnPurchase(c *gin.Context) {
	actor, ok := columnCommerceAdminActor(c)
	if !ok {
		return
	}
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	purchaseID, ok := creatorID(c, "purchaseId")
	if !ok {
		return
	}
	var in service.ColumnRefundInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrColumnCommerceInvalid)
		return
	}
	p, err := h.bizService.RefundColumnPurchase(c.Request.Context(), id, purchaseID, actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
