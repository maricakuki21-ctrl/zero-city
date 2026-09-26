package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/stretchr/testify/require"
)

type decimalBillingRepoFake struct {
	quote billingcontract.AcceptedQuote
	hold  billingcontract.Hold
}

func (f *decimalBillingRepoFake) PersistAcceptedQuoteAndHold(_ context.Context, quote billingcontract.AcceptedQuote, hold billingcontract.Hold) error {
	f.quote, f.hold = quote, hold
	return nil
}

func TestSharedPoolDecimalBilling_ReserveBeforeDispatch_persists_canonical_snapshot(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	snapshot := []byte(`{"model":"gpt-5.6","unit_price":"0.0001"}`)
	digest := sha256.Sum256(snapshot)
	amount, err := billingcontract.ParseDecimal("0.0001")
	require.NoError(t, err)
	quote, err := billingcontract.NewAcceptedQuote(billingcontract.AcceptedQuoteInput{
		QuoteID: "quote-1", PriceVersionID: "price-1", Model: "gpt-5.6", Protocol: billingcontract.ProtocolResponses,
		Asset: billingcontract.AssetBalance, CanonicalSnapshotJSON: snapshot, SnapshotSHA256: hex.EncodeToString(digest[:]),
		MaximumAuthorizedCost: amount, AcceptedAt: now, ExpiresAt: now.Add(time.Minute), SourceEpoch: 3,
	})
	require.NoError(t, err)
	repo := &decimalBillingRepoFake{}
	billing := NewSharedPoolDecimalBilling(repo)

	// When
	hold, err := billing.ReserveBeforeDispatch(context.Background(), DecimalReservationInput{
		Quote: quote, ReservationID: "reservation-1", BusinessEventID: "event-1", RequestID: "request-1",
		Amount: amount, AvailableBalance: amount, LeaseOwner: "worker-1", ActiveEpoch: 7, CreatedAt: now,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, "0.0001", hold.Amount().String())
	require.Equal(t, quote.SnapshotSHA256(), repo.quote.SnapshotSHA256())
	require.Equal(t, hold.ReservationID(), repo.hold.ReservationID())
}
