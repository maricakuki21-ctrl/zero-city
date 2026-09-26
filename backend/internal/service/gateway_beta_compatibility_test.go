//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBetaCompatibilityMessageControls(t *testing.T) {
	for _, content := range []string{`null`, `""`, `[]`, `[{"type":"text","text":""}]`} {
		body := []byte(`{"output_config":{"effort":"high"},"messages":[{"role":"system","content":` + content + `,"output_config":{"effort":"high"}},{"role":"user","content":"hello","output_config":{"effort":"low"}}]}`)
		out, changed := sanitizeAnthropicBodyForBetaTokens(body, "")
		require.True(t, changed)
		require.Len(t, gjson.GetBytes(out, "messages").Array(), 1)
		require.Equal(t, "hello", gjson.GetBytes(out, "messages.0.content").String())
		require.False(t, gjson.GetBytes(out, "messages.0.output_config").Exists())
		require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
		again, changed := sanitizeAnthropicBodyForBetaTokens(out, "")
		require.False(t, changed)
		require.Equal(t, out, again)
		preserved, changed := sanitizeAnthropicBodyForBetaTokens(body, claude.BetaMidConversationOutputConfig)
		require.False(t, changed)
		require.Equal(t, body, preserved)
	}
}

func TestBetaCompatibilityPreservesRealSystemContent(t *testing.T) {
	for _, content := range []string{`"system"`, `[{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}]`, `[{"future":"block"}]`} {
		body := []byte(`{"messages":[{"role":"system","content":` + content + `,"output_config":{"effort":"high"},"cache_control":{"type":"ephemeral"}}]}`)
		out, changed := sanitizeAnthropicBodyForBetaTokens(body, "")
		require.True(t, changed)
		require.Len(t, gjson.GetBytes(out, "messages").Array(), 1)
		require.Equal(t, content, gjson.GetBytes(out, "messages.0.content").Raw)
		require.True(t, gjson.GetBytes(out, "messages.0.cache_control").Exists())
	}
}

func TestBetaCompatibilityFallbackAndBinding(t *testing.T) {
	body := []byte(`{"context_management":{},"thinking":{"type":"adaptive","block_binding":"strict"},"fallbacks":[{"model":"other"}],"fallback_credit_token":"credit","messages":[{"role":"user","content":"hi"}]}`)
	out, changed := sanitizeAnthropicBodyForBetaTokens(body, "")
	require.True(t, changed)
	for _, path := range []string{"context_management", "thinking.block_binding", "fallbacks", "fallback_credit_token"} {
		require.False(t, gjson.GetBytes(out, path).Exists(), path)
	}
	require.Equal(t, "adaptive", gjson.GetBytes(out, "thinking.type").String())
	all := claude.BetaContextManagement + "," + claude.BetaThinkingBindingControls + "," + claude.BetaServerSideFallback
	preserved, changed := sanitizeAnthropicBodyForBetaTokens(body, all)
	require.False(t, changed)
	require.Equal(t, body, preserved)
	for _, token := range []string{claude.BetaFallbackCredit, claude.BetaFallbackCreditLegacy} {
		out, _ := sanitizeAnthropicBodyForBetaTokens(body, token)
		require.True(t, gjson.GetBytes(out, "fallback_credit_token").Exists())
		require.False(t, gjson.GetBytes(out, "fallbacks").Exists())
	}
	for _, token := range claude.FullClaudeCodeMimicryBetas() {
		require.NotEqual(t, claude.BetaServerSideFallback, token)
		require.NotEqual(t, claude.BetaFallbackCredit, token)
	}
}

func TestBetaCompatibilityRealPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		if enabled {
			c.Request.Header.Set("Anthropic-Beta", claude.BetaMidConversationOutputConfig)
		}
		body := []byte(`{"output_config":{"effort":"high"},"messages":[{"role":"system","content":[],"output_config":{"effort":"high"}},{"role":"user","content":"hello"}]}`)
		svc := &GatewayService{cfg: &config.Config{}}
		req, _, err := svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(context.Background(), c, newAnthropicAPIKeyPassthroughAccountForBetaTest(), body, "token")
		require.NoError(t, err)
		out := readUpstreamBodyForTest(t, req)
		require.Equal(t, enabled, gjson.GetBytes(out, "messages.0.output_config").Exists())
		require.Equal(t, enabled, anthropicBetaTokensContains(getHeaderRaw(req.Header, "anthropic-beta"), claude.BetaMidConversationOutputConfig))
		require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
	}
}
