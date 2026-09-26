package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/gin-gonic/gin"
)

// ErrSharedPoolPricingNotConfigured is returned before an upstream request when
// the selected shared-pool model has no resolvable price. Shared pools fail
// closed because a successful, unpriced request cannot be safely settled later.
var ErrSharedPoolPricingNotConfigured = errors.New("shared pool pricing is not configured")

var sharedPoolCapacityRetryDelays = [...]time.Duration{
	500 * time.Millisecond,
	1500 * time.Millisecond,
}

// ErrSharedPoolOfficialMediaVariantUnsupported keeps official-catalog media
// prices canonical. Shared pools intentionally expose one official image tier
// (1K) and one official video tier (480p); owner_custom prices remain uniform
// per image/per second across the protocol's supported variants.
var ErrSharedPoolOfficialMediaVariantUnsupported = errors.New("shared pool official media variant is not supported")

// ErrSharedPoolUsageUnavailable means the upstream returned a successful body
// without terminal usage data. The reservation must stay frozen for review; a
// token-priced request can never be released as if no work happened.
var ErrSharedPoolUsageUnavailable = errors.New("shared pool upstream usage is unavailable")

// ValidateSharedPoolPricing resolves a real one-token quote before forwarding a
// shared-pool request. The quote is intentionally tiny; it proves that a pricing
// rule exists without charging or mutating any balance.
func (s *OpenAIGatewayService) ValidateSharedPoolPricing(
	ctx context.Context,
	apiKey *APIKey,
	accessKey *SharedPoolAccessKey,
	requestedModel string,
) error {
	_ = ctx
	_ = apiKey
	if s == nil || accessKey == nil || accessKey.PriceQuote == nil {
		return ErrSharedPoolPricingNotConfigured
	}
	if err := ValidateSharedPoolPriceQuote(accessKey.PriceQuote); err == nil {
		return nil
	} else {
		model := strings.TrimSpace(accessKey.PublishedModelName)
		if model == "" {
			model = strings.TrimSpace(requestedModel)
		}
		if model == "" {
			model = "unknown"
		}
		return fmt.Errorf("%w: %s: %v", ErrSharedPoolPricingNotConfigured, model, err)
	}
}

// ForwardSharedPoolRawChatCompletions forwards a shared-pool key request to the
// pool owner's OpenAI-compatible upstream as raw /v1/chat/completions.
func (s *OpenAIGatewayService) ForwardSharedPoolRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	accessKey *SharedPoolAccessKey,
	body []byte,
) (*OpenAIForwardResult, error) {
	return forwardSharedPoolWithCapacityRetry(ctx, c, func() (*OpenAIForwardResult, error) {
		account, err := sharedPoolOpenAIAccount(accessKey)
		if err != nil {
			return nil, err
		}
		if account.Type == AccountTypeOAuth {
			return s.ForwardAsChatCompletions(ctx, c, account, body, "", accessKey.UpstreamModelName)
		}
		return s.forwardAsRawChatCompletions(ctx, c, account, body, accessKey.UpstreamModelName)
	})
}

// ForwardSharedPoolResponses serves shared-pool /v1/responses clients through
// the same OpenAI-compatible upstream. Shared pools are modeled as APIKey
// upstreams with force_chat_completions so Codex/Responses clients can use
// existing Responses -> Chat Completions fallback conversion without local
// account scheduling.
func (s *OpenAIGatewayService) ForwardSharedPoolResponses(
	ctx context.Context,
	c *gin.Context,
	accessKey *SharedPoolAccessKey,
	body []byte,
) (*OpenAIForwardResult, error) {
	return forwardSharedPoolWithCapacityRetry(ctx, c, func() (*OpenAIForwardResult, error) {
		account, err := sharedPoolOpenAIAccount(accessKey)
		if err != nil {
			return nil, err
		}
		return s.Forward(ctx, c, account, body)
	})
}

func forwardSharedPoolWithCapacityRetry(
	ctx context.Context,
	c *gin.Context,
	forward func() (*OpenAIForwardResult, error),
) (*OpenAIForwardResult, error) {
	for attempt := 0; ; attempt++ {
		result, err := forward()
		if err == nil || attempt >= len(sharedPoolCapacityRetryDelays) || !isSafeSharedPoolCapacityRetry(c, err) {
			return result, err
		}

		timer := time.NewTimer(sharedPoolCapacityRetryDelays[attempt])
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return result, ctx.Err()
		case <-timer.C:
		}
	}
}

