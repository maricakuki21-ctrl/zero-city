package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const canonicalSharedKeyContextKey = "canonical_shared_key_context"

type CanonicalSharedKeyContext struct {
	PoolID           int64
	MemberID         int64
	CanonicalGroup   int64
	CanonicalAccount *int64
	BillingPolicy    corecontracts.BillingPolicy
	AcceptedQuote    *service.SharedPoolPriceQuote
}

type canonicalCommitResponseWriter struct {
	gin.ResponseWriter
	boundary *CommitBoundary
}

func (w *canonicalCommitResponseWriter) Write(body []byte) (int, error) {
	return w.boundary.WriteBody(w.ResponseWriter, body)
}

func (w *canonicalCommitResponseWriter) WriteString(body string) (int, error) {
	return w.boundary.WriteBody(w.ResponseWriter, []byte(body))
}

func (w *canonicalCommitResponseWriter) Flush() {
	// Even an empty SSE flush commits headers to the client. It must close the
	// retry boundary just like a body write, rather than replaying another stream.
	_ = w.boundary.ObserveApplicationFrame()
	w.ResponseWriter.Flush()
}

func GetCanonicalSharedKeyContext(c *gin.Context) (CanonicalSharedKeyContext, bool) {
	value, ok := c.Get(canonicalSharedKeyContextKey)
	if !ok {
		return CanonicalSharedKeyContext{}, false
	}
	ctx, ok := value.(CanonicalSharedKeyContext)
	return ctx, ok
}

// CanonicalSharedKeyGatewayContext replaces only the routing identity. The
// request then enters the ordinary Sub2 handler, scheduler, router and usage
// path used by native keys.
func (h *OpenAIGatewayHandler) CanonicalSharedKeyGatewayContext(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || !service.IsSharedPoolAPIKey(apiKey) || !isCanonicalSharedKeyRoute(c.Request) {
		c.Next()
		return
	}
	if h == nil || h.gatewayService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "canonical_gateway_unavailable", "Canonical Sub2 gateway is unavailable")
		c.Abort()
		return
	}

	body, model, responsesEndpoint, compact, err := canonicalSharedKeyRequest(c)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read shared-market request context")
		c.Abort()
		return
	}
	resolved, err := h.gatewayService.ResolveSharedPoolCanonicalGatewayContext(
		c.Request.Context(), apiKey.ID, model, responsesEndpoint, body, compact,
	)
	if err != nil {
		status, code := canonicalSharedKeyError(err)
		h.errorResponse(c, status, code, "Shared-market key has no active canonical Sub2 binding")
		c.Abort()
		return
	}
	if resolved == nil || resolved.Identity == nil || resolved.AccessKey == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "canonical_binding_unavailable", "Shared-market key has no active canonical Sub2 binding")
		c.Abort()
		return
	}

	groupID := int64(resolved.Identity.GroupID)
	canonicalKey := *apiKey
	canonicalKey.GroupID = &groupID
	platform := service.PlatformOpenAI
	if provider := strings.ToLower(strings.TrimSpace(resolved.AccessKey.Provider)); provider == service.PlatformGrok || provider == "xai" {
		platform = service.PlatformGrok
	}
	canonicalKey.Group = &service.Group{
		ID: groupID, Platform: platform, RateMultiplier: 1,
		Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true,
	}
	canonicalKey.SharedPoolManaged = false
	canonicalKey.Key = ""
	c.Set(string(middleware2.ContextKeyAPIKey), &canonicalKey)
	canonicalContext := newCanonicalSharedKeyContext(resolved)
	c.Set(canonicalSharedKeyContextKey, canonicalContext)
	runtime, err := service.NewCanonicalGatewayRuntime(service.CanonicalGatewayRuntimeInput{
		GroupID:       canonicalContext.CanonicalGroup,
		AccountID:     canonicalAccountID(canonicalContext.CanonicalAccount),
		BillingPolicy: canonicalContext.BillingPolicy,
		AcceptedQuote: canonicalContext.AcceptedQuote,
	}, canonicalSharedKeyRetryable)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "canonical_gateway_unavailable", "Canonical Sub2 gateway is unavailable")
		c.Abort()
		return
	}
	c.Request = c.Request.WithContext(service.WithCanonicalGatewayRuntime(c.Request.Context(), runtime))
	boundary, err := NewCommitBoundary(runtime.Commit)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "canonical_gateway_unavailable", "Canonical Sub2 gateway is unavailable")
		c.Abort()
		return
	}
	c.Writer = &canonicalCommitResponseWriter{ResponseWriter: c.Writer, boundary: boundary}
	c.Next()
}

func canonicalAccountID(accountID *int64) int64 {
	if accountID == nil {
		return 0
	}
	return *accountID
}

func canonicalSharedKeyRetryable(err error) bool {
	var failoverErr *service.UpstreamFailoverError
	return errors.As(err, &failoverErr) && failoverErr != nil && failoverErr.ShouldRetryNextAccount()
}

func newCanonicalSharedKeyContext(resolved *service.SharedPoolCanonicalGatewayContext) CanonicalSharedKeyContext {
	context := CanonicalSharedKeyContext{
		PoolID:         resolved.Identity.PoolID,
		MemberID:       resolved.AccessKey.UserID,
		CanonicalGroup: int64(resolved.Identity.GroupID),
		BillingPolicy:  corecontracts.BillingPolicyBizDecipherLedger,
		AcceptedQuote:  resolved.AcceptedQuote,
	}
	if resolved.Identity.AccountID != nil {
		accountID := int64(*resolved.Identity.AccountID)
		context.CanonicalAccount = &accountID
	}
	return context
}

func isCanonicalSharedKeyRoute(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	return corecontracts.ClassifySharedPoolEndpoint(r.Method, r.URL.Path) == corecontracts.SharedPoolEndpointCanonical
}

func canonicalSharedKeyRequest(c *gin.Context) ([]byte, string, bool, bool, error) {
	responsesEndpoint := strings.Contains(c.Request.URL.Path, "/responses")
	if c.Request.Method == http.MethodGet {
		return nil, "", responsesEndpoint, false, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, "", responsesEndpoint, false, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	return body, model, responsesEndpoint, responsesEndpoint && isSharedPoolCompactRequest(c, body), nil
}

func canonicalSharedKeyError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrSharedPoolIdentityUnmapped):
		return http.StatusForbidden, "canonical_binding_unmapped"
	case errors.Is(err, service.ErrSharedPoolIdentityQuarantined):
		return http.StatusForbidden, "canonical_binding_quarantined"
	case errors.Is(err, service.ErrSharedPoolIdentityQueryInvalid):
		return http.StatusBadRequest, "canonical_binding_invalid"
	case errors.Is(err, service.ErrSharedPoolPricingNotConfigured), errors.Is(err, service.ErrSharedPoolPricingUnavailable):
		return http.StatusUnprocessableEntity, "pricing_not_configured"
	default:
		return http.StatusServiceUnavailable, "canonical_binding_unavailable"
	}
}
