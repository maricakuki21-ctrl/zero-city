package handler

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type capabilityAssetRequest struct {
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	Description       string   `json:"description"`
	AssetType         string   `json:"asset_type"`
	Status            string   `json:"status"`
	Tags              []string `json:"tags"`
	ScenarioTags      []string `json:"scenario_tags"`
	IntegrationTags   []string `json:"integration_tags"`
	CoverURL          string   `json:"cover_url"`
	ScreenshotURLs    []string `json:"screenshot_urls"`
	VideoURL          string   `json:"video_url"`
	DemoURL           string   `json:"demo_url"`
	DocURL            string   `json:"doc_url"`
	SourceURL         string   `json:"source_url"`
	TemplateURL       string   `json:"template_url"`
	PrimaryActionType string   `json:"primary_action_type"`
	PricingType       string   `json:"pricing_type"`
	ContactEnabled    bool     `json:"contact_enabled"`
}

type capabilityAssetReviewRequest struct {
	Status         string `json:"status"`
	ReviewNote     string `json:"review_note"`
	Featured       *bool  `json:"is_featured"`
	FeaturedWeight *int   `json:"featured_weight"`
}

func capabilityAssetQueryFromRequest(c *gin.Context) service.CapabilityAssetQuery {
	return service.CapabilityAssetQuery{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		AssetType: strings.TrimSpace(c.Query("asset_type")),
		Status:    strings.TrimSpace(c.Query("status")),
		Featured:  c.Query("featured") == "true" || c.Query("featured") == "1",
		Sort:      strings.TrimSpace(c.Query("sort")),
		Limit:     parseLimit(c),
	}
}

func handleCapabilityAssetActivityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCapabilityAssetActivityNotFound), errors.Is(err, sql.ErrNoRows):
		response.NotFound(c, "Capability asset not found")
	case errors.Is(err, service.ErrCapabilityAssetActivityForbidden):
		response.Unauthorized(c, "Capability asset operation forbidden")
	default:
		response.ErrorFrom(c, err)
	}
}

func (h *BizDecipherHandler) ListCapabilityAssets(c *gin.Context) {
	items, err := h.bizService.ListCapabilityAssets(c.Request.Context(), capabilityAssetQueryFromRequest(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) GetCapabilityAsset(c *gin.Context) {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid capability asset id")
		return
	}
	viewerID := int64(0)
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		viewerID = subject.UserID
	}
	asset, err := h.bizService.GetCapabilityAssetDetail(c.Request.Context(), assetID, viewerID)
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(c, "Capability asset not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	if asset == nil {
		response.NotFound(c, "Capability asset not found")
		return
	}
	response.Success(c, asset)
}

func (h *BizDecipherHandler) SetCapabilityAssetLike(c *gin.Context) {
	h.setCapabilityAssetLike(c, true)
}

func (h *BizDecipherHandler) UnsetCapabilityAssetLike(c *gin.Context) {
	h.setCapabilityAssetLike(c, false)
}

func (h *BizDecipherHandler) setCapabilityAssetLike(c *gin.Context, liked bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid capability asset id")
		return
	}
	asset, err := h.bizService.SetCapabilityAssetLike(c.Request.Context(), assetID, subject.UserID, liked)
	if err != nil {
		handleCapabilityAssetActivityError(c, err)
		return
	}
	response.Success(c, asset)
}

func (h *BizDecipherHandler) SetCapabilityAssetFavorite(c *gin.Context) {
	h.setCapabilityAssetFavorite(c, true)
}

func (h *BizDecipherHandler) UnsetCapabilityAssetFavorite(c *gin.Context) {
	h.setCapabilityAssetFavorite(c, false)
}

func (h *BizDecipherHandler) setCapabilityAssetFavorite(c *gin.Context, favorited bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid capability asset id")
		return
	}
	asset, err := h.bizService.SetCapabilityAssetFavorite(c.Request.Context(), assetID, subject.UserID, favorited)
	if err != nil {
		handleCapabilityAssetActivityError(c, err)
		return
	}
	response.Success(c, asset)
}

func (h *BizDecipherHandler) GetMyCapabilityAssetStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.bizService.GetMyCapabilityAssetStats(c.Request.Context(), subject.UserID)
	if err != nil {
		handleCapabilityAssetActivityError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) ListMyCapabilityAssets(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.bizService.ListMyCapabilityAssets(c.Request.Context(), subject.UserID, c.Query("status"), parseLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) CreateCapabilityAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req capabilityAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	asset, err := h.bizService.CreateCapabilityAsset(c.Request.Context(), subject.UserID, service.CapabilityAssetInput{
		Title:             req.Title,
		Summary:           req.Summary,
		Description:       req.Description,
		AssetType:         req.AssetType,
		Status:            req.Status,
		Tags:              req.Tags,
		ScenarioTags:      req.ScenarioTags,
		IntegrationTags:   req.IntegrationTags,
		CoverURL:          req.CoverURL,
		ScreenshotURLs:    req.ScreenshotURLs,
		VideoURL:          req.VideoURL,
		DemoURL:           req.DemoURL,
		DocURL:            req.DocURL,
		SourceURL:         req.SourceURL,
		TemplateURL:       req.TemplateURL,
		PrimaryActionType: req.PrimaryActionType,
		PricingType:       req.PricingType,
		ContactEnabled:    req.ContactEnabled,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, asset)
}

func (h *BizDecipherHandler) UpdateCapabilityAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid capability asset id")
		return
	}
	var req capabilityAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	asset, err := h.bizService.UpdateCapabilityAsset(c.Request.Context(), assetID, subject.UserID, service.CapabilityAssetInput{
		Title:             req.Title,
		Summary:           req.Summary,
		Description:       req.Description,
		AssetType:         req.AssetType,
		Status:            req.Status,
		Tags:              req.Tags,
		ScenarioTags:      req.ScenarioTags,
		IntegrationTags:   req.IntegrationTags,
		CoverURL:          req.CoverURL,
		ScreenshotURLs:    req.ScreenshotURLs,
		VideoURL:          req.VideoURL,
		DemoURL:           req.DemoURL,
		DocURL:            req.DocURL,
		SourceURL:         req.SourceURL,
		TemplateURL:       req.TemplateURL,
		PrimaryActionType: req.PrimaryActionType,
		PricingType:       req.PricingType,
		ContactEnabled:    req.ContactEnabled,
	})
	if err != nil {
		if errors.Is(err, service.ErrCapabilityAssetDraftNotFound) {
			response.NotFound(c, "Capability asset draft not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, asset)
}

func (h *BizDecipherHandler) AdminListCapabilityAssets(c *gin.Context) {
	items, err := h.bizService.AdminListCapabilityAssets(c.Request.Context(), capabilityAssetQueryFromRequest(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *BizDecipherHandler) AdminReviewCapabilityAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid capability asset id")
		return
	}
	var req capabilityAssetReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	asset, err := h.bizService.AdminReviewCapabilityAsset(c.Request.Context(), assetID, subject.UserID, service.CapabilityAssetReviewInput{
		Status:         req.Status,
		ReviewNote:     req.ReviewNote,
		Featured:       req.Featured,
		FeaturedWeight: req.FeaturedWeight,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, asset)
}