func isSafeSharedPoolCapacityRetry(c *gin.Context, err error) bool {
	if c != nil && c.Writer != nil && (c.Writer.Written() || IsResponseCommitted(c)) {
		return false
	}

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) || failoverErr == nil {
		return false
	}
	if !failoverErr.ShouldRetryNextAccount() {
		return false
	}
	switch failoverErr.StatusCode {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) ForwardSharedPoolImages(
	ctx context.Context,
	c *gin.Context,
	accessKey *SharedPoolAccessKey,
	body []byte,
	parsed *OpenAIImagesRequest,
) (*OpenAIForwardResult, error) {
	account, err := sharedPoolOpenAIAccount(accessKey)
	if err != nil {
		return nil, err
	}
	provider := strings.ToLower(strings.TrimSpace(accessKey.Provider))
	if provider == PlatformGrok || provider == "xai" {
		if account.Type == AccountTypeOAuth {
			return nil, errors.New("shared pool Grok OAuth media is not supported")
		}
		account.Platform = PlatformGrok
		endpoint := GrokMediaEndpointImagesGenerations
		if parsed != nil && parsed.IsEdits() {
			endpoint = GrokMediaEndpointImagesEdits
		}
		contentType := "application/json"
		if parsed != nil && strings.TrimSpace(parsed.ContentType) != "" {
			contentType = parsed.ContentType
		}
		return s.ForwardGrokMedia(ctx, c, account, endpoint, "", body, contentType)
	}
	if provider != "" && provider != PlatformOpenAI {
		return nil, fmt.Errorf("shared pool image provider %q is not supported", provider)
	}
	return s.ForwardImages(ctx, c, account, body, parsed, strings.TrimSpace(accessKey.UpstreamModelName))
}

func (s *OpenAIGatewayService) ForwardSharedPoolGrokVideo(
	ctx context.Context,
	c *gin.Context,
	accessKey *SharedPoolAccessKey,
	requestID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	provider := strings.ToLower(strings.TrimSpace(accessKey.Provider))
	if provider != PlatformGrok && provider != "xai" {
		return nil, fmt.Errorf("shared pool video provider %q is not supported", provider)
	}
	account, err := sharedPoolOpenAIAccount(accessKey)
	if err != nil {
		return nil, err
	}
	if account.Type == AccountTypeOAuth {
		return nil, errors.New("shared pool Grok OAuth video is not supported")
	}
	account.Platform = PlatformGrok
	endpoint := GrokMediaEndpointVideosGenerations
	if strings.TrimSpace(requestID) != "" {
		endpoint = GrokMediaEndpointVideoStatus
	}
	return s.ForwardGrokMedia(ctx, c, account, endpoint, strings.TrimSpace(requestID), body, contentType)
}

func sharedPoolOpenAIAccount(accessKey *SharedPoolAccessKey) (*Account, error) {
	if accessKey == nil {
		return nil, errors.New("shared pool access key is nil")
	}
	authType := strings.ToLower(strings.TrimSpace(accessKey.AuthType))
	if authType == "" || authType == "api_key" {
		authType = AccountTypeAPIKey
	}
	baseURL := strings.TrimRight(strings.TrimSpace(accessKey.UpstreamBaseURL), "/")
	credentials := map[string]any{}
	accountID := accessKey.AccountID
	if accountID <= 0 {
		accountID = accessKey.PoolID
	}
	proxyURL, proxyID, proxy, err := newSharedPoolRuntimeProxy(accessKey.ProxyURL, -accountID)
	if err != nil {
		return nil, err
	}
	extra := map[string]any{"shared_pool_proxy_url": proxyURL}
	if authType == AccountTypeOAuth {
		if len(accessKey.OAuthCredentials) == 0 || strings.TrimSpace(stringValue(accessKey.OAuthCredentials["access_token"])) == "" {
			return nil, errors.New("shared pool oauth credentials are not configured")
		}
		for key, value := range accessKey.OAuthCredentials {
			credentials[key] = value
		}
	} else {
		upstreamKey := strings.TrimSpace(accessKey.UpstreamAPIKey)
		if baseURL == "" || upstreamKey == "" {
			return nil, errors.New("shared pool upstream is not configured")
		}
		credentials["base_url"] = baseURL
		credentials["api_key"] = upstreamKey
		extra[openai_compat.ExtraKeyResponsesMode] = string(openai_compat.ResponsesSupportModeForceChatCompletions)
		extra["force_default_model_mapping"] = strings.TrimSpace(accessKey.UpstreamModelName) != ""
	}
	if upstreamModel := strings.TrimSpace(accessKey.UpstreamModelName); upstreamModel != "" {
		credentials["model_mapping"] = map[string]any{"*": upstreamModel}
	}
	return &Account{
		ID:                 -accountID,
		Name:               fmt.Sprintf("shared-pool-%d-account-%d", accessKey.PoolID, accountID),
		Platform:           PlatformOpenAI,
		Type:               authType,
		Credentials:        credentials,
		Extra:              extra,
		ProxyID:            proxyID,
		Proxy:              proxy,
		Concurrency:        accessKey.AccountConcurrency,
		Status:             StatusActive,
		Schedulable:        true,
		ExpiresAt:          accessKey.ExpiresAt,
		AutoPauseOnExpired: accessKey.ExpiresAt != nil,
	}, nil
}

