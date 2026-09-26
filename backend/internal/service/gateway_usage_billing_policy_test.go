package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/stretchr/testify/require"
)

type canonicalUsageBillingRepoRecorder struct {
	nativeCalls int
	outboxCalls int
	command     *UsageBillingCommand
	event       corecontracts.CanonicalUsageFinalized
	settlement  *CanonicalUsageSettlementSnapshot
}

func (r *canonicalUsageBillingRepoRecorder) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.nativeCalls++
	r.command = cmd
	return &UsageBillingApplyResult{}, nil
}

func (r *canonicalUsageBillingRepoRecorder) ApplyWithCanonicalUsage(
	_ context.Context,
	cmd *UsageBillingCommand,
	event corecontracts.CanonicalUsageFinalized,
	settlement *CanonicalUsageSettlementSnapshot,
) (*UsageBillingApplyResult, error) {
	r.outboxCalls++
	r.command = cmd
	r.event = event
	r.settlement = settlement
	return &UsageBillingApplyResult{}, nil
}

func (*canonicalUsageBillingRepoRecorder) ReserveBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func (*canonicalUsageBillingRepoRecorder) CaptureBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func (*canonicalUsageBillingRepoRecorder) ReleaseBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

type canonicalUsageQuotaUpdater struct{}

func (canonicalUsageQuotaUpdater) UpdateQuotaUsed(context.Context, int64, float64) error { return nil }
func (canonicalUsageQuotaUpdater) UpdateRateLimitUsage(context.Context, int64, float64) error {
	return nil
}

type usageBillingRepoWithoutOutbox struct {
	applyCalls int
}

func (r *usageBillingRepoWithoutOutbox) Apply(context.Context, *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.applyCalls++
	return &UsageBillingApplyResult{}, nil
}

func (*usageBillingRepoWithoutOutbox) ReserveBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func (*usageBillingRepoWithoutOutbox) CaptureBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func (*usageBillingRepoWithoutOutbox) ReleaseBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func Test_BillingPolicy_shared_market_skips_native_balance_and_emits_one_event(t *testing.T) {
	// Given
	groupID := int64(19)
	repo := &canonicalUsageBillingRepoRecorder{}
	usageLog := canonicalUsageTestLog(groupID)
	params := &postUsageBillingParams{
		Cost:                  &CostBreakdown{TotalCost: 3, ActualCost: 2},
		User:                  &User{ID: 11},
		APIKey:                &APIKey{ID: 17, GroupID: &groupID, Group: &Group{ID: groupID}, Quota: 20},
		Account:               &Account{ID: 13, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 100.0}},
		AccountRateMultiplier: 1.5,
		APIKeyService:         canonicalUsageQuotaUpdater{},
		BillingPolicy:         corecontracts.BillingPolicyBizDecipherLedger,
	}
	command := buildUsageBillingCommand(usageLog.RequestID, usageLog, params)
	event, err := buildCanonicalUsageFinalized(canonicalUsageEventSource{
		UsageLog: usageLog,
		APIKey:   params.APIKey,
		Account:  params.Account,
		Now:      usageLog.CreatedAt.Add(time.Second),
	})
	require.NoError(t, err)

	// When
	_, err = applyUsageBillingRepository(context.Background(), repo, usageBillingRepositoryApply{
		Policy:  params.BillingPolicy,
		Command: command,
		Event:   event,
	})

	// Then
	require.NoError(t, err)
	require.Zero(t, repo.nativeCalls)
	require.Equal(t, 1, repo.outboxCalls)
	require.Zero(t, repo.command.BalanceCost)
	require.Zero(t, repo.command.CreditCost)
	require.Zero(t, repo.command.SubscriptionCost)
	require.Equal(t, 2.0, repo.command.APIKeyQuotaCost)
	require.Equal(t, 4.5, repo.command.AccountQuotaCost)
	require.Equal(t, usageLog.RequestID, repo.event.RequestID())
	require.Equal(t, corecontracts.CanonicalUsageCommitStateUsageCommitted, repo.event.CommitState())
}

func Test_BillingPolicy_native_retains_existing_balance_path(t *testing.T) {
	// Given
	repo := &canonicalUsageBillingRepoRecorder{}
	command := &UsageBillingCommand{RequestID: "native-request", BalanceCost: 2}

	// When
	_, err := applyUsageBillingRepository(context.Background(), repo, usageBillingRepositoryApply{
		Policy:  corecontracts.BillingPolicyNativeSub2,
		Command: command,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, repo.nativeCalls)
	require.Zero(t, repo.outboxCalls)
	require.Equal(t, 2.0, repo.command.BalanceCost)
}

func Test_BillingPolicy_shared_market_rejects_repository_without_durable_outbox(t *testing.T) {
	// Given
	repo := &usageBillingRepoWithoutOutbox{}

	// When
	_, err := applyUsageBillingRepository(context.Background(), repo, usageBillingRepositoryApply{
		Policy:  corecontracts.BillingPolicyBizDecipherLedger,
		Command: &UsageBillingCommand{RequestID: "shared-request"},
	})

	// Then
	require.ErrorIs(t, err, ErrCanonicalUsageOutboxRequired)
	require.Zero(t, repo.applyCalls)
}

func canonicalUsageTestLog(groupID int64) *UsageLog {
	inbound := "/v1/responses"
	return &UsageLog{
		UserID:               11,
		APIKeyID:             17,
		AccountID:            13,
		GroupID:              &groupID,
		RequestID:            "req-7",
		Model:                "gpt-5.6",
		InputTokens:          101,
		OutputTokens:         37,
		CacheCreationTokens:  23,
		CacheReadTokens:      29,
		ImageInputTokens:     31,
		ImageOutputTokens:    41,
		ImageCount:           2,
		VideoCount:           1,
		VideoDurationSeconds: intPointer(9),
		RequestType:          RequestTypeStream,
		InboundEndpoint:      &inbound,
		CreatedAt:            time.Date(2026, time.September, 1, 8, 30, 0, 0, time.UTC),
	}
}

func intPointer(value int) *int { return &value }
