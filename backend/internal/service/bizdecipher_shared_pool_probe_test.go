package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSharedPoolFullProbeExecutionContextHasBoundedDeadline(t *testing.T) {
	ctx, cancel := sharedPoolProbeExecutionContext(context.Background(), "full")
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("full probe context must have a deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= sharedPoolFullProbeTimeout-time.Second || remaining > sharedPoolFullProbeTimeout {
		t.Fatalf("unexpected full probe deadline: %s", remaining)
	}
}

func TestSharedPoolProbeWriteContextSurvivesClientCancellation(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	writeCtx, cancelWrite := sharedPoolProbeWriteContext(requestCtx)
	defer cancelWrite()
	if err := writeCtx.Err(); err != nil {
		t.Fatalf("write context inherited client cancellation: %v", err)
	}
	if _, ok := writeCtx.Deadline(); !ok {
		t.Fatal("write context must have a bounded deadline")
	}
}

func TestSharedPoolProbeCancellationOnlyPersistsCompletedSuccess(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	if !shouldSkipSharedPoolProbeOutcomeAfterCancellation(requestCtx, nil, nil) {
		t.Fatal("canceled probe without a result must not be persisted")
	}
	if !shouldSkipSharedPoolProbeOutcomeAfterCancellation(requestCtx, &SharedPoolUpstreamProbeResult{OK: false}, nil) {
		t.Fatal("canceled incomplete probe must not be persisted as a failure")
	}
	if !shouldSkipSharedPoolProbeOutcomeAfterCancellation(requestCtx, &SharedPoolUpstreamProbeResult{OK: true}, context.Canceled) {
		t.Fatal("canceled probe with an error must not be persisted")
	}
	if shouldSkipSharedPoolProbeOutcomeAfterCancellation(requestCtx, &SharedPoolUpstreamProbeResult{OK: true}, nil) {
		t.Fatal("completed successful probe should still be persisted")
	}
}

type sharedPoolProbeSecretEncryptor struct{}

type sharedPoolFailingSecretEncryptor struct {
	errorText string
}

func (sharedPoolProbeSecretEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (sharedPoolProbeSecretEncryptor) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}

func (e sharedPoolFailingSecretEncryptor) Encrypt(string) (string, error) {
	return "", nil
}

func (e sharedPoolFailingSecretEncryptor) Decrypt(string) (string, error) {
	return "", errors.New(e.errorText)
}

type sharedPoolModelsRuntimeRepoStub struct {
	BizDecipherRepository
	poolRuntime    *SharedPoolUpstreamRuntime
	accountRuntime *SharedPoolUpstreamRuntime
	accessKey      *SharedPoolAccessKey
	poolID         int64
	accountID      int64
	ownerID        int64
}

func (r *sharedPoolModelsRuntimeRepoStub) GetSharedPoolUpstreamRuntime(_ context.Context, poolID, ownerID int64) (*SharedPoolUpstreamRuntime, error) {
	r.poolID = poolID
	r.ownerID = ownerID
	return r.poolRuntime, nil
}

func (r *sharedPoolModelsRuntimeRepoStub) GetSharedPoolAccountUpstreamRuntime(_ context.Context, poolID, accountID, ownerID int64) (*SharedPoolUpstreamRuntime, error) {
	r.poolID = poolID
	r.accountID = accountID
	r.ownerID = ownerID
	return r.accountRuntime, nil
}

func (r *sharedPoolModelsRuntimeRepoStub) GetSharedPoolAccessKeyByAPIKeyID(context.Context, int64, string) (*SharedPoolAccessKey, error) {
	return r.accessKey, nil
}

func TestProbeSharedPoolUpstreamFullCheckPassesWithOpenAICompatibleServer(t *testing.T) {
	server := newSharedPoolProbeTestServer(t, "")
	defer server.Close()

	result, err := probeSharedPoolUpstream(context.Background(), SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "publish_gate",
	})
	if err != nil {
		t.Fatalf("probeSharedPoolUpstream returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.OK || !result.GatePassed {
		t.Fatalf("expected gate passed, got ok=%v gate=%v message=%q", result.OK, result.GatePassed, result.Message)
	}
	if result.CheckLevel != "full" {
		t.Fatalf("expected full check level, got %q", result.CheckLevel)
	}
	if result.FullCheckPassed != 15 || result.FullCheckTotal != 15 {
		t.Fatalf("expected 15/15 checks, got %d/%d", result.FullCheckPassed, result.FullCheckTotal)
	}
	if len(result.Checks) != 15 {
		t.Fatalf("expected 15 check items, got %d", len(result.Checks))
	}
	if len(result.Models) != 1 || result.Models[0] != "gpt-test" {
		t.Fatalf("expected fetched model list, got %#v", result.Models)
	}
}

