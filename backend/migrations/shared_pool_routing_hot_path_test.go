package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolRoutingHotPathIndexesAreOnlineAndAdditive(t *testing.T) {
	content, err := FS.ReadFile("228_shared_pool_routing_hot_path_indexes_notx.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "create index concurrently if not exists idx_shared_pool_models_aliases_open_gin")
	require.Contains(t, sql, "using gin (model_aliases)")
	require.Contains(t, sql, "create index concurrently if not exists idx_shared_pool_accounts_schedulable_route")
	require.Contains(t, sql, "where deleted_at is null")
	require.NotContains(t, sql, "update shared_pool")
	require.NotContains(t, sql, "delete from")
}
