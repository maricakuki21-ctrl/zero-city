package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolUsageReviewResolutionMigrationIsAdditiveAndAuditable(t *testing.T) {
	content, err := FS.ReadFile("226_shared_pool_usage_review_resolutions.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "set local lock_timeout")
	require.Contains(t, sql, "create table if not exists shared_pool_usage_review_resolutions")
	require.Contains(t, sql, "reservation_id bigint not null")
	require.Contains(t, sql, "admin_user_id bigint not null")
	require.Contains(t, sql, "operation_id varchar(160) not null")
	require.Contains(t, sql, "trigger_reason varchar(160) not null")
	require.Contains(t, sql, "unique (reservation_id)")
	require.Contains(t, sql, "unique (operation_id)")
	require.Contains(t, sql, "on delete restrict")
	require.Contains(t, sql, "create or replace function sync_shared_pool_usage_review_trace")
	require.Contains(t, sql, "after update of status on shared_pool_usage_reservations")
	require.Contains(t, sql, "update shared_pool_usage_traces")
	require.Contains(t, sql, "settlement_outcome = 'settled'")
	require.Contains(t, sql, "settlement_outcome = 'released'")
	require.NotContains(t, sql, "delete from")
	require.NotContains(t, sql, "drop table")
	require.NotContains(t, sql, "update users")
}
