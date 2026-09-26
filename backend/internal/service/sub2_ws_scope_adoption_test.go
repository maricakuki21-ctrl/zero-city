package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func sub2ScopeContext(headers map[string]string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	for key, value := range headers {
		c.Request.Header.Set(key, value)
	}
	return c
}

func TestSub2WSExecutionScopeIsolation(t *testing.T) {
	scopeFor := func(thread, kind string, key int64) string {
		c := sub2ScopeContext(map[string]string{
			"session_id": "shared-session", "thread-id": thread,
			openAIWSTurnMetadataHeader: `{"thread_id":"` + thread + `","request_kind":"` + kind + `"}`,
		})
		scope, _ := resolveOpenAIWSExecutionScope(c, nil, key)
		return scope
	}
	parent := scopeFor("parent", "turn", 21)
	require.NotEmpty(t, parent)
	require.NotEqual(t, parent, scopeFor("child", "turn", 21))
	require.NotEqual(t, parent, scopeFor("parent", "turn", 22))
	require.NotEqual(t, parent, scopeFor("parent", "memory", 21))
	require.NotEqual(t, parent, scopeFor("parent", "future-task", 21))
	require.Equal(t, parent, scopeFor("parent", "prewarm", 21))
	require.Equal(t, parent, scopeFor("parent", "compaction", 21))
	require.Equal(t, parent, scopeFor("parent", "", 21))
	scope, _ := resolveOpenAIWSExecutionScope(nil, []byte(`{"input":"hello"}`), 21)
	require.Empty(t, scope)

	c := sub2ScopeContext(map[string]string{"session_id": "shared-session"})
	a, _ := resolveOpenAIWSExecutionScope(c, []byte(`{"input":"a"}`), 21)
	b, _ := resolveOpenAIWSExecutionScope(c, []byte(`{"input":"b"}`), 21)
	require.NotEmpty(t, a)
	require.Equal(t, a, b)
	c.Request.Header.Set(openAISubagentHeader, "guardian")
	guardian, _ := resolveOpenAIWSExecutionScope(c, nil, 21)
	require.NotEqual(t, a, guardian)
}

func TestSub2WSThreadIdentityPrecedence(t *testing.T) {
	body := []byte(`{"client_metadata":{"thread_id":"body","x-codex-turn-metadata":"{\"thread_id\":\"embedded\"}"}}`)
	c := sub2ScopeContext(map[string]string{
		openAIWSThreadIDHeader:     "explicit",
		openAIWSTurnMetadataHeader: `{"thread_id":"metadata"}`,
		openAIWSWindowIDHeader:     "window:0",
	})
	require.Equal(t, "explicit", resolveOpenAIWSClientThreadID(c, body))
	c.Request.Header.Del(openAIWSThreadIDHeader)
	require.Equal(t, "metadata", resolveOpenAIWSClientThreadID(c, body))
	c.Request.Header.Set(openAIWSTurnMetadataHeader, "{invalid")
	require.Equal(t, "window", resolveOpenAIWSClientThreadID(c, body))
	c.Request.Header.Del(openAIWSWindowIDHeader)
	require.Equal(t, "body", resolveOpenAIWSClientThreadID(c, body))
	require.Equal(t, "embedded", resolveOpenAIWSClientThreadID(nil, []byte(`{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"embedded\"}"}}`)))
}

func TestSub2WSForwardStoresThreadScopedTurnState(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, http.Header{"X-Codex-Turn-State": []string{"child-turn-state"}})
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read: %v", err)
			return
		}
		if err := conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{"id": "scope-response", "model": "gpt-5.1",
				"usage": map[string]any{"input_tokens": 2, "output_tokens": 1}},
		}); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	svc := &OpenAIGatewayService{
		cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(),
	}
	account := &Account{
		ID: 456, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "test", "base_url": server.URL},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}
	c := sub2ScopeContext(map[string]string{"session_id": "shared", "thread-id": "child"})
	group := int64(9)
	c.Set("api_key", &APIKey{ID: 21, GroupID: &group})
	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	_, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	scope, _ := resolveOpenAIWSExecutionScope(c, body, 21)
	state, ok := svc.getOpenAIWSStateStore().GetSessionTurnState(group, scope)
	require.True(t, ok)
	require.Equal(t, "child-turn-state", state)
	_, exists := svc.getOpenAIWSStateStore().GetSessionTurnState(group, svc.GenerateSessionHash(c, body))
	require.False(t, exists)
}

func TestSub2WSRetrySkipsStaleIdleConnections(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	stale := &openAIWSCaptureConn{events: [][]byte{
		[]byte(`{"type":"response.completed","response":{"id":"first","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`),
	}}
	fresh := &openAIWSCaptureConn{events: [][]byte{
		[]byte(`{"type":"response.completed","response":{"id":"fresh","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`),
	}}
	dialer := &openAIWSQueueDialer{conns: []openAIWSClientConn{stale, fresh}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	svc := &OpenAIGatewayService{
		cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool,
	}
	account := &Account{
		ID: 443, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "test"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}
	errors := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			errors <- err
			return
		}
		defer conn.CloseNow()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_, message, err := conn.Read(ctx)
		if err == nil {
			err = svc.ProxyResponsesWebSocketFromClient(ctx, c, conn, account, "test", message, nil)
		}
		errors <- err
	}))
	defer server.Close()
	run := func(expected string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","stream":false}`)))
		_, message, err := conn.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, expected, gjson.GetBytes(message, "response.id").String())
		require.NoError(t, conn.Close(coderws.StatusNormalClosure, "done"))
		select {
		case err := <-errors:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	run("first")
	require.Equal(t, 1, dialer.DialCount())
	ap := pool.getOrCreateAccountPool(account.ID)
	secondStale := newOpenAIWSConn(pool.nextConnID(account.ID), account.ID, &openAIWSCaptureConn{}, nil)
	ap.mu.Lock()
	ap.conns[secondStale.id] = secondStale
	count := len(ap.conns)
	ap.mu.Unlock()
	require.Equal(t, 2, count)
	run("fresh")
	require.Equal(t, 2, dialer.DialCount())
	fresh.mu.Lock()
	writes := len(fresh.writes)
	fresh.mu.Unlock()
	require.Equal(t, 1, writes)
}
