package handler

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) AdminGetSharedPoolUsageReviewPolicy(c *gin.Context) {
	policy, err := h.bizService.GetSharedPoolUsageReviewPolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

type updateSharedPoolUsageReviewPolicyRequest struct {
	AutoReleaseEnabled *bool    `json:"auto_release_enabled" binding:"required"`
	AutoReleaseMinutes *int     `json:"auto_release_minutes" binding:"required"`
	AutoReleaseMaxHold *float64 `json:"auto_release_max_hold" binding:"required"`
}

func (h *BizDecipherHandler) AdminUpdateSharedPoolUsageReviewPolicy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req updateSharedPoolUsageReviewPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.AutoReleaseEnabled == nil || req.AutoReleaseMinutes == nil || req.AutoReleaseMaxHold == nil {
		response.BadRequest(c, "Invalid shared pool usage review policy")
		return
	}
	policy, err := h.bizService.AdminUpdateSharedPoolUsageReviewPolicy(
		c.Request.Context(),
		service.UpdateSharedPoolUsageReviewPolicyInput{
			AutoReleaseEnabled: *req.AutoReleaseEnabled,
			AutoReleaseMinutes: *req.AutoReleaseMinutes,
			AutoReleaseMaxHold: *req.AutoReleaseMaxHold,
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	slog.Info("shared pool usage review auto-release policy updated",
		"admin_user_id", subject.UserID,
		"enabled", policy.AutoReleaseEnabled,
		"minutes", policy.AutoReleaseMinutes,
		"max_hold", policy.AutoReleaseMaxHold)
	response.Success(c, policy)
}

func (h *BizDecipherHandler) AdminListSharedPoolUsageReviews(c *gin.Context) {
	beforeID, ok := parseSharedPoolUsageTracePositiveQuery(c, "before_id")
	if !ok {
		response.BadRequest(c, "before_id must be a positive integer")
		return
	}
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			response.BadRequest(c, "limit must be a positive integer")
			return
		}
		limit = value
	}
	page, err := h.bizService.AdminListSharedPoolUsageReviews(
		c.Request.Context(), strings.TrimSpace(c.Query("state")), beforeID, limit,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

type resolveSharedPoolUsageReviewRequest struct {
	Action      string   `json:"action" binding:"required"`
	Amount      *float64 `json:"amount"`
	Note        string   `json:"note" binding:"required"`
	OperationID string   `json:"operation_id"`
}

func (h *BizDecipherHandler) AdminResolveSharedPoolUsageReview(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	reservationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || reservationID <= 0 {
		response.BadRequest(c, "Invalid shared pool usage review id")
		return
	}
	var req resolveSharedPoolUsageReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	if headerID := strings.TrimSpace(c.GetHeader("Idempotency-Key")); headerID != "" {
		if operationID != "" && operationID != headerID {
			response.BadRequest(c, "operation_id and Idempotency-Key must match")
			return
		}
		operationID = headerID
	}
	result, err := h.bizService.AdminResolveSharedPoolUsageReview(c.Request.Context(), service.ResolveSharedPoolUsageReviewInput{
		ReservationID: reservationID,
		AdminUserID:   subject.UserID,
		Action:        req.Action,
		Amount:        req.Amount,
		Note:          req.Note,
		OperationID:   operationID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type batchResolveSharedPoolUsageReviewsRequest struct {
	ReservationIDs []int64 `json:"reservation_ids" binding:"required"`
	Action         string  `json:"action" binding:"required"`
	Note           string  `json:"note" binding:"required"`
	OperationID    string  `json:"operation_id"`
}

func (h *BizDecipherHandler) AdminBatchResolveSharedPoolUsageReviews(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req batchResolveSharedPoolUsageReviewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	if headerID := strings.TrimSpace(c.GetHeader("Idempotency-Key")); headerID != "" {
		if operationID != "" && operationID != headerID {
			response.BadRequest(c, "operation_id and Idempotency-Key must match")
			return
		}
		operationID = headerID
	}
	result, err := h.bizService.AdminBatchResolveSharedPoolUsageReviews(
		c.Request.Context(),
		service.BatchResolveSharedPoolUsageReviewsInput{
			ReservationIDs: req.ReservationIDs,
			AdminUserID:    subject.UserID,
			Action:         req.Action,
			Note:           req.Note,
			OperationID:    operationID,
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