type SharedPoolCanonicalGatewayContext struct {
	AccessKey     *SharedPoolAccessKey
	Identity      *SharedPoolCanonicalIdentity
	BillingPolicy corecontracts.BillingPolicy
	AcceptedQuote *SharedPoolPriceQuote
}

// ResolveSharedPoolCanonicalGatewayContext resolves product identity and an
// accepted quote before the request enters the ordinary Sub2 scheduler. It
// deliberately performs no account selection, slot acquisition, retry,
// forwarding, health mutation, or throughput accounting.
func (s *OpenAIGatewayService) ResolveSharedPoolCanonicalGatewayContext(
	ctx context.Context,
	apiKeyID int64,
	requestedModel string,
	responsesEndpoint bool,
	body []byte,
	compact bool,
) (*SharedPoolCanonicalGatewayContext, error) {
	if s == nil || s.bizDecipherService == nil || apiKeyID <= 0 {
		return nil, ErrSharedPoolIdentityQueryInvalid
	}

	var (
		accessKey *SharedPoolAccessKey
		err       error
	)
	canonicalText := !compact && !IsExplicitImageGenerationIntent(openAIResponsesEndpoint, requestedModel, body)
	if canonicalText {
		accessKey, err = s.bizDecipherService.getCanonicalSharedPoolAccessKey(ctx, apiKeyID, requestedModel)
		if err == nil && accessKey != nil && strings.TrimSpace(requestedModel) != "" {
			endpoint := SharedPoolEndpointChat
			if responsesEndpoint {
				endpoint = SharedPoolEndpointResponses
			}
			accessKey, err = s.attachSharedPoolAccessKeyQuote(ctx, accessKey, requestedModel, endpoint)
		}
	} else {
		accessKey, _, err = s.GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact(
			ctx, apiKeyID, requestedModel, responsesEndpoint, body, compact,
		)
	}
	if err != nil {
		return nil, err
	}
	if accessKey == nil || accessKey.OwnerID == nil || *accessKey.OwnerID <= 0 {
		return nil, ErrSharedPoolIdentityUnmapped
	}
	supply := &SharedPoolSupplyReference{
		Kind: SharedPoolSupplyPoolDefault,
		ID:   accessKey.PoolID,
	}
	if accessKey.AccountID > 0 {
		supply.Kind = SharedPoolSupplyPoolAccount
		supply.ID = accessKey.AccountID
	} else if canonicalText {
		supply = nil
	}
	identity, err := s.bizDecipherService.ResolveSharedPoolCanonicalIdentity(ctx, SharedPoolIdentityQuery{
		PoolID:  accessKey.PoolID,
		OwnerID: *accessKey.OwnerID,
		Supply:  supply,
	})
	if err != nil {
		return nil, err
	}
	return &SharedPoolCanonicalGatewayContext{
		AccessKey:     accessKey,
		Identity:      identity,
		BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
		AcceptedQuote: accessKey.PriceQuote,
	}, nil
}
func (s *OpenAIGatewayService) GetSharedPoolAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil || apiKeyID <= 0 {
		return nil, nil
	}
	return s.bizDecipherService.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
}

func (s *OpenAIGatewayService) GetSharedPoolCompactAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil || apiKeyID <= 0 || strings.TrimSpace(reqModel) == "" {
		return nil, nil
	}
	return s.bizDecipherService.GetSharedPoolCompactAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
}

// GetSharedPoolOpenAIRequestQuoteByAPIKeyID resolves the endpoint that must
// authorize an OpenAI text-protocol request. A Responses request that can invoke
// the native image_generation tool is media work even though its HTTP path is
// /v1/responses, so it must use the endpoint-specific media route, current probe
// evidence and media price instead of inheriting the ordinary Responses quote.
//
// Responses does not expose a trustworthy upper bound on the number of hosted
// image tool invocations. Only a fixed per-request media quote is therefore
// safe to reserve on this transport; token or per-image quotes fail closed.
func (s *OpenAIGatewayService) GetSharedPoolOpenAIRequestQuoteByAPIKeyID(
	ctx context.Context,
	apiKeyID int64,
	reqModel string,
	responsesEndpoint bool,
	body []byte,
) (*SharedPoolAccessKey, string, error) {
	return s.GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact(
		ctx,
		apiKeyID,
		reqModel,
		responsesEndpoint,
		body,
		false,
	)
}

// GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact keeps the ordinary
// Responses/media routing behavior intact while giving the exact compact path
// a model-bound OAuth scheduler. A missing capable route falls back to the
// ordinary route only so the handler's second capability check can return the
// stable compact_not_supported response before reservation or forwarding.
func (s *OpenAIGatewayService) GetSharedPoolOpenAIRequestQuoteByAPIKeyIDWithCompact(
	ctx context.Context,
	apiKeyID int64,
	reqModel string,
	responsesEndpoint bool,
	body []byte,
	compact bool,
) (*SharedPoolAccessKey, string, error) {
	endpointType := SharedPoolEndpointChat
	if responsesEndpoint {
		endpointType = SharedPoolEndpointResponses
	}
	if compact {
		// A compact body may contain historical tool definitions, including
		// image_generation. Compact is always a text Responses operation and
		// must never inherit media routing or media pricing from that history.
		endpointType = SharedPoolEndpointResponses
		accessKey, err := s.GetSharedPoolCompactAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
		if err != nil {
			return nil, endpointType, err
		}
		if accessKey != nil {
			accessKey, err = s.attachSharedPoolAccessKeyQuote(ctx, accessKey, reqModel, endpointType)
			return accessKey, endpointType, err
		}
		// Resolve an ordinary raw route only to give the handler enough account
		// identity to return compact_not_supported. Do not touch pricing for a
		// route that is guaranteed to be rejected before reservation.
		accessKey, err = s.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
		return accessKey, endpointType, err
	}
	if !responsesEndpoint || !IsExplicitImageGenerationIntent(openAIResponsesEndpoint, reqModel, body) {
		accessKey, err := s.GetSharedPoolAccessKeyQuoteByAPIKeyID(ctx, apiKeyID, reqModel, endpointType)
		return accessKey, endpointType, err
	}

	endpointType = SharedPoolEndpointImageGeneration
	accessKey, err := s.GetSharedPoolMediaAccessKeyQuoteByAPIKeyID(
		ctx, apiKeyID, reqModel, endpointType, "", "",
	)
	if err != nil {
		return nil, endpointType, err
	}
	if accessKey == nil || accessKey.PriceQuote == nil {
		return nil, endpointType, fmt.Errorf("%w: image_generation endpoint is not currently eligible", ErrSharedPoolPricingNotConfigured)
	}
	quote := accessKey.PriceQuote
	if normalizeSharedPoolEndpoint(quote.EndpointType) != endpointType {
		return nil, endpointType, fmt.Errorf("%w: image_generation endpoint quote is missing", ErrSharedPoolPricingNotConfigured)
	}
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return nil, endpointType, fmt.Errorf("%w: image_generation: %v", ErrSharedPoolPricingNotConfigured, err)
	}
	if !strings.EqualFold(strings.TrimSpace(quote.BasePrice.BillingMode), "per_request") {
		return nil, endpointType, fmt.Errorf("%w: Responses image_generation requires a fixed per-request media price", ErrSharedPoolUnsafeCostEstimate)
	}
	return accessKey, endpointType, nil
}

func (s *OpenAIGatewayService) GetSharedPoolAccessKeyQuoteByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, endpointType string) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil || apiKeyID <= 0 {
		return nil, nil
	}
	accessKey, err := s.bizDecipherService.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	return s.attachSharedPoolAccessKeyQuote(ctx, accessKey, reqModel, endpointType)
}

func (s *OpenAIGatewayService) GetSharedPoolCompactAccessKeyQuoteByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, endpointType string) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil || apiKeyID <= 0 || strings.TrimSpace(reqModel) == "" {
		return nil, nil
	}
	accessKey, err := s.bizDecipherService.GetSharedPoolCompactAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	return s.attachSharedPoolAccessKeyQuote(ctx, accessKey, reqModel, endpointType)
}

func (s *OpenAIGatewayService) attachSharedPoolAccessKeyQuote(ctx context.Context, accessKey *SharedPoolAccessKey, reqModel, endpointType string) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil || accessKey == nil {
		return accessKey, nil
	}
	modelName := strings.TrimSpace(accessKey.PublishedModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(reqModel)
	}
	quote, err := s.bizDecipherService.ResolveSharedPoolPriceQuote(ctx, accessKey.PoolID, modelName, endpointType)
	billingModelName := strings.TrimSpace(accessKey.CanonicalModelName)
	officialPriceRequired := errors.Is(err, ErrSharedPoolOfficialPriceRequired) ||
		(err == nil && quote != nil && quote.PricingSource == SharedPoolPricingSourceOfficial)
	if officialPriceRequired && billingModelName != "" && s.billingService != nil {
		if official, pricingErr := s.billingService.GetModelPricing(billingModelName); pricingErr == nil && validCanonicalSharedPoolModelPricing(official) {
			base := sharedPoolOfficialBasePrice(official)
			quote, err = s.bizDecipherService.EnsureSharedPoolOfficialPriceQuote(ctx, accessKey.PoolID, modelName, endpointType, base)
			if err == nil {
				quote = sharedPoolQuoteWithOfficialPolicy(quote, official)
			}
		} else if pricingErr != nil {
			err = pricingErr
		} else {
			err = ErrModelPricingUnavailable
		}
	} else if officialPriceRequired && billingModelName == "" {
		// Never fall back to a provider-less/published name. Without the
		// provider-scoped model_catalog identity, an owner-defined model could
		// accidentally inherit an unrelated official model's price.
		err = ErrSharedPoolPricingUnavailable
	}
	if err != nil {
		if errors.Is(err, ErrSharedPoolPricingUnavailable) || errors.Is(err, ErrSharedPoolOfficialPriceRequired) {
			return nil, fmt.Errorf("%w: %s", ErrSharedPoolPricingNotConfigured, strings.TrimSpace(reqModel))
		}
		return nil, err
	}
	quote = sharedPoolQuoteWithEffectiveMultiplier(quote, accessKey.RateMultiplier)
	accessKey.PriceQuote = quote
	return accessKey, nil
}

