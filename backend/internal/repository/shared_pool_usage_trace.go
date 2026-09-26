package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) BeginSharedPoolUsageTrace(ctx context.Context, trace service.SharedPoolUsageTrace) error {
	if r == nil || r.db == nil {
		return errors.New("shared pool usage trace repository is unavailable")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO shared_pool_usage_traces (
			access_key_id, pool_id, user_id, request_id,
			pool_name_snapshot, model_snapshot, endpoint, account_alias,
			status, failure_stage, settlement_outcome,
			upstream_started, usage_observed,
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			image_count, image_size, video_count, video_resolution, video_duration_seconds,
			auth_latency_ms, seat_latency_ms, routing_latency_ms,
			concurrency_latency_ms, reservation_latency_ms,
			upstream_latency_ms, first_token_ms, settlement_latency_ms, total_latency_ms,
			retry_count, http_status, created_at, completed_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			'pending', '', 'pending',
			FALSE, FALSE,
			0, 0, 0, 0,
			0, '', 0, '', 0,
			$9, $10, $11, $12, $13,
			NULL, NULL, NULL, NULL,
			0, NULL, $14, NULL
		)
		ON CONFLICT (access_key_id, request_id) DO NOTHING`,
		trace.AccessKeyID,
		trace.PoolID,
		trace.UserID,
		trace.RequestID,
		trace.PoolNameSnapshot,
		trace.ModelSnapshot,
		trace.Endpoint,
		trace.AccountAlias,
		sharedPoolTraceNullInt64(trace.AuthLatencyMs),
		sharedPoolTraceNullInt64(trace.SeatLatencyMs),
		sharedPoolTraceNullInt64(trace.RoutingLatencyMs),
		sharedPoolTraceNullInt64(trace.ConcurrencyLatencyMs),
		sharedPoolTraceNullInt64(trace.ReservationLatencyMs),
		trace.CreatedAt,
	)
	return err
}

func (r *bizDecipherRepository) FinalizeSharedPoolUsageTrace(ctx context.Context, trace service.SharedPoolUsageTrace) error {
	if r == nil || r.db == nil {
		return errors.New("shared pool usage trace repository is unavailable")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO shared_pool_usage_traces (
			access_key_id, pool_id, user_id, request_id,
			pool_name_snapshot, model_snapshot, endpoint, account_alias,
			status, failure_stage, settlement_outcome,
			upstream_started, usage_observed,
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			image_count, image_size, video_count, video_resolution, video_duration_seconds,
			auth_latency_ms, seat_latency_ms, routing_latency_ms,
			concurrency_latency_ms, reservation_latency_ms,
			upstream_latency_ms, first_token_ms, settlement_latency_ms, total_latency_ms,
			retry_count, http_status, created_at, completed_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11,
			$12, $13,
			$14, $15, $16, $17,
			$18, $19, $20, $21, $22,
			$23, $24, $25, $26, $27,
			$28, $29, $30, $31,
			$32, $33, $34, $35
		)
		ON CONFLICT (access_key_id, request_id) DO UPDATE SET
			status = EXCLUDED.status,
			failure_stage = EXCLUDED.failure_stage,
			settlement_outcome = EXCLUDED.settlement_outcome,
			upstream_started = EXCLUDED.upstream_started,
			usage_observed = EXCLUDED.usage_observed,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			image_count = EXCLUDED.image_count,
			image_size = EXCLUDED.image_size,
			video_count = EXCLUDED.video_count,
			video_resolution = EXCLUDED.video_resolution,
			video_duration_seconds = EXCLUDED.video_duration_seconds,
			upstream_latency_ms = EXCLUDED.upstream_latency_ms,
			first_token_ms = EXCLUDED.first_token_ms,
			settlement_latency_ms = EXCLUDED.settlement_latency_ms,
			total_latency_ms = EXCLUDED.total_latency_ms,
			retry_count = EXCLUDED.retry_count,
			http_status = EXCLUDED.http_status,
			completed_at = EXCLUDED.completed_at
		WHERE shared_pool_usage_traces.status = 'pending'`,
		trace.AccessKeyID,
		trace.PoolID,
		trace.UserID,
		trace.RequestID,
		trace.PoolNameSnapshot,
		trace.ModelSnapshot,
		trace.Endpoint,
		trace.AccountAlias,
		trace.Status,
		trace.FailureStage,
		trace.SettlementOutcome,
		trace.UpstreamStarted,
		trace.UsageObserved,
		trace.InputTokens,
		trace.OutputTokens,
		trace.CacheReadTokens,
		trace.CacheCreationTokens,
		trace.ImageCount,
		trace.ImageSize,
		trace.VideoCount,
		trace.VideoResolution,
		trace.VideoDurationSeconds,
		sharedPoolTraceNullInt64(trace.AuthLatencyMs),
		sharedPoolTraceNullInt64(trace.SeatLatencyMs),
		sharedPoolTraceNullInt64(trace.RoutingLatencyMs),
		sharedPoolTraceNullInt64(trace.ConcurrencyLatencyMs),
		sharedPoolTraceNullInt64(trace.ReservationLatencyMs),
		sharedPoolTraceNullInt64(trace.UpstreamLatencyMs),
		sharedPoolTraceNullInt64(trace.FirstTokenMs),
		sharedPoolTraceNullInt64(trace.SettlementLatencyMs),
		sharedPoolTraceNullInt64(trace.TotalLatencyMs),
		trace.RetryCount,
		sharedPoolTraceNullInt(trace.HTTPStatus),
		trace.CreatedAt,
		sharedPoolTraceNullTime(trace.CompletedAt),
	)
	return err
}

