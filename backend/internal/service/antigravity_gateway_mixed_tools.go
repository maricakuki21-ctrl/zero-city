package service

import (
	"bytes"
	"encoding/json"
)

// Client functions take precedence over built-ins on the Code Assist endpoint.
// RawMessage preserves unrelated values, including integers above 2^53.
func enableMixedGeminiToolInvocations(body []byte) ([]byte, error) {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	var tools []map[string]json.RawMessage
	if err := json.Unmarshal(request["tools"], &tools); err != nil {
		return body, nil
	}
	hasFunctions := false
	for _, tool := range tools {
		var declarations []json.RawMessage
		if json.Unmarshal(tool["functionDeclarations"], &declarations) == nil && len(declarations) > 0 {
			hasFunctions = true
			break
		}
	}
	if !hasFunctions {
		return body, nil
	}
	changed := false
	filtered := make([]map[string]json.RawMessage, 0, len(tools))
	for _, tool := range tools {
		for _, key := range []string{"googleSearch", "codeExecution"} {
			if _, ok := tool[key]; ok {
				delete(tool, key)
				changed = true
			}
		}
		if len(tool) > 0 {
			filtered = append(filtered, tool)
		}
	}
	if !changed {
		return body, nil
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	request["tools"] = encoded
	var config map[string]json.RawMessage
	if json.Unmarshal(request["toolConfig"], &config) == nil && config != nil {
		delete(config, "includeServerSideToolInvocations")
		delete(config, "include_server_side_tool_invocations")
		if len(config) == 0 {
			delete(request, "toolConfig")
		} else {
			encoded, err = json.Marshal(config)
			if err != nil {
				return nil, err
			}
			request["toolConfig"] = encoded
		}
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(request); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}
