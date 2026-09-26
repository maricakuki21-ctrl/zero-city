package handler

import (
	"errors"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func writeCommunityAccessError(c *gin.Context, err error) bool {
	var appError *infraerrors.ApplicationError
	if errors.As(err, &appError) {
		response.ErrorFrom(c, err)
		return true
	}
	return false
}

func (h *BizDecipherHandler) GetCommunityParticipation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.bizService.GetCommunityParticipation(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) AdminSetCommunityParticipation(c *gin.Context) {
	if !requireGovernanceAdmin(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	var input struct {
		Level  *int   `json:"level" binding:"required"`
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请填写资格与依据")
		return
	}
	result, err := h.bizService.SetCommunityParticipation(c.Request.Context(), subject.UserID, userID, *input.Level, input.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
