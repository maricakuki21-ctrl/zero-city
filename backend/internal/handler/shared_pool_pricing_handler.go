package handler

import (
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type saveSharedPoolCustomPriceRequest struct {
	ModelName        string   `json:"model_name" binding:"required"`
	EndpointType     string   `json:"endpoint_type" binding:"required"`
	OperationID      string   `json:"operation_id" binding:"required"`
	BillingMode      string   `json:"billing_mode" binding:"required"`
	InputPrice       *float64 `json:"input_price"`
	OutputPrice      *float64 `json:"output_price"`
	CacheReadPrice   *float64 `json:"cache_read_price"`
	CacheWritePrice  *float64 `json:"cache_write_price"`
	ImageItemPrice   *float64 `json:"image_item_price"`
	VideoSecondPrice *float64 `json:"video_second_price"`
	PerRequestPrice  *float64 `json:"per_request_price"`
	Multiplier       float64  `json:"multiplier" binding:"required"`
	MinimumCharge    *float64 `json:"minimum_charge"`
	MaximumCharge    *float64 `json:"maximum_charge"`
}

// ListSharedPoolModelPricing returns the endpoint-scoped immutable price and
// gate state. Clients must require enabled + passed + ready before presenting
// an endpoint as callable.
func (h *BizDecipherHandler) ListSharedPoolModelPricing(c *gin.Context) {
	poolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var viewerID int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		viewerID = subject.UserID
	}
	items, err := h.bizService.ListSharedPoolModelEndpointPricing(c.Request.Context(), poolID, viewerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) SaveSharedPoolCustomPrice(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	poolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req saveSharedPoolCustomPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	quote, err := h.bizService.SaveSharedPoolCustomPrice(c.Request.Context(), service.SaveSharedPoolCustomPriceInput{
		PoolID: poolID, OwnerID: subject.UserID, ModelName: req.ModelName,
		EndpointType: req.EndpointType, OperationID: req.OperationID, BillingMode: req.BillingMode,
		InputPrice: req.InputPrice, OutputPrice: req.OutputPrice,
		CacheReadPrice: req.CacheReadPrice, CacheWritePrice: req.CacheWritePrice,
		ImageItemPrice: req.ImageItemPrice, VideoSecondPrice: req.VideoSecondPrice,
		PerRequestPrice: req.PerRequestPrice, Multiplier: req.Multiplier,
		MinimumCharge: req.MinimumCharge, MaximumCharge: req.MaximumCharge,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPoolForbidden):
			response.Forbidden(c, "Only the pool owner can change pricing")
		case errors.Is(err, service.ErrSharedPoolOfficialPriceOnly):
			response.BadRequest(c, "Official catalog models always use the official base price; change only the model multiplier")
		case errors.Is(err, service.ErrSharedPoolPricingOperationID):
			response.BadRequest(c, "This operation id was already used for different pricing")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"price": quote})
}
