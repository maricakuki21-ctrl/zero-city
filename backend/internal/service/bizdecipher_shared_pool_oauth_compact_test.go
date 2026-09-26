package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type sharedPoolOAuthCompactDoerFunc func(*http.Request) (*http.Response, error)

func (f sharedPoolOAuthCompactDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProbeSharedPoolOAuthCompactSuccessProducesDurableCapabilityItem(t *testing.T) {
	var captured *http.Request
	var body []byte
	doer := sharedPoolOAuthCompactDoerFunc(func(req *http.Request) (*http.Response, error) {
		captured = req
		body, _ = io.ReadAll(req.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
				"id":"resp_probe",
				"status":"completed",
				"output":[{"id":"cmp_probe","type":"compaction","status":"completed","encrypted_content":"probe-evidence"}]
			}`)),
		}, nil
	})

	item := probeSharedPoolOAuthCompactWithDoer(
		context.Background(),
		SharedPoolUpstreamProbeInput{PoolID: 9, AccountID: 27, ProbeModel: "gpt-5.6"},
		map[string]any{"access_token": "fixture-token", "chatgpt_account_id": "acct-27"},
		doer,
		"https://chatgpt.example/backend-api/codex/responses/compact",
	)

	require.True(t, item.Success)
	require.False(t, item.Required)
	require.Equal(t, sharedPoolOAuthCompactCheckID, item.ID)
	require.Equal(t, http.StatusOK, item.HTTPStatus)
	require.NotNil(t, captured)
	require.Equal(t, "chatgpt.com", captured.Host)
	require.Equal(t, "application/json", captured.Header.Get("Accept"))
	require.Equal(t, "Bearer fixture-token", captured.Header.Get("Authorization"))
	require.Equal(t, "acct-27", captured.Header.Get("chatgpt-account-id"))
	require.Equal(t, "gpt-5.6", gjson.GetBytes(body, "model").String())
	require.False(t, gjson.GetBytes(body, "max_output_tokens").Exists())
}

func TestProbeSharedPoolOAuthCompactRejectsSemanticallyEmptySuccess(t *testing.T) {
	doer := sharedPoolOAuthCompactDoerFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		}, nil
	})

	item := probeSharedPoolOAuthCompactWithDoer(
		context.Background(),
		SharedPoolUpstreamProbeInput{PoolID: 9, AccountID: 27, ProbeModel: "gpt-5.6"},
		map[string]any{"access_token": "fixture-token"},
		doer,
		"https://chatgpt.example/backend-api/codex/responses/compact",
	)

	require.False(t, item.Success)
	require.Equal(t, "invalid_response", item.ErrorType)
	require.Contains(t, item.ErrorMessage, "no completed compaction output")
}

func TestProbeSharedPoolOAuthCompactFailureDoesNotBecomeCapability(t *testing.T) {
	doer := sharedPoolOAuthCompactDoerFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"message":"compact not available"}}`)),
		}, nil
	})

	item := probeSharedPoolOAuthCompactWithDoer(
		context.Background(),
		SharedPoolUpstreamProbeInput{PoolID: 9, AccountID: 27, ProbeModel: "gpt-5.6"},
		map[string]any{"access_token": "fixture-token"},
		doer,
		"https://chatgpt.example/backend-api/codex/responses/compact",
	)

	require.False(t, item.Success)
	require.False(t, item.Required)
	require.Equal(t, http.StatusNotFound, item.HTTPStatus)
	require.NotEmpty(t, item.ErrorType)
	require.NotContains(t, item.ErrorMessage, "fixture-token")
}

func TestOptionalOAuthCompactCheckDoesNotLowerRequiredGateScore(t *testing.T) {
	checks := []SharedPoolFullCheckItem{
		{ID: "oauth_responses", Required: true, Success: true},
		{ID: sharedPoolOAuthCompactCheckID, Required: false, Success: false},
	}

	passed, total, score := summarizeSharedPoolFullCheck(checks)
	require.Equal(t, 1, passed)
	require.Equal(t, 1, total)
	require.Equal(t, 100.0, score)
}
