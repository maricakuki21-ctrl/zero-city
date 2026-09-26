package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ChatApplication interface {
	ListChannels(context.Context) ([]service.ChatChannel, error)
	ListMessages(context.Context, string, int64, service.ChatMessageQuery) (service.ChatMessagePage, error)
	SendMessage(context.Context, string, int64, service.ChatMessageInput) (*service.ChatMessage, error)
}

type ChatHandler struct {
	app     ChatApplication
	rewards *service.TokenRewardService
}

func NewChatHandler(app *service.ChatService, rewards *service.TokenRewardService) *ChatHandler {
	return &ChatHandler{app: app, rewards: rewards}
}

func (h *ChatHandler) ListChannels(c *gin.Context) {
	if _, ok := chatSubject(c); !ok {
		return
	}
	channels, err := h.app.ListChannels(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": channels})
}

func (h *ChatHandler) ListMessages(c *gin.Context) {
	subject, ok := chatSubject(c)
	if !ok {
		return
	}
	slug := strings.TrimSpace(c.Param("slug"))
	after, _ := strconv.ParseInt(strings.TrimSpace(c.Query("after")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	page, err := h.app.ListMessages(c.Request.Context(), slug, subject.UserID, service.ChatMessageQuery{After: after, Limit: limit})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	subject, ok := chatSubject(c)
	if !ok {
		return
	}
	var input service.ChatMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid chat message request")
		return
	}
	message, err := h.app.SendMessage(c.Request.Context(), strings.TrimSpace(c.Param("slug")), subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, message)
}

func chatSubject(c *gin.Context) (middleware.AuthSubject, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
	}
	return subject, ok
}
