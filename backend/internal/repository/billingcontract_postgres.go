package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
)

var ErrDecimalBillingConflict = errors.New("decimal billing record conflicts with persisted state")

const selectDecimalHold = `SELECT reservation_id, business_event_id, request_id, quote_sha256,
	amount::text, asset, original_expires_at, source_epoch, COALESCE(accepted_media_task_id, ''),
	state, lease_owner, lease_epoch, version, updated_at
	FROM bizdecipher_decimal_holds WHERE reservation_id = $1`

const selectDecimalHoldForUpdate = selectDecimalHold + ` FOR UPDATE`

func (r *bizDecipherRepository) PersistAcceptedQuoteAndHold(
	ctx context.Context,
	quote billingcontract.AcceptedQuote,
	hold billingcontract.Hold,
) (retErr error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin decimal reservation: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("rollback decimal reservation: %w", err))
		}
	}()

	var persistedSHA string
	err = tx.QueryRowContext(ctx, `WITH inserted AS (
		INSERT INTO bizdecipher_accepted_price_quotes
			(quote_id, price_version_id, model, protocol, asset, canonical_snapshot, snapshot_sha256,
			 maximum_authorized_cost, accepted_at, expires_at, source_epoch)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CAST($8 AS NUMERIC(24,12)), $9, $10, $11)
		ON CONFLICT (quote_id) DO NOTHING RETURNING snapshot_sha256
	)
	SELECT snapshot_sha256 FROM inserted
	UNION ALL
	SELECT snapshot_sha256 FROM bizdecipher_accepted_price_quotes WHERE quote_id = $1
	LIMIT 1`, quote.QuoteID(), quote.PriceVersionID(), quote.Model(), string(quote.Protocol()), string(quote.Asset()),
		quote.CanonicalSnapshotJSON(), quote.SnapshotSHA256(), quote.MaximumAuthorizedCost().String(),
		quote.AcceptedAt(), quote.ExpiresAt(), int64(quote.SourceEpoch())).Scan(&persistedSHA)
	if err != nil {
		return fmt.Errorf("persist accepted quote: %w", err)
	}
	if persistedSHA != quote.SnapshotSHA256() {
		return ErrDecimalBillingConflict
	}

	var reservationID string
	err = tx.QueryRowContext(ctx, `WITH inserted AS (
		INSERT INTO bizdecipher_decimal_holds
			(reservation_id, business_event_id, request_id, quote_sha256, amount, asset, original_expires_at,
			 source_epoch, accepted_media_task_id, state, lease_owner, lease_epoch, version, updated_at)
		VALUES ($1, $2, $3, $4, CAST($5 AS NUMERIC(24,12)), $6, $7, $8, NULLIF($9, ''), $10, $11, $12, $13, $14)
		ON CONFLICT DO NOTHING RETURNING reservation_id
	)
	SELECT reservation_id FROM inserted
	UNION ALL
	SELECT reservation_id FROM bizdecipher_decimal_holds
	 WHERE reservation_id = $1 AND business_event_id = $2 AND request_id = $3 AND quote_sha256 = $4
	   AND amount = CAST($5 AS NUMERIC(24,12)) AND asset = $6 AND original_expires_at = $7
	   AND source_epoch = $8 AND COALESCE(accepted_media_task_id, '') = $9 AND state = $10
	   AND lease_owner = $11 AND lease_epoch = $12 AND version = $13 AND updated_at = $14
	LIMIT 1`, hold.ReservationID(), hold.BusinessEventID(), hold.RequestID(), hold.QuoteSHA256(), hold.Amount().String(),
		string(hold.Asset()), hold.OriginalExpiresAt(), int64(hold.SourceEpoch()), hold.AcceptedMediaTaskID(),
		string(hold.State()), hold.LeaseOwner(), int64(hold.LeaseEpoch()), int64(hold.Version()), hold.UpdatedAt()).Scan(&reservationID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDecimalBillingConflict
	}
	if err != nil {
		return fmt.Errorf("persist decimal hold: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit decimal reservation: %w", err)
	}
	return nil
}

func (r *bizDecipherRepository) GetDecimalHold(ctx context.Context, reservationID string) (billingcontract.Hold, error) {
	return scanDecimalHold(r.db.QueryRowContext(ctx, selectDecimalHold, reservationID))
}

