package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolAccountRoutingChangesInvalidateQueuedProbeJobs(t *testing.T) {
	content, err := FS.ReadFile("216_shared_pool_probe_jobs.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	start := strings.Index(sql, "create or replace function bump_shared_pool_config_version_from_account()")
	require.NotEqual(t, -1, start)
	endOffset := strings.Index(sql[start:], "$$ language plpgsql;")
	require.NotEqual(t, -1, endOffset)
	functionSQL := sql[start : start+endOffset]

	for _, field := range []string{
		"provider", "auth_type", "upstream_base_url", "upstream_api_key",
		"credentials_encrypted", "expires_at", "auto_pause_on_expired",
		"schedulable", "status", "status_note", "disabled_reason",
		"proxy_id", "proxy_url", "proxy_region", "proxy_status",
		"account_weight", "priority", "rpm_limit", "account_concurrency",
		"user_concurrency", "tls_profile_id", "ttl_seconds", "cache_policy",
		"routing_policy", "model_configs", "gate_required", "deleted_at",
	} {
		require.Contains(t, functionSQL, "new."+field, "missing NEW fence for %s", field)
		require.Contains(t, functionSQL, "old."+field, "missing OLD fence for %s", field)
	}

	// Probe result projection must not invalidate itself merely because latency,
	// score or gate evidence was refreshed.
	for _, resultField := range []string{"last_probe_at", "full_check_score", "gate_passed"} {
		require.NotContains(t, functionSQL, "new."+resultField)
		require.NotContains(t, functionSQL, "old."+resultField)
	}
}

func TestSharedPoolModelRoutingChangesInvalidateQueuedProbeJobsWithoutPricingFalsePositives(t *testing.T) {
	content, err := FS.ReadFile("216_shared_pool_probe_jobs.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	start := strings.Index(sql, "create or replace function bump_shared_pool_config_version_from_model()")
	require.NotEqual(t, -1, start)
	endOffset := strings.Index(sql[start:], "$$ language plpgsql;")
	require.NotEqual(t, -1, endOffset)
	functionSQL := sql[start : start+endOffset]

	for _, field := range []string{
		"pool_id", "provider", "model_name", "upstream_model_name",
		"model_aliases", "enabled", "model_open", "max_concurrency",
	} {
		require.Contains(
			t,
			functionSQL,
			"new."+field+" is distinct from old."+field,
			"missing null-safe routing fence for %s",
			field,
		)
	}

	// These fields change pricing, owner policy, presentation, or ordering only.
	// Updating them must not invalidate an otherwise valid probe/gate result.
	for _, field := range []string{
		"display_name", "sort_order", "tags", "rank_weight", "rate_multiplier",
		"five_hour_protection_percent", "seven_day_protection_percent",
		"daily_protection_percent", "min_balance_admission", "hourly_seat_fee",
		"pricing_source", "pricing_status", "pricing_config_version",
		"custom_pricing", "pricing_updated_at",
	} {
		require.NotContains(t, functionSQL, "new."+field, "unexpected NEW fence for %s", field)
		require.NotContains(t, functionSQL, "old."+field, "unexpected OLD fence for %s", field)
	}
}
