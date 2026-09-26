package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sharedPoolMediaSettlementRepoStub struct {
	BizDecipherRepository
	staged   *SharedPoolUsageInput
	recorded *SharedPoolUsageInput
}

func (r *sharedPoolMediaSettlementRepoStub) ReserveSharedPoolUsageTx(_ context.Context, _ SharedPoolUsageReservationInput) (*SharedPoolUsageReservation, error) {
	return nil, nil
}

func (r *sharedPoolMediaSettlementRepoStub) MarkSharedPoolUsageForwardingTx(_ context.Context, _ int64, _ string) error {
	return nil
}

func (r *sharedPoolMediaSettlementRepoStub) MarkSharedPoolUsageReviewRequiredTx(_ context.Context, _ int64, _, _ string) error {
	return nil
}

func (r *sharedPoolMediaSettlementRepoStub) ReleaseSharedPoolUsageAfterVerifiedFailureTx(_ context.Context, _ int64, _, _ string) error {
	return nil
}

func (r *sharedPoolMediaSettlementRepoStub) StageSharedPoolUsageSettlementTx(_ context.Context, input SharedPoolUsageInput) error {
	copy := input
	r.staged = &copy
	return nil
}

func (r *sharedPoolMediaSettlementRepoStub) RecordSharedPoolUsageTx(_ context.Context, input SharedPoolUsageInput) error {
	copy := input
	r.recorded = &copy
	return nil
}

func (r *sharedPoolMediaSettlementRepoStub) ListPendingSharedPoolUsageSettlements(_ context.Context, _ int) ([]SharedPoolUsageInput, error) {
	return nil, nil
}

func (r *sharedPoolMediaSettlementRepoStub) RecoverExpiredSharedPoolUsageReservationsTx(_ context.Context, _ time.Time, _ int) (*SharedPoolUsageReservationRecoverySummary, error) {
	return nil, nil
}

func TestRecordSharedPoolImageUsageRequiresObservedCount(t *testing.T) {
	repo, gateway, apiKey, accessKey := newSharedPoolMediaSettlementFixture()

	err := gateway.RecordSharedPoolMediaUsage(
		context.Background(), apiKey, accessKey, SharedPoolEndpointImageGeneration,
		&OpenAIForwardResult{ImageCount: 1, ImageCountObserved: false}, "req-unobserved", 1,
	)
	require.ErrorIs(t, err, ErrSharedPoolUsageUnavailable)
	require.Nil(t, repo.staged)
	require.Nil(t, repo.recorded)
}

func TestRecordSharedPoolImageUsageRejectsCountBeyondReservation(t *testing.T) {
	repo, gateway, apiKey, accessKey := newSharedPoolMediaSettlementFixture()

	err := gateway.RecordSharedPoolMediaUsage(
		context.Background(), apiKey, accessKey, SharedPoolEndpointImageGeneration,
		&OpenAIForwardResult{ImageCount: 3, ImageCountObserved: true}, "req-over", 2,
	)
	require.ErrorIs(t, err, ErrSharedPoolUnsafeCostEstimate)
	require.Nil(t, repo.staged)
	require.Nil(t, repo.recorded)
}

func TestRecordSharedPoolImageUsageSettlesObservedCountWithinReservation(t *testing.T) {
	repo, gateway, apiKey, accessKey := newSharedPoolMediaSettlementFixture()

	err := gateway.RecordSharedPoolMediaUsage(
		context.Background(), apiKey, accessKey, SharedPoolEndpointImageGeneration,
		&OpenAIForwardResult{Model: "image-model", ImageCount: 2, ImageCountObserved: true, ImageSize: ImageBillingSize1K},
		"req-observed", 2,
	)
	require.NoError(t, err)
	require.NotNil(t, repo.staged)
	require.NotNil(t, repo.recorded)
	require.InDelta(t, 0.2, repo.staged.Cost, 1e-12)
	require.InDelta(t, 0.2, repo.recorded.Cost, 1e-12)
	require.Equal(t, "req-observed", repo.recorded.RequestID)
}

func newSharedPoolMediaSettlementFixture() (*sharedPoolMediaSettlementRepoStub, *OpenAIGatewayService, *APIKey, *SharedPoolAccessKey) {
	unit := 0.1
	quote := &SharedPoolPriceQuote{
		PriceVersionID: 77,
		PoolID:         9,
		PoolModelID:    10,
		EndpointID:     11,
		ModelName:      "image-model",
		EndpointType:   SharedPoolEndpointImageGeneration,
		PricingSource:  SharedPoolPricingSourceOwner,
		PricingStatus:  "ready",
		ConfigVersion:  2,
		BasePrice: SharedPoolPriceComponents{
			BillingMode:    "image",
			Currency:       "USD",
			ImageItemPrice: &unit,
		},
		Multiplier:    1,
		EffectiveFrom: time.Now().UTC(),
	}
	FinalizeSharedPoolPriceQuote(quote)
	repo := &sharedPoolMediaSettlementRepoStub{}
	biz := NewBizDecipherService(repo, nil, nil)
	return repo,
		&OpenAIGatewayService{bizDecipherService: biz},
		&APIKey{ID: 5},
		&SharedPoolAccessKey{ID: 6, PoolID: 9, UserID: 7, PublishedModelName: "image-model", PriceQuote: quote}
}
