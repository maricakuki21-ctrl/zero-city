package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSHandshakeCompatibility_IdentityIsolation(t *testing.T) {
	for _, name := range []string{
		"x-codex-installation-id", "x-codex-window-id", "session-id",
		"session_id", "conversation_id", "thread-id",
	} {
		t.Run(name, func(t *testing.T) {
			original := http.Header{name: {"client-a"}}
			canonical := make(http.Header)
			canonical.Set(name, "client-a")
			require.Equal(t, normalizeOpenAIWSHandshakeCompatibility(original), normalizeOpenAIWSHandshakeCompatibility(canonical))
			require.NotEqual(t, normalizeOpenAIWSHandshakeCompatibility(original), normalizeOpenAIWSHandshakeCompatibility(http.Header{name: {"client-b"}}))
			require.NotEqual(t, normalizeOpenAIWSHandshakeCompatibility(original), normalizeOpenAIWSHandshakeCompatibility(nil))
		})
	}
	require.NotEqual(t,
		normalizeOpenAIWSIdentityHeader(http.Header{"Session_id": {"a", "b"}}, "session_id"),
		normalizeOpenAIWSIdentityHeader(http.Header{"Session_id": {"a,b"}}, "session_id"))
	require.NotEqual(t,
		normalizeOpenAIWSIdentityHeader(http.Header{"Session_id": {""}}, "session_id"),
		normalizeOpenAIWSIdentityHeader(nil, "session_id"))
}

func TestOpenAIWSHandshakeCompatibility_IgnoresPerTurnMetadata(t *testing.T) {
	first := http.Header{"Session_id": {"stable"}, "X-Codex-Beta-Features": {"b,a"}, "X-Client-Request-Id": {"request-a"}}
	second := http.Header{"Session_id": {"stable"}, "X-Codex-Beta-Features": {"a", "b"}, "X-Client-Request-Id": {"request-b"}}
	require.Equal(t, normalizeOpenAIWSHandshakeCompatibility(first), normalizeOpenAIWSHandshakeCompatibility(second))
}

func TestOpenAIWSHandshakeCompatibility_AcquireAndPreferred(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	req := openAIWSAcquireRequest{
		Account: &Account{ID: 923, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		WSURL:   "wss://example.com/v1/responses",
		Headers: http.Header{"Session_id": {"user-a"}},
	}
	first, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	firstID := first.ConnID()
	first.Release()

	req.Headers = http.Header{"Session_id": {"user-b"}}
	req.PreferredConnID = firstID
	req.ForcePreferredConn = true
	_, err = pool.Acquire(context.Background(), req)
	require.ErrorIs(t, err, errOpenAIWSPreferredConnUnavailable)

	req.ForcePreferredConn = false
	second, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	require.NotEqual(t, firstID, second.ConnID())
	secondID := second.ConnID()
	second.Release()
	req.PreferredConnID = ""
	reused, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	require.True(t, reused.Reused())
	require.Equal(t, secondID, reused.ConnID())
	reused.Release()
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSHandshakeCompatibility_ReplacesIdleAtCapacity(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSCountingDialer{})
	req := openAIWSAcquireRequest{
		Account: &Account{ID: 924, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		WSURL:   "wss://example.com/v1/responses",
		Headers: http.Header{"X-Codex-Installation-Id": {"device-a"}},
	}
	first, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	firstID := first.ConnID()
	first.Release()
	req.Headers = http.Header{"X-Codex-Installation-Id": {"device-b"}}
	next, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	require.NotEqual(t, firstID, next.ConnID())
	require.False(t, next.Reused())
	next.Release()
}

func TestOpenAIWSHandshakeCompatibility_FactoryCannotChangeIdentity(t *testing.T) {
	pool := newOpenAIWSConnPool(&config.Config{})
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	req := openAIWSAcquireRequest{
		Account: &Account{ID: 925, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		WSURL:   "wss://example.com/v1/responses",
		Headers: http.Header{"Session_id": {"original"}},
		HeadersFactory: func(_ context.Context, headers http.Header) (http.Header, error) {
			headers.Set("session_id", "unexpected")
			return headers, nil
		},
	}
	_, err := pool.dialConn(context.Background(), req)
	require.ErrorContains(t, err, "changed handshake identity")
	require.Zero(t, dialer.DialCount())
	require.Equal(t, "original", req.Headers.Get("session_id"))
	req.HeadersFactory = func(_ context.Context, headers http.Header) (http.Header, error) {
		headers.Set("Authorization", "Bearer refreshed")
		return headers, nil
	}
	conn, err := pool.dialConn(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, normalizeOpenAIWSHandshakeCompatibility(req.Headers), conn.handshakeCompatibility)
	conn.close()
}
