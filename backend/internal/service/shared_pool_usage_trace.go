package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const (
	SharedPoolUsageTraceStatusPending   = "pending"
	SharedPoolUsageTraceStatusSucceeded = "succeeded"
	SharedPoolUsageTraceStatusFailed    = "failed"

	SharedPoolSettlementPending     = "pending"
	SharedPoolSettlementSettled     = "settled"
	SharedPoolSettlementReleased    = "released"
	SharedPoolSettlementFailed      = "failed"
	SharedPoolSettlementNotRequired = "not_required"
	SharedPoolSettlementUnknown     = "unknown"
)

// SharedPoolUsageTrace is a credential-free request audit record shown to the
// pool member. It deliberately excludes URLs, API keys, OAuth/proxy data,
// prompts, responses, and raw upstream errors.
type SharedPoolUsageTrace struct {
	ID                   int64      `json:"id"`
	AccessKeyID          int64      `json:"-"`
	PoolID               int64      `json:"pool_id"`
	UserID               int64      `json:"-"`
	RequestID            string     `json:"request_id"`
	PoolNameSnapshot     string     `json:"pool_name_snapshot"`
	ModelSnapshot        string     `json:"model"`
	Endpoint             string     `json:"endpoint"`
	AccountAlias         string     `json:"account_alias"`
	Status               string     `json:"status"`
	FailureStage         string     `json:"failure_stage,omitempty"`
	SettlementOutcome    string     `json:"settlement_outcome"`
	UpstreamStarted      bool       `json:"upstream_started"`
	UsageObserved        bool       `json:"usage_observed"`
	InputTokens          int        `json:"input_tokens"`
	OutputTokens         int        `json:"output_tokens"`
	CacheReadTokens      int        `json:"cache_read_tokens"`
	CacheCreationTokens  int        `json:"cache_creation_tokens"`
	ImageCount           int        `json:"image_count"`
	ImageSize            string     `json:"image_size,omitempty"`
	VideoCount           int        `json:"video_count"`
	VideoResolution      string     `json:"video_resolution,omitempty"`
	VideoDurationSeconds int        `json:"video_duration_seconds"`
	AuthLatencyMs        *int64     `json:"auth_latency_ms,omitempty"`
	SeatLatencyMs        *int64     `json:"seat_latency_ms,omitempty"`
	RoutingLatencyMs     *int64     `json:"routing_latency_ms,omitempty"`
	ConcurrencyLatencyMs *int64     `json:"concurrency_latency_ms,omitempty"`
	ReservationLatencyMs *int64     `json:"reservation_latency_ms,omitempty"`
	UpstreamLatencyMs    *int64     `json:"upstream_latency_ms,omitempty"`
	FirstTokenMs         *int64     `json:"first_token_ms,omitempty"`
	SettlementLatencyMs  *int64     `json:"settlement_latency_ms,omitempty"`
	TotalLatencyMs       *int64     `json:"total_latency_ms,omitempty"`
	RetryCount           int        `json:"retry_count"`
	HTTPStatus           *int       `json:"http_status,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
}

type SharedPoolUsageTracePage struct {
	Items        []SharedPoolUsageTrace `json:"items"`
	NextBeforeID int64                  `json:"next_before_id,omitempty"`
	HasMore      bool                   `json:"has_more"`
}

// SharedPoolUsageTraceRepository is optional so existing test doubles and
// deployments that have not applied migration 225 keep working. Production's
// bizDecipherRepository implements it in a separate, non-accounting file.
type SharedPoolUsageTraceRepository interface {
	BeginSharedPoolUsageTrace(ctx context.Context, trace SharedPoolUsageTrace) error
	FinalizeSharedPoolUsageTrace(ctx context.Context, trace SharedPoolUsageTrace) error
	ListSharedPoolUsageTraces(ctx context.Context, userID, beforeID int64, limit int) (*SharedPoolUsageTracePage, error)
}

func (s *BizDecipherService) BeginSharedPoolUsageTrace(ctx context.Context, trace SharedPoolUsageTrace) error {
	repo, ok := sharedPoolTraceRepository(s)
	if !ok {
		return nil
	}
	if err := normalizeSharedPoolUsageTrace(&trace, false); err != nil {
		return err
	}
	trace.Status = SharedPoolUsageTraceStatusPending
	trace.SettlementOutcome = SharedPoolSettlementPending
	trace.CompletedAt = nil
	return repo.BeginSharedPoolUsageTrace(ctx, trace)
}

func (s *BizDecipherService) FinalizeSharedPoolUsageTrace(ctx context.Context, trace SharedPoolUsageTrace) error {
	repo, ok := sharedPoolTraceRepository(s)
	if !ok {
		return nil
	}
	if err := normalizeSharedPoolUsageTrace(&trace, true); err != nil {
		return err
	}
	if trace.Status != SharedPoolUsageTraceStatusSucceeded {
		trace.Status = SharedPoolUsageTraceStatusFailed
	}
	if trace.CompletedAt == nil {
		now := time.Now().UTC()
		trace.CompletedAt = &now
	}
	return repo.FinalizeSharedPoolUsageTrace(ctx, trace)
}

func (s *BizDecipherService) ListMySharedPoolUsageTraces(ctx context.Context, userID, beforeID int64, limit int) (*SharedPoolUsageTracePage, error) {
	repo, ok := sharedPoolTraceRepository(s)
	if !ok {
		return &SharedPoolUsageTracePage{Items: []SharedPoolUsageTrace{}}, nil
	}
	if userID <= 0 || beforeID < 0 {
		return nil, errors.New("invalid shared pool usage trace query")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return repo.ListSharedPoolUsageTraces(ctx, userID, beforeID, limit)
}

func sharedPoolTraceRepository(s *BizDecipherService) (SharedPoolUsageTraceRepository, bool) {
	if s == nil || s.repo == nil {
		return nil, false
	}
	repo, ok := s.repo.(SharedPoolUsageTraceRepository)
	return repo, ok && repo != nil
}

// SharedPoolSafeAccountAlias makes a stable member-facing label without
// exposing the database account id. It is not a credential and is intentionally
// derived only from numeric internal identifiers.
func SharedPoolSafeAccountAlias(poolID, accountID int64) string {
	if poolID <= 0 {
		return "共享线路"
	}
	if accountID <= 0 {
		accountID = poolID
	}
	sum := sha256.Sum256([]byte(sharedPoolTraceInt64(poolID) + ":" + sharedPoolTraceInt64(accountID)))
	return "共享线路 " + strings.ToUpper(hex.EncodeToString(sum[:3]))
}

func sharedPoolTraceInt64(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func normalizeSharedPoolUsageTrace(trace *SharedPoolUsageTrace, finalized bool) error {
	if trace == nil || trace.AccessKeyID <= 0 || trace.PoolID <= 0 || trace.UserID <= 0 {
		return errors.New("invalid shared pool usage trace identity")
	}
	trace.RequestID = truncateSharedPoolTraceText(strings.TrimSpace(trace.RequestID), 128)
	if trace.RequestID == "" {
		return errors.New("shared pool usage trace request id is required")
	}
	trace.PoolNameSnapshot = truncateSharedPoolTraceText(strings.TrimSpace(trace.PoolNameSnapshot), 160)
	trace.ModelSnapshot = truncateSharedPoolTraceText(strings.TrimSpace(trace.ModelSnapshot), 128)
	trace.Endpoint = normalizeSharedPoolTraceEndpoint(trace.Endpoint)
	trace.AccountAlias = truncateSharedPoolTraceText(strings.TrimSpace(trace.AccountAlias), 64)
	if trace.AccountAlias == "" {
		trace.AccountAlias = SharedPoolSafeAccountAlias(trace.PoolID, 0)
	}
	trace.FailureStage = normalizeSharedPoolFailureStage(trace.FailureStage)
	trace.SettlementOutcome = normalizeSharedPoolSettlementOutcome(trace.SettlementOutcome, finalized)
	trace.InputTokens = maxSharedPoolTraceInt(trace.InputTokens)
	trace.OutputTokens = maxSharedPoolTraceInt(trace.OutputTokens)
	trace.CacheReadTokens = maxSharedPoolTraceInt(trace.CacheReadTokens)
	trace.CacheCreationTokens = maxSharedPoolTraceInt(trace.CacheCreationTokens)
	trace.ImageCount = maxSharedPoolTraceInt(trace.ImageCount)
	trace.ImageSize = truncateSharedPoolTraceText(strings.TrimSpace(trace.ImageSize), 32)
	trace.VideoCount = maxSharedPoolTraceInt(trace.VideoCount)
	trace.VideoResolution = truncateSharedPoolTraceText(strings.TrimSpace(trace.VideoResolution), 32)
	trace.VideoDurationSeconds = maxSharedPoolTraceInt(trace.VideoDurationSeconds)
	trace.RetryCount = maxSharedPoolTraceInt(trace.RetryCount)
	trace.AuthLatencyMs = normalizeSharedPoolLatency(trace.AuthLatencyMs)
	trace.SeatLatencyMs = normalizeSharedPoolLatency(trace.SeatLatencyMs)
	trace.RoutingLatencyMs = normalizeSharedPoolLatency(trace.RoutingLatencyMs)
	trace.ConcurrencyLatencyMs = normalizeSharedPoolLatency(trace.ConcurrencyLatencyMs)
	trace.ReservationLatencyMs = normalizeSharedPoolLatency(trace.ReservationLatencyMs)
	trace.UpstreamLatencyMs = normalizeSharedPoolLatency(trace.UpstreamLatencyMs)
	trace.FirstTokenMs = normalizeSharedPoolLatency(trace.FirstTokenMs)
	trace.SettlementLatencyMs = normalizeSharedPoolLatency(trace.SettlementLatencyMs)
	trace.TotalLatencyMs = normalizeSharedPoolLatency(trace.TotalLatencyMs)
	if trace.HTTPStatus != nil && (*trace.HTTPStatus < 100 || *trace.HTTPStatus > 599) {
		trace.HTTPStatus = nil
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = time.Now().UTC()
	}
	return nil
}

func normalizeSharedPoolTraceEndpoint(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexAny(value, "?#"); idx >= 0 {
		value = value[:idx]
	}
	if !strings.HasPrefix(value, "/") || strings.Contains(value, "://") {
		return ""
	}
	return truncateSharedPoolTraceText(value, 128)
}

func normalizeSharedPoolFailureStage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auth", "seat", "routing", "pricing", "concurrency", "reservation", "upstream", "settlement", "cancelled", "unknown":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeSharedPoolSettlementOutcome(value string, finalized bool) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SharedPoolSettlementPending, SharedPoolSettlementSettled, SharedPoolSettlementReleased,
		SharedPoolSettlementFailed, SharedPoolSettlementNotRequired, SharedPoolSettlementUnknown:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		if finalized {
			return SharedPoolSettlementUnknown
		}
		return SharedPoolSettlementPending
	}
}

func normalizeSharedPoolLatency(value *int64) *int64 {
	if value == nil || *value < 0 {
		return nil
	}
	out := *value
	return &out
}

func truncateSharedPoolTraceText(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func maxSharedPoolTraceInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
