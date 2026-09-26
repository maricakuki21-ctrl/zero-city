package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

type nativeUsersStub struct{ user *User }

func (s nativeUsersStub) GetByID(context.Context, int64) (*User, error) { return s.user, nil }

type nativeKeysStub struct {
	keys []APIKey
}

func (s *nativeKeysStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	for _, key := range s.keys {
		if key.ID == id {
			return &key, nil
		}
	}
	return nil, ErrWorkbenchCatalogUnavailable
}
func (s *nativeKeysStub) ListByUserID(_ context.Context, _ int64, p pagination.PaginationParams, _ APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	start := min(p.Offset(), len(s.keys))
	end := min(start+p.Limit(), len(s.keys))
	return s.keys[start:end], &pagination.PaginationResult{Pages: (len(s.keys) + p.Limit() - 1) / p.Limit()}, nil
}

func nativeWorkbenchFixture(t *testing.T, handler http.HandlerFunc) (*NativeWorkbench, *nativeKeysStub) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	group := int64(8)
	keys := &nativeKeysStub{keys: []APIKey{
		{ID: 17, UserID: 41, GroupID: &group, Key: "secret-owned", Name: "chosen", Status: StatusActive},
	}}
	return &NativeWorkbench{
		users: nativeUsersStub{user: &User{ID: 41, Status: StatusActive}}, keys: keys,
		baseURL: server.URL, client: &http.Client{Timeout: 3 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		secret: []byte("test-signing-secret"), clock: systemWorkbenchClock{},
	}, keys
}

func nativeRequest(t *testing.T, n *NativeWorkbench) WorkbenchCanonicalRequest {
	t.Helper()
	auth := nativeWorkbenchAuthorization{UserID: 41, KeyID: 17, GroupID: 8, Model: "chosen-model", Expires: time.Now().Add(time.Minute).Unix()}
	cap := n.capability(auth, "chosen")
	record, err := n.ResolveWorkbenchCatalog(context.Background(), workbench.Identity{ActorID: 41}, cap.ID, cap.AcceptedQuoteID)
	require.NoError(t, err)
	return WorkbenchCanonicalRequest{RunID: "wbr_native_test", IdempotencyKey: "test", Intent: "hello",
		Resolved: ResolvedWorkbenchLaunch{Capability: cap, Quote: record.Quote, User: record.User, APIKey: record.APIKey, GroupID: 8}}
}

