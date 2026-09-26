package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type revokeGovernanceRuleRequest struct {
	Reason string `json:"reason"`
}

type communityBadgeMutationRequest struct {
	UserID int64  `json:"user_id" binding:"required"`
	Reason string `json:"reason"`
}

func (h *BizDecipherHandler) ListGovernanceRules(c *gin.Context) {
	rules, err := h.bizService.ListGovernanceRules(c.Request.Context(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": rules})
}

func (h *BizDecipherHandler) AdoptCommunityPollAsRule(c *gin.Context) {
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
	rule, err := h.bizService.AdoptCommunityPollAsRule(c.Request.Context(), pollID, subject.UserID, role == service.RoleAdmin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, rule)
}

func (h *BizDecipherHandler) AdminRevokeGovernanceRule(c *gin.Context) {
	if !requireGovernanceAdmin(c) { return }
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ruleID <= 0 {
		response.BadRequest(c, "Invalid rule id")
		return
	}
	var input revokeGovernanceRuleRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.bizService.RevokeGovernanceRule(c.Request.Context(), ruleID, subject.UserID, input.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

func (h *BizDecipherHandler) ListCommunityBadges(c *gin.Context) {
	badges, err := h.bizService.ListCommunityBadges(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": badges})
}

func (h *BizDecipherHandler) ListUserBadges(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	grants, err := h.bizService.ListUserBadges(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": grants})
}

func (h *BizDecipherHandler) AdminGrantCommunityBadge(c *gin.Context) {
	if !requireGovernanceAdmin(c) { return }
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	badgeKey := strings.TrimSpace(c.Param("key"))
	if badgeKey == "" {
		response.BadRequest(c, "Invalid badge key")
		return
	}
	var input communityBadgeMutationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	grant, err := h.bizService.GrantCommunityBadge(c.Request.Context(), badgeKey, input.UserID, subject.UserID, input.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, grant)
}

func (h *BizDecipherHandler) AdminRevokeCommunityBadge(c *gin.Context) {
	if !requireGovernanceAdmin(c) { return }
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	badgeKey := strings.TrimSpace(c.Param("key"))
	if badgeKey == "" {
		response.BadRequest(c, "Invalid badge key")
		return
	}
	var input communityBadgeMutationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	grant, err := h.bizService.RevokeCommunityBadge(c.Request.Context(), badgeKey, input.UserID, subject.UserID, input.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, grant)
}

func requireGovernanceAdmin(c *gin.Context) bool {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return false
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	if role != service.RoleAdmin {
		response.Forbidden(c, "Administrator required")
		return false
	}
	return true
}
