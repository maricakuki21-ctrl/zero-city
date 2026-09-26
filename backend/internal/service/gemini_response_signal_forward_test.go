package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiResponseSignal_CompatForwardPaths(t *testing.T) {
	for _, oauth := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("oauth=%t/native=%t/stream=%t", oauth, native, stream), func(t *testing.T) {
					upstreamBody := `{"candidates":[{"content":{"parts":[{"text":"answer"}]},"finishReason":"SAFETY"}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}`
					if oauth {
						upstreamBody = `{"response":` + upstreamBody + `}`
					}
					if stream || oauth {
						upstreamBody = "data: " + upstreamBody + "\n\n"
					}
					stub := &geminiCompatHTTPUpstreamStub{response: &http.Response{
						StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(upstreamBody)),
					}}
					svc := &GeminiMessagesCompatService{httpUpstream: stub, cfg: &config.Config{}, tokenProvider: &GeminiTokenProvider{}}
					account := &Account{ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1,
						Credentials: map[string]any{"api_key": "test-key", "access_token": "test-token", "project_id": "project"}}
					if oauth {
						account.Type = AccountTypeOAuth
					}
					recorder := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(recorder)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
					var result *ForwardResult
					var err error
					if native {
						result, err = svc.ForwardNative(context.Background(), c, account, "gemini-2.5-flash",
							"generateContent", stream, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
					} else {
						body := []byte(fmt.Sprintf(`{"model":"gemini-2.5-flash","max_tokens":16,"stream":%t,"messages":[{"role":"user","content":"hi"}]}`, stream))
						result, err = svc.Forward(context.Background(), c, account, body)
					}
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, 7, result.Usage.InputTokens)
					require.Equal(t, 3, result.Usage.OutputTokens)
					value, found := c.Get(geminiResponseSignalContextKey)
					require.True(t, found)
					require.Equal(t, geminiSignalContentFilter, value.(geminiResponseSignal).Kind)
					_, attributed := c.Get(OpsUpstreamErrorsKey)
					require.False(t, attributed)
					require.Equal(t, http.StatusOK, recorder.Code)
					require.Equal(t, 1, stub.calls)
				})
			}
		}
	}
}

func TestGeminiResponseSignal_AntigravityForwardPaths(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("native=%t/stream=%t", native, stream), func(t *testing.T) {
				upstreamBody := "data: " + `{"response":{"candidates":[{"content":{"parts":[{"text":"answer"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}}` + "\n\n" +
					"data: " + `{"error":{"code":503,"status":"UNAVAILABLE","message":"unavailable"}}` + "\n\n"
				stub := &queuedHTTPUpstreamStub{responses: []*http.Response{{
					StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(upstreamBody)),
				}}}
				svc := &AntigravityGatewayService{
					settingService: NewSettingService(&antigravitySettingRepoStub{}, &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}),
					tokenProvider:  &AntigravityTokenProvider{}, httpUpstream: stub,
				}
				account := &Account{ID: 101, Name: "test", Platform: PlatformAntigravity, Type: AccountTypeOAuth, Status: StatusActive, Concurrency: 1,
					Credentials: map[string]any{"access_token": "token", "project_id": "project",
						"model_mapping": map[string]any{"gemini-2.5-flash": "gemini-2.5-flash"}}}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				var result *ForwardResult
				var err error
				if native {
					body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"tools":[{"functionDeclarations":[{"name":"weather"}]},{"googleSearch":{}},{"codeExecution":{}}]}`)
					result, err = svc.ForwardGemini(context.Background(), c, account, "gemini-2.5-flash", "generateContent", stream, body, false)
				} else {
					body := []byte(fmt.Sprintf(`{"model":"gemini-2.5-flash","max_tokens":16,"stream":%t,"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"weather","input_schema":{"type":"object"}},{"type":"web_search"},{"type":"code_execution"}]}`, stream))
					result, err = svc.Forward(context.Background(), c, account, body, false)
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 7, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
				require.Equal(t, "gemini-2.5-flash", result.UpstreamModel)
				require.Equal(t, 503, c.GetInt(OpsUpstreamStatusCodeKey))
				value, found := c.Get(geminiResponseSignalContextKey)
				require.True(t, found)
				require.Equal(t, geminiSignalError, value.(geminiResponseSignal).Kind)
				require.Len(t, stub.requestBodies, 1)
				request := stub.requestBodies[0]
				require.Equal(t, "gemini-2.5-flash", gjson.GetBytes(request, "model").String())
				require.Len(t, gjson.GetBytes(request, "request.tools").Array(), 1)
				require.Equal(t, "weather", gjson.GetBytes(request, "request.tools.0.functionDeclarations.0.name").String())
				require.False(t, gjson.GetBytes(request, "request.tools.0.googleSearch").Exists())
				require.False(t, gjson.GetBytes(request, "request.tools.0.codeExecution").Exists())
			})
		}
	}
}
