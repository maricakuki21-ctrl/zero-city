package handler

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func withdrawalError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		response.NotFound(c, "Withdrawal not found")
	case errors.Is(err, service.ErrWithdrawalConflict):
		response.Error(c, 409, err.Error())
	case errors.Is(err, service.ErrWithdrawalInvalid), errors.Is(err, service.ErrWithdrawalClosed), errors.Is(err, service.ErrInsufficientBalance):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "Withdrawal operation failed")
	}
}
func withdrawalIdentity(c *gin.Context, admin bool) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Authentication required")
		return 0, false
	}
	if admin {
		role, _ := middleware.GetUserRoleFromContext(c)
		if role != "admin" {
			response.Forbidden(c, "Administrator required")
			return 0, false
		}
	}
	return subject.UserID, true
}
func (h *BizDecipherHandler) GetWithdrawalPolicy(c *gin.Context) {
	if _, ok := withdrawalIdentity(c, false); !ok {
		return
	}
	r, err := h.bizService.WithdrawalRepo()
	if err != nil {
		withdrawalError(c, err)
		return
	}
	p, err := r.WithdrawalPolicy(c.Request.Context())
	if err != nil {
		withdrawalError(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) AdminSaveWithdrawalPolicy(c *gin.Context) {
	actor, ok := withdrawalIdentity(c, true)
	if !ok {
		return
	}
	var p service.WithdrawalPolicy
	if c.ShouldBindJSON(&p) != nil {
		response.BadRequest(c, "Invalid policy")
		return
	}
	r, err := h.bizService.WithdrawalRepo()
	if err == nil {
		err = r.SaveWithdrawalPolicy(c.Request.Context(), actor, p)
	}
	if err != nil {
		withdrawalError(c, err)
		return
	}
	response.Success(c, p)
}
func (h *BizDecipherHandler) ListWithdrawals(c *gin.Context)      { h.listWithdrawals(c, false) }
func (h *BizDecipherHandler) AdminListWithdrawals(c *gin.Context) { h.listWithdrawals(c, true) }
func (h *BizDecipherHandler) listWithdrawals(c *gin.Context, admin bool) {
	owner, ok := withdrawalIdentity(c, admin)
	if !ok {
		return
	}
	if admin {
		owner = 0
	}
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	r, err := h.bizService.WithdrawalRepo()
	if err != nil {
		withdrawalError(c, err)
		return
	}
	rows, err := r.ListWithdrawals(c.Request.Context(), owner, before)
	if err != nil {
		withdrawalError(c, err)
		return
	}
	response.Success(c, rows)
}
func (h *BizDecipherHandler) CreateWithdrawal(c *gin.Context) {
	owner, ok := withdrawalIdentity(c, false)
	if !ok {
		return
	}
	var in service.WithdrawalInput
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Amount must be a decimal string")
		return
	}
	r, err := h.bizService.WithdrawalRepo()
	if err != nil {
		withdrawalError(c, err)
		return
	}
	w, err := r.CreateWithdrawal(c.Request.Context(), owner, in)
	if err != nil {
		withdrawalError(c, err)
		return
	}
	response.Success(c, w)
}
func (h *BizDecipherHandler) CancelWithdrawal(c *gin.Context)   { h.actWithdrawal(c, false) }
func (h *BizDecipherHandler) AdminActWithdrawal(c *gin.Context) { h.actWithdrawal(c, true) }
func (h *BizDecipherHandler) actWithdrawal(c *gin.Context, admin bool) {
	actor, ok := withdrawalIdentity(c, admin)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	a := service.WithdrawalAction{Action: "cancelled"}
	if admin && c.ShouldBindJSON(&a) != nil {
		response.BadRequest(c, "Invalid action")
		return
	}
	r, err := h.bizService.WithdrawalRepo()
	if err != nil {
		withdrawalError(c, err)
		return
	}
	w, err := r.ActWithdrawal(c.Request.Context(), actor, id, admin, a)
	if err != nil {
		withdrawalError(c, err)
		return
	}
	response.Success(c, w)
}
