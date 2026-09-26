package apicompat

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	toolOutputMediaMarker      = "[Tool output media moved to the following user message]"
	toolOutputMediaAttribution = "[Tool output media for call %s]"
)

type toolOutputMediaByCallID map[string][]ChatContentPart

// normalizeResponsesDerivedChatMessageRoles keeps instructions at the start for
// strict Chat upstreams, without changing direct Chat passthrough semantics.
func normalizeResponsesDerivedChatMessageRoles(messages []ChatMessage) []ChatMessage {
	isInstructionRole := func(role string) bool {
		return role == "system" || role == "developer"
	}
	leading := 0
	for leading < len(messages) && isInstructionRole(messages[leading].Role) {
		leading++
	}
	out := make([]ChatMessage, 0, len(messages))
	switch leading {
	case 0:
	case 1:
		out = append(out, messages[0])
	default:
		merged := make([]string, 0, leading)
		for _, m := range messages[:leading] {
			if text := strings.TrimSpace(chatMessageContentText(m.Content)); text != "" {
				merged = append(merged, text)
			}
		}
		if len(merged) > 0 {
			content, _ := json.Marshal(strings.Join(merged, "\n\n"))
			out = append(out, ChatMessage{Role: "system", Content: content})
		}
	}
	for _, m := range messages[leading:] {
		if isInstructionRole(m.Role) {
			m.Role = "user"
		}
		out = append(out, m)
	}
	return out
}

// extractToolOutputMedia only rewrites recognized image shapes. Media-free
// outputs return false so their original bytes are preserved by the caller.
func extractToolOutputMedia(outputRaw json.RawMessage) (string, []ChatContentPart, bool) {
	outputRaw = bytesTrimSpace(outputRaw)
	if len(outputRaw) == 0 || string(outputRaw) == "null" {
		return "", nil, false
	}
	var outputString string
	if err := json.Unmarshal(outputRaw, &outputString); err == nil {
		if isToolOutputImageDataURL(outputString) {
			return toolOutputMediaMarker, []ChatContentPart{toolOutputImagePart(outputString)}, true
		}
		nested, ok := decodeToolOutputJSON([]byte(outputString))
		if !ok {
			return "", nil, false
		}
		rewritten, media, changed := rewriteToolOutputMediaValue(nested)
		if !changed {
			return "", nil, false
		}
		encoded, err := json.Marshal(rewritten)
		if err != nil {
			return "", nil, false
		}
		return string(encoded), media, true
	}
	value, ok := decodeToolOutputJSON(outputRaw)
	if !ok {
		return "", nil, false
	}
	rewritten, media, changed := rewriteToolOutputMediaValue(value)
	if !changed {
		return "", nil, false
	}
	encoded, err := json.Marshal(rewritten)
	if err != nil {
		return "", nil, false
	}
	return string(encoded), media, true
}

func decodeToolOutputJSON(raw []byte) (any, bool) {
	if !json.Valid(raw) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	return value, true
}

func rewriteToolOutputMediaValue(value any) (any, []ChatContentPart, bool) {
	switch typed := value.(type) {
	case []any:
		var media []ChatContentPart
		changed := false
		for i, item := range typed {
			rewritten, itemMedia, itemChanged := rewriteToolOutputMediaValue(item)
			if !itemChanged {
				continue
			}
			typed[i] = rewritten
			media = append(media, itemMedia...)
			changed = true
		}
		return typed, media, changed
	case map[string]any:
		if imageURL, ok := recognizedToolOutputImageURL(typed); ok {
			return map[string]any{"type": "input_text", "text": toolOutputMediaMarker},
				[]ChatContentPart{toolOutputImagePart(imageURL)}, true
		}
		content, ok := typed["content"]
		if !ok {
			return typed, nil, false
		}
		rewritten, media, changed := rewriteToolOutputMediaValue(content)
		if !changed {
			return typed, nil, false
		}
		typed["content"] = rewritten
		return typed, media, true
	default:
		return value, nil, false
	}
}

func recognizedToolOutputImageURL(value map[string]any) (string, bool) {
	partType, _ := value["type"].(string)
	if partType != "input_image" && partType != "image_url" {
		return "", false
	}
	switch imageURL := value["image_url"].(type) {
	case string:
		return imageURL, strings.TrimSpace(imageURL) != ""
	case map[string]any:
		url, _ := imageURL["url"].(string)
		return url, strings.TrimSpace(url) != ""
	default:
		return "", false
	}
}

func isToolOutputImageDataURL(value string) bool {
	const prefix = "data:image/"
	const separator = ";base64,"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	separatorIndex := strings.Index(value[len(prefix):], separator)
	if separatorIndex <= 0 {
		return false
	}
	payloadIndex := len(prefix) + separatorIndex + len(separator)
	return payloadIndex < len(value)
}

func toolOutputImagePart(imageURL string) ChatContentPart {
	return ChatContentPart{Type: "image_url", ImageURL: &ChatImageURL{URL: imageURL}}
}
