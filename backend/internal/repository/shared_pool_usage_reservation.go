package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type lockedSharedPoolUsageReservation struct {
	ID                 int64
	RequestID          string
	RequestFingerprint string
	AccessKeyID        int64
	PoolID             int64
	AccountID          sql.NullInt64
	UserID             int64
	PriceVersionID     sql.NullInt64
	EndpointType       string
	Model              string
	PricingSource      string
	PriceSnapshot      []byte
	HoldAmount         float64
	ReportedAmount     float64
	SettledAmount      float64
	Status             string
	ReservedAt         time.Time
	ExpiresAt          time.Time
}

func (r *bizDecipherRepository) ReserveSharedPoolUsageTx(
	ctx context.Context,
	input service.SharedPoolUsageReservationInput,
) (_ *service.SharedPoolUsageReservation, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool reservation repository is unavailable")
	}
	priceSnapshot, err := sharedPoolJSONBText(input.PriceSnapshot, "shared pool reservation price snapshot")
	if err != nil {
		return nil, err
	}
	input.PriceSnapshot = json.RawMessage(priceSnapshot)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := selectSharedPoolUsageReservationForUpdate(ctx, tx, input.AccessKeyID, input.RequestID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		if err := validateSharedPoolReservationReplay(existing, input); err != nil {
			return nil, err
		}
		return nil, service.ErrSharedPoolReservationFinalized
	}

	var created lockedSharedPoolUsageReservation
	err = tx.QueryRowContext(ctx, `
		INSERT INTO shared_pool_usage_reservations (
			request_id, request_fingerprint, access_key_id, pool_id, account_id,
			user_id, price_version_id, endpoint_type, model_snapshot,
			pricing_source_snapshot, price_snapshot, hold_amount, expires_at
		) VALUES (
			$1, $2, $3, $4, NULLIF($5, 0),
			$6, $7, $8, $9,
			$10, $11::jsonb, $12, $13
		)
		RETURNING id, request_id, request_fingerprint, access_key_id, pool_id,
		          account_id, user_id, price_version_id, hold_amount,
		          reported_amount, settled_amount, status, reserved_at, expires_at`,
		input.RequestID, input.RequestFingerprint, input.AccessKeyID, input.PoolID, input.AccountID,
		input.UserID, input.PriceVersionID, input.EndpointType, input.Model,
		input.PricingSource, priceSnapshot, input.HoldAmount, input.ExpiresAt,
	).Scan(
		&created.ID, &created.RequestID, &created.RequestFingerprint, &created.AccessKeyID,
		&created.PoolID, &created.AccountID, &created.UserID, &created.PriceVersionID,
		&created.HoldAmount, &created.ReportedAmount, &created.SettledAmount,
		&created.Status, &created.ReservedAt, &created.ExpiresAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrSharedPoolReservationConflict
		}
		return nil, err
	}

	if input.HoldAmount > 0 {
		var balance, frozenBalance float64
		err = tx.QueryRowContext(ctx, `
			UPDATE users
			SET balance = balance - $1,
			    frozen_balance = COALESCE(frozen_balance, 0) + $1,
			    updated_at = NOW()
			WHERE id = $2
			  AND deleted_at IS NULL
			  AND balance >= $1
			RETURNING balance, frozen_balance`,
			input.HoldAmount, input.UserID,
		).Scan(&balance, &frozenBalance)
		if errors.Is(err, sql.ErrNoRows) {
			exists, existsErr := userExistsForBilling(ctx, tx, input.UserID)
			if existsErr != nil {
				return nil, existsErr
			}
			if !exists {
				return nil, service.ErrUserNotFound
			}
			return nil, service.ErrInsufficientBalance
		}
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return sharedPoolUsageReservationFromLocked(&created), nil
}

