package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type BizDecipherHandler struct {
	bizService *service.BizDecipherService
	workbench  workbench.CanonicalAdapter
}

func NewBizDecipherHandler(bizService *service.BizDecipherService, adapters ...workbench.CanonicalAdapter) *BizDecipherHandler {
	var workbenchAdapter workbench.CanonicalAdapter = workbench.UnavailableAdapter{}
	if len(adapters) > 0 && adapters[0] != nil {
		workbenchAdapter = adapters[0]
	}
	return &BizDecipherHandler{bizService: bizService, workbench: workbenchAdapter}
}

func (h *BizDecipherHandler) GetPoolStatus(c *gin.Context) {
	status, err := h.bizService.GetPoolStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// GetPlatformStatus returns gateway-level health (channels & upstream
// accounts), deliberately separate from the marketplace pool status so the
// frontend can render an independent platform status bar.
func (h *BizDecipherHandler) GetPlatformStatus(c *gin.Context) {
	status, err := h.bizService.GetPlatformStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *BizDecipherHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	profile, err := h.bizService.GetProfile(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

func (h *BizDecipherHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.ZeroCityProfileIdentityInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.bizService.UpdateZeroCityProfileIdentity(c.Request.Context(), subject.UserID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, profile)
}

func (h *BizDecipherHandler) GetZeroCityPublicProfile(c *gin.Context) {
	viewerID := int64(0)
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		viewerID = subject.UserID
	}
	profile, err := h.bizService.GetZeroCityPublicProfile(c.Request.Context(), viewerID, c.Param("target"))
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Profile not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

func (h *BizDecipherHandler) FollowZeroCityProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	targetID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || targetID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	state, err := h.bizService.FollowZeroCityProfile(c.Request.Context(), subject.UserID, targetID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, state)
}

func (h *BizDecipherHandler) UnfollowZeroCityProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	targetID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || targetID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	state, err := h.bizService.UnfollowZeroCityProfile(c.Request.Context(), subject.UserID, targetID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, state)
}

func (h *BizDecipherHandler) UpdateZeroCityProfileBackground(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req zeroCityBackgroundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.bizService.UpdateZeroCityProfileBackground(c.Request.Context(), subject.UserID, service.ZeroCityProfileBackgroundInput{CardKey: req.CardKey, SerialNo: req.SerialNo})
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Card not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, profile)
}

type applyContributorRequest struct {
	CapacityTypes    []string `json:"capacity_types"`
	SettlementMethod string   `json:"settlement_method"`
	CapacityHint     string   `json:"capacity_hint"`
	Label            string   `json:"label"`
}