// GetSharedPoolMediaAccessKeyQuoteByAPIKeyID freezes the exact media unit
// price for this request. Official prices are derived only from the
// provider-scoped canonical model selected by the repository; the published
// name is never used as a provider-less fallback.
func (s *OpenAIGatewayService) GetSharedPoolMediaAccessKeyQuoteByAPIKeyID(
	ctx context.Context,
	apiKeyID int64,
	reqModel string,
	endpointType string,
	imageSize string,
	videoResolution string,
) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil || apiKeyID <= 0 {
		return nil, nil
	}
	accessKey, err := s.bizDecipherService.GetSharedPoolMediaAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel, endpointType)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	modelName := strings.TrimSpace(accessKey.PublishedModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(reqModel)
	}
	quote, err := s.bizDecipherService.ResolveSharedPoolPriceQuote(ctx, accessKey.PoolID, modelName, endpointType)
	officialPriceRequired := errors.Is(err, ErrSharedPoolOfficialPriceRequired) ||
		(err == nil && quote != nil && quote.PricingSource == SharedPoolPricingSourceOfficial)
	if officialPriceRequired {
		if variantErr := validateSharedPoolOfficialMediaVariant(endpointType, imageSize, videoResolution); variantErr != nil {
			return nil, variantErr
		}
		canonicalModel := strings.TrimSpace(accessKey.CanonicalModelName)
		if canonicalModel == "" || s.billingService == nil {
			return nil, fmt.Errorf("%w: %s", ErrSharedPoolPricingNotConfigured, strings.TrimSpace(reqModel))
		}
		base, priceErr := s.sharedPoolOfficialMediaBasePrice(canonicalModel, endpointType, imageSize, videoResolution)
		if priceErr != nil {
			return nil, priceErr
		}
		quote, err = s.bizDecipherService.EnsureSharedPoolOfficialPriceQuote(ctx, accessKey.PoolID, modelName, endpointType, base)
	}
	if err != nil {
		if errors.Is(err, ErrSharedPoolPricingUnavailable) || errors.Is(err, ErrSharedPoolOfficialPriceRequired) {
			return nil, fmt.Errorf("%w: %s", ErrSharedPoolPricingNotConfigured, strings.TrimSpace(reqModel))
		}
		return nil, err
	}
	quote = sharedPoolQuoteWithEffectiveMultiplier(quote, accessKey.RateMultiplier)
	accessKey.PriceQuote = quote
	return accessKey, nil
}

func (s *OpenAIGatewayService) sharedPoolOfficialMediaBasePrice(canonicalModel, endpointType, imageSize, videoResolution string) (SharedPoolPriceComponents, error) {
	endpointType = normalizeSharedPoolEndpoint(endpointType)
	if err := validateSharedPoolOfficialMediaVariant(endpointType, imageSize, videoResolution); err != nil {
		return SharedPoolPriceComponents{}, err
	}
	switch endpointType {
	case SharedPoolEndpointImageGeneration, SharedPoolEndpointImageEdit:
		cost := s.billingService.CalculateImageCost(canonicalModel, ImageBillingSize1K, 1, nil, 1)
		if cost == nil || cost.TotalCost <= 0 || math.IsNaN(cost.TotalCost) || math.IsInf(cost.TotalCost, 0) {
			return SharedPoolPriceComponents{}, fmt.Errorf("%w: official image unit price is unavailable", ErrSharedPoolPricingNotConfigured)
		}
		unit := cost.TotalCost
		return SharedPoolPriceComponents{BillingMode: "image", Currency: "USD", ImageItemPrice: &unit}, nil
	case SharedPoolEndpointVideo:
		cost := s.billingService.CalculateVideoCost(canonicalModel, VideoBillingResolution480P, 1, 1, nil, 1)
		if cost == nil || cost.TotalCost <= 0 || math.IsNaN(cost.TotalCost) || math.IsInf(cost.TotalCost, 0) {
			return SharedPoolPriceComponents{}, fmt.Errorf("%w: official video unit price is unavailable", ErrSharedPoolPricingNotConfigured)
		}
		unit := cost.TotalCost
		return SharedPoolPriceComponents{BillingMode: "video", Currency: "USD", VideoSecondPrice: &unit}, nil
	default:
		return SharedPoolPriceComponents{}, ErrSharedPoolPricingUnavailable
	}
}

