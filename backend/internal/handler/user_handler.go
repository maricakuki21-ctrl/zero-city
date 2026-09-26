package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/handler/quotaview"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userService           *service.UserService
	authService           *service.AuthService
	emailService          *service.EmailService
	emailCache            service.EmailCache
	affiliateService      *service.AffiliateService
	checkinService        *service.CheckinService
	userPlatformQuotaRepo service.UserPlatformQuotaRepository
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(
	userService *service.UserService,
	authService *service.AuthService,
	emailService *service.EmailService,
	emailCache service.EmailCache,
	affiliateService *service.AffiliateService,
	checkinService *service.CheckinService,
	userPlatformQuotaRepo service.UserPlatformQuotaRepository,
) *UserHandler {
	return &UserHandler{
		userService:           userService,
		authService:           authService,
		emailService:          emailService,
		emailCache:            emailCache,
		affiliateService:      affiliateService,
		checkinService:        checkinService,
		userPlatformQuotaRepo: userPlatformQuotaRepo,
	}
}

// GetMyPlatformQuotas GET /user/platform-quotas
// 返回当前 JWT 用户的 platform quota 状态。
// D14: 对每条记录逐档判断窗口过期，过期档位 usage=0、window_resets_at=null（不写 DB）
func (h *UserHandler) GetMyPlatformQuotas(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.userPlatformQuotaRepo == nil {
		response.Success(c, map[string]any{"platform_quotas": []any{}})
		return
	}
	records, err := h.userPlatformQuotaRepo.ListByUser(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	now := time.Now().UTC()
	out := make([]map[string]any, 0, len(records))
	for _, r := range records {
		out = append(out, quotaview.LazyZeroQuotaForResponse(r, now, false))
	}
	response.Success(c, map[string]any{"platform_quotas": out})
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UpdateProfileRequest represents the update profile request payload
type UpdateProfileRequest struct {
	Username               *string  `json:"username"`
	AvatarURL              *string  `json:"avatar_url"`
	BalanceNotifyEnabled   *bool    `json:"balance_notify_enabled"`
	BalanceNotifyThreshold *float64 `json:"balance_notify_threshold"`
}

type userProfileResponse struct {
	dto.User
	AvatarURL         string                                 `json:"avatar_url,omitempty"`
	AvatarSource      *userProfileSourceContext              `json:"avatar_source,omitempty"`
	UsernameSource    *userProfileSourceContext              `json:"username_source,omitempty"`
	DisplayNameSource *userProfileSourceContext              `json:"display_name_source,omitempty"`
	NicknameSource    *userProfileSourceContext              `json:"nickname_source,omitempty"`
	ProfileSources    map[string]*userProfileSourceContext   `json:"profile_sources,omitempty"`
	Identities        service.UserIdentitySummarySet         `json:"identities"`
	AuthBindings      map[string]service.UserIdentitySummary `json:"auth_bindings"`
	IdentityBindings  map[string]service.UserIdentitySummary `json:"identity_bindings"`
	EmailBound        bool                                   `json:"email_bound"`
	LinuxDoBound      bool                                   `json:"linuxdo_bound"`
	OIDCBound         bool                                   `json:"oidc_bound"`
	WeChatBound       bool                                   `json:"wechat_bound"`
	DingTalkBound     bool                                   `json:"dingtalk_bound"`
}

type userProfileSourceContext struct {
	Provider string `json:"provider,omitempty"`
	Source   string `json:"source,omitempty"`
}

// GetProfile handles getting user profile
// GET /api/v1/users/me
func (h *UserHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userData, err := h.userService.GetProfile(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, userData)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

// ChangePassword handles changing user password
// POST /api/v1/users/me/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.ChangePasswordRequest{
		CurrentPassword: req.OldPassword,
		NewPassword:     req.NewPassword,
	}
	err := h.userService.ChangePassword(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Password changed successfully"})
}

// UpdateProfile handles updating user profile
// PUT /api/v1/users/me
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.UpdateProfileRequest{
		Username:               req.Username,
		AvatarURL:              req.AvatarURL,
		BalanceNotifyEnabled:   req.BalanceNotifyEnabled,
		BalanceNotifyThreshold: req.BalanceNotifyThreshold,
	}
	updatedUser, err := h.userService.UpdateProfile(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, updatedUser)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

// GetCheckinStatus returns today's checkin status for the current user.
// GET /api/v1/user/checkin/status
func (h *UserHandler) GetCheckinStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.checkinService.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// GetCheckinRecords returns recent daily fortune draw records for the current user.
// GET /api/v1/user/checkin/records
func (h *UserHandler) GetCheckinRecords(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	records, err := h.checkinService.ListRecords(c.Request.Context(), subject.UserID, 20)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, records)
}

// GetCheckinCards returns collectible daily fortune cards for the current user.
// GET /api/v1/user/checkin/cards
func (h *UserHandler) GetCheckinCards(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	cards, err := h.checkinService.ListCollectibleCards(c.Request.Context(), subject.UserID, 60)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cards)
}

