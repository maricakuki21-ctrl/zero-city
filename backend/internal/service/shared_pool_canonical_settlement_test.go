package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/stretchr/testify/require"
)

type canonicalSettlementRepoStub struct {
	BizDecipherRepository
	sharedPoolPricingRepository

	claims          []CanonicalSharedPoolUsageClaim
	recorded        []SharedPoolUsageInput
	markedEventIDs  []string
	resolveCalls    int
	resolveErr      error
	recordErr       error
	posted          bool
	missingIdentity bool
}

func (r *canonicalSettlementRepoStub) ListPendingCanonicalSharedPoolUsage(context.Context, int) ([]CanonicalSharedPoolUsageClaim, error) {
	claims := append([]CanonicalSharedPoolUsageClaim(nil), r.claims...)
	for i := range claims {
		claims[i].RequestID = claims[i].Event.RequestID()
		claims[i].EventID = claims[i].Event.EventID()
		if claims[i].Settlement == nil && r.resolveErr == nil {
			quote, _ := r.ResolveSharedPoolPriceQuote(context.Background(), 9, "", "")
			claims[i].Settlement, _ = NewCanonicalUsageSettlementSnapshot(quote)
		}
	}
	return claims, nil
}

func (r *canonicalSettlementRepoStub) ResolveCanonicalSharedPoolSettlementIdentity(context.Context, CanonicalSharedPoolUsageClaim) (*SharedPoolAccessKey, error) {
	if r.missingIdentity {
		return nil, nil
	}
	return &SharedPoolAccessKey{ID: 41, PoolID: 9, UserID: 7, AccountID: 3}, nil
}

func (r *canonicalSettlementRepoStub) HasPostedCanonicalSharedPoolSettlement(context.Context, SharedPoolUsageInput) (bool, error) {
	return r.posted, nil
}

func (r *canonicalSettlementRepoStub) DeferCanonicalSharedPoolSettlement(context.Context, string) error {
	return nil
}

func (r *canonicalSettlementRepoStub) MarkCanonicalSharedPoolUsageSettled(_ context.Context, eventID string) error {
	r.markedEventIDs = append(r.markedEventIDs, eventID)
	return nil
}

func (r *canonicalSettlementRepoStub) GetSharedPoolAccessKeyByAPIKeyID(context.Context, int64, string) (*SharedPoolAccessKey, error) {
	return &SharedPoolAccessKey{
		ID: 41, PoolID: 9, UserID: 7, PublishedModelName: "published-alias",
		CanonicalModelName: "canonical-official-model", RateMultiplier: 1,
	}, nil
}

func (r *canonicalSettlementRepoStub) ResolveSharedPoolPriceQuote(context.Context, int64, string, string) (*SharedPoolPriceQuote, error) {
	r.resolveCalls++
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}
	inputPrice := 1e-6
	outputPrice := 2e-6
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 55, PoolID: 9, PoolModelID: 12, EndpointID: 13,
		ModelName: "published-alias", EndpointType: SharedPoolEndpointResponses,
		PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "ready",
		ConfigVersion: 1, Multiplier: 1, EffectiveFrom: time.Unix(1_700_000_000, 0).UTC(),
		BasePrice: SharedPoolPriceComponents{
			BillingMode: "token", Currency: "USD",
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		},
	}
	FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func TestSettlePendingCanonicalSharedPoolUsageUsesFrozenRequestQuote(t *testing.T) {
	inputPrice := 3e-6
	outputPrice := 4e-6
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 77, PoolID: 9, PoolModelID: 12, EndpointID: 13,
		ModelName: "published-alias", EndpointType: SharedPoolEndpointResponses,
		PricingSource: SharedPoolPricingSourceOwner, PricingStatus: "ready",
		ConfigVersion: 2, Multiplier: 1, EffectiveFrom: time.Unix(1_700_000_000, 0).UTC(),
		BasePrice: SharedPoolPriceComponents{
			BillingMode: "token", Currency: "USD",
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		},
	}
	FinalizeSharedPoolPriceQuote(quote)
	snapshot, err := NewCanonicalUsageSettlementSnapshot(quote)
	require.NoError(t, err)

	repo := &canonicalSettlementRepoStub{
		claims: []CanonicalSharedPoolUsageClaim{{
			EventID:    "event-frozen",
			RequestID:  "req-canonical-frozen",
			APIKeyID:   41,
			Event:      canonicalSettlementEvent(t),
			Settlement: snapshot,
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	summary, err := svc.SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Settled)
	require.Zero(t, repo.resolveCalls, "a frozen quote must not be re-resolved from mutable pricing")
	require.Len(t, repo.recorded, 1)
	require.Equal(t, int64(77), repo.recorded[0].PriceVersionID)
	require.InDelta(t, 5e-4, repo.recorded[0].Cost, 1e-12)
}

func (r *canonicalSettlementRepoStub) RecordSharedPoolUsageTx(_ context.Context, input SharedPoolUsageInput) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	r.recorded = append(r.recorded, input)
	return nil
}

