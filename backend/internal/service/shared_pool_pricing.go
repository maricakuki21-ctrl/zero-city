package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	SharedPoolPricingSourceOfficial   = "official_catalog"
	SharedPoolPricingSourceOwner      = "owner_custom"
	SharedPoolEndpointChat            = "chat"
	SharedPoolEndpointResponses       = "responses"
	SharedPoolEndpointImageGeneration = "image_generation"
	SharedPoolEndpointImageEdit       = "image_edit"
	SharedPoolEndpointVideo           = "video"
	SharedPoolFeeModeBuyerSurcharge   = "buyer_surcharge_v1"
)

var (
	ErrSharedPoolPricingUnavailable    = errors.New("shared pool pricing is unavailable")
	ErrSharedPoolOfficialPriceRequired = errors.New("shared pool official price must be resolved from the canonical billing catalog")
	ErrSharedPoolOfficialPriceOnly     = errors.New("official catalog models use official pricing")
	ErrSharedPoolPricingOperationID    = errors.New("pricing operation id was already used with different values")
)

// SharedPoolPriceComponents are USD prices per token or per request. Token
// prices are exposed as per-token values in the API; the UI renders them per
// million tokens so owners never have to reason about tiny decimals.
type SharedPoolPriceComponents struct {
	BillingMode      string   `json:"billing_mode"`
	Currency         string   `json:"currency"`
	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	CacheReadPrice   *float64 `json:"cache_read_price,omitempty"`
	CacheWritePrice  *float64 `json:"cache_write_price,omitempty"`
	ImageItemPrice   *float64 `json:"image_item_price,omitempty"`
	VideoSecondPrice *float64 `json:"video_second_price,omitempty"`
	PerRequestPrice  *float64 `json:"per_request_price,omitempty"`
	MinimumCharge    *float64 `json:"minimum_charge,omitempty"`
	MaximumCharge    *float64 `json:"maximum_charge,omitempty"`
}

type SharedPoolPriceQuote struct {
	PriceVersionID int64                     `json:"price_version_id"`
	PoolID         int64                     `json:"pool_id"`
	PoolModelID    int64                     `json:"pool_model_id"`
	EndpointID     int64                     `json:"endpoint_id"`
	ModelName      string                    `json:"model_name"`
	EndpointType   string                    `json:"endpoint_type"`
	PricingSource  string                    `json:"pricing_source"`
	PricingStatus  string                    `json:"pricing_status"`
	ConfigVersion  int64                     `json:"config_version"`
	BasePrice      SharedPoolPriceComponents `json:"base_price"`
	Multiplier     float64                   `json:"multiplier"`
	// PlatformFeePercent is an additional buyer-side fee. It never reduces the
	// creator/owner price; it is added to the settled buyer charge.
	PlatformFeePercent          float64                    `json:"platform_fee_percent,omitempty"`
	FeeMode                     string                     `json:"fee_mode,omitempty"`
	OwnerPrice                  *SharedPoolPriceComponents `json:"owner_price,omitempty"`
	LongContextInputThreshold   int                        `json:"long_context_input_threshold,omitempty"`
	LongContextInputMultiplier  float64                    `json:"long_context_input_multiplier,omitempty"`
	LongContextOutputMultiplier float64                    `json:"long_context_output_multiplier,omitempty"`
	UserPrice                   SharedPoolPriceComponents  `json:"user_price"`
	EffectiveFrom               time.Time                  `json:"effective_from"`
	PriceHash                   string                     `json:"-"`
	ExplicitFree                bool                       `json:"explicit_free"`
	ExampleCost                 float64                    `json:"example_cost"`
}