func validateSharedPoolOfficialMediaVariant(endpointType, imageSize, videoResolution string) error {
	switch normalizeSharedPoolEndpoint(endpointType) {
	case SharedPoolEndpointImageGeneration, SharedPoolEndpointImageEdit:
		if NormalizeImageBillingTierOrDefault(imageSize) != ImageBillingSize1K {
			return fmt.Errorf("%w: official shared-pool images support only 1K", ErrSharedPoolOfficialMediaVariantUnsupported)
		}
	case SharedPoolEndpointVideo:
		if NormalizeVideoBillingResolutionOrDefault(videoResolution) != VideoBillingResolution480P {
			return fmt.Errorf("%w: official shared-pool videos support only 480p", ErrSharedPoolOfficialMediaVariantUnsupported)
		}
	default:
		return ErrSharedPoolPricingUnavailable
	}
	return nil
}

func sharedPoolOfficialBasePrice(pricing *ModelPricing) SharedPoolPriceComponents {
	if pricing == nil {
		return SharedPoolPriceComponents{}
	}
	inputPrice := pricing.InputPricePerToken
	outputPrice := pricing.OutputPricePerToken
	cacheReadPrice := pricing.CacheReadPricePerToken
	cacheWritePrice := pricing.CacheCreationPricePerToken
	return SharedPoolPriceComponents{
		BillingMode: "token", Currency: "USD",
		InputPrice: &inputPrice, OutputPrice: &outputPrice,
		CacheReadPrice: &cacheReadPrice, CacheWritePrice: &cacheWritePrice,
	}
}

func sharedPoolQuoteWithOfficialPolicy(quote *SharedPoolPriceQuote, pricing *ModelPricing) *SharedPoolPriceQuote {
	if quote == nil || pricing == nil || pricing.LongContextInputThreshold <= 0 ||
		(pricing.LongContextInputMultiplier <= 1 && pricing.LongContextOutputMultiplier <= 1) {
		return quote
	}
	inputMultiplier := pricing.LongContextInputMultiplier
	if inputMultiplier <= 0 {
		inputMultiplier = 1
	}
	outputMultiplier := pricing.LongContextOutputMultiplier
	if outputMultiplier <= 0 {
		outputMultiplier = 1
	}
	cloned := *quote
	cloned.LongContextInputThreshold = pricing.LongContextInputThreshold
	cloned.LongContextInputMultiplier = inputMultiplier
	cloned.LongContextOutputMultiplier = outputMultiplier
	cloned.PriceHash = ""
	return &cloned
}

// MarkSharedPoolUsageForwarding persists the point of no automatic refund. It
// must succeed immediately before the upstream request starts; otherwise the
// caller must not contact the upstream provider.
func (s *OpenAIGatewayService) MarkSharedPoolUsageForwarding(ctx context.Context, accessKeyID int64, requestID string) error {
	if s == nil || s.bizDecipherService == nil {
		return errors.New("shared pool billing is unavailable")
	}
	return s.bizDecipherService.MarkSharedPoolUsageForwarding(ctx, accessKeyID, requestID)
}

func (s *OpenAIGatewayService) MarkSharedPoolUsageReviewRequired(ctx context.Context, accessKeyID int64, requestID, reason string) error {
	if s == nil || s.bizDecipherService == nil {
		return errors.New("shared pool billing is unavailable")
	}
	return s.bizDecipherService.MarkSharedPoolUsageReviewRequired(ctx, accessKeyID, requestID, reason)
}

func (s *OpenAIGatewayService) ReleaseSharedPoolUsageAfterVerifiedFailure(ctx context.Context, accessKeyID int64, requestID, reason string) error {
	if s == nil || s.bizDecipherService == nil {
		return errors.New("shared pool billing is unavailable")
	}
	return s.bizDecipherService.ReleaseSharedPoolUsageAfterVerifiedFailure(ctx, accessKeyID, requestID, reason)
}