func TestProbeSharedPoolUpstreamFullCheckFetchesModelsOnce(t *testing.T) {
	var mu sync.Mutex
	modelsCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			mu.Lock()
			modelsCalls++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
			return
		}
		serveSharedPoolProbeChatFixture(w, r, "")
	}))
	defer server.Close()

	result, err := probeSharedPoolUpstream(context.Background(), SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "publish_gate",
	})
	if err != nil || result == nil || !result.OK {
		t.Fatalf("full probe failed: result=%#v err=%v", result, err)
	}
	mu.Lock()
	gotModelsCalls := modelsCalls
	mu.Unlock()
	if gotModelsCalls != 1 {
		t.Fatalf("models endpoint calls = %d, want exactly 1", gotModelsCalls)
	}
}

func TestProbeSharedPoolUpstreamCoreFailureSkipsExpensiveChecks(t *testing.T) {
	var mu sync.Mutex
	modelsCalls := 0
	chatCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			mu.Lock()
			modelsCalls++
			mu.Unlock()
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			mu.Lock()
			chatCalls++
			mu.Unlock()
			http.Error(w, "core unavailable", http.StatusBadGateway)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	result, err := probeSharedPoolUpstream(context.Background(), SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "manual",
	})
	if err == nil || result == nil || result.OK {
		t.Fatalf("expected failed core probe, result=%#v err=%v", result, err)
	}
	mu.Lock()
	gotModelsCalls, gotChatCalls := modelsCalls, chatCalls
	mu.Unlock()
	if gotModelsCalls != 1 || gotChatCalls != 1 {
		t.Fatalf("request counts models=%d chat=%d, want 1 and 1", gotModelsCalls, gotChatCalls)
	}
	if len(result.Checks) != 15 {
		t.Fatalf("check count = %d, want 15 including explicit skipped items", len(result.Checks))
	}
	for _, item := range result.Checks[2:] {
		if item.ErrorType != "skipped_core_failure" {
			t.Fatalf("check %q error type = %q, want skipped_core_failure", item.ID, item.ErrorType)
		}
	}
}

func TestProbeSharedPoolUpstreamFullChecksUseBoundedConcurrencyAndStableOrder(t *testing.T) {
	var mu sync.Mutex
	concurrent := 0
	maxConcurrent := 0
	chatCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		content := sharedPoolProbeTestContent(payload.Messages)
		isCore := strings.EqualFold(strings.TrimSpace(content), "pong")
		mu.Lock()
		chatCalls++
		if !isCore {
			concurrent++
			if concurrent > maxConcurrent {
				maxConcurrent = concurrent
			}
		}
		mu.Unlock()
		if !isCore {
			time.Sleep(35 * time.Millisecond)
			mu.Lock()
			concurrent--
			mu.Unlock()
		}
		writeSharedPoolProbeFixtureResponse(w, payload.Stream, content)
	}))
	defer server.Close()

	result, err := probeSharedPoolUpstream(context.Background(), SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "scheduled_full",
	})
	if err != nil || result == nil || !result.OK {
		t.Fatalf("full probe failed: result=%#v err=%v", result, err)
	}
	mu.Lock()
	gotMaxConcurrent, gotChatCalls := maxConcurrent, chatCalls
	mu.Unlock()
	if gotMaxConcurrent < 2 || gotMaxConcurrent > sharedPoolFullProbeConcurrency {
		t.Fatalf("max concurrent checks = %d, want 2..%d", gotMaxConcurrent, sharedPoolFullProbeConcurrency)
	}
	if gotChatCalls != 1+len(sharedPoolFullCheckSpecs("gpt-test")) {
		t.Fatalf("chat calls = %d, want %d", gotChatCalls, 1+len(sharedPoolFullCheckSpecs("gpt-test")))
	}
	wantIDs := []string{"models", "chat_basic"}
	for _, spec := range sharedPoolFullCheckSpecs("gpt-test") {
		wantIDs = append(wantIDs, spec.item.ID)
	}
	for index, wantID := range wantIDs {
		if result.Checks[index].ID != wantID {
			t.Fatalf("check[%d] id = %q, want %q", index, result.Checks[index].ID, wantID)
		}
	}
}

