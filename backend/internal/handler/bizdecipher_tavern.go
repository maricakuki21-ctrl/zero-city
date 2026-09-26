package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type tavernScriptRequest struct {
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	Description        string   `json:"description"`
	Genre              string   `json:"genre"`
	Status             string   `json:"status"`
	Visibility         string   `json:"visibility"`
	PlayerMin          int      `json:"player_min"`
	PlayerMax          int      `json:"player_max"`
	EstimatedMinutes   int      `json:"estimated_minutes"`
	Difficulty         string   `json:"difficulty"`
	Tags               []string `json:"tags"`
	NPCCards           []string `json:"npc_cards"`
	HostBrief          string   `json:"host_brief"`
	OpeningPrompt      string   `json:"opening_prompt"`
	SafetyNotes        string   `json:"safety_notes"`
	PricingMode        string   `json:"pricing_mode"`
	EntryCreditCost    int      `json:"entry_credit_cost"`
	EntryBalanceCost   float64  `json:"entry_balance_cost"`
	AuthorRevenueShare float64  `json:"author_revenue_share"`
}

type tavernScriptReviewRequest struct {
	Status       string   `json:"status"`
	ReviewNote   string   `json:"review_note"`
	QualityScore *float64 `json:"quality_score"`
}

type tavernRoomRequest struct {
	ScriptID         int64          `json:"script_id"`
	Title            string         `json:"title"`
	Visibility       string         `json:"visibility"`
	HostMode         string         `json:"host_mode"`
	BillingMode      string         `json:"billing_mode"`
	EntryCreditCost  int            `json:"entry_credit_cost"`
	EntryBalanceCost float64        `json:"entry_balance_cost"`
	MaxPlayers       int            `json:"max_players"`
	RoomConfig       map[string]any `json:"room_config"`
}

type tavernRuntimeSessionRequest struct {
	Token string `json:"token"`
}

type tavernRoomTurnRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}

type tavernGamePackageRequest struct {
	Version  string          `json:"version"`
	Manifest json.RawMessage `json:"manifest"`
}

func tavernScriptQueryFromRequest(c *gin.Context) service.TavernScriptQuery {
	return service.TavernScriptQuery{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Genre:   strings.TrimSpace(c.Query("genre")),
		Status:  strings.TrimSpace(c.Query("status")),
		Sort:    strings.TrimSpace(c.Query("sort")),
		Limit:   parseLimit(c),
	}
}

