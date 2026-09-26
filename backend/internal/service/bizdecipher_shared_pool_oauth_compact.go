package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const sharedPoolOAuthCompactCheckID = "oauth_responses_compact"

type sharedPoolOAuthProbeDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// probeSharedPoolOAuthCompact records compact as an optional capability. A
// pool may continue serving ordinary Responses when compact is unavailable,
// but the compact gateway path remains fail-closed until this item succeeds.
func probeSharedPoolOAuthCompact(
	ctx context.Context,
	input SharedPoolUpstreamProbeInput,
	credentials map[string]any,
) SharedPoolFullCheckItem {
	return probeSharedPoolOAuthCompactWithDoer(
		ctx,
		input,
		credentials,
		sharedPoolProbeHTTPClient(strings.TrimSpace(input.ProxyURL)),
		chatgptCodexAPIURL+"/compact",
	)
}

func probeSharedPoolOAuthCompactWithDoer(
	ctx context.Context,
	input SharedPoolUpstreamProbeInput,
	credentials map[string]any,
	doer sharedPoolOAuthProbeDoer,
	endpoint string,
) (item SharedPoolFullCheckItem) {
	item = SharedPoolFullCheckItem{
		ID:       sharedPoolOAuthCompactCheckID,
		Title:    "Codex Responses Compact",
		Category: "compat",
		Required: false,
	}
	started := time.Now()
	defer func() { item.LatencyMs = int(time.Since(started).Milliseconds()) }()

	if doer == nil {
		item.ErrorType = "configuration_error"
		item.ErrorMessage = "compact probe transport is unavailable"
		return item
	}
	model := strings.TrimSpace(input.ProbeModel)
	accessToken := strings.TrimSpace(stringValue(credentials["access_token"]))
	accountID := strings.TrimSpace(firstNonEmpty(stringValue(credentials["chatgpt_account_id"]), stringValue(credentials["account_id"])))
	payload, err := json.Marshal(createOpenAICompactProbePayload(model))
	if err != nil {
		item.ErrorType = "invalid_request"
		item.ErrorMessage = "compact probe payload could not be encoded"
		return item
	}

	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, strings.TrimSpace(endpoint), bytes.NewReader(payload))
	if err != nil {
		item.ErrorType = "invalid_request"
		item.ErrorMessage = "compact probe request could not be created"
		return item
	}
	req = applySharedPoolOAuthProbeRequestShape(req, input, accessToken, accountID)
	req.Header.Set("Accept", "application/json")

	resp, err := doer.Do(req)
	if err != nil {
		probeErr := classifySharedPoolProbeError(err, 0)
		item.ErrorType = probeErr.ErrorType
		item.ErrorMessage = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Error()), accessToken)
		return item
	}
	defer func() { _ = resp.Body.Close() }()
	item.HTTPStatus = resp.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		probeErr := classifySharedPoolProbeError(readErr, resp.StatusCode)
		item.ErrorType = probeErr.ErrorType
		item.ErrorMessage = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Error()), accessToken)
		return item
	}
	if resp.StatusCode != http.StatusOK {
		probeErr := classifySharedPoolProbeError(
			fmt.Errorf("oauth compact probe failed with %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
			resp.StatusCode,
		)
		item.ErrorType = probeErr.ErrorType
		item.ErrorMessage = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Error()), accessToken)
		return item
	}
	if !validSharedPoolOAuthCompactResponse(body) {
		item.ErrorType = "invalid_response"
		item.ErrorMessage = "compact probe returned no completed compaction output"
		return item
	}
	item.Success = true
	item.Evidence = truncateSharedPoolEvidence(string(body))
	return item
}

func validSharedPoolOAuthCompactResponse(body []byte) bool {
	var response struct {
		Status string `json:"status"`
		Output []struct {
			Type             string            `json:"type"`
			Status           string            `json:"status"`
			EncryptedContent string            `json:"encrypted_content"`
			Summary          []json.RawMessage `json:"summary"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return false
	}
	if status := strings.ToLower(strings.TrimSpace(response.Status)); status != "" && status != "completed" {
		return false
	}
	for _, output := range response.Output {
		outputType := strings.ToLower(strings.TrimSpace(output.Type))
		if outputType != "compaction" && outputType != "compaction_summary" {
			continue
		}
		if status := strings.ToLower(strings.TrimSpace(output.Status)); status != "" && status != "completed" {
			continue
		}
		if strings.TrimSpace(output.EncryptedContent) != "" || len(output.Summary) > 0 {
			return true
		}
	}
	return false
}
