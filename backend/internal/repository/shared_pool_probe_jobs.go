package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const sharedPoolProbeJobColumns = `id::text, operation_id, pool_id, account_id, owner_id,
	model_name, upstream_model_name, probe_type, check_level, config_version, status,
	attempt, max_attempts, lease_owner, lease_expires_at, heartbeat_at, started_at,
	finished_at, result_summary, error_type, error_message, created_at, updated_at`

type sharedPoolProbeQueryExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func scanSharedPoolProbeJob(s scanner) (*service.SharedPoolProbeJob, error) {
	var job service.SharedPoolProbeJob
	var accountID sql.NullInt64
	var leaseExpiresAt, heartbeatAt, startedAt, finishedAt sql.NullTime
	var resultRaw []byte
	if err := s.Scan(
		&job.ID, &job.OperationID, &job.PoolID, &accountID, &job.OwnerID,
		&job.ModelName, &job.UpstreamModelName, &job.ProbeType, &job.CheckLevel, &job.ConfigVersion, &job.Status,
		&job.Attempt, &job.MaxAttempts, &job.LeaseOwner, &leaseExpiresAt, &heartbeatAt, &startedAt,
		&finishedAt, &resultRaw, &job.ErrorType, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if accountID.Valid {
		job.AccountID = &accountID.Int64
	}
	if leaseExpiresAt.Valid {
		job.LeaseExpiresAt = &leaseExpiresAt.Time
	}
	if heartbeatAt.Valid {
		job.HeartbeatAt = &heartbeatAt.Time
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	if len(resultRaw) > 0 && string(resultRaw) != "{}" && string(resultRaw) != "null" {
		var result service.SharedPoolUpstreamProbeResult
		if json.Unmarshal(resultRaw, &result) == nil {
			job.Result = &result
		}
	}
	return &job, nil
}

func (r *bizDecipherRepository) loadSharedPoolProbeJobItems(ctx context.Context, job *service.SharedPoolProbeJob) error {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT item_index, check_id, title, category, required,
		success, http_status, latency_ms, error_type, error_message, evidence
		FROM shared_pool_probe_job_items WHERE job_id = $1::uuid ORDER BY item_index`, job.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	job.Items = []service.SharedPoolProbeJobItem{}
	for rows.Next() {
		var item service.SharedPoolProbeJobItem
		if err := rows.Scan(&item.Index, &item.CheckID, &item.Title, &item.Category, &item.Required,
			&item.Success, &item.HTTPStatus, &item.LatencyMs, &item.ErrorType, &item.ErrorMessage, &item.Evidence); err != nil {
			return err
		}
		job.Items = append(job.Items, item)
	}
	return rows.Err()
}

func sharedPoolProbeJobRequestMatches(job *service.SharedPoolProbeJob, input service.SharedPoolProbeJobEnqueueInput) bool {
	if job == nil {
		return false
	}
	accountID := int64(0)
	if job.AccountID != nil {
		accountID = *job.AccountID
	}
	return job.PoolID == input.PoolID && accountID == input.AccountID &&
		strings.EqualFold(strings.TrimSpace(job.ModelName), strings.TrimSpace(input.ModelName)) &&
		strings.EqualFold(strings.TrimSpace(job.ProbeType), strings.TrimSpace(input.ProbeType))
}

func (r *bizDecipherRepository) EnqueueSharedPoolProbeJob(ctx context.Context, input service.SharedPoolProbeJobEnqueueInput) (*service.SharedPoolProbeJob, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("repository unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var configVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT config_version FROM shared_pools
		WHERE id = $1 AND owner_id = $2 FOR SHARE`, input.PoolID, input.OwnerID).Scan(&configVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, service.ErrPoolForbidden
		}
		return nil, false, err
	}
	if input.AccountID > 0 {
		var accountExists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM shared_pool_accounts
			WHERE id = $1 AND pool_id = $2 AND owner_id = $3 AND deleted_at IS NULL`,
			input.AccountID, input.PoolID, input.OwnerID).Scan(&accountExists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, false, service.ErrPoolForbidden
			}
			return nil, false, err
		}
	}

	operationID := strings.TrimSpace(input.OperationID)
	row := tx.QueryRowContext(ctx, `SELECT `+sharedPoolProbeJobColumns+`
		FROM shared_pool_probe_jobs WHERE owner_id = $1 AND operation_id = $2`, input.OwnerID, operationID)
	if existing, scanErr := scanSharedPoolProbeJob(row); scanErr == nil {
		if !sharedPoolProbeJobRequestMatches(existing, input) {
			return nil, false, errors.New("operation id was already used for a different probe request")
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return existing, false, nil
	} else if !errors.Is(scanErr, sql.ErrNoRows) {
		return nil, false, scanErr
	}

	jobID := uuid.NewString()
	var accountID any
	if input.AccountID > 0 {
		accountID = input.AccountID
	}
	row = tx.QueryRowContext(ctx, `INSERT INTO shared_pool_probe_jobs
		(id, operation_id, pool_id, account_id, owner_id, model_name, upstream_model_name,
		 probe_type, check_level, config_version, status)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'queued')
		ON CONFLICT (owner_id, operation_id) DO NOTHING
		RETURNING `+sharedPoolProbeJobColumns,
		jobID, operationID, input.PoolID, accountID, input.OwnerID,
		strings.TrimSpace(input.ModelName), strings.TrimSpace(input.UpstreamModelName),
		strings.TrimSpace(input.ProbeType), strings.TrimSpace(input.CheckLevel), configVersion)
	job, err := scanSharedPoolProbeJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		existing, getErr := scanSharedPoolProbeJob(tx.QueryRowContext(ctx, `SELECT `+sharedPoolProbeJobColumns+`
			FROM shared_pool_probe_jobs WHERE owner_id = $1 AND operation_id = $2`, input.OwnerID, operationID))
		if getErr != nil {
			return nil, false, getErr
		}
		if !sharedPoolProbeJobRequestMatches(existing, input) {
			return nil, false, errors.New("operation id was already used for a different probe request")
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return job, true, nil
}

func (r *bizDecipherRepository) GetSharedPoolProbeJob(ctx context.Context, jobID string, ownerID int64) (*service.SharedPoolProbeJob, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+sharedPoolProbeJobColumns+`
		FROM shared_pool_probe_jobs WHERE id = $1::uuid AND owner_id = $2`, strings.TrimSpace(jobID), ownerID)
	job, err := scanSharedPoolProbeJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	if err != nil {
		return nil, err
	}
	if err := r.loadSharedPoolProbeJobItems(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *bizDecipherRepository) ClaimSharedPoolProbeJobs(ctx context.Context, leaseOwner string, limit int, lease time.Duration) ([]service.SharedPoolProbeJob, error) {
	if limit <= 0 {
		limit = 1
	}
	leaseSeconds := int(lease.Seconds())
	if leaseSeconds < 30 {
		leaseSeconds = 30
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_probe_jobs SET
		status = 'timed_out', error_type = 'lease_exhausted',
		error_message = 'probe worker lease expired after maximum attempts',
		finished_at = NOW(), lease_owner = '', lease_expires_at = NULL, updated_at = NOW()
		WHERE status = 'running' AND lease_expires_at < NOW() AND attempt >= max_attempts`); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `WITH candidates AS (
		SELECT id FROM shared_pool_probe_jobs
		WHERE (status = 'queued' AND available_at <= NOW())
		   OR (status = 'running' AND lease_expires_at < NOW() AND attempt < max_attempts)
		ORDER BY available_at ASC, created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	)
	UPDATE shared_pool_probe_jobs j SET
		status = 'running', attempt = j.attempt + 1, lease_owner = $2,
		lease_expires_at = NOW() + ($3 * INTERVAL '1 second'), heartbeat_at = NOW(),
		started_at = COALESCE(j.started_at, NOW()), updated_at = NOW()
	FROM candidates c WHERE j.id = c.id
	RETURNING `+strings.ReplaceAll(sharedPoolProbeJobColumns, "id::text", "j.id::text"), limit, leaseOwner, leaseSeconds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	jobs := []service.SharedPoolProbeJob{}
	for rows.Next() {
		job, scanErr := scanSharedPoolProbeJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *bizDecipherRepository) HeartbeatSharedPoolProbeJob(ctx context.Context, jobID, leaseOwner string, lease time.Duration) error {
	leaseSeconds := int(lease.Seconds())
	if leaseSeconds < 30 {
		leaseSeconds = 30
	}
	result, err := r.db.ExecContext(ctx, `UPDATE shared_pool_probe_jobs SET
		heartbeat_at = NOW(), lease_expires_at = NOW() + ($3 * INTERVAL '1 second'), updated_at = NOW()
		WHERE id = $1::uuid AND status = 'running' AND lease_owner = $2`, jobID, leaseOwner, leaseSeconds)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errors.New("shared pool probe job lease is no longer owned by this worker")
	}
	return nil
}

