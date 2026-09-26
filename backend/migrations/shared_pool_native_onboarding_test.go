package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolNativeOnboardingMigration_extends_canonical_identity(t *testing.T) {
	raw, err := os.ReadFile("243_shared_pool_native_onboarding.sql")
	require.NoError(t, err)
	sql := string(raw)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS shared_pool_native_onboarding_progress",
		"native_binding_ref",
		"native_operation_id",
		"native_request_fingerprint",
		"native_binding_state",
		"native_account_observed_updated_at",
		"native_repair_operation_id",
		"native_repair_request_fingerprint",
		"native_repair_expected_config_version",
		"native_repair_state",
		"native_repair_started_at",
		"native_repair_completed_at",
		"accounts((extra->>'shared_pool_binding_ref'))",
		"shared_pool_native_repair_operation_unique",
		"ON DELETE RESTRICT",
		"validate_shared_pool_native_progress_source",
	} {
		require.Contains(t, sql, fragment)
	}
	require.NotContains(t, strings.ToLower(sql), "insert into groups")
	require.NotContains(t, strings.ToLower(sql), "insert into account_groups")
	require.NotContains(t, strings.ToLower(sql), "set schedulable = true")
	require.NotContains(t, strings.ToLower(sql), "native_account_id")
	require.NotContains(t, strings.ToLower(sql), "canonical_account_id")
	require.NotContains(t, strings.ToLower(sql), "update shared_pool_supply_dispositions")
	require.NotContains(t, strings.ToLower(sql), "delete from shared_pool_supply_dispositions")
}
