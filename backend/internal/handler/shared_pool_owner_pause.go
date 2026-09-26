package handler

import (
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) SetSharedPoolOwnerPause(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var input struct {
		Paused                *bool `json:"paused" binding:"required"`
		ExpectedConfigVersion int64 `json:"expected_config_version" binding:"required,min=1"`
	}
	if err = c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "paused and expected_config_version are required")
		return
	}
	pool, err := h.bizService.SetSharedPoolOwnerPause(c.Request.Context(), id, subject.UserID, input.ExpectedConfigVersion, *input.Paused)
	if err != nil {
		if errors.Is(err, service.ErrSharedPoolConcurrentUpdate) {
			response.Error(c, 409, "配置已变化，请刷新后重试")
			return
		}
		if errors.Is(err, service.ErrPoolForbidden) {
			response.Error(c, 403, "无权管理此池或池子已删除")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}
