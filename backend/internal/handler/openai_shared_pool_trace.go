package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// sharedPoolGatewayTrace carries timing marks only. Accounting state is owned
// by the existing reservation/settlement path and must never be inferred from
// this object.
type sharedPoolGatewayTrace struct {
	startedAt time.Time
	trace     service.SharedPoolUsageTrace
	begun     bool
}

func newSharedPoolGatewayTrace(c *gin.Context, model, endpoint string) *sharedPoolGatewayTrace {
	startedAt := time.Now()
	switch endpoint {
	case service.SharedPoolEndpointChat:
		endpoint = "/v1/chat/completions"
	case service.SharedPoolEndpointResponses:
		endpoint = "/v1/responses"
	case service.SharedPoolEndpointImageGeneration:
		endpoint = "/v1/images/generations"
	case service.SharedPoolEndpointImageEdit:
		endpoint = "/v1/images/edits"
	case service.SharedPoolEndpointVideo:
		endpoint = "/v1/videos/generations"
	}
	trace := service.SharedPoolUsageTrace{
		RequestID:         "shared-pool-trace-" + uuid.NewString(),
		ModelSnapshot:     model,
		Endpoint:          endpoint,
		Status:            service.SharedPoolUsageTraceStatusPending,
		SettlementOutcome: service.SharedPoolSettlementPending,
		CreatedAt:         startedAt.UTC(),
	}
	if authMs, ok := getContextInt64(c, service.OpsAuthLatencyMsKey); ok && authMs >= 0 {
		trace.AuthLatencyMs = sharedPoolTraceInt64Ptr(authMs)
	}
	return &sharedPoolGatewayTrace{startedAt: startedAt, trace: trace}
}

func (t *sharedPoolGatewayTrace) bindAccess(accessKey *service.SharedPoolAccessKey, requestID string) {
	if t == nil || accessKey == nil {
		return
	}
	t.trace.AccessKeyID = accessKey.ID
	t.trace.PoolID = accessKey.PoolID
	t.trace.UserID = accessKey.UserID
	if requestID != "" && !t.begun {
		t.trace.RequestID = requestID
	}
	t.trace.PoolNameSnapshot = accessKey.PoolName
	t.trace.AccountAlias = service.SharedPoolSafeAccountAlias(accessKey.PoolID, accessKey.AccountID)
	if published := sharedPoolPublishedModelName(accessKey, t.trace.ModelSnapshot); published != "" {
		t.trace.ModelSnapshot = published
	}
}

func (t *sharedPoolGatewayTrace) markRouting(start time.Time) {
	if t != nil {
		t.trace.RoutingLatencyMs = sharedPoolTraceDurationPtr(time.Since(start))
	}
}

func (t *sharedPoolGatewayTrace) markConcurrency(start time.Time) {
	if t != nil {
		t.trace.ConcurrencyLatencyMs = sharedPoolTraceDurationPtr(time.Since(start))
	}
}

func (t *sharedPoolGatewayTrace) markReservation(start time.Time) {
	if t != nil {
		t.trace.ReservationLatencyMs = sharedPoolTraceDurationPtr(time.Since(start))
	}
}

func (t *sharedPoolGatewayTrace) markUpstream(start time.Time) {
	if t != nil && !start.IsZero() {
		t.trace.UpstreamStarted = true
		t.trace.UpstreamLatencyMs = sharedPoolTraceDurationPtr(time.Since(start))
	}
}

func (t *sharedPoolGatewayTrace) begin(ctx context.Context, h *OpenAIGatewayHandler) error {
	if t == nil || h == nil || h.gatewayService == nil {
		return nil
	}
	if t.begun {
		return nil
	}
	t.begun = true
	return h.gatewayService.BeginSharedPoolGatewayUsageTrace(ctx, t.trace)
}

func (t *sharedPoolGatewayTrace) finalizeBeforeUpstream(
	parent context.Context,
	h *OpenAIGatewayHandler,
	c *gin.Context,
	failureStage string,
	httpStatus int,
) error {
	return t.finalize(
		parent,
		h,
		c,
		nil,
		false,
		failureStage,
		service.SharedPoolSettlementNotRequired,
		0,
		time.Time{},
		time.Time{},
		httpStatus,
	)
}

