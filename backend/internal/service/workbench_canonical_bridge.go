package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/gin-gonic/gin"
)

var (
	ErrWorkbenchCanonicalUnavailable = errors.New("workbench canonical gateway is unavailable")
	ErrWorkbenchCanonicalSettlement  = errors.New("workbench canonical settlement is unavailable")
)

type WorkbenchGateway interface {
	SelectAccountWithLoadAwareness(context.Context, *int64, string, string, map[int64]struct{}, string, int64) (*AccountSelectionResult, error)
	Forward(context.Context, *gin.Context, *Account, *ParsedRequest) (*ForwardResult, error)
	RecordUsage(context.Context, *RecordUsageInput) error
}

type WorkbenchCanonicalUsageClaim struct {
	RequestID string
	APIKeyID  int64
	UserID    int64
	AccountID int64
	GroupID   int64
	QuoteID   workbench.QuoteID
	QuoteSHA  workbench.Digest
}

type WorkbenchCanonicalSettlement struct {
	Event      corecontracts.CanonicalUsageFinalized
	Journal    platformledger.Journal
	ActualCost *workbench.Money
}

type WorkbenchSettlementSource interface {
	PreflightCanonicalUsage(context.Context, WorkbenchCanonicalRequest) error
	ClaimCanonicalUsage(context.Context, WorkbenchCanonicalUsageClaim) (WorkbenchCanonicalSettlement, error)
}

type WorkbenchLedgerPostResult struct {
	JournalID string
	Replayed  bool
}

type WorkbenchLedgerWriter interface {
	PostCanonicalUsage(context.Context, corecontracts.CanonicalUsageFinalized, platformledger.Journal) (WorkbenchLedgerPostResult, error)
}

type WorkbenchCanonicalBridgeDependencies struct {
	Gateway         WorkbenchGateway
	Settlement      WorkbenchSettlementSource
	Ledger          WorkbenchLedgerWriter
	RetryClassifier RetryClassifier
	Clock           WorkbenchClock
}

type WorkbenchCanonicalBridge struct {
	gateway         WorkbenchGateway
	settlement      WorkbenchSettlementSource
	ledger          WorkbenchLedgerWriter
	retryClassifier RetryClassifier
	clock           WorkbenchClock
	mu              sync.Mutex
	active          map[workbench.RunID]context.CancelFunc
}

type WorkbenchCanonicalRequest struct {
	RunID          workbench.RunID
	IdempotencyKey string
	Intent         string
	Resolved       ResolvedWorkbenchLaunch
}

type WorkbenchCanonicalResult struct {
	CanonicalRequestID    string
	CanonicalUsageEventID string
	JournalID             string
	Body                  []byte
	ContentType           string
	ActualCost            *workbench.Money
}

type WorkbenchCanonicalExecutionError struct {
	Err       error
	Retryable bool
}

func (e *WorkbenchCanonicalExecutionError) Error() string {
	return fmt.Sprintf("canonical workbench execution: %v", e.Err)
}

func (e *WorkbenchCanonicalExecutionError) Unwrap() error { return e.Err }

func NewWorkbenchCanonicalBridge(deps WorkbenchCanonicalBridgeDependencies) (*WorkbenchCanonicalBridge, error) {
	if deps.Gateway == nil || deps.Settlement == nil || deps.Ledger == nil {
		return nil, ErrWorkbenchCanonicalUnavailable
	}
	if deps.RetryClassifier == nil {
		deps.RetryClassifier = func(error) bool { return false }
	}
	if deps.Clock == nil {
		deps.Clock = systemWorkbenchClock{}
	}
	return &WorkbenchCanonicalBridge{gateway: deps.Gateway, settlement: deps.Settlement, ledger: deps.Ledger, retryClassifier: deps.RetryClassifier, clock: deps.Clock, active: make(map[workbench.RunID]context.CancelFunc)}, nil
}