func (s *OpenAIGatewayService) RecordSharedPoolOpenAIUsage(ctx context.Context, apiKey *APIKey, accessKey *SharedPoolAccessKey, result *OpenAIForwardResult, requestID string) error {
	if s == nil || s.bizDecipherService == nil || apiKey == nil || accessKey == nil {
		return nil
	}
	if result == nil {
		return s.bizDecipherService.RecordSharedPoolUsage(ctx, SharedPoolUsageInput{
			AccessKeyID: accessKey.ID, PoolID: accessKey.PoolID, AccountID: accessKey.AccountID,
			UserID: accessKey.UserID, RequestID: requestID, Success: false,
			PriceVersionID: sharedPoolQuoteVersionID(accessKey.PriceQuote),
			PricingSource:  sharedPoolQuoteSource(accessKey.PriceQuote),
			PriceSnapshot:  MarshalSharedPoolPriceSnapshot(accessKey.PriceQuote),
		})
	}
	if strings.TrimSpace(requestID) == "" {
		requestID = resolveUsageBillingRequestID(ctx, result.RequestID)
	}

	ApplyOpenAIImageBillingResolution(result)

	totalInputTokens := result.Usage.InputTokens
	if result.Usage.ImageInputTokens > totalInputTokens {
		totalInputTokens = result.Usage.ImageInputTokens
	}
	actualInputTokens := totalInputTokens - result.Usage.CacheReadInputTokens - result.Usage.CacheCreationInputTokens
	if actualInputTokens < 0 {
		actualInputTokens = 0
	}
	tokens := UsageTokens{
		InputTokens:         actualInputTokens,
		ImageInputTokens:    result.Usage.ImageInputTokens,
		OutputTokens:        result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
		ImageOutputTokens:   result.Usage.ImageOutputTokens,
	}
	billingModel := strings.TrimSpace(result.BillingModel)
	if billingModel == "" {
		billingModel = forwardResultBillingModel(result.Model, result.UpstreamModel)
	}
	quote := accessKey.PriceQuote
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return fmt.Errorf("%w: %s", ErrSharedPoolPricingNotConfigured, billingModel)
	}
	if err := validateSharedPoolForwardResultForQuote(quote, result); err != nil {
		return err
	}
	actualCost, err := calculateSharedPoolQuoteCost(quote, tokens)
	if err != nil {
		return err
	}
	if actualCost <= 0 && !quote.ExplicitFree {
		return fmt.Errorf("%w: accepted non-free quote produced zero cost", ErrSharedPoolPricingNotConfigured)
	}
	usage := SharedPoolUsageInput{
		AccessKeyID:    accessKey.ID,
		PoolID:         accessKey.PoolID,
		AccountID:      accessKey.AccountID,
		UserID:         accessKey.UserID,
		Cost:           actualCost,
		Model:          sharedPoolUsageModel(accessKey, result, billingModel),
		RequestID:      requestID,
		Success:        true,
		PriceVersionID: quote.PriceVersionID,
		PricingSource:  quote.PricingSource,
		PriceSnapshot:  MarshalSharedPoolPriceSnapshot(quote),
	}
	// Persist the complete settlement payload before touching the final money
	// ledger. If the process exits after this point, the background worker can
	// replay the same immutable payload idempotently.
	if err := s.bizDecipherService.StageSharedPoolUsageSettlement(ctx, usage); err != nil {
		return err
	}
	return s.bizDecipherService.RecordSharedPoolUsage(ctx, usage)
}

// RecordSharedPoolMediaUsage settles verified media output only. Missing
// terminal metadata or output beyond the pre-authorized request boundary is
// deliberately returned to the caller for review_required handling; it is
// never silently released or capped.
func (s *OpenAIGatewayService) RecordSharedPoolMediaUsage(
	ctx context.Context,
	apiKey *APIKey,
	accessKey *SharedPoolAccessKey,
	endpointType string,
	result *OpenAIForwardResult,
	requestID string,
	reservedUnits int,
) error {
	if s == nil || s.bizDecipherService == nil || apiKey == nil || accessKey == nil {
		return errors.New("shared pool media billing is unavailable")
	}
	if result == nil || reservedUnits <= 0 {
		return ErrSharedPoolUsageUnavailable
	}
	endpointType = normalizeSharedPoolEndpoint(endpointType)
	actualUnits := 0
	switch endpointType {
	case SharedPoolEndpointImageGeneration, SharedPoolEndpointImageEdit:
		if !result.ImageCountObserved {
			return ErrSharedPoolUsageUnavailable
		}
		actualUnits = result.ImageCount
	case SharedPoolEndpointVideo:
		if result.VideoCount <= 0 {
			return ErrSharedPoolUsageUnavailable
		}
		actualUnits = result.VideoDurationSeconds
	default:
		return ErrSharedPoolPricingUnavailable
	}
	if actualUnits <= 0 {
		return ErrSharedPoolUsageUnavailable
	}
	if actualUnits > reservedUnits {
		return fmt.Errorf("%w: observed media units %d exceed reserved units %d", ErrSharedPoolUnsafeCostEstimate, actualUnits, reservedUnits)
	}
	quote := accessKey.PriceQuote
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return fmt.Errorf("%w: %v", ErrSharedPoolPricingNotConfigured, err)
	}
	actualCost, err := calculateSharedPoolMediaCharge(quote, endpointType, actualUnits)
	if err != nil {
		return err
	}
	if actualCost <= 0 && !quote.ExplicitFree {
		return fmt.Errorf("%w: accepted non-free media quote produced zero cost", ErrSharedPoolPricingNotConfigured)
	}
	model := sharedPoolUsageModel(accessKey, result, result.BillingModel)
	usage := SharedPoolUsageInput{
		AccessKeyID: accessKey.ID, PoolID: accessKey.PoolID, AccountID: accessKey.AccountID,
		UserID: accessKey.UserID, Cost: actualCost, Model: model, RequestID: requestID,
		Success: true, PriceVersionID: quote.PriceVersionID, PricingSource: quote.PricingSource,
		PriceSnapshot: MarshalSharedPoolPriceSnapshot(quote),
	}
	if err := s.bizDecipherService.StageSharedPoolUsageSettlement(ctx, usage); err != nil {
		return err
	}
	return s.bizDecipherService.RecordSharedPoolUsage(ctx, usage)
}