func TestProbeSharedPoolUpstreamDeadlineProducesExplicitTimeoutItems(t *testing.T) {
	releaseHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
			return
		}
		select {
		case <-r.Context().Done():
		case <-releaseHandler:
		}
	}))
	defer func() {
		close(releaseHandler)
		server.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result, err := probeSharedPoolUpstream(ctx, SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "manual",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("probe error = %v, want context deadline exceeded", err)
	}
	if result == nil || result.ErrorType != "timeout" || result.GatePassed {
		t.Fatalf("unexpected timeout result: %#v", result)
	}
	if len(result.Checks) != 15 {
		t.Fatalf("timeout check count = %d, want 15", len(result.Checks))
	}
	for _, item := range result.Checks[1:] {
		if item.ErrorType != "timeout" {
			t.Fatalf("check %q error type = %q, want timeout", item.ID, item.ErrorType)
		}
	}
}

func TestExtractSharedPoolProbeStreamContentJoinsSplitDeltaMarkers(t *testing.T) {
	detail := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","content":""}}]}`,
		`data: {"choices":[{"delta":{"content":"CHECK_"}}]}`,
		`data: {"choices":[{"delta":{"content":"STREAM"}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}, "\n\n")

	content, err := extractSharedPoolProbeStreamContent(detail)
	if err != nil {
		t.Fatalf("extractSharedPoolProbeStreamContent returned error: %v", err)
	}
	if content != "CHECK_STREAM" {
		t.Fatalf("stream content = %q, want CHECK_STREAM", content)
	}
}

func TestProbeSharedPoolUpstreamScheduledUsesBasicCheck(t *testing.T) {
	server := newSharedPoolProbeTestServer(t, "")
	defer server.Close()

	result, err := probeSharedPoolUpstream(context.Background(), SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "scheduled",
	})
	if err != nil {
		t.Fatalf("probeSharedPoolUpstream returned error: %v", err)
	}
	if result.CheckLevel != "basic" {
		t.Fatalf("expected basic check level, got %q", result.CheckLevel)
	}
	if result.FullCheckPassed != 1 || result.FullCheckTotal != 1 || len(result.Checks) != 1 {
		t.Fatalf("expected only one basic check, got passed=%d total=%d len=%d", result.FullCheckPassed, result.FullCheckTotal, len(result.Checks))
	}
}

func TestProbeSharedPoolUpstreamFullCheckAllowsOneModelSpecificMismatch(t *testing.T) {
	server := newSharedPoolProbeTestServer(t, "check_system")
	defer server.Close()

	result, err := probeSharedPoolUpstream(context.Background(), SharedPoolUpstreamProbeInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-test",
		ProbeModel:      "gpt-test",
		ProbeType:       "manual",
	})
	if err != nil {
		t.Fatalf("expected tolerant full check to pass, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.OK || !result.GatePassed {
		t.Fatalf("expected tolerant gate pass, got ok=%v gate=%v", result.OK, result.GatePassed)
	}
	if result.FullCheckPassed != 14 || result.FullCheckTotal != 15 {
		t.Fatalf("expected 14/15 checks, got %d/%d", result.FullCheckPassed, result.FullCheckTotal)
	}
}

func TestSharedPoolFullCheckGateRequiresCoreChatAndSeventyPercent(t *testing.T) {
	checks := make([]SharedPoolFullCheckItem, 10)
	for i := range checks {
		checks[i] = SharedPoolFullCheckItem{ID: fmt.Sprintf("check_%d", i), Required: true, Success: i < 7}
	}
	checks[0].ID = "chat_basic"
	checks[0].Success = true
	_, _, score := summarizeSharedPoolFullCheck(checks)
	if !sharedPoolFullCheckGatePassed(checks, score) {
		t.Fatalf("expected 70%% with core chat to pass, score=%.1f", score)
	}
	checks[0].Success = false
	if sharedPoolFullCheckGatePassed(checks, score) {
		t.Fatal("expected failed core chat to block gate")
	}
}

func TestFetchSharedPoolUpstreamModelsOnlyCallsModelsEndpoint(t *testing.T) {
	chatCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			chatCalled = true
			http.NotFound(w, r)
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token-fetch" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-a"},{"id":"gpt-b"},{"id":"gpt-a"}]}`))
	}))
	defer server.Close()

	svc := &BizDecipherService{}
	result, err := svc.FetchSharedPoolUpstreamModels(context.Background(), SharedPoolUpstreamModelsInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "test-token-fetch",
	})
	if err != nil {
		t.Fatalf("FetchSharedPoolUpstreamModels returned error: %v", err)
	}
	if chatCalled {
		t.Fatal("expected model fetch to avoid chat/full capability probe endpoints")
	}
	if result == nil || result.HTTPStatus != http.StatusOK {
		t.Fatalf("expected HTTP 200 result, got %#v", result)
	}
	if got := strings.Join(result.Models, ","); got != "gpt-a,gpt-b" {
		t.Fatalf("expected normalized models, got %q", got)
	}
}

