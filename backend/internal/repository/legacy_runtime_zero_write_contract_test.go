package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestLegacyRuntimeZeroWriteMigration_guards_runtime_facts_without_dropping_audit_rows(t *testing.T) {
	content, err := migrations.FS.ReadFile("237_shared_pool_cutover_control.sql")
	require.NoError(t, err)
	sqlText := string(content)

	for _, required := range []string{
		"shared_pool_cutover_attempts",
		"shared_pool_cutover_control",
		"shared_pool_cutover_transitions",
		"shared_pool_cutover_transition(",
		"shared_pool_cutover_abort_after_fence(",
		"shared_pool_cutover_mark_first_canonical_effect(",
		"assert_shared_pool_legacy_runtime_epoch(",
		"from_authority_epoch",
		"to_authority_epoch",
		"trg_guard_shared_pool_usage_windows",
		"trg_guard_shared_pool_probe_histories",
		"trg_guard_shared_pool_usage_traces",
		"trg_guard_shared_pool_probe_jobs",
		"trg_guard_shared_pool_probe_job_items",
		"trg_guard_shared_pool_media_probes",
		"trg_guard_shared_pool_runtime_counters",
		"trg_guard_shared_pool_account_runtime",
		"expected_version + 1",
	} {
		require.Contains(t, sqlText, required)
	}
	require.NotContains(t, strings.ToUpper(sqlText), "DROP TABLE SHARED_POOL")
}
