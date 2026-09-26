package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CreditSystemHandler struct {
	creditSystemService *service.CreditSystemService
}

func NewCreditSystemHandler(creditSystemService *service.CreditSystemService) *CreditSystemHandler {
	return &CreditSystemHandler{creditSystemService: creditSystemService}
}

func (h *CreditSystemHandler) GetOverview(c *gin.Context) {
	overview, err := h.creditSystemService.GetOverview(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

func (h *CreditSystemHandler) ListLedger(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.CreditLedgerFilter{
		Search:     strings.TrimSpace(c.Query("search")),
		SourceType: strings.TrimSpace(c.Query("source_type")),
		Status:     strings.TrimSpace(c.Query("status")),
		Page:       page,
		PageSize:   pageSize,
	}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = userID
	}
	userTZ := c.Query("timezone")
	if t := parseCreditSystemStartTime(c.Query("start_at"), userTZ); t != nil {
		filter.StartAt = t
	}
	if t := parseCreditSystemEndTime(c.Query("end_at"), userTZ); t != nil {
		filter.EndAt = t
	}

	items, total, err := h.creditSystemService.ListLedger(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *CreditSystemHandler) GetUserSummary(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	summary, err := h.creditSystemService.GetUserSummary(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

type creditSystemGrantRequest struct {
	UserID     int64   `json:"user_id" binding:"required"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Amount     float64 `json:"amount" binding:"required"`
	Note       string  `json:"note"`
}

func (h *CreditSystemHandler) GrantCredit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req creditSystemGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	entry, err := h.creditSystemService.AdminGrantCredit(c.Request.Context(), service.BizCreditGrantInput{
		UserID:     req.UserID,
		SourceType: req.SourceType,
		SourceID:   req.SourceID,
		Amount:     req.Amount,
		Note:       req.Note,
		CreatedBy:  subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entry)
}

func parseCreditSystemStartTime(raw string, userTZ string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ); err == nil {
		return &parsed
	}
	return nil
}

func parseCreditSystemEndTime(raw string, userTZ string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ); err == nil {
		end := parsed.Add(24*time.Hour - time.Nanosecond)
		return &end
	}
	return nil
}
