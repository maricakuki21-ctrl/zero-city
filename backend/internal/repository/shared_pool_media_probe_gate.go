package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) ApplySharedPoolMediaEndpointProbeResultTx(ctx context.Context, input service.SharedPoolMediaEndpointProbeResult) (err error) {
	if r == nil || r.db == nil {
		return errors.New("shared pool media probe repository is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	accountConfigVersion := int64(0)
	if input.AccountID > 0 {
		err = tx.QueryRowContext(ctx, `
			SELECT spa.config_version
			FROM shared_pool_accounts spa
			WHERE spa.id = $1 AND spa.pool_id = $2 AND spa.deleted_at IS NULL
			FOR SHARE`, input.AccountID, input.PoolID).Scan(&accountConfigVersion)
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrSharedPoolMediaProbeConflict
		}
		if err != nil {
			return err
		}
		if accountConfigVersion != input.AccountConfigVersion {
			return service.ErrSharedPoolMediaProbeConflict
		}
	}

	var endpointID, probePlanVersion, endpointConfigVersion, poolConfigVersion int64
	var lastProbeAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT spe.id, spe.probe_plan_version, spe.config_version,
		       sp.config_version, spe.last_media_probe_at
		FROM shared_pool_model_endpoints spe
		JOIN shared_pool_models spm ON spm.id = spe.pool_model_id
		JOIN shared_pools sp ON sp.id = spm.pool_id
		WHERE sp.id = $1
		  AND (LOWER(spm.model_name) = LOWER($2) OR spm.model_aliases ? $2)
		  AND spe.endpoint_type = $3
		FOR SHARE OF sp
		FOR UPDATE OF spe`, input.PoolID, input.ModelName, input.EndpointType).Scan(
		&endpointID, &probePlanVersion, &endpointConfigVersion, &poolConfigVersion, &lastProbeAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrSharedPoolMediaProbeConflict
	}
	if err != nil {
		return err
	}
	if poolConfigVersion != input.PoolConfigVersion ||
		probePlanVersion != input.ProbePlanVersion || endpointConfigVersion != input.EndpointConfigVersion {
		return service.ErrSharedPoolMediaProbeConflict
	}

	var probeID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO shared_pool_media_endpoint_probes (
			endpoint_id, pool_id, account_id, endpoint_type, operation_id,
			pool_config_version, account_config_version,
			probe_plan_version, endpoint_config_version, result_status,
			upstream_http_status, output_observed, async_terminal_observed,
			error_type, error_message, checked_at, expires_at
		) VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6, NULLIF($7, 0), $8, $9, $10,
		          $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (endpoint_id, operation_id) DO NOTHING
		RETURNING id`, endpointID, input.PoolID, input.AccountID, input.EndpointType,
		input.OperationID, input.PoolConfigVersion, input.AccountConfigVersion,
		input.ProbePlanVersion, input.EndpointConfigVersion, input.ResultStatus,
		input.UpstreamHTTPStatus, input.OutputObserved, input.AsyncTerminalObserved,
		input.ErrorType, input.ErrorMessage, input.CheckedAt, input.ExpiresAt).Scan(&probeID)
	if errors.Is(err, sql.ErrNoRows) {
		var existingStatus, existingEndpointType, existingErrorType, existingErrorMessage string
		var existingAccountID, existingAccountConfigVersion, existingHTTPStatus sql.NullInt64
		var existingPoolConfigVersion, existingProbePlanVersion, existingEndpointConfigVersion int64
		var existingOutput, existingAsync bool
		var existingCheckedAt, existingExpiresAt time.Time
		err = tx.QueryRowContext(ctx, `SELECT id, account_id, endpoint_type, result_status,
			pool_config_version, account_config_version, probe_plan_version, endpoint_config_version,
			upstream_http_status, output_observed, async_terminal_observed,
			error_type, error_message, checked_at, expires_at
			FROM shared_pool_media_endpoint_probes
			WHERE endpoint_id = $1 AND operation_id = $2`, endpointID, input.OperationID).Scan(
			&probeID, &existingAccountID, &existingEndpointType, &existingStatus,
			&existingPoolConfigVersion, &existingAccountConfigVersion,
			&existingProbePlanVersion, &existingEndpointConfigVersion,
			&existingHTTPStatus, &existingOutput, &existingAsync,
			&existingErrorType, &existingErrorMessage, &existingCheckedAt, &existingExpiresAt)
		if err != nil {
			return err
		}
		if existingAccountID.Int64 != input.AccountID || existingEndpointType != input.EndpointType ||
			existingStatus != input.ResultStatus || existingPoolConfigVersion != input.PoolConfigVersion ||
			existingAccountConfigVersion.Int64 != input.AccountConfigVersion ||
			existingProbePlanVersion != input.ProbePlanVersion || existingEndpointConfigVersion != input.EndpointConfigVersion ||
			(existingHTTPStatus.Valid != (input.UpstreamHTTPStatus != nil)) ||
			(existingHTTPStatus.Valid && int(existingHTTPStatus.Int64) != *input.UpstreamHTTPStatus) ||
			existingOutput != input.OutputObserved || existingErrorType != input.ErrorType ||
			existingErrorMessage != input.ErrorMessage ||
			existingAsync != input.AsyncTerminalObserved || !existingCheckedAt.Equal(input.CheckedAt) ||
			!existingExpiresAt.Equal(input.ExpiresAt) {
			return service.ErrSharedPoolMediaProbeConflict
		}
	} else if err != nil {
		return err
	}

	// A late response is retained in history but cannot overwrite a newer gate.
	if !lastProbeAt.Valid || !input.CheckedAt.Before(lastProbeAt.Time) {
		gateStatus := "failed"
		if input.ResultStatus == "passed" {
			gateStatus = "passed"
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE shared_pool_model_endpoints
			SET gate_status = $2, last_media_probe_id = $3,
			    last_media_probe_at = $4, media_probe_expires_at = $5,
			    updated_at = NOW()
			WHERE id = $1 AND probe_plan_version = $6 AND config_version = $7`,
			endpointID, gateStatus, probeID, input.CheckedAt, input.ExpiresAt,
			input.ProbePlanVersion, input.EndpointConfigVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		if affected != 1 {
			return service.ErrSharedPoolMediaProbeConflict
		}
	}
	return tx.Commit()
}

func (r *bizDecipherRepository) ExpireSharedPoolMediaEndpointGates(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("shared pool media probe repository is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `WITH expired AS (
		SELECT id FROM shared_pool_model_endpoints
		WHERE endpoint_type IN ('image_generation', 'image_edit', 'video')
		  AND gate_status = 'passed' AND media_probe_expires_at <= $1
		ORDER BY media_probe_expires_at, id
		LIMIT $2 FOR UPDATE SKIP LOCKED
	)
	UPDATE shared_pool_model_endpoints spe
	SET gate_status = 'stale', updated_at = NOW()
	FROM expired WHERE spe.id = expired.id
	RETURNING spe.id`, now, limit)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}
