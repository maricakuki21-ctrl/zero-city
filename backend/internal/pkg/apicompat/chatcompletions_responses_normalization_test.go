package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesNormalizationPreservesReasoningAndToolPairing(t *testing.T) {
	messages, err := responsesInputToChatMessages("main instructions", json.RawMessage(`[
		{"type":"message","role":"developer","content":"project instructions"},
		{"type":"message","role":"user","content":"inspect"},
		{"type":"reasoning","summary":[{"type":"summary_text","text":"inspect both results"}]},
		{"type":"function_call","call_id":"a","name":"image","arguments":"{}"},
		{"type":"function_call","call_id":"b","name":"text","arguments":"{}"},
		{"type":"message","role":"developer","content":"approval saved"},
		{"type":"function_call_output","call_id":"b","output":"done"},
		{"type":"function_call_output","call_id":"a","output":[{"type":"input_image","image_url":"data:image/png;base64,AQID"}]}
	]`))
	require.NoError(t, err)
	assertChatInvariants(t, messages)
	require.Equal(t, []string{"system", "user", "assistant", "tool", "tool", "user", "user"}, chatMessageRoles(messages))
	require.JSONEq(t, `"main instructions\n\nproject instructions"`, string(messages[0].Content))
	require.Equal(t, "inspect both results", messages[2].ReasoningContent)
	require.Len(t, messages[2].ToolCalls, 2)
	require.Equal(t, "a", messages[3].ToolCallID)
	require.Equal(t, "b", messages[4].ToolCallID)
	require.Equal(t, "done", chatToolContentString(t, messages[4]))
	require.Equal(t, "[Tool output media for call a]", chatContentParts(t, messages[5])[0].Text)
	require.JSONEq(t, `"approval saved"`, string(messages[6].Content))
}

func TestResponsesNormalizationDoesNotChangeDirectChatRoles(t *testing.T) {
	messages := []ChatMessage{
		{Role: "user", Content: json.RawMessage(`"hello"`)},
		{Role: "system", Content: json.RawMessage(`"unchanged direct chat instruction"`)},
		{Role: "assistant", Content: json.RawMessage(`"answer"`)},
	}
	require.Equal(t, messages, normalizeChatMessages(messages))
}

func TestResponsesNormalizationPreservesUnattributedToolOutput(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[
		{"type":"function_call_output","output":[{"type":"input_image","image_url":"data:image/png;base64,AQID"}]}
	]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Empty(t, messages[0].ToolCallID)
	require.Equal(t, `[{"type":"input_image","image_url":"data:image/png;base64,AQID"}]`, chatToolContentString(t, messages[0]))
}

func TestResponsesNormalizationPreservesSingleLeadingContentBytes(t *testing.T) {
	raw := json.RawMessage(`"  keep whitespace\n "`)
	messages := []ChatMessage{{Role: "system", Content: raw}, {Role: "user", Content: json.RawMessage(`"hello"`)}}
	require.Equal(t, messages, normalizeResponsesDerivedChatMessageRoles(messages))
}
