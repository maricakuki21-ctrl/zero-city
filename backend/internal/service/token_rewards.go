package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Reward resources are deliberately operator-controlled. No client may choose a
// sponsor key, URL, arbitrary model, multimodal payload or tools.
type TokenRewardResource struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Model string `json:"model"`
	KeyID int64  `json:"-"`
}

type TokenPacketInput struct {
	ClientID     string `json:"client_id"`
	ResourceID   string `json:"resource_id"`
	Total        int64  `json:"total_tokens"`
	Portions     int    `json:"portions"`
	Mode         string `json:"mode"`
	Blessing     string `json:"blessing"`
	DelaySeconds int    `json:"delay_seconds"`
	ClaimHours   int    `json:"claim_hours"`
	UseHours     int    `json:"use_hours"`
}

type TokenPacket struct {
	ID         int64       `json:"id"`
	MessageID  int64       `json:"message_id"`
	SenderID   int64       `json:"sender_id"`
	ResourceID string      `json:"resource_id"`
	Model      string      `json:"model"`
	Total      int64       `json:"total_tokens"`
	Portions   int         `json:"portions"`
	Claimed    int         `json:"claimed"`
	Mode       string      `json:"mode"`
	Blessing   string      `json:"blessing"`
	OpensAt    time.Time   `json:"opens_at"`
	ClosesAt   time.Time   `json:"closes_at"`
	UseHours   int         `json:"use_hours"`
	ServerTime time.Time   `json:"server_time"`
	Mine       *TokenGrant `json:"mine,omitempty"`
}

type TokenGrant struct {
	ID         int64     `json:"id"`
	PacketID   int64     `json:"packet_id"`
	ResourceID string    `json:"resource_id"`
	Model      string    `json:"model"`
	Tokens     int64     `json:"tokens"`
	Used       int64     `json:"used"`
	Reserved   int64     `json:"reserved"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type TokenRewardRunInput struct {
	ClientID   string `json:"client_id"`
	ResourceID string `json:"resource_id"`
	Prompt     string `json:"prompt"`
	MaxOutput  int    `json:"max_output_tokens"`
}
type TokenRewardRun struct {
	ID         int64     `json:"id"`
	ClientID   string    `json:"client_id"`
	ResourceID string    `json:"resource_id"`
	Model      string    `json:"model"`
	Reserved   int64     `json:"reserved"`
	Actual     int64     `json:"actual"`
	Covered    int64     `json:"covered"`
	Status     string    `json:"status"`
	Result     string    `json:"result"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}
type TokenRewardWallet struct {
	Grants     []TokenGrant     `json:"grants"`
	Runs       []TokenRewardRun `json:"runs"`
	ServerTime time.Time        `json:"server_time"`
}

type TokenRewardRepository interface {
	ValidateSponsor(context.Context, int64) (string, error)
	CreateTokenPacket(context.Context, string, int64, TokenPacketInput, TokenRewardResource, []int64, string) (*TokenPacket, error)
	GetTokenPacket(context.Context, int64, int64) (*TokenPacket, error)
	ClaimTokenPacket(context.Context, int64, int64) (*TokenGrant, error)
	TokenRewardWallet(context.Context, int64) (*TokenRewardWallet, error)
	ReserveTokenRun(context.Context, int64, TokenRewardRunInput, TokenRewardResource, int64, string) (*TokenRewardRun, bool, error)
	FinishTokenRun(context.Context, int64, string, int64, string, string) (*TokenRewardRun, error)
	LinkTokenRunRequest(context.Context, int64, string) error
	ReviewTokenRuns(context.Context) ([]TokenRewardReview, error)
	ResolveTokenRun(context.Context, int64, int64, int64, string) (*TokenRewardRun, error)
}

type TokenRewardReview struct {
	TokenRewardRun
	UserID           int64  `json:"user_id"`
	GatewayRequestID string `json:"gateway_request_id"`
}

func (s *TokenRewardService) Reviews(ctx context.Context) ([]TokenRewardReview, error) {
	return s.repo.ReviewTokenRuns(ctx)
}
func (s *TokenRewardService) Resolve(ctx context.Context, id, operator, actual int64, evidence string) (*TokenRewardRun, error) {
	evidence = strings.TrimSpace(evidence)
	if actual < 0 || actual > 2e9 || utf8.RuneCountInString(evidence) < 10 || utf8.RuneCountInString(evidence) > 2000 {
		return nil, rewardBad("需要填写可追溯的用量核对依据（10～2000 字），不可凭猜测退还")
	}
	return s.repo.ResolveTokenRun(ctx, id, operator, actual, evidence)
}

