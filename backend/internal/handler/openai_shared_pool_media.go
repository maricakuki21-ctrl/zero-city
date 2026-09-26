package handler

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const sharedPoolMaximumImageCount = 10

// SharedPoolImages is a separate money-safe orchestration path for managed
// shared-pool keys. It deliberately does not reuse the ordinary key billing
// path: the member balance is pre-authorized and the pool owner wallet is paid
// only after verified output metadata is available.
func (h *OpenAIGatewayHandler) SharedPoolImages(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || !service.IsSharedPoolAPIKey(apiKey) {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid shared-pool key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || h == nil || h.gatewayService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Shared-pool media gateway is unavailable")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.shared_pool_images",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID))

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	parsed, err := h.gatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if err := validateSharedPoolImageCount(body, parsed); err != nil {
		h.errorResponse(c, http.StatusUnprocessableEntity, "unsafe_cost_estimate", err.Error())
		return
	}
	streamStarted = parsed.Stream
	endpointType := service.SharedPoolEndpointImageGeneration
	if parsed.IsEdits() {
		endpointType = service.SharedPoolEndpointImageEdit
	}
	setOpsRequestContext(c, parsed.Model, parsed.Stream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(parsed.Stream, false)))
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, parsed.Model, parsed.ModerationBody()); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}
	imageRelease, acquired := h.acquireImageGenerationSlot(c, streamStarted)
	if !acquired {
		return
	}
	if imageRelease != nil {
		defer imageRelease()
	}

	trace := newSharedPoolGatewayTrace(c, parsed.Model, endpointType)
	routingStarted := time.Now()
	accessKey, err := h.gatewayService.GetSharedPoolMediaAccessKeyQuoteByAPIKeyID(
		c.Request.Context(), apiKey.ID, parsed.Model, endpointType, parsed.SizeTier, "")
	trace.markRouting(routingStarted)
	if err != nil {
		if errors.Is(err, service.ErrSharedPoolPricingNotConfigured) || errors.Is(err, service.ErrSharedPoolPricingUnavailable) {
			h.errorResponse(c, http.StatusUnprocessableEntity, "pricing_not_configured", "This image endpoint has not passed its price and capability checks yet.")
			return
		}
		if errors.Is(err, service.ErrSharedPoolOfficialMediaVariantUnsupported) {
			h.errorResponse(c, http.StatusUnprocessableEntity, "unsupported_media_variant", err.Error())
			return
		}
		reqLog.Warn("shared_pool.image_lookup_failed", zap.Error(err))
		h.errorResponse(c, http.StatusBadGateway, "api_error", "Shared pool is temporarily unavailable")
		return
	}
	if !sharedPoolAccessKeyReady(accessKey) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "Shared pool is not available")
		return
	}
	if err := h.gatewayService.ValidateSharedPoolPricing(c.Request.Context(), apiKey, accessKey, parsed.Model); err != nil {
		h.errorResponse(c, http.StatusUnprocessableEntity, "pricing_not_configured", "This image endpoint has no valid price.")
		return
	}
	concurrencyStarted := time.Now()
	releaseSlots, acquired := h.acquireSharedPoolSlots(c, reqLog, accessKey, sharedPoolPublishedModelName(accessKey, parsed.Model))
	trace.markConcurrency(concurrencyStarted)
	if !acquired {
		return
	}
	if releaseSlots != nil {
		defer releaseSlots()
	}

	reservationStarted := time.Now()
	prepared, err := h.gatewayService.PrepareSharedPoolMediaRequest(c.Request.Context(), accessKey, endpointType, body, parsed.N, time.Time{})
	trace.markReservation(reservationStarted)
	if err != nil {
		handleSharedPoolReservationError(c, h, reqLog, accessKey, err)
		return
	}
	requestID := prepared.RequestID
	createContext, err := mediatask.NewCreateContext(requestID, c.GetHeader("Idempotency-Key"), body)
	if err != nil {
		h.errorResponse(c, http.StatusConflict, "idempotency_error", "The media request identity is invalid")
		return
	}
	c.Request = c.Request.WithContext(mediatask.WithCreateContext(c.Request.Context(), createContext))
	trace.bindAccess(accessKey, requestID)
	if traceErr := trace.begin(c.Request.Context(), h); traceErr != nil {
		reqLog.Warn("shared_pool.image_trace_begin_failed", zap.Error(traceErr))
	}
	forwarding := false
	finalized := false
	defer func() {
		if forwarding || finalized {
			return
		}
		if releaseErr := h.recordSharedPoolUsageDetached(c.Request.Context(), apiKey, accessKey, nil, requestID); releaseErr != nil {
			reqLog.Error("shared_pool.image_release_unstarted_failed", zap.Error(releaseErr))
		}
	}()
	if err := h.gatewayService.MarkSharedPoolUsageForwarding(c.Request.Context(), accessKey.ID, requestID); err != nil {
		finalized = true
		_, releaseErr := h.recordSharedPoolUsageDetachedWithAttempts(c.Request.Context(), apiKey, accessKey, nil, requestID)
		if releaseErr != nil {
			reqLog.Error("shared_pool.image_release_after_mark_failed", zap.Error(releaseErr))
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "billing_unavailable", "No upstream request was sent and no charge was made.")
		return
	}
	forwarding = true
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	writerSizeBeforeForward := c.Writer.Size()
	upstreamStartedAt := time.Now()
	forwardImages := h.gatewayService.ForwardSharedPoolImages
	result, forwardErr := forwardImages(c.Request.Context(), c, accessKey, body, parsed)
	trace.markUpstream(upstreamStartedAt)
	settlementStarted := time.Now()
	finalized = true
	settlementAttempts := 0
	settlementOutcome := service.SharedPoolSettlementUnknown
	failureStage := ""
	settlementErr := error(nil)
	if result != nil && result.ImageCountObserved && result.ImageCount > 0 && result.ImageCount <= parsed.N {
		settlementAttempts, settlementErr = h.recordSharedPoolMediaUsageDetachedWithAttempts(c.Request.Context(), apiKey, accessKey, endpointType, result, requestID, parsed.N)
		if settlementErr == nil {
			settlementOutcome = service.SharedPoolSettlementSettled
		}
	} else {
		settlementAttempts, settlementErr = h.markSharedPoolUsageReviewDetachedWithAttempts(c.Request.Context(), accessKey, requestID, sharedPoolImageReviewReason(result, parsed.N, forwardErr))
	}
	if settlementErr != nil {
		settlementOutcome = service.SharedPoolSettlementFailed
		failureStage = "settlement"
		reqLog.Error("shared_pool.image_finalize_failed", zap.Error(settlementErr))
	} else if settlementOutcome == service.SharedPoolSettlementUnknown {
		failureStage = "settlement"
	}
	if forwardErr != nil {
		if failureStage == "" {
			failureStage = "upstream"
		}
		reqLog.Warn("shared_pool.image_forward_failed", zap.Error(forwardErr))
		if !openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, forwardErr) {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Shared pool image request failed")
		}
	}
	forwardSucceeded := forwardErr == nil && settlementOutcome == service.SharedPoolSettlementSettled
	status := http.StatusOK
	if forwardErr != nil {
		status = http.StatusBadGateway
	}
	if traceErr := trace.finalize(c.Request.Context(), h, c, result, forwardSucceeded, failureStage, settlementOutcome, settlementAttempts, upstreamStartedAt, settlementStarted, status); traceErr != nil {
		reqLog.Warn("shared_pool.image_trace_finalize_failed", zap.Error(traceErr))
	}
}

