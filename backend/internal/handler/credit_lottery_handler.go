package handler

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CreditLotteryHandler struct {
	service *service.CreditLotteryService
}

func NewCreditLotteryHandler(service *service.CreditLotteryService) *CreditLotteryHandler {
	return &CreditLotteryHandler{service: service}
}

type creditLotteryCreateRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	OperationID    string `json:"operation_id"`
}

type creditLotteryContinueRequest struct {
	ExpectedRound  int    `json:"expected_round" binding:"required"`
	IdempotencyKey string `json:"idempotency_key"`
	OperationID    string `json:"operation_id"`
}

type creditLotterySettleRequest struct {
	ExpectedRound  int    `json:"expected_round"`
	IdempotencyKey string `json:"idempotency_key"`
	OperationID    string `json:"operation_id"`
}

// CreateSession starts or restores a credit lottery session.
// POST /api/v1/user/credit-lottery/sessions
func (h *CreditLotteryHandler) CreateSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req creditLotteryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	idempotencyKey, ok := resolveCreditLotteryOperationID(c, req.IdempotencyKey, req.OperationID)
	if !ok {
		return
	}
	session, err := h.service.CreateSession(c.Request.Context(), subject.UserID, idempotencyKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

// GetActiveSession returns the current active session, settling it first if expired.
// GET /api/v1/user/credit-lottery/sessions/active
func (h *CreditLotteryHandler) GetActiveSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	session, err := h.service.GetActiveSession(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

// GetSession returns a user-visible session snapshot.
// GET /api/v1/user/credit-lottery/sessions/:id
func (h *CreditLotteryHandler) GetSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sessionID, err := parseCreditLotterySessionID(c)
	if err != nil {
		response.BadRequest(c, "Invalid session id")
		return
	}
	session, err := h.service.GetSession(c.Request.Context(), subject.UserID, sessionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

// ContinueSession advances one round, with expected_round preventing stale clicks.
// POST /api/v1/user/credit-lottery/sessions/:id/continue
func (h *CreditLotteryHandler) ContinueSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sessionID, err := parseCreditLotterySessionID(c)
	if err != nil {
		response.BadRequest(c, "Invalid session id")
		return
	}
	var req creditLotteryContinueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	idempotencyKey, ok := resolveCreditLotteryOperationID(c, req.IdempotencyKey, req.OperationID)
	if !ok {
		return
	}
	session, err := h.service.ContinueSession(c.Request.Context(), subject.UserID, sessionID, req.ExpectedRound, idempotencyKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

// SettleSession settles the current effective round.
// POST /api/v1/user/credit-lottery/sessions/:id/settle
func (h *CreditLotteryHandler) SettleSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	sessionID, err := parseCreditLotterySessionID(c)
	if err != nil {
		response.BadRequest(c, "Invalid session id")
		return
	}
	var req creditLotterySettleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	expectedRound := req.ExpectedRound
	if expectedRound == 0 {
		expectedRound = 1
	}
	idempotencyKey, ok := resolveCreditLotteryOperationID(c, req.IdempotencyKey, req.OperationID)
	if !ok {
		return
	}
	session, err := h.service.SettleSession(c.Request.Context(), subject.UserID, sessionID, expectedRound, service.CreditLotterySettlementUserStop, idempotencyKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

func parseCreditLotterySessionID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

// GetOperationSession restores a credit draw or round result after a timeout.
// GET /api/v1/user/credit-lottery/operations/:operation_id
func (h *CreditLotteryHandler) GetOperationSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	session, err := h.service.GetOperationSession(c.Request.Context(), subject.UserID, strings.TrimSpace(c.Param("operation_id")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, session)
}

func resolveCreditLotteryOperationID(c *gin.Context, idempotencyKey, operationID string) (string, bool) {
	values := []string{
		strings.TrimSpace(idempotencyKey),
		strings.TrimSpace(operationID),
		strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	}
	resolved := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if resolved != "" && resolved != value {
			response.BadRequest(c, "operation_id, idempotency_key and Idempotency-Key must match")
			return "", false
		}
		resolved = value
	}
	return resolved, true
}