type SharedPoolModelEndpointPricing struct {
	PoolModelID           int64                 `json:"pool_model_id"`
	Provider              string                `json:"provider"`
	ModelName             string                `json:"model_name"`
	DisplayName           string                `json:"display_name"`
	PricingSource         string                `json:"pricing_source"`
	PricingStatus         string                `json:"pricing_status"`
	PricingConfigVersion  int64                 `json:"pricing_config_version"`
	EndpointID            int64                 `json:"endpoint_id"`
	EndpointType          string                `json:"endpoint_type"`
	Enabled               bool                  `json:"enabled"`
	GateStatus            string                `json:"gate_status"`
	EndpointPricingStatus string                `json:"endpoint_pricing_status"`
	ConfigVersion         int64                 `json:"config_version"`
	CurrentPrice          *SharedPoolPriceQuote `json:"current_price,omitempty"`
	// CanonicalModelName and RateMultiplier are repository-to-service routing
	// metadata. They are deliberately omitted from the public response.
	CanonicalModelName string  `json:"-"`
	PoolID             int64   `json:"-"`
	RateMultiplier     float64 `json:"-"`
}

type SaveSharedPoolCustomPriceInput struct {
	PoolID           int64
	OwnerID          int64
	ModelName        string
	EndpointType     string
	OperationID      string
	BillingMode      string
	InputPrice       *float64
	OutputPrice      *float64
	CacheReadPrice   *float64
	CacheWritePrice  *float64
	ImageItemPrice   *float64
	VideoSecondPrice *float64
	PerRequestPrice  *float64
	Multiplier       float64
	MinimumCharge    *float64
	MaximumCharge    *float64
}

type sharedPoolPricingRepository interface {
	ListSharedPoolModelEndpointPricing(ctx context.Context, poolID, viewerID int64) ([]SharedPoolModelEndpointPricing, error)
	SaveSharedPoolCustomPriceVersion(ctx context.Context, input SaveSharedPoolCustomPriceInput) (*SharedPoolPriceQuote, error)
	ResolveSharedPoolPriceQuote(ctx context.Context, poolID int64, modelName, endpointType string) (*SharedPoolPriceQuote, error)
	EnsureSharedPoolOfficialPriceVersion(ctx context.Context, poolID int64, modelName, endpointType string, base SharedPoolPriceComponents) (*SharedPoolPriceQuote, error)
}

