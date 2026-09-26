package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) ListSharedPoolUsageReviews(
	ctx context.Context,
	state string,
	beforeID int64,
	limit int,
) (*service.SharedPoolUsageReviewPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool usage review repository is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			r.id, r.request_id, r.access_key_id, r.pool_id,
			COALESCE(p.name, ''), r.account_id, r.user_id,
			COALESCE(NULLIF(u.username, ''), NULLIF(u.email, ''), ''),
			r.model_snapshot, r.endpoint_type, r.pricing_source_snapshot,
			r.hold_amount, r.reported_amount, r.settled_amount, r.status,
			r.failure_reason, r.reserved_at, r.forward_started_at, r.expires_at, r.finalized_at,
			rr.id, rr.admin_user_id, rr.action, rr.resolution_amount,
			rr.note, rr.operation_id, rr.created_at
		FROM shared_pool_usage_reservations r
		LEFT JOIN shared_pool_usage_review_resolutions rr ON rr.reservation_id = r.id
		LEFT JOIN shared_pools p ON p.id = r.pool_id
		LEFT JOIN users u ON u.id = r.user_id
		WHERE (
			($1 = 'pending' AND r.status = 'review_required')
			OR ($1 = 'processing' AND r.status = 'settlement_pending' AND rr.id IS NOT NULL)
			OR ($1 = 'resolved' AND r.status IN ('settled', 'released')
			    AND (rr.id IS NOT NULL OR r.failure_reason LIKE 'auto_released_low_value_after_grace:%'))
			OR ($1 = 'all' AND (r.status = 'review_required' OR rr.id IS NOT NULL
			    OR r.failure_reason LIKE 'auto_released_low_value_after_grace:%'))
		)
		AND ($2 = 0 OR r.id < $2)
		ORDER BY r.id DESC
		LIMIT $3`, state, beforeID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.SharedPoolUsageReview, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanSharedPoolUsageReview(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextBeforeID := int64(0)
	if hasMore && len(items) > 0 {
		nextBeforeID = items[len(items)-1].ReservationID
	}
	summary, err := r.summarizePendingSharedPoolUsageReviews(ctx, 20)
	if err != nil {
		return nil, err
	}
	return &service.SharedPoolUsageReviewPage{Items: items, NextBeforeID: nextBeforeID, HasMore: hasMore, Summary: summary}, nil
}

func (r *bizDecipherRepository) summarizePendingSharedPoolUsageReviews(
	ctx context.Context,
	groupLimit int,
) (*service.SharedPoolUsageReviewQueueSummary, error) {
	summary := &service.SharedPoolUsageReviewQueueSummary{Groups: []service.SharedPoolUsageReviewGroupSummary{}}
	var oldest sql.NullTime
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(hold_amount), 0), MIN(reserved_at)
		FROM shared_pool_usage_reservations
		WHERE status = 'review_required'`).Scan(&summary.PendingCount, &summary.PendingHold, &oldest); err != nil {
		return nil, err
	}
	if oldest.Valid {
		value := oldest.Time
		summary.OldestReserved = &value
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.pool_id, COALESCE(p.name, ''), r.model_snapshot, r.failure_reason,
		       COUNT(*), COALESCE(SUM(r.hold_amount), 0), MIN(r.reserved_at)
		FROM shared_pool_usage_reservations r
		LEFT JOIN shared_pools p ON p.id = r.pool_id
		WHERE r.status = 'review_required'
		GROUP BY r.pool_id, p.name, r.model_snapshot, r.failure_reason
		ORDER BY COUNT(*) DESC, SUM(r.hold_amount) DESC, r.pool_id
		LIMIT $1`, groupLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item service.SharedPoolUsageReviewGroupSummary
		var groupOldest time.Time
		if err := rows.Scan(
			&item.PoolID, &item.PoolName, &item.Model, &item.TriggerReason,
			&item.PendingCount, &item.PendingHold, &groupOldest,
		); err != nil {
			return nil, err
		}
		item.OldestReserved = &groupOldest
		summary.Groups = append(summary.Groups, item)
	}
	return summary, rows.Err()
}

func (r *bizDecipherRepository) PrepareSharedPoolUsageReviewResolutionTx(
	ctx context.Context,
	input service.ResolveSharedPoolUsageReviewInput,
) (_ *service.PreparedSharedPoolUsageReviewResolution, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool usage review repository is unavailable")
	}
	if input.Action != service.SharedPoolUsageReviewActionRelease {
		return nil, fmt.Errorf("shared pool usage review action %q is retired", input.Action)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, _, err := selectSharedPoolUsageReviewResolutionByOperation(ctx, tx, input.OperationID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		if err := validateSharedPoolUsageReviewReplay(existing, input); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.PreparedSharedPoolUsageReviewResolution{Review: existing, Replay: true}, nil
	}

	review, _, _, err := selectSharedPoolUsageReviewForUpdate(ctx, tx, input.ReservationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSharedPoolUsageReviewNotFound
	}
	if err != nil {
		return nil, err
	}
	if review.ReservationStatus != "review_required" {
		return nil, service.ErrSharedPoolUsageReviewFinalized
	}

	var resolutionID int64
	var resolvedAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO shared_pool_usage_review_resolutions (
			reservation_id, admin_user_id, operation_id, action,
			resolution_amount, trigger_reason, note
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		input.ReservationID, input.AdminUserID, input.OperationID, input.Action,
		0.0, review.TriggerReason, input.Note,
	).Scan(&resolutionID, &resolvedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrSharedPoolUsageReviewConflict
		}
		return nil, err
	}

	review.ResolutionID = resolutionID
	review.ResolutionAdminID = input.AdminUserID
	review.ResolutionAction = input.Action
	review.ResolutionAmount = 0
	review.ResolutionNote = input.Note
	review.ResolutionOperation = input.OperationID
	review.ResolvedAt = &resolvedAt

	if err := releaseReviewedSharedPoolUsageReservation(ctx, tx, review); err != nil {
		return nil, err
	}
	review.ReservationStatus = "released"
	review.ReportedAmount = 0
	review.SettledAmount = 0

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.PreparedSharedPoolUsageReviewResolution{Review: review}, nil
}

func selectSharedPoolUsageReviewForUpdate(
	ctx context.Context,
	tx *sql.Tx,
	reservationID int64,
) (*service.SharedPoolUsageReview, int64, json.RawMessage, error) {
	var (
		review         service.SharedPoolUsageReview
		accountID      sql.NullInt64
		priceVersionID sql.NullInt64
		priceSnapshot  []byte
		forwardedAt    sql.NullTime
		finalizedAt    sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `
		SELECT
			r.id, r.request_id, r.access_key_id, r.pool_id,
			COALESCE(p.name, ''), r.account_id, r.user_id,
			COALESCE(NULLIF(u.username, ''), NULLIF(u.email, ''), ''),
			r.model_snapshot, r.endpoint_type, r.pricing_source_snapshot,
			r.hold_amount, r.reported_amount, r.settled_amount, r.status,
			r.failure_reason, r.reserved_at, r.forward_started_at, r.expires_at, r.finalized_at,
			r.price_version_id, r.price_snapshot
		FROM shared_pool_usage_reservations r
		LEFT JOIN shared_pools p ON p.id = r.pool_id
		LEFT JOIN users u ON u.id = r.user_id
		WHERE r.id = $1
		FOR UPDATE OF r`, reservationID).Scan(
		&review.ReservationID, &review.RequestID, &review.AccessKeyID, &review.PoolID,
		&review.PoolName, &accountID, &review.UserID, &review.UserLabel,
		&review.Model, &review.EndpointType, &review.PricingSource,
		&review.HoldAmount, &review.ReportedAmount, &review.SettledAmount, &review.ReservationStatus,
		&review.TriggerReason, &review.ReservedAt, &forwardedAt, &review.ExpiresAt, &finalizedAt,
		&priceVersionID, &priceSnapshot,
	)
	if err != nil {
		return nil, 0, nil, err
	}
	review.AccountID = accountID.Int64
	if forwardedAt.Valid {
		value := forwardedAt.Time
		review.ForwardStartedAt = &value
	}
	if finalizedAt.Valid {
		value := finalizedAt.Time
		review.FinalizedAt = &value
	}
	return &review, priceVersionID.Int64, json.RawMessage(priceSnapshot), nil
}

func selectSharedPoolUsageReviewResolutionByOperation(
	ctx context.Context,
	tx *sql.Tx,
	operationID string,
) (*service.SharedPoolUsageReview, *service.SharedPoolUsageInput, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT
			r.id, r.request_id, r.access_key_id, r.pool_id,
			COALESCE(p.name, ''), r.account_id, r.user_id,
			COALESCE(NULLIF(u.username, ''), NULLIF(u.email, ''), ''),
			r.model_snapshot, r.endpoint_type, r.pricing_source_snapshot,
			r.hold_amount, r.reported_amount, r.settled_amount, r.status,
			rr.trigger_reason, r.reserved_at, r.forward_started_at, r.expires_at, r.finalized_at,
			rr.id, rr.admin_user_id, rr.action, rr.resolution_amount,
			rr.note, rr.operation_id, rr.created_at, r.usage_payload
		FROM shared_pool_usage_review_resolutions rr
		JOIN shared_pool_usage_reservations r ON r.id = rr.reservation_id
		LEFT JOIN shared_pools p ON p.id = r.pool_id
		LEFT JOIN users u ON u.id = r.user_id
		WHERE rr.operation_id = $1
		FOR SHARE OF rr, r`, operationID)
	review, payload, err := scanSharedPoolUsageReviewWithPayload(row)
	if err != nil {
		return nil, nil, err
	}
	var usage *service.SharedPoolUsageInput
	if review.ReservationStatus == "settlement_pending" && len(payload) > 0 && string(payload) != "{}" {
		var decoded service.SharedPoolUsageInput
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, nil, fmt.Errorf("decode reviewed shared pool settlement: %w", err)
		}
		usage = &decoded
	}
	return review, usage, nil
}

