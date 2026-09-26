package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMixedToolsRegistry_BuiltinPolicy(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tools     []ClaudeTool
		search    bool
		code      bool
		functions int
	}{
		{"builtins only", []ClaudeTool{{Type: "web_search_20250305"}, {Type: "code_execution_20250825"}}, true, true, 0},
		{"invalid custom keeps builtin", []ClaudeTool{{Type: "web_search"}, {Type: "custom", Name: "broken"}}, true, false, 0},
		{"blank function keeps builtin", []ClaudeTool{{Type: "web_search"}, {Name: " "}}, true, false, 0},
		{"function drops builtins", []ClaudeTool{{Type: "web_search"}, {Type: "code_execution"}, {Name: "weather"}}, false, false, 1},
		{"custom drops builtins", []ClaudeTool{{Type: "code_execution"}, {Type: "custom", Name: "weather", Custom: &CustomToolSpec{InputSchema: map[string]any{}}}}, false, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := &ClaudeRequest{Model: "gemini-3.8-flash-high", Tools: tc.tools,
				Messages: []ClaudeMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}}
			body, err := TransformClaudeToGemini(request, "project", request.Model)
			require.NoError(t, err)
			var transformed V1InternalRequest
			require.NoError(t, json.Unmarshal(body, &transformed))
			search, code, functions := false, false, 0
			for _, tool := range transformed.Request.Tools {
				search = search || tool.GoogleSearch != nil
				code = code || tool.CodeExecution != nil
				functions += len(tool.FunctionDeclarations)
			}
			require.Equal(t, tc.search, search)
			require.Equal(t, tc.code, code)
			require.Equal(t, tc.functions, functions)
			if tc.search {
				require.Equal(t, "web_search", transformed.RequestType)
				require.Equal(t, webSearchFallbackModel, transformed.Model)
			} else {
				require.Equal(t, "agent", transformed.RequestType)
				require.Equal(t, request.Model, transformed.Model)
			}
		})
	}
}

func TestMixedToolsRegistry_ModelsAndLocalAliases(t *testing.T) {
	models := map[string]bool{}
	for _, model := range DefaultGeminiModels() {
		models[model.Name] = true
	}
	for _, version := range []string{"3.6", "3.7", "3.8"} {
		for _, suffix := range []string{"", "-high", "-low", "-medium", "-tiered"} {
			id := "gemini-" + version + "-flash" + suffix
			require.True(t, models["models/"+id], id)
			require.Equal(t, suffix != "", IsGeminiReasoningModel(id), id)
		}
	}
	for _, alias := range []string{"gemini-2.5-flash-image-preview", "gemini-3.1-flash-image-preview"} {
		require.True(t, models["models/"+alias], alias)
	}
	request := &ClaudeRequest{Messages: []ClaudeMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}}
	body, err := TransformClaudeToGemini(request, "project", "gemini-3.8-flash-high")
	require.NoError(t, err)
	var transformed V1InternalRequest
	require.NoError(t, json.Unmarshal(body, &transformed))
	require.Nil(t, transformed.Request.ToolConfig)
}
