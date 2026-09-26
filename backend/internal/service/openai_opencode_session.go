package service

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const openCodeSessionHeader = "X-OpenCode-Session"
const openCodeInboundBodyContextKey = "opencode_inbound_body"

func rememberOpenCodeInboundBody(c *gin.Context, body []byte) {
	if c == nil || len(body) == 0 {
		return
	}
	if _, exists := c.Get(openCodeInboundBodyContextKey); !exists {
		c.Set(openCodeInboundBodyContextKey, body)
	}
}

// Restrict automatic session disclosure to the official HTTPS destination.
// Existing generic API-key accounts work without creating a duplicate platform.
func applyOpenCodeSessionHeader(c *gin.Context, account *Account, target string, headers http.Header, bodies ...[]byte) {
	if account == nil || account.Type != AccountTypeAPIKey || headers == nil {
		return
	}
	u, err := url.Parse(target)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Hostname(), "opencode.ai") || u.User != nil {
		return
	}
	candidates := make([]string, 0, len(bodies)+4)
	if c != nil && c.Request != nil {
		candidates = append(candidates, c.GetHeader(openCodeSessionHeader), explicitOpenAIHeaderSessionID(c), c.GetHeader("x-claude-code-session-id"))
		if raw, exists := c.Get(openCodeInboundBodyContextKey); exists {
			if body, ok := raw.([]byte); ok {
				bodies = append([][]byte{body}, bodies...)
			}
		}
	}
	for _, body := range bodies {
		key := gjson.GetBytes(body, "prompt_cache_key").String()
		if strings.TrimSpace(key) == "" {
			key = gjson.GetBytes(body, "metadata.user_id").String()
			if strings.HasPrefix(strings.TrimSpace(key), "{") {
				if session := gjson.Get(key, "session_id").String(); strings.TrimSpace(session) != "" {
					key = session
				}
			}
		}
		candidates = append(candidates, key)
	}
	for key, values := range headers {
		if strings.EqualFold(key, openCodeSessionHeader) && len(values) > 0 {
			candidates = append(candidates, values[0])
		}
	}
	session := ""
	for _, candidate := range candidates {
		// Validate before trimming so header injection cannot become a valid ID.
		if strings.ContainsAny(candidate, "\r\n\x00") {
			continue
		}
		if session = sanitizeOpenCodeSessionID(candidate); session != "" {
			break
		}
	}
	if session == "" && (u.Path == "/zen/go" || strings.HasPrefix(u.Path, "/zen/go/")) {
		session = uuid.NewString()
	}
	if session == "" {
		return
	}
	for key := range headers {
		if strings.EqualFold(key, openCodeSessionHeader) {
			delete(headers, key)
		}
	}
	headers.Set(openCodeSessionHeader, session)
}

func sanitizeOpenCodeSessionID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		return ""
	}
	for _, c := range value {
		if c < 33 || c > 126 {
			return ""
		}
	}
	return value
}
