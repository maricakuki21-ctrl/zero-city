package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Only published native pools and active official groups participate.
// Credentials remain in accounts and never enter monitoring configuration.
const automaticResourceModelsSQL = `
 SELECT DISTINCT ag.account_id, model.name AS model
 FROM account_groups ag
 JOIN accounts a ON a.id=ag.account_id AND a.deleted_at IS NULL
 JOIN groups g ON g.id=ag.group_id AND g.deleted_at IS NULL AND g.status='active'
 LEFT JOIN shared_pool_sub2_bindings b ON b.canonical_group_id=g.id
 LEFT JOIN shared_pools sp ON sp.id=b.pool_id AND sp.owner_id=b.owner_id
 CROSS JOIN LATERAL (
   SELECT value AS name FROM jsonb_array_elements_text(
     CASE WHEN g.models_list_config->>'enabled'='true'
       THEN COALESCE(g.models_list_config->'models','[]'::jsonb) ELSE '[]'::jsonb END)
   UNION
   SELECT model_name FROM shared_pool_models
     WHERE pool_id=sp.id AND enabled=true AND model_open=true
   UNION
   SELECT name FROM jsonb_object_keys(
     CASE WHEN jsonb_typeof(a.credentials->'model_mapping')='object'
       THEN a.credentials->'model_mapping' ELSE '{}'::jsonb END) AS mappings(name)
     WHERE b.pool_id IS NULL AND COALESCE(g.models_list_config->>'enabled','false')<>'true'
       AND name<>'' AND name NOT LIKE '%*%'
   UNION
   SELECT g.default_mapped_model
     WHERE b.pool_id IS NULL AND COALESCE(g.models_list_config->>'enabled','false')<>'true'
       AND COALESCE(g.default_mapped_model,'')<>'' AND g.default_mapped_model NOT LIKE '%*%'
 ) model
 WHERE a.status='active' AND a.schedulable=true
   AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at<=NOW())
   AND (a.overload_until IS NULL OR a.overload_until<=NOW())
   AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until<=NOW())
   AND (a.expires_at IS NULL OR a.expires_at>NOW() OR a.auto_pause_on_expired=false)
   AND (b.pool_id IS NULL OR (b.lifecycle='active' AND sp.listed=true AND sp.deleted_at IS NULL
     AND sp.lifecycle_state<>'archived' AND sp.governance_status NOT IN ('banned','suppressed')))
`

