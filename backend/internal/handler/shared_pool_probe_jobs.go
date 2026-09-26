package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) GetSharedPoolProbeJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	jobID := strings.TrimSpace(c.Param("jobId"))
	if jobID == "" {
		response.BadRequest(c, "Invalid probe job id")
		return
	}
	job, err := h.bizService.GetSharedPoolProbeJob(c.Request.Context(), jobID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}
