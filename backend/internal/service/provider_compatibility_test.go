package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenCodeOfficialSessionCompatibility(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rememberOpenCodeInboundBody(c, []byte(`{"metadata":{"user_id":"{\"session_id\":\"stable-conversation\"}"}}`))
	rememberOpenCodeInboundBody(c, []byte(`{"prompt_cache_key":"converted-body"}`))
	headers := http.Header{}
	applyOpenCodeSessionHeader(c, account, "https://opencode.ai/zen/go/v1/responses", headers)
	require.Equal(t, "stable-conversation", headers.Get(openCodeSessionHeader))
	c.Request.Header.Set(openCodeSessionHeader, "explicit-session")
	applyOpenCodeSessionHeader(c, account, "https://opencode.ai/zen/go/v1/chat/completions", headers)
	require.Equal(t, "explicit-session", headers.Get(openCodeSessionHeader))
	for _, target := range []string{"http://opencode.ai/zen/go", "https://opencode.ai.evil.test/zen/go", "https://other.test", "https://user@opencode.ai"} {
		headers := http.Header{}
		applyOpenCodeSessionHeader(c, account, target, headers)
		require.Empty(t, headers.Get(openCodeSessionHeader), target)
	}
}

func TestOpenCodeSessionGenerationAndInputValidation(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey}
	headers := http.Header{}
	applyOpenCodeSessionHeader(nil, account, "https://opencode.ai/zen/go/v1/messages", headers)
	require.NotEmpty(t, headers.Get(openCodeSessionHeader))
	headers = http.Header{}
	applyOpenCodeSessionHeader(nil, account, "https://opencode.ai/zen/v1/messages", headers)
	require.Empty(t, headers.Get(openCodeSessionHeader))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Header[openCodeSessionHeader] = []string{"bad\r\nheader"}
	applyOpenCodeSessionHeader(c, account, "https://opencode.ai/zen/go/v1/messages", headers, []byte(`{"prompt_cache_key":"good"}`))
	require.Equal(t, "good", headers.Get(openCodeSessionHeader))
}

func TestOllamaProviderTokenCompatibility(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey}
	body := []byte(`{"model":"deepseek-v4","max_tokens":100000,"max_completion_tokens":90000,"max_output_tokens":80000}`)
	for _, path := range []string{"chat/completions", "responses", "messages"} {
		out := clampOllamaCloudMaxTokensForURL(account, "https://ollama.com/v1/"+path, body)
		for _, key := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
			require.Equal(t, int64(65535), gjson.GetBytes(out, key).Int())
		}
	}
	for _, target := range []string{"https://ollama.com.evil.test/v1/messages", "http://ollama.com/v1/messages", "https://ollama.com:8443/v1/messages", "https://user@ollama.com/v1/messages"} {
		require.Equal(t, body, clampOllamaCloudMaxTokensForURL(account, target, body))
	}
	account.Extra = map[string]any{OllamaCloudMaxTokensCapExtraKey: 0}
	require.Equal(t, body, clampOllamaCloudMaxTokensForURL(account, "https://ollama.com/v1/messages", body))
}

func TestOllamaReasoningWireCompatibility(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://ollama.com"}}
	body := []byte(`{"messages":[{"role":"assistant","reasoning_content":"reasoning","content":"answer"},{"role":"user","content":"next"}]}`)
	out := normalizeOllamaCloudChatRequest(account, body)
	require.Equal(t, "reasoning", gjson.GetBytes(out, "messages.0.reasoning").String())
	require.Equal(t, "answer", gjson.GetBytes(out, "messages.0.content").String())
	for _, container := range []string{"message", "delta"} {
		raw := `{"choices":[{"` + container + `":{"thinking":"trace","content":"answer"}}],"usage":{"prompt_tokens":12}}`
		out := normalizeOllamaCloudChatResponse([]byte(raw))
		require.Equal(t, "trace", gjson.GetBytes(out, "choices.0."+container+".reasoning_content").String())
		require.Equal(t, int64(12), gjson.GetBytes(out, "usage.prompt_tokens").Int())
		line := normalizeOllamaCloudSSELine(account, "data: "+raw)
		require.Contains(t, line, `"reasoning_content":"trace"`)
	}
	require.Equal(t, "data: [DONE]", normalizeOllamaCloudSSELine(account, "data: [DONE]"))
	raw := []byte(`{"choices":[{"message":{"reasoning":"trace","reasoning_content":"original"}}]}`)
	require.Equal(t, raw, normalizeOllamaCloudChatResponse(raw))
}