// SharedPoolGrokVideoGeneration submits an asynchronous xAI video task after a
// durable upper-bound reservation. Routing still fails closed unless the exact
// video endpoint has current pricing and media-probe evidence.
func (h *OpenAIGatewayHandler) SharedPoolGrokVideoGeneration(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || !service.IsSharedPoolAPIKey(apiKey) {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid shared-pool key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || h == nil || h.gatewayService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Shared-pool video gateway is unavailable")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.shared_pool_video_generation",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID))
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	contentType := c.GetHeader("Content-Type")
	requestInfo := service.ParseGrokMediaRequest(contentType, body)
	if err := validateSharedPoolVideoGenerationRequest(body, contentType, requestInfo); err != nil {
		h.errorResponse(c, http.StatusUnprocessableEntity, "unsafe_cost_estimate", err.Error())
		return
	}
	setOpsRequestContext(c, requestInfo.Model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(false, false)))
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, requestInfo.Model, requestInfo.ModerationBody()); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}

	trace := newSharedPoolGatewayTrace(c, requestInfo.Model, service.SharedPoolEndpointVideo)
	routingStarted := time.Now()
	accessKey, err := h.gatewayService.GetSharedPoolMediaAccessKeyQuoteByAPIKeyID(
		c.Request.Context(), apiKey.ID, requestInfo.Model, service.SharedPoolEndpointVideo, "", requestInfo.Resolution)
	trace.markRouting(routingStarted)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSharedPoolOfficialMediaVariantUnsupported):
			h.errorResponse(c, http.StatusUnprocessableEntity, "unsupported_media_variant", err.Error())
		case errors.Is(err, service.ErrSharedPoolPricingNotConfigured), errors.Is(err, service.ErrSharedPoolPricingUnavailable):
			h.errorResponse(c, http.StatusUnprocessableEntity, "pricing_not_configured", "This video endpoint has not passed its price and capability checks yet.")
		default:
			reqLog.Warn("shared_pool.video_lookup_failed", zap.Error(err))
			h.errorResponse(c, http.StatusBadGateway, "api_error", "Shared pool is temporarily unavailable")
		}
		return
	}
	if !sharedPoolAccessKeyReady(accessKey) || (accessKey.Provider != service.PlatformGrok && !strings.EqualFold(accessKey.Provider, "xai")) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "Shared-pool video route is not available")
		return
	}
	if err := h.gatewayService.ValidateSharedPoolPricing(c.Request.Context(), apiKey, accessKey, requestInfo.Model); err != nil {
		h.errorResponse(c, http.StatusUnprocessableEntity, "pricing_not_configured", "This video endpoint has no valid price.")
		return
	}
	concurrencyStarted := time.Now()
	releaseSlots, acquired := h.acquireSharedPoolSlots(c, reqLog, accessKey, sharedPoolPublishedModelName(accessKey, requestInfo.Model))
	trace.markConcurrency(concurrencyStarted)
	if !acquired {
		return
	}
	if releaseSlots != nil {
		defer releaseSlots()
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	reservationStarted := time.Now()
	prepared, err := h.gatewayService.PrepareSharedPoolMediaRequest(c.Request.Context(), accessKey, service.SharedPoolEndpointVideo, body, requestInfo.DurationSeconds, expiresAt)
	trace.markReservation(reservationStarted)
	if err != nil {
		handleSharedPoolReservationError(c, h, reqLog, accessKey, err)
		return
	}
	requestID := prepared.RequestID
	createContext, err := mediatask.NewCreateContext(requestID, c.GetHeader("Idempotency-Key"), body)
	if err != nil {
		h.errorResponse(c, http.StatusConflict, "idempotency_error", "The media request identity is invalid")
		return
	}
	c.Request = c.Request.WithContext(mediatask.WithCreateContext(c.Request.Context(), createContext))
	trace.bindAccess(accessKey, requestID)
	if traceErr := trace.begin(c.Request.Context(), h); traceErr != nil {
		reqLog.Warn("shared_pool.video_trace_begin_failed", zap.Error(traceErr))
	}
	if err := h.gatewayService.MarkSharedPoolUsageForwarding(c.Request.Context(), accessKey.ID, requestID); err != nil {
		_, releaseErr := h.recordSharedPoolUsageDetachedWithAttempts(c.Request.Context(), apiKey, accessKey, nil, requestID)
		if releaseErr != nil {
			reqLog.Error("shared_pool.video_release_after_mark_failed", zap.Error(releaseErr))
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "billing_unavailable", "No upstream request was sent and no charge was made.")
		return
	}
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	writerSizeBeforeForward := c.Writer.Size()
	upstreamStartedAt := time.Now()
	forwardVideoCreate := h.gatewayService.ForwardSharedPoolGrokVideo
	result, forwardErr := forwardVideoCreate(c.Request.Context(), c, accessKey, "", body, contentType)
	trace.markUpstream(upstreamStartedAt)
	if forwardErr != nil || result == nil || strings.TrimSpace(result.ResponseID) == "" {
		reason := "video_submission_id_missing"
		if forwardErr != nil {
			reason = "video_submission_result_unknown"
		}
		attempts, reviewErr := h.markSharedPoolUsageReviewDetachedWithAttempts(c.Request.Context(), accessKey, requestID, reason)
		if reviewErr != nil {
			reqLog.Error("shared_pool.video_submission_review_failed", zap.Error(reviewErr))
		}
		_ = trace.finalize(c.Request.Context(), h, c, result, false, "upstream", service.SharedPoolSettlementUnknown, attempts, upstreamStartedAt, time.Now(), http.StatusBadGateway)
		if forwardErr != nil && !openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, forwardErr) {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Shared-pool video submission failed")
		}
		return
	}
	_, taskErr := h.gatewayService.CreateSharedPoolVideoTask(c.Request.Context(), service.CreateSharedPoolMediaTaskInput{
		ReservationID: prepared.Reservation.ID, AccessKeyID: accessKey.ID, PoolID: accessKey.PoolID,
		AccountID: accessKey.AccountID, UserID: accessKey.UserID,
		PriceVersionID: accessKey.PriceQuote.PriceVersionID, Provider: accessKey.Provider,
		ModelSnapshot:         sharedPoolPublishedModelName(accessKey, requestInfo.Model),
		UpstreamModelSnapshot: strings.TrimSpace(accessKey.UpstreamModelName),
		UpstreamRequestID:     result.ResponseID, ReservationRequestID: requestID,
		RequestedResolution: requestInfo.Resolution, RequestedDurationSeconds: requestInfo.DurationSeconds,
		ExpiresAt: expiresAt,
	})
	if taskErr != nil {
		attempts, reviewErr := h.markSharedPoolUsageReviewDetachedWithAttempts(c.Request.Context(), accessKey, requestID, "video_task_ownership_persist_failed")
		if reviewErr != nil {
			reqLog.Error("shared_pool.video_task_review_failed", zap.Error(reviewErr))
		}
		reqLog.Error("shared_pool.video_task_create_failed", zap.Error(taskErr))
		_ = trace.finalize(c.Request.Context(), h, c, result, false, "settlement", service.SharedPoolSettlementUnknown, attempts, upstreamStartedAt, time.Now(), http.StatusOK)
		return
	}
	if traceErr := trace.finalize(c.Request.Context(), h, c, result, true, "", service.SharedPoolSettlementPending, 0, upstreamStartedAt, time.Time{}, http.StatusOK); traceErr != nil {
		reqLog.Warn("shared_pool.video_trace_pending_failed", zap.Error(traceErr))
	}
}