func validateSharedPoolForwardResultForQuote(quote *SharedPoolPriceQuote, result *OpenAIForwardResult) error {
	if quote == nil || result == nil || quote.BasePrice.BillingMode != "token" {
		return nil
	}
	if result.Usage.ImageOutputTokens > 0 ||
		result.ImageCount > 0 || result.VideoCount > 0 || result.WebSearchCalls > 0 {
		return fmt.Errorf("%w: token-priced result contains media or hosted-tool usage without complete price components", ErrSharedPoolUnsafeCostEstimate)
	}
	totalTokens := result.Usage.InputTokens + result.Usage.OutputTokens +
		result.Usage.CacheCreationInputTokens + result.Usage.CacheReadInputTokens
	if totalTokens <= 0 && !quote.ExplicitFree {
		return ErrSharedPoolUsageUnavailable
	}
	return nil
}

func calculateSharedPoolQuoteCost(quote *SharedPoolPriceQuote, tokens UsageTokens) (float64, error) {
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return 0, err
	}
	base := quote.BasePrice
	var cost float64
	switch base.BillingMode {
	case "token":
		cacheReadPrice := base.CacheReadPrice
		if cacheReadPrice == nil {
			cacheReadPrice = base.InputPrice
		}
		cacheWritePrice := base.CacheWritePrice
		if cacheWritePrice == nil {
			cacheWritePrice = base.InputPrice
		}
		inputCost := float64(tokens.InputTokens)*derefSharedPoolPrice(base.InputPrice) +
			float64(tokens.CacheReadTokens)*derefSharedPoolPrice(cacheReadPrice) +
			float64(tokens.CacheCreationTokens)*derefSharedPoolPrice(cacheWritePrice)
		outputCost := float64(tokens.OutputTokens) * derefSharedPoolPrice(base.OutputPrice)
		if sharedPoolQuoteUsesLongContextPricing(quote, tokens) {
			inputCost *= quote.LongContextInputMultiplier
			outputCost *= quote.LongContextOutputMultiplier
		}
		cost = inputCost + outputCost
	case "per_request":
		cost = derefSharedPoolPrice(base.PerRequestPrice)
	default:
		return 0, ErrSharedPoolPricingUnavailable
	}
	cost *= quote.Multiplier
	cost = applySharedPoolChargeBounds(cost, base.MinimumCharge, base.MaximumCharge)
	return sharedPoolQuoteBuyerCharge(quote, cost), nil
}

func sharedPoolQuoteUsesLongContextPricing(quote *SharedPoolPriceQuote, tokens UsageTokens) bool {
	if quote == nil || quote.LongContextInputThreshold <= 0 {
		return false
	}
	totalInputTokens := tokens.InputTokens + tokens.CacheCreationTokens + tokens.CacheReadTokens
	return totalInputTokens > quote.LongContextInputThreshold
}

func derefSharedPoolPrice(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func sharedPoolQuoteVersionID(quote *SharedPoolPriceQuote) int64 {
	if quote == nil {
		return 0
	}
	return quote.PriceVersionID
}

func sharedPoolQuoteSource(quote *SharedPoolPriceQuote) string {
	if quote == nil {
		return ""
	}
	return quote.PricingSource
}

func sharedPoolUsageModel(accessKey *SharedPoolAccessKey, result *OpenAIForwardResult, billingModel string) string {
	if accessKey != nil {
		if model := strings.TrimSpace(accessKey.PublishedModelName); model != "" {
			return model
		}
	}
	if result != nil {
		if model := strings.TrimSpace(result.Model); model != "" {
			return model
		}
	}
	return strings.TrimSpace(billingModel)
}
