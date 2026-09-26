package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const sharedPoolMediaTaskColumns = `
	t.id, t.reservation_id, t.access_key_id, t.pool_id, t.account_id,
	t.user_id, t.price_version_id, t.endpoint_type, t.provider,
	t.model_snapshot, t.upstream_model_snapshot, t.upstream_request_id, t.reservation_request_id,
	t.requested_resolution, t.requested_duration_seconds, t.status,
	t.last_upstream_status, t.last_http_status, r.pricing_source_snapshot,
	r.price_snapshot, r.status, t.expires_at, t.completed_at, t.created_at, t.updated_at`

func scanSharedPoolMediaTask(s scanner) (*service.SharedPoolMediaTask, error) {
	var task service.SharedPoolMediaTask
	var accountID sql.NullInt64
	var httpStatus sql.NullInt64
	var completedAt sql.NullTime
	if err := s.Scan(
		&task.ID, &task.ReservationID, &task.AccessKeyID, &task.PoolID, &accountID,
		&task.UserID, &task.PriceVersionID, &task.EndpointType, &task.Provider,
		&task.ModelSnapshot, &task.UpstreamModelSnapshot, &task.UpstreamRequestID, &task.ReservationRequestID,
		&task.RequestedResolution, &task.RequestedDurationSeconds, &task.Status,
		&task.LastUpstreamStatus, &httpStatus, &task.PricingSource,
		&task.PriceSnapshot, &task.ReservationStatus, &task.ExpiresAt,
		&completedAt, &task.CreatedAt, &task.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if accountID.Valid {
		task.AccountID = accountID.Int64
	}
	if httpStatus.Valid {
		value := int(httpStatus.Int64)
		task.LastHTTPStatus = &value
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	return &task, nil
}

func (r *bizDecipherRepository) CreateSharedPoolMediaTask(ctx context.Context, input service.CreateSharedPoolMediaTaskInput) (*service.SharedPoolMediaTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool media task repository is unavailable")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO shared_pool_media_tasks (
			reservation_id, access_key_id, pool_id, account_id, user_id,
			price_version_id, endpoint_type, provider, model_snapshot, upstream_model_snapshot,
			upstream_request_id, reservation_request_id, requested_resolution,
			requested_duration_seconds, expires_at
		)
		SELECT r.id, r.access_key_id, r.pool_id, r.account_id, r.user_id,
		       r.price_version_id, 'video', $7, $8, $9, $10, r.request_id, $11, $12, $13
		FROM shared_pool_usage_reservations r
		WHERE r.id = $1 AND r.access_key_id = $2 AND r.pool_id = $3
		  AND COALESCE(r.account_id, 0) = $4 AND r.user_id = $5
		  AND r.price_version_id = $6 AND r.request_id = $14
		  AND r.endpoint_type = 'video' AND r.status = 'forwarding'
		ON CONFLICT DO NOTHING
		RETURNING id`,
		input.ReservationID, input.AccessKeyID, input.PoolID, input.AccountID,
		input.UserID, input.PriceVersionID, input.Provider, input.ModelSnapshot,
		input.UpstreamModelSnapshot, input.UpstreamRequestID, input.RequestedResolution,
		input.RequestedDurationSeconds, input.ExpiresAt, input.ReservationRequestID,
	)
	var taskID int64
	err := row.Scan(&taskID)
	if err == nil {
		return r.getSharedPoolMediaTaskByID(ctx, taskID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Idempotent replay is accepted only when every immutable owner/billing
	// field matches. A provider id already owned by another reservation never
	// crosses this boundary.
	existing, existingErr := r.getSharedPoolMediaTaskByReservationID(ctx, input.ReservationID)
	if existingErr != nil {
		if errors.Is(existingErr, sql.ErrNoRows) {
			return nil, service.ErrSharedPoolMediaTaskConflict
		}
		return nil, existingErr
	}
	if existing.AccessKeyID != input.AccessKeyID || existing.PoolID != input.PoolID ||
		existing.AccountID != input.AccountID || existing.UserID != input.UserID ||
		existing.PriceVersionID != input.PriceVersionID ||
		!strings.EqualFold(existing.Provider, input.Provider) ||
		existing.ModelSnapshot != input.ModelSnapshot ||
		existing.UpstreamModelSnapshot != input.UpstreamModelSnapshot ||
		existing.UpstreamRequestID != input.UpstreamRequestID ||
		existing.ReservationRequestID != input.ReservationRequestID ||
		existing.RequestedResolution != input.RequestedResolution ||
		existing.RequestedDurationSeconds != input.RequestedDurationSeconds {
		return nil, service.ErrSharedPoolMediaTaskConflict
	}
	return existing, nil
}

func (r *bizDecipherRepository) getSharedPoolMediaTaskByID(ctx context.Context, taskID int64) (*service.SharedPoolMediaTask, error) {
	return scanSharedPoolMediaTask(r.db.QueryRowContext(ctx, `SELECT `+sharedPoolMediaTaskColumns+`
		FROM shared_pool_media_tasks t
		JOIN shared_pool_usage_reservations r ON r.id = t.reservation_id
		WHERE t.id = $1`, taskID))
}

func (r *bizDecipherRepository) getSharedPoolMediaTaskByReservationID(ctx context.Context, reservationID int64) (*service.SharedPoolMediaTask, error) {
	return scanSharedPoolMediaTask(r.db.QueryRowContext(ctx, `SELECT `+sharedPoolMediaTaskColumns+`
		FROM shared_pool_media_tasks t
		JOIN shared_pool_usage_reservations r ON r.id = t.reservation_id
		WHERE t.reservation_id = $1`, reservationID))
}

func (r *bizDecipherRepository) GetSharedPoolMediaTaskByAPIKeyID(ctx context.Context, apiKeyID int64, upstreamRequestID string) (*service.SharedPoolMediaTask, error) {
	if r == nil || r.db == nil || apiKeyID <= 0 || strings.TrimSpace(upstreamRequestID) == "" {
		return nil, service.ErrSharedPoolMediaTaskNotFound
	}
	task, err := scanSharedPoolMediaTask(r.db.QueryRowContext(ctx, `SELECT `+sharedPoolMediaTaskColumns+`
		FROM shared_pool_media_tasks t
		JOIN shared_pool_usage_reservations r ON r.id = t.reservation_id
		JOIN shared_pool_access_keys sak
		  ON sak.id = t.access_key_id AND sak.pool_id = t.pool_id AND sak.user_id = t.user_id
		WHERE sak.api_key_id = $1 AND t.upstream_request_id = $2`, apiKeyID, strings.TrimSpace(upstreamRequestID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSharedPoolMediaTaskNotFound
	}
	return task, err
}

// GetSharedPoolAccessKeyForMediaTask reconstructs the exact submitted route
// from current encrypted pool/account configuration. The task stores only ids;
// credentials never enter shared_pool_media_tasks. Pool-level routes keep a
// NULL account_id while account-mode routes are pinned to their original row.
func (r *bizDecipherRepository) GetSharedPoolAccessKeyForMediaTask(ctx context.Context, apiKeyID, taskID int64) (*service.SharedPoolAccessKey, error) {
	if r == nil || r.db == nil || apiKeyID <= 0 || taskID <= 0 {
		return nil, service.ErrSharedPoolMediaTaskNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT sak.id, sp.id, sp.name, sak.user_id, sak.api_key_id, sak.name,
		       ak.key, sak.status, sak.allowed_models, sak.total_used,
		       sak.last_used_at, sak.created_at, sp.owner_id,
		       COALESCE(spa.id, 0) AS shared_pool_account_id,
		       COALESCE(NULLIF(spa.auth_type, ''), 'apikey') AS auth_type,
		       CASE WHEN spa.id IS NULL THEN sp.upstream_base_url ELSE COALESCE(spa.upstream_base_url, '') END,
		       CASE WHEN spa.id IS NULL THEN sp.upstream_api_key WHEN spa.auth_type = 'oauth' THEN '' ELSE COALESCE(spa.upstream_api_key, '') END,
		       COALESCE(spa.credentials_encrypted, ''), spa.expires_at,
		       COALESCE(NULLIF(spa.proxy_url, ''), sp.proxy_url),
		       sp.rate_multiplier, sp.owner_share_percent,
		       COALESCE(NULLIF(spa.account_concurrency, 0), sp.account_concurrency),
		       COALESCE(NULLIF(spa.user_concurrency, 0), sp.user_concurrency),
		       COALESCE(NULLIF(spm.max_concurrency, 0), 0),
		       sak.account_mode,
		       COALESCE(NULLIF(spm.rate_multiplier, 0), sp.rate_multiplier),
		       t.model_snapshot,
		       t.upstream_model_snapshot,
		       COALESCE((
		           SELECT mc.model_name FROM model_catalog mc
		           WHERE mc.enabled = TRUE AND LOWER(mc.provider) = LOWER(t.provider)
		             AND (LOWER(mc.model_name) IN (LOWER(t.model_snapshot), LOWER(t.upstream_model_snapshot)) OR EXISTS (
		                 SELECT 1 FROM jsonb_array_elements_text(
		                   CASE WHEN jsonb_typeof(mc.aliases) = 'array' THEN mc.aliases ELSE '[]'::jsonb END
		                 ) alias(value) WHERE LOWER(alias.value) IN (LOWER(t.model_snapshot), LOWER(t.upstream_model_snapshot))))
		           ORDER BY CASE WHEN LOWER(mc.model_name) = LOWER(t.upstream_model_snapshot) THEN 0
		                         WHEN LOWER(mc.model_name) = LOWER(t.model_snapshot) THEN 1 ELSE 2 END,
		                    mc.sort_order, mc.id LIMIT 1
		       ), ''), t.provider
		FROM shared_pool_media_tasks t
		JOIN shared_pool_access_keys sak
		  ON sak.id = t.access_key_id AND sak.pool_id = t.pool_id AND sak.user_id = t.user_id
		JOIN api_keys ak ON ak.id = sak.api_key_id
		JOIN shared_pools sp ON sp.id = t.pool_id
		JOIN shared_pool_price_versions pv
		  ON pv.id = t.price_version_id AND pv.pool_id = t.pool_id
		JOIN shared_pool_model_endpoints spe
		  ON spe.id = pv.endpoint_id AND spe.pool_model_id = pv.pool_model_id
		 AND spe.endpoint_type = t.endpoint_type
		JOIN shared_pool_models spm
		  ON spm.id = pv.pool_model_id AND spm.pool_id = t.pool_id
		LEFT JOIN shared_pool_accounts spa ON spa.id = t.account_id AND spa.pool_id = t.pool_id AND spa.deleted_at IS NULL
		WHERE t.id = $1 AND sak.api_key_id = $2
		  AND ((t.account_id IS NULL AND btrim(sp.upstream_base_url) <> '' AND btrim(sp.upstream_api_key) <> '')
		       OR (t.account_id IS NOT NULL AND spa.id IS NOT NULL AND
		           ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '')
		             OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))))`, taskID, apiKeyID)
	accessKey, err := scanSharedPoolAccessKeyRuntime(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSharedPoolMediaTaskNotFound
	}
	return accessKey, err
}

