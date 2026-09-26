package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageBillingRepository) ClaimCanonicalMediaTask(
	ctx context.Context,
	input mediatask.ClaimBindingInput,
) (mediatask.Binding, bool, error) {
	if r == nil || r.db == nil {
		return mediatask.Binding{}, false, errors.New("canonical media task repository db is nil")
	}
	binding, err := scanCanonicalMediaTask(r.db.QueryRowContext(ctx, `
		INSERT INTO canonical_media_task_bindings (
			business_event_id, idempotency_key, request_hash, api_key_id, user_id,
			group_id, account_id, endpoint
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (api_key_id, idempotency_key) DO NOTHING
		RETURNING business_event_id, idempotency_key, request_hash, api_key_id, user_id,
			group_id, account_id, endpoint, state, COALESCE(upstream_task_id, '')
	`, input.Context.BusinessEventID(), input.Context.IdempotencyKey(), input.Context.RequestHash(),
		input.APIKeyID, input.UserID, input.GroupID, input.AccountID, strings.TrimSpace(input.Endpoint)))
	if err == nil {
		return binding, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return mediatask.Binding{}, false, err
	}
	binding, err = scanCanonicalMediaTask(r.db.QueryRowContext(ctx, `
		SELECT business_event_id, idempotency_key, request_hash, api_key_id, user_id,
			group_id, account_id, endpoint, state, COALESCE(upstream_task_id, '')
		FROM canonical_media_task_bindings
		WHERE api_key_id = $1 AND idempotency_key = $2
	`, input.APIKeyID, input.Context.IdempotencyKey()))
	if err != nil {
		return mediatask.Binding{}, false, err
	}
	if binding.Context.BusinessEventID() != input.Context.BusinessEventID() ||
		binding.Context.RequestHash() != input.Context.RequestHash() ||
		binding.UserID != input.UserID || binding.Endpoint != strings.TrimSpace(input.Endpoint) {
		return mediatask.Binding{}, false, service.ErrCanonicalMediaTaskConflict
	}
	return binding, false, nil
}

func (r *usageBillingRepository) AcceptCanonicalMediaTask(
	ctx context.Context,
	createContext mediatask.CreateContext,
	upstreamTaskID string,
) (mediatask.Binding, error) {
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if upstreamTaskID == "" {
		return mediatask.Binding{}, service.ErrCanonicalMediaTaskConflict
	}
	binding, err := scanCanonicalMediaTask(r.db.QueryRowContext(ctx, `
		UPDATE canonical_media_task_bindings
		SET state = 'accepted', upstream_task_id = $2, accepted_at = NOW()
		WHERE business_event_id = $1 AND state = 'pending' AND upstream_task_id IS NULL
		RETURNING business_event_id, idempotency_key, request_hash, api_key_id, user_id,
			group_id, account_id, endpoint, state, upstream_task_id
	`, createContext.BusinessEventID(), upstreamTaskID))
	if err == nil {
		return binding, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return mediatask.Binding{}, err
	}
	binding, err = r.getCanonicalMediaTaskByBusinessEvent(ctx, createContext.BusinessEventID())
	if err != nil {
		return mediatask.Binding{}, err
	}
	if !binding.Accepted() || binding.UpstreamTaskID != upstreamTaskID {
		return mediatask.Binding{}, service.ErrCanonicalMediaTaskConflict
	}
	return binding, nil
}

func (r *usageBillingRepository) ResolveCanonicalMediaTask(
	ctx context.Context,
	apiKeyID, userID int64,
	upstreamTaskID string,
) (mediatask.Binding, error) {
	binding, err := scanCanonicalMediaTask(r.db.QueryRowContext(ctx, `
		SELECT business_event_id, idempotency_key, request_hash, api_key_id, user_id,
			group_id, account_id, endpoint, state, upstream_task_id
		FROM canonical_media_task_bindings
		WHERE api_key_id = $1 AND user_id = $2 AND upstream_task_id = $3 AND state = 'accepted'
	`, apiKeyID, userID, strings.TrimSpace(upstreamTaskID)))
	if errors.Is(err, sql.ErrNoRows) {
		return mediatask.Binding{}, service.ErrCanonicalMediaTaskNotFound
	}
	return binding, err
}

func (r *usageBillingRepository) getCanonicalMediaTaskByBusinessEvent(ctx context.Context, businessEventID string) (mediatask.Binding, error) {
	return scanCanonicalMediaTask(r.db.QueryRowContext(ctx, `
		SELECT business_event_id, idempotency_key, request_hash, api_key_id, user_id,
			group_id, account_id, endpoint, state, COALESCE(upstream_task_id, '')
		FROM canonical_media_task_bindings WHERE business_event_id = $1
	`, businessEventID))
}

func scanCanonicalMediaTask(row scanner) (mediatask.Binding, error) {
	var businessEventID, idempotencyKey, requestHash, endpoint, state, upstreamTaskID string
	var apiKeyID, userID, groupID, accountID int64
	if err := row.Scan(&businessEventID, &idempotencyKey, &requestHash, &apiKeyID, &userID,
		&groupID, &accountID, &endpoint, &state, &upstreamTaskID); err != nil {
		return mediatask.Binding{}, err
	}
	ctx, err := mediatask.RestoreCreateContext(businessEventID, idempotencyKey, requestHash)
	if err != nil {
		return mediatask.Binding{}, fmt.Errorf("restore canonical media task context: %w", err)
	}
	return mediatask.Binding{Context: ctx, APIKeyID: apiKeyID, UserID: userID, GroupID: groupID,
		AccountID: accountID, Endpoint: endpoint, State: mediatask.BindingState(state), UpstreamTaskID: upstreamTaskID}, nil
}
