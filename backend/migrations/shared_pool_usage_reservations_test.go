package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolUsageReservationMigrationProtectsBalancesAndAuditHistory(t *testing.T) {
	content, err := FS.ReadFile("223_shared_pool_usage_reservations.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "set local lock_timeout")
	require.Contains(t, sql, "create table if not exists shared_pool_usage_reservations")
	require.Contains(t, sql, "request_fingerprint char(64)")
	require.Contains(t, sql, "hold_amount numeric(24, 12)")
	require.Contains(t, sql, "usage_payload jsonb")
	require.Contains(t, sql, "settled_amount <= hold_amount")
	require.Contains(t, sql, "'settlement_pending'")
	require.Contains(t, sql, "'review_required'")
	require.Contains(t, sql, "unique (access_key_id, request_id)")
	require.Contains(t, sql, "where status in ('reserved', 'forwarding')")
	require.Contains(t, sql, "on delete restrict")
	require.Contains(t, sql, "add constraint shared_pool_usage_reservations_price_version_id_fkey")
	require.Contains(t, sql, "validate constraint shared_pool_usage_reservations_price_version_id_fkey")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "delete from")
	require.NotContains(t, sql, "drop table")
}
