package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
)

var ErrCanonicalUsageOutboxRequired = errors.New("canonical usage outbox repository is required")

type CanonicalUsageBillingRepository interface {
	UsageBillingRepository
	ApplyWithCanonicalUsage(
		ctx context.Context,
		cmd *UsageBillingCommand,
		event corecontracts.CanonicalUsageFinalized,
		settlement *CanonicalUsageSettlementSnapshot,
	) (*UsageBillingApplyResult, error)
}

type canonicalUsageEventSource struct {
	UsageLog *UsageLog
	APIKey   *APIKey
	Account  *Account
	Now      time.Time
}

type usageBillingRepositoryApply struct {
	Policy  corecontracts.BillingPolicy
	Command *UsageBillingCommand
	Event   corecontracts.CanonicalUsageFinalized
}

func applyUsageBillingRepository(
	ctx context.Context,
	repo UsageBillingRepository,
	input usageBillingRepositoryApply,
) (*UsageBillingApplyResult, error) {
	resolved, err := corecontracts.ResolveBillingPolicy(input.Policy)
	if err != nil {
		return nil, err
	}
	switch resolved {
	case corecontracts.BillingPolicyNativeSub2:
		return repo.Apply(ctx, input.Command)
	case corecontracts.BillingPolicyBizDecipherLedger:
		outboxRepo, ok := repo.(CanonicalUsageBillingRepository)
		if !ok {
			return nil, ErrCanonicalUsageOutboxRequired
		}
		settlement, err := canonicalUsageSettlementSnapshotFromContext(ctx)
		if err != nil {
			return nil, err
		}
		return outboxRepo.ApplyWithCanonicalUsage(ctx, input.Command, input.Event, settlement)
	default:
		return nil, fmt.Errorf("%w: %q", corecontracts.ErrBillingPolicyInvalid, resolved)
	}
}

func canonicalUsageSettlementSnapshotFromContext(ctx context.Context) (*CanonicalUsageSettlementSnapshot, error) {
	runtime, ok := CanonicalGatewayRuntimeFromContext(ctx)
	if !ok {
		return nil, nil
	}
	quote := runtime.AcceptedQuote()
	if quote == nil {
		return nil, nil
	}
	return NewCanonicalUsageSettlementSnapshot(quote)
}

func buildCanonicalUsageFinalized(source canonicalUsageEventSource) (corecontracts.CanonicalUsageFinalized, error) {
	if source.UsageLog == nil || source.APIKey == nil || source.Account == nil || source.APIKey.GroupID == nil {
		return corecontracts.CanonicalUsageFinalized{}, corecontracts.ErrCanonicalUsageIdentityRequired
	}
	usageLog := source.UsageLog
	committedAt := usageLog.CreatedAt
	if committedAt.IsZero() {
		committedAt = source.Now
	}
	protocol := usageLog.EffectiveRequestType().String()
	if usageLog.InboundEndpoint != nil && strings.TrimSpace(*usageLog.InboundEndpoint) != "" {
		protocol = strings.TrimSpace(*usageLog.InboundEndpoint) + ":" + protocol
	}
	videoDurationSeconds := 0
	if usageLog.VideoDurationSeconds != nil {
		videoDurationSeconds = *usageLog.VideoDurationSeconds
	}
	return corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      usageLog.RequestID,
		UsageReference: fmt.Sprintf("sub2-usage:%d:%s", source.APIKey.ID, usageLog.RequestID),
		UserID:         usageLog.UserID,
		AccountID:      source.Account.ID,
		GroupID:        *source.APIKey.GroupID,
		Model:          usageLog.Model,
		Protocol:       protocol,
		Units: corecontracts.ExactUsageUnits{
			InputTokens:          int64(usageLog.InputTokens),
			OutputTokens:         int64(usageLog.OutputTokens),
			CacheCreationTokens:  int64(usageLog.CacheCreationTokens),
			CacheReadTokens:      int64(usageLog.CacheReadTokens),
			ImageInputTokens:     int64(usageLog.ImageInputTokens),
			ImageOutputTokens:    int64(usageLog.ImageOutputTokens),
			ImageCount:           int64(usageLog.ImageCount),
			VideoCount:           int64(usageLog.VideoCount),
			VideoDurationSeconds: int64(videoDurationSeconds),
		},
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt: committedAt,
		FinalizedAt: source.Now,
	})
}
