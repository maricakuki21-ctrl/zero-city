package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestTokenPacketAllocation(t *testing.T) {
	for _, mode := range []string{"equal", "random"} {
		shares, err := AllocateTokenPacket(100_000_000, 64, mode)
		require.NoError(t, err)
		var sum int64
		for _, v := range shares {
			sum += v
			require.GreaterOrEqual(t, v, int64(781250))
			require.LessOrEqual(t, v, int64(2343750))
		}
		require.Equal(t, int64(100_000_000), sum)
		require.Len(t, shares, 64)
	}
	for total := int64(1); total < 50; total++ {
		for n := 1; n <= int(total); n++ {
			shares, err := AllocateTokenPacket(total, n, "random")
			require.NoError(t, err)
			var sum int64
			for _, v := range shares {
				require.Positive(t, v)
				sum += v
			}
			require.Equal(t, total, sum)
		}
	}
	for _, tc := range []struct {
		total int64
		n     int
		mode  string
	}{{0, 1, "equal"}, {1, 2, "random"}, {10, 3, "equal"}, {100, 2, "other"}, {1e9 + 1, 64, "random"}, {500, 501, "equal"}} {
		_, err := AllocateTokenPacket(tc.total, tc.n, tc.mode)
		require.Error(t, err)
	}
}

type tokenRewardRepoStub struct {
	TokenRewardRepository
	created   bool
	run       TokenRewardRun
	calls     int
	bound     int64
	requestID string
}

func (r *tokenRewardRepoStub) ValidateSponsor(context.Context, int64) (string, error) {
	return "sponsor-only", nil
}
func (r *tokenRewardRepoStub) ReserveTokenRun(_ context.Context, _ int64, _ TokenRewardRunInput, _ TokenRewardResource, bound int64, _ string) (*TokenRewardRun, bool, error) {
	r.bound = bound
	return &r.run, r.created, nil
}
func (r *tokenRewardRepoStub) FinishTokenRun(_ context.Context, _ int64, status string, actual int64, result, note string) (*TokenRewardRun, error) {
	r.calls++
	r.run.Status = status
	r.run.Actual = actual
	r.run.Result = result
	r.run.Note = note
	return &r.run, nil
}
func (r *tokenRewardRepoStub) LinkTokenRunRequest(_ context.Context, _ int64, id string) error {
	r.requestID = id
	return nil
}

func TestTokenRewardRunUsageAndFailure(t *testing.T) {
	for _, tc := range []struct {
		name, body, status string
		httpStatus         int
		actual             int64
	}{
		{"valid", `{"choices":[{"message":{"content":"作品"}}],"usage":{"prompt_tokens":50,"completion_tokens":20,"total_tokens":70,"prompt_tokens_details":{"cached_tokens":40},"completion_tokens_details":{"reasoning_tokens":10}}}`, "succeeded", 200, 70},
		{"missing_usage", `{"choices":[]}`, "review", 200, 0},
		{"missing_component", `{"usage":{"prompt_tokens":50}}`, "review", 200, 0},
		{"inconsistent", `{"usage":{"prompt_tokens":50,"completion_tokens":20,"total_tokens":90}}`, "review", 200, 0},
		{"negative", `{"usage":{"prompt_tokens":-1,"completion_tokens":20}}`, "review", 200, 0},
		{"upstream_error", `{"error":"unavailable"}`, "review", 500, 0},
		{"auth_error_not_proof_of_zero", `{"error":"denied"}`, "review", 403, 0},
		{"overshoot", `{"usage":{"prompt_tokens":90000,"completion_tokens":20}}`, "succeeded", 200, 90020},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer sponsor-only", r.Header.Get("Authorization"))
				require.Equal(t, "token-reward-12", r.Header.Get("X-Request-ID"))
				w.Header().Set("X-Client-Request-ID", "gateway-123")
				w.WriteHeader(tc.httpStatus)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			repo := &tokenRewardRepoStub{created: true, run: TokenRewardRun{ID: 12}}
			s := &TokenRewardService{repo: repo, resources: []TokenRewardResource{{ID: "text", Model: "test-model", KeyID: 1}}, endpoint: server.URL, client: server.Client()}
			got, err := s.Run(context.Background(), 10, TokenRewardRunInput{ClientID: "request_123", ResourceID: "text", Prompt: "你好", MaxOutput: 100})
			require.NoError(t, err)
			require.Equal(t, tc.status, got.Status)
			require.Equal(t, tc.actual, got.Actual)
			require.Equal(t, "client:gateway-123", repo.requestID)
			require.Equal(t, int64(len("你好")*4+4096+100), repo.bound)
		})
	}
}
func TestTokenRewardReplayDoesNotCallUpstream(t *testing.T) {
	repo := &tokenRewardRepoStub{created: false, run: TokenRewardRun{ID: 12, Status: "pending"}}
	s := &TokenRewardService{repo: repo, resources: []TokenRewardResource{{ID: "text", Model: "m", KeyID: 1}}}
	got, err := s.Run(context.Background(), 1, TokenRewardRunInput{ClientID: "request_123", ResourceID: "text", Prompt: "hello", MaxOutput: 100})
	require.NoError(t, err)
	require.Equal(t, "pending", got.Status)
	require.Zero(t, repo.calls)
}
func TestTokenRewardConfigurationFailsClosed(t *testing.T) {
	for _, raw := range []string{"", "invalid", `[{"id":"text","label":"Text","model":"m","key_id":1},{"id":"text","label":"dup","model":"m","key_id":2}]`} {
		t.Setenv("TOKEN_REWARD_RESOURCES", raw)
		require.Empty(t, NewTokenRewardService(nil, &config.Config{}).Resources())
	}
}
