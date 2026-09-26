package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateSharedPoolAccessKeyTxRefreshesLegacyBindingModels(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}

	const (
		poolID    = int64(17)
		userID    = int64(23)
		seatID    = int64(31)
		bindingID = int64(41)
		apiKeyID  = int64(53)
	)
	modelsJSON := `["legacy-model","other-model"]`
	createdAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT name, status, listed, lifecycle_state, max_users, current_users, min_balance_admission, native_onboarding_state FROM shared_pools WHERE id = \$1 FOR UPDATE`).
		WithArgs(poolID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "status", "listed", "lifecycle_state", "max_users", "current_users", "min_balance_admission", "native_onboarding_state"}).
			AddRow("fixture pool", "healthy", true, "operating", 20, 1, 0, service.SharedPoolOnboardingLegacyExisting))
	mock.ExpectQuery(`(?s)SELECT id FROM pool_seat_bindings WHERE pool_id = \$1 AND user_id = \$2 AND status = 'active' FOR UPDATE`).
		WithArgs(poolID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(seatID))
	mock.ExpectQuery(`(?s)SELECT COALESCE\(jsonb_agg\(model_name ORDER BY sort_order, id\), '\[\]'::jsonb\) FROM shared_pool_models WHERE pool_id = \$1 AND enabled = TRUE AND model_open = TRUE`).
		WithArgs(poolID).
		WillReturnRows(sqlmock.NewRows([]string{"models"}).AddRow([]byte(modelsJSON)))
	mock.ExpectQuery(`(?s)UPDATE shared_pool_access_keys sak SET\s+account_mode = TRUE,\s+allowed_models = \$3::jsonb,.*RETURNING sak.id`).
		WithArgs(poolID, userID, modelsJSON).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "pool_id", "pool_name", "user_id", "api_key_id", "name", "key", "status", "allowed_models", "total_used", "last_used_at", "created_at",
		}).AddRow(bindingID, poolID, "fixture pool", userID, apiKeyID, "legacy key", "sk-share-existing-secret", service.StatusActive, []byte(modelsJSON), 0.0, nil, createdAt))
	mock.ExpectCommit()

	result, err := repo.CreateSharedPoolAccessKeyTx(context.Background(), poolID, userID, "Unified shared key", "sk-share-unused")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.AlreadyHeld)
	require.Equal(t, bindingID, result.AccessKey.ID)
	require.True(t, result.AccessKey.AccountMode)
	require.Equal(t, []string{"legacy-model", "other-model"}, result.AccessKey.AllowedModels)
	require.NoError(t, mock.ExpectationsWereMet())
}
