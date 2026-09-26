package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type communityPollVoteRequest struct {
	OptionID int64 `json:"option_id" binding:"required"`
}

func (h *BizDecipherHandler) ListCommunityPolls(c *gin.Context) {
	viewerID := int64(0)
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		viewerID = subject.UserID
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	polls, err := h.bizService.ListCommunityPolls(c.Request.Context(), viewerID, role == service.RoleAdmin, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": polls})
}

func (h *BizDecipherHandler) CreateCommunityPoll(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var input service.CommunityPollInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	poll, err := h.bizService.CreateCommunityPoll(c.Request.Context(), subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, poll)
}

func (h *BizDecipherHandler) VoteCommunityPoll(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	pollID, ok := pollIDFromRequest(c)
	if !ok {
		return
	}
	var input communityPollVoteRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	poll, err := h.bizService.VoteCommunityPoll(c.Request.Context(), pollID, subject.UserID, input.OptionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, poll)
}

func (h *BizDecipherHandler) CloseCommunityPoll(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	pollID, ok := pollIDFromRequest(c)
	if !ok {
		return
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	poll, err := h.bizService.CloseCommunityPoll(c.Request.Context(), pollID, subject.UserID, role == service.RoleAdmin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, poll)
}

func (h *BizDecipherHandler) AdminCloseCommunityPoll(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	pollID, ok := pollIDFromRequest(c)
	if !ok {
		return
	}
	poll, err := h.bizService.CloseCommunityPoll(c.Request.Context(), pollID, subject.UserID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, poll)
}

func pollIDFromRequest(c *gin.Context) (int64, bool) {
	pollID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || pollID <= 0 {
		response.BadRequest(c, "Invalid poll id")
		return 0, false
	}
	return pollID, true
}