func (h *BizDecipherHandler) ListTavernScripts(c *gin.Context) {
	items, err := h.bizService.ListTavernScripts(c.Request.Context(), tavernScriptQueryFromRequest(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) GetTavernScript(c *gin.Context) {
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || scriptID <= 0 {
		response.BadRequest(c, "Invalid tavern script id")
		return
	}
	script, err := h.bizService.GetTavernScript(c.Request.Context(), scriptID)
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Tavern script not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	if script == nil {
		response.NotFound(c, "Tavern script not found")
		return
	}
	response.Success(c, script)
}

func (h *BizDecipherHandler) ListMyTavernScripts(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.bizService.ListMyTavernScripts(c.Request.Context(), subject.UserID, c.Query("status"), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) CreateTavernScript(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req tavernScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	script, err := h.bizService.CreateTavernScript(c.Request.Context(), subject.UserID, service.TavernScriptInput{
		Title:              req.Title,
		Summary:            req.Summary,
		Description:        req.Description,
		Genre:              req.Genre,
		Status:             req.Status,
		Visibility:         req.Visibility,
		PlayerMin:          req.PlayerMin,
		PlayerMax:          req.PlayerMax,
		EstimatedMinutes:   req.EstimatedMinutes,
		Difficulty:         req.Difficulty,
		Tags:               req.Tags,
		NPCCards:           req.NPCCards,
		HostBrief:          req.HostBrief,
		OpeningPrompt:      req.OpeningPrompt,
		SafetyNotes:        req.SafetyNotes,
		PricingMode:        req.PricingMode,
		EntryCreditCost:    req.EntryCreditCost,
		EntryBalanceCost:   req.EntryBalanceCost,
		AuthorRevenueShare: req.AuthorRevenueShare,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, script)
}

func (h *BizDecipherHandler) ListTavernGamePackages(c *gin.Context) {
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || scriptID <= 0 {
		response.BadRequest(c, "Invalid tavern script id")
		return
	}
	var viewerUserID int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		viewerUserID = subject.UserID
	}
	items, err := h.bizService.ListTavernGamePackages(c.Request.Context(), scriptID, viewerUserID)
	if err != nil {
		respondTavernGamePackageError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) CreateTavernGamePackage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || scriptID <= 0 {
		response.BadRequest(c, "Invalid tavern script id")
		return
	}
	var req tavernGamePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	pkg, err := h.bizService.CreateTavernGamePackage(c.Request.Context(), scriptID, subject.UserID, service.TavernGamePackageInput{
		Version:  req.Version,
		Manifest: req.Manifest,
	})
	if err != nil {
		respondTavernGamePackageError(c, err)
		return
	}
	response.Success(c, pkg)
}

func (h *BizDecipherHandler) PublishTavernGamePackage(c *gin.Context) {
	h.setTavernGamePackageStatus(c, true)
}

func (h *BizDecipherHandler) RevokeTavernGamePackage(c *gin.Context) {
	h.setTavernGamePackageStatus(c, false)
}

func (h *BizDecipherHandler) setTavernGamePackageStatus(c *gin.Context, publish bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || scriptID <= 0 {
		response.BadRequest(c, "Invalid tavern script id")
		return
	}
	version := strings.TrimSpace(c.Param("version"))
	if version == "" {
		response.BadRequest(c, "Invalid tavern game package version")
		return
	}
	var pkg *service.TavernGamePackage
	if publish {
		pkg, err = h.bizService.PublishTavernGamePackage(c.Request.Context(), scriptID, subject.UserID, version)
	} else {
		pkg, err = h.bizService.RevokeTavernGamePackage(c.Request.Context(), scriptID, subject.UserID, version)
	}
	if err != nil {
		respondTavernGamePackageError(c, err)
		return
	}
	response.Success(c, pkg)
}

func (h *BizDecipherHandler) AdminListTavernScripts(c *gin.Context) {
	items, err := h.bizService.AdminListTavernScripts(c.Request.Context(), tavernScriptQueryFromRequest(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminReviewTavernScript(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	scriptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || scriptID <= 0 {
		response.BadRequest(c, "Invalid tavern script id")
		return
	}
	var req tavernScriptReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	script, err := h.bizService.AdminReviewTavernScript(c.Request.Context(), scriptID, subject.UserID, service.TavernScriptReviewInput{
		Status:       req.Status,
		ReviewNote:   req.ReviewNote,
		QualityScore: req.QualityScore,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, script)
}

func (h *BizDecipherHandler) CreateTavernRoom(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req tavernRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	room, err := h.bizService.CreateTavernRoom(c.Request.Context(), subject.UserID, service.TavernRoomInput{
		ScriptID:         req.ScriptID,
		Title:            req.Title,
		Visibility:       req.Visibility,
		HostMode:         req.HostMode,
		BillingMode:      req.BillingMode,
		EntryCreditCost:  req.EntryCreditCost,
		EntryBalanceCost: req.EntryBalanceCost,
		MaxPlayers:       req.MaxPlayers,
		RoomConfig:       req.RoomConfig,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Tavern script not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, room)
}

func (h *BizDecipherHandler) ListMyTavernRooms(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.bizService.ListMyTavernRooms(c.Request.Context(), subject.UserID, c.Query("status"), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) GetTavernRoomRuntime(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || roomID <= 0 {
		response.BadRequest(c, "Invalid tavern room id")
		return
	}
	config, err := h.bizService.GetTavernRoomRuntime(c.Request.Context(), subject.UserID, roomID)
	if err != nil {
		respondTavernRoomActionError(c, err)
		return
	}
	response.Success(c, config)
}

func (h *BizDecipherHandler) CreateTavernRuntimeSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || roomID <= 0 {
		response.BadRequest(c, "Invalid tavern room id")
		return
	}
	session, err := h.bizService.CreateTavernRuntimeSession(c.Request.Context(), subject.UserID, roomID)
	if err != nil {
		respondTavernRoomActionError(c, err)
		return
	}
	response.Success(c, session)
}

func (h *BizDecipherHandler) GetTavernRuntimeSession(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req tavernRuntimeSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Token) == "" {
		response.BadRequest(c, "runtime session token is required")
		return
	}
	session, err := h.bizService.GetTavernRuntimeSession(c.Request.Context(), subject.UserID, req.Token)
	if err != nil {
		respondTavernRoomActionError(c, err)
		return
	}
	response.Success(c, session)
}

func (h *BizDecipherHandler) ListTavernRoomTurns(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || roomID <= 0 {
		response.BadRequest(c, "Invalid tavern room id")
		return
	}
	afterIndex := 0
	if raw := strings.TrimSpace(c.Query("after")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 0 {
			response.BadRequest(c, "Invalid turn cursor")
			return
		}
		afterIndex = parsed
	}
	items, err := h.bizService.ListTavernRoomTurns(c.Request.Context(), subject.UserID, roomID, afterIndex, parseLimit(c))
	if err != nil {
		respondTavernRoomActionError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AppendTavernRoomTurn(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || roomID <= 0 {
		response.BadRequest(c, "Invalid tavern room id")
		return
	}
	var req tavernRoomTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	turn, err := h.bizService.AppendTavernRoomTurn(c.Request.Context(), subject.UserID, roomID, service.TavernRoomTurnInput{
		ClientMessageID: req.ClientMessageID,
		Body:            req.Body,
	})
	if err != nil {
		respondTavernRoomActionError(c, err)
		return
	}
	response.Success(c, turn)
}

func (h *BizDecipherHandler) OpenTavernRoom(c *gin.Context) {
	h.handleTavernRoomAction(c, h.bizService.OpenTavernRoom)
}

func (h *BizDecipherHandler) JoinTavernRoom(c *gin.Context) {
	h.handleTavernRoomAction(c, h.bizService.JoinTavernRoom)
}

func (h *BizDecipherHandler) StartTavernRoom(c *gin.Context) {
	h.handleTavernRoomAction(c, h.bizService.StartTavernRoom)
}

func (h *BizDecipherHandler) CompleteTavernRoom(c *gin.Context) {
	h.handleTavernRoomAction(c, h.bizService.CompleteTavernRoom)
}

func (h *BizDecipherHandler) CancelTavernRoom(c *gin.Context) {
	h.handleTavernRoomAction(c, h.bizService.CancelTavernRoom)
}

func (h *BizDecipherHandler) handleTavernRoomAction(c *gin.Context, action func(ctx context.Context, userID, roomID int64) (*service.TavernRoom, error)) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || roomID <= 0 {
		response.BadRequest(c, "Invalid tavern room id")
		return
	}
	room, err := action(c.Request.Context(), subject.UserID, roomID)
	if err != nil {
		respondTavernRoomActionError(c, err)
		return
	}
	response.Success(c, room)
}

func respondTavernRoomActionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		response.NotFound(c, "Tavern room not found")
	case errors.Is(err, service.ErrTavernRoomForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrTavernRoomNotJoinable), errors.Is(err, service.ErrTavernRoomFull), errors.Is(err, service.ErrTavernRoomState), errors.Is(err, service.ErrTavernInsufficientCredit), errors.Is(err, service.ErrTavernBalanceBillingUnsupported):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrTavernTurnConflict):
		response.Error(c, http.StatusConflict, err.Error())
	default:
		response.ErrorFrom(c, err)
	}
}

func respondTavernGamePackageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTavernGamePackageNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrTavernGamePackageForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrTavernGamePackageConflict), errors.Is(err, service.ErrTavernGamePackageUnavailable):
		response.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrTavernGamePackageInvalid):
		response.BadRequest(c, err.Error())
	default:
		response.ErrorFrom(c, err)
	}
}