func TestFetchSharedPoolUpstreamModelsUsesSavedPoolCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-saved-pool" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"saved-model"}]}`))
	}))
	defer server.Close()

	repo := &sharedPoolModelsRuntimeRepoStub{poolRuntime: &SharedPoolUpstreamRuntime{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-saved-pool",
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	result, err := svc.FetchSharedPoolUpstreamModels(context.Background(), SharedPoolUpstreamModelsInput{
		PoolID:  17,
		OwnerID: 29,
	})
	if err != nil {
		t.Fatalf("FetchSharedPoolUpstreamModels returned error: %v", err)
	}
	if repo.poolID != 17 || repo.ownerID != 29 {
		t.Fatalf("runtime lookup = pool %d owner %d, want pool 17 owner 29", repo.poolID, repo.ownerID)
	}
	if got := strings.Join(result.Models, ","); got != "saved-model" {
		t.Fatalf("models = %q, want saved-model", got)
	}
}

func TestFetchSharedPoolUpstreamModelsUsesSavedAccountCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-saved-account" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"account-model"}]}`))
	}))
	defer server.Close()

	repo := &sharedPoolModelsRuntimeRepoStub{accountRuntime: &SharedPoolUpstreamRuntime{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  "sk-saved-account",
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	result, err := svc.FetchSharedPoolUpstreamModels(context.Background(), SharedPoolUpstreamModelsInput{
		PoolID:    17,
		AccountID: 31,
		OwnerID:   29,
	})
	if err != nil {
		t.Fatalf("FetchSharedPoolUpstreamModels returned error: %v", err)
	}
	if repo.poolID != 17 || repo.accountID != 31 || repo.ownerID != 29 {
		t.Fatalf("runtime lookup = pool %d account %d owner %d", repo.poolID, repo.accountID, repo.ownerID)
	}
	if got := strings.Join(result.Models, ","); got != "account-model" {
		t.Fatalf("models = %q, want account-model", got)
	}
}

func TestFetchSharedPoolUpstreamModelsOAuthUsesConfiguredModelWithoutModelsEndpoint(t *testing.T) {
	credentials := `{"access_token":"fixture-access","chatgpt_account_id":"fixture-account"}`
	repo := &sharedPoolModelsRuntimeRepoStub{accountRuntime: &SharedPoolUpstreamRuntime{
		AuthType:             AccountTypeOAuth,
		CredentialsEncrypted: "enc:" + credentials,
		ProbeModel:           "gpt-fixture",
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(sharedPoolProbeSecretEncryptor{})
	result, err := svc.FetchSharedPoolUpstreamModels(context.Background(), SharedPoolUpstreamModelsInput{
		PoolID: 17, AccountID: 31, OwnerID: 29,
	})
	if err != nil {
		t.Fatalf("FetchSharedPoolUpstreamModels returned error: %v", err)
	}
	if result == nil || strings.Join(result.Models, ",") != "gpt-fixture" {
		t.Fatalf("unexpected OAuth model result: %#v", result)
	}
}

func TestReadSharedPoolOAuthProbeStream(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"po"}`,
		`data: {"type":"response.output_text.delta","delta":"ng"}`,
		`data: {"type":"response.completed"}`,
	}, "\n\n"))
	content, err := readSharedPoolOAuthProbeStream(stream)
	if err != nil {
		t.Fatalf("readSharedPoolOAuthProbeStream returned error: %v", err)
	}
	if content != "pong" {
		t.Fatalf("content = %q, want pong", content)
	}
}

func TestApplySharedPoolOAuthProbeRequestShapeAddsCodexHeaders(t *testing.T) {
	baseReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, chatgptCodexAPIURL, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	shaped := applySharedPoolOAuthProbeRequestShape(baseReq, SharedPoolUpstreamProbeInput{PoolID: 31, AccountID: 77}, "fixture-access", "fixture-account")
	if shaped == nil {
		t.Fatal("expected shaped request")
	}
	if got := HTTPUpstreamProfileFromContext(shaped.Context()); got != HTTPUpstreamProfileOpenAI {
		t.Fatalf("profile = %q, want %q", got, HTTPUpstreamProfileOpenAI)
	}
	if got := shaped.Host; got != "chatgpt.com" {
		t.Fatalf("host = %q, want chatgpt.com", got)
	}
	if got := shaped.Header.Get("Authorization"); got != "Bearer fixture-access" {
		t.Fatalf("authorization = %q", got)
	}
	if got := shaped.Header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("accept = %q", got)
	}
	if got := shaped.Header.Get("OpenAI-Beta"); got != "responses=experimental" {
		t.Fatalf("openai-beta = %q", got)
	}
	if got := shaped.Header.Get("Originator"); got != "codex_cli_rs" {
		t.Fatalf("originator = %q", got)
	}
	if got := shaped.Header.Get("User-Agent"); got != codexCLIUserAgent {
		t.Fatalf("user-agent = %q", got)
	}
	if got := shaped.Header.Get("Version"); got != codexCLIVersion {
		t.Fatalf("version = %q", got)
	}
	if got := shaped.Header.Get("chatgpt-account-id"); got != "fixture-account" {
		t.Fatalf("chatgpt-account-id = %q", got)
	}
	if got := shaped.Header.Get("Session_ID"); got != compactProbeSessionID(77) {
		t.Fatalf("session_id = %q", got)
	}
	if got := shaped.Header.Get("Conversation_ID"); got != compactProbeSessionID(77) {
		t.Fatalf("conversation_id = %q", got)
	}
}

func TestApplySharedPoolOAuthProbeRequestShapeFallsBackToPoolSessionID(t *testing.T) {
	baseReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, chatgptCodexAPIURL, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	shaped := applySharedPoolOAuthProbeRequestShape(baseReq, SharedPoolUpstreamProbeInput{PoolID: 31}, "fixture-access", "")
	if shaped.Header.Get("Session_ID") != "probe_shared_pool_31" {
		t.Fatalf("session_id = %q", shaped.Header.Get("Session_ID"))
	}
	if shaped.Header.Get("Conversation_ID") != "probe_shared_pool_31" {
		t.Fatalf("conversation_id = %q", shaped.Header.Get("Conversation_ID"))
	}
}

func TestSharedPoolOAuthCredentialsRedactsDecryptFailure(t *testing.T) {
	svc := &BizDecipherService{}
	_, err := svc.sharedPoolOAuthCredentials(&SharedPoolUpstreamRuntime{AuthType: AccountTypeOAuth, CredentialsEncrypted: "fixture-secret"})
	if err == nil {
		t.Fatal("expected decryption unavailable error")
	}
	if strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("credential leaked in error: %q", err.Error())
	}
}

func TestSharedPoolUpstreamRuntimeRedactsAllSensitiveFieldsFromJSON(t *testing.T) {
	payload, err := json.Marshal(SharedPoolUpstreamRuntime{
		AuthType:             AccountTypeOAuth,
		UpstreamBaseURL:      "https://fixture.invalid/backend-api/codex",
		UpstreamAPIKey:       "fixture-api-key",
		CredentialsEncrypted: "fixture-ciphertext",
		ProxyURL:             "https://fixture.invalid/proxy",
		ProbeModel:           "fixture-model",
	})
	if err != nil {
		t.Fatalf("marshal shared pool runtime: %v", err)
	}
	if string(payload) != `{}` {
		t.Fatalf("shared pool runtime leaked through JSON: %s", payload)
	}
}

func TestGetSharedPoolAccessKeyRedactsOAuthDecryptFailure(t *testing.T) {
	const secretContext = "fixture-secret-context"
	repo := &sharedPoolModelsRuntimeRepoStub{accessKey: &SharedPoolAccessKey{
		ID:                        1,
		OAuthCredentialsEncrypted: "fixture-ciphertext",
	}}
	svc := NewBizDecipherService(repo, nil, nil)
	svc.SetSecretEncryptor(sharedPoolFailingSecretEncryptor{errorText: secretContext})

	_, err := svc.GetSharedPoolAccessKeyByAPIKeyID(context.Background(), 1, "gpt-fixture")
	if err == nil {
		t.Fatal("expected OAuth decryption error")
	}
	if strings.Contains(err.Error(), secretContext) || strings.Contains(err.Error(), "fixture-ciphertext") {
		t.Fatalf("credential context leaked in error: %q", err.Error())
	}
}

func TestFetchSharedPoolUpstreamModelsRedactsAPIKeyFromError(t *testing.T) {
	const tokenValue = "test-token-fetch"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key "+tokenValue, http.StatusUnauthorized)
	}))
	defer server.Close()

	svc := &BizDecipherService{}
	_, err := svc.FetchSharedPoolUpstreamModels(context.Background(), SharedPoolUpstreamModelsInput{
		UpstreamBaseURL: server.URL,
		UpstreamAPIKey:  tokenValue,
	})
	if err == nil {
		t.Fatal("expected upstream error")
	}
	if strings.Contains(err.Error(), tokenValue) {
		t.Fatalf("expected redacted error, got %q", err.Error())
	}
}

func newSharedPoolProbeTestServer(t *testing.T, failMarker string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		content := sharedPoolProbeTestContent(payload.Messages)
		if failMarker != "" && strings.Contains(strings.ToLower(content), strings.ToLower(failMarker)) {
			content = "marker intentionally omitted"
		}
		if payload.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			mid := len(content) / 2
			first, second := content[:mid], content[mid:]
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + first + "\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + second + "\"}}]}\n\ndata: [DONE]\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
}

func serveSharedPoolProbeChatFixture(w http.ResponseWriter, r *http.Request, failMarker string) {
	if r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
		http.Error(w, "missing auth", http.StatusUnauthorized)
		return
	}
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream bool `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	content := sharedPoolProbeTestContent(payload.Messages)
	if failMarker != "" && strings.Contains(strings.ToLower(content), strings.ToLower(failMarker)) {
		content = "marker intentionally omitted"
	}
	writeSharedPoolProbeFixtureResponse(w, payload.Stream, content)
}

func writeSharedPoolProbeFixtureResponse(w http.ResponseWriter, stream bool, content string) {
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		mid := len(content) / 2
		first, second := content[:mid], content[mid:]
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + first + "\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + second + "\"}}]}\n\ndata: [DONE]\n\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{{"message": map[string]string{"content": content}}},
		"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
}

func sharedPoolProbeTestContent(messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		parts = append(parts, message.Content)
	}
	joined := strings.ToLower(strings.Join(parts, "\n"))
	switch {
	case strings.Contains(joined, "check_reasoning"):
		return "42 CHECK_REASONING"
	case strings.Contains(joined, "check_code"):
		return "function add(a, b) { return a + b } CHECK_CODE"
	case strings.Contains(joined, "check_json"):
		return `{"ok":true,"marker":"CHECK_JSON"}`
	case strings.Contains(joined, "check_zh"):
		return "共享池满血检测通过 CHECK_ZH"
	case strings.Contains(joined, "check_context"):
		return "Context summary CHECK_CONTEXT"
	case strings.Contains(joined, "check_temp"):
		return "CHECK_TEMP 7"
	case strings.Contains(joined, "check_stop"):
		return "CHECK_STOP"
	case strings.Contains(joined, "check_system"):
		return "hello CHECK_SYSTEM"
	case strings.Contains(joined, "check_unicode"):
		return "CHECK_UNICODE_零点城"
	case strings.Contains(joined, "check_usage"):
		return "CHECK_USAGE"
	case strings.Contains(joined, "check_stream"):
		return "CHECK_STREAM"
	case strings.Contains(joined, "check_error_shape"):
		return "CHECK_ERROR_SHAPE"
	case strings.Contains(joined, "check_latency"):
		return "CHECK_LATENCY"
	case strings.Contains(joined, "pong"):
		return "pong"
	default:
		return "ok"
	}
}
