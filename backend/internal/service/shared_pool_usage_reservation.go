package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	sharedPoolDefaultMaxOutputTokens = int64(8192)
	sharedPoolMaximumOutputTokens    = int64(32768)
	// Responses forwarding may add Codex compatibility instructions after this
	// pre-authorization step. Keep a conservative byte/token ceiling above the
	// largest current injected template so the eventual upstream body is covered.
	sharedPoolInputTokenOverhead = int64(32768)
	// Remote image URLs do not reveal their eventual vision-token footprint.
	// Reserve a deliberately high per-image allowance, capped at a full large
	// context window, while data URLs remain additionally covered by body bytes.
	sharedPoolVisionTokensPerImage = int64(32768)
	sharedPoolVisionTokenCeiling   = int64(1048576)
	// Responses continuation requests reference server-side history that is not
	// present in the wire body. Reserve a full large context window rather than
	// rejecting normal Codex/OpenAI continuation requests before forwarding.
	// Generated media and hosted tools remain separate fail-closed categories.
	sharedPoolContinuationTokenCeiling = int64(1048576)
	sharedPoolReservationTTL           = 2 * time.Hour
)

var (
	ErrSharedPoolReservationConflict  = errors.New("shared pool reservation conflicts with an existing request")
	ErrSharedPoolReservationFinalized = errors.New("shared pool reservation is already finalized")
	ErrSharedPoolUnsafeCostEstimate   = errors.New("shared pool request cost cannot be safely estimated")
	ErrSharedPoolOutputLimitTooHigh   = errors.New("shared pool output limit exceeds the supported maximum")
)

type SharedPoolUsageReservationInput struct {
	RequestID          string
	RequestFingerprint string
	AccessKeyID        int64
	PoolID             int64
	AccountID          int64
	UserID             int64
	PriceVersionID     int64
	EndpointType       string
	Model              string
	PricingSource      string
	PriceSnapshot      json.RawMessage
	HoldAmount         float64
	ExpiresAt          time.Time
}

type SharedPoolUsageReservation struct {
	ID            int64
	RequestID     string
	AccessKeyID   int64
	PoolID        int64
	AccountID     int64
	UserID        int64
	HoldAmount    float64
	SettledAmount float64
	Status        string
	ExpiresAt     time.Time
}

type SharedPoolUsageReservationRecoverySummary struct {
	Scanned           int
	Released          int
	ReviewRequired    int
	AutoReleased      int
	AutoReleaseFailed int
	PendingScanned    int
	Settled           int
	Failed            int
	CanonicalScanned  int
	CanonicalSettled  int
	CanonicalDeferred int
	CanonicalFailed   int
}

type SharedPoolUsageReviewAutoReleaseSummary struct {
	Scanned  int
	Released int
	Failed   int
}

type sharedPoolUsageReservationRepository interface {
	ReserveSharedPoolUsageTx(ctx context.Context, input SharedPoolUsageReservationInput) (*SharedPoolUsageReservation, error)
	MarkSharedPoolUsageForwardingTx(ctx context.Context, accessKeyID int64, requestID string) error
	MarkSharedPoolUsageReviewRequiredTx(ctx context.Context, accessKeyID int64, requestID, reason string) error
	ReleaseSharedPoolUsageAfterVerifiedFailureTx(ctx context.Context, accessKeyID int64, requestID, reason string) error
	StageSharedPoolUsageSettlementTx(ctx context.Context, input SharedPoolUsageInput) error
	ListPendingSharedPoolUsageSettlements(ctx context.Context, limit int) ([]SharedPoolUsageInput, error)
	RecoverExpiredSharedPoolUsageReservationsTx(ctx context.Context, now time.Time, limit int) (*SharedPoolUsageReservationRecoverySummary, error)
}

type sharedPoolUsageReviewAutoReleaseRepository interface {
	AutoReleaseAgedSharedPoolUsageReviewsTx(
		ctx context.Context,
		cutoff time.Time,
		maxHoldAmount float64,
		limit int,
	) (*SharedPoolUsageReviewAutoReleaseSummary, error)
}

type PreparedSharedPoolOpenAIRequest struct {
	Body        []byte
	RequestID   string
	HoldAmount  float64
	Reservation *SharedPoolUsageReservation
}

