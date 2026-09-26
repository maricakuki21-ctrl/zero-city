package billingcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAcceptedQuoteOwnsImmutableSnapshotBytes(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	quote := mustQuote(t, now, now.Add(time.Minute))
	originalSHA := quote.SnapshotSHA256()

	// When
	copyOfSnapshot := quote.CanonicalSnapshotJSON()
	copyOfSnapshot[0] = '['

	// Then
	require.Equal(t, byte('{'), quote.CanonicalSnapshotJSON()[0])
	require.Equal(t, originalSHA, quote.SnapshotSHA256())
}

func TestAcceptedQuoteCanonicalizesCompleteDecimalSnapshot(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	snapshot := []byte(`{
		"source_epoch":3,
		"fee_policy":"v1",
		"asset":"balance",
		"unit_prices":{"output":"0.0002","input":"0.0001"},
		"model":"gpt-5.6",
		"version":"price-1",
		"protocol":"responses",
		"multiplier":"1",
		"max_authorized_cost":"0.0003",
		"expires_at":"2026-09-02T08:01:00Z"
	}`)
	input := quoteInput(t, snapshot, now)

	// When
	quote, err := NewAcceptedQuote(input)

	// Then
	require.NoError(t, err)
	require.Equal(t, `{"model":"gpt-5.6","version":"price-1","protocol":"responses","asset":"balance","unit_prices":{"input":"0.0001","output":"0.0002"},"multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0003","expires_at":"2026-09-02T08:01:00Z","source_epoch":3}`, string(quote.CanonicalSnapshotJSON()))
	digest := sha256.Sum256(quote.CanonicalSnapshotJSON())
	require.Equal(t, hex.EncodeToString(digest[:]), quote.SnapshotSHA256())
}

func TestAcceptedQuoteRejectsIncompleteOrUnsafeCanonicalSnapshot(t *testing.T) {
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	complete := `{"model":"gpt-5.6","version":"price-1","protocol":"responses","asset":"balance","unit_prices":{"input":"0.0001"},"multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0001","expires_at":"2026-09-02T08:01:00Z","source_epoch":3}`
	tests := []struct {
		name     string
		snapshot string
	}{
		{name: "empty object", snapshot: `{}`},
		{name: "missing unit prices", snapshot: `{"model":"gpt-5.6","version":"price-1","protocol":"responses","asset":"balance","multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0001","expires_at":"2026-09-02T08:01:00Z","source_epoch":3}`},
		{name: "float unit price", snapshot: `{"model":"gpt-5.6","version":"price-1","protocol":"responses","asset":"balance","unit_prices":{"input":0.0001},"multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0001","expires_at":"2026-09-02T08:01:00Z","source_epoch":3}`},
		{name: "unknown asset", snapshot: `{"model":"gpt-5.6","version":"price-1","protocol":"responses","asset":"cash","unit_prices":{"input":"0.0001"},"multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0001","expires_at":"2026-09-02T08:01:00Z","source_epoch":3}`},
		{name: "empty asset", snapshot: `{"model":"gpt-5.6","version":"price-1","protocol":"responses","asset":"","unit_prices":{"input":"0.0001"},"multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0001","expires_at":"2026-09-02T08:01:00Z","source_epoch":3}`},
		{name: "unknown field", snapshot: complete[:len(complete)-1] + `,"unexpected":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			input := quoteInput(t, []byte(test.snapshot), now)

			// When
			_, err := NewAcceptedQuote(input)

			// Then
			require.ErrorIs(t, err, ErrInvalidQuote)
		})
	}
}

func quoteInput(t *testing.T, snapshot []byte, acceptedAt time.Time) AcceptedQuoteInput {
	t.Helper()
	digest := sha256.Sum256(snapshot)
	return AcceptedQuoteInput{
		QuoteID: "quote-1", PriceVersionID: "price-1", Model: "gpt-5.6", Protocol: ProtocolResponses,
		Asset: AssetBalance, CanonicalSnapshotJSON: snapshot, SnapshotSHA256: hex.EncodeToString(digest[:]),
		MaximumAuthorizedCost: mustDecimal(t, "0.0003"), AcceptedAt: acceptedAt,
		ExpiresAt: acceptedAt.Add(time.Minute), SourceEpoch: 3,
	}
}
