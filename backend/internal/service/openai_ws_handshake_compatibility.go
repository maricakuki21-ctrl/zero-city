package service

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Connection handshakes cannot be changed by a later response.create payload.
// Keep the actual outgoing identities isolated without rewriting client IDs.
type openAIWSHandshakeCompatibilityKey struct {
	betaFeatures   string
	installationID string
	windowID       string
	sessionHyphen  string
	sessionID      string
	conversationID string
	threadID       string
}

func normalizeOpenAIWSHandshakeCompatibility(headers http.Header) openAIWSHandshakeCompatibilityKey {
	return openAIWSHandshakeCompatibilityKey{
		betaFeatures:   normalizeOpenAIWSBetaFeatures(headers),
		installationID: normalizeOpenAIWSIdentityHeader(headers, "x-codex-installation-id"),
		windowID:       normalizeOpenAIWSIdentityHeader(headers, "x-codex-window-id"),
		sessionHyphen:  normalizeOpenAIWSIdentityHeader(headers, "session-id"),
		sessionID:      normalizeOpenAIWSIdentityHeader(headers, "session_id"),
		conversationID: normalizeOpenAIWSIdentityHeader(headers, "conversation_id"),
		threadID:       normalizeOpenAIWSIdentityHeader(headers, "thread-id"),
	}
}

func normalizeOpenAIWSIdentityHeader(headers http.Header, name string) string {
	var keys []string
	for key := range headers {
		if strings.EqualFold(key, name) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var values []string
	for _, key := range keys {
		values = append(values, headers[key]...)
	}
	if len(values) == 0 {
		return ""
	}
	// Preserve repeated-value ordering and whitespace as sent on the wire.
	// Length prefixes avoid collisions between empty, repeated and joined values.
	var normalized strings.Builder
	for _, value := range values {
		normalized.WriteString(strconv.Itoa(len(value)))
		normalized.WriteByte(':')
		normalized.WriteString(value)
	}
	return normalized.String()
}