// SharedPoolGrokVideoStatus enforces durable task ownership before contacting
// xAI and applies exactly one terminal money transition.
func (h *OpenAIGatewayHandler) SharedPoolGrokVideoStatus(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || !service.IsSharedPoolAPIKey(apiKey) {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid shared-pool key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || h == nil || h.gatewayService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Shared-pool video gateway is unavailable")
		return
	}
	upstreamRequestID := strings.TrimSpace(c.Param("request_id"))
	reqLog := requestLogger(c, "handler.openai_gateway.shared_pool_video_status",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID))
	task, err := h.gatewayService.GetSharedPoolVideoTask(c.Request.Context(), apiKey.ID, upstreamRequestID)
	if err != nil {
		if errors.Is(err, service.ErrSharedPoolMediaTaskNotFound) {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task was not found for this key")
			return
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Video task ownership is temporarily unavailable")
		return
	}
	accessKey, err := h.gatewayService.GetSharedPoolVideoTaskRuntime(c.Request.Context(), apiKey.ID, task)
	if err != nil {
		reqLog.Warn("shared_pool.video_task_runtime_failed", zap.Error(err))
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "The original video route is temporarily unavailable")
		return
	}
	trace := newSharedPoolGatewayTrace(c, task.ModelSnapshot, service.SharedPoolEndpointVideo)
	trace.bindAccess(accessKey, task.ReservationRequestID)
	if traceErr := trace.begin(c.Request.Context(), h); traceErr != nil {
		reqLog.Warn("shared_pool.video_status_trace_begin_failed", zap.Error(traceErr))
	}
	upstreamStartedAt := time.Now()
	writerSizeBeforeForward := c.Writer.Size()
	forwardVideoStatus := h.gatewayService.ForwardSharedPoolGrokVideo
	result, forwardErr := forwardVideoStatus(c.Request.Context(), c, accessKey, upstreamRequestID, nil, "")
	trace.markUpstream(upstreamStartedAt)
	if forwardErr != nil || result == nil {
		_, _ = h.updateSharedPoolVideoTaskDetached(c.Request.Context(), service.UpdateSharedPoolMediaTaskStateInput{
			TaskID: task.ID, AccessKeyID: task.AccessKeyID, UpstreamRequestID: task.UpstreamRequestID,
			Status: service.SharedPoolMediaTaskProcessing, LastUpstreamStatus: "poll_failed",
		})
		_ = trace.finalize(c.Request.Context(), h, c, result, true, "", service.SharedPoolSettlementPending, 0, upstreamStartedAt, time.Time{}, http.StatusBadGateway)
		if forwardErr != nil && !openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, forwardErr) {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Shared-pool video status request failed")
		}
		return
	}
	result.VideoDurationSeconds = task.RequestedDurationSeconds
	result.VideoResolution = task.RequestedResolution
	httpStatus := c.Writer.Status()
	if httpStatus < 100 || httpStatus > 599 {
		httpStatus = http.StatusOK
	}
	stateInput := service.UpdateSharedPoolMediaTaskStateInput{
		TaskID: task.ID, AccessKeyID: task.AccessKeyID, UpstreamRequestID: task.UpstreamRequestID,
		LastUpstreamStatus: result.MediaTaskStatus, LastHTTPStatus: &httpStatus,
	}
	settlementOutcome := service.SharedPoolSettlementPending
	settlementAttempts := 0
	failureStage := ""
	forwardSucceeded := true
	switch {
	case !result.MediaTaskTerminal && result.MediaTaskStatus != "unknown":
		stateInput.Status = service.SharedPoolMediaTaskProcessing
		_, err = h.updateSharedPoolVideoTaskDetached(c.Request.Context(), stateInput)
	case result.MediaTaskTerminal && !result.MediaTaskSucceeded:
		stateInput.Status = service.SharedPoolMediaTaskFailed
		stateInput.Reason = "verified_video_task_failed"
		_, err = h.updateSharedPoolVideoTaskDetached(c.Request.Context(), stateInput)
		settlementOutcome = service.SharedPoolSettlementReleased
	case result.MediaTaskTerminal && result.MediaTaskSucceeded && result.MediaOutputObserved:
		settlementAttempts, err = h.recordSharedPoolMediaUsageDetachedWithAttempts(c.Request.Context(), apiKey, accessKey, service.SharedPoolEndpointVideo, result, task.ReservationRequestID, task.RequestedDurationSeconds)
		if err == nil {
			stateInput.Status = service.SharedPoolMediaTaskSucceeded
			_, err = h.updateSharedPoolVideoTaskDetached(c.Request.Context(), stateInput)
		}
		settlementOutcome = service.SharedPoolSettlementSettled
	default:
		stateInput.Status = service.SharedPoolMediaTaskReviewRequired
		stateInput.Reason = "video_terminal_output_or_status_unknown"
		_, err = h.updateSharedPoolVideoTaskDetached(c.Request.Context(), stateInput)
		settlementOutcome = service.SharedPoolSettlementUnknown
	}
	if err != nil {
		failureStage = "settlement"
		forwardSucceeded = false
		settlementOutcome = service.SharedPoolSettlementFailed
		reqLog.Error("shared_pool.video_status_finalize_failed", zap.Error(err))
	}
	if traceErr := trace.finalize(c.Request.Context(), h, c, result, forwardSucceeded, failureStage, settlementOutcome, settlementAttempts, upstreamStartedAt, time.Now(), httpStatus); traceErr != nil {
		reqLog.Warn("shared_pool.video_status_trace_finalize_failed", zap.Error(traceErr))
	}
}

