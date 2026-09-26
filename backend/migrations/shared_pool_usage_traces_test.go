package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolUsageTraceMigrationIsAdditiveAndCredentialFree(t *testing.T) {
	content, err := FS.ReadFile("225_shared_pool_usage_traces.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "create table if not exists shared_pool_usage_traces")
	require.Contains(t, sql, "unique (access_key_id, request_id)")
	require.Contains(t, sql, "pool_name_snapshot")
	require.Contains(t, sql, "cache_read_tokens")
	require.Contains(t, sql, "first_token_ms")
	require.Contains(t, sql, "settlement_latency_ms")
	require.Contains(t, sql, "failure_stage")
	require.Contains(t, sql, "retry_count")
	require.NotContains(t, sql, "upstream_base_url")
	require.NotContains(t, sql, "api_key text")
	require.NotContains(t, sql, "oauth_credentials")
	require.NotContains(t, sql, "proxy_url")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "update shared_pool_balance_ledger")
	require.NotContains(t, sql, "delete from")
	require.NotContains(t, sql, "drop table")
}
