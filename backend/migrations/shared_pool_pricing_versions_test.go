package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolPricingMigrationIsAppendOnlyAndIdempotent(t *testing.T) {
	content, err := FS.ReadFile("217_shared_pool_endpoint_pricing_versions.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "create table if not exists shared_pool_price_versions")
	require.Contains(t, sql, "add column if not exists operation_id")
	require.Contains(t, sql, "shared_pool_price_versions_operation_unique")
	require.Contains(t, sql, "drop index if exists shared_pool_price_versions_hash_unique")
	require.Contains(t, sql, "billing_mode = 'token' and input_price is not null and output_price is not null")
	require.Contains(t, sql, "('chat'::varchar, '/v1/chat/completions'::text)")
	require.Contains(t, sql, "('responses'::varchar, '/v1/responses'::text)")
	require.Contains(t, sql, "ensure_shared_pool_text_endpoints")
	require.Contains(t, sql, "validate constraint shared_pool_price_versions_complete_check")
	require.Contains(t, sql, "add constraint shared_pool_price_versions_created_by_fkey")
	require.Contains(t, sql, "validate constraint shared_pool_price_versions_created_by_fkey")
	require.NotContains(t, sql, "created_by bigint null references users(id) on delete set null")
	require.Contains(t, sql, "create or replace function reject_shared_pool_price_version_mutation()")
	require.Contains(t, sql, "before update or delete on shared_pool_price_versions")
	require.Contains(t, sql, "raise exception 'shared_pool_price_versions is append-only")
	require.NotContains(t, sql, "delete from shared_pool_price_versions")
	require.NotContains(t, sql, "update shared_pool_price_versions\nset input_price")
}

func TestSharedPoolPriceVersionForeignKeysRestrictDeletionAndValidate(t *testing.T) {
	tests := []struct {
		migration  string
		table      string
		constraint string
	}{
		{
			migration:  "218_shared_pool_owner_earnings_wallet.sql",
			table:      "shared_pool_owner_earnings_ledger",
			constraint: "shared_pool_owner_earnings_ledger_price_version_id_fkey",
		},
		{
			migration:  "218_shared_pool_owner_earnings_wallet.sql",
			table:      "shared_pool_balance_ledger",
			constraint: "shared_pool_balance_ledger_price_version_id_fkey",
		},
		{
			migration:  "222_shared_pool_usage_price_snapshots.sql",
			table:      "shared_pool_balance_ledger",
			constraint: "shared_pool_balance_ledger_price_version_id_fkey",
		},
		{
			migration:  "223_shared_pool_usage_reservations.sql",
			table:      "shared_pool_usage_reservations",
			constraint: "shared_pool_usage_reservations_price_version_id_fkey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.migration+"/"+tt.table, func(t *testing.T) {
			content, err := FS.ReadFile(tt.migration)
			require.NoError(t, err)
			sql := strings.ToLower(string(content))

			require.Contains(t, sql, "alter table "+tt.table)
			require.Contains(t, sql, "add constraint "+tt.constraint)
			require.Contains(t, sql, "references shared_pool_price_versions(id)")
			require.Contains(t, sql, "on delete restrict")
			require.Contains(t, sql, "validate constraint "+tt.constraint)
			require.NotContains(t, sql, "references shared_pool_price_versions(id) on delete set null")
		})
	}
}

func TestSharedPoolOwnerWalletMigrationDefaultsNewActivityToUserBalance(t *testing.T) {
	content, err := FS.ReadFile("218_shared_pool_owner_earnings_wallet.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	legacyDefault := "add column if not exists settlement_destination varchar(32) not null default 'legacy_balance'"
	activityDefault := "alter column settlement_destination set default 'user_balance'"
	require.Contains(t, sql, legacyDefault)
	require.Contains(t, sql, activityDefault)
	require.Less(t, strings.Index(sql, legacyDefault), strings.Index(sql, activityDefault))
}

func TestSharedPoolOwnerWalletMigrationClassifiesDeletedPoolHistoryIndependently(t *testing.T) {
	content, err := FS.ReadFile("218_shared_pool_owner_earnings_wallet.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	classificationStart := strings.Index(sql, "update shared_pool_balance_ledger\nset settlement_destination")
	snapshotStart := strings.Index(sql, "update shared_pool_balance_ledger ledger\nset pool_name_snapshot")
	require.NotEqual(t, -1, classificationStart)
	require.NotEqual(t, -1, snapshotStart)
	require.Less(t, classificationStart, snapshotStart)

	classificationSQL := sql[classificationStart:snapshotStart]
	for sourceType, destination := range map[string]string{
		"share_pool_usage":  "user_balance",
		"pool_seat_fee":     "user_balance",
		"share_pool_payout": "legacy_balance",
		"pool_owner_payout": "legacy_balance",
	} {
		require.Contains(t, classificationSQL, "when '"+sourceType+"' then '"+destination+"'")
	}
	require.Contains(t, classificationSQL, "where owner_earnings_ledger_id is null")
	// A deleted pool leaves pool_id NULL. Classification must therefore have no
	// pool_id predicate or shared_pools join.
	require.NotContains(t, classificationSQL, "pool_id")
	require.NotContains(t, classificationSQL, "from shared_pools")

	snapshotSQL := sql[snapshotStart:]
	require.Contains(t, snapshotSQL, "from shared_pools pool")
	require.Contains(t, snapshotSQL, "where pool.id = ledger.pool_id")

	// The migration classifies metadata only; it must not copy historical
	// earnings into the wallet or modify either balance.
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "update shared_pool_owner_wallets")
	require.NotContains(t, sql, "insert into shared_pool_owner_earnings_ledger")
}

func TestSharedPoolPriceVersionIndexesUseNonTransactionalMigration(t *testing.T) {
	content, err := FS.ReadFile("224_shared_pool_price_version_indexes_notx.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	for _, index := range []string{
		"idx_shared_pool_owner_earnings_price_version",
		"idx_shared_pool_balance_ledger_price_version",
		"idx_shared_pool_usage_reservations_price_version",
	} {
		require.Contains(t, sql, "create index concurrently if not exists "+index)
	}
	require.NotContains(t, sql, "begin;")
	require.NotContains(t, sql, "commit;")

	for _, migration := range []string{
		"218_shared_pool_owner_earnings_wallet.sql",
		"222_shared_pool_usage_price_snapshots.sql",
		"223_shared_pool_usage_reservations.sql",
	} {
		transactionalContent, readErr := FS.ReadFile(migration)
		require.NoError(t, readErr)
		transactionalSQL := strings.ToLower(string(transactionalContent))
		require.NotContains(t, transactionalSQL, "create index if not exists idx_shared_pool_balance_ledger_price_version")
		require.NotContains(t, transactionalSQL, "create index if not exists idx_shared_pool_owner_earnings_price_version")
		require.NotContains(t, transactionalSQL, "create index if not exists idx_shared_pool_usage_reservations_price_version")
	}
}

func TestSharedPoolUsagePriceSnapshotMigrationDoesNotTouchBalances(t *testing.T) {
	content, err := FS.ReadFile("222_shared_pool_usage_price_snapshots.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "add column if not exists price_version_id")
	require.Contains(t, sql, "on delete restrict")
	require.Contains(t, sql, "validate constraint shared_pool_balance_ledger_price_version_id_fkey")
	require.Contains(t, sql, "pricing_source_snapshot")
	require.Contains(t, sql, "price_snapshot jsonb")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "update shared_pool_balance_ledger")
	require.NotContains(t, sql, "delete from")
}
