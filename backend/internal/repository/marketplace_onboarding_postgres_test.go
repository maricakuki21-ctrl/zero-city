package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMarketplaceOnboardingPostgres(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	var database string
	require.NoError(t, db.QueryRow("SELECT current_database()").Scan(&database))
	require.True(t, strings.HasPrefix(database, "bizdecipher_columns_acceptance_test_"))
	first := insertMarketplaceTestUser(t, db, fmt.Sprintf("guide-first-%d@example.test", time.Now().UnixNano()))
	second := insertMarketplaceTestUser(t, db, fmt.Sprintf("guide-second-%d@example.test", time.Now().UnixNano()))
	defer func() {
		_, err := db.Exec("DELETE FROM marketplace_onboarding WHERE user_id IN ($1,$2)", first, second)
		require.NoError(t, err)
		_, err = db.Exec("DELETE FROM users WHERE id IN ($1,$2)", first, second)
		require.NoError(t, err)
	}()
	repo := &marketplaceRepository{db: db}
	ctx := context.Background()
	before, err := repo.GetMarketplaceOnboarding(ctx, first, 1)
	require.NoError(t, err)
	require.Nil(t, before.CompletedAt)
	saved, err := repo.SaveMarketplaceOnboarding(ctx, first, 1, "hire")
	require.NoError(t, err)
	require.NotNil(t, saved.CompletedAt)
	reloaded, err := repo.GetMarketplaceOnboarding(ctx, first, 1)
	require.NoError(t, err)
	require.Equal(t, "hire", reloaded.Intent)
	require.Equal(t, saved.CompletedAt, reloaded.CompletedAt)
	repeated, err := repo.SaveMarketplaceOnboarding(ctx, first, 1, "browse")
	require.NoError(t, err)
	require.Equal(t, saved.CompletedAt, repeated.CompletedAt)
	other, err := repo.GetMarketplaceOnboarding(ctx, second, 1)
	require.NoError(t, err)
	require.Nil(t, other.CompletedAt)
}