func normalizeSharedPoolProbeJobTerminalStatus(status string) string {
	switch strings.TrimSpace(status) {
	case service.SharedPoolProbeJobSucceeded, service.SharedPoolProbeJobFailed, service.SharedPoolProbeJobTimedOut, service.SharedPoolProbeJobCancelled:
		return strings.TrimSpace(status)
	default:
		return service.SharedPoolProbeJobFailed
	}
}

func (r *bizDecipherRepository) insertSharedPoolProbeJobItemsTx(ctx context.Context, tx *sql.Tx, jobID string, result *service.SharedPoolUpstreamProbeResult) error {
	if result == nil {
		return nil
	}
	for index, item := range result.Checks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_probe_job_items
			(job_id, item_index, check_id, title, category, required, success, http_status,
			 latency_ms, error_type, error_message, evidence)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (job_id, item_index) DO UPDATE SET
				check_id = EXCLUDED.check_id, title = EXCLUDED.title, category = EXCLUDED.category,
				required = EXCLUDED.required, success = EXCLUDED.success,
				http_status = EXCLUDED.http_status, latency_ms = EXCLUDED.latency_ms,
				error_type = EXCLUDED.error_type, error_message = EXCLUDED.error_message,
				evidence = EXCLUDED.evidence`,
			jobID, index, item.ID, item.Title, item.Category, item.Required, item.Success,
			item.HTTPStatus, item.LatencyMs, item.ErrorType, item.ErrorMessage, item.Evidence); err != nil {
			return err
		}
	}
	return nil
}

func (r *bizDecipherRepository) insertSharedPoolProbeJobHistoryTx(ctx context.Context, tx *sql.Tx, job *service.SharedPoolProbeJob, history service.SharedPoolProbeHistoryInput) error {
	checkedAt := history.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	var accountID any
	if history.AccountID > 0 {
		accountID = history.AccountID
	}
	metadata, _ := json.Marshal(history.Metadata)
	_, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_probe_histories
		(pool_id, account_id, owner_id, model_name, upstream_model_name, probe_type,
		 success, http_status, error_type, error_message, latency_ms, checked_at, metadata,
		 job_id, config_version)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6, $7, $8, $9, $10, $11, $12,
		 $13::jsonb, $14::uuid, $15)`,
		history.PoolID, accountID, history.OwnerID, history.ModelName, history.UpstreamModelName,
		history.ProbeType, history.Success, history.HTTPStatus, history.ErrorType,
		history.ErrorMessage, history.LatencyMs, checkedAt, string(metadata), job.ID, job.ConfigVersion)
	return err
}