func validateSharedPoolVideoGenerationRequest(body []byte, contentType string, info service.GrokMediaRequestInfo) error {
	if len(body) == 0 || !gjson.ValidBytes(body) || !strings.Contains(strings.ToLower(contentType), "json") {
		return errors.New("shared-pool video generation currently requires a JSON request body")
	}
	if strings.TrimSpace(info.Model) == "" {
		return errors.New("model is required")
	}
	if duration := gjson.GetBytes(body, "duration"); duration.Exists() {
		if duration.Type != gjson.Number || duration.Num != math.Trunc(duration.Num) || duration.Int() < service.VideoBillingMinDurationSeconds || duration.Int() > service.VideoBillingMaxDurationSeconds {
			return errors.New("duration must be an integer between 1 and 15 seconds")
		}
	}
	if n := gjson.GetBytes(body, "n"); n.Exists() && (n.Type != gjson.Number || n.Num != 1) {
		return errors.New("shared-pool video generation supports exactly one video per request")
	}
	if resolution := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "resolution").String())); resolution != "" {
		switch resolution {
		case service.VideoBillingResolution480P, service.VideoBillingResolution720P, service.VideoBillingResolution1080P:
		default:
			return errors.New("resolution must be 480p, 720p, or 1080p")
		}
	}
	return nil
}

