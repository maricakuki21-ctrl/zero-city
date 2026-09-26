package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
)

// CanonicalSharedPoolUsageClaim is one durable canonical usage event that the
// ledger billing policy recorded instead of charging. The event still has to be
// priced and turned into a member debit plus a pool-owner credit.
type CanonicalSharedPoolUsageClaim struct {
	EventID    string
	RequestID  string
	APIKeyID   int64
	Event      corecontracts.CanonicalUsageFinalized
	Settlement *CanonicalUsageSettlementSnapshot
}

const canonicalUsageSettlementKindSharedPoolQuote = "shared_pool_quote"

// CanonicalUsageSettlementSnapshot freezes the request-time accepted shared
// pool quote on the durable outbox row. The canonical usage event keeps its
// stable schema; pricing provenance is stored beside it instead of changing the
// event digest consumed by the ledger.
type CanonicalUsageSettlementSnapshot struct {
	Kind            string                `json:"kind"`
	SharedPoolQuote *SharedPoolPriceQuote `json:"shared_pool_quote,omitempty"`
}

func NewCanonicalUsageSettlementSnapshot(quote *SharedPoolPriceQuote) (*CanonicalUsageSettlementSnapshot, error) {
	if err := ValidateSharedPoolPriceQuote(quote); err != nil {
		return nil, err
	}
	cloned := cloneSharedPoolPriceQuote(quote)
	FinalizeSharedPoolPriceQuote(cloned)
	return &CanonicalUsageSettlementSnapshot{
		Kind:            canonicalUsageSettlementKindSharedPoolQuote,
		SharedPoolQuote: cloned,
	}, nil
}

func (s *CanonicalUsageSettlementSnapshot) Validate() error {
	if s == nil {
		return errors.New("canonical usage settlement snapshot is nil")
	}
	if s.Kind != canonicalUsageSettlementKindSharedPoolQuote || s.SharedPoolQuote == nil {
		return errors.New("canonical usage settlement snapshot kind is unsupported")
	}
	if err := ValidateSharedPoolPriceQuote(s.SharedPoolQuote); err != nil {
		return fmt.Errorf("canonical usage settlement quote is invalid: %w", err)
	}
	return nil
}

