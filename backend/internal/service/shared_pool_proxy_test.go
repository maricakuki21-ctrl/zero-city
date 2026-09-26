package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolProbeAndRuntimeForwardUseIdenticalProxy(t *testing.T) {
	rawProxyURL := "socks5://proxy-user:proxy-pass@127.0.0.1:1080"
	expectedProxyURL, _, err := parseSharedPoolProxyURL(rawProxyURL)
	require.NoError(t, err)
	require.Equal(t, "socks5h://proxy-user:proxy-pass@127.0.0.1:1080", expectedProxyURL)

	probeClient := sharedPoolProbeHTTPClient(rawProxyURL)
	transport, ok := probeClient.Transport.(*http.Transport)
	require.True(t, ok)
	probeRequest, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/codex/responses/compact", nil)
	require.NoError(t, err)
	probeProxyURL, err := transport.Proxy(probeRequest)
	require.NoError(t, err)
	require.NotNil(t, probeProxyURL)

	account, err := sharedPoolOpenAIAccount(&SharedPoolAccessKey{
		PoolID:             9,
		AccountID:          27,
		AuthType:           AccountTypeOAuth,
		OAuthCredentials:   map[string]any{"access_token": "fixture-token"},
		UpstreamModelName:  "gpt-5.6",
		ProxyURL:           rawProxyURL,
		AccountConcurrency: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, account.ProxyID)
	require.NotNil(t, account.Proxy)
	require.Equal(t, account.Proxy.ID, *account.ProxyID)
	require.Equal(t, expectedProxyURL, probeProxyURL.String())
	require.Equal(t, expectedProxyURL, account.Proxy.URL())
}

func TestSharedPoolInvalidProxyFailsClosedBeforeCredentialHydration(t *testing.T) {
	probeClient := sharedPoolProbeHTTPClient("file:///tmp/not-a-proxy")
	transport, ok := probeClient.Transport.(*http.Transport)
	require.True(t, ok)
	probeRequest, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/codex/responses/compact", nil)
	require.NoError(t, err)
	_, err = transport.Proxy(probeRequest)
	require.ErrorIs(t, err, errInvalidSharedPoolProxy)

	service := &BizDecipherService{}
	accessKey, err := service.hydrateSharedPoolAccessKeyCredentials(&SharedPoolAccessKey{
		PoolID:          9,
		AuthType:        AccountTypeAPIKey,
		UpstreamBaseURL: "https://upstream.example/v1",
		UpstreamAPIKey:  "fixture-key",
		ProxyURL:        "file:///tmp/not-a-proxy",
	})
	require.Nil(t, accessKey)
	require.ErrorIs(t, err, errInvalidSharedPoolProxy)
}
