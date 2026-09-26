package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolRoutingUsesPoolModelRateAndPreservesUnlimitedModelConcurrency(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, actual string) error {
		query = actual
		return nil
	})))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectQuery("capture shared-pool route").
		WithArgs(int64(42), "gpt-5.6-luna").
		WillReturnRows(sqlmock.NewRows([]string{"unused"}))

	accessKey, err := repo.getSharedPoolAccessKeyByAPIKeyID(context.Background(), 42, "gpt-5.6-luna", "", false)
	require.NoError(t, err)
	require.Nil(t, accessKey)
	require.Contains(t, query, "COALESCE(NULLIF(spm_req.rate_multiplier, 0), NULLIF(sp.rate_multiplier, 0), 1) AS effective_rate_multiplier")
	require.NotContains(t, query, "model_rate_multiplier", "account model JSON must not influence user pricing")
	require.Contains(t, query, "COALESCE(NULLIF(spa_req.model_concurrency, 0), COALESCE(spm_req.max_concurrency, 0)) AS model_concurrency")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolEndpointPricingFallsBackFromModelRateToPoolRate(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, actual string) error {
		query = actual
		return nil
	})))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectQuery("capture shared-pool pricing").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"unused"}))

	items, err := repo.ListSharedPoolModelEndpointPricing(context.Background(), 9, 7)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Contains(t, query, "COALESCE(NULLIF(spm.rate_multiplier, 0), NULLIF(sp.rate_multiplier, 0), 1)")
	require.NoError(t, mock.ExpectationsWereMet())
}
