package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckinMilestoneHardeningMigrationOnlyBackfillsAuditLedger(t *testing.T) {
	content, err := FS.ReadFile("220_checkin_milestone_cycle_and_balance_ledger_hardening.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "set local lock_timeout")
	require.Contains(t, sql, "set local statement_timeout")
	require.Contains(t, sql, "array['user_id', 'cycle_no', 'milestone_days']")
	require.Contains(t, sql, "array['user_id', 'milestone_days']")
	require.Contains(t, sql, "i.indisvalid")
	require.Contains(t, sql, "i.indisready")
	require.Contains(t, sql, "'checkin_milestone'")
	require.Contains(t, sql, "insert into user_balance_ledger")
	require.Contains(t, sql, "from checkin_milestone_claims claim")
	require.Contains(t, sql, "claim.balance_reward > 0")
	require.Contains(t, sql, "on conflict do nothing")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "insert into users")
}

func TestDailyFortuneOperationMigrationAddsScopedUniqueness(t *testing.T) {
	content, err := FS.ReadFile("221_daily_fortune_operation_id.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "add column if not exists operation_id varchar(128) not null default ''")
	require.Contains(t, sql, "on daily_checkins(user_id, operation_id)")
	require.Contains(t, sql, "where operation_id <> ''")
}
