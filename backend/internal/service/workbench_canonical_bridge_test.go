package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchGatewayStub struct {
	mu             sync.Mutex
	account        *Account
	forwardResult  *ForwardResult
	forwardErr     error
	blockUntilDone bool
	started        chan struct{}
	selectCalls    int
	forwardCalls   int
	recordCalls    int
	forwardRuntime *CanonicalGatewayRuntime
	recordRuntime  *CanonicalGatewayRuntime
}

func (s *workbenchGatewayStub) SelectAccountWithLoadAwareness(
	context.Context,
	*int64,
	string,
	string,
	map[int64]struct{},
	string,
	int64,
) (*AccountSelectionResult, error) {
	s.mu.Lock()
	s.selectCalls++
	s.mu.Unlock()
	return &AccountSelectionResult{Account: s.account, Acquired: true, ReleaseFunc: func() {}}, nil
}

func (s *workbenchGatewayStub) Forward(ctx context.Context, c *gin.Context, _ *Account, _ *ParsedRequest) (*ForwardResult, error) {
	s.mu.Lock()
	s.forwardCalls++
	s.forwardRuntime, _ = CanonicalGatewayRuntimeFromContext(ctx)
	started := s.started
	block := s.blockUntilDone
	s.mu.Unlock()
	if started != nil {
		select {
		case <-started:
		default:
			close(started)
		}
	}
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	c.Header("Content-Type", "application/json")
	_, _ = c.Writer.Write([]byte(`{"content":"done"}`))
	return s.forwardResult, s.forwardErr
}

func (s *workbenchGatewayStub) RecordUsage(ctx context.Context, _ *RecordUsageInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordCalls++
	s.recordRuntime, _ = CanonicalGatewayRuntimeFromContext(ctx)
	return nil
}

type workbenchSettlementSourceStub struct {
	claim WorkbenchCanonicalSettlement
	calls int
	preflightErr error
	preflightCalls int
}

func (s *workbenchSettlementSourceStub) PreflightCanonicalUsage(context.Context, WorkbenchCanonicalRequest) error {
	s.preflightCalls++
	return s.preflightErr
}

func (s *workbenchSettlementSourceStub) ClaimCanonicalUsage(context.Context, WorkbenchCanonicalUsageClaim) (WorkbenchCanonicalSettlement, error) {
	s.calls++
	return s.claim, nil
}

type workbenchLedgerStub struct {
	calls int
}

func (s *workbenchLedgerStub) PostCanonicalUsage(context.Context, corecontracts.CanonicalUsageFinalized, platformledger.Journal) (WorkbenchLedgerPostResult, error) {
	s.calls++
	return WorkbenchLedgerPostResult{JournalID: "journal_1"}, nil
}

func TestWorkbenchCanonicalBridgeUsesOneRuntimeAndCanonicalAccountingSeams(t *testing.T) {
	// Given
	request := canonicalWorkbenchCanonicalRequest(t)
	gateway := &workbenchGatewayStub{account: &Account{ID: 81}, forwardResult: &ForwardResult{RequestID: "req_1", Model: request.Resolved.Capability.CanonicalModelID}}
	settlement := &workbenchSettlementSourceStub{claim: canonicalWorkbenchSettlement(t, request, "req_1")}
	ledger := &workbenchLedgerStub{}
	bridge, err := NewWorkbenchCanonicalBridge(WorkbenchCanonicalBridgeDependencies{Gateway: gateway, Settlement: settlement, Ledger: ledger})
	require.NoError(t, err)

	// When
	result, err := bridge.Execute(context.Background(), request)

	// Then
	require.NoError(t, err)
	require.Equal(t, "req_1", result.CanonicalRequestID)
	require.Equal(t, settlement.claim.Event.EventID(), result.CanonicalUsageEventID)
	require.Equal(t, "journal_1", result.JournalID)
	require.Equal(t, 1, gateway.selectCalls)
	require.Equal(t, 1, gateway.forwardCalls)
	require.Equal(t, 1, gateway.recordCalls)
	require.Same(t, gateway.forwardRuntime, gateway.recordRuntime)
	require.Equal(t, 1, settlement.calls)
	require.Equal(t, 1, ledger.calls)
	require.JSONEq(t, `{"content":"done"}`, string(result.Body))
}

