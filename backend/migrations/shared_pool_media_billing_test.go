package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolMediaBillingMigrationFencesEvidenceAndConfiguration(t *testing.T) {
	content, err := FS.ReadFile("227_shared_pool_media_billing.sql")
	require.NoError(t, err)
	sql := string(content)

	for _, fragment := range []string{
		"shared_pool_media_endpoint_probes_passed_evidence_check",
		"output_observed AND (endpoint_type <> 'video' OR async_terminal_observed)",
		"pool_config_version BIGINT NOT NULL",
		"account_config_version BIGINT NULL",
		"FOREIGN KEY (account_id, pool_id)",
		"validate_shared_pool_media_probe_scope",
		"validate_shared_pool_endpoint_last_media_probe",
		"invalidate_shared_pool_media_endpoint_on_config_change",
		"invalidate_shared_pool_media_endpoints_from_pool",
		"SET config_version = endpoint.config_version + 1",
		"gate_status = CASE",
		"media_probe_expires_at = NULL",
	} {
		require.Contains(t, sql, fragment)
	}

	require.NotContains(t, strings.ToUpper(sql), "DROP TABLE SHARED_POOL")
	require.NotContains(t, strings.ToUpper(sql), "TRUNCATE")
}

func TestSharedPoolMediaTasksKeepCredentialFreeHistoricalRouteIdentity(t *testing.T) {
	content, err := FS.ReadFile("227_shared_pool_media_billing.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "account_id BIGINT NULL REFERENCES shared_pool_accounts(id) ON DELETE RESTRICT")
	require.Contains(t, sql, "price_version_id BIGINT NOT NULL REFERENCES shared_pool_price_versions(id) ON DELETE RESTRICT")
	require.Contains(t, sql, "upstream_model_snapshot VARCHAR(200) NOT NULL")
	require.Contains(t, sql, "idx_shared_pool_media_tasks_recovery")
	require.NotContains(t, strings.ToLower(sql), "credential_snapshot")
	require.NotContains(t, strings.ToLower(sql), "upstream_api_key_snapshot")
}
