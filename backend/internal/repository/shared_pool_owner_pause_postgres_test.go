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

// Runs only against an explicitly provisioned isolated database, never production.
func TestSharedPoolOwnerPausePostgres(t *testing.T) {
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
	ctx := context.Background()
	owner := insertMarketplaceTestUser(t, db, fmt.Sprintf("pause-owner-%d@example.test", time.Now().UnixNano()))
	defer func() {
		_, err := db.Exec("DELETE FROM shared_pools WHERE owner_id=$1", owner)
		require.NoError(t, err)
		_, err = db.Exec("DELETE FROM users WHERE id=$1", owner)
		require.NoError(t, err)
	}()
	repo := &bizDecipherRepository{db: db}
	for _, state := range []string{"legacy_existing", "billing_active"} {
		var id, version int64
		require.NoError(t, db.QueryRow(`INSERT INTO shared_pools(owner_id,name,listed,status,lifecycle_state,native_onboarding_state)
			VALUES($1,$2,TRUE,'healthy','operating',$3) RETURNING id,config_version`, owner, "pause-"+state, state).Scan(&id, &version))
		result, err := repo.SetSharedPoolOwnerPauseTx(ctx, id, owner, version, true)
		require.NoError(t, err)
		require.True(t, result.OwnerPaused)
		require.False(t, result.Listed)
		require.Equal(t, "suspended", result.LifecycleState)
		// A successful background probe is not permission to reopen a manual pause.
		_, err = db.Exec(`UPDATE shared_pools SET listed=TRUE,status='healthy',lifecycle_state='operating' WHERE id=$1`, id)
		require.NoError(t, err)
		var listed bool
		var lifecycle, status string
		require.NoError(t, db.QueryRow(`SELECT listed,lifecycle_state,status,config_version FROM shared_pools WHERE id=$1`, id).Scan(&listed, &lifecycle, &status, &version))
		require.False(t, listed)
		require.Equal(t, "suspended", lifecycle)
		require.Equal(t, "maintenance", status)
		result, err = repo.SetSharedPoolOwnerPauseTx(ctx, id, owner, version, false)
		require.NoError(t, err)
		require.False(t, result.OwnerPaused)
		require.False(t, result.Listed)
		require.Equal(t, "draft", result.LifecycleState)
		if state == "billing_active" {
			require.Equal(t, "supply_ready_billing_blocked", result.NativeOnboardingState)
		}
	}
}