func applySharedPoolProbeJobAccountResultTx(ctx context.Context, tx *sql.Tx, history service.SharedPoolProbeHistoryInput) error {
	if history.AccountID <= 0 {
		return nil
	}
	checkedAt := history.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	statusExpr := `CASE WHEN status IN ('testing', 'limited', 'offline') THEN 'active' ELSE status END`
	if !history.Success {
		statusExpr = `status`
		if history.HTTPStatus == 401 || history.HTTPStatus == 402 || history.HTTPStatus == 403 {
			statusExpr = `'offline'`
		} else if history.HTTPStatus == 429 || strings.TrimSpace(history.ErrorType) == "rate_limited" {
			statusExpr = `'limited'`
		} else if history.HTTPStatus == 0 || history.HTTPStatus >= 500 || strings.TrimSpace(history.ErrorType) == "timeout" || strings.TrimSpace(history.ErrorType) == "network_error" {
			statusExpr = `CASE WHEN status = 'active' THEN 'testing' ELSE status END`
		}
	}
	fullProbe := strings.EqualFold(strings.TrimSpace(history.Metadata.CheckLevel), "full") || history.Metadata.GateRequired
	_, err := tx.ExecContext(ctx, `UPDATE shared_pool_accounts SET
		last_probe_at = $4, last_probe_success = $5,
		last_probe_error_type = CASE WHEN $5 THEN '' ELSE $6 END,
		last_probe_error_message = CASE WHEN $5 THEN '' ELSE $7 END,
		last_successful_probe_at = CASE WHEN $5 THEN $4 ELSE last_successful_probe_at END,
		gate_required = CASE WHEN $8 THEN $9 ELSE gate_required END,
		gate_passed = CASE WHEN $8 THEN $10 ELSE gate_passed END,
		full_check_score = CASE WHEN $8 THEN $11 ELSE full_check_score END,
		full_check_passed = CASE WHEN $8 THEN $12 ELSE full_check_passed END,
		full_check_total = CASE WHEN $8 THEN $13 ELSE full_check_total END,
		status = `+statusExpr+`, updated_at = NOW()
		WHERE pool_id = $1 AND id = $2 AND owner_id = $3 AND deleted_at IS NULL`,
		history.PoolID, history.AccountID, history.OwnerID, checkedAt, history.Success,
		history.ErrorType, history.ErrorMessage, fullProbe, history.Metadata.GateRequired,
		history.Metadata.GatePassed, history.Metadata.FullCheckScore,
		history.Metadata.FullCheckPassed, history.Metadata.FullCheckTotal)
	return err
}

