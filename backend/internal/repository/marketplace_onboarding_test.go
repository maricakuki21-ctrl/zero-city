package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceOnboardingRepositoryScopesAndUpserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &marketplaceRepository{db: db}
	mock.ExpectQuery("SELECT intent, completed_at FROM marketplace_onboarding WHERE user_id=\\$1 AND version=\\$2").
		WithArgs(int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"intent", "completed_at"}))
	state, err := repo.GetMarketplaceOnboarding(context.Background(), 7, 1)
	require.NoError(t, err)
	require.Nil(t, state.CompletedAt)
	now := time.Now()
	mock.ExpectQuery("INSERT INTO marketplace_onboarding.*ON CONFLICT.*RETURNING intent,completed_at").
		WithArgs(int64(7), 1, "browse").WillReturnRows(sqlmock.NewRows([]string{"intent", "completed_at"}).AddRow("browse", now))
	state, err = repo.SaveMarketplaceOnboarding(context.Background(), 7, 1, "browse")
	require.NoError(t, err)
	require.Equal(t, "browse", state.Intent)
	require.NotNil(t, state.CompletedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