func (t *sharedPoolGatewayTrace) finalize(
	parent context.Context,
	h *OpenAIGatewayHandler,
	c *gin.Context,
	result *service.OpenAIForwardResult,
	forwardSucceeded bool,
	failureStage string,
	settlementOutcome string,
	settlementAttempts int,
	upstreamStarted time.Time,
	settlementStarted time.Time,
	fallbackHTTPStatus int,
) error {
	if t == nil || h == nil || h.gatewayService == nil {
		return nil
	}
	trace := t.trace
	trace.UpstreamStarted = trace.UpstreamStarted || !upstreamStarted.IsZero()
	if trace.UpstreamLatencyMs == nil && !upstreamStarted.IsZero() {
		trace.UpstreamLatencyMs = sharedPoolTraceDurationPtr(time.Since(upstreamStarted))
	}
	if result != nil {
		actualInput := result.Usage.InputTokens - result.Usage.CacheReadInputTokens - result.Usage.CacheCreationInputTokens
		if actualInput < 0 {
			actualInput = 0
		}
		trace.InputTokens = actualInput
		trace.OutputTokens = maxSharedPoolTraceToken(result.Usage.OutputTokens)
		trace.CacheReadTokens = maxSharedPoolTraceToken(result.Usage.CacheReadInputTokens)
		trace.CacheCreationTokens = maxSharedPoolTraceToken(result.Usage.CacheCreationInputTokens)
		trace.UsageObserved = result.Usage.InputTokens > 0 || result.Usage.OutputTokens > 0 ||
			result.Usage.CacheReadInputTokens > 0 || result.Usage.CacheCreationInputTokens > 0 ||
			result.ImageCount > 0 || result.VideoCount > 0
		trace.ImageCount = maxSharedPoolTraceToken(result.ImageCount)
		trace.ImageSize = result.ImageSize
		trace.VideoCount = maxSharedPoolTraceToken(result.VideoCount)
		trace.VideoResolution = result.VideoResolution
		trace.VideoDurationSeconds = maxSharedPoolTraceToken(result.VideoDurationSeconds)
		if result.Duration > 0 {
			trace.UpstreamLatencyMs = sharedPoolTraceDurationPtr(result.Duration)
		}
		if result.FirstTokenMs != nil && *result.FirstTokenMs >= 0 {
			firstToken := int64(*result.FirstTokenMs)
			trace.FirstTokenMs = &firstToken
		}
	}
	if trace.UpstreamLatencyMs == nil {
		if upstreamMs, ok := getContextInt64(c, service.OpsUpstreamLatencyMsKey); ok && upstreamMs >= 0 {
			trace.UpstreamLatencyMs = sharedPoolTraceInt64Ptr(upstreamMs)
		}
	}
	if trace.FirstTokenMs == nil {
		if firstTokenMs, ok := getContextInt64(c, service.OpsTimeToFirstTokenMsKey); ok && firstTokenMs >= 0 {
			trace.FirstTokenMs = sharedPoolTraceInt64Ptr(firstTokenMs)
		}
	}
	if !settlementStarted.IsZero() {
		trace.SettlementLatencyMs = sharedPoolTraceDurationPtr(time.Since(settlementStarted))
	}
	trace.RetryCount = settlementAttempts - 1
	if trace.RetryCount < 0 {
		trace.RetryCount = 0
	}
	trace.SettlementOutcome = settlementOutcome
	trace.FailureStage = failureStage
	if forwardSucceeded && settlementOutcome == service.SharedPoolSettlementSettled {
		trace.Status = service.SharedPoolUsageTraceStatusSucceeded
		trace.FailureStage = ""
	} else if forwardSucceeded && settlementOutcome == service.SharedPoolSettlementPending {
		trace.Status = service.SharedPoolUsageTraceStatusPending
		trace.FailureStage = ""
	} else {
		trace.Status = service.SharedPoolUsageTraceStatusFailed
		if trace.FailureStage == "" {
			trace.FailureStage = "unknown"
		}
	}
	totalMs := time.Since(t.startedAt).Milliseconds()
	if trace.AuthLatencyMs != nil {
		totalMs += *trace.AuthLatencyMs
	}
	trace.TotalLatencyMs = sharedPoolTraceInt64Ptr(totalMs)
	status := fallbackHTTPStatus
	if c != nil && c.Writer != nil && c.Writer.Written() {
		status = c.Writer.Status()
	}
	if status >= 100 && status <= 599 {
		trace.HTTPStatus = &status
	}
	if trace.Status != service.SharedPoolUsageTraceStatusPending {
		now := time.Now().UTC()
		trace.CompletedAt = &now
	}

	base := usageRecordContext(parent, context.Background())
	ctx, cancel := context.WithTimeout(base, 5*time.Second)
	defer cancel()
	return h.gatewayService.FinalizeSharedPoolGatewayUsageTrace(ctx, trace)
}

func sharedPoolTraceDurationPtr(duration time.Duration) *int64 {
	return sharedPoolTraceInt64Ptr(duration.Milliseconds())
}

func sharedPoolTraceInt64Ptr(value int64) *int64 {
	if value < 0 {
		return nil
	}
	out := value
	return &out
}

func maxSharedPoolTraceToken(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
