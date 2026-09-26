package migrations

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolIdentitySoftDeleteCompatibilityPrecedesBindings(t *testing.T) {
	const name = "234a_shared_pool_identity_soft_delete_compat.sql"
	names, err := fs.Glob(FS, "*.sql")
	require.NoError(t, err)
	sort.Strings(names)
	index := sort.SearchStrings(names, name)
	require.Less(t, index, len(names)-1)
	require.Equal(t, name, names[index])
	bindingIndex := sort.SearchStrings(names, "235_shared_pool_sub2_identity_bindings.sql")
	require.Less(t, index, bindingIndex)
	if compatIndex := sort.SearchStrings(names, "234b_shared_pools_deleted_at_compat.sql"); compatIndex < len(names) && names[compatIndex] == "234b_shared_pools_deleted_at_compat.sql" {
		require.Less(t, compatIndex, bindingIndex)
	}

	content, err := FS.ReadFile(name)
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "add column if not exists deleted_at timestamptz")
	require.Contains(t, sql, "set deleted_at = archived_at")
	require.Contains(t, sql, "where lifecycle_state = 'archived'")
	require.Contains(t, sql, "and archived_at is not null")
	require.Contains(t, sql, "and deleted_at is null")
	require.NotContains(t, sql, "delete from")
	require.NotContains(t, sql, "drop ")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "schema_migrations")
}
