package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *ChatHandler) RewardResources(c *gin.Context) {
	if _, ok := chatSubject(c); !ok {
		return
	}
	response.Success(c, gin.H{"items": h.rewards.Resources()})
}
func (h *ChatHandler) CreateTokenPacket(c *gin.Context) {
	sub, ok := chatSubject(c)
	if !ok {
		return
	}
	role, _ := middleware.GetUserRoleFromContext(c)
	if role != service.RoleAdmin {
		response.Forbidden(c, "仅运营方可以发红包")
		return
	}
	var in service.TokenPacketInput
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "红包请求格式不正确")
		return
	}
	p, err := h.rewards.Create(c.Request.Context(), c.Param("slug"), sub.UserID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, p)
}
func tokenPacketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "红包编号不正确")
		return 0, false
	}
	return id, true
}
func (h *ChatHandler) GetTokenPacket(c *gin.Context) {
	sub, ok := chatSubject(c)
	if !ok {
		return
	}
	id, ok := tokenPacketID(c)
	if !ok {
		return
	}
	p, err := h.rewards.Get(c.Request.Context(), id, sub.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *ChatHandler) ClaimTokenPacket(c *gin.Context) {
	sub, ok := chatSubject(c)
	if !ok {
		return
	}
	id, ok := tokenPacketID(c)
	if !ok {
		return
	}
	g, err := h.rewards.Claim(c.Request.Context(), id, sub.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, g)
}
func (h *ChatHandler) TokenRewardWallet(c *gin.Context) {
	sub, ok := chatSubject(c)
	if !ok {
		return
	}
	w, err := h.rewards.Wallet(c.Request.Context(), sub.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, w)
}
func (h *ChatHandler) RunTokenReward(c *gin.Context) {
	sub, ok := chatSubject(c)
	if !ok {
		return
	}
	var in service.TokenRewardRunInput
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "奖励调用请求格式不正确")
		return
	}
	r, err := h.rewards.Run(c.Request.Context(), sub.UserID, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, r)
}

func (h *ChatHandler) ReviewTokenRewards(c *gin.Context) {
	if _, ok := chatSubject(c); !ok {
		return
	}
	list, err := h.rewards.Reviews(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": list})
}
func (h *ChatHandler) ResolveTokenReward(c *gin.Context) {
	sub, ok := chatSubject(c)
	if !ok {
		return
	}
	id, ok := tokenPacketID(c)
	if !ok {
		return
	}
	var in struct {
		Actual   *int64 `json:"actual_tokens"`
		Evidence string `json:"evidence"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Actual == nil {
		response.BadRequest(c, "请提供核实后的实际用量与依据")
		return
	}
	run, err := h.rewards.Resolve(c.Request.Context(), id, sub.UserID, *in.Actual, in.Evidence)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, run)
}