func (r *bizDecipherRepository) TransitionDecimalHold(
	ctx context.Context,
	reservationID string,
	command billingcontract.TransitionCommand,
) (billingcontract.Decision, error) {
	return r.mutateDecimalHold(ctx, reservationID, func(current billingcontract.Hold) (billingcontract.Decision, error) {
		return billingcontract.DecideTransition(current, command)
	}, func(tx *sql.Tx, decision billingcontract.Decision) (sql.Result, error) {
		next := decision.Hold()
		return tx.ExecContext(ctx, `UPDATE bizdecipher_decimal_holds
			SET state = $2, lease_owner = $3, lease_epoch = $4, version = $5, updated_at = $6
			WHERE reservation_id = $1 AND version = $7 AND lease_epoch = $8`,
			reservationID, string(next.State()), next.LeaseOwner(), int64(next.LeaseEpoch()), int64(next.Version()),
			next.UpdatedAt(), int64(command.ExpectedVersion), int64(command.ActiveEpoch))
	})
}

func (r *bizDecipherRepository) RetainDecimalMediaTask(
	ctx context.Context,
	reservationID string,
	command billingcontract.MediaTaskCommand,
) (billingcontract.Decision, error) {
	return r.mutateDecimalHold(ctx, reservationID, func(current billingcontract.Hold) (billingcontract.Decision, error) {
		return billingcontract.RetainAcceptedMediaTask(current, command)
	}, func(tx *sql.Tx, decision billingcontract.Decision) (sql.Result, error) {
		next := decision.Hold()
		return tx.ExecContext(ctx, `UPDATE bizdecipher_decimal_holds
			SET accepted_media_task_id = $2, version = $3, updated_at = $6
			WHERE reservation_id = $1 AND version = $4 AND lease_epoch = $5 AND accepted_media_task_id IS NULL`,
			reservationID, next.AcceptedMediaTaskID(), int64(next.Version()), int64(command.ExpectedVersion),
			int64(command.ActiveEpoch), next.UpdatedAt())
	})
}

type decimalHoldDecision func(billingcontract.Hold) (billingcontract.Decision, error)
type decimalHoldUpdate func(*sql.Tx, billingcontract.Decision) (sql.Result, error)

func (r *bizDecipherRepository) mutateDecimalHold(
	ctx context.Context,
	reservationID string,
	decide decimalHoldDecision,
	update decimalHoldUpdate,
) (_ billingcontract.Decision, retErr error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return billingcontract.Decision{}, fmt.Errorf("begin decimal hold mutation: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("rollback decimal hold mutation: %w", err))
		}
	}()
	current, err := scanDecimalHold(tx.QueryRowContext(ctx, selectDecimalHoldForUpdate, reservationID))
	if err != nil {
		return billingcontract.Decision{}, err
	}
	decision, err := decide(current)
	if err != nil {
		return billingcontract.Decision{}, err
	}
	if decision.Applied() {
		result, err := update(tx, decision)
		if err != nil {
			return billingcontract.Decision{}, fmt.Errorf("update decimal hold: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return billingcontract.Decision{}, fmt.Errorf("read decimal hold update count: %w", err)
		}
		if rows != 1 {
			return billingcontract.Decision{}, billingcontract.ErrStaleVersion
		}
	}
	if err := tx.Commit(); err != nil {
		return billingcontract.Decision{}, fmt.Errorf("commit decimal hold mutation: %w", err)
	}
	return decision, nil
}

type decimalHoldScanner interface {
	Scan(...any) error
}

func scanDecimalHold(row decimalHoldScanner) (billingcontract.Hold, error) {
	var reservationID, businessEventID, requestID, quoteSHA, amountText, assetText string
	var mediaTaskID, stateText, leaseOwner string
	var sourceEpoch, leaseEpoch, version int64
	var originalExpiresAt, updatedAt time.Time
	if err := row.Scan(&reservationID, &businessEventID, &requestID, &quoteSHA, &amountText, &assetText,
		&originalExpiresAt, &sourceEpoch, &mediaTaskID, &stateText, &leaseOwner, &leaseEpoch, &version, &updatedAt); err != nil {
		return billingcontract.Hold{}, fmt.Errorf("scan decimal hold: %w", err)
	}
	amount, err := billingcontract.ParseDecimal(amountText)
	if err != nil {
		return billingcontract.Hold{}, fmt.Errorf("restore decimal amount: %w", err)
	}
	hold, err := billingcontract.RestoreHold(billingcontract.RestoreHoldInput{
		ReservationID: reservationID, BusinessEventID: businessEventID, RequestID: requestID, QuoteSHA256: quoteSHA,
		Amount: amount, Asset: billingcontract.Asset(assetText), OriginalExpiresAt: originalExpiresAt,
		SourceEpoch: uint64(sourceEpoch), AcceptedMediaTaskID: mediaTaskID, State: billingcontract.HoldState(stateText),
		LeaseOwner: leaseOwner, LeaseEpoch: uint64(leaseEpoch), Version: uint64(version), UpdatedAt: updatedAt,
	})
	if err != nil {
		return billingcontract.Hold{}, fmt.Errorf("restore decimal hold: %w", err)
	}
	return hold, nil
}