func (b *WorkbenchCanonicalBridge) Execute(ctx context.Context, request WorkbenchCanonicalRequest) (WorkbenchCanonicalResult, error) {
	if err := validateWorkbenchCanonicalRequest(request); err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	if err := b.settlement.PreflightCanonicalUsage(ctx, request); err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("canonical settlement preflight: %w", err)
	}
	groupID := request.Resolved.GroupID
	selection, err := b.gateway.SelectAccountWithLoadAwareness(ctx, &groupID, string(request.RunID), request.Resolved.Capability.CanonicalModelID, nil, strconv.FormatInt(request.Resolved.User.ID, 10), request.Resolved.User.ID)
	if err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("select canonical account: %w", err)
	}
	if selection == nil || selection.Account == nil || !selection.Acquired {
		return WorkbenchCanonicalResult{}, ErrWorkbenchCanonicalUnavailable
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	runtime, err := NewCanonicalGatewayRuntime(CanonicalGatewayRuntimeInput{GroupID: groupID, AccountID: selection.Account.ID, BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger}, b.retryClassifier)
	if err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("create canonical gateway runtime: %w", err)
	}
	executionCtx, cancel := context.WithCancel(ctx)
	if err := b.register(request.RunID, cancel); err != nil {
		cancel()
		return WorkbenchCanonicalResult{}, err
	}
	defer b.unregister(request.RunID)
	defer cancel()

	body, parsed, ginContext, recorder, err := buildWorkbenchGatewayRequest(executionCtx, request)
	if err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	forwardCtx := WithCanonicalGatewayRuntime(executionCtx, runtime)
	parsed.GroupID = &groupID
	forwardResult, err := b.gateway.Forward(forwardCtx, ginContext, selection.Account, parsed)
	if err != nil {
		committed := recorder.Body.Len() > 0
		if committed {
			_ = runtime.Commit()
		}
		return WorkbenchCanonicalResult{}, &WorkbenchCanonicalExecutionError{Err: err, Retryable: !committed && runtime.CanRetry(err)}
	}
	if forwardResult == nil || forwardResult.RequestID == "" {
		return WorkbenchCanonicalResult{}, ErrWorkbenchCanonicalUnavailable
	}
	if err := runtime.Commit(); err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	payloadHash := sha256.Sum256(body)
	if err := b.gateway.RecordUsage(WithCanonicalGatewayRuntime(executionCtx, runtime), &RecordUsageInput{
		Result: forwardResult, APIKey: request.Resolved.APIKey, User: request.Resolved.User, Account: selection.Account,
		InboundEndpoint: "/biz/workbench/runs", UpstreamEndpoint: "/v1/messages", RequestPayloadHash: hex.EncodeToString(payloadHash[:]),
		QuotaPlatform: PlatformFromAPIKey(request.Resolved.APIKey), BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
	}); err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("record canonical usage: %w", err)
	}
	claim, err := b.settlement.ClaimCanonicalUsage(executionCtx, WorkbenchCanonicalUsageClaim{RequestID: forwardResult.RequestID, APIKeyID: request.Resolved.APIKey.ID, UserID: request.Resolved.User.ID, AccountID: selection.Account.ID, GroupID: groupID, QuoteID: request.Resolved.Quote.ID, QuoteSHA: request.Resolved.Quote.SHA256})
	if err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("claim canonical usage: %w", err)
	}
	if err := validateWorkbenchSettlementClaim(claim, request, forwardResult.RequestID, selection.Account.ID); err != nil {
		return WorkbenchCanonicalResult{}, err
	}
	posted, err := b.ledger.PostCanonicalUsage(executionCtx, claim.Event, claim.Journal)
	if err != nil {
		return WorkbenchCanonicalResult{}, fmt.Errorf("post canonical usage ledger: %w", err)
	}
	if posted.JournalID == "" {
		return WorkbenchCanonicalResult{}, ErrWorkbenchCanonicalSettlement
	}
	contentType := ginContext.Writer.Header().Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	return WorkbenchCanonicalResult{CanonicalRequestID: forwardResult.RequestID, CanonicalUsageEventID: claim.Event.EventID(), JournalID: posted.JournalID, Body: append([]byte(nil), recorder.Body.Bytes()...), ContentType: contentType, ActualCost: claim.ActualCost}, nil
}

func (b *WorkbenchCanonicalBridge) Cancel(ctx context.Context, runID workbench.RunID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	cancel := b.active[runID]
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (b *WorkbenchCanonicalBridge) register(runID workbench.RunID, cancel context.CancelFunc) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.active[runID]; exists {
		return fmt.Errorf("%w: run already active", ErrWorkbenchCanonicalUnavailable)
	}
	b.active[runID] = cancel
	return nil
}

func (b *WorkbenchCanonicalBridge) unregister(runID workbench.RunID) {
	b.mu.Lock()
	delete(b.active, runID)
	b.mu.Unlock()
}

func validateWorkbenchCanonicalRequest(request WorkbenchCanonicalRequest) error {
	if request.RunID == "" || request.IdempotencyKey == "" || request.Intent == "" || request.Resolved.User == nil || request.Resolved.APIKey == nil || request.Resolved.GroupID <= 0 {
		return fmt.Errorf("%w: incomplete canonical request", workbench.ErrInvalidCommand)
	}
	return nil
}

func buildWorkbenchGatewayRequest(ctx context.Context, request WorkbenchCanonicalRequest) ([]byte, *ParsedRequest, *gin.Context, *httptest.ResponseRecorder, error) {
	body, err := json.Marshal(struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Stream    bool   `json:"stream"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}{Model: request.Resolved.Capability.CanonicalModelID, MaxTokens: 4096, Messages: []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{{Role: "user", Content: request.Intent}}})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), domain.PlatformAnthropic)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parse canonical request: %w", err)
	}
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request, err = http.NewRequestWithContext(WithCanonicalGatewayRuntime(ctx, nil), http.MethodPost, "/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ginContext.Request.Header.Set("Content-Type", "application/json")
	ginContext.Request.Header.Set("Idempotency-Key", request.IdempotencyKey)
	return body, parsed, ginContext, recorder, nil
}

func validateWorkbenchSettlementClaim(claim WorkbenchCanonicalSettlement, request WorkbenchCanonicalRequest, requestID string, accountID int64) error {
	event := claim.Event
	if event.EventID() == "" || event.RequestID() != requestID || event.UserID() != request.Resolved.User.ID || event.AccountID() != accountID || event.GroupID() != request.Resolved.GroupID || event.Model() != request.Resolved.Capability.CanonicalModelID {
		return ErrWorkbenchCanonicalSettlement
	}
	return nil
}