type TokenRewardService struct {
	repo      TokenRewardRepository
	resources []TokenRewardResource
	endpoint  string
	client    *http.Client
}

func NewTokenRewardService(repo TokenRewardRepository, cfg *config.Config) *TokenRewardService {
	s := &TokenRewardService{repo: repo, endpoint: fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", cfg.Server.Port),
		client: &http.Client{Timeout: 150 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{Proxy: nil}}}
	// Fail closed on missing/invalid configuration; no real giveaway is enabled
	// until an operator has tested a reliable text model and sponsor budget.
	var entries []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Model string `json:"model"`
		KeyID int64  `json:"key_id"`
	}
	if json.Unmarshal([]byte(os.Getenv("TOKEN_REWARD_RESOURCES")), &entries) != nil {
		return s
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`).MatchString(e.ID) || e.KeyID <= 0 || e.Model == "" || len(e.Model) > 200 || e.Label == "" || seen[e.ID] {
			s.resources = nil
			return s
		}
		seen[e.ID] = true
		s.resources = append(s.resources, TokenRewardResource{e.ID, e.Label, e.Model, e.KeyID})
	}
	return s
}
func (s *TokenRewardService) Resources() []TokenRewardResource {
	if s.resources == nil {
		return []TokenRewardResource{}
	}
	return s.resources
}
func (s *TokenRewardService) resource(id string) (TokenRewardResource, error) {
	for _, r := range s.resources {
		if r.ID == id {
			return r, nil
		}
	}
	return TokenRewardResource{}, rewardBad("奖励资源尚未启用或不可用")
}
func rewardBad(message string) error { return infraerrors.BadRequest("TOKEN_REWARD_INVALID", message) }
func rewardHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func validRewardClientID(id string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_-]{8,100}$`).MatchString(id)
}