func (r *bizDecipherRepository) MarkSharedPoolUsageForwardingTx(ctx context.Context, accessKeyID int64, requestID string) error {
	if r == nil || r.db == nil || accessKeyID <= 0 || strings.TrimSpace(requestID) == "" {
		return errors.New("shared pool reservation identity is invalid")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE shared_pool_usage_reservations
		SET status = 'forwarding',
		    forward_started_at = COALESCE(forward_started_at, NOW()),
		    updated_at = NOW()
		WHERE access_key_id = $1
		  AND request_id = $2
		  AND status = 'reserved'`, accessKeyID, strings.TrimSpace(requestID))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrSharedPoolReservationFinalized
	}
	return nil
}

func (r *bizDecipherRepository) MarkSharedPoolUsageReviewRequiredTx(ctx context.Context, accessKeyID int64, requestID, reason string) error {
	if r == nil || r.db == nil || accessKeyID <= 0 || strings.TrimSpace(requestID) == "" {
		return errors.New("shared pool reservation identity is invalid")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "upstream_result_unknown"
	}
	if len(reason) > 160 {
		reason = reason[:160]
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE shared_pool_usage_reservations
		SET status = 'review_required',
		    failure_reason = $3,
		    finalized_at = NOW(),
		    updated_at = NOW()
		WHERE access_key_id = $1
		  AND request_id = $2
		  AND status = 'forwarding'`, accessKeyID, strings.TrimSpace(requestID), reason)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var status string
	if err := r.db.QueryRowContext(ctx, `SELECT status FROM shared_pool_usage_reservations WHERE access_key_id = $1 AND request_id = $2`, accessKeyID, strings.TrimSpace(requestID)).Scan(&status); err != nil {
		return err
	}
	if status == "review_required" {
		return nil
	}
	return service.ErrSharedPoolReservationFinalized
}