func (s *BizDecipherService) ReserveSharedPoolUsage(ctx context.Context, input SharedPoolUsageReservationInput) (*SharedPoolUsageReservation, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("shared pool reservation service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolUsageReservationRepository)
	if !ok {
		return nil, errors.New("shared pool reservation repository is unavailable")
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.RequestFingerprint = strings.ToLower(strings.TrimSpace(input.RequestFingerprint))
	input.EndpointType = normalizeSharedPoolEndpoint(input.EndpointType)
	input.Model = strings.TrimSpace(input.Model)
	input.PricingSource = strings.TrimSpace(input.PricingSource)
	if input.RequestID == "" || len(input.RequestID) > 160 {
		return nil, errors.New("shared pool reservation request id is invalid")
	}
	if len(input.RequestFingerprint) != 64 {
		return nil, errors.New("shared pool reservation fingerprint is invalid")
	}
	if input.AccessKeyID <= 0 || input.PoolID <= 0 || input.UserID <= 0 || input.PriceVersionID <= 0 || input.EndpointType == "" {
		return nil, errors.New("shared pool reservation identity is invalid")
	}
	if input.HoldAmount < 0 || math.IsNaN(input.HoldAmount) || math.IsInf(input.HoldAmount, 0) {
		return nil, errors.New("shared pool reservation amount is invalid")
	}
	if len(input.PriceSnapshot) == 0 {
		input.PriceSnapshot = json.RawMessage(`{}`)
	}
	if input.ExpiresAt.IsZero() {
		input.ExpiresAt = time.Now().UTC().Add(sharedPoolReservationTTL)
	}
	return repo.ReserveSharedPoolUsageTx(ctx, input)
}

func (s *BizDecipherService) RecoverExpiredSharedPoolUsageReservations(ctx context.Context, now time.Time, limit int) (*SharedPoolUsageReservationRecoverySummary, error) {
	if s == nil || s.repo == nil {
		return &SharedPoolUsageReservationRecoverySummary{}, nil
	}
	repo, ok := s.repo.(sharedPoolUsageReservationRepository)
	if !ok {
		return nil, errors.New("shared pool reservation repository is unavailable")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	summary, err := repo.RecoverExpiredSharedPoolUsageReservationsTx(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	pending, err := repo.ListPendingSharedPoolUsageSettlements(ctx, limit)
	if err != nil {
		return nil, err
	}
	summary.PendingScanned = len(pending)
	for _, input := range pending {
		if err := s.RecordSharedPoolUsage(ctx, input); err != nil {
			summary.Failed++
			continue
		}
		summary.Settled++
	}
	// Canonical shared-market text requests record a durable usage event but do
	// not charge on the request path. Drain those events through the same
	// member-debit / owner-credit settlement used above.
	canonicalSummary, canonicalErr := s.SettlePendingCanonicalSharedPoolUsage(ctx, limit)
	if canonicalErr != nil {
		return nil, canonicalErr
	}
	if canonicalSummary != nil {
		summary.CanonicalScanned = canonicalSummary.Scanned
		summary.CanonicalSettled = canonicalSummary.Settled
		summary.CanonicalDeferred = canonicalSummary.Deferred
		summary.CanonicalFailed = canonicalSummary.Failed
	}
	policy, err := s.GetSharedPoolUsageReviewPolicy(ctx)
	if err != nil {
		return nil, err
	}
	if policy.AutoReleaseEnabled && policy.AutoReleaseMaxHold > 0 {
		if releaser, ok := s.repo.(sharedPoolUsageReviewAutoReleaseRepository); ok && releaser != nil {
			autoSummary, autoErr := releaser.AutoReleaseAgedSharedPoolUsageReviewsTx(
				ctx,
				policy.Cutoff(now),
				policy.AutoReleaseMaxHold,
				limit,
			)
			if autoErr != nil {
				return nil, autoErr
			}
			if autoSummary != nil {
				summary.AutoReleased = autoSummary.Released
				summary.AutoReleaseFailed = autoSummary.Failed
				summary.Scanned += autoSummary.Scanned
				summary.Failed += autoSummary.Failed
			}
		}
	}
	return summary, nil
}

func (s *BizDecipherService) MarkSharedPoolUsageForwarding(ctx context.Context, accessKeyID int64, requestID string) error {
	if s == nil || s.repo == nil {
		return errors.New("shared pool reservation service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolUsageReservationRepository)
	if !ok {
		return errors.New("shared pool reservation repository is unavailable")
	}
	return repo.MarkSharedPoolUsageForwardingTx(ctx, accessKeyID, strings.TrimSpace(requestID))
}

// MarkSharedPoolUsageReviewRequired keeps the pre-authorized amount frozen
// when an upstream request may have performed work but no trustworthy terminal
// usage result is available. Only an audited review may release or capture it.
func (s *BizDecipherService) MarkSharedPoolUsageReviewRequired(ctx context.Context, accessKeyID int64, requestID, reason string) error {
	if s == nil || s.repo == nil {
		return errors.New("shared pool reservation service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolUsageReservationRepository)
	if !ok {
		return errors.New("shared pool reservation repository is unavailable")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "upstream_result_unknown"
	}
	return repo.MarkSharedPoolUsageReviewRequiredTx(ctx, accessKeyID, strings.TrimSpace(requestID), reason)
}

// ReleaseSharedPoolUsageAfterVerifiedFailure releases a forwarding hold only
// when the provider returned a terminal HTTP failure proving there is no
// successful response to charge. Timeouts and broken streams must not use it.
func (s *BizDecipherService) ReleaseSharedPoolUsageAfterVerifiedFailure(ctx context.Context, accessKeyID int64, requestID, reason string) error {
	if s == nil || s.repo == nil {
		return errors.New("shared pool reservation service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolUsageReservationRepository)
	if !ok {
		return errors.New("shared pool reservation repository is unavailable")
	}
	reason = strings.TrimSpace(reason)
	if accessKeyID <= 0 || strings.TrimSpace(requestID) == "" || !strings.HasPrefix(reason, "upstream_http_") {
		return errors.New("verified shared pool failure identity is invalid")
	}
	return repo.ReleaseSharedPoolUsageAfterVerifiedFailureTx(ctx, accessKeyID, strings.TrimSpace(requestID), reason)
}

func (s *BizDecipherService) StageSharedPoolUsageSettlement(ctx context.Context, input SharedPoolUsageInput) error {
	if s == nil || s.repo == nil {
		return errors.New("shared pool reservation service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolUsageReservationRepository)
	if !ok {
		return errors.New("shared pool reservation repository is unavailable")
	}
	if input.AccessKeyID <= 0 || strings.TrimSpace(input.RequestID) == "" {
		return errors.New("shared pool settlement identity is invalid")
	}
	if !input.Success {
		return errors.New("only successful shared pool usage can be staged for settlement")
	}
	if input.PriceVersionID <= 0 ||
		(input.PricingSource != SharedPoolPricingSourceOfficial && input.PricingSource != SharedPoolPricingSourceOwner) {
		return errors.New("shared pool settlement price snapshot is invalid")
	}
	if input.Cost < 0 || math.IsNaN(input.Cost) || math.IsInf(input.Cost, 0) {
		return errors.New("shared pool settlement amount is invalid")
	}
	if len(input.PriceSnapshot) == 0 {
		input.PriceSnapshot = json.RawMessage(`{}`)
	}
	input.Cost = normalizedSharedPoolMoney(input.Cost)
	return repo.StageSharedPoolUsageSettlementTx(ctx, input)
}

// PrepareSharedPoolOpenAIRequest applies a bounded output limit, computes a
// conservative maximum charge from the immutable quote, and atomically reserves
// that amount before an upstream request may start.
func (s *OpenAIGatewayService) PrepareSharedPoolOpenAIRequest(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
	endpointType string,
	body []byte,
) (*PreparedSharedPoolOpenAIRequest, error) {
	return s.PrepareSharedPoolOpenAIRequestWithCompact(ctx, accessKey, endpointType, body, false)
}

// PrepareSharedPoolOpenAIRequestWithCompact preserves the compact wire body
// byte-for-byte while still reserving against the conservative default output
// ceiling. The compact endpoint does not accept an injected max_output_tokens
// field, but that protocol difference must not weaken the billing hold.
func (s *OpenAIGatewayService) PrepareSharedPoolOpenAIRequestWithCompact(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
	endpointType string,
	body []byte,
	compact bool,
) (*PreparedSharedPoolOpenAIRequest, error) {
	if s == nil || s.bizDecipherService == nil || accessKey == nil {
		return nil, errors.New("shared pool billing is unavailable")
	}
	if err := ValidateSharedPoolPriceQuote(accessKey.PriceQuote); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSharedPoolPricingNotConfigured, err)
	}
	preparedBody, maxOutputTokens, unknownContext, err := prepareSharedPoolBoundedRequestBodyWithCompact(body, endpointType, compact)
	if err != nil {
		return nil, err
	}
	holdAmount, err := calculateSharedPoolMaximumHoldWithCompact(accessKey.PriceQuote, preparedBody, maxOutputTokens, unknownContext, compact)
	if err != nil {
		return nil, err
	}
	requestID := sharedPoolReservationRequestID(sharedPoolReservationAttemptBase(ctx, resolveUsageBillingRequestID(ctx, "")), accessKey.ID)
	priceSnapshot := MarshalSharedPoolPriceSnapshot(accessKey.PriceQuote)
	fingerprint := sharedPoolReservationFingerprint(
		requestID,
		accessKey,
		normalizeSharedPoolEndpoint(endpointType),
		preparedBody,
		priceSnapshot,
		holdAmount,
	)
	reservation, err := s.bizDecipherService.ReserveSharedPoolUsage(ctx, SharedPoolUsageReservationInput{
		RequestID:          requestID,
		RequestFingerprint: fingerprint,
		AccessKeyID:        accessKey.ID,
		PoolID:             accessKey.PoolID,
		AccountID:          accessKey.AccountID,
		UserID:             accessKey.UserID,
		PriceVersionID:     accessKey.PriceQuote.PriceVersionID,
		EndpointType:       normalizeSharedPoolEndpoint(endpointType),
		Model:              sharedPoolPublishedModel(accessKey),
		PricingSource:      accessKey.PriceQuote.PricingSource,
		PriceSnapshot:      priceSnapshot,
		HoldAmount:         holdAmount,
	})
	if err != nil {
		return nil, err
	}
	return &PreparedSharedPoolOpenAIRequest{
		Body: preparedBody, RequestID: requestID, HoldAmount: holdAmount, Reservation: reservation,
	}, nil
}

func prepareSharedPoolBoundedRequestBody(body []byte, endpointType string) ([]byte, int64, bool, error) {
	return prepareSharedPoolBoundedRequestBodyWithCompact(body, endpointType, false)
}

func prepareSharedPoolBoundedRequestBodyWithCompact(body []byte, endpointType string, compact bool) ([]byte, int64, bool, error) {
	if !gjson.ValidBytes(body) {
		return nil, 0, false, errors.New("shared pool request body must be valid JSON")
	}
	endpointType = normalizeSharedPoolEndpoint(endpointType)
	if endpointType == "" {
		return nil, 0, false, errors.New("shared pool endpoint is unsupported")
	}
	if compact && endpointType != SharedPoolEndpointResponses {
		return nil, 0, false, errors.New("shared pool compact requests require the responses endpoint")
	}

	root := gjson.ParseBytes(body)
	serviceTier := strings.ToLower(strings.TrimSpace(root.Get("service_tier").String()))
	if serviceTier != "" && serviceTier != "default" && serviceTier != "auto" {
		return nil, 0, false, fmt.Errorf("%w: service_tier %q is not supported by immutable shared-pool quotes", ErrSharedPoolUnsafeCostEstimate, serviceTier)
	}
	features := inspectSharedPoolRequestFeaturesWithCompact(root, compact)
	// Only previous_response_id/conversation continuations are bounded locally.
	// Hidden item references and stored prompts remain unpriced server state and
	// must stay fail-closed.
	unknownContext := features.unpricedMedia || features.hostedTool ||
		(features.serverManagedContext && !features.continuationContext)
	if n := root.Get("n"); n.Exists() && (n.Type != gjson.Number || n.Num != 1) {
		return nil, 0, false, fmt.Errorf("%w: shared-pool requests currently require n=1", ErrSharedPoolUnsafeCostEstimate)
	}

	fields := []string{"max_output_tokens", "max_completion_tokens", "max_tokens"}
	if endpointType == SharedPoolEndpointChat {
		fields = []string{"max_completion_tokens", "max_tokens", "max_output_tokens"}
	}
	var maxOutput int64
	var found bool
	var firstField string
	for _, field := range fields {
		value := root.Get(field)
		if !value.Exists() {
			continue
		}
		if value.Type != gjson.Number || value.Int() <= 0 || value.Num != float64(value.Int()) {
			return nil, 0, false, fmt.Errorf("%s must be a positive integer", field)
		}
		if found && value.Int() != maxOutput {
			return nil, 0, false, fmt.Errorf("conflicting output limits: %s and %s must match", firstField, field)
		}
		maxOutput = value.Int()
		firstField = field
		found = true
	}
	if !found {
		if compact {
			// The compact wire does not accept an injected output limit, so the
			// gateway cannot enforce the ordinary default upstream. Reserve the
			// full locally accepted ceiling instead; otherwise settlement could
			// be capped by an undersized hold and silently undercharge the pool.
			maxOutput = sharedPoolMaximumOutputTokens
		} else {
			maxOutput = sharedPoolDefaultMaxOutputTokens
			targetField := "max_output_tokens"
			if endpointType == SharedPoolEndpointChat {
				targetField = "max_tokens"
			}
			var err error
			body, err = sjson.SetBytes(body, targetField, maxOutput)
			if err != nil {
				return nil, 0, false, fmt.Errorf("set shared pool output limit: %w", err)
			}
		}
	}
	if maxOutput > sharedPoolMaximumOutputTokens {
		return nil, 0, false, fmt.Errorf("%w: requested %d, maximum %d", ErrSharedPoolOutputLimitTooHigh, maxOutput, sharedPoolMaximumOutputTokens)
	}
	return body, maxOutput, unknownContext, nil
}

type sharedPoolRequestFeatures struct {
	unpricedMedia        bool
	visionInputCount     int
	hostedTool           bool
	serverManagedContext bool
	continuationContext  bool
}

func inspectSharedPoolRequestFeatures(root gjson.Result) sharedPoolRequestFeatures {
	return inspectSharedPoolRequestFeaturesWithCompact(root, false)
}

// inspectSharedPoolRequestFeaturesWithCompact treats the top-level tools and
// tool_choice fields, plus completed hosted-tool call items in input, as
// passive history. Codex includes hosted tools such as image_generation even
// when no tool is being executed, and compact only summarizes the supplied
// history. Actual media references or server-managed context found anywhere in
// the payload remain fail-closed.
func inspectSharedPoolRequestFeaturesWithCompact(root gjson.Result, compact bool) sharedPoolRequestFeatures {
	var decoded any
	if err := json.Unmarshal([]byte(root.Raw), &decoded); err != nil {
		// prepareSharedPoolBoundedRequestBody validates JSON first. Treat an
		// unexpected second-pass decode failure as unknown context so token-priced
		// requests fail closed.
		return sharedPoolRequestFeatures{unpricedMedia: true, hostedTool: true, serverManagedContext: true}
	}
	features := sharedPoolRequestFeatures{}
	var walk func(any, int)
	walk = func(value any, depth int) {
		switch current := value.(type) {
		case map[string]any:
			kind := strings.ToLower(sharedPoolNormalizedMapString(current, "type"))
			visionBlock := isSharedPoolVisionInputType(kind)
			if visionBlock {
				features.visionInputCount++
			}
			if compact && depth > 0 {
				if isSharedPoolHostedToolType(kind) {
					if !strings.EqualFold(sharedPoolNormalizedMapString(current, "status"), "completed") {
						features.hostedTool = true
					}
				}
			}
			for rawKey, child := range current {
				key := strings.ToLower(strings.TrimSpace(rawKey))
				if compact && depth == 0 && (key == "tools" || key == "tool_choice") {
					continue
				}
				switch key {
				case "input_image", "image_url":
					if !visionBlock && sharedPoolFeatureValuePresent(child) {
						features.visionInputCount++
					}
				case "input_file", "file_id", "input_audio", "audio_url", "video_url", "image_generation":
					if sharedPoolFeatureValuePresent(child) {
						features.unpricedMedia = true
					}
				case "previous_response_id", "conversation":
					if sharedPoolFeatureValuePresent(child) {
						features.serverManagedContext = true
						features.continuationContext = true
					}
				case "prompt":
					if depth == 0 && sharedPoolFeatureValuePresent(child) {
						features.serverManagedContext = true
					}
				case "type", "tool_choice":
					if kind, ok := child.(string); ok {
						kind = strings.ToLower(strings.TrimSpace(kind))
						if kind == "item_reference" {
							features.serverManagedContext = true
						}
						if isSharedPoolVisionInputType(kind) {
							// Counted once at the containing content block above.
						} else if isSharedPoolMediaType(kind) {
							features.unpricedMedia = true
						}
						if !compact && isSharedPoolHostedToolType(kind) {
							features.hostedTool = true
						}
					}
				}
				walk(child, depth+1)
			}
		case []any:
			for _, child := range current {
				walk(child, depth+1)
			}
		}
	}
	walk(decoded, 0)
	return features
}

func sharedPoolFeatureValuePresent(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	case bool:
		return typed
	default:
		return true
	}
}

func sharedPoolNormalizedMapString(value map[string]any, wantedKey string) string {
	for rawKey, rawValue := range value {
		if !strings.EqualFold(strings.TrimSpace(rawKey), wantedKey) {
			continue
		}
		text, _ := rawValue.(string)
		return strings.TrimSpace(text)
	}
	return ""
}

func isSharedPoolMediaType(kind string) bool {
	switch kind {
	case "input_audio", "audio_url", "input_file", "video_url", "image_generation", "image_generation_call":
		return true
	default:
		return false
	}
}

func isSharedPoolVisionInputType(kind string) bool {
	switch kind {
	case "input_image", "image_url":
		return true
	default:
		return false
	}
}

func isSharedPoolHostedToolType(kind string) bool {
	return strings.HasPrefix(kind, "web_search") ||
		strings.HasPrefix(kind, "file_search") ||
		strings.HasPrefix(kind, "code_interpreter") ||
		strings.HasPrefix(kind, "computer_use") ||
		strings.HasPrefix(kind, "computer_") ||
		strings.HasPrefix(kind, "mcp_") ||
		kind == "image_generation" ||
		kind == "mcp"
}

func sharedPoolRequestHasUnboundedMedia(root gjson.Result) bool {
	return inspectSharedPoolRequestFeatures(root).unpricedMedia
}

func calculateSharedPoolMaximumHold(quote *SharedPoolPriceQuote, body []byte, maxOutputTokens int64, unknownContext bool) (float64, error) {
	return calculateSharedPoolMaximumHoldWithCompact(quote, body, maxOutputTokens, unknownContext, false)
}

func calculateSharedPoolMaximumHoldWithCompact(quote *SharedPoolPriceQuote, body []byte, maxOutputTokens int64, unknownContext, compact bool) (float64, error) {
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return 0, err
	}
	base := quote.BasePrice
	root := gjson.ParseBytes(body)
	features := inspectSharedPoolRequestFeaturesWithCompact(root, compact)
	if base.BillingMode == "token" {
		switch {
		case isSharedPoolLikelyMediaOutputModel(quote.ModelName):
			return 0, fmt.Errorf("%w: media-generation models require an explicit fixed per-request price", ErrSharedPoolUnsafeCostEstimate)
		case features.unpricedMedia:
			return 0, fmt.Errorf("%w: generated image, audio, file, or video work requires explicit price components", ErrSharedPoolUnsafeCostEstimate)
		case features.hostedTool:
			return 0, fmt.Errorf("%w: hosted tools require an explicit fixed per-request price", ErrSharedPoolUnsafeCostEstimate)
		}
	}
	if base.MaximumCharge != nil {
		return sharedPoolQuoteBuyerCharge(quote, *base.MaximumCharge), nil
	}
	if unknownContext && base.BillingMode == "token" {
		return 0, fmt.Errorf("%w: request context cannot be safely bounded", ErrSharedPoolUnsafeCostEstimate)
	}

	var hold float64
	switch base.BillingMode {
	case "per_request":
		hold = derefSharedPoolPrice(base.PerRequestPrice) * quote.Multiplier
	case "token":
		inputPrice := derefSharedPoolPrice(base.InputPrice)
		if base.CacheReadPrice != nil && *base.CacheReadPrice > inputPrice {
			inputPrice = *base.CacheReadPrice
		}
		if base.CacheWritePrice != nil && *base.CacheWritePrice > inputPrice {
			inputPrice = *base.CacheWritePrice
		}
		// A byte is a conservative upper bound for BPE token count. The fixed
		// overhead covers message framing and provider-specific tokenization.
		maxInputTokens := int64(len(bytes.TrimSpace(body))) + sharedPoolInputTokenOverhead
		if features.continuationContext && maxInputTokens < sharedPoolContinuationTokenCeiling {
			maxInputTokens = sharedPoolContinuationTokenCeiling
		}
		if features.visionInputCount > 0 {
			visionTokens := int64(features.visionInputCount) * sharedPoolVisionTokensPerImage
			if visionTokens > sharedPoolVisionTokenCeiling {
				visionTokens = sharedPoolVisionTokenCeiling
			}
			maxInputTokens += visionTokens
		}
		inputCost := float64(maxInputTokens) * inputPrice
		outputCost := float64(maxOutputTokens) * derefSharedPoolPrice(base.OutputPrice)
		if sharedPoolQuoteUsesLongContextPricing(quote, UsageTokens{InputTokens: int(maxInputTokens), OutputTokens: int(maxOutputTokens)}) {
			inputCost *= quote.LongContextInputMultiplier
			outputCost *= quote.LongContextOutputMultiplier
		}
		hold = (inputCost + outputCost) * quote.Multiplier
	default:
		return 0, ErrSharedPoolPricingUnavailable
	}
	hold = applySharedPoolChargeBounds(hold, base.MinimumCharge, base.MaximumCharge)
	if hold < 0 || math.IsNaN(hold) || math.IsInf(hold, 0) {
		return 0, ErrSharedPoolUnsafeCostEstimate
	}
	return sharedPoolQuoteBuyerCharge(quote, hold), nil
}

func isSharedPoolLikelyMediaOutputModel(modelName string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	for _, marker := range []string{
		"gpt-image", "dall-e", "sora", "video", "veo", "imagen", "imagegen",
		"audio", "whisper", "transcribe", "text-to-speech", "tts", "realtime",
	} {
		if strings.Contains(modelName, marker) {
			return true
		}
	}
	return false
}

func normalizedSharedPoolMoney(value float64) float64 {
	// users.balance, users.frozen_balance and the user-facing balance ledger are
	// NUMERIC(...,8). Quantize every debit, hold and payout source to that same
	// unit before the transaction so sub-cent fractions cannot mint owner income.
	return math.Ceil(value*1e8-1e-8) / 1e8
}

func withdrawableSharedPoolMoney(value float64) float64 {
	// Transfers enter users.balance, whose scale is eight decimals. Always round
	// down before debiting the higher-precision owner wallet so the destination
	// can never receive more than the wallet lost.
	return math.Floor(value*1e8+1e-8) / 1e8
}

func quantizedSharedPoolCharge(value float64, maximum *float64) float64 {
	quantized := normalizedSharedPoolMoney(value)
	if maximum == nil {
		return quantized
	}
	// A configured maximum is a user-visible hard ceiling. Round that ceiling
	// down to the balance ledger's 8-decimal unit, never above what was accepted.
	maxQuantized := math.Floor(*maximum*1e8+1e-8) / 1e8
	if quantized > maxQuantized {
		return maxQuantized
	}
	return quantized
}

func sharedPoolReservationRequestID(base string, accessKeyID int64) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(base)))
	return fmt.Sprintf("sp:%d:%s", accessKeyID, hex.EncodeToString(sum[:]))
}

func sharedPoolReservationFingerprint(
	requestID string,
	accessKey *SharedPoolAccessKey,
	endpointType string,
	body []byte,
	priceSnapshot json.RawMessage,
	holdAmount float64,
) string {
	payload := struct {
		RequestID      string          `json:"request_id"`
		AccessKeyID    int64           `json:"access_key_id"`
		PoolID         int64           `json:"pool_id"`
		UserID         int64           `json:"user_id"`
		EndpointType   string          `json:"endpoint_type"`
		BodyHash       string          `json:"body_hash"`
		PriceSnapshot  json.RawMessage `json:"price_snapshot"`
		HoldAmountText string          `json:"hold_amount"`
	}{
		RequestID: requestID, AccessKeyID: accessKey.ID, PoolID: accessKey.PoolID,
		UserID: accessKey.UserID, EndpointType: endpointType,
		BodyHash:       fmt.Sprintf("%x", sha256.Sum256(body)),
		PriceSnapshot:  priceSnapshot,
		HoldAmountText: fmt.Sprintf("%.12f", holdAmount),
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func sharedPoolPublishedModel(accessKey *SharedPoolAccessKey) string {
	if accessKey == nil {
		return ""
	}
	if model := strings.TrimSpace(accessKey.PublishedModelName); model != "" {
		return model
	}
	if accessKey.PriceQuote != nil {
		return strings.TrimSpace(accessKey.PriceQuote.ModelName)
	}
	return ""
}