func (s *BizDecipherService) EnsureSharedPoolOfficialPriceQuote(ctx context.Context, poolID int64, modelName, endpointType string, base SharedPoolPriceComponents) (*SharedPoolPriceQuote, error) {
	endpointType = normalizeSharedPoolEndpoint(endpointType)
	if poolID <= 0 || strings.TrimSpace(modelName) == "" || endpointType == "" {
		return nil, ErrSharedPoolPricingUnavailable
	}
	if err := validateSharedPoolPriceComponents(base); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSharedPoolPricingUnavailable, err)
	}
	repo, err := s.sharedPoolPricingRepository()
	if err != nil {
		return nil, err
	}
	quote, err := repo.EnsureSharedPoolOfficialPriceVersion(ctx, poolID, strings.TrimSpace(modelName), endpointType, base)
	if err != nil {
		return nil, err
	}
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSharedPoolPricingUnavailable, err)
	}
	if err := s.attachSharedPoolPlatformFee(ctx, quote); err != nil {
		return nil, err
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func (s *BizDecipherService) sharedPoolPricingRepository() (sharedPoolPricingRepository, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	repo, ok := s.repo.(sharedPoolPricingRepository)
	if !ok {
		return nil, errors.New("shared pool pricing repository is unavailable")
	}
	return repo, nil
}

func (s *BizDecipherService) ListSharedPoolModelEndpointPricing(ctx context.Context, poolID, viewerID int64) ([]SharedPoolModelEndpointPricing, error) {
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	repo, err := s.sharedPoolPricingRepository()
	if err != nil {
		return nil, err
	}
	items, err := repo.ListSharedPoolModelEndpointPricing(ctx, poolID, viewerID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CurrentPrice = sharedPoolQuoteWithEffectiveMultiplier(items[i].CurrentPrice, items[i].RateMultiplier)
		canonicalModel := strings.TrimSpace(items[i].CanonicalModelName)
		if canonicalModel == "" {
			continue
		}
		items[i].PricingSource = SharedPoolPricingSourceOfficial
		pricing, ok := s.canonicalSharedPoolModelPricing(canonicalModel)
		if !ok && items[i].EndpointType != SharedPoolEndpointImageGeneration && items[i].EndpointType != SharedPoolEndpointImageEdit && items[i].EndpointType != SharedPoolEndpointVideo {
			items[i].PricingStatus = "invalid"
			items[i].EndpointPricingStatus = "invalid"
			items[i].CurrentPrice = nil
			continue
		}
		var preview *SharedPoolPriceQuote
		switch items[i].EndpointType {
		case SharedPoolEndpointImageGeneration, SharedPoolEndpointImageEdit:
			if s.billingService != nil {
				cost := s.billingService.CalculateImageCost(canonicalModel, ImageBillingSize1K, 1, nil, 1)
				if cost != nil && cost.TotalCost > 0 && !math.IsNaN(cost.TotalCost) && !math.IsInf(cost.TotalCost, 0) {
					unit := cost.TotalCost
					preview = canonicalSharedPoolPreviewQuoteWithBase(items[i], SharedPoolPriceComponents{BillingMode: "image", Currency: "USD", ImageItemPrice: &unit})
				}
			}
		case SharedPoolEndpointVideo:
			if s.billingService != nil {
				cost := s.billingService.CalculateVideoCost(canonicalModel, VideoBillingResolution480P, 1, 1, nil, 1)
				if cost != nil && cost.TotalCost > 0 && !math.IsNaN(cost.TotalCost) && !math.IsInf(cost.TotalCost, 0) {
					unit := cost.TotalCost
					preview = canonicalSharedPoolPreviewQuoteWithBase(items[i], SharedPoolPriceComponents{BillingMode: "video", Currency: "USD", VideoSecondPrice: &unit})
				}
			}
		default:
			if ok {
				preview = canonicalSharedPoolPreviewQuote(items[i], pricing)
			}
		}
		if preview == nil {
			items[i].PricingStatus = "invalid"
			items[i].EndpointPricingStatus = "invalid"
			items[i].CurrentPrice = nil
			continue
		}
		items[i].PricingStatus = "ready"
		items[i].EndpointPricingStatus = "ready"
		items[i].CurrentPrice = preview
	}
	feeQuote := &SharedPoolPriceQuote{PoolID: poolID}
	if len(items) > 0 {
		if err := s.attachSharedPoolPlatformFee(ctx, feeQuote); err != nil {
			return nil, err
		}
	}
	for i := range items {
		if quote := items[i].CurrentPrice; quote != nil {
			cloned := *quote
			cloned.FeeMode = feeQuote.FeeMode
			cloned.PlatformFeePercent = feeQuote.PlatformFeePercent
			FinalizeSharedPoolPriceQuote(&cloned)
			items[i].CurrentPrice = &cloned
		}
	}
	return items, nil
}

func (s *BizDecipherService) canonicalSharedPoolModelPricing(modelName string) (*ModelPricing, bool) {
	if s == nil || s.billingService == nil || strings.TrimSpace(modelName) == "" {
		return nil, false
	}
	pricing, err := s.billingService.GetModelPricing(strings.TrimSpace(modelName))
	if err != nil || !validCanonicalSharedPoolModelPricing(pricing) {
		return nil, false
	}
	return pricing, true
}

func validCanonicalSharedPoolModelPricing(pricing *ModelPricing) bool {
	if pricing == nil {
		return false
	}
	values := []float64{
		pricing.InputPricePerToken,
		pricing.OutputPricePerToken,
		pricing.CacheReadPricePerToken,
		pricing.CacheCreationPricePerToken,
	}
	for _, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	// BillingService currently has no explicit "officially free" marker. A
	// zero/zero token price is therefore indistinguishable from incomplete
	// catalog data and must fail closed instead of advertising free usage.
	return pricing.InputPricePerToken > 0 || pricing.OutputPricePerToken > 0
}

func (s *BizDecipherService) canonicalSharedPoolPriceSnapshot(modelName string) *SharedPoolPriceSnapshot {
	pricing, ok := s.canonicalSharedPoolModelPricing(modelName)
	if !ok {
		return nil
	}
	inputPrice := pricing.InputPricePerToken
	outputPrice := pricing.OutputPricePerToken
	cacheReadPrice := pricing.CacheReadPricePerToken
	cacheWritePrice := pricing.CacheCreationPricePerToken
	return &SharedPoolPriceSnapshot{
		BillingMode:     "token",
		InputPrice:      &inputPrice,
		OutputPrice:     &outputPrice,
		CacheReadPrice:  &cacheReadPrice,
		CacheWritePrice: &cacheWritePrice,
		PriceSource:     "billing_service",
	}
}

func canonicalSharedPoolPreviewQuote(item SharedPoolModelEndpointPricing, pricing *ModelPricing) *SharedPoolPriceQuote {
	if pricing == nil {
		return nil
	}
	return sharedPoolQuoteWithOfficialPolicy(canonicalSharedPoolPreviewQuoteWithBase(item, sharedPoolOfficialBasePrice(pricing)), pricing)
}

func canonicalSharedPoolPreviewQuoteWithBase(item SharedPoolModelEndpointPricing, base SharedPoolPriceComponents) *SharedPoolPriceQuote {
	multiplier := item.RateMultiplier
	if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		multiplier = 1
	}
	quote := &SharedPoolPriceQuote{
		PoolID:        item.PoolID,
		PoolModelID:   item.PoolModelID,
		EndpointID:    item.EndpointID,
		ModelName:     item.ModelName,
		EndpointType:  item.EndpointType,
		PricingSource: SharedPoolPricingSourceOfficial,
		PricingStatus: "ready",
		ConfigVersion: item.ConfigVersion,
		BasePrice:     base,
		Multiplier:    multiplier,
		EffectiveFrom: time.Now().UTC(),
	}
	priceHash, _ := BuildSharedPoolPriceHash(quote.ModelName, quote.EndpointType, quote.PricingSource, quote.BasePrice, quote.Multiplier)
	quote.PriceHash = priceHash
	if current := item.CurrentPrice; current != nil && current.PricingSource == SharedPoolPricingSourceOfficial && current.PriceHash == priceHash {
		quote.PriceVersionID = current.PriceVersionID
		quote.PoolID = current.PoolID
		quote.ConfigVersion = current.ConfigVersion
		quote.EffectiveFrom = current.EffectiveFrom
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote
}

// BuildSharedPoolPriceHash is shared by repository persistence and service UI
// previews so both compare the exact same canonical component set.
func BuildSharedPoolPriceHash(modelName, endpointType, source string, base SharedPoolPriceComponents, multiplier float64) (string, error) {
	payload := struct {
		ModelName    string                    `json:"model_name"`
		EndpointType string                    `json:"endpoint_type"`
		Source       string                    `json:"source"`
		Base         SharedPoolPriceComponents `json:"base"`
		Multiplier   float64                   `json:"multiplier"`
	}{strings.ToLower(strings.TrimSpace(modelName)), endpointType, source, base, multiplier}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *BizDecipherService) SaveSharedPoolCustomPrice(ctx context.Context, input SaveSharedPoolCustomPriceInput) (*SharedPoolPriceQuote, error) {
	if err := normalizeAndValidateSharedPoolCustomPrice(&input); err != nil {
		return nil, err
	}
	repo, err := s.sharedPoolPricingRepository()
	if err != nil {
		return nil, err
	}
	quote, err := repo.SaveSharedPoolCustomPriceVersion(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.attachSharedPoolPlatformFee(ctx, quote); err != nil {
		return nil, err
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func (s *BizDecipherService) ResolveSharedPoolPriceQuote(ctx context.Context, poolID int64, modelName, endpointType string) (*SharedPoolPriceQuote, error) {
	if poolID <= 0 || strings.TrimSpace(modelName) == "" {
		return nil, ErrSharedPoolPricingUnavailable
	}
	endpointType = normalizeSharedPoolEndpoint(endpointType)
	if endpointType == "" {
		return nil, ErrSharedPoolPricingUnavailable
	}
	repo, err := s.sharedPoolPricingRepository()
	if err != nil {
		return nil, err
	}
	quote, err := repo.ResolveSharedPoolPriceQuote(ctx, poolID, strings.TrimSpace(modelName), endpointType)
	if err != nil {
		return nil, err
	}
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSharedPoolPricingUnavailable, err)
	}
	if err := s.attachSharedPoolPlatformFee(ctx, quote); err != nil {
		return nil, err
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

type sharedPoolPlatformFeeRepository interface {
	SharedPoolPlatformFeePercent(ctx context.Context, poolID int64) (float64, error)
}

func (s *BizDecipherService) attachSharedPoolPlatformFee(ctx context.Context, quote *SharedPoolPriceQuote) error {
	if quote == nil || quote.PoolID <= 0 || s == nil || s.repo == nil {
		return nil
	}
	repo, ok := s.repo.(sharedPoolPlatformFeeRepository)
	if !ok {
		// Lightweight service fakes and historical snapshots predate the fee
		// field. Treat them as zero rather than making pricing unavailable.
		quote.PlatformFeePercent = 0
		return nil
	}
	fee, err := repo.SharedPoolPlatformFeePercent(ctx, quote.PoolID)
	if err != nil {
		return err
	}
	if fee < 0 || fee > 100 || math.IsNaN(fee) || math.IsInf(fee, 0) {
		return errors.New("shared pool platform fee is invalid")
	}
	quote.PlatformFeePercent = fee
	quote.FeeMode = SharedPoolFeeModeBuyerSurcharge
	return nil
}

// GetSharedPoolAccessKeyQuoteByAPIKeyID resolves routing and freezes the exact
// accepted quote before any upstream bytes are sent. The returned quote is the
// future pre-authorization boundary: balance reservation can use its version,
// components and bounds without re-reading mutable configuration.
func (s *BizDecipherService) GetSharedPoolAccessKeyQuoteByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, endpointType string) (*SharedPoolAccessKey, error) {
	accessKey, err := s.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	modelName := strings.TrimSpace(accessKey.PublishedModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(reqModel)
	}
	quote, err := s.ResolveSharedPoolPriceQuote(ctx, accessKey.PoolID, modelName, endpointType)
	if err != nil {
		return nil, err
	}
	accessKey.PriceQuote = quote
	return accessKey, nil
}

func normalizeAndValidateSharedPoolCustomPrice(input *SaveSharedPoolCustomPriceInput) error {
	if input == nil || input.PoolID <= 0 || input.OwnerID <= 0 {
		return errors.New("invalid pool or owner id")
	}
	input.ModelName = strings.TrimSpace(input.ModelName)
	if input.ModelName == "" || len([]rune(input.ModelName)) > 200 {
		return errors.New("model name is required and must not exceed 200 characters")
	}
	input.EndpointType = normalizeSharedPoolEndpoint(input.EndpointType)
	if input.EndpointType == "" {
		return errors.New("unsupported shared-pool endpoint")
	}
	input.OperationID = strings.TrimSpace(input.OperationID)
	if input.OperationID == "" || len(input.OperationID) > 128 {
		return errors.New("operation id is required and must not exceed 128 characters")
	}
	input.BillingMode = strings.ToLower(strings.TrimSpace(input.BillingMode))
	if input.Multiplier <= 0 || math.IsNaN(input.Multiplier) || math.IsInf(input.Multiplier, 0) {
		return errors.New("multiplier must be greater than zero")
	}
	components := SharedPoolPriceComponents{
		BillingMode: input.BillingMode, Currency: "USD", InputPrice: input.InputPrice,
		OutputPrice: input.OutputPrice, CacheReadPrice: input.CacheReadPrice,
		CacheWritePrice: input.CacheWritePrice, ImageItemPrice: input.ImageItemPrice,
		VideoSecondPrice: input.VideoSecondPrice, PerRequestPrice: input.PerRequestPrice,
		MinimumCharge: input.MinimumCharge, MaximumCharge: input.MaximumCharge,
	}
	if err := validateSharedPoolBillingModeForEndpoint(input.EndpointType, input.BillingMode); err != nil {
		return err
	}
	return validateSharedPoolPriceComponents(components)
}

func validateSharedPoolBillingModeForEndpoint(endpointType, billingMode string) error {
	switch normalizeSharedPoolEndpoint(endpointType) {
	case SharedPoolEndpointChat, SharedPoolEndpointResponses:
		if billingMode != "token" && billingMode != "per_request" {
			return errors.New("text endpoints require token or per_request pricing")
		}
	case SharedPoolEndpointImageGeneration, SharedPoolEndpointImageEdit:
		if billingMode != "image" && billingMode != "per_request" {
			return errors.New("image endpoints require image or per_request pricing")
		}
	case SharedPoolEndpointVideo:
		if billingMode != "video" && billingMode != "per_request" {
			return errors.New("video endpoints require video or per_request pricing")
		}
	default:
		return errors.New("unsupported shared-pool endpoint")
	}
	return nil
}

func ValidateSharedPoolPriceQuote(quote *SharedPoolPriceQuote) error {
	if quote == nil || quote.PriceVersionID <= 0 || quote.PoolID <= 0 || quote.EndpointID <= 0 {
		return errors.New("price version is missing")
	}
	if normalizeSharedPoolEndpoint(quote.EndpointType) == "" {
		return errors.New("endpoint is unsupported")
	}
	if err := validateSharedPoolBillingModeForEndpoint(quote.EndpointType, strings.ToLower(strings.TrimSpace(quote.BasePrice.BillingMode))); err != nil {
		return err
	}
	if quote.PricingSource != SharedPoolPricingSourceOfficial && quote.PricingSource != SharedPoolPricingSourceOwner {
		return errors.New("pricing source is invalid")
	}
	if quote.Multiplier <= 0 || math.IsNaN(quote.Multiplier) || math.IsInf(quote.Multiplier, 0) {
		return errors.New("multiplier is invalid")
	}
	if quote.FeeMode != "" && quote.FeeMode != SharedPoolFeeModeBuyerSurcharge {
		return errors.New("fee mode is unsupported")
	}
	if quote.PlatformFeePercent < 0 || quote.PlatformFeePercent > 100 ||
		math.IsNaN(quote.PlatformFeePercent) || math.IsInf(quote.PlatformFeePercent, 0) {
		return errors.New("platform fee is invalid")
	}
	if quote.LongContextInputThreshold < 0 {
		return errors.New("long-context threshold is invalid")
	}
	if quote.LongContextInputThreshold > 0 {
		if quote.LongContextInputMultiplier <= 0 || math.IsNaN(quote.LongContextInputMultiplier) || math.IsInf(quote.LongContextInputMultiplier, 0) ||
			quote.LongContextOutputMultiplier <= 0 || math.IsNaN(quote.LongContextOutputMultiplier) || math.IsInf(quote.LongContextOutputMultiplier, 0) {
			return errors.New("long-context multipliers are invalid")
		}
	}
	return validateSharedPoolPriceComponents(quote.BasePrice)
}

func validateSharedPoolPriceComponents(components SharedPoolPriceComponents) error {
	components.BillingMode = strings.ToLower(strings.TrimSpace(components.BillingMode))
	if components.Currency != "" && !strings.EqualFold(components.Currency, "USD") {
		return errors.New("only USD pricing is supported")
	}
	for name, value := range map[string]*float64{
		"input price": components.InputPrice, "output price": components.OutputPrice,
		"cache read price": components.CacheReadPrice, "cache write price": components.CacheWritePrice,
		"image item price": components.ImageItemPrice, "video second price": components.VideoSecondPrice,
		"per-request price": components.PerRequestPrice, "minimum charge": components.MinimumCharge,
		"maximum charge": components.MaximumCharge,
	} {
		if value != nil && (*value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return fmt.Errorf("%s must be a non-negative finite number", name)
		}
	}
	if components.MinimumCharge != nil && components.MaximumCharge != nil && *components.MinimumCharge > *components.MaximumCharge {
		return errors.New("minimum charge must not exceed maximum charge")
	}
	if components.MaximumCharge != nil && *components.MaximumCharge > 0 && *components.MaximumCharge < 0.00000001 {
		return errors.New("maximum charge must be zero or at least 0.00000001")
	}
	switch components.BillingMode {
	case "token":
		if components.InputPrice == nil || components.OutputPrice == nil {
			return errors.New("token pricing requires both input and output prices")
		}
	case "per_request":
		if components.PerRequestPrice == nil {
			return errors.New("per-request pricing requires a request price")
		}
	case "image":
		if components.ImageItemPrice == nil {
			return errors.New("image pricing requires an image item price")
		}
	case "video":
		if components.VideoSecondPrice == nil {
			return errors.New("video pricing requires a video second price")
		}
	default:
		return errors.New("billing mode must be token, per_request, image, or video")
	}
	return nil
}

func normalizeSharedPoolEndpoint(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SharedPoolEndpointChat, "chat_completions", "/v1/chat/completions":
		return SharedPoolEndpointChat
	case SharedPoolEndpointResponses, "/v1/responses":
		return SharedPoolEndpointResponses
	case SharedPoolEndpointImageGeneration, "images_generations", "/v1/images/generations", "/images/generations":
		return SharedPoolEndpointImageGeneration
	case SharedPoolEndpointImageEdit, "images_edits", "/v1/images/edits", "/images/edits":
		return SharedPoolEndpointImageEdit
	case SharedPoolEndpointVideo, "video_generation", "videos_generations", "/v1/videos/generations", "/videos/generations":
		return SharedPoolEndpointVideo
	default:
		return ""
	}
}

func BuildSharedPoolUserPrice(base SharedPoolPriceComponents, multiplier float64) SharedPoolPriceComponents {
	user := base
	user.InputPrice = multiplyOptionalPrice(base.InputPrice, multiplier)
	user.OutputPrice = multiplyOptionalPrice(base.OutputPrice, multiplier)
	user.CacheReadPrice = multiplyOptionalPrice(base.CacheReadPrice, multiplier)
	user.CacheWritePrice = multiplyOptionalPrice(base.CacheWritePrice, multiplier)
	user.ImageItemPrice = multiplyOptionalPrice(base.ImageItemPrice, multiplier)
	user.VideoSecondPrice = multiplyOptionalPrice(base.VideoSecondPrice, multiplier)
	user.PerRequestPrice = multiplyOptionalPrice(base.PerRequestPrice, multiplier)
	// Minimum/maximum are final per-call bounds and are not multiplied again.
	return user
}

// AddSharedPoolPlatformFee keeps owner bounds intact and exposes buyer-inclusive
// unit prices and bounds for display.
func AddSharedPoolPlatformFee(price SharedPoolPriceComponents, percent float64) SharedPoolPriceComponents {
	if percent <= 0 || math.IsNaN(percent) || math.IsInf(percent, 0) {
		return price
	}
	factor := 1 + math.Min(percent, 100)/100
	price.InputPrice = multiplyOptionalPrice(price.InputPrice, factor)
	price.OutputPrice = multiplyOptionalPrice(price.OutputPrice, factor)
	price.CacheReadPrice = multiplyOptionalPrice(price.CacheReadPrice, factor)
	price.CacheWritePrice = multiplyOptionalPrice(price.CacheWritePrice, factor)
	price.ImageItemPrice = multiplyOptionalPrice(price.ImageItemPrice, factor)
	price.VideoSecondPrice = multiplyOptionalPrice(price.VideoSecondPrice, factor)
	price.PerRequestPrice = multiplyOptionalPrice(price.PerRequestPrice, factor)
	price.MinimumCharge = multiplyOptionalPrice(price.MinimumCharge, factor)
	price.MaximumCharge = multiplyOptionalPrice(price.MaximumCharge, factor)
	return price
}

func SharedPoolBuyerCharge(ownerPrice, percent float64) float64 {
	if ownerPrice <= 0 || math.IsNaN(ownerPrice) || math.IsInf(ownerPrice, 0) {
		return math.Max(0, ownerPrice)
	}
	if percent <= 0 || math.IsNaN(percent) || math.IsInf(percent, 0) {
		return ownerPrice
	}
	ownerUnits := math.Round(ownerPrice * 1e8)
	feeUnits := math.Round(ownerUnits * math.Min(percent, 100) / 100)
	return (ownerUnits + feeUnits) / 1e8
}

func sharedPoolQuoteBuyerCharge(quote *SharedPoolPriceQuote, ownerPrice float64) float64 {
	ownerPrice = quantizedSharedPoolCharge(ownerPrice, quote.BasePrice.MaximumCharge)
	if quote.FeeMode != SharedPoolFeeModeBuyerSurcharge {
		return ownerPrice
	}
	return SharedPoolBuyerCharge(ownerPrice, quote.PlatformFeePercent)
}

func FinalizeSharedPoolPriceQuote(quote *SharedPoolPriceQuote) {
	if quote == nil {
		return
	}
	if quote.BasePrice.Currency == "" {
		quote.BasePrice.Currency = "USD"
	}
	quote.UserPrice = BuildSharedPoolUserPrice(quote.BasePrice, quote.Multiplier)
	if quote.FeeMode == SharedPoolFeeModeBuyerSurcharge {
		owner := quote.UserPrice
		quote.OwnerPrice = &owner
		quote.UserPrice = AddSharedPoolPlatformFee(quote.UserPrice, quote.PlatformFeePercent)
	}
	quote.ExplicitFree = sharedPoolComponentsAreFree(quote.BasePrice)
	quote.ExampleCost = SharedPoolQuoteExampleCost(quote)
}

func sharedPoolQuoteWithEffectiveMultiplier(quote *SharedPoolPriceQuote, multiplier float64) *SharedPoolPriceQuote {
	if quote == nil || multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || math.Abs(multiplier-quote.Multiplier) <= 0.000000000001 {
		return quote
	}
	cloned := *quote
	cloned.Multiplier = multiplier
	cloned.PriceHash = ""
	FinalizeSharedPoolPriceQuote(&cloned)
	return &cloned
}

func SharedPoolQuoteExampleCost(quote *SharedPoolPriceQuote) float64 {
	if quote == nil {
		return 0
	}
	if quote.FeeMode == SharedPoolFeeModeBuyerSurcharge {
		ownerQuote := *quote
		ownerQuote.FeeMode = ""
		ownerQuote.UserPrice = BuildSharedPoolUserPrice(quote.BasePrice, quote.Multiplier)
		return sharedPoolQuoteBuyerCharge(quote, SharedPoolQuoteExampleCost(&ownerQuote))
	}
	var cost float64
	switch quote.BasePrice.BillingMode {
	case "token":
		if quote.UserPrice.InputPrice != nil {
			cost += 1000 * *quote.UserPrice.InputPrice
		}
		if quote.UserPrice.OutputPrice != nil {
			cost += 500 * *quote.UserPrice.OutputPrice
		}
	case "per_request":
		if quote.UserPrice.PerRequestPrice != nil {
			cost = *quote.UserPrice.PerRequestPrice
		}
	case "image":
		if quote.UserPrice.ImageItemPrice != nil {
			cost = *quote.UserPrice.ImageItemPrice
		}
	case "video":
		if quote.UserPrice.VideoSecondPrice != nil {
			cost = *quote.UserPrice.VideoSecondPrice * VideoBillingDefaultDurationSeconds
		}
	}
	return applySharedPoolChargeBounds(cost, quote.BasePrice.MinimumCharge, quote.BasePrice.MaximumCharge)
}

func MarshalSharedPoolPriceSnapshot(quote *SharedPoolPriceQuote) json.RawMessage {
	if quote == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(quote)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func multiplyOptionalPrice(value *float64, multiplier float64) *float64 {
	if value == nil {
		return nil
	}
	result := *value * multiplier
	return &result
}

func sharedPoolComponentsAreFree(components SharedPoolPriceComponents) bool {
	values := []*float64{
		components.InputPrice, components.OutputPrice, components.CacheReadPrice,
		components.CacheWritePrice, components.ImageItemPrice, components.VideoSecondPrice,
		components.PerRequestPrice, components.MinimumCharge,
	}
	for _, value := range values {
		if value != nil && *value > 0 {
			return false
		}
	}
	return true
}

func applySharedPoolChargeBounds(cost float64, minimum, maximum *float64) float64 {
	if minimum != nil && cost < *minimum {
		cost = *minimum
	}
	if maximum != nil && cost > *maximum {
		cost = *maximum
	}
	return cost
}
