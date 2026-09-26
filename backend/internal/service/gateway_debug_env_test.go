package service

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseDebugEnvBool(t *testing.T) {
	t.Run("empty is false", func(t *testing.T) {
		if parseDebugEnvBool("") {
			t.Fatalf("expected false for empty string")
		}
	})

	t.Run("true-like values", func(t *testing.T) {
		for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
			t.Run(value, func(t *testing.T) {
				if !parseDebugEnvBool(value) {
					t.Fatalf("expected true for %q", value)
				}
			})
		}
	})

	t.Run("false-like values", func(t *testing.T) {
		for _, value := range []string{"0", "false", "off", "debug"} {
			t.Run(value, func(t *testing.T) {
				if parseDebugEnvBool(value) {
					t.Fatalf("expected false for %q", value)
				}
			})
		}
	})
}

func TestClaudeMimicDebugLineSummarizesCacheWithoutLeakingPromptText(t *testing.T) {
	body := []byte(`{
		"system":[
			{"type":"text","text":"SECRET SYSTEM INSTRUCTIONS","cache_control":{"type":"ephemeral","ttl":"1h"}},
			{"type":"thinking","thinking":"SECRET THINKING","cache_control":{"type":"ephemeral"}}
		],
		"metadata":{"user_id":"user_secret_session"},
		"messages":[{"role":"user","content":[{"type":"text","text":"SECRET USER MESSAGE","cache_control":{"type":"ephemeral","ttl":"5m"}}]}],
		"tools":[{"name":"lookup","input_schema":{},"cache_control":{"type":"ephemeral"}}]
	}`)
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("User-Agent", "opencode/1.0")

	line := buildClaudeMimicDebugLine(req, body, &Account{ID: 42, Name: "claude-oauth"}, "oauth", true)

	for _, forbidden := range []string{
		"SECRET SYSTEM INSTRUCTIONS",
		"SECRET THINKING",
		"SECRET USER MESSAGE",
		"user_secret_session",
		"secret-token",
		"system.preview",
	} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("debug line leaked %q: %s", forbidden, line)
		}
	}

	for _, expected := range []string{
		"meta.user_id_sha256=",
		"system={kind=array blocks=2 text_blocks=1 thinking_blocks=1 sha256=",
		"cache_control={total=3 system=1 messages=1 tools=1 invalid_thinking=1",
		"system.0.cache_control:ephemeral/1h",
		"messages.0.content.0.cache_control:ephemeral/5m",
		"tools.0.cache_control:ephemeral/-",
		"invalid=[system.1.cache_control]",
		"authorization=\"Bearer [redacted]\"",
	} {
		if !strings.Contains(line, expected) {
			t.Fatalf("debug line missing %q: %s", expected, line)
		}
	}
}
