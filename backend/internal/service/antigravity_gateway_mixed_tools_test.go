package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAntigravityMixedTools_RawPolicy(t *testing.T) {
	body := []byte(`{
		"tools":[{"functionDeclarations":[{"name":"weather"}],"googleSearch":{}},{"codeExecution":{}},{"otherTool":{"value":1}}],
		"toolConfig":{"includeServerSideToolInvocations":true,"include_server_side_tool_invocations":true,"functionCallingConfig":{"mode":"AUTO"}},
		"largeInteger":9007199254740993,"contents":[{"parts":[{"text":"a < b"}]}]
	}`)
	out, err := enableMixedGeminiToolInvocations(body)
	require.NoError(t, err)
	require.True(t, json.Valid(out))
	require.Len(t, gjson.GetBytes(out, "tools").Array(), 2)
	require.Equal(t, "weather", gjson.GetBytes(out, "tools.0.functionDeclarations.0.name").String())
	require.False(t, gjson.GetBytes(out, "tools.0.googleSearch").Exists())
	require.True(t, gjson.GetBytes(out, "tools.1.otherTool").Exists())
	require.Equal(t, "9007199254740993", gjson.GetBytes(out, "largeInteger").Raw)
	require.Equal(t, "a < b", gjson.GetBytes(out, "contents.0.parts.0.text").String())
	require.False(t, gjson.GetBytes(out, "toolConfig.includeServerSideToolInvocations").Exists())
	require.False(t, gjson.GetBytes(out, "toolConfig.include_server_side_tool_invocations").Exists())
	require.Equal(t, "AUTO", gjson.GetBytes(out, "toolConfig.functionCallingConfig.mode").String())
	second, err := enableMixedGeminiToolInvocations(out)
	require.NoError(t, err)
	require.Equal(t, out, second)
}

func TestAntigravityMixedTools_UnchangedAndEmptyConfig(t *testing.T) {
	for _, body := range []string{
		`{"tools":[{"googleSearch":{}},{"codeExecution":{}}],"toolConfig":{"includeServerSideToolInvocations":true}}`,
		`{"tools":[{"functionDeclarations":[]},{"googleSearch":{}}]}`,
		`{"tools":[{"functionDeclarations":[{"name":"weather"}]}]}`,
		`{"contents":[]}`, `{"tools":null}`,
	} {
		out, err := enableMixedGeminiToolInvocations([]byte(body))
		require.NoError(t, err)
		require.Equal(t, body, string(out))
	}
	out, err := enableMixedGeminiToolInvocations([]byte(`{"tools":[{"functionDeclarations":[{"name":"weather"}]},{"googleSearch":{}}],"toolConfig":{"includeServerSideToolInvocations":true}}`))
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(out, "toolConfig").Exists())
	_, err = enableMixedGeminiToolInvocations([]byte(`{`))
	require.Error(t, err)
}