func TestNativeWorkbenchCatalogBindsOwnedKeyAndReturnsNoSecret(t *testing.T) {
	n, keys := nativeWorkbenchFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer secret-owned", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":[{"id":"model-a"},{"id":"model-a"},{"id":"model-b"},{"id":"gpt-image-1"},{"id":"grok-imagine-video"}]}`))
	})
	foreign := keys.keys[0]
	foreign.ID, foreign.UserID = 19, 99
	keys.keys = append(keys.keys, foreign)
	caps, err := n.ListWorkbenchCapabilities(context.Background(), workbench.Identity{ActorID: 41})
	require.NoError(t, err)
	require.Len(t, caps, 2)
	raw, err := json.Marshal(caps)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret-owned")
	require.Nil(t, caps[0].Estimate)
	record, err := n.ResolveWorkbenchCatalog(context.Background(), workbench.Identity{ActorID: 41}, caps[0].ID, caps[0].AcceptedQuoteID)
	require.NoError(t, err)
	require.Equal(t, int64(17), record.APIKey.ID)
	require.Equal(t, "model-a", record.Quote.ModelID)
}

func TestNativeWorkbenchRejectsChangedAuthorization(t *testing.T) {
	for _, scenario := range []string{"signature", "actor", "group", "owner", "expired", "disabled", "quota", "capability"} {
		t.Run(scenario, func(t *testing.T) {
			n, keys := nativeWorkbenchFixture(t, func(http.ResponseWriter, *http.Request) { t.Error("no network expected") })
			request := nativeRequest(t, n)
			cap := request.Resolved.Capability
			identity := workbench.Identity{ActorID: 41}
			switch scenario {
			case "signature":
				cap.AcceptedQuoteID += "1"
			case "actor":
				identity.ActorID = 99
			case "group":
				group := int64(9)
				keys.keys[0].GroupID = &group
			case "owner":
				keys.keys[0].UserID = 99
			case "expired":
				n.clock = fixedWorkbenchClock{now: time.Now().Add(time.Hour)}
			case "disabled":
				keys.keys[0].Status = StatusAPIKeyDisabled
			case "quota":
				keys.keys[0].Quota, keys.keys[0].QuotaUsed = 1, 1
			case "capability":
				cap.ID += "x"
			}
			_, err := n.ResolveWorkbenchCatalog(context.Background(), identity, cap.ID, cap.AcceptedQuoteID)
			require.Error(t, err)
		})
	}
}

func TestNativeWorkbenchExecutesMeteredGatewayWithoutSecondLedger(t *testing.T) {
	n, _ := nativeWorkbenchFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.Equal(t, "Bearer secret-owned", r.Header.Get("Authorization"))
		require.Equal(t, "wbr_native_test", r.Header.Get("Idempotency-Key"))
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "chosen-model", payload["model"])
		require.Equal(t, false, payload["stream"])
		w.Header().Set("request-id", "actual-provider-request")
		_, _ = w.Write([]byte(`{"id":"message-id","content":[{"type":"thinking","text":"private"},{"type":"text","text":"result"}]}`))
	})
	request := nativeRequest(t, n)
	request.Resolved.APIKey.Key = "untrusted-caller-key"
	result, err := n.Execute(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "result", string(result.Body))
	require.Equal(t, "actual-provider-request", result.CanonicalRequestID)
	require.Empty(t, result.JournalID)
	require.Empty(t, result.CanonicalUsageEventID)
	require.Nil(t, result.ActualCost)
}

func TestNativeWorkbenchNoRedirectOrProviderErrorLeak(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			n, _ := nativeWorkbenchFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/stolen-key")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("secret-upstream-token"))
			})
			_, err := n.Execute(context.Background(), nativeRequest(t, n))
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret-upstream-token")
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestNativeWorkbenchCancelStopsActiveRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	n, _ := nativeWorkbenchFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	request := nativeRequest(t, n)
	done := make(chan error, 1)
	go func() { _, err := n.Execute(context.Background(), request); done <- err }()
	<-started
	require.NoError(t, n.Cancel(context.Background(), request.RunID))
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop gateway request")
	}
}

func TestNativeWorkbenchRejectsEmptyOrOversizedOutput(t *testing.T) {
	for _, body := range []string{`{"content":[]}`, `not json`, strings.Repeat("a", (4<<20)+1)} {
		n, _ := nativeWorkbenchFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		_, err := n.Execute(context.Background(), nativeRequest(t, n))
		require.Error(t, err)
	}
}

func TestNativeWorkbenchCatalogPaginatesAndProbesConcurrently(t *testing.T) {
	var active, maximum atomic.Int32
	n, keys := nativeWorkbenchFixture(t, func(w http.ResponseWriter, r *http.Request) {
		count := active.Add(1)
		for old := maximum.Load(); count > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, count) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(time.Millisecond)
		_, _ = w.Write([]byte(`{"data":[{"id":"model"}]}`))
	})
	key := keys.keys[0]
	for i := 0; i < 101; i++ {
		key.ID = int64(100 + i)
		keys.keys = append(keys.keys, key)
	}
	caps, err := n.ListWorkbenchCapabilities(context.Background(), workbench.Identity{ActorID: 41})
	require.NoError(t, err)
	require.Len(t, caps, 102)
	require.LessOrEqual(t, maximum.Load(), int32(6))
	require.Greater(t, maximum.Load(), int32(1))
}