func (h *OpenAIGatewayHandler) updateSharedPoolVideoTaskDetached(parent context.Context, input service.UpdateSharedPoolMediaTaskStateInput) (*service.SharedPoolMediaTask, error) {
	base := usageRecordContext(parent, context.Background())
	var task *service.SharedPoolMediaTask
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(base, 10*time.Second)
		task, lastErr = h.gatewayService.UpdateSharedPoolVideoTaskState(ctx, input)
		cancel()
		if lastErr == nil {
			return task, nil
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return nil, lastErr
}

func validateSharedPoolImageCount(body []byte, parsed *service.OpenAIImagesRequest) error {
	if parsed == nil || parsed.N <= 0 || parsed.N > sharedPoolMaximumImageCount {
		return errors.New("image count n must be an integer between 1 and 10")
	}
	if parsed.Multipart || !gjson.ValidBytes(body) {
		return nil
	}
	value := gjson.GetBytes(body, "n")
	if value.Exists() && (value.Type != gjson.Number || value.Num != math.Trunc(value.Num)) {
		return errors.New("image count n must be an integer between 1 and 10")
	}
	return nil
}

func sharedPoolImageReviewReason(result *service.OpenAIForwardResult, requested int, forwardErr error) string {
	switch {
	case result == nil && forwardErr != nil:
		return "image_forward_result_unknown"
	case result == nil:
		return "image_terminal_usage_missing"
	case result.ImageCount <= 0:
		return "image_count_missing"
	case result.ImageCount > requested:
		return "image_count_exceeded_reservation"
	default:
		return "image_settlement_unknown"
	}
}

func handleSharedPoolReservationError(c *gin.Context, h *OpenAIGatewayHandler, reqLog *zap.Logger, accessKey *service.SharedPoolAccessKey, err error) {
	switch {
	case errors.Is(err, service.ErrInsufficientBalance):
		h.errorResponse(c, http.StatusPaymentRequired, "insufficient_balance", "Your balance is not enough to reserve this request.")
	case errors.Is(err, service.ErrSharedPoolUnsafeCostEstimate), errors.Is(err, service.ErrSharedPoolOutputLimitTooHigh):
		h.errorResponse(c, http.StatusUnprocessableEntity, "unsafe_cost_estimate", "This request cannot be safely pre-authorized.")
	case errors.Is(err, service.ErrSharedPoolReservationConflict), errors.Is(err, service.ErrSharedPoolReservationFinalized):
		h.errorResponse(c, http.StatusConflict, "duplicate_request", "This request ID has already been used.")
	default:
		if reqLog != nil {
			fields := []zap.Field{zap.Error(err)}
			if accessKey != nil {
				fields = append(fields, zap.Int64("pool_id", accessKey.PoolID))
			}
			reqLog.Error("shared_pool.media_reserve_failed", fields...)
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "billing_unavailable", "No upstream request was sent and no charge was made.")
	}
}

func (h *OpenAIGatewayHandler) recordSharedPoolMediaUsageDetachedWithAttempts(
	parent context.Context,
	apiKey *service.APIKey,
	accessKey *service.SharedPoolAccessKey,
	endpointType string,
	result *service.OpenAIForwardResult,
	requestID string,
	reservedUnits int,
) (int, error) {
	base := usageRecordContext(parent, context.Background())
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(base, 10*time.Second)
		lastErr = h.gatewayService.RecordSharedPoolMediaUsage(ctx, apiKey, accessKey, endpointType, result, requestID, reservedUnits)
		cancel()
		if lastErr == nil || errors.Is(lastErr, service.ErrSharedPoolReservationFinalized) {
			return attempt + 1, nil
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	return 3, lastErr
}
