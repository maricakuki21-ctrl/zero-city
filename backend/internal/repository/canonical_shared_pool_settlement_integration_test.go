//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestCanonicalSharedPoolUsageSettlementSnapshotRoundTrip proves the request
// quote snapshot survives a real PostgreSQL JSONB write/read and remains bound
// to its canonical usage event by both hashes.
func TestCanonicalSharedPoolUsageSettlementSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &bizDecipherRepository{db: integrationDB}

	requestID := fmt.Sprintf("req-snapshot-roundtrip-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      requestID,
		UsageReference: fmt.Sprintf("sub2-usage:99:%s", requestID),
		UserID:         7,
		AccountID:      19,
		GroupID:        23,
		Model:          "published-alias",
		Protocol:       "/v1/responses",
		Units:          corecontracts.ExactUsageUnits{InputTokens: 100, OutputTokens: 50},
		CommitState:    corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt:    now,
		FinalizedAt:    now.Add(time.Second),
	})
	require.NoError(t, err)
	payload, err := event.MarshalJSON()
	require.NoError(t, err)
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	require.NoError(t, err)

	inputPrice := 1e-6
	outputPrice := 2e-6
	quote := &service.SharedPoolPriceQuote{
		PriceVersionID: 77, PoolID: 9, PoolModelID: 12, EndpointID: 13,
		ModelName: "published-alias", EndpointType: service.SharedPoolEndpointResponses,
		PricingSource: service.SharedPoolPricingSourceOwner, PricingStatus: "ready",
		ConfigVersion: 1, Multiplier: 1, EffectiveFrom: now,
		BasePrice: service.SharedPoolPriceComponents{
			BillingMode: "token", Currency: "USD",
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		},
	}
	service.FinalizeSharedPoolPriceQuote(quote)
	snapshot, err := service.NewCanonicalUsageSettlementSnapshot(quote)
	require.NoError(t, err)
	snapshotPayload, err := service.MarshalCanonicalUsageSettlementSnapshot(snapshot)
	require.NoError(t, err)
	snapshotSHA, err := service.CanonicalUsageSettlementSnapshotSHA256(snapshot)
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, `
INSERT INTO canonical_usage_outbox (
    event_id, request_id, api_key_id, account_id, group_id, payload, payload_sha256,
    settlement_snapshot, settlement_snapshot_sha256
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8::jsonb, $9)`,
		event.EventID(), requestID, int64(99), int64(19), int64(23),
		string(payload), payloadSHA, string(snapshotPayload), snapshotSHA)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			`DELETE FROM canonical_usage_outbox WHERE event_id = $1`, event.EventID())
	})

	claims, err := repo.ListPendingCanonicalSharedPoolUsage(ctx, 500)
	require.NoError(t, err)
	var found *service.CanonicalSharedPoolUsageClaim
	for i := range claims {
		if claims[i].EventID == event.EventID() {
			found = &claims[i]
			break
		}
	}
	require.NotNil(t, found)
	require.NotNil(t, found.Settlement)
	require.Equal(t, int64(77), found.Settlement.SharedPoolQuote.PriceVersionID)
	require.InDelta(t, 1e-6, *found.Settlement.SharedPoolQuote.BasePrice.InputPrice, 1e-15)
	require.InDelta(t, 2e-6, *found.Settlement.SharedPoolQuote.BasePrice.OutputPrice, 1e-15)
}