// GetCheckinCardsPage returns every collectible card through explicit
// pagination so the album is not silently truncated to the newest 60 assets.
// GET /api/v1/user/checkin/cards/page
func (h *UserHandler) GetCheckinCardsPage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		response.BadRequest(c, "page must be a positive integer")
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	if err != nil || pageSize <= 0 || pageSize > 100 {
		response.BadRequest(c, "page_size must be between 1 and 100")
		return
	}
	result, err := h.checkinService.ListCollectibleCardsPage(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type ClaimCheckinRequest struct {
	Type        string `json:"type" binding:"required"`
	OperationID string `json:"operation_id"`
}

type BuyCheckinCardRequest struct {
	Rarity string `json:"rarity" binding:"required"`
}

type ClaimCheckinMilestoneRequest struct {
	Days int `json:"days" binding:"required"`
}

// ClaimCheckinMilestone claims a 3/5/7 day milestone mini gift inside the daily fortune box.
// POST /api/v1/user/checkin/milestone/claim
func (h *UserHandler) ClaimCheckinMilestone(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req ClaimCheckinMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.checkinService.ClaimMilestone(c.Request.Context(), subject.UserID, req.Days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ClaimCheckin claims today's free or paid checkin blind box.
// POST /api/v1/user/checkin/claim
func (h *UserHandler) ClaimCheckin(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req ClaimCheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	headerOperationID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if operationID != "" && headerOperationID != "" && operationID != headerOperationID {
		response.BadRequest(c, "operation_id and Idempotency-Key must match")
		return
	}
	if operationID == "" {
		operationID = headerOperationID
	}
	result, err := h.checkinService.ClaimWithOperation(c.Request.Context(), subject.UserID, strings.TrimSpace(req.Type), operationID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetCheckinOperation returns a settled free or balance draw by operation ID.
// GET /api/v1/user/checkin/operations/:operation_id
func (h *UserHandler) GetCheckinOperation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.checkinService.GetOperationResult(c.Request.Context(), subject.UserID, strings.TrimSpace(c.Param("operation_id")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// BuyCheckinCard buys one point-shop collectible card from common/good/rare tiers.
// POST /api/v1/user/checkin/cards/buy
func (h *UserHandler) BuyCheckinCard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req BuyCheckinCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.checkinService.BuyCollectibleCard(c.Request.Context(), subject.UserID, strings.TrimSpace(req.Rarity))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetAffiliate returns the current user's affiliate details.
// GET /api/v1/user/aff
func (h *UserHandler) GetAffiliate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	detail, err := h.affiliateService.GetAffiliateDetail(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// TransferAffiliateQuota transfers all available affiliate rebate quota into current credit balance.
// POST /api/v1/user/aff/transfer
func (h *UserHandler) TransferAffiliateQuota(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	transferred, creditBalance, err := h.affiliateService.TransferAffiliateQuota(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"transferred_quota": transferred,
		"credit_balance":    creditBalance,
		"balance":           creditBalance,
	})
}

type StartIdentityBindingRequest struct {
	Provider   string `json:"provider" binding:"required"`
	RedirectTo string `json:"redirect_to"`
}

type BindEmailIdentityRequest struct {
	Email      string `json:"email" binding:"required,email"`
	VerifyCode string `json:"verify_code" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type SendEmailBindingCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// StartIdentityBinding returns the backend authorize URL for starting a third-party identity bind flow.
// POST /api/v1/user/auth-identities/bind/start
func (h *UserHandler) StartIdentityBinding(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req StartIdentityBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.userService.PrepareIdentityBindingStart(c.Request.Context(), service.StartUserIdentityBindingRequest{
		Provider:   req.Provider,
		RedirectTo: req.RedirectTo,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// BindEmailIdentity verifies and binds a local email identity for the current user.
// POST /api/v1/user/account-bindings/email
func (h *UserHandler) BindEmailIdentity(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.authService == nil {
		response.InternalError(c, "Auth service not configured")
		return
	}

	var req BindEmailIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updatedUser, err := h.authService.BindEmailIdentity(
		c.Request.Context(),
		subject.UserID,
		req.Email,
		req.VerifyCode,
		req.Password,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, updatedUser)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

// UnbindIdentity removes a third-party sign-in provider from the current user.
// DELETE /api/v1/user/account-bindings/:provider
func (h *UserHandler) UnbindIdentity(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	updatedUser, unbound, err := h.userService.UnbindUserAuthProviderWithResult(
		c.Request.Context(),
		subject.UserID,
		c.Param("provider"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if unbound && h.authService != nil {
		if err := h.authService.RevokeAllUserTokens(c.Request.Context(), subject.UserID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, updatedUser)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

// SendEmailBindingCode sends a verification code for the current user's email binding flow.
// POST /api/v1/user/account-bindings/email/send-code
func (h *UserHandler) SendEmailBindingCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.authService == nil {
		response.InternalError(c, "Auth service not configured")
		return
	}

	var req SendEmailBindingCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.authService.SendEmailIdentityBindCode(c.Request.Context(), subject.UserID, req.Email, c.GetHeader("Accept-Language")); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Verification code sent successfully"})
}

// SendNotifyEmailCodeRequest represents the request to send notify email verification code
type SendNotifyEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// SendNotifyEmailCode sends verification code to extra notification email
// POST /api/v1/user/notify-email/send-code
func (h *UserHandler) SendNotifyEmailCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req SendNotifyEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.SendNotifyEmailCode(c.Request.Context(), subject.UserID, req.Email, h.emailService, h.emailCache, c.GetHeader("Accept-Language"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Verification code sent successfully"})
}

// VerifyNotifyEmailRequest represents the request to verify and add notify email
type VerifyNotifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// VerifyNotifyEmail verifies code and adds email to notification list
// POST /api/v1/user/notify-email/verify
func (h *UserHandler) VerifyNotifyEmail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req VerifyNotifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.VerifyAndAddNotifyEmail(c.Request.Context(), subject.UserID, req.Email, req.Code, h.emailCache)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return updated user
	updatedUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, updatedUser)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

// RemoveNotifyEmailRequest represents the request to remove a notify email
type RemoveNotifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RemoveNotifyEmail removes email from notification list
// DELETE /api/v1/user/notify-email
func (h *UserHandler) RemoveNotifyEmail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req RemoveNotifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.RemoveNotifyEmail(c.Request.Context(), subject.UserID, req.Email)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return updated user
	updatedUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, updatedUser)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

// ToggleNotifyEmailRequest represents the request to toggle a notify email's disabled state
type ToggleNotifyEmailRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Disabled bool   `json:"disabled"`
}

// ToggleNotifyEmail toggles the disabled state of a notification email
// PUT /api/v1/user/notify-email/toggle
func (h *UserHandler) ToggleNotifyEmail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ToggleNotifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.ToggleNotifyEmail(c.Request.Context(), subject.UserID, req.Email, req.Disabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	profileResp, err := h.buildUserProfileResponse(c.Request.Context(), subject.UserID, updatedUser)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, profileResp)
}

func (h *UserHandler) buildUserProfileResponse(ctx context.Context, userID int64, user *service.User) (userProfileResponse, error) {
	identities, err := h.userService.GetProfileIdentitySummaries(ctx, userID, user)
	if err != nil {
		return userProfileResponse{}, err
	}
	return userProfileResponseFromService(user, identities), nil
}

func userProfileResponseFromService(user *service.User, identities service.UserIdentitySummarySet) userProfileResponse {
	base := dto.UserFromService(user)
	if base == nil {
		return userProfileResponse{}
	}
	bindings := userProfileBindingMap(identities)
	profileSources, avatarSource, usernameSource := inferUserProfileSources(user, identities)
	return userProfileResponse{
		User:              *base,
		AvatarURL:         user.AvatarURL,
		AvatarSource:      avatarSource,
		UsernameSource:    usernameSource,
		DisplayNameSource: usernameSource,
		NicknameSource:    usernameSource,
		ProfileSources:    profileSources,
		Identities:        identities,
		AuthBindings:      bindings,
		IdentityBindings:  bindings,
		EmailBound:        identities.Email.Bound,
		LinuxDoBound:      identities.LinuxDo.Bound,
		OIDCBound:         identities.OIDC.Bound,
		WeChatBound:       identities.WeChat.Bound,
		DingTalkBound:     identities.DingTalk.Bound,
	}
}

func userProfileBindingMap(identities service.UserIdentitySummarySet) map[string]service.UserIdentitySummary {
	return map[string]service.UserIdentitySummary{
		"email":    identities.Email,
		"linuxdo":  identities.LinuxDo,
		"oidc":     identities.OIDC,
		"wechat":   identities.WeChat,
		"dingtalk": identities.DingTalk,
	}
}

func inferUserProfileSources(user *service.User, identities service.UserIdentitySummarySet) (
	map[string]*userProfileSourceContext,
	*userProfileSourceContext,
	*userProfileSourceContext,
) {
	if user == nil {
		return nil, nil, nil
	}

	thirdParty := thirdPartyIdentityProviders(identities)
	var avatarSource *userProfileSourceContext
	avatarValue := strings.TrimSpace(user.AvatarURL)
	for _, summary := range thirdParty {
		if avatarValue != "" && avatarValue == strings.TrimSpace(summary.AvatarURL) {
			avatarSource = buildUserProfileSourceContext(summary.Provider)
			break
		}
	}

	usernameValue := strings.TrimSpace(user.Username)
	var usernameSource *userProfileSourceContext
	for _, summary := range thirdParty {
		if usernameValue != "" && usernameValue == strings.TrimSpace(summary.DisplayName) {
			usernameSource = buildUserProfileSourceContext(summary.Provider)
			break
		}
	}

	profileSources := map[string]*userProfileSourceContext{}
	if avatarSource != nil {
		profileSources["avatar"] = avatarSource
	}
	if usernameSource != nil {
		profileSources["username"] = usernameSource
		profileSources["display_name"] = usernameSource
		profileSources["nickname"] = usernameSource
	}
	if len(profileSources) == 0 {
		return nil, avatarSource, usernameSource
	}
	return profileSources, avatarSource, usernameSource
}

func thirdPartyIdentityProviders(identities service.UserIdentitySummarySet) []service.UserIdentitySummary {
	out := make([]service.UserIdentitySummary, 0, 3)
	for _, summary := range []service.UserIdentitySummary{identities.LinuxDo, identities.OIDC, identities.WeChat, identities.DingTalk} {
		if summary.Bound {
			out = append(out, summary)
		}
	}
	return out
}

func buildUserProfileSourceContext(provider string) *userProfileSourceContext {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil
	}
	return &userProfileSourceContext{
		Provider: provider,
		Source:   provider,
	}
}
