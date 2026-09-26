package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestProviderQuotaAlignmentPostgres(t *testing.T) {
	dsn := os.Getenv("QUOTA_ALIGNMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("QUOTA_ALIGNMENT_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
	require.True(t, strings.HasPrefix(database, "bizdecipher_quota_acceptance_test_"), "requires disposable database")
	_, err = db.ExecContext(ctx, `CREATE TABLE user_platform_quotas (
		user_id BIGINT NOT NULL, platform VARCHAR(32) NOT NULL
		CONSTRAINT user_platform_quotas_platform_check CHECK(platform IN ('anthropic','openai','gemini','antigravity','grok')),
		daily_limit_usd NUMERIC(20,10), daily_usage_usd NUMERIC(20,10) NOT NULL DEFAULT 0,
		PRIMARY KEY(user_id, platform));
		INSERT INTO user_platform_quotas VALUES(1,'openai',5,2.5);`)
	require.NoError(t, err)
	body, err := migrations.FS.ReadFile("272_user_platform_quota_provider_alignment.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, string(body))
		if err != nil {
			_ = tx.Rollback()
		}
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	for _, platform := range []string{"kimi", "zhipu", "deepseek", "minimax", "opencode_go"} {
		_, err = db.ExecContext(ctx, `INSERT INTO user_platform_quotas VALUES(2,$1,7,3)`, platform)
		require.NoError(t, err, platform)
		var limit, usage float64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT daily_limit_usd,daily_usage_usd FROM user_platform_quotas WHERE user_id=2 AND platform=$1`, platform).Scan(&limit, &usage))
		require.Equal(t, 7.0, limit)
		require.Equal(t, 3.0, usage)
	}
	var limit, usage float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT daily_limit_usd,daily_usage_usd FROM user_platform_quotas WHERE user_id=1`).Scan(&limit, &usage))
	require.Equal(t, 5.0, limit)
	require.Equal(t, 2.5, usage)
	_, err = db.ExecContext(ctx, `INSERT INTO user_platform_quotas VALUES(2,'invalid',7,3)`)
	require.Error(t, err)
}