func (r *bizDecipherRepository) ReleaseSharedPoolUsageAfterVerifiedFailureTx(
	ctx context.Context,
	accessKeyID int64,
	requestID string,
	reason string,
) (_ error) {
	if r == nil || r.db == nil || accessKeyID <= 0 || strings.TrimSpace(requestID) == "" {
		return errors.New("shared pool reservation identity is invalid")
	}
	reason = strings.TrimSpace(reason)
	if !strings.HasPrefix(reason, "upstream_http_") {
		return errors.New("shared pool verified failure reason is invalid")
	}
	if len(reason) > 160 {
		reason = reason[:160]
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	reservation, err := selectSharedPoolUsageReservationForUpdate(ctx, tx, accessKeyID, requestID)
	if err != nil {
		return err
	}
	if reservation.Status == "released" {
		return tx.Commit()
	}
	if err := releaseForwardedSharedPoolUsageReservationAfterVerifiedFailureTx(ctx, tx, reservation, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *bizDecipherRepository) StageSharedPoolUsageSettlementTx(ctx context.Context, input service.SharedPoolUsageInput) error {
	if r == nil || r.db == nil || input.AccessKeyID <= 0 || strings.TrimSpace(input.RequestID) == "" {
		return errors.New("shared pool settlement identity is invalid")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal shared pool settlement payload: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE shared_pool_usage_reservations
		SET status = 'settlement_pending',
		    reported_amount = $3,
		    usage_payload = CASE WHEN status = 'forwarding' THEN $4::jsonb ELSE usage_payload END,
		    settlement_staged_at = COALESCE(settlement_staged_at, NOW()),
		    updated_at = NOW()
		WHERE access_key_id = $1
		  AND request_id = $2
		  AND (
		      status = 'forwarding'
		      OR (status = 'settlement_pending' AND usage_payload = $4::jsonb)
		  )`,
		input.AccessKeyID, strings.TrimSpace(input.RequestID), input.Cost, string(payload))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var status string
	err = r.db.QueryRowContext(ctx, `
		SELECT status FROM shared_pool_usage_reservations
		WHERE access_key_id = $1 AND request_id = $2`, input.AccessKeyID, strings.TrimSpace(input.RequestID)).Scan(&status)
	if err != nil {
		return err
	}
	if status == "settled" {
		return nil
	}
	if status == "settlement_pending" {
		return service.ErrSharedPoolReservationConflict
	}
	return service.ErrSharedPoolReservationFinalized
}

func (r *bizDecipherRepository) ListPendingSharedPoolUsageSettlements(ctx context.Context, limit int) ([]service.SharedPoolUsageInput, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool reservation repository is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT usage_payload
		FROM shared_pool_usage_reservations
		WHERE status = 'settlement_pending'
		  AND usage_payload <> '{}'::jsonb
		ORDER BY settlement_staged_at, id
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.SharedPoolUsageInput, 0, limit)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var input service.SharedPoolUsageInput
		if err := json.Unmarshal(payload, &input); err != nil {
			return nil, fmt.Errorf("decode shared pool settlement payload: %w", err)
		}
		out = append(out, input)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) RecoverExpiredSharedPoolUsageReservationsTx(
	ctx context.Context,
	now time.Time,
	limit int,
) (_ *service.SharedPoolUsageReservationRecoverySummary, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool reservation repository is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, hold_amount
		FROM shared_pool_usage_reservations
		WHERE status = 'reserved'
		  AND expires_at <= $1
		ORDER BY expires_at, id
		LIMIT $2
		FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, err
	}
	type expiredReservation struct {
		ID         int64
		UserID     int64
		HoldAmount float64
	}
	expired := make([]expiredReservation, 0, limit)
	for rows.Next() {
		var item expiredReservation
		if err := rows.Scan(&item.ID, &item.UserID, &item.HoldAmount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	summary := &service.SharedPoolUsageReservationRecoverySummary{Scanned: len(expired)}
	for _, item := range expired {
		if item.HoldAmount > 0 {
			result, err := tx.ExecContext(ctx, `
				UPDATE users
				SET balance = balance + $1,
				    frozen_balance = COALESCE(frozen_balance, 0) - $1,
				    updated_at = NOW()
				WHERE id = $2
				  AND deleted_at IS NULL
				  AND COALESCE(frozen_balance, 0) >= $1`,
				item.HoldAmount, item.UserID,
			)
			if err != nil {
				return nil, err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return nil, err
			}
			if affected != 1 {
				summary.Failed++
				if _, err := tx.ExecContext(ctx, `
					UPDATE shared_pool_usage_reservations
					SET failure_reason = 'recovery_frozen_balance_mismatch',
					    updated_at = NOW()
					WHERE id = $1 AND status = 'reserved'`, item.ID); err != nil {
					return nil, err
				}
				continue
			}
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE shared_pool_usage_reservations
			SET status = 'released',
			    settled_amount = 0,
			    failure_reason = 'recovered_after_timeout',
			    finalized_at = $2,
			    updated_at = NOW()
			WHERE id = $1 AND status = 'reserved'`, item.ID, now)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected == 1 {
			summary.Released++
		}
	}
	reviewResult, err := tx.ExecContext(ctx, `
		WITH expired_forwarding AS (
			SELECT id
			FROM shared_pool_usage_reservations
			WHERE status = 'forwarding'
			  AND expires_at <= $1
			ORDER BY expires_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE shared_pool_usage_reservations reservation
		SET status = 'review_required',
		    failure_reason = 'forward_result_missing_after_timeout',
		    finalized_at = $1,
		    updated_at = NOW()
		FROM expired_forwarding
		WHERE reservation.id = expired_forwarding.id`, now, limit)
	if err != nil {
		return nil, err
	}
	reviewCount, err := reviewResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	summary.ReviewRequired = int(reviewCount)
	summary.Scanned += int(reviewCount)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return summary, nil
}

func selectSharedPoolUsageReservationForUpdate(
	ctx context.Context,
	tx *sql.Tx,
	accessKeyID int64,
	requestID string,
) (*lockedSharedPoolUsageReservation, error) {
	var reservation lockedSharedPoolUsageReservation
	err := tx.QueryRowContext(ctx, `
		SELECT id, request_id, request_fingerprint, access_key_id, pool_id,
		       account_id, user_id, price_version_id, endpoint_type, model_snapshot,
		       pricing_source_snapshot, price_snapshot, hold_amount,
		       reported_amount, settled_amount, status, reserved_at, expires_at
		FROM shared_pool_usage_reservations
		WHERE access_key_id = $1 AND request_id = $2
		FOR UPDATE`, accessKeyID, strings.TrimSpace(requestID)).Scan(
		&reservation.ID, &reservation.RequestID, &reservation.RequestFingerprint,
		&reservation.AccessKeyID, &reservation.PoolID, &reservation.AccountID,
		&reservation.UserID, &reservation.PriceVersionID, &reservation.EndpointType,
		&reservation.Model, &reservation.PricingSource, &reservation.PriceSnapshot,
		&reservation.HoldAmount,
		&reservation.ReportedAmount, &reservation.SettledAmount,
		&reservation.Status, &reservation.ReservedAt, &reservation.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func validateSharedPoolReservationReplay(
	existing *lockedSharedPoolUsageReservation,
	input service.SharedPoolUsageReservationInput,
) error {
	if existing == nil {
		return service.ErrSharedPoolReservationConflict
	}
	expectedAccountID := int64(0)
	if existing.AccountID.Valid {
		expectedAccountID = existing.AccountID.Int64
	}
	if !strings.EqualFold(existing.RequestFingerprint, input.RequestFingerprint) ||
		existing.AccessKeyID != input.AccessKeyID ||
		existing.PoolID != input.PoolID ||
		expectedAccountID != input.AccountID ||
		existing.UserID != input.UserID ||
		!existing.PriceVersionID.Valid ||
		existing.PriceVersionID.Int64 != input.PriceVersionID ||
		strings.TrimSpace(existing.EndpointType) != strings.TrimSpace(input.EndpointType) ||
		strings.TrimSpace(existing.Model) != strings.TrimSpace(input.Model) ||
		strings.TrimSpace(existing.PricingSource) != strings.TrimSpace(input.PricingSource) ||
		!sharedPoolJSONBEqual(existing.PriceSnapshot, input.PriceSnapshot) ||
		math.Abs(existing.HoldAmount-input.HoldAmount) > 0.000000000001 {
		return service.ErrSharedPoolReservationConflict
	}
	return nil
}

func sharedPoolUsageReservationFromLocked(input *lockedSharedPoolUsageReservation) *service.SharedPoolUsageReservation {
	if input == nil {
		return nil
	}
	return &service.SharedPoolUsageReservation{
		ID: input.ID, RequestID: input.RequestID, AccessKeyID: input.AccessKeyID,
		PoolID: input.PoolID, AccountID: input.AccountID.Int64, UserID: input.UserID,
		HoldAmount: input.HoldAmount, SettledAmount: input.SettledAmount,
		Status: input.Status, ExpiresAt: input.ExpiresAt,
	}
}

func lockSharedPoolUsageReservationForSettlement(
	ctx context.Context,
	tx *sql.Tx,
	input service.SharedPoolUsageInput,
) (*lockedSharedPoolUsageReservation, error) {
	if strings.TrimSpace(input.RequestID) == "" {
		return nil, nil
	}
	reservation, err := selectSharedPoolUsageReservationForUpdate(ctx, tx, input.AccessKeyID, input.RequestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	expectedAccountID := int64(0)
	if reservation.AccountID.Valid {
		expectedAccountID = reservation.AccountID.Int64
	}
	expectedPriceVersionID := int64(0)
	if reservation.PriceVersionID.Valid {
		expectedPriceVersionID = reservation.PriceVersionID.Int64
	}
	inputModel := strings.TrimSpace(input.Model)
	if reservation.PoolID != input.PoolID ||
		reservation.UserID != input.UserID ||
		expectedAccountID != input.AccountID ||
		expectedPriceVersionID != input.PriceVersionID ||
		((input.Success || inputModel != "") && strings.TrimSpace(reservation.Model) != inputModel) ||
		strings.TrimSpace(reservation.PricingSource) != strings.TrimSpace(input.PricingSource) ||
		!sharedPoolJSONBEqual(reservation.PriceSnapshot, input.PriceSnapshot) {
		return nil, service.ErrSharedPoolReservationConflict
	}
	return reservation, nil
}

func captureSharedPoolUsageReservationTx(
	ctx context.Context,
	tx *sql.Tx,
	reservation *lockedSharedPoolUsageReservation,
	reportedAmount float64,
) (settledAmount float64, balanceAfter float64, capped bool, err error) {
	if reservation == nil {
		return 0, 0, false, errors.New("shared pool reservation is missing")
	}
	if reservation.Status != "forwarding" && reservation.Status != "settlement_pending" {
		return reservation.SettledAmount, 0, false, service.ErrSharedPoolReservationFinalized
	}
	if reportedAmount < 0 || math.IsNaN(reportedAmount) || math.IsInf(reportedAmount, 0) {
		return 0, 0, false, errors.New("shared pool reported amount is invalid")
	}
	settledAmount = reportedAmount
	if settledAmount > reservation.HoldAmount {
		settledAmount = reservation.HoldAmount
		capped = true
	}
	if settledAmount < 0.0000000000005 {
		settledAmount = 0
	}

	if reservation.HoldAmount > 0 {
		err = tx.QueryRowContext(ctx, `
			UPDATE users
			SET balance = balance + ($1 - $2),
			    frozen_balance = COALESCE(frozen_balance, 0) - $1,
			    updated_at = NOW()
			WHERE id = $3
			  AND deleted_at IS NULL
			  AND COALESCE(frozen_balance, 0) >= $1
			RETURNING balance`,
			reservation.HoldAmount, settledAmount, reservation.UserID,
		).Scan(&balanceAfter)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, false, errors.New("shared pool frozen balance is insufficient for settlement")
		}
		if err != nil {
			return 0, 0, false, err
		}
	} else {
		if err := tx.QueryRowContext(ctx, `
			SELECT balance FROM users
			WHERE id = $1 AND deleted_at IS NULL
			FOR UPDATE`, reservation.UserID).Scan(&balanceAfter); err != nil {
			return 0, 0, false, err
		}
	}

	failureReason := ""
	if capped {
		failureReason = "reported_cost_exceeded_hold"
		slog.Error("shared pool reported cost exceeded the pre-authorized hold; settlement was capped",
			"reservation_id", reservation.ID,
			"access_key_id", reservation.AccessKeyID,
			"pool_id", reservation.PoolID,
			"reported_amount", reportedAmount,
			"hold_amount", reservation.HoldAmount)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE shared_pool_usage_reservations
		SET status = 'settled',
		    reported_amount = $2,
		    settled_amount = $3,
		    failure_reason = $4,
		    finalized_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('forwarding', 'settlement_pending')`,
		reservation.ID, reportedAmount, settledAmount, failureReason,
	)
	if err != nil {
		return 0, 0, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, 0, false, err
	}
	if affected != 1 {
		return 0, 0, false, service.ErrSharedPoolReservationFinalized
	}
	return settledAmount, balanceAfter, capped, nil
}

func releaseSharedPoolUsageReservationTx(
	ctx context.Context,
	tx *sql.Tx,
	reservation *lockedSharedPoolUsageReservation,
	reason string,
) error {
	if reservation == nil {
		return nil
	}
	// Only a reservation that has never crossed the forwarding boundary may be
	// released automatically. Forwarding/unknown requests must enter review.
	if reservation.Status != "reserved" {
		return service.ErrSharedPoolReservationFinalized
	}
	if reservation.HoldAmount > 0 {
		result, err := tx.ExecContext(ctx, `
			UPDATE users
			SET balance = balance + $1,
			    frozen_balance = COALESCE(frozen_balance, 0) - $1,
			    updated_at = NOW()
			WHERE id = $2
			  AND deleted_at IS NULL
			  AND COALESCE(frozen_balance, 0) >= $1`,
			reservation.HoldAmount, reservation.UserID,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return errors.New("shared pool frozen balance is insufficient for release")
		}
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 160 {
		reason = reason[:160]
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE shared_pool_usage_reservations
		SET status = 'released',
		    reported_amount = 0,
		    settled_amount = 0,
		    failure_reason = $2,
		    finalized_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'reserved'`, reservation.ID, reason)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%w: reservation %d", service.ErrSharedPoolReservationFinalized, reservation.ID)
	}
	return nil
}