func (r *bizDecipherRepository) ListSharedPoolUsageTraces(
	ctx context.Context,
	userID, beforeID int64,
	limit int,
) (*service.SharedPoolUsageTracePage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("shared pool usage trace repository is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, access_key_id, pool_id, user_id, request_id,
			pool_name_snapshot, model_snapshot, endpoint, account_alias,
			status, failure_stage, settlement_outcome,
			upstream_started, usage_observed,
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			image_count, image_size, video_count, video_resolution, video_duration_seconds,
			auth_latency_ms, seat_latency_ms, routing_latency_ms,
			concurrency_latency_ms, reservation_latency_ms,
			upstream_latency_ms, first_token_ms, settlement_latency_ms, total_latency_ms,
			retry_count, http_status, created_at, completed_at
		FROM shared_pool_usage_traces
		WHERE user_id = $1 AND ($2 = 0 OR id < $2)
		ORDER BY id DESC
		LIMIT $3`, userID, beforeID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.SharedPoolUsageTrace, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanSharedPoolUsageTrace(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextBeforeID := int64(0)
	if hasMore && len(items) > 0 {
		nextBeforeID = items[len(items)-1].ID
	}
	return &service.SharedPoolUsageTracePage{
		Items:        items,
		NextBeforeID: nextBeforeID,
		HasMore:      hasMore,
	}, nil
}

func scanSharedPoolUsageTrace(scanner interface{ Scan(...any) error }) (service.SharedPoolUsageTrace, error) {
	var (
		item          service.SharedPoolUsageTrace
		authMs        sql.NullInt64
		seatMs        sql.NullInt64
		routingMs     sql.NullInt64
		concurrencyMs sql.NullInt64
		reservationMs sql.NullInt64
		upstreamMs    sql.NullInt64
		firstTokenMs  sql.NullInt64
		settlementMs  sql.NullInt64
		totalMs       sql.NullInt64
		httpStatus    sql.NullInt64
		completedAt   sql.NullTime
	)
	err := scanner.Scan(
		&item.ID,
		&item.AccessKeyID,
		&item.PoolID,
		&item.UserID,
		&item.RequestID,
		&item.PoolNameSnapshot,
		&item.ModelSnapshot,
		&item.Endpoint,
		&item.AccountAlias,
		&item.Status,
		&item.FailureStage,
		&item.SettlementOutcome,
		&item.UpstreamStarted,
		&item.UsageObserved,
		&item.InputTokens,
		&item.OutputTokens,
		&item.CacheReadTokens,
		&item.CacheCreationTokens,
		&item.ImageCount,
		&item.ImageSize,
		&item.VideoCount,
		&item.VideoResolution,
		&item.VideoDurationSeconds,
		&authMs,
		&seatMs,
		&routingMs,
		&concurrencyMs,
		&reservationMs,
		&upstreamMs,
		&firstTokenMs,
		&settlementMs,
		&totalMs,
		&item.RetryCount,
		&httpStatus,
		&item.CreatedAt,
		&completedAt,
	)
	if err != nil {
		return service.SharedPoolUsageTrace{}, err
	}
	item.AuthLatencyMs = sharedPoolTraceInt64Ptr(authMs)
	item.SeatLatencyMs = sharedPoolTraceInt64Ptr(seatMs)
	item.RoutingLatencyMs = sharedPoolTraceInt64Ptr(routingMs)
	item.ConcurrencyLatencyMs = sharedPoolTraceInt64Ptr(concurrencyMs)
	item.ReservationLatencyMs = sharedPoolTraceInt64Ptr(reservationMs)
	item.UpstreamLatencyMs = sharedPoolTraceInt64Ptr(upstreamMs)
	item.FirstTokenMs = sharedPoolTraceInt64Ptr(firstTokenMs)
	item.SettlementLatencyMs = sharedPoolTraceInt64Ptr(settlementMs)
	item.TotalLatencyMs = sharedPoolTraceInt64Ptr(totalMs)
	if httpStatus.Valid {
		value := int(httpStatus.Int64)
		item.HTTPStatus = &value
	}
	if completedAt.Valid {
		value := completedAt.Time
		item.CompletedAt = &value
	}
	return item, nil
}

func sharedPoolTraceNullInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func sharedPoolTraceNullInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func sharedPoolTraceNullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func sharedPoolTraceInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}
