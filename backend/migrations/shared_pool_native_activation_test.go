package migrations_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolNativeActivationMigration_isExplicitAndRevocable(t *testing.T) {
	data, err := os.ReadFile("246_shared_pool_native_billing_activation.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(data))
	require.Contains(t, sql, "'billing_active'")
	require.Contains(t, sql, "native_activation_operation_id")
	require.Contains(t, sql, "revoke_native_pool_activation_on_config_change")
	require.Contains(t, sql, "bizdecipher.native_activation_revocation")
	require.Contains(t, sql, "set schedulable = false")
	require.NotContains(t, sql, "update users set balance")
	require.NotContains(t, sql, "legacy_existing' where")
}