func validateSharedPoolUsageReviewReplay(review *service.SharedPoolUsageReview, input service.ResolveSharedPoolUsageReviewInput) error {
	if review == nil || review.ReservationID != input.ReservationID || review.ResolutionAdminID != input.AdminUserID ||
		review.ResolutionAction != input.Action || review.ResolutionNote != input.Note || review.ResolutionOperation != input.OperationID {
		return service.ErrSharedPoolUsageReviewConflict
	}
	return nil
}

func releaseReviewedSharedPoolUsageReservation(ctx context.Context, tx *sql.Tx, review *service.SharedPoolUsageReview) error {
	if review == nil || review.ReservationStatus != "review_required" {
		return service.ErrSharedPoolUsageReviewFinalized
	}
	if review.HoldAmount > 0 {
		result, err := tx.ExecContext(ctx, `
			UPDATE users
			SET balance = balance + $1,
			    frozen_balance = COALESCE(frozen_balance, 0) - $1,
			    updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL
			  AND COALESCE(frozen_balance, 0) >= $1`, review.HoldAmount, review.UserID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return errors.New("shared pool frozen balance is insufficient for reviewed release")
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE shared_pool_usage_reservations
		SET status = 'released',
		    reported_amount = 0,
		    settled_amount = 0,
		    failure_reason = 'manual_review_released',
		    finalized_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'review_required'`, review.ReservationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrSharedPoolUsageReviewFinalized
	}
	return nil
}

func scanSharedPoolUsageReview(scanner interface{ Scan(...any) error }) (service.SharedPoolUsageReview, error) {
	review, _, err := scanSharedPoolUsageReviewInternal(scanner, false)
	if review == nil {
		return service.SharedPoolUsageReview{}, err
	}
	return *review, err
}

func scanSharedPoolUsageReviewWithPayload(scanner interface{ Scan(...any) error }) (*service.SharedPoolUsageReview, []byte, error) {
	return scanSharedPoolUsageReviewInternal(scanner, true)
}

func scanSharedPoolUsageReviewInternal(scanner interface{ Scan(...any) error }, withPayload bool) (*service.SharedPoolUsageReview, []byte, error) {
	var (
		item              service.SharedPoolUsageReview
		accountID         sql.NullInt64
		forwardedAt       sql.NullTime
		finalizedAt       sql.NullTime
		resolutionID      sql.NullInt64
		resolutionAdminID sql.NullInt64
		resolutionAction  sql.NullString
		resolutionAmount  sql.NullFloat64
		resolutionNote    sql.NullString
		resolutionOp      sql.NullString
		resolvedAt        sql.NullTime
		payload           []byte
	)
	dest := []any{
		&item.ReservationID, &item.RequestID, &item.AccessKeyID, &item.PoolID,
		&item.PoolName, &accountID, &item.UserID, &item.UserLabel,
		&item.Model, &item.EndpointType, &item.PricingSource,
		&item.HoldAmount, &item.ReportedAmount, &item.SettledAmount, &item.ReservationStatus,
		&item.TriggerReason, &item.ReservedAt, &forwardedAt, &item.ExpiresAt, &finalizedAt,
		&resolutionID, &resolutionAdminID, &resolutionAction, &resolutionAmount,
		&resolutionNote, &resolutionOp, &resolvedAt,
	}
	if withPayload {
		dest = append(dest, &payload)
	}
	if err := scanner.Scan(dest...); err != nil {
		return nil, nil, err
	}
	item.AccountID = accountID.Int64
	item.ResolutionID = resolutionID.Int64
	item.ResolutionAdminID = resolutionAdminID.Int64
	item.ResolutionAction = resolutionAction.String
	item.ResolutionAmount = resolutionAmount.Float64
	item.ResolutionNote = resolutionNote.String
	item.ResolutionOperation = resolutionOp.String
	if forwardedAt.Valid {
		value := forwardedAt.Time
		item.ForwardStartedAt = &value
	}
	if finalizedAt.Valid {
		value := finalizedAt.Time
		item.FinalizedAt = &value
	}
	if resolvedAt.Valid {
		value := resolvedAt.Time
		item.ResolvedAt = &value
	}
	return &item, payload, nil
}
