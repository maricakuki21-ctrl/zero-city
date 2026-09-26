package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestListPendingCanonicalSharedPoolUsageDecodesEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      "req-list-1",
		UsageReference: "sub2-usage:41:req-list-1",
		UserID:         7,
		AccountID:      19,
		GroupID:        23,
		Model:          "published-alias",
		Protocol:       "/v1/responses",
		Units:          corecontracts.ExactUsageUnits{InputTokens: 10, OutputTokens: 5},
		CommitState:    corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt:    time.Unix(1_700_000_100, 0).UTC(),
		FinalizedAt:    time.Unix(1_700_000_101, 0).UTC(),
	})
	require.NoError(t, err)
	payload, err := event.MarshalJSON()
	require.NoError(t, err)
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	require.NoError(t, err)

	mock.ExpectQuery("FROM canonical_usage_outbox").
		WithArgs(25).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "payload", "payload_sha256", "settlement_snapshot", "settlement_snapshot_sha256"}).
			AddRow(event.EventID(), "req-list-1", int64(41), payload, payloadSHA, nil, nil))

	repo := &bizDecipherRepository{db: db}
	claims, err := repo.ListPendingCanonicalSharedPoolUsage(context.Background(), 25)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, event.EventID(), claims[0].EventID)
	require.Equal(t, "req-list-1", claims[0].RequestID)
	require.Equal(t, int64(41), claims[0].APIKeyID)
	require.Equal(t, "req-list-1", claims[0].Event.RequestID())
	require.Equal(t, "published-alias", claims[0].Event.Model())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPendingCanonicalSharedPoolUsageRejectsPayloadHashMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      "req-tampered",
		UsageReference: "sub2-usage:41:req-tampered",
		UserID:         7,
		AccountID:      19,
		GroupID:        23,
		Model:          "published-alias",
		Protocol:       "/v1/responses",
		Units:          corecontracts.ExactUsageUnits{InputTokens: 10, OutputTokens: 5},
		CommitState:    corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt:    time.Unix(1_700_000_100, 0).UTC(),
		FinalizedAt:    time.Unix(1_700_000_101, 0).UTC(),
	})
	require.NoError(t, err)
	payload, err := event.MarshalJSON()
	require.NoError(t, err)

	mock.ExpectQuery("FROM canonical_usage_outbox").
		WithArgs(25).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "payload", "payload_sha256", "settlement_snapshot", "settlement_snapshot_sha256"}).
			AddRow("event-1", "req-tampered", int64(41), payload, strings.Repeat("0", 64), nil, nil))

	repo := &bizDecipherRepository{db: db}
	_, err = repo.ListPendingCanonicalSharedPoolUsage(context.Background(), 25)
	require.ErrorContains(t, err, "payload hash mismatch")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPendingCanonicalSharedPoolUsageDecodesSettlementSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      "req-snapshot-1",
		UsageReference: "sub2-usage:41:req-snapshot-1",
		UserID:         7,
		AccountID:      19,
		GroupID:        23,
		Model:          "published-alias",
		Protocol:       "/v1/responses",
		Units:          corecontracts.ExactUsageUnits{InputTokens: 10, OutputTokens: 5},
		CommitState:    corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt:    time.Unix(1_700_000_100, 0).UTC(),
		FinalizedAt:    time.Unix(1_700_000_101, 0).UTC(),
	})
	require.NoError(t, err)
	payload, err := event.MarshalJSON()
	require.NoError(t, err)
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	require.NoError(t, err)

	inputPrice := 0.0001
	outputPrice := 0.0002
	quote := &service.SharedPoolPriceQuote{
		PriceVersionID: 81, PoolID: 9, PoolModelID: 12, EndpointID: 13,
		ModelName: "published-alias", EndpointType: service.SharedPoolEndpointResponses,
		PricingSource: service.SharedPoolPricingSourceOwner, PricingStatus: "ready",
		ConfigVersion: 1, Multiplier: 1, EffectiveFrom: time.Unix(1_700_000_000, 0).UTC(),
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

	mock.ExpectQuery("FROM canonical_usage_outbox").
		WithArgs(25).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "payload", "payload_sha256", "settlement_snapshot", "settlement_snapshot_sha256"}).
			AddRow(event.EventID(), "req-snapshot-1", int64(41), payload, payloadSHA, snapshotPayload, snapshotSHA))

	repo := &bizDecipherRepository{db: db}
	claims, err := repo.ListPendingCanonicalSharedPoolUsage(context.Background(), 25)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.NotNil(t, claims[0].Settlement)
	require.Equal(t, int64(81), claims[0].Settlement.SharedPoolQuote.PriceVersionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPendingCanonicalSharedPoolUsageRejectsSettlementHashMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:         corecontracts.BillingPolicyBizDecipherLedger,
		RequestID:      "req-snapshot-tampered",
		UsageReference: "sub2-usage:41:req-snapshot-tampered",
		UserID:         7,
		AccountID:      19,
		GroupID:        23,
		Model:          "published-alias",
		Protocol:       "/v1/responses",
		Units:          corecontracts.ExactUsageUnits{InputTokens: 10, OutputTokens: 5},
		CommitState:    corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt:    time.Unix(1_700_000_100, 0).UTC(),
		FinalizedAt:    time.Unix(1_700_000_101, 0).UTC(),
	})
	require.NoError(t, err)
	payload, err := event.MarshalJSON()
	require.NoError(t, err)
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	require.NoError(t, err)

	inputPrice := 0.0001
	outputPrice := 0.0002
	quote := &service.SharedPoolPriceQuote{
		PriceVersionID: 82, PoolID: 9, PoolModelID: 12, EndpointID: 13,
		ModelName: "published-alias", EndpointType: service.SharedPoolEndpointResponses,
		PricingSource: service.SharedPoolPricingSourceOwner, PricingStatus: "ready",
		ConfigVersion: 1, Multiplier: 1, EffectiveFrom: time.Unix(1_700_000_000, 0).UTC(),
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

	mock.ExpectQuery("FROM canonical_usage_outbox").
		WithArgs(25).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "payload", "payload_sha256", "settlement_snapshot", "settlement_snapshot_sha256"}).
			AddRow(event.EventID(), "req-snapshot-tampered", int64(41), payload, payloadSHA, snapshotPayload, strings.Repeat("0", 64)))

	repo := &bizDecipherRepository{db: db}
	_, err = repo.ListPendingCanonicalSharedPoolUsage(context.Background(), 25)
	require.ErrorContains(t, err, "settlement snapshot")
	require.ErrorContains(t, err, "hash mismatch")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkCanonicalSharedPoolUsageSettledUsesPendingGuard(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec("UPDATE canonical_usage_outbox").
		WithArgs("event-9").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &bizDecipherRepository{db: db}
	require.NoError(t, repo.MarkCanonicalSharedPoolUsageSettled(context.Background(), "event-9"))
	require.NoError(t, mock.ExpectationsWereMet())
}
