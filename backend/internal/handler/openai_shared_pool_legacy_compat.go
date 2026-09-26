package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func shouldFailoverSharedPoolRoute(c *gin.Context, writerSizeBeforeForward int, err error, status int) bool {
	if c == nil || c.Request == nil || c.Writer == nil || err == nil || status < http.StatusBadRequest || status > 599 {
		return false
	}
	if service.SharedPoolRouteFailoverCount(c.Request.Context()) >= 2 {
		return false
	}
	if c.Writer.Written() || service.IsResponseCommitted(c) || c.Writer.Size() != writerSizeBeforeForward {
		return false
	}
	var failoverErr *service.UpstreamFailoverError
	if !errors.As(err, &failoverErr) || failoverErr == nil || !failoverErr.ShouldRetryNextAccount() {
		return false
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestTimeout,
		http.StatusTooEarly, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (h *OpenAIGatewayHandler) recordSharedPoolUsageDetached(
	parent context.Context,
	apiKey *service.APIKey,
	accessKey *service.SharedPoolAccessKey,
	result *service.OpenAIForwardResult,
	requestID string,
) error {
	_, err := h.recordSharedPoolUsageDetachedWithAttempts(parent, apiKey, accessKey, result, requestID)
	return err
}

func (h *OpenAIGatewayHandler) recordSharedPoolUsageDetachedWithAttempts(
	parent context.Context,
	apiKey *service.APIKey,
	accessKey *service.SharedPoolAccessKey,
	result *service.OpenAIForwardResult,
	requestID string,
) (int, error) {
	if h == nil || h.gatewayService == nil {
		return 0, errors.New("shared pool gateway is unavailable")
	}
	base := usageRecordContext(parent, context.Background())
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(base, 10*time.Second)
		lastErr = h.gatewayService.RecordSharedPoolOpenAIUsage(ctx, apiKey, accessKey, result, requestID)
		cancel()
		if lastErr == nil || errors.Is(lastErr, service.ErrSharedPoolReservationFinalized) {
			return attempt + 1, nil
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			<-timer.C
		}
	}
	return 3, lastErr
}

func (h *OpenAIGatewayHandler) markSharedPoolUsageReviewDetachedWithAttempts(
	parent context.Context,
	accessKey *service.SharedPoolAccessKey,
	requestID string,
	reason string,
) (int, error) {
	if h == nil || h.gatewayService == nil || accessKey == nil {
		return 0, errors.New("shared pool gateway is unavailable")
	}
	base := usageRecordContext(parent, context.Background())
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(base, 10*time.Second)
		lastErr = h.gatewayService.MarkSharedPoolUsageReviewRequired(ctx, accessKey.ID, requestID, reason)
		cancel()
		if lastErr == nil {
			return attempt + 1, nil
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			<-timer.C
		}
	}
	return 3, lastErr
}

func (h *OpenAIGatewayHandler) releaseSharedPoolUsageDetachedWithAttempts(
	parent context.Context,
	accessKey *service.SharedPoolAccessKey,
	requestID string,
	reason string,
) (int, error) {
	if h == nil || h.gatewayService == nil || accessKey == nil {
		return 0, errors.New("shared pool gateway is unavailable")
	}
	base := usageRecordContext(parent, context.Background())
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(base, 10*time.Second)
		lastErr = h.gatewayService.ReleaseSharedPoolUsageAfterVerifiedFailure(ctx, accessKey.ID, requestID, reason)
		cancel()
		if lastErr == nil {
			return attempt + 1, nil
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			<-timer.C
		}
	}
	return 3, lastErr
}

// settleFailedSharedPoolForward keeps the billing decision separate from the
// transport result. Once forwarding started, an empty or unbillable result is
// unknown rather than free and therefore must enter audited review.
func settleFailedSharedPoolForward(
	result *service.OpenAIForwardResult,
	reviewReason string,
	record func(*service.OpenAIForwardResult) (int, error),
	markReview func(string) (int, error),
) (int, string, error) {
	if markReview == nil {
		return 0, service.SharedPoolSettlementFailed, errors.New("shared pool usage review recorder is unavailable")
	}
	if result == nil {
		attempts, err := markReview(reviewReason)
		if err != nil {
			return attempts, service.SharedPoolSettlementFailed, err
		}
		return attempts, service.SharedPoolSettlementUnknown, nil
	}
	if record == nil {
		return 0, service.SharedPoolSettlementFailed, errors.New("shared pool usage recorder is unavailable")
	}
	attempts, err := record(result)
	if err == nil {
		return attempts, service.SharedPoolSettlementSettled, nil
	}
	reviewAttempts, reviewErr := markReview(reviewReason)
	if reviewErr == nil {
		return attempts + reviewAttempts, service.SharedPoolSettlementUnknown, nil
	}
	return attempts + reviewAttempts, service.SharedPoolSettlementFailed, errors.Join(err, reviewErr)
}

func sharedPoolUnknownForwardReason(err error, partialResponse bool) string {
	if partialResponse {
		return "upstream_partial_response_result_unknown"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "upstream_timeout_result_unknown"
	}
	if errors.Is(err, context.Canceled) {
		return "upstream_canceled_result_unknown"
	}
	message := strings.ToLower(strings.TrimSpace(errString(err)))
	for _, marker := range []string{"connection reset", "broken pipe", "unexpected eof", "connection aborted"} {
		if strings.Contains(message, marker) {
			return "upstream_connection_lost_result_unknown"
		}
	}
	return "upstream_result_unknown"
}

func sharedPoolVerifiedHTTPFailureStatus(c *gin.Context, err error, partialResponse bool) int {
	if partialResponse {
		return 0
	}
	var failoverErr *service.UpstreamFailoverError
	if errors.As(err, &failoverErr) && failoverErr != nil && failoverErr.StatusCode >= 400 && failoverErr.StatusCode <= 599 {
		return failoverErr.StatusCode
	}
	if c == nil {
		return 0
	}
	value, ok := c.Get(service.OpsUpstreamStatusCodeKey)
	if !ok {
		return 0
	}
	status, ok := value.(int)
	if !ok || status < 400 || status > 599 {
		return 0
	}
	return status
}

func sharedPoolVerifiedHTTPFailureReason(status int) string {
	return fmt.Sprintf("upstream_http_%d_no_success", status)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