func (r *scheduledTestPlanRepository) ListAutomaticTestCandidates(ctx context.Context) ([]service.AutomaticTestCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT account_id,jsonb_agg(model ORDER BY model) FROM (`+automaticResourceModelsSQL+`) m GROUP BY account_id ORDER BY account_id LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.AutomaticTestCandidate{}
	for rows.Next() {
		var item service.AutomaticTestCandidate
		var models []byte
		if err := rows.Scan(&item.AccountID, &models); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(models, &item.Models); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *channelMonitorRepository) ReadGroupAccountObservation(ctx context.Context, groupID int64, models []string) (*service.UserMonitorView, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH evidence AS (
		 SELECT r.id,p.model_id,r.status,r.latency_ms,r.started_at
		 FROM scheduled_test_results r
		 JOIN scheduled_test_plans p ON p.id=r.plan_id
		 JOIN account_groups ag ON ag.account_id=p.account_id AND ag.group_id=$1
		 JOIN accounts a ON a.id=p.account_id AND a.deleted_at IS NULL
		 WHERE r.started_at>=NOW()-INTERVAL '7 days' AND r.started_at>=ag.created_at
		   AND p.model_id=ANY($2) AND p.enabled=true
		), primary_model AS (SELECT model_id FROM evidence ORDER BY started_at DESC,id DESC LIMIT 1)
		SELECT e.model_id,e.status,e.latency_ms,e.started_at,
		       COUNT(*) OVER(),COUNT(*) FILTER(WHERE e.status='success') OVER()
		FROM evidence e JOIN primary_model p USING(model_id)
		ORDER BY e.started_at DESC,e.id DESC LIMIT 24`, groupID, pq.Array(models))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	view := &service.UserMonitorView{Timeline: []service.UserMonitorTimelinePoint{}}
	for rows.Next() {
		var point service.UserMonitorTimelinePoint
		var status, model string
		var latency int
		var total, passed int64
		if err := rows.Scan(&model, &status, &latency, &point.CheckedAt, &total, &passed); err != nil {
			return nil, err
		}
		point.Status = service.MonitorStatusFailed
		if status == "success" {
			point.Status = service.MonitorStatusOperational
		}
		point.LatencyMs = &latency
		if len(view.Timeline) == 0 {
			view.PrimaryModel, view.PrimaryStatus, view.PrimaryLatencyMs = model, point.Status, point.LatencyMs
			view.Availability7d = float64(passed) * 100 / float64(total)
		}
		view.Timeline = append(view.Timeline, point)
	}
	return view, rows.Err()
}

func (r *bizDecipherRepository) nativePoolAccountHistory(ctx context.Context, poolID, ownerID int64, limit int) ([]service.SharedPoolProbeHistory, bool, error) {
	var groupID int64
	err := r.db.QueryRowContext(ctx, `SELECT b.canonical_group_id FROM shared_pool_sub2_bindings b
		JOIN shared_pools sp ON sp.id=b.pool_id AND sp.owner_id=b.owner_id
		WHERE b.pool_id=$1 AND b.lifecycle='active' AND sp.deleted_at IS NULL
		  AND ($2::bigint=0 OR sp.owner_id=$2)`, poolID, ownerID).Scan(&groupID)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT r.id,p.model_id,r.status,r.latency_ms,r.started_at
		FROM scheduled_test_results r JOIN scheduled_test_plans p ON p.id=r.plan_id
		JOIN account_groups ag ON ag.account_id=p.account_id AND ag.group_id=$1
		JOIN accounts a ON a.id=p.account_id AND a.deleted_at IS NULL
		WHERE p.enabled=true AND r.started_at>=ag.created_at AND r.started_at>=NOW()-INTERVAL '7 days'
		  AND EXISTS(SELECT 1 FROM shared_pool_models m WHERE m.pool_id=$2 AND m.model_name=p.model_id AND m.enabled=true AND m.model_open=true)
		ORDER BY r.started_at DESC,r.id DESC LIMIT $3`, groupID, poolID, clampLimit(limit))
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()
	out := []service.SharedPoolProbeHistory{}
	for rows.Next() {
		var item service.SharedPoolProbeHistory
		var status string
		if err := rows.Scan(&item.ID, &item.ModelName, &status, &item.LatencyMs, &item.CheckedAt); err != nil {
			return nil, true, err
		}
		item.PoolID = poolID
		item.Success = status == "success"
		item.ProbeType = "scheduled_native"
		item.CreatedAt = item.CheckedAt
		// Do not leak upstream credentials, raw errors, or native account IDs to members.
		out = append(out, item)
	}
	return out, true, rows.Err()
}

func (r *scheduledTestPlanRepository) EnsureAutomaticTest(ctx context.Context, accountID int64, model string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(274, $1::integer)`, accountID); err != nil {
		return err
	}
	// Keep a manually disabled plan disabled. Model changes retire only auto plans.
	if _, err = tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false,updated_at=NOW()
		WHERE account_id=$1 AND auto_managed=true AND model_id<>$2 AND enabled=true`, accountID, model); err != nil {
		return err
	}
	if model == "" {
		return tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO scheduled_test_plans
		(account_id,model_id,cron_expression,enabled,max_results,auto_recover,next_run_at,auto_managed)
		SELECT $1::bigint,$2::varchar,'*/15 * * * *',true,800,false,NOW(),true
		WHERE NOT EXISTS (SELECT 1 FROM scheduled_test_plans WHERE account_id=$1::bigint AND model_id=$2::varchar)
		ON CONFLICT DO NOTHING`, accountID, model)
	if err != nil {
		return err
	}
	return tx.Commit()
}
