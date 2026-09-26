package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type marketplaceOnboardingApplication interface {
	GetOnboarding(context.Context, int64) (*service.MarketplaceOnboarding, error)
	CompleteOnboarding(context.Context, int64, string) (*service.MarketplaceOnboarding, error)
}

func (h *MarketplaceHandler) GetOnboarding(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	app, ok := h.app.(marketplaceOnboardingApplication)
	if !ok {
		response.Error(c, 503, "marketplace guide is unavailable")
		return
	}
	result, err := app.GetOnboarding(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *MarketplaceHandler) CompleteOnboarding(c *gin.Context) {
	subject, ok := marketplaceSubject(c)
	if !ok {
		return
	}
	var input struct {
		Intent string `json:"intent"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid onboarding request")
		return
	}
	app, ok := h.app.(marketplaceOnboardingApplication)
	if !ok {
		response.Error(c, 503, "marketplace guide is unavailable")
		return
	}
	result, err := app.CompleteOnboarding(c.Request.Context(), subject.UserID, input.Intent)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
