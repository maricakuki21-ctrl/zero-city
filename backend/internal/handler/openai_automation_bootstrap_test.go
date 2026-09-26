package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func automationTestBody(t *testing.T, output string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"model": "test", "input": []any{map[string]any{
		"type": "function_call_output", "namespace": "codex_app", "name": "automation_update", "output": output,
	}}})
	require.NoError(t, err)
	return b
}

func TestAutomationBootstrapNormalizesOnlyKnownEnvelope(t *testing.T) {
	for _, output := range []string{
		"<heartbeat><automation_id>wiki</automation_id></heartbeat>",
		"<heartbeat><automation_id>wiki</automation_id><current_time_iso>2026-09-19T12:00:00Z</current_time_iso><instructions>Review project</instructions></heartbeat>",
		"Automation: Review\nAutomation ID: wiki\nAutomation memory: $CODEX_HOME/automations/wiki/memory.md\nLast run: never\n\nReview project",
	} {
		body := automationTestBody(t, output)
		out, changed := normalizeCodexAutomationBootstrap(body)
		require.True(t, changed)
		require.Equal(t, "user", gjson.GetBytes(out, "input.0.role").String())
		require.Equal(t, output, gjson.GetBytes(out, "input.0.content.0.text").String())
		again, changed := normalizeCodexAutomationBootstrap(out)
		require.False(t, changed)
		require.Equal(t, out, again)
	}
}

func TestAutomationBootstrapRejectsUnsafeEnvelopes(t *testing.T) {
	for _, output := range []string{
		"ordinary tool result",
		"<heartbeat a=\"b\"><automation_id>wiki</automation_id></heartbeat>",
		"<heartbeat><automation_id>../wiki</automation_id></heartbeat>",
		"<heartbeat><automation_id>wiki</automation_id><automation_id>other</automation_id></heartbeat>",
		"<heartbeat><automation_id>wiki</automation_id><current_time_iso>2026-09-19T12:00:00Z</current_time_iso></heartbeat>",
		"<heartbeat><automation_id>wiki</automation_id><current_time_iso>invalid</current_time_iso><instructions>Review</instructions></heartbeat>",
		"<heartbeat><!-- comment --><automation_id>wiki</automation_id></heartbeat>",
		"<heartbeat><automation_id>wiki</automation_id><instructions><nested/></instructions></heartbeat>",
	} {
		body := automationTestBody(t, output)
		out, changed := normalizeCodexAutomationBootstrap(body)
		require.False(t, changed, output)
		require.Equal(t, body, out)
	}
}

func TestAutomationBootstrapRetainsRealToolPairing(t *testing.T) {
	body := automationTestBody(t, "<heartbeat><automation_id>wiki</automation_id></heartbeat>")
	var request map[string]any
	require.NoError(t, json.Unmarshal(body, &request))
	item := request["input"].([]any)[0].(map[string]any)
	for _, id := range []any{"call_actual", nil, 1} {
		item["call_id"] = id
		encoded, err := json.Marshal(request)
		require.NoError(t, err)
		out, changed := normalizeCodexAutomationBootstrap(encoded)
		require.False(t, changed)
		require.Equal(t, encoded, out)
	}
	delete(item, "call_id")
	for _, previous := range []any{"resp_actual", nil, 2} {
		request["previous_response_id"] = previous
		encoded, _ := json.Marshal(request)
		_, changed := normalizeCodexAutomationBootstrap(encoded)
		require.False(t, changed)
	}
	delete(request, "previous_response_id")
	for _, typ := range []string{"function_call", "function_call_output", "item_reference", "tool_search_output", "computer_call_output"} {
		request["input"] = []any{item, map[string]any{"type": typ, "call_id": "real"}}
		encoded, _ := json.Marshal(request)
		_, changed := normalizeCodexAutomationBootstrap(encoded)
		require.False(t, changed, typ)
	}
	require.False(t, automationUniqueJSON([]byte(`{"input":[],"input":[]}`)))
	require.False(t, automationUniqueJSON([]byte(`{"input":[]} {}`)))
}
