package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sharedPoolUsageAutoReleaseCandidate struct {
	ID            int64
	UserID        int64
	HoldAmount    float64
	TriggerReason string
}

// AutoReleaseAgedSharedPoolUsageReviewsTx applies the conservative default for
// low-value ambiguous requests: after the configured grace period, return the
// user's hold instead of requiring an operator click or guessing a charge.
// Every balance and status mutation is committed in the same transaction.
func (r *bizDecipherRepository) AutoReleaseAgedSharedPoolUsageReviewsTx(
	ctx context.Context,
	cutoff time.Time,
	maxHoldAmount float64,
	limit int,
) (_ *service.SharedPoolUsageReviewAutoReleaseSummary, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool usage review repository is unavailable")
	}
	if cutoff.IsZero() || maxHoldAmount <= 0 || limit <= 0 {
		return &service.SharedPoolUsageReviewAutoReleaseSummary{}, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, hold_amount, failure_reason
		FROM shared_pool_usage_reservations
		WHERE status = 'review_required'
		  AND finalized_at IS NOT NULL
		  AND finalized_at <= $1
		  AND hold_amount <= $2
		  AND failure_reason IN (
		      'upstream_result_unknown',
		      'upstream_timeout_result_unknown',
		      'upstream_canceled_result_unknown',
		      'upstream_connection_lost_result_unknown',
		      'upstream_partial_response_result_unknown',
		      'forward_result_missing_after_timeout',
		      'media_task_result_unknown',
		      'video_task_expired_without_terminal_result'
		  )
		ORDER BY finalized_at, id
		LIMIT $3
		FOR UPDATE SKIP LOCKED`, cutoff, maxHoldAmount, limit)
	if err != nil {
		return nil, err
	}
	candidates := make([]sharedPoolUsageAutoReleaseCandidate, 0, limit)
	for rows.Next() {
		var candidate sharedPoolUsageAutoReleaseCandidate
		if err := rows.Scan(&candidate.ID, &candidate.UserID, &candidate.HoldAmount, &candidate.TriggerReason); err != nil {
			_ = rows.Close()
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	summary := &service.SharedPoolUsageReviewAutoReleaseSummary{Scanned: len(candidates)}
	for _, candidate := range candidates {
		if candidate.HoldAmount > 0 {
			result, updateErr := tx.ExecContext(ctx, `
				UPDATE users
				SET balance = balance + $1,
				    frozen_balance = COALESCE(frozen_balance, 0) - $1,
				    updated_at = NOW()
				WHERE id = $2
				  AND deleted_at IS NULL
				  AND COALESCE(frozen_balance, 0) >= $1`, candidate.HoldAmount, candidate.UserID)
			if updateErr != nil {
				return nil, updateErr
			}
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil {
				return nil, affectedErr
			}
			if affected != 1 {
				summary.Failed++
				blockedReason := sharedPoolAutoReleaseReason("auto_release_blocked_balance_mismatch", candidate.TriggerReason)
				if _, updateErr := tx.ExecContext(ctx, `
					UPDATE shared_pool_usage_reservations
					SET failure_reason = $2, updated_at = NOW()
					WHERE id = $1 AND status = 'review_required'`, candidate.ID, blockedReason); updateErr != nil {
					return nil, updateErr
				}
				continue
			}
		}

		releaseReason := sharedPoolAutoReleaseReason("auto_released_low_value_after_grace", candidate.TriggerReason)
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE shared_pool_usage_reservations
			SET status = 'released',
			    reported_amount = 0,
			    settled_amount = 0,
			    failure_reason = $2,
			    updated_at = NOW()
			WHERE id = $1 AND status = 'review_required'`, candidate.ID, releaseReason)
		if updateErr != nil {
			return nil, updateErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return nil, affectedErr
		}
		if affected != 1 {
			return nil, fmt.Errorf("%w: reservation %d", service.ErrSharedPoolReservationFinalized, candidate.ID)
		}
		if _, updateErr := tx.ExecContext(ctx, `
			UPDATE shared_pool_usage_traces trace
			SET settlement_outcome = 'released'
			FROM shared_pool_usage_reservations reservation
			WHERE reservation.id = $1
			  AND trace.access_key_id = reservation.access_key_id
			  AND trace.request_id = reservation.request_id`, candidate.ID); updateErr != nil {
			return nil, updateErr
		}
		if _, updateErr := tx.ExecContext(ctx, `
			UPDATE shared_pool_media_tasks
			SET status = 'failed',
			    last_upstream_status = 'auto_released_low_value_after_grace',
			    completed_at = COALESCE(completed_at, NOW()),
			    updated_at = NOW()
			WHERE reservation_id = $1 AND status = 'review_required'`, candidate.ID); updateErr != nil {
			return nil, updateErr
		}
		summary.Released++
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return summary, nil
}

func sharedPoolAutoReleaseReason(prefix, trigger string) string {
	reason := strings.TrimSpace(prefix) + ":" + strings.TrimSpace(trigger)
	if len(reason) > 160 {
		reason = reason[:160]
	}
	return reason
}

var _ interface {
	AutoReleaseAgedSharedPoolUsageReviewsTx(context.Context, time.Time, float64, int) (*service.SharedPoolUsageReviewAutoReleaseSummary, error)
} = (*bizDecipherRepository)(nil)
