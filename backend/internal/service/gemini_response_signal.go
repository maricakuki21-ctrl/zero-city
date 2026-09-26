package service

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type geminiResponseSignalKind int

const (
	geminiSignalNone geminiResponseSignalKind = iota
	geminiSignalPromptBlocked
	geminiSignalContentFilter
	geminiSignalAbnormalStop
	geminiSignalError
)

type geminiResponseSignal struct {
	Kind    geminiResponseSignalKind
	Reason  string
	Message string
	Status  int
	Detail  string
}

// Request-local normalized observation; Ops persistence carries explicit scope.
const geminiResponseSignalContextKey = "gemini_response_signal"

func detectGeminiResponseSignal(payload []byte) (geminiResponseSignal, bool) {
	if !gjson.ValidBytes(payload) {
		return geminiResponseSignal{}, false
	}
	if inner := gjson.GetBytes(payload, "response"); inner.IsObject() {
		payload = []byte(inner.Raw)
	}
	if errObj := gjson.GetBytes(payload, "error"); errObj.IsObject() {
		reason := strings.ToUpper(strings.TrimSpace(errObj.Get("status").String()))
		status := int(errObj.Get("code").Int())
		if status < 400 || status > 599 {
			status = geminiGoogleStatusToHTTPStatus(reason)
		}
		if reason == "" {
			reason = "UPSTREAM_ERROR"
		}
		message := strings.TrimSpace(errObj.Get("message").String())
		if message == "" {
			message = "Gemini upstream returned an error event"
		}
		return geminiResponseSignal{geminiSignalError, reason, message, status, truncateString(string(payload), 2048)}, true
	}
	reason := strings.ToUpper(strings.TrimSpace(gjson.GetBytes(payload, "promptFeedback.blockReason").String()))
	if reason != "" && reason != "BLOCKED_REASON_UNSPECIFIED" {
		return geminiResponseSignal{Kind: geminiSignalPromptBlocked, Reason: reason,
			Message: fmt.Sprintf("Gemini content policy block (blockReason=%s): prompt was blocked before generation", reason),
			Status:  http.StatusBadRequest}, true
	}
	var primary gjson.Result
	gjson.GetBytes(payload, "candidates").ForEach(func(_, candidate gjson.Result) bool {
		if index := candidate.Get("index"); index.Exists() && index.Int() != 0 {
			return true
		}
		primary = candidate
		return false
	})
	reason = strings.ToUpper(strings.TrimSpace(primary.Get("finishReason").String()))
	switch reason {
	case "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII",
		"IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_RECITATION":
		message := strings.TrimSpace(primary.Get("finishMessage").String())
		if message == "" {
			message = "Response stopped due to content policy."
		}
		return geminiResponseSignal{Kind: geminiSignalContentFilter, Reason: reason,
			Message: fmt.Sprintf("Gemini content policy stop (finishReason=%s): %s", reason, truncateString(message, 512)),
			Status:  http.StatusBadRequest}, true
	default:
		return geminiResponseSignal{}, false
	}
}

func detectGeminiResponseSignalInBody(body []byte) (geminiResponseSignal, bool) {
	if !gjson.ValidBytes(body) {
		return geminiResponseSignal{}, false
	}
	parsed := gjson.ParseBytes(body)
	if !parsed.IsArray() {
		return detectGeminiResponseSignal(body)
	}
	var best geminiResponseSignal
	parsed.ForEach(func(_, item gjson.Result) bool {
		if signal, ok := detectGeminiResponseSignal([]byte(item.Raw)); ok && signal.Kind > best.Kind {
			best = signal
		}
		return true
	})
	return best, best.Kind != geminiSignalNone
}

func isGeminiEmptyResponseBody(body []byte) bool {
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	if !gjson.ValidBytes(body) {
		return false
	}
	parsed := gjson.ParseBytes(body)
	if parsed.IsArray() {
		empty := true
		parsed.ForEach(func(_, item gjson.Result) bool {
			empty = isGeminiEmptyResponseObject(item)
			return empty
		})
		return empty
	}
	return isGeminiEmptyResponseObject(parsed)
}

func isGeminiEmptyResponseObject(item gjson.Result) bool {
	if !item.IsObject() {
		return false
	}
	if inner := item.Get("response"); inner.IsObject() {
		item = inner
	}
	empty := true
	item.ForEach(func(_, _ gjson.Result) bool { empty = false; return false })
	if empty {
		return true
	}
	candidates := item.Get("candidates")
	return candidates.IsArray() && len(candidates.Array()) == 0 && !item.Get("promptFeedback").Exists()
}

func geminiGoogleStatusToHTTPStatus(status string) int {
	switch status {
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION", "OUT_OF_RANGE":
		return http.StatusBadRequest
	case "UNAUTHENTICATED":
		return http.StatusUnauthorized
	case "PERMISSION_DENIED":
		return http.StatusForbidden
	case "NOT_FOUND":
		return http.StatusNotFound
	case "RESOURCE_EXHAUSTED":
		return http.StatusTooManyRequests
	case "INTERNAL", "UNKNOWN", "DATA_LOSS":
		return http.StatusInternalServerError
	case "UNAVAILABLE":
		return http.StatusServiceUnavailable
	case "DEADLINE_EXCEEDED":
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}

func recordGeminiResponseSignal(c *gin.Context, account *Account, signal geminiResponseSignal, stream bool, requestID string) {
	if c == nil || signal.Kind == geminiSignalNone {
		return
	}
	signal.Message = sanitizeUpstreamErrorMessage(signal.Message)
	// Do not expose raw response contents through context by default.
	signal.Detail = ""
	c.Set(geminiResponseSignalContextKey, signal)
	if signal.Kind == geminiSignalPromptBlocked || signal.Kind == geminiSignalContentFilter {
		MarkOpsStreamErrorValue(c, OpsStreamError{
			ErrType: "invalid_request_error", Code: signal.Reason, Message: signal.Message,
			IntendedStatus: signal.Status, RequestScoped: true, NonStream: !stream,
		})
		return
	}
	setOpsUpstreamError(c, signal.Status, signal.Message, "")
	kind := "http_error"
	if stream {
		kind = "stream_failed"
	}
	event := OpsUpstreamErrorEvent{
		Platform: PlatformGemini, UpstreamStatusCode: signal.Status,
		UpstreamRequestID: strings.TrimSpace(requestID), Kind: kind, Message: signal.Message,
	}
	if account != nil {
		event.Platform, event.AccountID, event.AccountName = account.Platform, account.ID, account.Name
	}
	appendOpsUpstreamError(c, event)
	MarkOpsStreamErrorValue(c, OpsStreamError{
		ErrType: "upstream_error", Code: signal.Reason, Message: signal.Message,
		IntendedStatus: signal.Status, CountTowardsSLA: true, NonStream: !stream,
	})
}