func applySharedPoolProbeJobPoolResultTx(ctx context.Context, tx *sql.Tx, history service.SharedPoolProbeHistoryInput, allowAutoList bool) error {
	checkedAt := history.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	if history.Success {
		_, err := tx.ExecContext(ctx, `UPDATE shared_pools SET
			last_probe_at = $2, last_probe_success = TRUE,
			last_probe_error_type = '', last_probe_error_message = '',
			consecutive_probe_failures = 0, last_successful_probe_at = $2,
			avg_latency_ms = CASE WHEN $3 > 0 THEN $3 ELSE avg_latency_ms END,
			status = CASE WHEN NOT owner_paused AND status IN ('offline', 'limited', 'testing') THEN 'healthy' ELSE status END,
			`+sharedPoolProbeAvailabilityAssignments+`
			quality_score = LEAST(100, GREATEST(quality_score, 70)),
			listed = CASE
				WHEN owner_paused THEN FALSE
				WHEN $4 AND status <> 'maintenance' AND COALESCE(lifecycle_state, 'active') NOT IN ('archived', 'suspended')
					AND (governance_status IN ('normal', 'boosted') OR
						(governance_status = 'watch' AND governance_note IN ($5, $6))) THEN TRUE
				ELSE listed END,
			governance_status = CASE
				WHEN governance_status = 'watch' AND governance_note IN ($5, $6) THEN 'normal'
				ELSE governance_status END,
			governance_note = CASE
				WHEN governance_status = 'watch' AND governance_note IN ($5, $6) THEN ''
				ELSE governance_note END,
			status_note = CASE WHEN status_note IN ($5, $6) THEN '' ELSE status_note END,
			updated_at = NOW()
		WHERE id = $1 AND native_onboarding_state = 'legacy_existing' AND deleted_at IS NULL`, history.PoolID, checkedAt, history.LatencyMs, allowAutoList,
			sharedPoolObservationNote, sharedPoolDelistNote)
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE shared_pools SET
		last_probe_at = $2, last_probe_success = FALSE,
		last_probe_error_type = $3, last_probe_error_message = $4,
		consecutive_probe_failures = consecutive_probe_failures + 1,
		status = CASE
			WHEN owner_paused OR status = 'maintenance' THEN status
			WHEN $5 IN (401, 402, 403) THEN 'offline'
			WHEN $5 = 429 OR $3 = 'rate_limited' THEN 'limited'
			WHEN consecutive_probe_failures + 1 >= $6 THEN 'offline'
			WHEN $5 = 0 OR $5 >= 500 OR $3 IN ('timeout', 'network_error') THEN 'limited'
			ELSE status END,
		listed = CASE WHEN owner_paused OR consecutive_probe_failures + 1 >= $6 THEN FALSE ELSE listed END,
		`+sharedPoolProbeAvailabilityAssignments+`
		quality_score = GREATEST(quality_score - 2, 0), updated_at = NOW()
		WHERE id = $1 AND native_onboarding_state = 'legacy_existing' AND deleted_at IS NULL`, history.PoolID, checkedAt, strings.TrimSpace(history.ErrorType),
		strings.TrimSpace(history.ErrorMessage), history.HTTPStatus, sharedPoolDelistFailureThreshold)
	return err
}

func alignSharedPoolProbeJobHistoryConfigVersionTx(ctx context.Context, tx *sql.Tx, jobID string, poolID int64) error {
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_probe_histories history
		SET config_version = pool.config_version
		FROM shared_pools pool
		WHERE history.job_id = $1::uuid
		  AND history.pool_id = $2
		  AND pool.id = history.pool_id`, jobID, poolID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("shared pool probe history config alignment affected %d rows", affected)
	}
	return nil
}

