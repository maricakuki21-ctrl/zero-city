package service

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	CapabilityAssetStatusDraft    = "draft"
	CapabilityAssetStatusPending  = "pending"
	CapabilityAssetStatusListed   = "listed"
	CapabilityAssetStatusRejected = "rejected"
	CapabilityAssetStatusArchived = "archived"
	CapabilityAssetStatusDelisted = "delisted"
)

var ErrCapabilityAssetDraftNotFound = errors.New("capability asset draft not found")

type CapabilityAssetInput struct {
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

type CapabilityAssetQuery struct {
	Keyword   string
	AssetType string
	Status    string
	Featured  bool
	Sort      string
	Limit     int
}

type CapabilityAssetReviewInput struct {
	Status         string `json:"status"`
	ReviewNote     string `json:"review_note"`
	Featured       *bool  `json:"is_featured,omitempty"`
	FeaturedWeight *int   `json:"featured_weight,omitempty"`
}

type CapabilityAsset struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"user_id"`
	Author            string     `json:"author"`
	Title             string     `json:"title"`
	Slug              string     `json:"slug"`
	Summary           string     `json:"summary"`
	Description       string     `json:"description"`
	AssetType         string     `json:"asset_type"`
	Status            string     `json:"status"`
	Tags              []string   `json:"tags"`
	ScenarioTags      []string   `json:"scenario_tags"`
	IntegrationTags   []string   `json:"integration_tags"`
	CoverURL          string     `json:"cover_url"`
	ScreenshotURLs    []string   `json:"screenshot_urls"`
	VideoURL          string     `json:"video_url"`
	DemoURL           string     `json:"demo_url"`
	DocURL            string     `json:"doc_url"`
	SourceURL         string     `json:"source_url"`
	TemplateURL       string     `json:"template_url"`
	PrimaryActionType string     `json:"primary_action_type"`
	PricingType       string     `json:"pricing_type"`
	ContactEnabled    bool       `json:"contact_enabled"`
	Featured          bool       `json:"is_featured"`
	FeaturedWeight    int        `json:"featured_weight"`
	ViewCount         int64      `json:"view_count"`
	LikeCount         int64      `json:"like_count"`
	FavoriteCount     int64      `json:"favorite_count"`
	DownloadCount     int64      `json:"download_count"`
	UseCount          int64      `json:"use_count"`
	LikedByMe         bool       `json:"liked_by_me"`
	FavoritedByMe     bool       `json:"favorited_by_me"`
	DownloadedByMe    bool       `json:"downloaded_by_me"`
	ViewedToday       bool       `json:"viewed_today"`
	CommentCount      int64      `json:"comment_count"`
	RatingAvg         float64    `json:"rating_avg"`
	RatingCount       int64      `json:"rating_count"`
	ReviewNote        string     `json:"review_note"`
	ReviewedBy        *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	ArchivedAt        *time.Time `json:"archived_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

var capabilitySlugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func (s *BizDecipherService) ListCapabilityAssets(ctx context.Context, query CapabilityAssetQuery) ([]CapabilityAsset, error) {
	query.Status = CapabilityAssetStatusListed
	query.AssetType = normalizeCapabilityAssetTypeFilter(query.AssetType)
	query.Sort = normalizeCapabilityAssetSort(query.Sort)
	items, err := s.repo.ListCapabilityAssets(ctx, query)
	if err != nil {
		return nil, err
	}
	for i := range items {
		hidePaidAssetLinks(&items[i])
	}
	return items, nil
}

func (s *BizDecipherService) GetCapabilityAsset(ctx context.Context, id int64) (*CapabilityAsset, error) {
	if id <= 0 {
		return nil, errors.New("invalid capability asset id")
	}
	asset, err := s.repo.GetCapabilityAsset(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if asset == nil || asset.Status != CapabilityAssetStatusListed {
		return nil, sql.ErrNoRows
	}
	hidePaidAssetLinks(asset)
	return asset, nil
}

func (s *BizDecipherService) ListMyCapabilityAssets(ctx context.Context, userID int64, status string, limit int) ([]CapabilityAsset, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.repo.ListMyCapabilityAssets(ctx, userID, normalizeCapabilityAssetStatusFilter(status), limit)
}

func (s *BizDecipherService) CreateCapabilityAsset(ctx context.Context, userID int64, input CapabilityAssetInput) (*CapabilityAsset, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	input = normalizeCapabilityAssetInput(input)
	if input.Title == "" {
		return nil, errors.New("asset title is required")
	}
	if input.Summary == "" {
		return nil, errors.New("asset summary is required")
	}
	if input.Description == "" {
		return nil, errors.New("asset description is required")
	}
	switch input.Status {
	case "", CapabilityAssetStatusDraft:
		input.Status = CapabilityAssetStatusDraft
	case CapabilityAssetStatusPending:
		input.Status = CapabilityAssetStatusPending
	default:
		return nil, errors.New("asset can only be created as draft or pending")
	}
	return s.repo.CreateCapabilityAsset(ctx, userID, capabilityAssetSlugBase(input.Title), input)
}

func (s *BizDecipherService) UpdateCapabilityAsset(ctx context.Context, assetID, userID int64, input CapabilityAssetInput) (*CapabilityAsset, error) {
	if assetID <= 0 {
		return nil, errors.New("invalid capability asset id")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	input = normalizeCapabilityAssetInput(input)
	if input.Title == "" {
		return nil, errors.New("asset title is required")
	}
	if input.Summary == "" {
		return nil, errors.New("asset summary is required")
	}
	if input.Description == "" {
		return nil, errors.New("asset description is required")
	}
	if input.Status != "" && input.Status != CapabilityAssetStatusDraft {
		return nil, errors.New("asset status cannot be changed by draft edit")
	}
	input.Status = CapabilityAssetStatusDraft
	return s.repo.UpdateCapabilityAsset(ctx, assetID, userID, input)
}

func (s *BizDecipherService) AdminListCapabilityAssets(ctx context.Context, query CapabilityAssetQuery) ([]CapabilityAsset, error) {
	query.Status = normalizeCapabilityAssetStatusFilter(query.Status)
	query.AssetType = normalizeCapabilityAssetTypeFilter(query.AssetType)
	query.Sort = normalizeCapabilityAssetSort(query.Sort)
	return s.repo.AdminListCapabilityAssets(ctx, query)
}

func (s *BizDecipherService) AdminReviewCapabilityAsset(ctx context.Context, assetID, reviewerID int64, input CapabilityAssetReviewInput) (*CapabilityAsset, error) {
	if assetID <= 0 {
		return nil, errors.New("invalid capability asset id")
	}
	if reviewerID <= 0 {
		return nil, errors.New("invalid reviewer id")
	}
	input.Status = normalizeCapabilityAssetReviewStatus(input.Status)
	if input.Status == "" {
		return nil, errors.New("invalid capability asset review status")
	}
	input.ReviewNote = strings.TrimSpace(input.ReviewNote)
	return s.repo.AdminReviewCapabilityAsset(ctx, assetID, reviewerID, input)
}

func normalizeCapabilityAssetInput(input CapabilityAssetInput) CapabilityAssetInput {
	input.Title = truncateCapabilityString(strings.TrimSpace(input.Title), 160)
	input.Summary = truncateCapabilityString(strings.TrimSpace(input.Summary), 360)
	input.Description = strings.TrimSpace(input.Description)
	input.AssetType = normalizeCapabilityAssetType(input.AssetType)
	input.Status = strings.TrimSpace(input.Status)
	input.Tags = normalizeCapabilityStringList(input.Tags, 8, 32)
	input.ScenarioTags = normalizeCapabilityStringList(input.ScenarioTags, 8, 40)
	input.IntegrationTags = normalizeCapabilityStringList(input.IntegrationTags, 12, 40)
	input.CoverURL = strings.TrimSpace(input.CoverURL)
	input.ScreenshotURLs = normalizeCapabilityStringList(input.ScreenshotURLs, 6, 512)
	input.VideoURL = strings.TrimSpace(input.VideoURL)
	input.DemoURL = strings.TrimSpace(input.DemoURL)
	input.DocURL = strings.TrimSpace(input.DocURL)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	input.TemplateURL = strings.TrimSpace(input.TemplateURL)
	input.PrimaryActionType = normalizeCapabilityActionType(input.PrimaryActionType, input.AssetType)
	input.PricingType = normalizeCapabilityPricing(input.PricingType)
	return input
}

func normalizeCapabilityAssetType(value string) string {
	switch strings.TrimSpace(value) {
	case "product_app", "game", "workflow", "agent", "api_tool", "plugin_template", "prompt_solution", "dataset_report", "other":
		return strings.TrimSpace(value)
	default:
		return "other"
	}
}

func normalizeCapabilityAssetTypeFilter(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "all" {
		return ""
	}
	return normalizeCapabilityAssetType(value)
}

func normalizeCapabilityAssetStatusFilter(status string) string {
	status = strings.TrimSpace(status)
	if status == "" || status == "all" {
		return ""
	}
	switch status {
	case CapabilityAssetStatusDraft, CapabilityAssetStatusPending, CapabilityAssetStatusListed, CapabilityAssetStatusRejected, CapabilityAssetStatusArchived, CapabilityAssetStatusDelisted:
		return status
	default:
		return ""
	}
}

func normalizeCapabilityAssetReviewStatus(status string) string {
	switch strings.TrimSpace(status) {
	case CapabilityAssetStatusListed, CapabilityAssetStatusRejected, CapabilityAssetStatusDelisted:
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func normalizeCapabilityAssetSort(sort string) string {
	switch strings.TrimSpace(sort) {
	case "latest", "popular", "featured", "updated":
		return strings.TrimSpace(sort)
	default:
		return "featured"
	}
}

func normalizeCapabilityActionType(value, assetType string) string {
	switch strings.TrimSpace(value) {
	case "visit_product", "open_demo", "view_workflow", "open_agent", "view_docs", "copy_prompt", "view_report", "download_template", "contact_author", "view_detail":
		return strings.TrimSpace(value)
	}
	switch assetType {
	case "product_app":
		return "visit_product"
	case "game":
		return "open_demo"
	case "workflow":
		return "view_workflow"
	case "agent":
		return "open_agent"
	case "api_tool":
		return "view_docs"
	case "plugin_template":
		return "download_template"
	case "prompt_solution":
		return "copy_prompt"
	case "dataset_report":
		return "view_report"
	default:
		return "view_detail"
	}
}

func normalizeCapabilityPricing(value string) string {
	switch strings.TrimSpace(value) {
	case "free", "paid", "contact", "open_source":
		return strings.TrimSpace(value)
	default:
		return "free"
	}
}

func normalizeCapabilityStringList(values []string, maxItems, maxLen int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = truncateCapabilityString(strings.TrimSpace(value), maxLen)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func capabilityAssetSlugBase(title string) string {
	base := strings.ToLower(strings.TrimSpace(title))
	base = capabilitySlugUnsafe.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "asset"
	}
	return truncateCapabilityString(base, 120)
}

func truncateCapabilityString(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