func (r *bizDecipherRepository) RecoverExpiredSharedPoolMediaTasksTx(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("shared pool media task repository is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `WITH expired AS (
		SELECT t.id, t.reservation_id
		FROM shared_pool_media_tasks t
		JOIN shared_pool_usage_reservations reservation ON reservation.id = t.reservation_id
		WHERE t.status IN ('submitted', 'processing')
		  AND t.expires_at <= $1
		  AND reservation.status IN ('forwarding', 'review_required')
		ORDER BY t.expires_at, t.id
		LIMIT $2
		FOR UPDATE OF t, reservation SKIP LOCKED
	), reservation_review AS (
		UPDATE shared_pool_usage_reservations reservation
		SET status = 'review_required',
		    failure_reason = CASE WHEN btrim(reservation.failure_reason) = ''
		                          THEN 'video_task_expired_without_terminal_result'
		                          ELSE reservation.failure_reason END,
		    finalized_at = COALESCE(reservation.finalized_at, $1),
		    updated_at = NOW()
		FROM expired
		WHERE reservation.id = expired.reservation_id
		  AND reservation.status = 'forwarding'
		RETURNING reservation.id
	)
	UPDATE shared_pool_media_tasks task
	SET status = 'review_required',
	    last_upstream_status = CASE WHEN btrim(task.last_upstream_status) = ''
		                                THEN 'expired_without_terminal_result'
		                                ELSE task.last_upstream_status END,
	    completed_at = COALESCE(task.completed_at, $1),
	    updated_at = NOW()
	FROM expired
	WHERE task.id = expired.id
	RETURNING task.id`, now, limit)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var taskID int64
		if err := rows.Scan(&taskID); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func (r *bizDecipherRepository) UpdateSharedPoolMediaTaskStateTx(ctx context.Context, input service.UpdateSharedPoolMediaTaskStateInput) (_ *service.SharedPoolMediaTask, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool media task repository is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var taskStatus, reservationRequestID string
	var reservationID int64
	err = tx.QueryRowContext(ctx, `
		SELECT status, reservation_id, reservation_request_id
		FROM shared_pool_media_tasks
		WHERE id = $1 AND access_key_id = $2 AND upstream_request_id = $3
		FOR UPDATE`, input.TaskID, input.AccessKeyID, input.UpstreamRequestID).Scan(
		&taskStatus, &reservationID, &reservationRequestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSharedPoolMediaTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	if isFinalSharedPoolMediaTaskStatus(taskStatus) {
		if taskStatus != input.Status && input.Status != service.SharedPoolMediaTaskSubmitted && input.Status != service.SharedPoolMediaTaskProcessing {
			return nil, service.ErrSharedPoolMediaTaskConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.getSharedPoolMediaTaskByID(ctx, input.TaskID)
	}

	reservation, err := selectSharedPoolUsageReservationForUpdate(ctx, tx, input.AccessKeyID, reservationRequestID)
	if err != nil {
		return nil, err
	}
	if reservation.ID != reservationID {
		return nil, service.ErrSharedPoolMediaTaskConflict
	}
	switch input.Status {
	case service.SharedPoolMediaTaskSucceeded:
		if reservation.Status != "settled" {
			return nil, service.ErrSharedPoolMediaTaskConflict
		}
	case service.SharedPoolMediaTaskFailed:
		if err := releaseForwardedSharedPoolUsageReservationAfterVerifiedFailureTx(ctx, tx, reservation, input.Reason); err != nil {
			return nil, err
		}
	case service.SharedPoolMediaTaskReviewRequired:
		if reservation.Status != "review_required" {
			if reservation.Status != "forwarding" {
				return nil, service.ErrSharedPoolMediaTaskConflict
			}
			result, updateErr := tx.ExecContext(ctx, `UPDATE shared_pool_usage_reservations
				SET status = 'review_required', failure_reason = $2,
				    finalized_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND status = 'forwarding'`, reservation.ID, normalizedSharedPoolMediaFailureReason(input.Reason, "media_task_result_unknown"))
			if updateErr != nil {
				return nil, updateErr
			}
			if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
				if affectedErr != nil {
					return nil, affectedErr
				}
				return nil, service.ErrSharedPoolMediaTaskConflict
			}
		}
	default:
		if reservation.Status != "forwarding" {
			return nil, service.ErrSharedPoolMediaTaskConflict
		}
	}

	terminal := isFinalSharedPoolMediaTaskStatus(input.Status)
	_, err = tx.ExecContext(ctx, `UPDATE shared_pool_media_tasks
		SET status = $2, last_upstream_status = $3, last_http_status = $4,
		    last_polled_at = NOW(), completed_at = CASE WHEN $5 THEN COALESCE(completed_at, NOW()) ELSE completed_at END,
		    updated_at = NOW()
		WHERE id = $1`, input.TaskID, input.Status, input.LastUpstreamStatus, input.LastHTTPStatus, terminal)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getSharedPoolMediaTaskByID(ctx, input.TaskID)
}

func isFinalSharedPoolMediaTaskStatus(status string) bool {
	switch status {
	case service.SharedPoolMediaTaskSucceeded, service.SharedPoolMediaTaskFailed, service.SharedPoolMediaTaskReviewRequired:
		return true
	default:
		return false
	}
}

func normalizedSharedPoolMediaFailureReason(reason, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = fallback
	}
	if len(reason) > 160 {
		reason = reason[:160]
	}
	return reason
}

// A provider-declared terminal failure is stronger evidence than a transport
// error. Callers must have a terminal provider result; timeouts and broken
// streams continue to enter review instead of using this release path.
func releaseForwardedSharedPoolUsageReservationAfterVerifiedFailureTx(ctx context.Context, tx *sql.Tx, reservation *lockedSharedPoolUsageReservation, reason string) error {
	if reservation == nil {
		return service.ErrSharedPoolReservationFinalized
	}
	if reservation.Status == "released" {
		return nil
	}
	if reservation.Status != "forwarding" {
		return service.ErrSharedPoolReservationFinalized
	}
	if reservation.HoldAmount > 0 {
		result, err := tx.ExecContext(ctx, `UPDATE users
			SET balance = balance + $1,
			    frozen_balance = COALESCE(frozen_balance, 0) - $1,
			    updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL
			  AND COALESCE(frozen_balance, 0) >= $1`, reservation.HoldAmount, reservation.UserID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return errors.New("shared pool frozen balance is insufficient for verified failure release")
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_usage_reservations
		SET status = 'released', reported_amount = 0, settled_amount = 0,
		    failure_reason = $2, finalized_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'forwarding'`, reservation.ID,
		normalizedSharedPoolMediaFailureReason(reason, "verified_media_task_failed"))
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