func canonicalSettlementEvent(t *testing.T) corecontracts.CanonicalUsageFinalized {
	t.Helper()
	inbound := "/v1/responses"
	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      "req-canonical-1",
		UsageReference: "sub2-usage:41:req-canonical-1",
		UserID:         7,
		AccountID:      19,
		GroupID:        23,
		Model:          "published-alias",
		Protocol:       inbound,
		Units: corecontracts.ExactUsageUnits{
			InputTokens:  100,
			OutputTokens: 50,
		},
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt: time.Unix(1_700_000_100, 0).UTC(),
		FinalizedAt: time.Unix(1_700_000_101, 0).UTC(),
	})
	require.NoError(t, err)
	return event
}

func TestSettlePendingCanonicalSharedPoolUsageDebitsMemberAndMarksSettled(t *testing.T) {
	repo := &canonicalSettlementRepoStub{
		claims: []CanonicalSharedPoolUsageClaim{{
			EventID:   "event-1",
			RequestID: "req-canonical-1",
			APIKeyID:  41,
			Event:     canonicalSettlementEvent(t),
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	summary, err := svc.SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Scanned)
	require.Equal(t, 1, summary.Settled)
	require.Zero(t, summary.Deferred)
	require.Zero(t, summary.Failed)

	require.Len(t, repo.recorded, 1)
	got := repo.recorded[0]
	require.Equal(t, "req-canonical-1", got.RequestID)
	require.Equal(t, int64(41), got.AccessKeyID)
	require.Equal(t, int64(9), got.PoolID)
	require.Equal(t, int64(7), got.UserID)
	require.True(t, got.Success)
	require.Equal(t, int64(55), got.PriceVersionID)
	require.Equal(t, SharedPoolPricingSourceOwner, got.PricingSource)
	// 100 input * 1e-6 + 50 output * 2e-6 = 2e-4
	require.InDelta(t, 2e-4, got.Cost, 1e-12)
	require.Equal(t, int64(3), got.AccountID, "canonical account 19 is not shared-pool account 3")
	require.Equal(t, []string{canonicalSettlementEvent(t).EventID()}, repo.markedEventIDs)
}

func TestSettlePendingCanonicalSharedPoolUsageDefersWhenPriceIsUnavailable(t *testing.T) {
	repo := &canonicalSettlementRepoStub{
		resolveErr: ErrSharedPoolPricingUnavailable,
		claims: []CanonicalSharedPoolUsageClaim{{
			EventID:   "event-2",
			RequestID: "req-canonical-2",
			APIKeyID:  41,
			Event:     canonicalSettlementEvent(t),
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	summary, err := svc.SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Scanned)
	require.Zero(t, summary.Settled)
	require.Equal(t, 1, summary.Deferred)
	require.Empty(t, repo.recorded)
	require.Empty(t, repo.markedEventIDs, "a deferred event must stay pending for a later retry")
}

func TestSettlePendingCanonicalSharedPoolUsageVerifiesReplayBeforeSettled(t *testing.T) {
	repo := &canonicalSettlementRepoStub{
		recordErr: ErrSharedPoolReservationConflict,
		posted:    true,
		claims: []CanonicalSharedPoolUsageClaim{{
			EventID:   "event-3",
			RequestID: "req-canonical-3",
			APIKeyID:  41,
			Event:     canonicalSettlementEvent(t),
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	summary, err := svc.SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Settled)
	require.Zero(t, summary.Failed)
	require.Equal(t, []string{canonicalSettlementEvent(t).EventID()}, repo.markedEventIDs)
}

func TestSettlePendingCanonicalSharedPoolUsageNeverPublishesUnverifiedConflict(t *testing.T) {
	repo := &canonicalSettlementRepoStub{recordErr: ErrSharedPoolReservationConflict,
		claims: []CanonicalSharedPoolUsageClaim{{Event: canonicalSettlementEvent(t), APIKeyID: 41}}}
	summary, err := NewBizDecipherService(repo, nil, nil).SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Failed)
	require.Empty(t, repo.markedEventIDs)
}

func TestSettlePendingCanonicalSharedPoolUsageMissingIdentityStaysPending(t *testing.T) {
	repo := &canonicalSettlementRepoStub{missingIdentity: true,
		claims: []CanonicalSharedPoolUsageClaim{{Event: canonicalSettlementEvent(t), APIKeyID: 41}}}
	summary, err := NewBizDecipherService(repo, nil, nil).SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Deferred)
	require.Empty(t, repo.markedEventIDs)
	require.Empty(t, repo.recorded)
}

func TestSettlePendingCanonicalSharedPoolUsageReportsRepositoryFailure(t *testing.T) {
	repo := &canonicalSettlementRepoStub{recordErr: errors.New("ledger unavailable")}
	repo.claims = []CanonicalSharedPoolUsageClaim{{
		EventID:   "event-4",
		RequestID: "req-canonical-4",
		APIKeyID:  41,
		Event:     canonicalSettlementEvent(t),
	}}
	svc := NewBizDecipherService(repo, nil, nil)

	summary, err := svc.SettlePendingCanonicalSharedPoolUsage(context.Background(), 50)
	require.NoError(t, err)
	require.Zero(t, summary.Settled)
	require.Equal(t, 1, summary.Failed)
	require.Empty(t, repo.markedEventIDs)
}