func MarshalCanonicalUsageSettlementSnapshot(snapshot *CanonicalUsageSettlementSnapshot) (json.RawMessage, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func CanonicalUsageSettlementSnapshotSHA256(snapshot *CanonicalUsageSettlementSnapshot) (string, error) {
	payload, err := MarshalCanonicalUsageSettlementSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// CanonicalSharedPoolSettlementSummary reports one drain pass. Scanned counts
// every pending event considered; Settled counts events that reached the money
// ledger; Deferred counts events intentionally left pending (for example an
// ambiguous or missing price) that must not be charged by guessing.
type CanonicalSharedPoolSettlementSummary struct {
	Scanned  int
	Settled  int
	Deferred int
	Failed   int
}

// CanonicalSharedPoolUsageRepository is the durable storage boundary for
// canonical shared-market usage that still needs settlement.
type CanonicalSharedPoolUsageRepository interface {
	ListPendingCanonicalSharedPoolUsage(ctx context.Context, limit int) ([]CanonicalSharedPoolUsageClaim, error)
	MarkCanonicalSharedPoolUsageSettled(ctx context.Context, eventID string) error
	ResolveCanonicalSharedPoolSettlementIdentity(ctx context.Context, claim CanonicalSharedPoolUsageClaim) (*SharedPoolAccessKey, error)
	HasPostedCanonicalSharedPoolSettlement(ctx context.Context, input SharedPoolUsageInput) (bool, error)
	DeferCanonicalSharedPoolSettlement(ctx context.Context, eventID string) error
}

// SettlePendingCanonicalSharedPoolUsage drains durable canonical usage events
// that the shared-market ledger policy recorded without charging. Each event is
// priced against the pool's immutable quote and settled through the same
// member-debit / owner-credit path used by request-time billing, so a retry is
// idempotent by request id.
//
// Pricing is resolved from the event protocol plus the access key binding. When
// the price cannot be determined unambiguously the event stays pending and is
// counted as deferred rather than being charged at a guessed rate.
func (s *BizDecipherService) SettlePendingCanonicalSharedPoolUsage(ctx context.Context, limit int) (*CanonicalSharedPoolSettlementSummary, error) {
	summary := &CanonicalSharedPoolSettlementSummary{}
	if s == nil || s.repo == nil {
		return summary, nil
	}
	repo, ok := s.repo.(CanonicalSharedPoolUsageRepository)
	if !ok {
		return summary, errors.New("canonical shared pool settlement repository is unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	claims, err := repo.ListPendingCanonicalSharedPoolUsage(ctx, limit)
	if err != nil {
		return summary, err
	}
	summary.Scanned = len(claims)
	for _, claim := range claims {
		switch err := s.settleOneCanonicalSharedPoolUsage(ctx, claim); {
		case err == nil:
			if markErr := repo.MarkCanonicalSharedPoolUsageSettled(ctx, claim.EventID); markErr != nil {
				summary.Failed++
				_ = repo.DeferCanonicalSharedPoolSettlement(ctx, claim.EventID)
				continue
			}
			summary.Settled++
		case errors.Is(err, errCanonicalSettlementDeferred):
			summary.Deferred++
			if retryErr := repo.DeferCanonicalSharedPoolSettlement(ctx, claim.EventID); retryErr != nil {
				return summary, retryErr
			}
		default:
			summary.Failed++
			slog.Error("canonical shared pool settlement failed", "event_id", claim.EventID, "error", err)
			if retryErr := repo.DeferCanonicalSharedPoolSettlement(ctx, claim.EventID); retryErr != nil {
				return summary, retryErr
			}
		}
	}
	return summary, nil
}

// errCanonicalSettlementDeferred marks an event that must stay pending because
// its price cannot be established without guessing.
var errCanonicalSettlementDeferred = errors.New("canonical shared pool settlement deferred")

func (s *BizDecipherService) settleOneCanonicalSharedPoolUsage(ctx context.Context, claim CanonicalSharedPoolUsageClaim) error {
	requestID := strings.TrimSpace(claim.RequestID)
	if requestID == "" || requestID != claim.Event.RequestID() || claim.EventID != claim.Event.EventID() {
		return fmt.Errorf("%w: canonical usage event %s has no request id", errCanonicalSettlementDeferred, claim.EventID)
	}
	if claim.Event.CommitState() != corecontracts.CanonicalUsageCommitStateUsageCommitted || claim.Settlement == nil {
		return fmt.Errorf("%w: missing accepted quote or final usage", errCanonicalSettlementDeferred)
	}
	repo := s.repo.(CanonicalSharedPoolUsageRepository)
	accessKey, err := repo.ResolveCanonicalSharedPoolSettlementIdentity(ctx, claim)
	if err != nil {
		return err
	}
	if accessKey == nil {
		return fmt.Errorf("%w: historical shared pool identity unavailable", errCanonicalSettlementDeferred)
	}
	if accessKey.UserID != claim.Event.UserID() {
		return fmt.Errorf("%w: settlement member mismatch", errCanonicalSettlementDeferred)
	}
	quote, err := s.resolveCanonicalSettlementQuoteForClaim(ctx, accessKey, claim)
	if err != nil {
		if isDeferrableSettlementPricingError(err) {
			return fmt.Errorf("%w: request %s: %v", errCanonicalSettlementDeferred, requestID, err)
		}
		return err
	}
	cost, err := calculateSharedPoolQuoteCost(quote, canonicalUsageTokens(claim.Event.Units()))
	if err != nil {
		if isDeferrableSettlementPricingError(err) {
			return fmt.Errorf("%w: request %s: %v", errCanonicalSettlementDeferred, requestID, err)
		}
		return err
	}
	if cost <= 0 && !quote.ExplicitFree {
		return fmt.Errorf("%w: request %s produced zero cost for a non-free quote", errCanonicalSettlementDeferred, requestID)
	}
	input := SharedPoolUsageInput{
		AccessKeyID:    accessKey.ID,
		PoolID:         accessKey.PoolID,
		AccountID:      accessKey.AccountID,
		UserID:         claim.Event.UserID(),
		Cost:           cost,
		Model:          quote.ModelName,
		RequestID:      requestID,
		Success:        true,
		PriceVersionID: quote.PriceVersionID,
		PricingSource:  quote.PricingSource,
		PriceSnapshot:  MarshalSharedPoolPriceSnapshot(quote),
	}
	if err := s.RecordSharedPoolUsage(ctx, input); err != nil {
		if errors.Is(err, ErrSharedPoolReservationConflict) {
			posted, verifyErr := repo.HasPostedCanonicalSharedPoolSettlement(ctx, input)
			if verifyErr != nil {
				return verifyErr
			}
			if posted {
				return nil
			}
		}
		return err
	}
	return nil
}

func (s *BizDecipherService) resolveCanonicalSettlementQuoteForClaim(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
	claim CanonicalSharedPoolUsageClaim,
) (*SharedPoolPriceQuote, error) {
	if claim.Settlement == nil {
		return nil, errCanonicalSettlementDeferred
	}
	if err := claim.Settlement.Validate(); err != nil {
		return nil, fmt.Errorf("request %s has an invalid settlement snapshot: %w", claim.RequestID, err)
	}
	quote := claim.Settlement.SharedPoolQuote
	if quote.PoolID != accessKey.PoolID {
		return nil, fmt.Errorf("request %s settlement quote belongs to pool %d, not %d", claim.RequestID, quote.PoolID, accessKey.PoolID)
	}
	endpoint := normalizeSharedPoolEndpoint(quote.EndpointType)
	candidates := canonicalSettlementEndpointCandidates(claim.Event.Protocol())
	matched := false
	for _, candidate := range candidates {
		if endpoint == normalizeSharedPoolEndpoint(candidate) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("request %s settlement quote endpoint %q does not match protocol %q", claim.RequestID, quote.EndpointType, claim.Event.Protocol())
	}
	cloned := cloneSharedPoolPriceQuote(quote)
	FinalizeSharedPoolPriceQuote(cloned)
	return cloned, nil
}

func (s *BizDecipherService) resolveCanonicalSettlementQuote(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
	requestModel string,
	protocol string,
) (*SharedPoolPriceQuote, error) {
	candidates := canonicalSettlementEndpointCandidates(protocol)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no pricing endpoint could be derived from protocol %q", errCanonicalSettlementDeferred, protocol)
	}
	var chosen *SharedPoolPriceQuote
	for _, endpointType := range candidates {
		quote, err := s.resolveCanonicalSettlementQuoteForEndpoint(ctx, accessKey, requestModel, endpointType)
		if err != nil {
			if isDeferrableSettlementPricingError(err) {
				continue
			}
			return nil, err
		}
		if quote == nil {
			continue
		}
		if chosen != nil && chosen.PriceHash != quote.PriceHash {
			return nil, fmt.Errorf("%w: ambiguous pricing across endpoints for model %q", errCanonicalSettlementDeferred, requestModel)
		}
		chosen = quote
	}
	if chosen == nil {
		return nil, fmt.Errorf("%w: no configured price for model %q", errCanonicalSettlementDeferred, requestModel)
	}
	return chosen, nil
}

func (s *BizDecipherService) resolveCanonicalSettlementQuoteForEndpoint(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
	requestModel string,
	endpointType string,
) (*SharedPoolPriceQuote, error) {
	modelName := strings.TrimSpace(accessKey.PublishedModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(requestModel)
	}
	quote, err := s.ResolveSharedPoolPriceQuote(ctx, accessKey.PoolID, modelName, endpointType)
	officialRequired := errors.Is(err, ErrSharedPoolOfficialPriceRequired) ||
		(err == nil && quote != nil && quote.PricingSource == SharedPoolPricingSourceOfficial)
	if !officialRequired {
		if err != nil {
			return nil, err
		}
		if quote == nil {
			return nil, ErrSharedPoolPricingUnavailable
		}
		return sharedPoolQuoteWithEffectiveMultiplier(quote, accessKey.RateMultiplier), nil
	}
	canonicalModel := strings.TrimSpace(accessKey.CanonicalModelName)
	if canonicalModel == "" || s.billingService == nil {
		return nil, fmt.Errorf("%w: %s", ErrSharedPoolPricingNotConfigured, modelName)
	}
	official, pricingErr := s.billingService.GetModelPricing(canonicalModel)
	if pricingErr != nil {
		return nil, pricingErr
	}
	if !validCanonicalSharedPoolModelPricing(official) {
		return nil, ErrModelPricingUnavailable
	}
	base := sharedPoolOfficialBasePrice(official)
	officialQuote, err := s.EnsureSharedPoolOfficialPriceQuote(ctx, accessKey.PoolID, modelName, endpointType, base)
	if err != nil {
		return nil, err
	}
	officialQuote = sharedPoolQuoteWithOfficialPolicy(officialQuote, official)
	return sharedPoolQuoteWithEffectiveMultiplier(officialQuote, accessKey.RateMultiplier), nil
}

// canonicalSettlementEndpointCandidates returns the pricing endpoints that may
// have served a request. A protocol that names its inbound path yields exactly
// one candidate; an unknown protocol falls back to the two text endpoints and
// is rejected later if their prices disagree.
func canonicalSettlementEndpointCandidates(protocol string) []string {
	raw := strings.ToLower(strings.TrimSpace(protocol))
	if raw == "" {
		return []string{SharedPoolEndpointResponses, SharedPoolEndpointChat}
	}
	candidate := raw
	if idx := strings.Index(raw, ":"); idx > 0 {
		head := strings.TrimSpace(raw[:idx])
		if strings.HasPrefix(head, "/") {
			candidate = head
		}
	}
	if endpoint := normalizeSharedPoolEndpoint(candidate); endpoint != "" {
		return []string{endpoint}
	}
	switch {
	case strings.Contains(raw, "/chat/completions"):
		return []string{SharedPoolEndpointChat}
	case strings.Contains(raw, "/responses"):
		return []string{SharedPoolEndpointResponses}
	case strings.Contains(raw, "/images/edits"):
		return []string{SharedPoolEndpointImageEdit}
	case strings.Contains(raw, "/images/"):
		return []string{SharedPoolEndpointImageGeneration}
	case strings.Contains(raw, "/videos/"):
		return []string{SharedPoolEndpointVideo}
	default:
		return []string{SharedPoolEndpointResponses, SharedPoolEndpointChat}
	}
}

func canonicalUsageTokens(units corecontracts.ExactUsageUnits) UsageTokens {
	return UsageTokens{
		InputTokens:         int(units.InputTokens),
		ImageInputTokens:    int(units.ImageInputTokens),
		OutputTokens:        int(units.OutputTokens),
		CacheCreationTokens: int(units.CacheCreationTokens),
		CacheReadTokens:     int(units.CacheReadTokens),
		ImageOutputTokens:   int(units.ImageOutputTokens),
	}
}

func isDeferrableSettlementPricingError(err error) bool {
	return errors.Is(err, ErrSharedPoolPricingUnavailable) ||
		errors.Is(err, ErrSharedPoolPricingNotConfigured) ||
		errors.Is(err, ErrSharedPoolOfficialPriceRequired) ||
		errors.Is(err, ErrModelPricingUnavailable)
}
