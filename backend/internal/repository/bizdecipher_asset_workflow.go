package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) AdvanceAssetWorkflow(ctx context.Context, actor int64, key, digest string,
	initial service.AssetWorkflowState, advance func(*service.AssetWorkflowState) error) (*service.AssetWorkflowState, error) {
	body, err := json.Marshal(initial)
	if err != nil {
		return nil, err
	}
	// Persist the request binding even if dispatch fails or the HTTP client disconnects.
	if _, err = r.db.ExecContext(ctx, `INSERT INTO asset_workflow_checkpoints(actor_id,request_id,input_digest,state)
		VALUES($1,$2,$3,$4::jsonb) ON CONFLICT(actor_id,request_id) DO NOTHING`, actor, key, digest, string(body)); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var storedDigest string
	if err = tx.QueryRowContext(ctx, `SELECT input_digest,state FROM asset_workflow_checkpoints
		WHERE actor_id=$1 AND request_id=$2 FOR UPDATE`, actor, key).Scan(&storedDigest, &body); err != nil {
		return nil, err
	}
	if storedDigest != digest {
		return nil, service.ErrAssetCommerceConflict
	}
	var state service.AssetWorkflowState
	if err = json.Unmarshal(body, &state); err != nil {
		return nil, err
	}
	if err = advance(&state); err != nil {
		return nil, err
	}
	body, err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE asset_workflow_checkpoints SET state=$3::jsonb,updated_at=NOW()
		WHERE actor_id=$1 AND request_id=$2`, actor, key, string(body)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *bizDecipherRepository) GetAssetWorkflow(ctx context.Context, actor int64, key string) (*service.AssetWorkflowState, error) {
	var body []byte
	err := r.db.QueryRowContext(ctx, `SELECT state FROM asset_workflow_checkpoints WHERE actor_id=$1 AND request_id=$2`, actor, key).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCapabilityAssetPackageNotFound
	}
	if err != nil {
		return nil, err
	}
	var state service.AssetWorkflowState
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

var _ service.AssetWorkflowRepository = (*bizDecipherRepository)(nil)

func (r *bizDecipherRepository) ListAssetWorkflows(ctx context.Context, actor, assetID int64, version string) ([]service.AssetWorkflowState, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT state FROM asset_workflow_checkpoints
		WHERE actor_id=$1 AND state->>'asset_id'=$2 AND state->>'version'=$3 ORDER BY created_at DESC LIMIT 20`,
		actor, fmt.Sprint(assetID), version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.AssetWorkflowState{}
	for rows.Next() {
		var body []byte
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		var state service.AssetWorkflowState
		if err := json.Unmarshal(body, &state); err != nil {
			return nil, err
		}
		state.Output = ""
		for i := range state.Steps {
			state.Steps[i].Output = ""
		}
		result = append(result, state)
	}
	return result, rows.Err()
}
