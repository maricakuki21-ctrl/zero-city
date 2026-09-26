package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetLatestSharedPoolFullCheckHistoryUsesCurrentConfigFence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	checkedAt := time.Now().UTC()

	mock.ExpectQuery(`(?s)FROM shared_pool_probe_histories h.*JOIN shared_pools sp ON sp.id = h.pool_id.*h.config_version = sp.config_version.*metadata->>'check_level'.*= 'full'.*LIMIT 1`).
		WithArgs(int64(274)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "pool_id", "account_id", "owner_id", "model_name",
			"upstream_model_name", "probe_type", "success", "http_status",
			"error_type", "error_message", "latency_ms", "checked_at",
			"created_at", "metadata",
		}).AddRow(
			int64(90), int64(274), nil, int64(7), "gpt-5.6-sol",
			"gpt-5.6-sol", "publish_gate", true, 200,
			"", "", 123, checkedAt, checkedAt,
			[]byte(`{"check_level":"full","gate_passed":true,"full_check_passed":15,"full_check_total":15,"full_check_score":100,"checks":[{"id":"models","title":"Models","category":"capability","required":true,"success":true,"http_status":200,"latency_ms":12}]}`),
		))

	history, err := repo.GetLatestSharedPoolFullCheckHistory(context.Background(), 274)
	require.NoError(t, err)
	require.NotNil(t, history)
	require.Equal(t, "full", history.Metadata.CheckLevel)
	require.Equal(t, 15, history.Metadata.FullCheckTotal)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHydrateSharedPoolProbeSummariesIgnoresBasicAndStaleConfigResults(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	pools := []service.SharedPool{{ID: 274, ConfigVersion: 8}}

	mock.ExpectQuery(`(?s)SELECT DISTINCT ON \(history.pool_id\).*JOIN shared_pools pool ON pool.id = history.pool_id.*history.account_id IS NULL.*history.config_version = pool.config_version.*metadata->>'check_level'.*= 'full'`).
		WithArgs(int64(274)).
		WillReturnRows(sqlmock.NewRows([]string{"pool_id", "metadata"}).AddRow(
			int64(274),
			[]byte(`{"check_level":"full","gate_passed":true,"full_check_passed":15,"full_check_total":15,"full_check_score":100}`),
		))

	require.NoError(t, repo.hydrateSharedPoolProbeSummaries(context.Background(), pools))
	require.Equal(t, 15, pools[0].LastProbeFullCheckTotal)
	require.InDelta(t, 100, pools[0].LastProbeFullCheckScore, 1e-12)
	require.NoError(t, mock.ExpectationsWereMet())
}