func TestWorkbenchCanonicalBridgePreflightRejectsBeforeAnyGatewayCall(t *testing.T) {
	request := canonicalWorkbenchCanonicalRequest(t)
	gateway := &workbenchGatewayStub{}
	settlement := &workbenchSettlementSourceStub{preflightErr: ErrWorkbenchCanonicalSettlement}
	ledger := &workbenchLedgerStub{}
	bridge, err := NewWorkbenchCanonicalBridge(WorkbenchCanonicalBridgeDependencies{
		Gateway: gateway, Settlement: settlement, Ledger: ledger,
	})
	require.NoError(t, err)
	result, err := bridge.Execute(context.Background(), request)
	require.ErrorIs(t, err, ErrWorkbenchCanonicalSettlement)
	require.ErrorContains(t, err, "preflight")
	require.Equal(t, 1, settlement.preflightCalls)
	require.Zero(t, gateway.selectCalls)
	require.Zero(t, gateway.forwardCalls)
	require.Zero(t, gateway.recordCalls)
	require.Zero(t, settlement.calls)
	require.Zero(t, ledger.calls)
	require.Empty(t, result.CanonicalRequestID)
	require.Nil(t, result.ActualCost)
}

func TestWorkbenchCanonicalBridgePostCommitDisconnectDoesNotRedispatch(t *testing.T) {
	// Given
	request := canonicalWorkbenchCanonicalRequest(t)
	gateway := &workbenchGatewayStub{account: &Account{ID: 81}, forwardResult: &ForwardResult{RequestID: "req_disconnect", Model: request.Resolved.Capability.CanonicalModelID, ClientDisconnect: true}}
	settlement := &workbenchSettlementSourceStub{claim: canonicalWorkbenchSettlement(t, request, "req_disconnect")}
	bridge, err := NewWorkbenchCanonicalBridge(WorkbenchCanonicalBridgeDependencies{Gateway: gateway, Settlement: settlement, Ledger: &workbenchLedgerStub{}})
	require.NoError(t, err)

	// When
	_, err = bridge.Execute(context.Background(), request)

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, gateway.forwardCalls)
	require.False(t, gateway.forwardRuntime.CanRetry(errors.New("retryable")))
}

func TestWorkbenchCanonicalBridgeCancelPropagatesToActiveRequest(t *testing.T) {
	// Given
	request := canonicalWorkbenchCanonicalRequest(t)
	gateway := &workbenchGatewayStub{account: &Account{ID: 81}, blockUntilDone: true, started: make(chan struct{})}
	bridge, err := NewWorkbenchCanonicalBridge(WorkbenchCanonicalBridgeDependencies{Gateway: gateway, Settlement: &workbenchSettlementSourceStub{}, Ledger: &workbenchLedgerStub{}})
	require.NoError(t, err)
	errCh := make(chan error, 1)
	go func() {
		_, executeErr := bridge.Execute(context.Background(), request)
		errCh <- executeErr
	}()
	<-gateway.started

	// When
	err = bridge.Cancel(context.Background(), request.RunID)

	// Then
	require.NoError(t, err)
	require.ErrorIs(t, <-errCh, context.Canceled)
	require.Equal(t, 1, gateway.forwardCalls)
}

func canonicalWorkbenchCanonicalRequest(t *testing.T) WorkbenchCanonicalRequest {
	t.Helper()
	now := time.Date(2026, time.September, 3, 3, 0, 0, 0, time.UTC)
	record := canonicalWorkbenchCatalogRecord(now)
	return WorkbenchCanonicalRequest{
		RunID: "wbr_bridge", IdempotencyKey: "dispatch-once", Intent: "summarize the release",
		Resolved: ResolvedWorkbenchLaunch{Capability: record.Capability, Quote: record.Quote, User: record.User, APIKey: record.APIKey, GroupID: record.Quote.GroupID},
	}
}

func canonicalWorkbenchSettlement(t *testing.T, request WorkbenchCanonicalRequest, requestID string) WorkbenchCanonicalSettlement {
	t.Helper()
	now := time.Date(2026, time.September, 3, 3, 0, 0, 0, time.UTC)
	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy: corecontracts.BillingPolicyBizDecipherLedger, RequestID: requestID, UsageReference: "sub2-usage:71:" + requestID,
		UserID: request.Resolved.User.ID, AccountID: 81, GroupID: request.Resolved.GroupID,
		Model: request.Resolved.Capability.CanonicalModelID, Protocol: "/v1/messages:messages",
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted, CommittedAt: now, FinalizedAt: now,
	})
	require.NoError(t, err)
	return WorkbenchCanonicalSettlement{Event: event, Journal: platformledger.Journal{ID: "journal_1", EventID: event.EventID()}}
}

var _ = http.MethodPost
var _ workbench.RunID
