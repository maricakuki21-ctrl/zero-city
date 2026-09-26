package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// ListMySharedPoolUsageTraces returns only the authenticated member's safe
// shared-pool request metadata. Credential-bearing account details are never
// selected by the repository or exposed by the DTO.
func (h *BizDecipherHandler) ListMySharedPoolUsageTraces(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
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
	page, err := h.bizService.ListMySharedPoolUsageTraces(c.Request.Context(), subject.UserID, beforeID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func parseSharedPoolUsageTracePositiveQuery(c *gin.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil && value > 0
}
