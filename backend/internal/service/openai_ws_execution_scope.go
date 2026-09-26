package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Adapted from Sub2 v0.2.5 execution scopes. Account affinity remains unchanged;
// thread-scoped connection state must not be shared by a parent and its agents.
const (
	openAIWSThreadIDHeader = "thread-id"
	openAIWSWindowIDHeader = "x-codex-window-id"
	openAISubagentHeader   = "x-openai-subagent"
)

type codexTurnMetadata struct {
	ThreadID    string `json:"thread_id"`
	RequestKind string `json:"request_kind"`
}

func parseCodexTurnMetadata(raw string) (codexTurnMetadata, bool) {
	var metadata codexTurnMetadata
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &metadata); err != nil {
		return metadata, false
	}
	metadata.ThreadID = strings.TrimSpace(metadata.ThreadID)
	metadata.RequestKind = strings.ToLower(strings.TrimSpace(metadata.RequestKind))
	return metadata, true
}

func openAIWSExecutionTurnMetadata(c *gin.Context, body []byte) codexTurnMetadata {
	if c != nil && c.Request != nil {
		if metadata, ok := parseCodexTurnMetadata(c.GetHeader(openAIWSTurnMetadataHeader)); ok {
			return metadata
		}
	}
	metadata, _ := parseCodexTurnMetadata(gjson.GetBytes(body, "client_metadata."+openAIWSTurnMetadataHeader).String())
	return metadata
}

func resolveOpenAIWSClientThreadID(c *gin.Context, body []byte) string {
	if c != nil && c.Request != nil {
		if id := strings.TrimSpace(c.GetHeader(openAIWSThreadIDHeader)); id != "" {
			return id
		}
		if metadata, ok := parseCodexTurnMetadata(c.GetHeader(openAIWSTurnMetadataHeader)); ok && metadata.ThreadID != "" {
			return metadata.ThreadID
		}
		if window := strings.TrimSpace(c.GetHeader(openAIWSWindowIDHeader)); window != "" {
			if id := strings.TrimSpace(strings.SplitN(window, ":", 2)[0]); id != "" {
				return id
			}
		}
	}
	if id := strings.TrimSpace(gjson.GetBytes(body, "client_metadata.thread_id").String()); id != "" {
		return id
	}
	metadata, _ := parseCodexTurnMetadata(gjson.GetBytes(body, "client_metadata."+openAIWSTurnMetadataHeader).String())
	return metadata.ThreadID
}

func resolveOpenAIWSExecutionLane(c *gin.Context, body []byte) string {
	metadata := openAIWSExecutionTurnMetadata(c, body)
	switch metadata.RequestKind {
	case "", "turn", "prewarm", "compaction":
	default:
		return "kind=" + metadata.RequestKind
	}
	if metadata.ThreadID == "" {
		subagent := ""
		if c != nil && c.Request != nil {
			subagent = strings.TrimSpace(c.GetHeader(openAISubagentHeader))
		}
		if subagent == "" {
			subagent = strings.TrimSpace(gjson.GetBytes(body, "client_metadata."+openAISubagentHeader).String())
		}
		if subagent != "" {
			return "subagent=" + strings.ToLower(subagent)
		}
	}
	return ""
}

func resolveOpenAIWSExecutionScope(c *gin.Context, body []byte, apiKeyID int64) (scope, threadID string) {
	lane := resolveOpenAIWSExecutionLane(c, body)
	identity := "thread"
	threadID = resolveOpenAIWSClientThreadID(c, body)
	value := threadID
	if value == "" {
		identity = "session"
		value = strings.TrimSpace(explicitOpenAIRequestSessionID(c, body))
	}
	if value == "" {
		return "", ""
	}
	seed := fmt.Sprintf("openai_ws_exec:%d|%s=%s", apiKeyID, identity, value)
	if lane != "" {
		seed += "|" + lane
	}
	scope, _ = deriveOpenAISessionHashes(seed)
	return scope, threadID
}