// Allocation is bounded to 0.5x..1.5x the equal share (rounded), then securely
// shuffled. Integer totals are conserved and every share is positive.
func AllocateTokenPacket(total int64, count int, mode string) ([]int64, error) {
	if total < 1 || total > 1_000_000_000 || count < 1 || count > 500 || total < int64(count) || (mode != "equal" && mode != "random") {
		return nil, rewardBad("红包总量、份数或模式不正确")
	}
	if mode == "equal" && total%int64(count) != 0 {
		return nil, rewardBad("普通红包总量必须能被份数整除")
	}
	out := make([]int64, count)
	base := total / int64(count)
	for i := range out {
		out[i] = base
		if int64(i) < total%int64(count) {
			out[i]++
		}
	}
	if mode == "random" {
		minShare := base / 2
		if minShare < 1 {
			minShare = 1
		}
		maxShare := (total + int64(count) - 1) / int64(count) * 3 / 2
		if maxShare < base+1 {
			maxShare = base + 1
		}
		for i := 0; i < count*8; i++ {
			a, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
			if err != nil {
				return nil, err
			}
			b, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
			if err != nil {
				return nil, err
			}
			x, y := int(a.Int64()), int(b.Int64())
			if x == y {
				continue
			}
			room := out[x] - minShare
			if maxShare-out[y] < room {
				room = maxShare - out[y]
			}
			if room <= 0 {
				continue
			}
			n, err := rand.Int(rand.Reader, big.NewInt(room+1))
			if err != nil {
				return nil, err
			}
			out[x] -= n.Int64()
			out[y] += n.Int64()
		}
		for i := count - 1; i > 0; i-- {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
			if err != nil {
				return nil, err
			}
			j := int(n.Int64())
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}
func (s *TokenRewardService) Create(ctx context.Context, slug string, user int64, in TokenPacketInput) (*TokenPacket, error) {
	in.Blessing = strings.TrimSpace(in.Blessing)
	if !validRewardClientID(in.ClientID) || utf8.RuneCountInString(in.Blessing) > 160 || in.Blessing == "" || in.DelaySeconds < 0 || in.DelaySeconds > 3600 || in.ClaimHours < 1 || in.ClaimHours > 168 || in.UseHours < 1 || in.UseHours > 720 {
		return nil, rewardBad("红包规则不正确")
	}
	r, err := s.resource(in.ResourceID)
	if err != nil {
		return nil, err
	}
	if _, err = s.repo.ValidateSponsor(ctx, r.KeyID); err != nil {
		return nil, err
	}
	shares, err := AllocateTokenPacket(in.Total, in.Portions, in.Mode)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateTokenPacket(ctx, slug, user, in, r, shares, rewardHash(in))
}
func (s *TokenRewardService) Get(ctx context.Context, id, user int64) (*TokenPacket, error) {
	return s.repo.GetTokenPacket(ctx, id, user)
}
func (s *TokenRewardService) Claim(ctx context.Context, id, user int64) (*TokenGrant, error) {
	return s.repo.ClaimTokenPacket(ctx, id, user)
}
func (s *TokenRewardService) Wallet(ctx context.Context, user int64) (*TokenRewardWallet, error) {
	return s.repo.TokenRewardWallet(ctx, user)
}

func (s *TokenRewardService) Run(ctx context.Context, user int64, in TokenRewardRunInput) (*TokenRewardRun, error) {
	if !validRewardClientID(in.ClientID) || strings.TrimSpace(in.Prompt) == "" || len(in.Prompt) > 16000 || in.MaxOutput < 1 || in.MaxOutput > 2048 {
		return nil, rewardBad("请输入文本，最大输出为 1–2048 Token")
	}
	r, err := s.resource(in.ResourceID)
	if err != nil {
		return nil, err
	}
	key, err := s.repo.ValidateSponsor(ctx, r.KeyID)
	if err != nil {
		return nil, err
	}
	// Text-only conservative allowance. Overshoot is recorded, never taken from
	// cash. Actual usage is the provider's prompt+completion, not this estimate.
	bound := int64(len(in.Prompt)*4 + 4096 + in.MaxOutput)
	run, created, err := s.repo.ReserveTokenRun(ctx, user, in, r, bound, rewardHash(in))
	if err != nil || !created {
		return run, err
	}
	payload, _ := json.Marshal(map[string]any{"model": r.Model, "messages": []map[string]string{{"role": "user", "content": in.Prompt}}, "max_completion_tokens": in.MaxOutput, "stream": false})
	// Browser disconnect must not release an in-flight entitlement or replay a
	// paid request. A crash leaves a persisted pending run for reconciliation.
	callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 160*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(callCtx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Request-ID", fmt.Sprintf("token-reward-%d", run.ID))
	resp, callErr := s.client.Do(req)
	finish := func(status string, actual int64, result, note string) (*TokenRewardRun, error) {
		finalCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		return s.repo.FinishTokenRun(finalCtx, run.ID, status, actual, result, note)
	}
	if callErr != nil {
		return finish("review", 0, "", "调用结果待核对；不会扣充值余额，请勿重复提交")
	}
	defer resp.Body.Close()
	if correlation := resp.Header.Get("X-Client-Request-ID"); correlation != "" && len(correlation) <= 180 {
		if err := s.repo.LinkTokenRunRequest(callCtx, run.ID, "client:"+correlation); err != nil {
			return finish("review", 0, "", "调用追踪写入失败，请按活动调用编号核对")
		}
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if readErr != nil || len(body) > 2*1024*1024 {
		return finish("review", 0, "", "响应不完整，运营需核对实际用量")
	}
	// Even error HTTP responses may follow a partial upstream execution. Do not
	// infer zero use from a status code; preserve reservation for reconciliation.
	if resp.StatusCode != 200 {
		return finish("review", 0, "", "上游调用未正常完成，奖励预留等待用量核对")
	}
	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			Prompt     *int64 `json:"prompt_tokens"`
			Completion *int64 `json:"completion_tokens"`
			Total      *int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &data) != nil || data.Usage == nil || data.Usage.Prompt == nil || data.Usage.Completion == nil || *data.Usage.Prompt < 0 || *data.Usage.Completion < 0 || *data.Usage.Prompt > 1e9 || *data.Usage.Completion > 1e9 {
		return finish("review", 0, "", "供应商缺少可靠 Token 用量，待核对")
	}
	actual := *data.Usage.Prompt + *data.Usage.Completion
	if actual <= 0 || (data.Usage.Total != nil && *data.Usage.Total != actual) {
		return finish("review", 0, "", "供应商用量口径不一致，待核对")
	}
	result := ""
	if len(data.Choices) > 0 {
		result = data.Choices[0].Message.Content
	}
	note := ""
	if actual > bound {
		note = "用量超出预留，差额由活动方承担"
	}
	return finish("succeeded", actual, result, note)
}
