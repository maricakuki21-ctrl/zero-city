package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolUsageTraceBeginAndFinalizeAreIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	trace := service.SharedPoolUsageTrace{
		AccessKeyID:      11,
		PoolID:           22,
		UserID:           33,
		RequestID:        "sp-trace-1",
		PoolNameSnapshot: "小白友好池",
		ModelSnapshot:    "gpt-test",
		Endpoint:         "/v1/responses",
		AccountAlias:     "共享线路 ABC123",
		CreatedAt:        now,
	}

	mock.ExpectExec(`(?s)INSERT INTO shared_pool_usage_traces.*ON CONFLICT \(access_key_id, request_id\) DO NOTHING`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.BeginSharedPoolUsageTrace(context.Background(), trace))

	completed := now.Add(time.Second)
	trace.Status = service.SharedPoolUsageTraceStatusSucceeded
	trace.SettlementOutcome = service.SharedPoolSettlementSettled
	trace.UpstreamStarted = true
	trace.InputTokens = 10
	trace.OutputTokens = 5
	trace.CompletedAt = &completed
	mock.ExpectExec(`(?s)INSERT INTO shared_pool_usage_traces.*DO UPDATE SET.*WHERE shared_pool_usage_traces.status = 'pending'`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.FinalizeSharedPoolUsageTrace(context.Background(), trace))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSharedPoolUsageTracesUsesUserCursorAndReturnsSafeFields(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	columns := []string{
		"id", "access_key_id", "pool_id", "user_id", "request_id",
		"pool_name_snapshot", "model_snapshot", "endpoint", "account_alias",
		"status", "failure_stage", "settlement_outcome",
		"upstream_started", "usage_observed",
		"input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens",
		"image_count", "image_size", "video_count", "video_resolution", "video_duration_seconds",
		"auth_latency_ms", "seat_latency_ms", "routing_latency_ms",
		"concurrency_latency_ms", "reservation_latency_ms",
		"upstream_latency_ms", "first_token_ms", "settlement_latency_ms", "total_latency_ms",
		"retry_count", "http_status", "created_at", "completed_at",
	}
	rows := sqlmock.NewRows(columns).
		AddRow(7, 11, 22, 33, "req-7", "小白友好池", "gpt-test", "/v1/responses", "共享线路 ABC123", "succeeded", "", "settled", true, true, 10, 5, 3, 2, 0, "", 0, "", 0, 4, nil, 8, 1, 2, 120, 30, 6, 171, 0, 200, now, now.Add(time.Second))
	mock.ExpectQuery(`(?s)FROM shared_pool_usage_traces.*WHERE user_id = \$1 AND \(\$2 = 0 OR id < \$2\)`).
		WithArgs(int64(33), int64(0), 21).
		WillReturnRows(rows)

	page, err := repo.ListSharedPoolUsageTraces(context.Background(), 33, 0, 20)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "共享线路 ABC123", page.Items[0].AccountAlias)
	require.Equal(t, int64(120), *page.Items[0].UpstreamLatencyMs)
	require.False(t, page.HasMore)
	require.NoError(t, mock.ExpectationsWereMet())
}
