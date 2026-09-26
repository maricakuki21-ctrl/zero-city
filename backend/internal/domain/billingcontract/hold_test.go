package billingcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHoldRejectsMissingExpiredQuoteAndInsufficientFundsBeforeDispatch(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	amount := mustDecimal(t, "0.0001")

	// When
	_, missingErr := NewHold(validHoldInput(AcceptedQuote{}, amount, amount, now))
	expired := mustQuote(t, now.Add(-time.Minute), now)
	_, expiredErr := NewHold(validHoldInput(expired, amount, amount, now))
	valid := mustQuote(t, now, now.Add(time.Minute))
	_, fundsErr := NewHold(validHoldInput(valid, amount, mustDecimal(t, "0"), now))

	// Then
	require.ErrorIs(t, missingErr, ErrInvalidQuote)
	require.ErrorIs(t, expiredErr, ErrExpiredQuote)
	require.ErrorIs(t, fundsErr, ErrInsufficientFunds)
}

func TestHoldTransitionCASRejectsStaleVersionAndEpoch(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	hold := mustHold(t, now)

	// When
	_, versionErr := DecideTransition(hold, TransitionCommand{To: HoldDispatching, ExpectedVersion: 2, ActiveEpoch: 7, UpdatedAt: now.Add(time.Second)})
	_, epochErr := DecideTransition(hold, TransitionCommand{To: HoldDispatching, ExpectedVersion: 1, ActiveEpoch: 6, UpdatedAt: now.Add(time.Second)})

	// Then
	require.ErrorIs(t, versionErr, ErrStaleVersion)
	require.ErrorIs(t, epochErr, ErrStaleEpoch)
}

func TestHoldTransitionTableRejectsCaptureBeforeDispatch(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	hold := mustHold(t, now)

	// When
	_, err := DecideTransition(hold, TransitionCommand{To: HoldCaptured, ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(time.Second)})

	// Then
	require.ErrorIs(t, err, ErrInvalidTransition)
}

func TestHoldCaptureAndReleaseAreIdempotentButMutuallyExclusive(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	hold := mustHold(t, now)
	hold = mustTransition(t, hold, HoldDispatching, now.Add(time.Second))
	hold = mustTransition(t, hold, HoldSettlementPending, now.Add(2*time.Second))
	hold = mustTransition(t, hold, HoldCaptured, now.Add(3*time.Second))

	// When
	duplicate, duplicateErr := DecideTransition(hold, TransitionCommand{To: HoldCaptured, ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(4 * time.Second)})
	_, releaseErr := DecideTransition(hold, TransitionCommand{To: HoldReleased, ExpectedVersion: hold.Version(), ActiveEpoch: 7, UpdatedAt: now.Add(4 * time.Second)})

	// Then
	require.NoError(t, duplicateErr)
	require.False(t, duplicate.Applied())
	require.Equal(t, hold.Version(), duplicate.Hold().Version())
	require.True(t, errors.Is(releaseErr, ErrHoldFinalized))
}

func TestHoldReleaseReplayIsIdempotent(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	hold := mustTransition(t, mustHold(t, now), HoldReleased, now.Add(time.Second))

	// When
	replay, err := DecideTransition(hold, TransitionCommand{To: HoldReleased, ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(2 * time.Second)})

	// Then
	require.NoError(t, err)
	require.False(t, replay.Applied())
	require.Equal(t, hold.Version(), replay.Hold().Version())
}

func TestLeaseReassignmentRequiresVersionAndAdvancingEpoch(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	hold := mustHold(t, now)

	// When
	decision, err := ReassignLease(hold, LeaseCommand{Owner: "worker-2", LeaseEpoch: 8, ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(time.Second)})
	_, staleErr := ReassignLease(decision.Hold(), LeaseCommand{Owner: "worker-3", LeaseEpoch: 9, ExpectedVersion: 2, ActiveEpoch: 7, UpdatedAt: now.Add(2 * time.Second)})

	// Then
	require.NoError(t, err)
	require.True(t, decision.Applied())
	require.Equal(t, uint64(8), decision.Hold().LeaseEpoch())
	require.Equal(t, uint64(2), decision.Hold().Version())
	require.ErrorIs(t, staleErr, ErrStaleEpoch)
}

