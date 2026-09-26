package handler

import (
	"encoding/base64"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) GetAssetCommercePolicy(c *gin.Context) {
	r, err := h.bizService.AssetCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.GetAssetCommercePolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) AdminSetAssetCommercePolicy(c *gin.Context) {
	actor, ok := columnCommerceAdminActor(c)
	if !ok {
		return
	}
	var in struct {
		Enabled *bool  `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Enabled == nil {
		response.ErrorFrom(c, service.ErrAssetCommerceInvalid)
		return
	}
	r, err := h.bizService.AssetCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.SetAssetCommercePolicy(c.Request.Context(), actor, *in.Enabled, in.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) GetAssetCommerce(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	q, ok := creatorColumnQuery(c, false)
	if !ok {
		return
	}
	r, err := h.bizService.AssetCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.GetAssetCommerce(c.Request.Context(), id, q.ViewerID, q.Admin)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) SetAssetPricing(c *gin.Context) {
	actor, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	var in service.ColumnPricingInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrAssetCommerceInvalid)
		return
	}
	r, err := h.bizService.AssetCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := r.SetAssetPricing(c.Request.Context(), id, actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) PurchaseAsset(c *gin.Context) {
	actor, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	var in service.ColumnPurchaseInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrAssetCommerceInvalid)
		return
	}
	p, err := h.bizService.PurchaseAsset(c.Request.Context(), id, actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) ListAssetPurchases(c *gin.Context) {
	if _, ok := creatorActor(c); !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	q, ok := creatorColumnQuery(c, false)
	if !ok {
		return
	}
	r, err := h.bizService.AssetCommerceRepo()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, err := r.ListAssetPurchases(c.Request.Context(), id, q)
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
func (h *BizDecipherHandler) AdminRefundAssetPurchase(c *gin.Context) {
	actor, ok := columnCommerceAdminActor(c)
	if !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	purchaseID, ok := creatorID(c, "purchaseId")
	if !ok {
		return
	}
	var in service.ColumnRefundInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrAssetCommerceInvalid)
		return
	}
	p, err := h.bizService.RefundAssetPurchase(c.Request.Context(), id, purchaseID, actor, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) ReuseCapabilityAssetPackage(c *gin.Context) {
	actor, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	var in struct {
		OperationID string `json:"operation_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, service.ErrAssetCommerceInvalid)
		return
	}
	pkg, err := h.bizService.ReuseCapabilityAssetPackage(c.Request.Context(), id, actor, version, in.OperationID)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	files := make([]gin.H, 0, len(pkg.Files))
	for _, f := range pkg.Files {
		files = append(files, gin.H{"path": f.Path, "content_type": f.ContentType, "sha256": f.SHA256, "content_base64": base64.StdEncoding.EncodeToString(f.Content)})
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{"version": pkg.Version, "files": files})
}
