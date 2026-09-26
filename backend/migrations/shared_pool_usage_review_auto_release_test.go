package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolUsageReviewAutoReleaseMigrationOnlyAddsIndex(t *testing.T) {
	content, err := FS.ReadFile("234_shared_pool_usage_review_auto_release_index_notx.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "create index concurrently if not exists idx_shared_pool_usage_reservations_review_age")
	require.Contains(t, sql, "where status = 'review_required'")
	require.NotContains(t, sql, "update shared_pool_usage_reservations")
	require.NotContains(t, sql, "update users")
}