func (r *bizDecipherRepository) CompleteSharedPoolProbeJob(ctx context.Context, completion service.SharedPoolProbeJobCompletion) (*service.SharedPoolProbeJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanSharedPoolProbeJob(tx.QueryRowContext(ctx, `SELECT `+sharedPoolProbeJobColumns+`
		FROM shared_pool_probe_jobs WHERE id = $1::uuid FOR UPDATE`, completion.JobID))
	if err != nil {
		return nil, err
	}
	if job.Status != service.SharedPoolProbeJobRunning || job.LeaseOwner != completion.LeaseOwner {
		return nil, errors.New("shared pool probe job lease is no longer owned by this worker")
	}
	var currentVersion int64
	var governanceStatus, governanceNote string
	if err := tx.QueryRowContext(ctx, `SELECT config_version, COALESCE(governance_status, 'normal'),
		COALESCE(governance_note, '')
		FROM shared_pools WHERE id = $1 AND owner_id = $2 FOR UPDATE`, job.PoolID, job.OwnerID).
		Scan(&currentVersion, &governanceStatus, &governanceNote); err != nil {
		return nil, err
	}
	if err := r.insertSharedPoolProbeJobItemsTx(ctx, tx, job.ID, completion.Result); err != nil {
		return nil, err
	}
	resultJSON := []byte(`{}`)
	if completion.Result != nil {
		if encoded, marshalErr := json.Marshal(completion.Result); marshalErr == nil {
			resultJSON = encoded
		}
	}
	terminalStatus := normalizeSharedPoolProbeJobTerminalStatus(completion.TerminalStatus)
	errorType := strings.TrimSpace(completion.History.ErrorType)
	errorMessage := strings.TrimSpace(completion.History.ErrorMessage)
	if currentVersion != job.ConfigVersion {
		terminalStatus = service.SharedPoolProbeJobStale
		errorType = "stale_config"
		errorMessage = fmt.Sprintf("pool configuration changed from version %d to %d while probe was running", job.ConfigVersion, currentVersion)
	} else {
		if err := r.insertSharedPoolProbeJobHistoryTx(ctx, tx, job, completion.History); err != nil {
			return nil, err
		}
		if completion.History.AccountID > 0 {
			if err := applySharedPoolProbeJobAccountResultTx(ctx, tx, completion.History); err != nil {
				return nil, err
			}
			if err := r.rollupSharedPoolMetricsFromAccountProbesWithExecutor(ctx, tx, job.PoolID); err != nil {
				return nil, err
			}
		} else {
			allowAutoList := sharedPoolProbeAllowsAutoListing(completion.History) &&
				sharedPoolAutoObservationManaged(governanceStatus, governanceNote)
			if err := applySharedPoolProbeJobPoolResultTx(ctx, tx, completion.History, allowAutoList); err != nil {
				return nil, err
			}
		}
		// Applying a successful or failed probe can legitimately change status or
		// listing fields, and those fields advance the pool's fencing version. Keep
		// the just-written history aligned with that post-projection version while
		// the same transaction is locked; later configuration edits still invalidate it.
		if err := alignSharedPoolProbeJobHistoryConfigVersionTx(ctx, tx, job.ID, job.PoolID); err != nil {
			return nil, err
		}
	}
	finishedAt := time.Now().UTC()
	row := tx.QueryRowContext(ctx, `UPDATE shared_pool_probe_jobs SET
		status = $3, result_summary = $4::jsonb, error_type = $5, error_message = $6,
		finished_at = $7, lease_owner = '', lease_expires_at = NULL, heartbeat_at = NOW(), updated_at = NOW()
		WHERE id = $1::uuid AND lease_owner = $2
		RETURNING `+sharedPoolProbeJobColumns,
		job.ID, completion.LeaseOwner, terminalStatus, string(resultJSON), errorType, errorMessage, finishedAt)
	completed, err := scanSharedPoolProbeJob(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return completed, nil
}