func TestRestoreHoldPreservesImmutableAndControlFieldsWithoutTransition(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	quote := mustQuote(t, now, now.Add(time.Minute))

	// When
	restored, err := RestoreHold(RestoreHoldInput{
		ReservationID: "legacy-reservation", BusinessEventID: "legacy-event", RequestID: "legacy-request",
		QuoteSHA256: quote.SnapshotSHA256(), Amount: mustDecimal(t, "0.0001"), Asset: AssetBalance,
		OriginalExpiresAt: quote.ExpiresAt(), SourceEpoch: 3, AcceptedMediaTaskID: "media-123",
		State: HoldUnknown, LeaseOwner: "canonical-standby", LeaseEpoch: 8, Version: 11, UpdatedAt: now,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, "legacy-reservation", restored.ReservationID())
	require.Equal(t, quote.SnapshotSHA256(), restored.QuoteSHA256())
	require.Equal(t, "0.0001", restored.Amount().String())
	require.Equal(t, "media-123", restored.AcceptedMediaTaskID())
	require.Equal(t, HoldUnknown, restored.State())
	require.Equal(t, uint64(11), restored.Version())
}

func TestAcceptedMediaTaskIDIsRetainedAcrossUnknownRecovery(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	hold := mustTransition(t, mustHold(t, now), HoldDispatching, now.Add(time.Second))

	// When
	bound, err := RetainAcceptedMediaTask(hold, MediaTaskCommand{TaskID: "media-123", ExpectedVersion: hold.Version(), ActiveEpoch: 7, UpdatedAt: now.Add(2 * time.Second)})
	require.NoError(t, err)
	unknown := mustTransition(t, bound.Hold(), HoldUnknown, now.Add(3*time.Second))
	replay, replayErr := RetainAcceptedMediaTask(unknown, MediaTaskCommand{TaskID: "media-123", ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(4 * time.Second)})
	_, conflictErr := RetainAcceptedMediaTask(unknown, MediaTaskCommand{TaskID: "media-456", ExpectedVersion: unknown.Version(), ActiveEpoch: 7, UpdatedAt: now.Add(4 * time.Second)})

	// Then
	require.NoError(t, replayErr)
	require.False(t, replay.Applied())
	require.Equal(t, "media-123", replay.Hold().AcceptedMediaTaskID())
	require.ErrorIs(t, conflictErr, ErrMediaTaskIDConflict)
}

func mustDecimal(t *testing.T, raw string) Decimal {
	t.Helper()
	value, err := ParseDecimal(raw)
	require.NoError(t, err)
	return value
}

func mustQuote(t *testing.T, acceptedAt, expiresAt time.Time) AcceptedQuote {
	t.Helper()
	snapshot := []byte(`{"model":"gpt-5.6","unit_price":"0.0001","multiplier":"1","fee_policy":"v1"}`)
	digest := sha256.Sum256(snapshot)
	quote, err := NewAcceptedQuote(AcceptedQuoteInput{
		QuoteID: "quote-1", PriceVersionID: "price-1", Model: "gpt-5.6", Protocol: ProtocolResponses,
		Asset: AssetBalance, CanonicalSnapshotJSON: snapshot, SnapshotSHA256: hex.EncodeToString(digest[:]),
		MaximumAuthorizedCost: mustDecimal(t, "0.0001"), AcceptedAt: acceptedAt, ExpiresAt: expiresAt, SourceEpoch: 3,
	})
	require.NoError(t, err)
	return quote
}

func validHoldInput(quote AcceptedQuote, amount, available Decimal, now time.Time) NewHoldInput {
	return NewHoldInput{
		ReservationID: "reservation-1", BusinessEventID: "event-1", RequestID: "request-1",
		Quote: quote, Amount: amount, AvailableBalance: available, LeaseOwner: "worker-1", ActiveEpoch: 7, CreatedAt: now,
	}
}

func mustHold(t *testing.T, now time.Time) Hold {
	t.Helper()
	amount := mustDecimal(t, "0.0001")
	hold, err := NewHold(validHoldInput(mustQuote(t, now, now.Add(time.Minute)), amount, amount, now))
	require.NoError(t, err)
	return hold
}

func mustTransition(t *testing.T, hold Hold, to HoldState, at time.Time) Hold {
	t.Helper()
	decision, err := DecideTransition(hold, TransitionCommand{To: to, ExpectedVersion: hold.Version(), ActiveEpoch: hold.LeaseEpoch(), UpdatedAt: at})
	require.NoError(t, err)
	require.True(t, decision.Applied())
	return decision.Hold()
}
