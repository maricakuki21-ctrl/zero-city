package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolSub2IdentityMigrationOwnsOnlyIdentityAndAuditState(t *testing.T) {
	content, err := FS.ReadFile("235_shared_pool_sub2_identity_bindings.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	bindingDDL := sql[strings.Index(sql, "create table if not exists shared_pool_sub2_bindings"):strings.Index(sql, "create table if not exists shared_pool_supply_dispositions")]
	dispositionEnd := strings.Index(sql, "create unique index if not exists idx_shared_pool_supply_dispositions_account")
	dispositionDDL := sql[strings.Index(sql, "create table if not exists shared_pool_supply_dispositions"):dispositionEnd]
	identityDDL := bindingDDL + dispositionDDL

	for _, fragment := range []string{
		"create table if not exists shared_pool_sub2_bindings",
		"canonical_group_id bigint",
		"references groups(id)",
		"create table if not exists shared_pool_supply_dispositions",
		"canonical_account_id bigint",
		"references accounts(id)",
		"source_checksum char(64)",
		"('mapped', 'quarantined', 'invalid', 'manual-review')",
		"duplicate_account",
		"orphan_pool",
		"orphan_owner",
		"cross_pool_reuse",
		"model_alias_collision",
		"invalid_credential",
		"reject_shared_pool_supply_disposition_mutation",
	} {
		require.Contains(t, sql, fragment)
	}

	for _, forbidden := range []string{
		"api_key text",
		"credentials jsonb",
		"concurrency int",
		"health_status",
		"total_calls",
		"successful_calls",
		"failed_calls",
		"delete from shared_pool",
		"truncate",
	} {
		require.NotContains(t, identityDDL, forbidden)
	}
	require.NotContains(t, sql, "delete from shared_pool")
	require.NotContains(t, sql, "truncate")
}

func TestSharedPoolSub2IdentityMigrationIsRepeatSafeAndCredentialSafe(t *testing.T) {
	content, err := FS.ReadFile("235_shared_pool_sub2_identity_bindings.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "on conflict do nothing")
	require.Contains(t, sql, "encode(sha256(convert_to(")
	require.Contains(t, strings.Join(strings.Fields(sql), " "), "jsonb_build_object( 'api_key'")
	require.Contains(t, sql, "credential_hash")
	require.Contains(t, sql, "source_checksum")
	require.NotContains(t, sql, "raise notice")
	require.NotContains(t, sql, "raise log")
	require.NotContains(t, sql, "returning upstream_api_key")
	require.NotContains(t, sql, "returning credentials_encrypted")
}

func TestSharedPoolSub2IdentityMigrationPinsCanonicalConcurrencyAndKeepsSourceOnlyAsAudit(t *testing.T) {
	content, err := FS.ReadFile("235_shared_pool_sub2_identity_bindings.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "source_concurrency int not null")
	require.Contains(t, sql, "'bizdecipher_source_concurrency', source.source_concurrency")
	require.NotContains(t, sql, "canonical_concurrency")

	accountInsertStart := strings.Index(sql, "insert into accounts (")
	require.NotEqual(t, -1, accountInsertStart)
	accountInsertEnd := strings.Index(sql[accountInsertStart:], "on conflict do nothing")
	require.NotEqual(t, -1, accountInsertEnd)
	accountInsert := sql[accountInsertStart : accountInsertStart+accountInsertEnd]
	require.Contains(t, accountInsert, "    1,\n    source.canonical_priority,")
}
