package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/stretchr/testify/require"
)

func TestPersistAcceptedQuoteAndHold_preserves_decimal_strings(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	quote, hold := decimalQuoteAndHold(t, now)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO bizdecipher_accepted_price_quotes.*SELECT snapshot_sha256`).
		WithArgs("quote-1", "price-1", "gpt-5.6", "responses", "balance", []byte(`{"model":"gpt-5.6","unit_price":"0.0001"}`), quote.SnapshotSHA256(), "0.0001", now, now.Add(time.Minute), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"snapshot_sha256"}).AddRow(quote.SnapshotSHA256()))
	mock.ExpectQuery(`(?s)INSERT INTO bizdecipher_decimal_holds.*SELECT reservation_id`).
		WithArgs("reservation-1", "event-1", "request-1", quote.SnapshotSHA256(), "0.0001", "balance", now.Add(time.Minute), int64(3), "", "reserved", "worker-1", int64(7), int64(1), now).
		WillReturnRows(sqlmock.NewRows([]string{"reservation_id"}).AddRow("reservation-1"))
	mock.ExpectCommit()

	// When
	err = repo.PersistAcceptedQuoteAndHold(context.Background(), quote, hold)

	// Then
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTransitionDecimalHold_uses_version_and_epoch_CAS(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	quote, hold := decimalQuoteAndHold(t, now)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .* FROM bizdecipher_decimal_holds.*FOR UPDATE`).WithArgs("reservation-1").
		WillReturnRows(decimalHoldRows().AddRow(hold.ReservationID(), hold.BusinessEventID(), hold.RequestID(), quote.SnapshotSHA256(), "0.0001", "balance", hold.OriginalExpiresAt(), int64(3), "", "reserved", "worker-1", int64(7), int64(1), now))
	mock.ExpectExec(`(?s)UPDATE bizdecipher_decimal_holds.*WHERE reservation_id = \$1.*version = \$7.*lease_epoch = \$8`).
		WithArgs("reservation-1", "dispatching", "worker-1", int64(7), int64(2), now.Add(time.Second), int64(1), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// When
	decision, err := repo.TransitionDecimalHold(context.Background(), "reservation-1", billingcontract.TransitionCommand{
		To: billingcontract.HoldDispatching, ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(time.Second),
	})

	// Then
	require.NoError(t, err)
	require.True(t, decision.Applied())
	require.Equal(t, uint64(2), decision.Hold().Version())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRetainDecimalMediaTask_is_write_once_and_replay_safe(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	quote, hold := decimalQuoteAndHold(t, now)
	dispatching, err := billingcontract.DecideTransition(hold, billingcontract.TransitionCommand{To: billingcontract.HoldDispatching, ExpectedVersion: 1, ActiveEpoch: 7, UpdatedAt: now.Add(time.Second)})
	require.NoError(t, err)
	current := dispatching.Hold()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .* FROM bizdecipher_decimal_holds.*FOR UPDATE`).WithArgs("reservation-1").
		WillReturnRows(decimalHoldRows().AddRow(current.ReservationID(), current.BusinessEventID(), current.RequestID(), quote.SnapshotSHA256(), "0.0001", "balance", current.OriginalExpiresAt(), int64(3), "", "dispatching", "worker-1", int64(7), int64(2), current.UpdatedAt()))
	mock.ExpectExec(`(?s)UPDATE bizdecipher_decimal_holds.*accepted_media_task_id.*WHERE reservation_id = \$1.*version = \$4.*lease_epoch = \$5`).
		WithArgs("reservation-1", "media-123", int64(3), int64(2), int64(7), now.Add(2*time.Second)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// When
	decision, err := repo.RetainDecimalMediaTask(context.Background(), "reservation-1", billingcontract.MediaTaskCommand{
		TaskID: "media-123", ExpectedVersion: 2, ActiveEpoch: 7, UpdatedAt: now.Add(2 * time.Second),
	})

	// Then
	require.NoError(t, err)
	require.True(t, decision.Applied())
	require.Equal(t, "media-123", decision.Hold().AcceptedMediaTaskID())
	require.NoError(t, mock.ExpectationsWereMet())
}

func decimalQuoteAndHold(t *testing.T, now time.Time) (billingcontract.AcceptedQuote, billingcontract.Hold) {
	t.Helper()
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
	hold, err := billingcontract.NewHold(billingcontract.NewHoldInput{
		ReservationID: "reservation-1", BusinessEventID: "event-1", RequestID: "request-1", Quote: quote,
		Amount: amount, AvailableBalance: amount, LeaseOwner: "worker-1", ActiveEpoch: 7, CreatedAt: now,
	})
	require.NoError(t, err)
	return quote, hold
}

func decimalHoldRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"reservation_id", "business_event_id", "request_id", "quote_sha256", "amount", "asset", "original_expires_at",
		"source_epoch", "accepted_media_task_id", "state", "lease_owner", "lease_epoch", "version", "updated_at",
	})
}
