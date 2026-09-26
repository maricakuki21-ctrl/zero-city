package handler

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"time"
)

// Codex bootstrap outputs are client-injected user input, not responses to an
// unpaired tool call. Only normalize the exact upstream bootstrap contract.
func normalizeCodexAutomationBootstrap(body []byte) ([]byte, bool) {
	if !automationUniqueJSON(body) {
		return body, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var request map[string]any
	if decoder.Decode(&request) != nil {
		return body, false
	}
	if previous, exists := request["previous_response_id"]; exists {
		text, ok := previous.(string)
		if !ok || strings.TrimSpace(text) != "" {
			return body, false
		}
	}
	input, ok := request["input"].([]any)
	if !ok {
		return body, false
	}
	candidate := func(item map[string]any) bool {
		if item["type"] != "function_call_output" || item["namespace"] != "codex_app" || item["name"] != "automation_update" {
			return false
		}
		output, ok := item["output"].(string)
		return ok && (validCodexAutomationBootstrap(output) || validCodexAutomationHeartbeat(output))
	}
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if candidate(item) {
			if id, exists := item["call_id"]; exists {
				text, ok := id.(string)
				if !ok || strings.TrimSpace(text) != "" {
					return body, false
				}
			}
			continue
		}
		typ, _ := item["type"].(string)
		if typ == "item_reference" || typ == "tool_search_output" || strings.HasSuffix(typ, "_call") || strings.HasSuffix(typ, "_call_output") {
			return body, false
		}
	}
	changed := false
	for i, raw := range input {
		item, ok := raw.(map[string]any)
		if ok && candidate(item) {
			input[i] = map[string]any{
				"type": "message", "role": "user",
				"content": []any{map[string]any{"type": "input_text", "text": item["output"]}},
			}
			changed = true
		}
	}
	if !changed {
		return body, false
	}
	out, err := json.Marshal(request)
	if err != nil {
		return body, false
	}
	return out, true
}

func automationUniqueJSON(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var consume func() bool
	consume = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			members := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok {
					return false
				}
				if _, duplicate := members[key]; duplicate {
					return false
				}
				members[key] = struct{}{}
				if !consume() {
					return false
				}
			}
			end, err := decoder.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for decoder.More() {
				if !consume() {
					return false
				}
			}
			end, err := decoder.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !consume() {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func validCodexAutomationID(value string) bool {
	if len(value) == 0 || len(value) > 128 || value == "." || value == ".." {
		return false
	}
	for _, c := range value {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func validCodexAutomationBootstrap(value string) bool {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	if strings.ContainsRune(value, '\r') {
		return false
	}
	lines := strings.Split(value, "\n")
	if len(lines) < 6 {
		return false
	}
	header := func(line, prefix string) (string, bool) {
		text := strings.TrimPrefix(line, prefix)
		return text, strings.HasPrefix(line, prefix) && text != "" && text == strings.TrimSpace(text)
	}
	if _, ok := header(lines[0], "Automation: "); !ok {
		return false
	}
	id, ok := header(lines[1], "Automation ID: ")
	if !ok || !validCodexAutomationID(id) || lines[2] != "Automation memory: $CODEX_HOME/automations/"+id+"/memory.md" {
		return false
	}
	last, ok := header(lines[3], "Last run: ")
	if !ok || lines[4] != "" || strings.TrimSpace(strings.Join(lines[5:], "\n")) == "" {
		return false
	}
	if last == "never" {
		return true
	}
	separator := strings.LastIndex(last, " (")
	if separator <= 0 || !strings.HasSuffix(last, ")") {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, last[:separator])
	if err != nil {
		return false
	}
	millis, err := strconv.ParseInt(last[separator+2:len(last)-1], 10, 64)
	return err == nil && at.UnixMilli() == millis
}

func validCodexAutomationHeartbeat(value string) bool {
	decoder := xml.NewDecoder(strings.NewReader(value))
	var rootSeen bool
	var childName string
	var text bytes.Buffer
	fields := make(map[string]string)
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			id := fields["automation_id"]
			at, hasTime := fields["current_time_iso"]
			prompt, hasPrompt := fields["instructions"]
			if hasTime != hasPrompt {
				return false
			}
			if hasTime {
				if _, err := time.Parse(time.RFC3339Nano, at); err != nil || strings.TrimSpace(prompt) == "" {
					return false
				}
			}
			return rootSeen && depth == 0 && validCodexAutomationID(id)
		}
		if err != nil {
			return false
		}
		switch token := token.(type) {
		case xml.StartElement:
			depth++
			if token.Name.Space != "" || len(token.Attr) != 0 || depth > 2 {
				return false
			}
			if depth == 1 {
				if rootSeen || token.Name.Local != "heartbeat" {
					return false
				}
				rootSeen = true
			} else {
				childName = token.Name.Local
				switch childName {
				case "automation_id", "current_time_iso", "instructions":
				default:
					return false
				}
				if _, duplicate := fields[childName]; duplicate {
					return false
				}
				text.Reset()
			}
		case xml.EndElement:
			if token.Name.Space != "" {
				return false
			}
			if depth == 2 {
				fields[childName] = text.String()
			}
			depth--
			if depth < 0 {
				return false
			}
		case xml.CharData:
			if depth == 2 {
				_, _ = text.Write(token)
			} else if len(bytes.TrimSpace(token)) != 0 {
				return false
			}
		default:
			return false
		}
	}
}