func (h *BizDecipherHandler) ApplyContributor(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req applyContributorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.bizService.ApplyContributor(c.Request.Context(), subject.UserID, service.BizContributorApplication{
		CapacityTypes:    normalizeStringList(req.CapacityTypes),
		SettlementMethod: strings.TrimSpace(req.SettlementMethod),
		CapacityHint:     strings.TrimSpace(req.CapacityHint),
		Label:            strings.TrimSpace(req.Label),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

type applyOperatorRequest struct {
	Skills []string `json:"skills"`
}

func (h *BizDecipherHandler) ApplyOperator(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req applyOperatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.bizService.ApplyOperator(c.Request.Context(), subject.UserID, service.BizOperatorApplication{Skills: normalizeStringList(req.Skills)})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

type customRequestPayload struct {
	Goal       string   `json:"goal"`
	Context    string   `json:"context"`
	Budget     string   `json:"budget_range"`
	Deadline   string   `json:"deadline"`
	References []string `json:"references"`
}

func (h *BizDecipherHandler) CreateCustomRequest(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req customRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Goal) == "" {
		response.BadRequest(c, "Goal is required")
		return
	}
	created, err := h.bizService.CreateCustomRequest(c.Request.Context(), subject.UserID, service.BizCustomRequestInput{
		Goal:       strings.TrimSpace(req.Goal),
		Context:    strings.TrimSpace(req.Context),
		Budget:     strings.TrimSpace(req.Budget),
		Deadline:   strings.TrimSpace(req.Deadline),
		References: normalizeStringList(req.References),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, created)
}

func (h *BizDecipherHandler) ListMyCredits(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	entries, err := h.bizService.ListCreditLedger(c.Request.Context(), subject.UserID, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": entries})
}

func (h *BizDecipherHandler) ListMySharedPoolLedger(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ledger, err := h.bizService.ListMySharedPoolLedger(c.Request.Context(), subject.UserID, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ledger)
}

func (h *BizDecipherHandler) ListSharedPoolOwnerEarningsPage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	parseOptionalID := func(name string) (int64, bool) {
		raw := strings.TrimSpace(c.Query(name))
		if raw == "" {
			return 0, true
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		return value, err == nil && value > 0
	}
	beforeID, ok := parseOptionalID("before_id")
	if !ok {
		response.BadRequest(c, "before_id must be a positive integer")
		return
	}
	poolID, ok := parseOptionalID("pool_id")
	if !ok {
		response.BadRequest(c, "pool_id must be a positive integer")
		return
	}
	page, err := h.bizService.ListSharedPoolOwnerEarningsPage(
		c.Request.Context(),
		subject.UserID,
		poolID,
		beforeID,
		parseLimit(c),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

type transferSharedPoolOwnerEarningsRequest struct {
	Amount      float64 `json:"amount"`
	OperationID string  `json:"operation_id"`
}

func (h *BizDecipherHandler) TransferSharedPoolOwnerEarnings(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req transferSharedPoolOwnerEarningsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	if headerID := strings.TrimSpace(c.GetHeader("Idempotency-Key")); headerID != "" {
		if operationID != "" && operationID != headerID {
			response.BadRequest(c, "operation_id and Idempotency-Key must match")
			return
		}
		operationID = headerID
	}
	result, err := h.bizService.TransferSharedPoolOwnerEarnings(c.Request.Context(), subject.UserID, req.Amount, operationID)
	if err != nil {
		if err == service.ErrInsufficientBalance {
			response.BadRequest(c, "可转收益不足，请刷新收益账本后重试")
			return
		}
		if errors.Is(err, service.ErrIdempotencyKeyConflict) {
			response.ErrorFrom(c, err)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

type communityPostRequest struct {
	Kind         string          `json:"kind"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Tags         []string        `json:"tags"`
	District     string          `json:"district"`
	Channel      string          `json:"channel"`
	Private      bool            `json:"private"`
	SourceType   string          `json:"source_type"`
	SourceID     string          `json:"source_id"`
	Scenario     string          `json:"scenario"`
	SubjectType  string          `json:"subject_type"`
	SubjectID    string          `json:"subject_id"`
	SubjectTitle string          `json:"subject_title"`
	ActionType   string          `json:"action_type"`
	Evidence     json.RawMessage `json:"evidence"`
	TrustSignals json.RawMessage `json:"trust_signals"`
}

type zeroCityBackgroundRequest struct {
	CardKey  string `json:"card_key"`
	SerialNo *int64 `json:"serial_no"`
}

type communityCommentRequest struct {
	Body       string `json:"body"`
	HelperRole string `json:"helper_role"`
}

type communityStatusRequest struct {
	Status   string `json:"status"`
	Official bool   `json:"official"`
	Pinned   *bool  `json:"pinned"`
}

type communityPostActionRequest struct {
	Action    string `json:"action"`
	Status    string `json:"status"`
	CommentID int64  `json:"comment_id"`
}

func communityPostQueryFromRequest(c *gin.Context, admin bool) service.CommunityPostQuery {
	return service.CommunityPostQuery{
		Kind:        c.Query("kind"),
		District:    c.Query("district"),
		Channel:     c.Query("channel"),
		SourceType:  c.Query("source_type"),
		SourceID:    c.Query("source_id"),
		Scenario:    c.Query("scenario"),
		SubjectType: c.Query("subject_type"),
		SubjectID:   c.Query("subject_id"),
		ActionType:  c.Query("action_type"),
		Status:      c.Query("status"),
		PrivateOnly: admin && (c.Query("private") == "true" || c.Query("private") == "1"),
		Limit:       parseLimit(c),
	}
}

func (h *BizDecipherHandler) ListCommunityPosts(c *gin.Context) {
	items, err := h.bizService.ListCommunityPosts(c.Request.Context(), communityPostQueryFromRequest(c, false))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func parseInt64CSV(values []string) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id <= 0 || seen[id] {
				continue
			}
			out = append(out, id)
			seen[id] = true
			if len(out) >= 100 {
				return out
			}
		}
	}
	return out
}

func (h *BizDecipherHandler) ListSharedPoolCommunitySummaries(c *gin.Context) {
	poolIDs := parseInt64CSV(c.QueryArray("pool_ids"))
	if len(poolIDs) == 0 {
		poolIDs = parseInt64CSV([]string{c.Query("ids")})
	}
	summaries, err := h.bizService.ListSharedPoolCommunitySummaries(c.Request.Context(), poolIDs, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": summaries})
}

func (h *BizDecipherHandler) ListMyCommunityPosts(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.bizService.ListMyCommunityPosts(c.Request.Context(), subject.UserID, c.Query("kind"), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListCommunityPosts(c *gin.Context) {
	items, err := h.bizService.AdminListCommunityPosts(c.Request.Context(), communityPostQueryFromRequest(c, true))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminUpdateCommunityPostStatus(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		response.BadRequest(c, "Invalid post id")
		return
	}
	var req communityStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	post, err := h.bizService.UpdateCommunityPostModeration(c.Request.Context(), postID, req.Status, req.Pinned)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, post)
}

func (h *BizDecipherHandler) AdminUpdateCommunityCommentStatus(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || commentID <= 0 {
		response.BadRequest(c, "Invalid comment id")
		return
	}
	var req communityStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	comment, err := h.bizService.UpdateCommunityCommentStatus(c.Request.Context(), commentID, req.Status, req.Official)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, comment)
}

func (h *BizDecipherHandler) CreateCommunityPost(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req communityPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	post, err := h.bizService.CreateCommunityPost(c.Request.Context(), subject.UserID, service.CommunityPostInput{
		Kind:         req.Kind,
		Title:        req.Title,
		Body:         req.Body,
		Tags:         normalizeStringList(req.Tags),
		District:     req.District,
		Channel:      req.Channel,
		Private:      req.Private,
		SourceType:   req.SourceType,
		SourceID:     req.SourceID,
		Scenario:     req.Scenario,
		SubjectType:  req.SubjectType,
		SubjectID:    req.SubjectID,
		SubjectTitle: req.SubjectTitle,
		ActionType:   req.ActionType,
		Evidence:     req.Evidence,
		TrustSignals: req.TrustSignals,
	})
	if err != nil {
		if writeCommunityAccessError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, post)
}

func (h *BizDecipherHandler) UpdateCommunityPostAction(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		response.BadRequest(c, "Invalid post id")
		return
	}
	var req communityPostActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	action := strings.TrimSpace(req.Action)
	if action == "" && req.Status != "" {
		action = "set_status"
	}
	var post *service.CommunityPost
	switch action {
	case "accept_comment":
		post, err = h.bizService.AcceptCommunityComment(c.Request.Context(), postID, req.CommentID, subject.UserID)
	case "set_status", "mark_resolved", "reopen":
		status := req.Status
		if action == "mark_resolved" {
			status = "resolved"
		}
		if action == "reopen" {
			status = "open"
		}
		post, err = h.bizService.UpdateOwnedCommunityPostStatus(c.Request.Context(), postID, subject.UserID, status)
	default:
		response.BadRequest(c, "Invalid community action")
		return
	}
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Community post or comment not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, post)
}

func (h *BizDecipherHandler) ListCommunityComments(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		response.BadRequest(c, "Invalid post id")
		return
	}
	items, err := h.bizService.ListCommunityComments(c.Request.Context(), postID, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) CreateCommunityComment(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		response.BadRequest(c, "Invalid post id")
		return
	}
	var req communityCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	comment, err := h.bizService.CreateCommunityComment(c.Request.Context(), postID, subject.UserID, service.CommunityCommentInput{Body: req.Body, HelperRole: req.HelperRole})
	if err != nil {
		if writeCommunityAccessError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, comment)
}

func (h *BizDecipherHandler) GetTokenPowerLeaderboard(c *gin.Context) {
	leaderboard, err := h.bizService.GetTokenPowerLeaderboard(c.Request.Context(), time.Now(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, leaderboard)
}

type communityMascotChatRequest struct {
	MascotKey    string                       `json:"mascot_key"`
	Message      string                       `json:"message"`
	Locale       string                       `json:"locale"`
	Conversation []communityMascotChatMessage `json:"conversation"`
}

type communityMascotChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type communityMascotDuty struct {
	Key            string
	Name           string
	Prompt         string
	Suggestions    []string
	ModerationHint string
}

var communityMascotDuties = map[string]communityMascotDuty{
	"communityAnnouncer": {
		Key:         "communityAnnouncer",
		Name:        "社区广播员",
		Prompt:      "你是 BizDecipher 零点城的社区广播员。语气温暖、简短、有轻微小人 IP 感。你负责欢迎新居民、引导用户去反馈通道、把模糊问题整理成可被社区接住的帖子。回答要优先让用户补充模型名、客户端、报错码、时间点、订单号或截图，不要承诺已经人工处理。",
		Suggestions: []string{"我要反馈余额问题", "Claude Code 409 怎么办", "怎么发求助帖"},
	},
	"repairSupport": {
		Key:         "repairSupport",
		Name:        "云端修补匠",
		Prompt:      "你是 BizDecipher 零点城的云端修补匠。你专门处理 API、模型、客户端和代理错误。回答必须给排查清单：模型名、Base URL、客户端、HTTP 状态码、请求时间、是否重试、是否有余额。语气可靠、直接，不编造后台状态。",
		Suggestions: []string{"API 一直 401", "模型不可用", "Base URL 怎么填"},
	},
	"creditCashier": {
		Key:         "creditCashier",
		Name:        "积分收银员",
		Prompt:      "你是 BizDecipher 零点城的积分收银员。你只做余额、充值、兑换码、签到积分、返利的引导。回答要提醒用户提供邮箱、订单号、支付方式、兑换码、支付时间和截图。不要要求用户公开隐私到广场，建议走反馈通道或私聊客服。",
		Suggestions: []string{"充值没到账", "兑换码失败", "余额变少了"},
	},
	"sharedPoolOwner": {
		Key:         "sharedPoolOwner",
		Name:        "共享池主",
		Prompt:      "你是 BizDecipher 零点城的共享池主向导。你帮助用户理解如何挂 Key、如何看收益、如何保持稳定、如何避免滥用风险。回答要强调可用率、延迟、成功率、最低余额、席位小时费和治理规则。",
		Suggestions: []string{"怎么成为池主", "收益怎么算", "如何提高可用率"},
	},
	"archiveLibrarian": {
		Key:         "archiveLibrarian",
		Name:        "小人档案管理员",
		Prompt:      "你是 BizDecipher 零点城的小人档案管理员。你负责解释小人卡、晒卡、收藏、个人背景和未来工作流资产。语气有世界观，但回答要落到产品操作：抽卡、收藏、发帖、反馈、资料背景。",
		Suggestions: []string{"小人卡有什么用", "怎么晒卡", "稀有卡以后能干嘛"},
	},
	"riskSweeper": {
		Key:         "riskSweeper",
		Name:        "风险清扫员",
		Prompt:      "你是 BizDecipher 零点城的风险清扫员。你负责社区治理、公开客服协作、举报、置顶和升级处理。回答要强调：人人可协助，但涉及余额、订单、账号、Key、隐私必须转官方；社区不能公开敏感信息；恶意误导、钓鱼、刷屏会被治理。",
		Suggestions: []string{"怎么当客服", "如何举报误导回复", "哪些信息不能公开"},
	},
	"gatewayOperator": {
		Key:         "gatewayOperator",
		Name:        "网关操作员",
		Prompt:      "你是 BizDecipher 零点城的网关操作员。你帮助新用户完成 API Key、OpenAI compatible endpoint、模型测试和 Workbench 试通。回答要短，优先给第一步操作，不把用户带去复杂后台。",
		Suggestions: []string{"怎么开始调用", "API Key 在哪里", "Workbench 怎么测"},
	},
}

func (h *BizDecipherHandler) ChatCommunityMascot(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req communityMascotChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		response.BadRequest(c, "Message is required")
		return
	}
	if len([]rune(message)) > 1200 {
		response.BadRequest(c, "Message is too long")
		return
	}

	duty, ok := communityMascotDuties[strings.TrimSpace(req.MascotKey)]
	if !ok {
		duty = communityMascotDuties["communityAnnouncer"]
	}
	reply := buildCommunityMascotFallbackReply(duty, message)
	response.Success(c, gin.H{
		"mascot_key":      duty.Key,
		"name":            duty.Name,
		"reply":           reply,
		"mode":            "ai-ready-fallback",
		"prompt":          duty.Prompt,
		"suggestions":     duty.Suggestions,
		"moderation_hint": communityMascotModerationHint(),
	})
}

func buildCommunityMascotFallbackReply(duty communityMascotDuty, message string) string {
	text := strings.ToLower(message)
	switch {
	case strings.Contains(message, "余额") || strings.Contains(message, "充值") || strings.Contains(message, "订单") || strings.Contains(text, "payment") || strings.Contains(text, "redeem"):
		return fmt.Sprintf("%s先接住：余额/充值/兑换问题不要公开订单截图。请走反馈通道，附邮箱、订单号、支付方式、时间点和必要截图；社区居民可以帮你整理信息，最终由官方核对。", duty.Name)
	case strings.Contains(message, "409") || strings.Contains(message, "401") || strings.Contains(text, "api") || strings.Contains(text, "base url") || strings.Contains(message, "模型"):
		return fmt.Sprintf("%s先接住：API 问题请补模型名、Base URL、客户端、HTTP 状态码、请求时间、是否重试和余额状态。我会先把它整理成可复现的小崩溃。", duty.Name)
	case strings.Contains(message, "共享") || strings.Contains(message, "池主") || strings.Contains(text, "pool"):
		return fmt.Sprintf("%s先接住：共享池问题先看可用率、延迟、成功率、席位小时费、最低余额和治理状态。想当池主可以去池主区留下容量说明。", duty.Name)
	case strings.Contains(message, "客服") || strings.Contains(message, "治理") || strings.Contains(message, "举报"):
		return fmt.Sprintf("%s先接住：人人可以帮忙回答公开问题，但余额、订单、账号、Key 和隐私必须转官方；误导、钓鱼、刷屏会进入治理流程。", duty.Name)
	default:
		return fmt.Sprintf("%s先接住：你可以把问题发到反馈通道；公开区只放可公开信息，账号、订单、Key 和隐私走官方处理。我会先帮你整理成清楚的一段。", duty.Name)
	}
}

func communityMascotModerationHint() string {
	return "Community helpers may triage public issues, but account, balance, order, API key, and privacy-sensitive matters must be routed to official handling."
}

func (h *BizDecipherHandler) AdminListContributors(c *gin.Context) {
	items, err := h.bizService.AdminListContributors(c.Request.Context(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListOperators(c *gin.Context) {
	items, err := h.bizService.AdminListOperators(c.Request.Context(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListCustomRequests(c *gin.Context) {
	items, err := h.bizService.AdminListCustomRequests(c.Request.Context(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListCreditLedger(c *gin.Context) {
	items, err := h.bizService.AdminListCreditLedger(c.Request.Context(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListSharedPools(c *gin.Context) {
	filter := service.SharedPoolFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		Model:     strings.TrimSpace(c.Query("model")),
		Status:    strings.TrimSpace(c.Query("status")),
		Lifecycle: strings.TrimSpace(c.Query("lifecycle")),
		SortBy:    strings.TrimSpace(c.Query("sort")),
		Limit:     parseLimit(c),
	}
	if minAvail := strings.TrimSpace(c.Query("min_availability")); minAvail != "" {
		if v, err := strconv.ParseFloat(minAvail, 64); err == nil {
			filter.MinAvailability = v
		}
	}
	view, err := h.bizService.AdminListSharedPools(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *BizDecipherHandler) AdminListSharedPoolOwnerEarnings(c *gin.Context) {
	parseOptionalID := func(name string) (int64, bool) {
		raw := strings.TrimSpace(c.Query(name))
		if raw == "" {
			return 0, true
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		return value, err == nil && value > 0
	}
	ownerID, ok := parseOptionalID("owner_id")
	if !ok {
		response.BadRequest(c, "owner_id must be a positive integer")
		return
	}
	poolID, ok := parseOptionalID("pool_id")
	if !ok {
		response.BadRequest(c, "pool_id must be a positive integer")
		return
	}
	beforeID, ok := parseOptionalID("before_id")
	if !ok {
		response.BadRequest(c, "before_id must be a positive integer")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(c.Query("kind")))
	if kind != "" && kind != "earning" && kind != "api" && kind != "seat" && kind != "transfer" && kind != "adjustment" {
		response.BadRequest(c, "kind is invalid")
		return
	}
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" && status != "pending" && status != "available" && status != "settled" && status != "reversed" {
		response.BadRequest(c, "status is invalid")
		return
	}
	page, err := h.bizService.AdminListSharedPoolOwnerEarningsPage(c.Request.Context(), service.AdminSharedPoolOwnerEarningsFilter{
		OwnerID: ownerID, PoolID: poolID, BeforeID: beforeID, Kind: kind, Status: status, Limit: parseLimit(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, page)
}

type sharedPoolGovernanceRequest struct {
	PlatformFeePercent *float64 `json:"platform_fee_percent"`
	FeaturedScore      *float64 `json:"featured_score"`
	RewardScore        *float64 `json:"reward_score"`
	PenaltyScore       *float64 `json:"penalty_score"`
	GovernanceStatus   *string  `json:"governance_status"`
	GovernanceNote     *string  `json:"governance_note"`
	AdminNote          *string  `json:"admin_note"`
	Listed             *bool    `json:"listed"`
	Status             *string  `json:"status"`
}

func (h *BizDecipherHandler) AdminUpdateSharedPoolGovernance(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	var req sharedPoolGovernanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	pool, err := h.bizService.AdminUpdateSharedPoolGovernance(c.Request.Context(), id, subject.UserID, service.SharedPoolGovernanceInput{
		PlatformFeePercent: req.PlatformFeePercent,
		FeaturedScore:      req.FeaturedScore,
		RewardScore:        req.RewardScore,
		PenaltyScore:       req.PenaltyScore,
		GovernanceStatus:   req.GovernanceStatus,
		GovernanceNote:     req.GovernanceNote,
		AdminNote:          req.AdminNote,
		Listed:             req.Listed,
		Status:             req.Status,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

type restoreSharedPoolRequest struct {
	Reason      string `json:"reason"`
	OperationID string `json:"operation_id"`
}

func (h *BizDecipherHandler) AdminRestoreSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	var req restoreSharedPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	if headerID := strings.TrimSpace(c.GetHeader("Idempotency-Key")); headerID != "" {
		if operationID != "" && operationID != headerID {
			response.BadRequest(c, "operation_id and Idempotency-Key must match")
			return
		}
		operationID = headerID
	}
	pool, err := h.bizService.AdminRestoreSharedPool(c.Request.Context(), id, subject.UserID, req.Reason, operationID)
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Shared pool not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

func (h *BizDecipherHandler) AdminRunSharedPoolProbeAggregation(c *gin.Context) {
	summary, err := h.bizService.RunSharedPoolProbeAggregation(c.Request.Context(), time.Now(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *BizDecipherHandler) ListSharedPoolProbeHistories(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	items, err := h.bizService.ListSharedPoolProbeHistories(c.Request.Context(), id, 0, 0, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// GetSharedPoolFullCheckReport returns the latest full-capability detection report
// for a pool in a public, user-friendly format. No authentication required.
func (h *BizDecipherHandler) GetSharedPoolFullCheckReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	item, err := h.bizService.GetLatestSharedPoolFullCheckHistory(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	type FullCheckReportItem struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Category  string `json:"category"`
		Required  bool   `json:"required"`
		Success   bool   `json:"success"`
		LatencyMs int    `json:"latency_ms,omitempty"`
	}

	type FullCheckReport struct {
		PoolID       int64                 `json:"pool_id"`
		ReportID     string                `json:"report_id"`
		CheckedAt    time.Time             `json:"checked_at"`
		Model        string                `json:"model"`
		Score        float64               `json:"score"`
		Passed       int                   `json:"passed"`
		Total        int                   `json:"total"`
		GatePassed   bool                  `json:"gate_passed"`
		ProbeType    string                `json:"probe_type"`
		Checks       []FullCheckReportItem `json:"checks"`
		HasFullCheck bool                  `json:"has_full_check"`
	}

	if item != nil && len(item.Metadata.Checks) > 0 {
		checks := make([]FullCheckReportItem, 0, len(item.Metadata.Checks))
		for _, ch := range item.Metadata.Checks {
			checks = append(checks, FullCheckReportItem{
				ID:        ch.ID,
				Title:     ch.Title,
				Category:  ch.Category,
				Required:  ch.Required,
				Success:   ch.Success,
				LatencyMs: ch.LatencyMs,
			})
		}
		reportID := fmt.Sprintf("%d-%s", item.PoolID, item.CheckedAt.UTC().Format("20060102T150405Z"))
		report := FullCheckReport{
			PoolID:       item.PoolID,
			ReportID:     reportID,
			CheckedAt:    item.CheckedAt,
			Model:        item.ModelName,
			Score:        item.Metadata.FullCheckScore,
			Passed:       item.Metadata.FullCheckPassed,
			Total:        item.Metadata.FullCheckTotal,
			GatePassed:   item.Metadata.GatePassed,
			ProbeType:    item.ProbeType,
			Checks:       checks,
			HasFullCheck: true,
		}
		response.Success(c, report)
		return
	}

	// No full check found yet
	response.Success(c, gin.H{
		"pool_id":        id,
		"has_full_check": false,
		"message":        "This pool has not completed a full capability check yet.",
	})
}

func (h *BizDecipherHandler) ListOwnerSharedPoolProbeHistories(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	accountID := int64(0)
	if raw := strings.TrimSpace(c.Query("account_id")); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid shared pool account id")
			return
		}
		accountID = parsed
	}
	items, err := h.bizService.ListSharedPoolProbeHistories(c.Request.Context(), id, accountID, subject.UserID, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) ListSharedPoolAccountProbeHistories(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid shared pool account id")
		return
	}
	items, err := h.bizService.ListSharedPoolProbeHistories(c.Request.Context(), id, accountID, subject.UserID, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListSharedPoolProbeHistories(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	accountID := int64(0)
	if raw := strings.TrimSpace(c.Query("account_id")); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid shared pool account id")
			return
		}
		accountID = parsed
	}
	items, err := h.bizService.ListSharedPoolProbeHistories(c.Request.Context(), id, accountID, 0, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminListSharedPoolGovernanceLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid shared pool id")
		return
	}
	items, err := h.bizService.AdminListSharedPoolGovernanceLogs(c.Request.Context(), id, parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

type grantCreditRequest struct {
	UserID     int64   `json:"user_id" binding:"required"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Amount     float64 `json:"amount" binding:"required"`
	Note       string  `json:"note"`
}

func (h *BizDecipherHandler) AdminGrantCredit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req grantCreditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Amount == 0 {
		response.BadRequest(c, "Amount must not be zero")
		return
	}
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = "admin_adjustment"
	}
	entry, err := h.bizService.AdminGrantCredit(c.Request.Context(), service.BizCreditGrantInput{
		UserID: req.UserID, SourceType: sourceType, SourceID: strings.TrimSpace(req.SourceID), Amount: req.Amount, Note: strings.TrimSpace(req.Note), CreatedBy: subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entry)
}

// ---------------------------------------------------------------------------
// Promo campaigns
// ---------------------------------------------------------------------------

// GetActivePromo returns the currently-active campaign plus the caller's claim
// state. Auth is optional: an anonymous caller sees the campaign without a claim
// state. This drives the frontend PromoBanner (replaces the hardcoded mock).
func (h *BizDecipherHandler) GetActivePromo(c *gin.Context) {
	var userID int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	view, err := h.bizService.GetActivePromo(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// ClaimPromo credits the active campaign's reward to the authenticated user
// exactly once. Idempotent: a repeat call returns already_claimed=true.
func (h *BizDecipherHandler) ClaimPromo(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	promoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || promoID <= 0 {
		response.BadRequest(c, "Invalid promo id")
		return
	}
	result, err := h.bizService.ClaimPromo(c.Request.Context(), promoID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) AdminListPromos(c *gin.Context) {
	items, err := h.bizService.AdminListPromos(c.Request.Context(), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

type promoCampaignRequest struct {
	Name         string  `json:"name" binding:"required"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Enabled      bool    `json:"enabled"`
	CreditAmount float64 `json:"credit_amount"`
	StartAt      string  `json:"start_at"`
	EndAt        string  `json:"end_at"`
	Target       string  `json:"target"`
	AutoHide     bool    `json:"auto_hide"`
}

func (r promoCampaignRequest) toInput() (service.PromoCampaignInput, error) {
	input := service.PromoCampaignInput{
		Name:         strings.TrimSpace(r.Name),
		Type:         strings.TrimSpace(r.Type),
		Title:        strings.TrimSpace(r.Title),
		Description:  strings.TrimSpace(r.Description),
		Enabled:      r.Enabled,
		CreditAmount: r.CreditAmount,
		Target:       strings.TrimSpace(r.Target),
		AutoHide:     r.AutoHide,
	}
	if input.Type == "" {
		input.Type = "register_bonus"
	}
	if input.Target == "" {
		input.Target = "new_users"
	}
	if r.StartAt != "" {
		t, err := time.Parse(time.RFC3339, r.StartAt)
		if err != nil {
			return input, err
		}
		input.StartAt = t
	}
	if r.EndAt != "" {
		t, err := time.Parse(time.RFC3339, r.EndAt)
		if err != nil {
			return input, err
		}
		input.EndAt = t
	}
	return input, nil
}

func (h *BizDecipherHandler) AdminCreatePromo(c *gin.Context) {
	var req promoCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, err := req.toInput()
	if err != nil {
		response.BadRequest(c, "Invalid time format (expect RFC3339): "+err.Error())
		return
	}
	created, err := h.bizService.AdminCreatePromo(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, created)
}

func (h *BizDecipherHandler) AdminUpdatePromo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid promo id")
		return
	}
	var req promoCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, err := req.toInput()
	if err != nil {
		response.BadRequest(c, "Invalid time format (expect RFC3339): "+err.Error())
		return
	}
	updated, err := h.bizService.AdminUpdatePromo(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

// ---------------------------------------------------------------------------
// Shared pools (Account Square marketplace, read-only)
// ---------------------------------------------------------------------------

func (h *BizDecipherHandler) ListModelCatalog(c *gin.Context) {
	models, err := h.bizService.ListModelCatalog(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"models": models})
}

type modelCapabilityProfileRequest struct {
	ModalitiesIn  []string `json:"modalities_in"`
	ModalitiesOut []string `json:"modalities_out"`
	AdapterKind   string   `json:"adapter_kind"`
	ContextWindow *int     `json:"context_window"`
	Orchestrator  *bool    `json:"orchestrator"`
	RuntimeRole   string   `json:"runtime_role"`
}

// AdminUpdateModelCatalogProfile edits one entry of the model capability matrix.
// Absent fields are left unchanged; unsupported values are rejected instead of
// being stored, so package/runtime capability matching can trust the row.
func (h *BizDecipherHandler) AdminUpdateModelCatalogProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid model catalog id")
		return
	}
	var req modelCapabilityProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	entry, err := h.bizService.UpdateModelCatalogProfile(c.Request.Context(), id, service.ModelCapabilityProfileInput{
		ModalitiesIn:  req.ModalitiesIn,
		ModalitiesOut: req.ModalitiesOut,
		AdapterKind:   req.AdapterKind,
		ContextWindow: req.ContextWindow,
		Orchestrator:  req.Orchestrator,
		RuntimeRole:   req.RuntimeRole,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if entry == nil {
		response.NotFound(c, "Model catalog entry not found")
		return
	}
	response.Success(c, entry)
}

// ListSharedPools returns the marketplace catalog plus aggregate header stats.
// Public read: no auth required to browse.
func (h *BizDecipherHandler) ListSharedPools(c *gin.Context) {
	filter := service.SharedPoolFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Model:   strings.TrimSpace(c.Query("model")),
		Status:  strings.TrimSpace(c.Query("status")),
		View:    strings.TrimSpace(c.Query("view")),
		SortBy:  strings.TrimSpace(c.Query("sort")),
		Limit:   parseLimit(c),
	}
	if minAvail := c.Query("min_availability"); minAvail != "" {
		if v, err := strconv.ParseFloat(minAvail, 64); err == nil {
			filter.MinAvailability = v
		}
	}
	view, err := h.bizService.ListSharedPools(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if view != nil {
		if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
			view.Pools = h.bizService.AttachSharedPoolLikeState(c.Request.Context(), subject.UserID, view.Pools)
		}
	}
	response.Success(c, view)
}

// GetSharedPool returns a single pool by id.
func (h *BizDecipherHandler) GetSharedPool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	pool, err := h.bizService.GetSharedPool(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if pool == nil {
		response.NotFound(c, "Shared pool not found")
		return
	}
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		pool = h.bizService.AttachSingleSharedPoolLikeState(c.Request.Context(), subject.UserID, pool)
	}
	response.Success(c, pool)
}

// ---------------------------------------------------------------------------
// Pool seats (money-sensitive: join / leave / hourly billing / owner payout)
// ---------------------------------------------------------------------------

// JoinSharedPool takes a seat in a shared pool for the authenticated user.
func (h *BizDecipherHandler) JoinSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	result, err := h.bizService.JoinSharedPool(c.Request.Context(), id, subject.UserID)
	if err != nil {
		switch err {
		case service.ErrPoolNotJoinable:
			response.BadRequest(c, "This pool is not currently joinable")
		case service.ErrPoolFull:
			response.BadRequest(c, "This pool is at capacity")
		case service.ErrPoolInsufficientBalance:
			response.BadRequest(c, "Insufficient balance for this pool's admission requirement")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, result)
}

// LeaveSharedPool releases the authenticated user's active seat in a pool.
func (h *BizDecipherHandler) LeaveSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	if err := h.bizService.LeaveSharedPool(c.Request.Context(), id, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"released": true})
}

// ListMySeats returns the authenticated user's pool seats.
type sharedPoolModelConfigRequest struct {
	Provider                  string  `json:"provider"`
	ModelName                 string  `json:"model_name"`
	UpstreamModelName         string  `json:"upstream_model_name"`
	RateMultiplier            float64 `json:"rate_multiplier"`
	FiveHourProtectionPercent float64 `json:"five_hour_protection_percent"`
	SevenDayProtectionPercent float64 `json:"seven_day_protection_percent"`
	DailyProtectionPercent    float64 `json:"daily_protection_percent"`
	MaxConcurrency            int     `json:"max_concurrency"`
	ModelOpen                 *bool   `json:"model_open"`
}

type createSharedPoolRequest struct {
	SupplyMode                  string                         `json:"supply_mode"`
	OperationID                 string                         `json:"operation_id"`
	Name                        string                         `json:"name"`
	Description                 string                         `json:"description"`
	AvatarURL                   string                         `json:"avatar_url"`
	StatusNote                  string                         `json:"status_note"`
	DisabledReason              string                         `json:"disabled_reason"`
	UpstreamBaseURL             string                         `json:"upstream_base_url"`
	UpstreamAPIKey              string                         `json:"upstream_api_key"`
	Models                      []string                       `json:"models"`
	ModelConfigs                []sharedPoolModelConfigRequest `json:"model_configs"`
	RateMultiplier              float64                        `json:"rate_multiplier"`
	MaxUsers                    int                            `json:"max_users"`
	MinBalanceAdmission         float64                        `json:"min_balance_admission"`
	HourlySeatFee               float64                        `json:"hourly_seat_fee"`
	HourlyMinUsageWaiver        float64                        `json:"hourly_min_usage_waiver"`
	ProxyID                     *int64                         `json:"proxy_id"`
	ProxyURL                    string                         `json:"proxy_url"`
	ProxyRegion                 string                         `json:"proxy_region"`
	ProxyStatus                 string                         `json:"proxy_status"`
	AccountConcurrency          int                            `json:"account_concurrency"`
	UserConcurrency             int                            `json:"user_concurrency"`
	AccountModeEnabled          *bool                          `json:"account_mode_enabled"`
	OAuthProvider               string                         `json:"oauth_provider"`
	VerificationMode            string                         `json:"verification_mode"`
	VerificationExemptionReason string                         `json:"verification_exemption_reason"`
	ProbeModel                  string                         `json:"probe_model"`
	Listed                      *bool                          `json:"listed"`
	Status                      string                         `json:"status"`
}

type optionalNullableInt64 struct {
	Value *int64
	Set   bool
}

func (value *optionalNullableInt64) UnmarshalJSON(data []byte) error {
	value.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		value.Value = nil
		return nil
	}
	var parsed int64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	value.Value = &parsed
	return nil
}

type updateSharedPoolRequest struct {
	ExpectedConfigVersion       *int64                          `json:"expected_config_version"`
	Name                        *string                         `json:"name"`
	Description                 *string                         `json:"description"`
	AvatarURL                   *string                         `json:"avatar_url"`
	StatusNote                  *string                         `json:"status_note"`
	DisabledReason              *string                         `json:"disabled_reason"`
	UpstreamBaseURL             *string                         `json:"upstream_base_url"`
	UpstreamAPIKey              *string                         `json:"upstream_api_key"`
	Models                      *[]string                       `json:"models"`
	ModelConfigs                *[]sharedPoolModelConfigRequest `json:"model_configs"`
	RateMultiplier              *float64                        `json:"rate_multiplier"`
	SyncModelRates              *bool                           `json:"sync_model_rates"`
	MaxUsers                    *int                            `json:"max_users"`
	MinBalanceAdmission         *float64                        `json:"min_balance_admission"`
	HourlySeatFee               *float64                        `json:"hourly_seat_fee"`
	HourlyMinUsageWaiver        *float64                        `json:"hourly_min_usage_waiver"`
	ProxyID                     optionalNullableInt64           `json:"proxy_id"`
	ProxyURL                    *string                         `json:"proxy_url"`
	ProxyRegion                 *string                         `json:"proxy_region"`
	ProxyStatus                 *string                         `json:"proxy_status"`
	AccountConcurrency          *int                            `json:"account_concurrency"`
	UserConcurrency             *int                            `json:"user_concurrency"`
	AccountModeEnabled          *bool                           `json:"account_mode_enabled"`
	OAuthProvider               *string                         `json:"oauth_provider"`
	VerificationMode            *string                         `json:"verification_mode"`
	VerificationExemptionReason *string                         `json:"verification_exemption_reason"`
	ProbeModel                  *string                         `json:"probe_model"`
	Listed                      *bool                           `json:"listed"`
	Status                      *string                         `json:"status"`
}

type importSharedPoolsRequest struct {
	Items []createSharedPoolRequest `json:"items"`
}

type sharedPoolAccountRequest struct {
	OperationID           string                         `json:"operation_id"`
	RepairOperationID     string                         `json:"repair_operation_id"`
	ExpectedConfigVersion int64                          `json:"expected_config_version"`
	Name                  string                         `json:"name"`
	Description           string                         `json:"description"`
	Provider              string                         `json:"provider"`
	AuthType              string                         `json:"auth_type"`
	UpstreamBaseURL       string                         `json:"upstream_base_url"`
	UpstreamAPIKey        string                         `json:"upstream_api_key"`
	Credentials           map[string]any                 `json:"credentials"`
	ExpiresAt             *time.Time                     `json:"expires_at"`
	AutoPauseOnExpired    *bool                          `json:"auto_pause_on_expired"`
	Schedulable           *bool                          `json:"schedulable"`
	Status                string                         `json:"status"`
	StatusNote            string                         `json:"status_note"`
	DisabledReason        string                         `json:"disabled_reason"`
	GroupName             string                         `json:"group_name"`
	ProxyID               *int64                         `json:"proxy_id"`
	ProxyURL              string                         `json:"proxy_url"`
	ProxyRegion           string                         `json:"proxy_region"`
	ProxyStatus           string                         `json:"proxy_status"`
	AccountWeight         float64                        `json:"account_weight"`
	Priority              int                            `json:"priority"`
	RPMLimit              int                            `json:"rpm_limit"`
	AccountConcurrency    int                            `json:"account_concurrency"`
	UserConcurrency       int                            `json:"user_concurrency"`
	TLSProfileID          *int64                         `json:"tls_profile_id"`
	TTLSeconds            int                            `json:"ttl_seconds"`
	CachePolicy           map[string]any                 `json:"cache_policy"`
	RoutingPolicy         map[string]any                 `json:"routing_policy"`
	ModelConfigs          []sharedPoolModelConfigRequest `json:"model_configs"`
	GateRequired          *bool                          `json:"gate_required"`
	GatePassed            *bool                          `json:"gate_passed"`
	FullCheckScore        float64                        `json:"full_check_score"`
	FullCheckPassed       int                            `json:"full_check_passed"`
	FullCheckTotal        int                            `json:"full_check_total"`
	presentFields         map[string]json.RawMessage
}

type importSharedPoolAccountsRequest struct {
	Items []sharedPoolAccountRequest `json:"items"`
}

type importSharedPoolOAuthPackageRequest struct {
	Data           service.SharedPoolOAuthDataPackage `json:"data"`
	UpdateExisting *bool                              `json:"update_existing"`
}

type probeSharedPoolUpstreamRequest struct {
	PoolID          int64  `json:"pool_id"`
	AccountID       int64  `json:"account_id"`
	UpstreamBaseURL string `json:"upstream_base_url"`
	UpstreamAPIKey  string `json:"upstream_api_key"`
	ProbeModel      string `json:"probe_model"`
	ProbeType       string `json:"probe_type"`
	ProxyURL        string `json:"proxy_url"`
	OperationID     string `json:"operation_id"`
}

type fetchSharedPoolUpstreamModelsRequest struct {
	PoolID          int64  `json:"pool_id"`
	AccountID       int64  `json:"account_id"`
	UpstreamBaseURL string `json:"upstream_base_url"`
	UpstreamAPIKey  string `json:"upstream_api_key"`
	ProxyURL        string `json:"proxy_url"`
}

type createSharedPoolAccessKeyRequest struct {
	Name string `json:"name"`
}

type reportSharedPoolRequest struct {
	Reason string `json:"reason"`
}

func sharedPoolModelConfigInputs(reqs []sharedPoolModelConfigRequest) []service.SharedPoolModelInput {
	out := make([]service.SharedPoolModelInput, 0, len(reqs))
	for _, req := range reqs {
		modelOpen := true
		if req.ModelOpen != nil {
			modelOpen = *req.ModelOpen
		}
		out = append(out, service.SharedPoolModelInput{
			Provider:                  req.Provider,
			ModelName:                 req.ModelName,
			UpstreamModelName:         req.UpstreamModelName,
			RateMultiplier:            req.RateMultiplier,
			FiveHourProtectionPercent: req.FiveHourProtectionPercent,
			SevenDayProtectionPercent: req.SevenDayProtectionPercent,
			DailyProtectionPercent:    req.DailyProtectionPercent,
			MaxConcurrency:            req.MaxConcurrency,
			ModelOpen:                 modelOpen,
		})
	}
	return out
}

func accountModeEnabledFromRequest(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

func createSharedPoolInputFromRequest(ownerID int64, req createSharedPoolRequest) service.CreateSharedPoolInput {
	listed := true
	if req.Listed != nil {
		listed = *req.Listed
	}
	return service.CreateSharedPoolInput{
		OwnerID:                     ownerID,
		Name:                        req.Name,
		Description:                 req.Description,
		AvatarURL:                   req.AvatarURL,
		StatusNote:                  req.StatusNote,
		DisabledReason:              req.DisabledReason,
		UpstreamBaseURL:             req.UpstreamBaseURL,
		UpstreamAPIKey:              req.UpstreamAPIKey,
		Models:                      req.Models,
		ModelConfigs:                sharedPoolModelConfigInputs(req.ModelConfigs),
		RateMultiplier:              req.RateMultiplier,
		MaxUsers:                    req.MaxUsers,
		MinBalanceAdmission:         req.MinBalanceAdmission,
		HourlySeatFee:               req.HourlySeatFee,
		HourlyMinUsageWaiver:        req.HourlyMinUsageWaiver,
		ProxyID:                     req.ProxyID,
		ProxyURL:                    req.ProxyURL,
		ProxyRegion:                 req.ProxyRegion,
		ProxyStatus:                 req.ProxyStatus,
		AccountConcurrency:          req.AccountConcurrency,
		UserConcurrency:             req.UserConcurrency,
		AccountModeEnabled:          accountModeEnabledFromRequest(req.AccountModeEnabled),
		OAuthProvider:               req.OAuthProvider,
		VerificationMode:            req.VerificationMode,
		VerificationExemptionReason: req.VerificationExemptionReason,
		ProbeModel:                  req.ProbeModel,
		Listed:                      listed,
		Status:                      req.Status,
	}
}

func updateSharedPoolInputFromRequest(req updateSharedPoolRequest) service.UpdateSharedPoolInput {
	input := service.UpdateSharedPoolInput{}
	if req.ExpectedConfigVersion != nil {
		input.ExpectedConfigVersion = *req.ExpectedConfigVersion
	}
	if req.Name != nil {
		input.Name, input.NameSet = *req.Name, true
	}
	if req.Description != nil {
		input.Description, input.DescriptionSet = *req.Description, true
	}
	if req.AvatarURL != nil {
		input.AvatarURL, input.AvatarURLSet = *req.AvatarURL, true
	}
	if req.StatusNote != nil {
		input.StatusNote, input.StatusNoteSet = *req.StatusNote, true
	}
	if req.DisabledReason != nil {
		input.DisabledReason, input.DisabledReasonSet = *req.DisabledReason, true
	}
	if req.UpstreamBaseURL != nil {
		input.UpstreamBaseURL, input.UpstreamBaseURLSet = *req.UpstreamBaseURL, true
	}
	if req.UpstreamAPIKey != nil {
		input.UpstreamAPIKey, input.UpstreamAPIKeySet = *req.UpstreamAPIKey, true
	}
	if req.Models != nil {
		input.Models, input.ModelsSet = append([]string{}, (*req.Models)...), true
	}
	if req.ModelConfigs != nil {
		input.ModelConfigs, input.ModelConfigsSet = sharedPoolModelConfigInputs(*req.ModelConfigs), true
	}
	if req.RateMultiplier != nil {
		input.RateMultiplier, input.RateMultiplierSet = *req.RateMultiplier, true
	}
	input.SyncModelRates = req.SyncModelRates != nil && *req.SyncModelRates
	if req.MaxUsers != nil {
		input.MaxUsers, input.MaxUsersSet = *req.MaxUsers, true
	}
	if req.MinBalanceAdmission != nil {
		input.MinBalanceAdmission, input.MinBalanceAdmissionSet = *req.MinBalanceAdmission, true
	}
	if req.HourlySeatFee != nil {
		input.HourlySeatFee, input.HourlySeatFeeSet = *req.HourlySeatFee, true
	}
	if req.HourlyMinUsageWaiver != nil {
		input.HourlyMinUsageWaiver, input.HourlyMinUsageWaiverSet = *req.HourlyMinUsageWaiver, true
	}
	if req.ProxyID.Set {
		input.ProxyID, input.ProxyIDSet = req.ProxyID.Value, true
	}
	if req.ProxyURL != nil {
		input.ProxyURL, input.ProxyURLSet = *req.ProxyURL, true
	}
	if req.ProxyRegion != nil {
		input.ProxyRegion, input.ProxyRegionSet = *req.ProxyRegion, true
	}
	if req.ProxyStatus != nil {
		input.ProxyStatus, input.ProxyStatusSet = *req.ProxyStatus, true
	}
	if req.AccountConcurrency != nil {
		input.AccountConcurrency, input.AccountConcurrencySet = *req.AccountConcurrency, true
	}
	if req.UserConcurrency != nil {
		input.UserConcurrency, input.UserConcurrencySet = *req.UserConcurrency, true
	}
	if req.AccountModeEnabled != nil {
		input.AccountModeEnabled, input.AccountModeSet = *req.AccountModeEnabled, true
	}
	if req.OAuthProvider != nil {
		input.OAuthProvider, input.OAuthProviderSet = *req.OAuthProvider, true
	}
	if req.VerificationMode != nil {
		input.VerificationMode, input.VerificationModeSet = *req.VerificationMode, true
	}
	if req.VerificationExemptionReason != nil {
		input.VerificationExemptionReason, input.VerificationReasonSet = *req.VerificationExemptionReason, true
	}
	if req.ProbeModel != nil {
		input.ProbeModel, input.ProbeModelSet = *req.ProbeModel, true
	}
	if req.Listed != nil {
		input.Listed, input.ListedSet = *req.Listed, true
	}
	if req.Status != nil {
		input.Status, input.StatusSet = *req.Status, true
	}
	return input
}

func sharedPoolAccountInputFromRequest(poolID, ownerID int64, req sharedPoolAccountRequest) service.SharedPoolAccountInput {
	gateRequired := true
	if req.GateRequired != nil {
		gateRequired = *req.GateRequired
	}
	gatePassed := false
	if req.GatePassed != nil {
		gatePassed = *req.GatePassed
	}
	autoPauseOnExpired := true
	if req.AutoPauseOnExpired != nil {
		autoPauseOnExpired = *req.AutoPauseOnExpired
	}
	schedulable := true
	if req.Schedulable != nil {
		schedulable = *req.Schedulable
	}
	cachePolicy, _ := json.Marshal(req.CachePolicy)
	if req.CachePolicy == nil {
		cachePolicy = []byte(`{}`)
	}
	routingPolicy, _ := json.Marshal(req.RoutingPolicy)
	if req.RoutingPolicy == nil {
		routingPolicy = []byte(`{}`)
	}
	return service.SharedPoolAccountInput{
		PoolID:                poolID,
		OwnerID:               ownerID,
		Name:                  req.Name,
		Description:           req.Description,
		Provider:              req.Provider,
		AuthType:              req.AuthType,
		UpstreamBaseURL:       req.UpstreamBaseURL,
		UpstreamAPIKey:        req.UpstreamAPIKey,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    autoPauseOnExpired,
		AutoPauseOnExpiredSet: req.AutoPauseOnExpired != nil,
		Schedulable:           schedulable,
		SchedulableSet:        req.Schedulable != nil,
		Status:                req.Status,
		StatusNote:            req.StatusNote,
		DisabledReason:        req.DisabledReason,
		GroupName:             req.GroupName,
		ProxyID:               req.ProxyID,
		ProxyURL:              req.ProxyURL,
		ProxyRegion:           req.ProxyRegion,
		ProxyStatus:           req.ProxyStatus,
		AccountWeight:         req.AccountWeight,
		Priority:              req.Priority,
		RPMLimit:              req.RPMLimit,
		AccountConcurrency:    req.AccountConcurrency,
		UserConcurrency:       req.UserConcurrency,
		TLSProfileID:          req.TLSProfileID,
		TTLSeconds:            req.TTLSeconds,
		CachePolicy:           cachePolicy,
		RoutingPolicy:         routingPolicy,
		ModelConfigs:          sharedPoolModelConfigInputs(req.ModelConfigs),
		GateRequired:          gateRequired,
		GatePassed:            gatePassed,
		FullCheckScore:        req.FullCheckScore,
		FullCheckPassed:       req.FullCheckPassed,
		FullCheckTotal:        req.FullCheckTotal,
	}
}

func (h *BizDecipherHandler) ProbeSharedPoolUpstream(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req probeSharedPoolUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	if headerID := strings.TrimSpace(c.GetHeader("Idempotency-Key")); headerID != "" {
		if operationID != "" && operationID != headerID {
			response.BadRequest(c, "operation_id and Idempotency-Key must match")
			return
		}
		operationID = headerID
	}
	job, err := h.bizService.EnqueueSharedPoolProbeJob(c.Request.Context(), service.SharedPoolUpstreamProbeInput{
		PoolID:          req.PoolID,
		AccountID:       req.AccountID,
		OwnerID:         subject.UserID,
		UpstreamBaseURL: req.UpstreamBaseURL,
		UpstreamAPIKey:  req.UpstreamAPIKey,
		ProbeModel:      req.ProbeModel,
		ProbeType:       req.ProbeType,
		ProxyURL:        req.ProxyURL,
	}, operationID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(202, response.Response{
		Code:    0,
		Message: "accepted",
		Data:    job,
	})
}

func (h *BizDecipherHandler) FetchSharedPoolUpstreamModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req fetchSharedPoolUpstreamModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.bizService.FetchSharedPoolUpstreamModels(c.Request.Context(), service.SharedPoolUpstreamModelsInput{
		PoolID:          req.PoolID,
		AccountID:       req.AccountID,
		OwnerID:         subject.UserID,
		UpstreamBaseURL: req.UpstreamBaseURL,
		UpstreamAPIKey:  req.UpstreamAPIKey,
		ProxyURL:        req.ProxyURL,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) ImportSharedPools(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req importSharedPoolsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	items := make([]service.CreateSharedPoolInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, createSharedPoolInputFromRequest(subject.UserID, item))
	}
	result, err := h.bizService.ImportSharedPools(c.Request.Context(), service.ImportSharedPoolsInput{OwnerID: subject.UserID, Items: items})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) CreateSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createSharedPoolRequest
	fields, err := bindSharedPoolOwnerJSON(c, &req)
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.EqualFold(strings.TrimSpace(req.SupplyMode), "native") {
		if err := requireNativeOwnerFields(fields, nativeDraftFields); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		operationID, err := reconcileNativeOperationID(c, req.OperationID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		input, err := service.NewSharedPoolNativeDraftInput(subject.UserID, req.Name, req.Description, operationID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		pool, err := h.bizService.CreateSharedPoolNativeDraft(c.Request.Context(), input)
		if err != nil {
			writeSharedPoolNativeError(c, err)
			return
		}
		response.Success(c, pool)
		return
	}
	if strings.TrimSpace(req.SupplyMode) != "" || strings.TrimSpace(req.OperationID) != "" {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	pool, err := h.bizService.CreateSharedPool(c.Request.Context(), createSharedPoolInputFromRequest(subject.UserID, req))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, pool)
}

func (h *BizDecipherHandler) ListMySharedPools(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	pools, err := h.bizService.ListMySharedPools(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"pools": pools})
}

func (h *BizDecipherHandler) ListSharedPoolAccounts(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	accounts, err := h.bizService.ListSharedPoolAccounts(c.Request.Context(), id, subject.UserID)
	if err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can view accounts")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"accounts": redactSharedPoolNativeAccounts(accounts)})
}

func (h *BizDecipherHandler) CreateSharedPoolAccount(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req sharedPoolAccountRequest
	fields, err := bindSharedPoolOwnerJSON(c, &req)
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if isNativeSharedPoolAccountRequest(c, req, fields) {
		account, err := h.onboardSharedPoolNativeAccount(c, id, subject.UserID, req, fields)
		if err != nil {
			writeSharedPoolNativeError(c, err)
			return
		}
		response.Success(c, newSharedPoolNativeAccountResponse(account))
		return
	}
	account, err := h.bizService.CreateSharedPoolAccount(c.Request.Context(), sharedPoolAccountInputFromRequest(id, subject.UserID, req))
	if err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can create accounts")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, account)
}

func (h *BizDecipherHandler) ImportSharedPoolAccounts(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req importSharedPoolAccountsRequest
	fields, err := bindSharedPoolOwnerJSON(c, &req)
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if hasNativeSharedPoolAccountItems(req.Items) {
		if err := requireNativeOwnerFields(fields, nativeAccountImportFields); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		result, err := h.importSharedPoolNativeAccounts(c, id, subject.UserID, req.Items)
		if err != nil {
			writeSharedPoolNativeError(c, err)
			return
		}
		response.Success(c, result)
		return
	}
	items := make([]service.SharedPoolAccountInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, sharedPoolAccountInputFromRequest(id, subject.UserID, item))
	}
	result, err := h.bizService.ImportSharedPoolAccounts(c.Request.Context(), service.ImportSharedPoolAccountsInput{PoolID: id, OwnerID: subject.UserID, Items: items})
	if err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can import accounts")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) ImportSharedPoolOAuthPackage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req importSharedPoolOAuthPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updateExisting := true
	if req.UpdateExisting != nil {
		updateExisting = *req.UpdateExisting
	}
	result, err := h.bizService.ImportSharedPoolOAuthPackage(c.Request.Context(), service.ImportSharedPoolOAuthPackageInput{
		PoolID: id, OwnerID: subject.UserID, Data: req.Data, UpdateExisting: updateExisting,
	})
	if err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can import accounts")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) UpdateSharedPoolAccount(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account id")
		return
	}
	var req sharedPoolAccountRequest
	fields, err := bindSharedPoolOwnerJSON(c, &req)
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if _, ok := fields["repair_operation_id"]; ok {
		account, err := h.repairSharedPoolNativeAccount(c, id, accountID, subject.UserID, req, fields)
		if err != nil {
			writeSharedPoolNativeError(c, err)
			return
		}
		response.Success(c, newSharedPoolNativeAccountResponse(account))
		return
	}
	if isNativeSharedPoolAccountRequest(c, req, fields) {
		response.ErrorFrom(c, service.ErrOwnerNativeFieldRejected)
		return
	}
	account, err := h.bizService.UpdateSharedPoolAccount(c.Request.Context(), accountID, sharedPoolAccountInputFromRequest(id, subject.UserID, req))
	if err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can update accounts")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, account)
}

func (h *BizDecipherHandler) DeleteSharedPoolAccount(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account id")
		return
	}
	if err := h.bizService.DeleteSharedPoolAccount(c.Request.Context(), id, accountID, subject.UserID); err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can delete accounts")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"archived": true, "data_preserved": true})
}

func (h *BizDecipherHandler) UpdateSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req updateSharedPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	// Older owner pages did not include the optimistic-lock version. The
	// service resolves a missing version against the current row, while a
	// supplied version is still checked for concurrent edits.
	if req.ExpectedConfigVersion != nil && *req.ExpectedConfigVersion <= 0 {
		response.BadRequest(c, "expected_config_version must be positive when provided")
		return
	}
	pool, err := h.bizService.UpdateSharedPool(c.Request.Context(), id, subject.UserID, updateSharedPoolInputFromRequest(req))
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Shared pool not found")
			return
		}
		if errors.Is(err, service.ErrSharedPoolConcurrentUpdate) ||
			errors.Is(err, service.ErrSharedPoolGovernanceBlocked) ||
			errors.Is(err, service.ErrSharedPoolProbeRequired) {
			response.ErrorFrom(c, err)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, pool)
}

func (h *BizDecipherHandler) DeleteSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	if err := h.bizService.DeleteSharedPool(c.Request.Context(), id, subject.UserID); err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Shared pool not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *BizDecipherHandler) CreateSharedPoolAccessKey(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req createSharedPoolAccessKeyRequest
	_ = c.ShouldBindJSON(&req)
	result, err := h.bizService.CreateSharedPoolAccessKey(c.Request.Context(), id, subject.UserID, req.Name)
	if err != nil {
		switch err {
		case service.ErrPoolNotJoinable:
			response.BadRequest(c, "This pool is not currently joinable")
		case service.ErrPoolFull:
			response.BadRequest(c, "This pool is at capacity")
		case service.ErrPoolInsufficientBalance:
			response.BadRequest(c, "Insufficient balance for this pool's admission requirement")
		case service.ErrPoolSeatRequired:
			response.BadRequest(c, "Join this shared pool before creating an access key")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, result)
}

func (h *BizDecipherHandler) ReportSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	var req reportSharedPoolRequest
	_ = c.ShouldBindJSON(&req)
	pool, err := h.bizService.ReportSharedPool(c.Request.Context(), id, subject.UserID, req.Reason)
	if err != nil {
		switch err {
		case service.ErrPoolNotJoinable:
			response.BadRequest(c, "This pool is not currently joinable")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, pool)
}

func (h *BizDecipherHandler) LikeSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	pool, err := h.bizService.LikeSharedPool(c.Request.Context(), id, subject.UserID)
	if err != nil {
		switch err {
		case service.ErrPoolNotJoinable:
			response.BadRequest(c, "This pool is not currently available")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, pool)
}

func (h *BizDecipherHandler) UnlikeSharedPool(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	pool, err := h.bizService.UnlikeSharedPool(c.Request.Context(), id, subject.UserID)
	if err != nil {
		switch err {
		case service.ErrPoolNotJoinable:
			response.BadRequest(c, "This pool is not currently available")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, pool)
}

func (h *BizDecipherHandler) ListMySharedPoolAccessKeys(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keys, err := h.bizService.ListMySharedPoolAccessKeys(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"keys": keys})
}

func (h *BizDecipherHandler) DeleteSharedPoolAccessKey(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	apiKeyID, err := strconv.ParseInt(c.Param("apiKeyId"), 10, 64)
	if err != nil || apiKeyID <= 0 {
		response.BadRequest(c, "Invalid shared pool key id")
		return
	}
	if err := h.bizService.DeleteSharedPoolAccessKey(c.Request.Context(), apiKeyID, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *BizDecipherHandler) ListMySeats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	seats, err := h.bizService.ListMySeats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"seats": seats})
}

func (h *BizDecipherHandler) ListSharedPoolMembers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	status := strings.ToLower(strings.TrimSpace(c.DefaultQuery("status", "active")))
	if status != "active" && status != "released" && status != "all" {
		response.BadRequest(c, "status must be active, released, or all")
		return
	}
	members, err := h.bizService.ListSharedPoolMembers(c.Request.Context(), id, subject.UserID, status)
	if err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can view members")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"members": members})
}

func (h *BizDecipherHandler) RemoveSharedPoolMember(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid pool id")
		return
	}
	seatID, err := strconv.ParseInt(c.Param("seatId"), 10, 64)
	if err != nil || seatID <= 0 {
		response.BadRequest(c, "Invalid seat id")
		return
	}
	if err := h.bizService.RemoveSharedPoolMember(c.Request.Context(), id, seatID, subject.UserID); err != nil {
		if err == service.ErrPoolForbidden {
			response.Forbidden(c, "Only the pool owner can remove members")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"removed": true})
}

func parseLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		return 50
	}
	return limit
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// SetPoolCardSkin PATCH /biz/pools/:id/card-skin
func (h *BizDecipherHandler) SetPoolCardSkin(c *gin.Context) {
	poolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid pool id"})
		return
	}
	sub, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	var body struct {
		CardKey    string `json:"card_key"`
		CardRarity string `json:"card_rarity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid body"})
		return
	}
	body.CardKey = strings.TrimSpace(body.CardKey)
	body.CardRarity = strings.ToLower(strings.TrimSpace(body.CardRarity))
	if body.CardKey == "" {
		c.JSON(400, gin.H{"error": "card_key required"})
		return
	}
	if err := h.bizService.SetPoolCardSkin(c.Request.Context(), poolID, sub.UserID, body.CardKey, body.CardRarity); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ClearPoolCardSkin DELETE /biz/pools/:id/card-skin
func (h *BizDecipherHandler) ClearPoolCardSkin(c *gin.Context) {
	poolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid pool id"})
		return
	}
	sub, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.bizService.ClearPoolCardSkin(c.Request.Context(), poolID, sub.UserID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
