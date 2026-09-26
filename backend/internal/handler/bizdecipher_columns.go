package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func creatorColumnQuery(c *gin.Context, admin bool) (service.CreatorColumnQuery, bool) {
	q := service.CreatorColumnQuery{Admin: admin, Limit: 20}
	if role, ok := middleware2.GetUserRoleFromContext(c); ok && role == service.RoleAdmin {
		q.Admin = true
	}
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		q.ViewerID = subject.UserID
	}
	if v := c.Query("mine"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			response.BadRequest(c, "Invalid mine")
			return q, false
		}
		q.Mine = b
	}
	if q.Mine && q.ViewerID == 0 {
		response.Unauthorized(c, "User not authenticated")
		return q, false
	}
	if v := c.Query("cursor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			response.BadRequest(c, "Invalid cursor")
			return q, false
		}
		q.Cursor = n
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			response.BadRequest(c, "Invalid limit")
			return q, false
		}
		q.Limit = n
	}
	return q, true
}
func creatorID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}
func creatorActor(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}
func (h *BizDecipherHandler) ListCreatorColumns(c *gin.Context)      { h.listCreatorColumns(c, false) }
func (h *BizDecipherHandler) AdminListCreatorColumns(c *gin.Context) { h.listCreatorColumns(c, true) }
func (h *BizDecipherHandler) listCreatorColumns(c *gin.Context, admin bool) {
	q, ok := creatorColumnQuery(c, admin)
	if !ok {
		return
	}
	items, err := h.bizService.ListCreatorColumns(c.Request.Context(), q)
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
func (h *BizDecipherHandler) GetCreatorColumn(c *gin.Context) {
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	q, ok := creatorColumnQuery(c, false)
	if !ok {
		return
	}
	item, err := h.bizService.GetCreatorColumn(c.Request.Context(), id, q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
func (h *BizDecipherHandler) CreateCreatorColumn(c *gin.Context) { h.saveCreatorColumn(c, false) }
func (h *BizDecipherHandler) UpdateCreatorColumn(c *gin.Context) { h.saveCreatorColumn(c, true) }
func (h *BizDecipherHandler) saveCreatorColumn(c *gin.Context, update bool) {
	userID, ok := creatorActor(c)
	if !ok {
		return
	}
	var id int64
	if update {
		id, ok = creatorID(c, "id")
		if !ok {
			return
		}
	}
	var in service.CreatorColumnInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid input")
		return
	}
	item, err := h.bizService.SaveCreatorColumn(c.Request.Context(), id, userID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if update {
		response.Success(c, item)
	} else {
		response.Created(c, item)
	}
}
func (h *BizDecipherHandler) AdminModerateCreatorColumn(c *gin.Context) {
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Status           string `json:"status"`
		ModerationReason string `json:"moderation_reason"`
		Reason           string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid input")
		return
	}
	if in.ModerationReason == "" {
		in.ModerationReason = in.Reason
	}
	item, err := h.bizService.ModerateCreatorColumn(c.Request.Context(), id, in.Status, in.ModerationReason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
func (h *BizDecipherHandler) ListCreatorColumnArticles(c *gin.Context) {
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	q, ok := creatorColumnQuery(c, false)
	if !ok {
		return
	}
	items, err := h.bizService.ListCreatorColumnArticles(c.Request.Context(), id, q)
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
func (h *BizDecipherHandler) GetCreatorColumnArticle(c *gin.Context) {
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	articleID, ok := creatorID(c, "articleId")
	if !ok {
		return
	}
	q, ok := creatorColumnQuery(c, false)
	if !ok {
		return
	}
	item, err := h.bizService.GetCreatorColumnArticle(c.Request.Context(), id, articleID, q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
func (h *BizDecipherHandler) CreateCreatorColumnArticle(c *gin.Context) {
	h.saveCreatorColumnArticle(c, false)
}
func (h *BizDecipherHandler) UpdateCreatorColumnArticle(c *gin.Context) {
	h.saveCreatorColumnArticle(c, true)
}
func (h *BizDecipherHandler) saveCreatorColumnArticle(c *gin.Context, update bool) {
	userID, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := creatorID(c, "id")
	if !ok {
		return
	}
	var articleID int64
	if update {
		articleID, ok = creatorID(c, "articleId")
		if !ok {
			return
		}
	}
	var in service.CreatorColumnArticleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid input")
		return
	}
	item, err := h.bizService.SaveCreatorColumnArticle(c.Request.Context(), id, articleID, userID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if update {
		response.Success(c, item)
	} else {
		response.Created(c, item)
	}
}
