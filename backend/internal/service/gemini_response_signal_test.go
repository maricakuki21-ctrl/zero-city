package service

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiResponseSignal_Detection(t *testing.T) {
	for _, tc := range []struct {
		body string
		kind geminiResponseSignalKind
		code int
	}{
		{`{"error":{"status":"RESOURCE_EXHAUSTED","message":"quota"}}`, geminiSignalError, 429},
		{`{"response":{"error":{"code":503,"message":"unavailable"}}}`, geminiSignalError, 503},
		{`[{"promptFeedback":{"blockReason":"SAFETY"}},{"error":{"status":"INTERNAL"}}]`, geminiSignalError, 500},
		{`{"promptFeedback":{"blockReason":"SAFETY"}}`, geminiSignalPromptBlocked, 400},
		{`{"candidates":[{"index":1,"finishReason":"SAFETY"},{"index":0,"finishReason":"STOP"}]}`, geminiSignalNone, 0},
		{`{"candidates":[{"finishReason":"SAFETY"}]}`, geminiSignalContentFilter, 400},
		{`{"candidates":[{"finishReason":"MAX_TOKENS"}]}`, geminiSignalNone, 0},
		{`{"candidates":[{"finishReason":"MALFORMED_FUNCTION_CALL"}]}`, geminiSignalNone, 0},
		{`{"candidates":[{"finishReason":"OTHER"}]}`, geminiSignalNone, 0},
		{`{"promptFeedback":{"blockReason":"BLOCKED_REASON_UNSPECIFIED"}}`, geminiSignalNone, 0},
		{`{"error":`, geminiSignalNone, 0},
	} {
		t.Run(tc.body, func(t *testing.T) {
			signal, found := detectGeminiResponseSignalInBody([]byte(tc.body))
			require.Equal(t, tc.kind != geminiSignalNone, found)
			require.Equal(t, tc.kind, signal.Kind)
			require.Equal(t, tc.code, signal.Status)
		})
	}
}

func TestGeminiResponseSignal_Observer(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		sse  bool
		kind geminiResponseSignalKind
		code string
	}{
		{"json empty", " \n{}", false, geminiSignalAbnormalStop, "EMPTY_RESPONSE"},
		{"array empty", `[{},{"response":{"candidates":[]}}]`, false, geminiSignalAbnormalStop, "EMPTY_RESPONSE"},
		{"stream empty", ": ping\r\n\r\n", true, geminiSignalAbnormalStop, "EMPTY_STREAM"},
		{"multiline error", "data: {\"error\":\r\ndata: {\"code\":503}}\r\n\r\n", true, geminiSignalError, "UPSTREAM_ERROR"},
		{"no final newline", "data: {\"error\":{\"code\":503}}", true, geminiSignalError, "UPSTREAM_ERROR"},
		{"priority", "data: {\"candidates\":[{\"finishReason\":\"SAFETY\"}]}\n\ndata: {\"error\":{\"code\":503}}\n\n", true, geminiSignalError, "UPSTREAM_ERROR"},
		{"json fallback", "{\"error\":{\"code\":503}}\n", true, geminiSignalError, "UPSTREAM_ERROR"},
		{"done marker", "data: [DONE]\n\n", true, geminiSignalNone, ""},
		{"normal data", "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hi\"}]}}]}\n\n", true, geminiSignalNone, ""},
		{"bounded fallback", strings.Repeat("x", geminiSignalBufferLimit+1), true, geminiSignalNone, ""},
		{"bounded json", strings.Repeat(" ", geminiSignalBufferLimit+1), false, geminiSignalNone, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observer := &geminiResponseObserver{ReadCloser: io.NopCloser(strings.NewReader(tc.body)), sse: tc.sse}
			var out strings.Builder
			_, err := io.CopyBuffer(&out, observer, make([]byte, 7))
			require.NoError(t, err)
			require.Equal(t, tc.body, out.String())
			signal := observer.finish()
			require.Equal(t, tc.kind, signal.Kind)
			require.Equal(t, tc.code, signal.Reason)
			require.Equal(t, geminiSignalNone, observer.finish().Kind)
		})
	}
}

type geminiSignalPartialReader struct{}

func (geminiSignalPartialReader) Read([]byte) (int, error) { return 0, errors.New("read interrupted") }
func (geminiSignalPartialReader) Close() error             { return nil }

func TestGeminiResponseSignal_PartialAndConcurrentFinalization(t *testing.T) {
	observer := &geminiResponseObserver{ReadCloser: geminiSignalPartialReader{}, sse: true}
	_, err := io.ReadAll(observer)
	require.Error(t, err)
	require.Equal(t, geminiSignalNone, observer.finish().Kind)

	reader, writer := io.Pipe()
	observer = &geminiResponseObserver{ReadCloser: reader, sse: true}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = io.Copy(io.Discard, observer) }()
	require.Equal(t, geminiSignalNone, observer.finish().Kind)
	_, err = io.WriteString(writer, "data: {}\n\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	wg.Wait()
}

func TestGeminiResponseSignal_NativeHandlerPreservesBodyAndUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, policy := range []bool{false, true} {
			body := `{"error":{"code":503,"message":"unavailable"},"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}`
			if policy {
				body = `{"candidates":[{"finishReason":"SAFETY"}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}`
			}
			if stream {
				body = "data: " + body + "\n\n"
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/test:generateContent", nil)
			resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
			account := &Account{ID: 42, Name: "test", Platform: PlatformGemini}
			finalize := observeGeminiResponse(c, resp, account, stream, stream, "request-1")
			service := &GeminiMessagesCompatService{}
			var usage *ClaudeUsage
			if stream {
				result, err := service.handleNativeStreamingResponse(c, resp, time.Now(), false)
				require.NoError(t, err)
				usage = result.usage
			} else {
				var err error
				usage, err = service.handleNativeNonStreamingResponse(c, resp, false)
				require.NoError(t, err)
			}
			finalize()
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, body, recorder.Body.String())
			require.Equal(t, 7, usage.InputTokens)
			require.Equal(t, 3, usage.OutputTokens)
			value, found := c.Get(geminiResponseSignalContextKey)
			require.True(t, found)
			signal := value.(geminiResponseSignal)
			if policy {
				require.Equal(t, geminiSignalContentFilter, signal.Kind)
				_, attributed := c.Get(OpsUpstreamStatusCodeKey)
				require.False(t, attributed)
				marker, marked := GetOpsStreamError(c)
				require.True(t, marked)
				require.True(t, marker.RequestScoped)
				require.False(t, marker.CountTowardsSLA)
				require.Equal(t, !stream, marker.NonStream)
			} else {
				require.Equal(t, geminiSignalError, signal.Kind)
				require.Equal(t, 503, c.GetInt(OpsUpstreamStatusCodeKey))
				marked, found := GetOpsStreamError(c)
				require.True(t, found)
				require.True(t, marked.CountTowardsSLA)
				require.False(t, marked.RequestScoped)
				require.Equal(t, !stream, marked.NonStream)
			}
		}
	}
}
