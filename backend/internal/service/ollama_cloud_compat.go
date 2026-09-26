package service

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const OllamaCloudMaxTokensCapExtraKey = "ollama_max_tokens_cap"
const ollamaCloudDefaultMaxTokensCap int64 = 65535

func isOllamaCloudTargetURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && strings.EqualFold(u.Scheme, "https") &&
		strings.EqualFold(u.Hostname(), "ollama.com") && u.User == nil &&
		(u.Port() == "" || u.Port() == "443")
}

func ollamaCloudMaxTokensCap(account *Account) int64 {
	if account == nil {
		return ollamaCloudDefaultMaxTokensCap
	}
	switch n := account.Extra[OllamaCloudMaxTokensCapExtraKey].(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		if parsed, err := n.Int64(); err == nil {
			return parsed
		}
	}
	return ollamaCloudDefaultMaxTokensCap
}

// Called after model mapping and only for the actual official destination.
func clampOllamaCloudMaxTokensForURL(account *Account, target string, body []byte) []byte {
	if account == nil || account.Type != AccountTypeAPIKey || !isOllamaCloudTargetURL(target) || !gjson.ValidBytes(body) {
		return body
	}
	cap := ollamaCloudMaxTokensCap(account)
	if cap <= 0 {
		return body
	}
	u, _ := url.Parse(target)
	if !strings.Contains(u.Path, "/chat/completions") &&
		!strings.Contains(strings.ToLower(gjson.GetBytes(body, "model").String()), "deepseek") {
		return body
	}
	out := body
	for _, key := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
		value := gjson.GetBytes(out, key)
		if value.Type != gjson.Number || value.Int() <= cap {
			continue
		}
		next, err := sjson.SetBytes(out, key, cap)
		if err != nil {
			return body
		}
		out = next
	}
	return out
}

func isOllamaCloudRawAccount(account *Account) bool {
	return account != nil && account.Type == AccountTypeAPIKey &&
		account.Platform == PlatformOpenAI && isOllamaCloudBaseURL(account.GetOpenAIBaseURL())
}

func normalizeOllamaCloudChatRequest(account *Account, body []byte) []byte {
	if !isOllamaCloudRawAccount(account) || !gjson.ValidBytes(body) {
		return body
	}
	out := body
	for i, message := range gjson.GetBytes(body, "messages").Array() {
		value := message.Get("reasoning_content")
		if message.Get("role").String() != "assistant" || value.Type != gjson.String || value.Str == "" ||
			message.Get("reasoning").String() != "" || message.Get("thinking").String() != "" {
			continue
		}
		next, err := sjson.SetBytes(out, "messages."+strconv.Itoa(i)+".reasoning", value.Str)
		if err != nil {
			return body
		}
		out = next
	}
	return out
}

func normalizeOllamaCloudChatResponse(body []byte) []byte {
	if !gjson.ValidBytes(body) {
		return body
	}
	out := body
	for i, choice := range gjson.GetBytes(body, "choices").Array() {
		for _, container := range []string{"message", "delta"} {
			obj := choice.Get(container)
			if !obj.IsObject() || obj.Get("reasoning_content").Exists() {
				continue
			}
			value := obj.Get("reasoning")
			if value.Type != gjson.String || value.Str == "" {
				value = obj.Get("thinking")
			}
			if value.Type != gjson.String || value.Str == "" {
				continue
			}
			next, err := sjson.SetBytes(out, "choices."+strconv.Itoa(i)+"."+container+".reasoning_content", value.Str)
			if err != nil {
				return body
			}
			out = next
		}
	}
	return out
}

func normalizeOllamaCloudSSELine(account *Account, line string) string {
	if !isOllamaCloudRawAccount(account) {
		return line
	}
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok || strings.TrimSpace(payload) == "[DONE]" {
		return line
	}
	out := normalizeOllamaCloudChatResponse([]byte(payload))
	return line[:len(line)-len(payload)] + string(out)
}
